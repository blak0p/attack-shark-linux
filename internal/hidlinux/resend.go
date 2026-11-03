package hidlinux

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/blak0p/attack-shark-linux/internal/mouse"
)

// Resend policy for feature reports the X6 receiver acknowledges. The receiver
// sometimes drops a write: it answers nothing, or it answers 03 10 xx yy zz
// carrying mouse motion instead of the ACK. The vendor tool resends the same
// payload in both cases, and so does every caller of awaitACKWithResend.
const (
	defaultMaxAttempts = 4
	defaultResendDelay = 500 * time.Millisecond
	// defaultSilenceDelay matches the vendor tool, which resends after ~0.5–0.7s
	// when the dongle answers nothing at all.
	defaultSilenceDelay = 700 * time.Millisecond
)

// awaitACKWithResend reads node after payload was written once and returns
// when continueReading reports the ACK. It resends the identical payload after
// silenceDelay without an answer, or after resendDelay once the receiver
// rejects it, up to resendAttempts writes in total. Each write restarts the
// readTimeout deadline. attempts counts the writes, the first one included;
// reports counts the reports read.
//
// Only use it for payloads that are safe to apply twice (whole settings
// reports); never for chunked uploads whose order the device tracks.
func (b *HidrawBackend) awaitACKWithResend(ctx context.Context, node hidrawNode, binding mouse.Binding, payload []byte, continueReading func([]byte) bool) (attempts, reports int, err error) {
	started := time.Now()
	attempts = 1
	maxAttempts, resendDelay, silenceDelay := b.resendAttempts(), b.resendDelay, b.silenceDelay
	incoming, next, stop := readNodeReports(node)
	defer stop()
	deadline := time.NewTimer(b.readTimeout)
	defer deadline.Stop()
	// resend fires after silenceDelay without any ACK, or sooner (resendDelay)
	// once the dongle rejects the write; both resend the identical payload.
	var resend <-chan time.Time
	var resendTimer *time.Timer
	armResend := func(delay time.Duration) {
		if resendTimer != nil {
			resendTimer.Stop()
		}
		resendTimer = time.NewTimer(delay)
		resend = resendTimer.C
	}
	defer func() {
		if resendTimer != nil {
			resendTimer.Stop()
		}
	}()
	if silenceDelay > 0 && maxAttempts > 1 {
		armResend(silenceDelay)
	}
	var lastRejected []byte
	rejected := false
	for {
		select {
		case <-ctx.Done():
			return attempts, reports, &diagnosticError{operation: "ack_failure", err: b.ackTimeout(ctx.Err(), attempts, lastRejected)}
		case <-deadline.C:
			return attempts, reports, &diagnosticError{operation: "ack_failure", err: b.ackTimeout(context.DeadlineExceeded, attempts, lastRejected)}
		case <-resend:
			resend = nil
			if attempts >= maxAttempts {
				if rejected {
					return attempts, reports, &diagnosticError{operation: "ack_failure", err: b.ackTimeout(errors.New("no ACK after final attempt"), attempts, lastRejected)}
				}
				continue
			}
			attempts++
			reason := "silence"
			if rejected {
				reason = "rejected"
			}
			inventoryDiagnosticf("event=command_resend report_id=%#02x attempt=%d reason=%s rejected=%x elapsed_ms=%d", payloadID(payload), attempts, reason, lastRejected, time.Since(started).Milliseconds())
			rejected = false
			count, err := node.SendFeatureReport(payload)
			if err != nil {
				return attempts, reports, &diagnosticError{operation: "transfer", err: classify(err)}
			}
			if count != len(payload) {
				return attempts, reports, &diagnosticError{operation: "transfer", err: fmt.Errorf("feature report wrote %d bytes, want %d", count, len(payload))}
			}
			deadline.Reset(b.readTimeout)
			if silenceDelay > 0 && attempts < maxAttempts {
				armResend(silenceDelay)
			}
		case read := <-incoming:
			if read.err != nil {
				return attempts, reports, &diagnosticError{operation: "ack_failure", err: classify(read.err)}
			}
			reports++
			report := read.report
			b.dispatchListenerReport(binding.Path, report)
			more := continueReading(report)
			inventoryDiagnosticf("event=command_report report_id=%#02x index=%d elapsed_ms=%d ack=%t sent=%x received=%x", payloadID(payload), reports, time.Since(started).Milliseconds(), !more, payload, report)
			if !more {
				return attempts, reports, nil
			}
			next()
			if !rejected && isRejectionReport(report) {
				rejected = true
				lastRejected = report
				armResend(resendDelay)
			}
		}
	}
}

func (b *HidrawBackend) resendAttempts() int {
	if b.maxAttempts <= 0 {
		return defaultMaxAttempts
	}
	return b.maxAttempts
}

func (b *HidrawBackend) ackTimeout(cause error, attempts int, rejected []byte) error {
	if len(rejected) > 0 {
		cause = fmt.Errorf("device rejected %d attempts (last %x): %w", attempts, rejected, cause)
	}
	return &Error{Kind: Timeout, Err: cause}
}

type nodeReport struct {
	report []byte
	err    error
}

// readNodeReports reads node on demand (one outstanding read, started by
// next) so a resend can reuse the node without a timed-out read closing it.
// stop releases the reader.
func readNodeReports(node hidrawNode) (<-chan nodeReport, func(), func()) {
	out := make(chan nodeReport, 1)
	want := make(chan struct{}, 1)
	done := make(chan struct{})
	want <- struct{}{}
	go func() {
		buffer := make([]byte, 64)
		for {
			select {
			case <-want:
			case <-done:
				return
			}
			count, err := node.Read(buffer)
			item := nodeReport{err: err}
			if err == nil {
				item.report = append([]byte(nil), buffer[:count]...)
			}
			select {
			case out <- item:
			case <-done:
				return
			}
			if err != nil {
				return
			}
		}
	}()
	return out, func() { want <- struct{}{} }, func() { close(done) }
}

// isRejectionReport reports whether an unexpected report looks like the
// dongle's 03 10 xx yy zz answer carrying mouse motion instead of the ACK.
// Battery heartbeats (xx=0x40) and DPI stage events (xx=0x10) are not.
func isRejectionReport(report []byte) bool {
	return len(report) == 5 && report[0] == 0x03 && report[1] == 0x10 && report[2] != 0x40 && report[2] != 0x10
}
