//go:build linux

package hidlinux

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/blak0p/attack-shark-linux/internal/macros"
	"github.com/blak0p/attack-shark-linux/internal/mouse"
	"github.com/blak0p/attack-shark-linux/internal/protocol/x6"
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
	return b.awaitMacroStatus(ctx, node, binding, 9)
}

// SendX6MacroAssignmentBound owns report08 and the whole report09 batch under
// one command lock. It never retries or rolls back a possibly mutated device.
func (b *HidrawBackend) SendX6MacroAssignmentBound(ctx context.Context, binding mouse.Binding, assignment x6.MacroAssignment, click macros.X6Click) (mouse.MacroProgress, error) {
	return b.sendX6MacroAssignmentBound(ctx, binding, assignment, click, waitMacroChunk)
}

func (b *HidrawBackend) sendX6MacroAssignmentBound(ctx context.Context, binding mouse.Binding, assignment x6.MacroAssignment, click macros.X6Click, wait macroWait) (progress mouse.MacroProgress, err error) {
	report, err := x6.EncodeMacroAssignmentReport(assignment)
	if err != nil {
		return progress, err
	}
	destination, err := x6.MacroDestinationForButton(assignment.Button)
	if err != nil {
		return progress, err
	}
	chunks, err := macros.EncodeX6Upload(macros.X6Upload{Destination: destination, Click: click})
	if err != nil {
		return progress, err
	}
	return b.sendX6MacroAssignmentReportsBound(ctx, binding, report, chunks, wait)
}

// SendX6MacroSequenceAssignmentBound admits only the bounded offline sequence
// codec and retains the composite command's ownership and progress semantics.
func (b *HidrawBackend) SendX6MacroSequenceAssignmentBound(ctx context.Context, binding mouse.Binding, assignment x6.MacroAssignment, sequence macros.X6Sequence) (mouse.MacroProgress, error) {
	return b.sendX6MacroSequenceAssignmentBound(ctx, binding, assignment, sequence, waitMacroChunk)
}

func (b *HidrawBackend) sendX6MacroSequenceAssignmentBound(ctx context.Context, binding mouse.Binding, assignment x6.MacroAssignment, sequence macros.X6Sequence, wait macroWait) (mouse.MacroProgress, error) {
	report, err := x6.EncodeMacroAssignmentReport(assignment)
	if err != nil {
		return mouse.MacroProgress{}, err
	}
	destination, err := x6.MacroDestinationForButton(assignment.Button)
	if err != nil {
		return mouse.MacroProgress{}, err
	}
	chunks, err := macros.EncodeX6SequenceUpload(macros.X6SequenceUpload{Destination: destination, Sequence: sequence})
	if err != nil {
		return mouse.MacroProgress{}, err
	}
	return b.sendX6MacroAssignmentReportsBound(ctx, binding, report, chunks, wait)
}

// SendX6MultiMacroSequenceAssignmentBound admits multiple macro sequences on distinct buttons.
func (b *HidrawBackend) SendX6MultiMacroSequenceAssignmentBound(ctx context.Context, binding mouse.Binding, config x6.RemapConfig, items []mouse.MacroSequenceItem) (mouse.MacroProgress, error) {
	return b.sendX6MultiMacroSequenceAssignmentBound(ctx, binding, config, items, waitMacroChunk)
}

func (b *HidrawBackend) sendX6MultiMacroSequenceAssignmentBound(ctx context.Context, binding mouse.Binding, config x6.RemapConfig, items []mouse.MacroSequenceItem, wait macroWait) (mouse.MacroProgress, error) {
	if len(items) == 0 {
		return mouse.MacroProgress{}, errors.New("no macro sequence items")
	}
	buttons := make([]uint8, len(items))
	seen := make(map[uint8]bool, len(items))
	for i, item := range items {
		if seen[item.Button] {
			return mouse.MacroProgress{}, fmt.Errorf("duplicate macro button %d", item.Button)
		}
		seen[item.Button] = true
		buttons[i] = item.Button
	}
	report, err := x6.EncodeMultiMacroAssignmentReport(config, buttons)
	if err != nil {
		return mouse.MacroProgress{}, err
	}
	var allChunks [][]byte
	for _, item := range items {
		destination, err := x6.MacroDestinationForButton(item.Button)
		if err != nil {
			return mouse.MacroProgress{}, err
		}
		chunks, err := macros.EncodeX6SequenceUpload(macros.X6SequenceUpload{Destination: destination, Sequence: item.Sequence})
		if err != nil {
			return mouse.MacroProgress{}, err
		}
		allChunks = append(allChunks, chunks...)
	}
	return b.sendX6MacroAssignmentReportsBound(ctx, binding, report, allChunks, wait)
}

