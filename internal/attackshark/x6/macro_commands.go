package x6

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/blak0p/attack-shark-linux/internal/attackshark/x6/macrocodec"
	protocol "github.com/blak0p/attack-shark-linux/internal/attackshark/x6/protocol"
	"github.com/blak0p/attack-shark-linux/internal/macros"
	"github.com/blak0p/attack-shark-linux/internal/mouse"
)

// MacroSpacing is conservative engineering policy, not a validated firmware
// minimum. Captured inter-chunk intervals were 811.726–1190.951ms.
const MacroSpacing = 1200 * time.Millisecond

// MacroSequenceItem associates one button with an offline sequence.
type MacroSequenceItem struct {
	Button   uint8
	Sequence macrocodec.X6Sequence
}

// MacroLocalItem retains exact local source events; no unchecked raw blocks are accepted.
type MacroLocalItem struct {
	Button uint8
	Events []macros.Event
	Repeat int
	Mode   uint8
}

// ChunkCommand is the generic transport seam for a chunk-only upload.
type ChunkCommand interface {
	SendChunksBound(context.Context, mouse.Binding, mouse.Batch) error
}

// macroStatus classifies reports after the last report09 chunk. A final09 status
// before the report08 ACK must not authorize uploading, so only 031050 00 09 ends it.
func macroStatus(r []byte) mouse.BatchStatus {
	if len(r) == 5 && r[0] == 3 && r[1] == 0x10 && r[2] == 0x50 && r[3] == 0 && r[4] == 9 {
		return mouse.StatusDone
	}
	if len(r) != 5 || r[0] != 3 || r[1] != 0x10 {
		return mouse.StatusUnexpected
	}
	// Heartbeat byte 3 is not an ACK field: captured 031040010a is valid.
	// This matches the existing X6 battery decoder's five-byte status shape.
	if r[2] == 0x40 {
		return mouse.StatusIgnore
	}
	if r[2] != 0x50 || r[3] != 0 {
		return mouse.StatusUnexpected
	}
	switch r[4] {
	case 4, 5, 6, 8:
		return mouse.StatusIgnore
	}
	return mouse.StatusUnexpected
}

func assignmentBatch(setup []byte, chunks [][]byte) mouse.Batch {
	return mouse.Batch{
		Setup:            setup,
		SetupKeepReading: func(r []byte) bool { return !protocol.MatchesRemapACK(r) },
		Chunks:           chunks,
		Spacing:          MacroSpacing,
		Status:           macroStatus,
	}
}

// MacroClickBatch encodes the strict report09 template without an assignment.
func MacroClickBatch(click macrocodec.X6Click) (mouse.Batch, error) {
	reports, err := macrocodec.EncodeX6Reports(click)
	if err != nil {
		return mouse.Batch{}, err
	}
	return mouse.Batch{Chunks: reports, Spacing: MacroSpacing, Status: macroStatus}, nil
}

// SendMacroClickBound uploads the strict report09 template. Success means only
// normal upload status, not playback, readback or persistence. Never retries.
func SendMacroClickBound(ctx context.Context, command ChunkCommand, binding mouse.Binding, click macrocodec.X6Click) error {
	batch, err := MacroClickBatch(click)
	if err != nil {
		return err
	}
	return command.SendChunksBound(ctx, binding, batch)
}

// MacroAssignmentBatch encodes report08 plus the report09 click upload.
func MacroAssignmentBatch(assignment protocol.MacroAssignment, click macrocodec.X6Click) (mouse.Batch, error) {
	report, err := protocol.EncodeMacroAssignmentReport(assignment)
	if err != nil {
		return mouse.Batch{}, err
	}
	destination, err := protocol.MacroDestinationForButton(assignment.Button)
	if err != nil {
		return mouse.Batch{}, err
	}
	chunks, err := macrocodec.EncodeX6Upload(macrocodec.X6Upload{Destination: destination, Click: click})
	if err != nil {
		return mouse.Batch{}, err
	}
	return assignmentBatch(report, chunks), nil
}

// MacroSequenceAssignmentBatch admits only the bounded offline sequence codec.
func MacroSequenceAssignmentBatch(assignment protocol.MacroAssignment, sequence macrocodec.X6Sequence) (mouse.Batch, error) {
	report, err := protocol.EncodeMacroAssignmentReport(assignment)
	if err != nil {
		return mouse.Batch{}, err
	}
	destination, err := protocol.MacroDestinationForButton(assignment.Button)
	if err != nil {
		return mouse.Batch{}, err
	}
	chunks, err := macrocodec.EncodeX6SequenceUpload(macrocodec.X6SequenceUpload{Destination: destination, Sequence: sequence})
	if err != nil {
		return mouse.Batch{}, err
	}
	return assignmentBatch(report, chunks), nil
}

