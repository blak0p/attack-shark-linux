//go:build linux

package hidlinux

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/blak0p/attack-shark-linux/internal/macros"
	"github.com/blak0p/attack-shark-linux/internal/mouse"
	"github.com/blak0p/attack-shark-linux/internal/transport"
)

// x6MacroSpacing is conservative engineering policy, not a validated firmware
// minimum. Captured inter-chunk intervals were 811.726–1190.951ms.
const x6MacroSpacing = 1200 * time.Millisecond

type macroWait func(context.Context, time.Duration) error

func waitMacroChunk(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// SendX6MacroBound uploads the strict report09 template. Success means only
// normal upload status, not playback, readback or persistence. Never retries.
func (b *HidrawBackend) SendX6MacroBound(ctx context.Context, binding mouse.Binding, click macros.X6Click) error {
	return b.sendX6MacroBound(ctx, binding, click, waitMacroChunk)
}

func (b *HidrawBackend) sendX6MacroBound(ctx context.Context, binding mouse.Binding, click macros.X6Click, wait macroWait) (err error) {
	reports, err := macros.EncodeX6Reports(click)
	if err != nil {
		return err
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	release, err := b.beginCommand(ctx)
	if err != nil {
		return err
	}
	defer release()
	// Resolve only after ownership: queued targets may have changed identity.
	path, err := b.macroBindingPath(ctx, binding)
	if err != nil {
		return err
	}
	node, err := b.opener.OpenNode(path)
	if err != nil {
		return &diagnosticError{operation: "transfer", err: classify(err)}
	}
	defer node.Close()
	attempted := false
	defer func() {
		if err != nil && attempted {
			err = fmt.Errorf("macro upload: possible device partial mutation; no rollback or retry: %w", err)
		}
	}()
	for i, report := range reports {
		if err = ctx.Err(); err != nil {
			return err
		}
		attempted = true
		count, writeErr := node.SendFeatureReport(report)
		if writeErr != nil {
			return &diagnosticError{operation: "transfer", err: classify(writeErr)}
		}
		if count != len(report) {
			return &diagnosticError{operation: "transfer", err: fmt.Errorf("macro chunk %d wrote %d bytes, want %d", i, count, len(report))}
		}
		if i < len(reports)-1 {
			if err = wait(ctx, x6MacroSpacing); err != nil {
				return err
			}
		}
	}
	// Pacing is outside the final ACK deadline. No per-chunk ACK is expected.
	bounded, cancel := context.WithTimeout(ctx, b.readTimeout)
	defer cancel()
	buffer := make([]byte, 64)
	for {
		if err = bounded.Err(); err != nil {
			return err
		}
		count, readErr := b.readNode(bounded, node, buffer)
		if readErr != nil {
			return &diagnosticError{operation: "ack_failure", err: readErr}
		}
		if count < 0 || count > len(buffer) {
			return fmt.Errorf("invalid macro status length %d", count)
		}
		report := append([]byte(nil), buffer[:count]...)
		b.dispatchListenerReport(binding.Path, report)
		if bytes.Equal(report, []byte{3, 0x10, 0x50, 0, 9}) {
			return nil
		}
		if knownMacroInterleave(report) {
			continue
		}
		return &diagnosticError{operation: "ack_failure", err: fmt.Errorf("unexpected macro status %x", report)}
	}
}

func knownMacroInterleave(r []byte) bool {
	if len(r) != 5 || r[0] != 3 || r[1] != 0x10 {
		return false
	}
	// Heartbeat byte 3 is not an ACK field: captured 031040010a is valid.
	// This matches the existing X6 battery decoder's five-byte status shape.
	if r[2] == 0x40 {
		return true
	}
	if r[2] != 0x50 || r[3] != 0 {
		return false
	}
	switch r[4] {
	case 4, 5, 6, 8:
		return true
	}
	return false
}

func (b *HidrawBackend) macroBindingPath(ctx context.Context, binding mouse.Binding) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if _, err := b.Enumerate(ctx, transport.Match{VendorID: binding.ID.VendorID, ProductID: binding.ID.ProductID}); err != nil {
		return "", mouse.ErrStaleBinding
	}
	b.mu.Lock()
	candidate, ok := b.sources[binding.Path]
	b.mu.Unlock()
	serialMatches := b.serial(candidate) == binding.ID.Serial
	if binding.SessionOnly {
		serialMatches = b.serial(candidate) == ""
	}
	if !ok || candidateKey(candidate) != binding.Path || candidate.VendorID != binding.ID.VendorID || candidate.ProductID != binding.ID.ProductID || !serialMatches {
		return "", mouse.ErrStaleBinding
	}
	if _, err := b.ValidateDescriptor(ctx, transport.Candidate{Path: binding.Path}, transport.InputDescriptor{InterfaceNumber: hidInterface, UsagePage: 1, Usage: 0x80, EndpointAddress: statusEndpoint}); err != nil {
		return "", mouse.ErrStaleBinding
	}
	path, err := b.hidrawPath(candidate)
	if err != nil {
		return "", mouse.ErrStaleBinding
	}
	return path, nil
}