func (b *HidrawBackend) sendX6MacroAssignmentReportsBound(ctx context.Context, binding mouse.Binding, report []byte, chunks [][]byte, wait macroWait) (progress mouse.MacroProgress, err error) {
	if err = ctx.Err(); err != nil {
		return progress, err
	}
	release, err := b.beginCommand(ctx)
	if err != nil {
		return progress, err
	}
	defer release()
	path, err := b.macroBindingPath(ctx, binding)
	if err != nil {
		return progress, err
	}
	node, err := b.opener.OpenNode(path)
	if err != nil {
		return progress, &diagnosticError{operation: "transfer", err: classify(err)}
	}
	defer node.Close()
	defer func() {
		if err != nil && progress.Assignment != mouse.MacroAssignmentNotStarted {
			err = fmt.Errorf("macro assignment: possible device partial mutation; no rollback or retry: %w", err)
		}
	}()
	validate := func() error {
		current, e := b.macroBindingPath(ctx, binding)
		if e != nil {
			return e
		}
		if current != path {
			return mouse.ErrStaleBinding
		}
		return ctx.Err()
	}
	write := func(payload []byte) error {
		count, e := node.SendFeatureReport(payload)
		if e != nil {
			return &diagnosticError{operation: "transfer", err: classify(e)}
		}
		if count != len(payload) {
			return fmt.Errorf("feature report wrote %d bytes, want %d", count, len(payload))
		}
		return nil
	}
	if err = validate(); err != nil {
		return progress, err
	}
	progress.Assignment = mouse.MacroAssignmentUnknown
	if err = write(report); err != nil {
		return progress, err
	}
	if err = b.awaitMacroStatus(ctx, node, binding, 8); err != nil {
		return progress, err
	}
	progress.Assignment = mouse.MacroAssignmentACKConfirmed
	for i, chunk := range chunks {
		if err = validate(); err != nil {
			return progress, err
		}
		progress.Upload = mouse.MacroUploadPossiblyPartial
		if err = write(chunk); err != nil {
			return progress, err
		}
		if i < len(chunks)-1 {
			if err = wait(ctx, x6MacroSpacing); err != nil {
				return progress, err
			}
		}
	}
	if err = validate(); err != nil {
		return progress, err
	}
	if err = b.awaitMacroStatus(ctx, node, binding, 9); err != nil {
		return progress, err
	}
	progress.Upload = mouse.MacroUploadConfirmed
	return progress, nil
}

func (b *HidrawBackend) awaitMacroStatus(ctx context.Context, node hidrawNode, binding mouse.Binding, id byte) error {
	bounded, cancel := context.WithTimeout(ctx, b.readTimeout)
	defer cancel()
	buffer := make([]byte, 64)
	for {
		if err := bounded.Err(); err != nil {
			return err
		}
		count, err := b.readNode(bounded, node, buffer)
		if err != nil {
			return &diagnosticError{operation: "ack_failure", err: err}
		}
		if count < 0 || count > len(buffer) {
			return fmt.Errorf("invalid macro status length %d", count)
		}
		report := append([]byte(nil), buffer[:count]...)
		b.dispatchListenerReport(binding.Path, report)
		if bytes.Equal(report, []byte{3, 0x10, 0x50, 0, id}) {
			return nil
		}
		// A final09 status before assignment ACK must not authorize uploading.
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
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return path, nil
}