// MultiMacroSequenceBatch admits multiple macro sequences on distinct buttons.
func MultiMacroSequenceBatch(config protocol.RemapConfig, items []MacroSequenceItem) (mouse.Batch, error) {
	if len(items) == 0 {
		return mouse.Batch{}, errors.New("no macro sequence items")
	}
	buttons := make([]uint8, len(items))
	seen := make(map[uint8]bool, len(items))
	var chunks [][]byte
	for i, item := range items {
		if seen[item.Button] {
			return mouse.Batch{}, fmt.Errorf("duplicate macro button %d", item.Button)
		}
		seen[item.Button] = true
		buttons[i] = item.Button
		destination, err := protocol.MacroDestinationForButton(item.Button)
		if err != nil {
			return mouse.Batch{}, err
		}
		encoded, err := macrocodec.EncodeX6SequenceUpload(macrocodec.X6SequenceUpload{Destination: destination, Sequence: item.Sequence})
		if err != nil {
			return mouse.Batch{}, err
		}
		chunks = append(chunks, encoded...)
	}
	report, err := protocol.EncodeMultiMacroAssignmentReport(config, buttons)
	if err != nil {
		return mouse.Batch{}, err
	}
	return assignmentBatch(report, chunks), nil
}

// MultiMacroLocalBatch admits exact balanced local events in one report08 plus
// ordered report09 transaction. Confirmation describes transport only.
func MultiMacroLocalBatch(config protocol.RemapConfig, items []MacroLocalItem) (mouse.Batch, error) {
	if len(items) == 0 {
		return mouse.Batch{}, errors.New("no macro local items")
	}
	buttons := make([]uint8, len(items))
	seen := make(map[uint8]bool, len(items))
	var chunks [][]byte
	for i, item := range items {
		if seen[item.Button] {
			return mouse.Batch{}, fmt.Errorf("duplicate macro button %d", item.Button)
		}
		seen[item.Button] = true
		buttons[i] = item.Button
		destination, err := protocol.MacroDestinationForButton(item.Button)
		if err != nil {
			return mouse.Batch{}, err
		}
		encoded, err := macrocodec.EncodeX6LocalUpload(destination, item.Events, item.Repeat, item.Mode)
		if err != nil {
			return mouse.Batch{}, err
		}
		chunks = append(chunks, encoded...)
	}
	report, err := protocol.EncodeMultiMacroAssignmentReport(config, buttons)
	if err != nil {
		return mouse.Batch{}, err
	}
	return assignmentBatch(report, chunks), nil
}

// The Apply helpers admit and encode every payload before the service queues, so
// nothing reaches transport unless the whole assignment is valid.

func ApplyMacroAssignmentBound(ctx context.Context, inventory *mouse.TargetedService, binding mouse.Binding, assignment protocol.MacroAssignment, click macrocodec.X6Click) (mouse.MacroProgress, error) {
	batch, err := MacroAssignmentBatch(assignment, click)
	if err != nil {
		return mouse.MacroProgress{}, err
	}
	return inventory.ApplyBatchBound(ctx, binding, batch)
}

func ApplyMacroSequenceAssignmentBound(ctx context.Context, inventory *mouse.TargetedService, binding mouse.Binding, assignment protocol.MacroAssignment, sequence macrocodec.X6Sequence) (mouse.MacroProgress, error) {
	batch, err := MacroSequenceAssignmentBatch(assignment, sequence)
	if err != nil {
		return mouse.MacroProgress{}, err
	}
	return inventory.ApplyBatchBound(ctx, binding, batch)
}

func ApplyMultiMacroSequenceAssignmentBound(ctx context.Context, inventory *mouse.TargetedService, binding mouse.Binding, config protocol.RemapConfig, items []MacroSequenceItem) (mouse.MacroProgress, error) {
	batch, err := MultiMacroSequenceBatch(config, items)
	if err != nil {
		return mouse.MacroProgress{}, err
	}
	return inventory.ApplyBatchBound(ctx, binding, batch)
}

func ApplyMultiMacroLocalAssignmentBound(ctx context.Context, inventory *mouse.TargetedService, binding mouse.Binding, config protocol.RemapConfig, items []MacroLocalItem) (mouse.MacroProgress, error) {
	batch, err := MultiMacroLocalBatch(config, items)
	if err != nil {
		return mouse.MacroProgress{}, err
	}
	return inventory.ApplyBatchBound(ctx, binding, batch)
}
