package mouse

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/blak0p/attack-shark-linux/internal/transport"
)

type batchFake struct {
	commandFake
	batches  []Batch
	progress MacroProgress
	err      error
	during   func()
}

func (c *batchFake) SendBatchBound(_ context.Context, _ Binding, batch Batch) (MacroProgress, error) {
	c.batches = append(c.batches, batch)
	if c.during != nil {
		c.during()
	}
	return c.progress, c.err
}

func batchFixture(t *testing.T, command TargetedCommand) (*TargetedService, Binding) {
	t.Helper()
	registry, _ := NewProfileRegistry(targetedProfile{})
	svc := NewTargetedService(registry, inventoryFake{candidates: []transport.Candidate{{Path: "hidraw-1", Serial: "A", VendorID: 0x1d57, ProductID: 0xfa60}}}, command)
	if _, err := svc.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	binding, _ := svc.Selection()
	return svc, binding
}

func sampleBatch() Batch {
	return Batch{Setup: []byte{8, 1}, SetupKeepReading: func([]byte) bool { return false }, Chunks: [][]byte{{9, 1}, {9, 2}}}
}

func TestApplyBatchBoundFreezesBatchAndReportsProgress(t *testing.T) {
	want := MacroProgress{Assignment: MacroAssignmentACKConfirmed, Upload: MacroUploadPossiblyPartial}
	command := &batchFake{progress: want, err: errors.New("short write")}
	svc, binding := batchFixture(t, command)
	batch := sampleBatch()
	command.during = func() { batch.Setup[0], batch.Chunks[1][1] = 0xff, 0xff }
	// The copy is taken before the command runs, so in-flight mutation is invisible.
	p, err := svc.ApplyBatchBound(context.Background(), binding, batch)
	if err != command.err || p != want || len(command.batches) != 1 {
		t.Fatal(p, err, command.batches)
	}
	got := command.batches[0]
	if !reflect.DeepEqual(got.Setup, []byte{8, 1}) || !reflect.DeepEqual(got.Chunks, [][]byte{{9, 1}, {9, 2}}) {
		t.Fatalf("batch not frozen: %x %x", got.Setup, got.Chunks)
	}
}

func TestApplyBatchBoundRejectsBeforeCommand(t *testing.T) {
	command := &batchFake{}
	svc, binding := batchFixture(t, command)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	stale := binding
	stale.Path = "other"
	before, _ := svc.State(binding.ID)
	if _, err := svc.ApplyBatchBound(ctx, binding, sampleBatch()); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := svc.ApplyBatchBound(context.Background(), stale, sampleBatch()); !errors.Is(err, ErrStaleBinding) {
		t.Fatal(err)
	}
	after, _ := svc.State(binding.ID)
	if len(command.batches) != 0 || !reflect.DeepEqual(before, after) {
		t.Fatal(command.batches, before, after)
	}
}

func TestApplyBatchBoundRequiresBatchCapability(t *testing.T) {
	command := &commandFake{}
	svc, binding := batchFixture(t, command)
	if svc.BatchAvailable() {
		t.Fatal("plain command reported batch capability")
	}
	p, err := svc.ApplyBatchBound(context.Background(), binding, sampleBatch())
	if err == nil || err.Error() != "bound macro command unavailable" || p != (MacroProgress{}) || command.calls != 0 {
		t.Fatal(p, err)
	}
	if capable, _ := batchFixture(t, &batchFake{}); !capable.BatchAvailable() {
		t.Fatal("batch command not detected")
	}
}

// Err is the earliest cancellation checkpoint after the batch is frozen.
type queueCheckpointContext struct {
	context.Context
	entered, release chan struct{}
	once             sync.Once
}

func (c *queueCheckpointContext) Err() error {
	c.once.Do(func() { close(c.entered); <-c.release })
	return c.Context.Err()
}

func TestApplyBatchBoundFreezeBeforeQueueAndCancellation(t *testing.T) {
	for _, mutation := range []string{"copy", "cancel", "selection"} {
		t.Run(mutation, func(t *testing.T) {
			command := &batchFake{progress: MacroProgress{Assignment: MacroAssignmentACKConfirmed, Upload: MacroUploadConfirmed}}
			svc, binding := batchFixture(t, command)
			_, state, _, _ := svc.selectedState()
			state.applyMu.Lock()
			base, cancel := context.WithCancel(context.Background())
			defer cancel()
			ctx := &queueCheckpointContext{Context: base, entered: make(chan struct{}), release: make(chan struct{})}
			batch := sampleBatch()
			type result struct {
				p   MacroProgress
				err error
			}
			done := make(chan result, 1)
			go func() {
				p, err := svc.ApplyBatchBound(ctx, binding, batch)
				done <- result{p, err}
			}()
			select {
			case <-ctx.entered:
			case <-time.After(time.Second):
				state.applyMu.Unlock()
				t.Fatal("no prequeue cancellation checkpoint")
			}
			batch.Chunks[1][1] = 0xff
			if mutation == "cancel" {
				cancel()
			}
			if mutation == "selection" {
				svc.mu.Lock()
				svc.selection = nil
				svc.mu.Unlock()
			}
			close(ctx.release)
			state.applyMu.Unlock()
			got := <-done
			if mutation == "copy" {
				if got.err != nil || len(command.batches) != 1 || command.batches[0].Chunks[1][1] != 2 {
					t.Fatal(got, command)
				}
				return
			}
			if got.err == nil || len(command.batches) != 0 || got.p != (MacroProgress{}) {
				t.Fatal(got)
			}
			if mutation == "cancel" && !errors.Is(got.err, context.Canceled) {
				t.Fatal(got.err)
			}
			if mutation == "selection" && !errors.Is(got.err, ErrStaleBinding) {
				t.Fatal(got.err)
			}
		})
	}
}
