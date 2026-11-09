//go:build linux

package hidlinux

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/blak0p/attack-shark-linux/internal/mouse"
	"github.com/blak0p/attack-shark-linux/internal/transport"
)

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

// SendChunksBound uploads model-encoded chunks Spacing apart. Success means only
// the batch's final status, not playback, readback or persistence. Never retries.
func (b *HidrawBackend) SendChunksBound(ctx context.Context, binding mouse.Binding, batch mouse.Batch) error {
	return b.sendChunksBound(ctx, binding, batch, waitMacroChunk)
}

func (b *HidrawBackend) sendChunksBound(ctx context.Context, binding mouse.Binding, batch mouse.Batch, wait macroWait) (err error) {
	if batch.Status == nil {
		return errors.New("batch status classifier is required")
	}
	reports := batch.Clone().Chunks
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
			if err = wait(ctx, batch.Spacing); err != nil {
				return err
			}
		}
	}
	// Pacing is outside the final ACK deadline. No per-chunk ACK is expected.
	return b.awaitBatchStatus(ctx, node, binding, batch.Status)
}

// SendBatchBound owns the setup report and the whole chunk batch under one
// command lock. Only the setup report is resent when the receiver drops it; the
// chunks are never retried and nothing is rolled back.
func (b *HidrawBackend) SendBatchBound(ctx context.Context, binding mouse.Binding, batch mouse.Batch) (mouse.MacroProgress, error) {
	return b.sendBatchBound(ctx, binding, batch, waitMacroChunk)
}

func (b *HidrawBackend) sendBatchBound(ctx context.Context, binding mouse.Binding, batch mouse.Batch, wait macroWait) (progress mouse.MacroProgress, err error) {
	if len(batch.Setup) == 0 || batch.SetupKeepReading == nil || batch.Status == nil {
		return progress, errors.New("batch requires a setup report, setup ACK matcher and status classifier")
	}
	batch = batch.Clone()
	report, chunks := batch.Setup, batch.Chunks
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
	// report08 is a whole settings report, so it gets the same resend as the
	// plain remap command. The report09 chunks below are never resent.
	if _, _, err = b.awaitACKWithResend(ctx, node, binding, report, batch.SetupKeepReading); err != nil {
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
			if err = wait(ctx, batch.Spacing); err != nil {
				return progress, err
			}
		}
	}
	if err = validate(); err != nil {
		return progress, err
	}
	if err = b.awaitBatchStatus(ctx, node, binding, batch.Status); err != nil {
		return progress, err
	}
	progress.Upload = mouse.MacroUploadConfirmed
	return progress, nil
}

func (b *HidrawBackend) awaitBatchStatus(ctx context.Context, node hidrawNode, binding mouse.Binding, classify func([]byte) mouse.BatchStatus) error {
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
		switch classify(report) {
		case mouse.StatusDone:
			return nil
		// A final status before the setup ACK must not authorize uploading.
		case mouse.StatusIgnore:
			continue
		}
		return &diagnosticError{operation: "ack_failure", err: fmt.Errorf("unexpected macro status %x", report)}
	}
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
	if _, err := b.ValidateDescriptor(ctx, transport.Candidate{Path: binding.Path}, statusInputFor(b.registry, candidate.VendorID, candidate.ProductID)); err != nil {
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
