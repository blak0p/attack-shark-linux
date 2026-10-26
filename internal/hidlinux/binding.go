package hidlinux

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/blak0p/attack-shark-linux/internal/mouse"
	"github.com/blak0p/attack-shark-linux/internal/transport"
)

const (
	pollingReportLength  = 9
	lightingReportLength = 13
	remapReportLength    = 59

	defaultMaxAttempts = 4
	defaultResendDelay = 500 * time.Millisecond
	// defaultSilenceDelay matches the vendor tool, which resends after ~0.5–0.7s
	// when the dongle answers nothing at all.
	defaultSilenceDelay = 700 * time.Millisecond
)

// SendAndAwaitBound revalidates one captured binding and uses only its matching
// hidraw node. It performs no USB claims, resets, rebinding, or live discovery
// fallback to another device.
func (b *HidrawBackend) SendAndAwaitBound(ctx context.Context, binding mouse.Binding, payload []byte, continueReading func([]byte) bool) (err error) {
	started, stage, reports, attempts := time.Now(), "validate", 0, 1
	defer func() {
		outcome := "ok"
		if err != nil {
			outcome = "error"
		}
		inventoryDiagnosticf("event=command report_id=%#02x len=%d stage=%s reports=%d attempts=%d duration_ms=%d outcome=%s error=%q", payloadID(payload), len(payload), stage, reports, attempts, time.Since(started).Milliseconds(), outcome, errorText(err))
	}()
	if !supportedFeatureReport(payload) {
		return &Error{Kind: Mismatch, Err: fmt.Errorf("unsupported feature report ID/length = %#x/%d", payloadID(payload), len(payload))}
	}
	if err := ctx.Err(); err != nil {
		return classify(err)
	}
	if _, err := b.Enumerate(ctx, transport.Match{VendorID: binding.ID.VendorID, ProductID: binding.ID.ProductID}); err != nil {
		return mouse.ErrStaleBinding
	}
	b.mu.Lock()
	candidate, ok := b.sources[binding.Path]
	b.mu.Unlock()
	serialMatches := b.serial(candidate) == binding.ID.Serial
	if binding.SessionOnly {
		serialMatches = b.serial(candidate) == ""
	}
	if !ok || candidateKey(candidate) != binding.Path || candidate.VendorID != binding.ID.VendorID || candidate.ProductID != binding.ID.ProductID || !serialMatches {
		return mouse.ErrStaleBinding
	}
	if _, err := b.ValidateDescriptor(ctx, transport.Candidate{Path: binding.Path}, transport.InputDescriptor{InterfaceNumber: hidInterface, UsagePage: 1, Usage: 0x80, EndpointAddress: statusEndpoint}); err != nil {
		return mouse.ErrStaleBinding
	}
	path, err := b.hidrawPath(candidate)
	if err != nil {
		return mouse.ErrStaleBinding
	}
	stage = "begin_command"
	release, err := b.beginCommand(ctx)
	if err != nil {
		return &diagnosticError{operation: "ack_failure", err: err}
	}
	defer release()
	// Commands own a separate node from the always-on listener. The listener
	// may close its node while its bounded read shuts down.
	stage = "open"
	node, err := b.opener.OpenNode(path)
	if err != nil {
		return &diagnosticError{operation: "transfer", err: classify(err)}
	}
	defer node.Close()
	stage = "write"
	count, err := node.SendFeatureReport(payload)
	if err != nil {
		return &diagnosticError{operation: "transfer", err: classify(err)}
	}
	if count != len(payload) {
		return &diagnosticError{operation: "transfer", err: fmt.Errorf("feature report wrote %d bytes, want %d", count, len(payload))}
	}
	stage = "await_ack"
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
			return &diagnosticError{operation: "ack_failure", err: b.ackTimeout(ctx.Err(), attempts, lastRejected)}
		case <-deadline.C:
			return &diagnosticError{operation: "ack_failure", err: b.ackTimeout(context.DeadlineExceeded, attempts, lastRejected)}
		case <-resend:
			resend = nil
			if attempts >= maxAttempts {
				if rejected {
					return &diagnosticError{operation: "ack_failure", err: b.ackTimeout(errors.New("no ACK after final attempt"), attempts, lastRejected)}
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
				return &diagnosticError{operation: "transfer", err: classify(err)}
			}
			if count != len(payload) {
				return &diagnosticError{operation: "transfer", err: fmt.Errorf("feature report wrote %d bytes, want %d", count, len(payload))}
			}
			deadline.Reset(b.readTimeout)
			if silenceDelay > 0 && attempts < maxAttempts {
				armResend(silenceDelay)
			}
		case read := <-incoming:
			if read.err != nil {
				return &diagnosticError{operation: "ack_failure", err: classify(read.err)}
			}
			reports++
			report := read.report
			b.dispatchListenerReport(binding.Path, report)
			more := continueReading(report)
			inventoryDiagnosticf("event=command_report report_id=%#02x index=%d elapsed_ms=%d ack=%t sent=%x received=%x", payloadID(payload), reports, time.Since(started).Milliseconds(), !more, payload, report)
			if !more {
				return nil
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

func supportedFeatureReport(payload []byte) bool {
	return (len(payload) == dpiReportLength && payload[0] == 0x04) ||
		(len(payload) == pollingReportLength && payload[0] == 0x06) ||
		(len(payload) == lightingReportLength && payload[0] == 0x05) ||
		(len(payload) == remapReportLength && payload[0] == 0x08)
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func payloadID(payload []byte) byte {
	if len(payload) == 0 {
		return 0
	}
	return payload[0]
}

func (b *HidrawBackend) serial(candidate Candidate) string {
	data, err := os.ReadFile(filepath.Join(b.sysRoot, "bus/usb/devices", candidate.PortPath, "serial"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}
