//go:build linux

package hidlinux

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/blak0p/attack-shark-linux/internal/mouse"
)

type macroNode struct {
	*commandHidrawNode
	// writesMu guards writes: the command's reader goroutine calls Read while
	// a resend may be writing.
	writesMu sync.Mutex
	writes   [][]byte
	failAt   int
	short    bool
	onWrite  func(int)
}

func (n *macroNode) SendFeatureReport(r []byte) (int, error) {
	n.writesMu.Lock()
	n.writes = append(n.writes, append([]byte(nil), r...))
	count := len(n.writes)
	n.writesMu.Unlock()
	if n.onWrite != nil {
		n.onWrite(count)
	}
	if count == n.failAt {
		if n.short {
			return len(r) - 1, nil
		}
		return 0, errors.New("write fault")
	}
	return len(r), nil
}
func (n *macroNode) Read(p []byte) (int, error) {
	n.writesMu.Lock()
	ready := len(n.writes) == 3 || (len(n.writes) == 1 && n.writes[0][0] == 8) || (len(n.writes) >= 4 && n.writes[0][0] == 8)
	n.writesMu.Unlock()
	if !ready {
		return 0, errors.New("read before all chunks")
	}
	return n.commandHidrawNode.Read(p)
}

// fixtureSpacing mimics a model's inter-chunk pacing policy.
const fixtureSpacing = 1200 * time.Millisecond

// fixtureBatch mimics a model batch: a 59-byte setup report (ID 8) followed by
// 64-byte chunks (ID 9). The transport must treat every byte as opaque.
func fixtureBatch(chunks int) mouse.Batch {
	batch := fixtureChunks(chunks)
	batch.Setup = fixturePayload(8, 59)
	batch.SetupKeepReading = func(r []byte) bool { return !fixtureACK(8, r) }
	return batch
}

func fixtureChunks(chunks int) mouse.Batch {
	batch := mouse.Batch{Spacing: fixtureSpacing, Status: fixtureStatus}
	for i := 0; i < chunks; i++ {
		chunk := fixturePayload(9, 64)
		chunk[1], chunk[3] = 0x40, byte(i)
		if i == chunks-1 {
			chunk[1] = 0x0c
		}
		batch.Chunks = append(batch.Chunks, chunk)
	}
	return batch
}

// fixtureStatus classifies the final status like a model would: 03105000 09 ends
// the batch, heartbeats and other settings ACKs are skipped, anything else fails.
func fixtureStatus(r []byte) mouse.BatchStatus {
	if fixtureACK(9, r) {
		return mouse.StatusDone
	}
	if len(r) != 5 || r[0] != 3 || r[1] != 0x10 {
		return mouse.StatusUnexpected
	}
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

func macroFixture(t *testing.T, reports [][]byte) (*HidrawBackend, mouse.Binding, *macroNode, string) {
	t.Helper()
	root := fixtureRoot(t)
	writeFixtureFile(t, filepath.Join(root, "sys/bus/usb/devices/1-4/serial"), "A\n")
	n := &macroNode{commandHidrawNode: newCommandHidrawNode(reports)}
	b := &HidrawBackend{registry: fixtureRegistry(t), sysRoot: filepath.Join(root, "sys"), devRoot: filepath.Join(root, "dev"), readTimeout: 20 * time.Millisecond, opener: &countingHidrawOpener{path: filepath.Join(root, "dev/hidraw3"), node: n}}
	binding := mouse.Binding{ID: mouse.DeviceID{VendorID: 0x1d57, ProductID: 0xfa60, Serial: "A"}, Path: "1:1-4"}
	return b, binding, n, root
}
func noMacroWait(context.Context, time.Duration) error { return nil }

// onlyReport08 reports whether something was written and every write was the
// report08 assignment (the first write or its resends), so no report09 chunk
// went out before the assignment ACK.
func onlyReport08(writes [][]byte) bool {
	if len(writes) == 0 {
		return false
	}
	for _, w := range writes {
		if !reflect.DeepEqual(w, writes[0]) || w[0] != 8 {
			return false
		}
	}
	return true
}

var macroACK = []byte{3, 0x10, 0x50, 0, 9}
func TestBatchOrderSpacingAndProgress(t *testing.T) {
	b, binding, n, _ := macroFixture(t, [][]byte{{3, 0x10, 0x50, 0, 8}, macroACK})
	batch := fixtureBatch(6)
	waits := 0
	p, err := b.sendBatchBound(context.Background(), binding, batch, func(ctx context.Context, d time.Duration) error {
		waits++
		if d != fixtureSpacing || len(n.writes) != waits+1 {
			t.Fatal("pacing/order")
		}
		return nil
	})
	want := append([][]byte{batch.Setup}, batch.Chunks...)
	if err != nil || waits != 5 || !reflect.DeepEqual(n.writes, want) || p != (mouse.MacroProgress{Assignment: mouse.MacroAssignmentACKConfirmed, Upload: mouse.MacroUploadConfirmed}) {
		t.Fatal(p, err, n.writes)
	}
}

func TestBatchPartialProgressAtEveryWrite(t *testing.T) {
	for at := 1; at <= 7; at++ {
		for _, short := range []bool{false, true} {
			t.Run(fmt.Sprint(at, short), func(t *testing.T) {
				b, binding, n, _ := macroFixture(t, [][]byte{{3, 0x10, 0x50, 0, 8}, macroACK})
				n.failAt, n.short = at, short
				p, err := b.sendBatchBound(context.Background(), binding, fixtureBatch(6), noMacroWait)
				if err == nil || len(n.writes) != at || !strings.Contains(err.Error(), "partial mutation") {
					t.Fatal(p, err)
				}
				if at == 1 && p != (mouse.MacroProgress{Assignment: mouse.MacroAssignmentUnknown}) {
					t.Fatal(p)
				}
				if at > 1 && p != (mouse.MacroProgress{Assignment: mouse.MacroAssignmentACKConfirmed, Upload: mouse.MacroUploadPossiblyPartial}) {
					t.Fatal(p)
				}
			})
		}
	}
}

func TestBatchRejectsInvalidBatchBeforeIO(t *testing.T) {
	noSetup, noMatcher, noStatus := fixtureBatch(2), fixtureBatch(2), fixtureBatch(2)
	noSetup.Setup = nil
	noMatcher.SetupKeepReading = nil
	noStatus.Status = nil
	for name, batch := range map[string]mouse.Batch{"no setup": noSetup, "no ACK matcher": noMatcher, "no status": noStatus} {
		t.Run(name, func(t *testing.T) {
			b, binding, n, _ := macroFixture(t, nil)
			p, err := b.sendBatchBound(context.Background(), binding, batch, noMacroWait)
			if err == nil || p != (mouse.MacroProgress{}) || len(n.writes) != 0 || b.opener.(*countingHidrawOpener).count != 0 {
				t.Fatal(p, err, n.writes)
			}
		})
	}
	if _, err := (&HidrawBackend{}).SendBatchBound(context.Background(), mouse.Binding{}, mouse.Batch{}); err == nil {
		t.Fatal("empty batch accepted")
	}
}

func TestBatchSetupACKGuards(t *testing.T) {
	for _, ack := range [][]byte{{3, 0x10, 0x50, 0, 9}, {3, 0x10, 0x50, 1, 8}, {3, 0x10, 0x1e, 0, 0xaa}} {
		b, binding, n, _ := macroFixture(t, [][]byte{ack})
		p, err := b.sendBatchBound(context.Background(), binding, fixtureBatch(6), noMacroWait)
		if err == nil || !onlyReport08(n.writes) || p != (mouse.MacroProgress{Assignment: mouse.MacroAssignmentUnknown}) {
			t.Fatal(p, err)
		}
	}
	for _, ack := range [][]byte{{3, 0x10, 0x50, 1, 9}, {3, 0x10, 0x1e, 0, 0xaa}} {
		b, binding, n, _ := macroFixture(t, [][]byte{{3, 0x10, 0x50, 0, 8}, ack})
		p, err := b.sendBatchBound(context.Background(), binding, fixtureBatch(6), noMacroWait)
		if err == nil || len(n.writes) != 7 || p.Upload != mouse.MacroUploadPossiblyPartial {
			t.Fatal(p, err)
		}
	}
	// A bad ACK before any chunk keeps the upload from starting at all.
	for _, ack := range [][]byte{{3, 0x10, 0x50, 0, 9}, {3, 0x10, 0x1e, 0, 0xaa}} {
		b, binding, n, _ := macroFixture(t, [][]byte{ack})
		p, err := b.sendBatchBound(context.Background(), binding, fixtureBatch(2), noMacroWait)
		if err == nil || !onlyReport08(n.writes) || p.Upload != mouse.MacroUploadNotStarted {
			t.Fatal(p, err)
		}
	}
}

func TestBatchCancellationAndStalenessAtEveryPhase(t *testing.T) {
	for _, stale := range []bool{false, true} {
		for phase := 0; phase <= 7; phase++ {
			t.Run(fmt.Sprintf("%t/%d", stale, phase), func(t *testing.T) {
				b, binding, n, root := macroFixture(t, [][]byte{{3, 0x10, 0x50, 0, 8}, macroACK})
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				mutate := func() {
					if stale {
						writeFixtureFile(t, filepath.Join(root, "sys/bus/usb/devices/1-4/serial"), "B\n")
					} else {
						cancel()
					}
				}
				if phase == 0 {
					mutate()
				}
				n.onWrite = func(i int) {
					if i == phase {
						mutate()
					}
				}
				p, err := b.sendBatchBound(ctx, binding, fixtureBatch(6), func(ctx context.Context, _ time.Duration) error { return ctx.Err() })
				if err == nil || len(n.writes) != phase || p.Upload == mouse.MacroUploadConfirmed {
					t.Fatal(p, err, len(n.writes))
				}
				if phase == 0 && p != (mouse.MacroProgress{}) {
					t.Fatal(p)
				}
				if phase > 0 && !strings.Contains(err.Error(), "partial mutation") {
					t.Fatal(err)
				}
				unlock, e := b.beginCommand(context.Background())
				if e != nil {
					t.Fatal(e)
				}
				unlock()
			})
		}
	}
}

func TestBatchQueuedFreezeAndRevalidation(t *testing.T) {
	for _, stale := range []bool{false, true} {
		t.Run(fmt.Sprint(stale), func(t *testing.T) {
			b, binding, n, root := macroFixture(t, [][]byte{{3, 0x10, 0x50, 0, 8}, macroACK})
			batch := fixtureBatch(6)
			original := mouse.Batch.Clone(batch)
			release, err := b.acquireIO(context.Background(), false)
			if err != nil {
				t.Fatal(err)
			}
			type result struct {
				p   mouse.MacroProgress
				err error
			}
			done := make(chan result, 1)
			go func() {
				p, err := b.sendBatchBound(context.Background(), binding, batch, noMacroWait)
				done <- result{p, err}
			}()
			deadline := time.Now().Add(time.Second)
			for {
				b.ioMu.Lock()
				pending := b.commandPending
				b.ioMu.Unlock()
				if pending {
					break
				}
				if time.Now().After(deadline) {
					release()
					t.Fatal("command not queued")
				}
				time.Sleep(time.Millisecond)
			}
			batch.Setup[3] = 0xff
			batch.Chunks[2][8] = 0xff
			if stale {
				writeFixtureFile(t, filepath.Join(root, "sys/bus/usb/devices/1-4/serial"), "B\n")
			}
			if len(n.writes) != 0 {
				t.Fatal("queued I/O")
			}
			release()
			got := <-done
			if stale {
				if !errors.Is(got.err, mouse.ErrStaleBinding) || len(n.writes) != 0 || got.p != (mouse.MacroProgress{}) {
					t.Fatal(got)
				}
			} else if want := append([][]byte{original.Setup}, original.Chunks...); got.err != nil || !reflect.DeepEqual(n.writes, want) {
				t.Fatal(got, n.writes)
			}
		})
	}
}

func TestBatchFinalStatusAndWaitFailure(t *testing.T) {
	for _, status := range [][]byte{{3, 0x10, 0x1e, 0, 0xaa}, {3, 0x10, 0x50, 1, 9}, nil} {
		b, binding, n, _ := macroFixture(t, [][]byte{{3, 0x10, 0x50, 0, 8}, status})
		p, err := b.sendBatchBound(context.Background(), binding, fixtureBatch(3), noMacroWait)
		if err == nil || len(n.writes) != 4 || p.Assignment != mouse.MacroAssignmentACKConfirmed || p.Upload != mouse.MacroUploadPossiblyPartial {
			t.Fatal(p, err)
		}
	}
	for boundary := 1; boundary <= 2; boundary++ {
		b, binding, n, _ := macroFixture(t, [][]byte{{3, 0x10, 0x50, 0, 8}})
		waits := 0
		p, err := b.sendBatchBound(context.Background(), binding, fixtureBatch(3), func(context.Context, time.Duration) error {
			waits++
			if waits == boundary {
				return context.Canceled
			}
			return nil
		})
		if !errors.Is(err, context.Canceled) || len(n.writes) != boundary+1 || p.Assignment != mouse.MacroAssignmentACKConfirmed || p.Upload != mouse.MacroUploadPossiblyPartial {
			t.Fatal(p, err)
		}
	}
	b, binding, _, _ := macroFixture(t, [][]byte{{3, 0x10, 0x40, 1, 10}, {3, 0x10, 0x50, 0, 8}, {3, 0x10, 0x40, 1, 10}, macroACK})
	seen := 0
	b.activePath = binding.Path
	b.listener = func([]byte) bool { seen++; return true }
	if _, err := b.sendBatchBound(context.Background(), binding, fixtureBatch(3), noMacroWait); err != nil || seen != 4 {
		t.Fatal(seen, err)
	}
	b, binding, n, _ := macroFixture(t, nil)
	n.readWait = make(chan struct{})
	if _, err := b.sendBatchBound(context.Background(), binding, fixtureBatch(3), noMacroWait); !IsErrorKind(err, Timeout) {
		t.Fatal(err)
	}
}

func TestBatchWaitOwnership(t *testing.T) {
	b, binding, n, _ := macroFixture(t, [][]byte{{3, 0x10, 0x50, 0, 8}, macroACK})
	waits := 0
	_, err := b.sendBatchBound(context.Background(), binding, fixtureBatch(3), func(ctx context.Context, d time.Duration) error {
		waits++
		if d != fixtureSpacing || len(n.writes) != waits+1 {
			t.Fatal("pacing/order")
		}
		bounded, cancel := context.WithTimeout(ctx, time.Millisecond)
		defer cancel()
		release, e := b.beginCommand(bounded)
		if e == nil {
			release()
			t.Fatal("interleaved command")
		}
		return nil
	})
	if err != nil || waits != 2 {
		t.Fatal(err, waits)
	}
}

func TestChunksInvalidBeforeIO(t *testing.T) {
	noStatus := fixtureChunks(3)
	noStatus.Status = nil
	b, binding, n, _ := macroFixture(t, nil)
	if err := b.sendChunksBound(context.Background(), binding, noStatus, noMacroWait); err == nil || len(n.writes) != 0 || b.opener.(*countingHidrawOpener).count != 0 {
		t.Fatal("batch without status classifier accepted", err)
	}
	if err := (&HidrawBackend{}).SendChunksBound(context.Background(), mouse.Binding{}, mouse.Batch{}); err == nil {
		t.Fatal("empty batch accepted")
	}
}
func TestChunksBatchAndDispatch(t *testing.T) {
	// Captured idle heartbeat: docs/protocol-x6.md, status report.
	reports := [][]byte{{3, 0x10, 0x40, 1, 0x0a}, {3, 0x10, 0x50, 0, 4}, macroACK}
	b, binding, n, _ := macroFixture(t, reports)
	var seen [][]byte
	b.activePath = binding.Path
	b.listener = func(r []byte) bool { seen = append(seen, r); return true }
	waits := 0
	batch := fixtureChunks(3)
	err := b.sendChunksBound(context.Background(), binding, batch, func(ctx context.Context, d time.Duration) error {
		waits++
		if d != fixtureSpacing || len(n.writes) != waits {
			t.Fatal("incorrect pacing")
		}
		return nil
	})
	if err != nil || !reflect.DeepEqual(n.writes, batch.Chunks) || !reflect.DeepEqual(seen, reports) || waits != 2 || !n.isClosed() {
		t.Fatalf("batch failed: %v", err)
	}
	if b.supportedFeatureReport(binding, batch.Chunks[0]) {
		t.Fatal("generic chunk report admitted")
	}
}
func TestChunksWriteFailuresNoRetry(t *testing.T) {
	for i := 1; i <= 3; i++ {
		for _, short := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/%t", i, short), func(t *testing.T) {
				b, binding, n, _ := macroFixture(t, nil)
				n.failAt, n.short = i, short
				err := b.sendChunksBound(context.Background(), binding, fixtureChunks(3), noMacroWait)
				if err == nil || !strings.Contains(err.Error(), "possible device partial mutation") || len(n.writes) != i || !n.isClosed() {
					t.Fatalf("failure not contained: %v", err)
				}
			})
		}
	}
}
func TestChunksStatusFailsClosed(t *testing.T) {
	for _, r := range [][]byte{
		{3, 0x10, 0x1e, 0, 0xaa},
		{3, 0x10, 0x40, 1},          // truncated heartbeat
		{3, 0x10, 0x40, 1, 0x0a, 0}, // extended heartbeat
		{2, 0x10, 0x40, 1, 0x0a},    // wrong report ID
		{3, 0x11, 0x40, 1, 0x0a},    // wrong sub-status
		{3, 0x10, 0x50, 1, 4},       // ACK requires byte 3 == 0
		{3, 0x10, 0x50, 1, 9},       // malformed final ACK
		{3, 0x10, 0x50, 0},
		{3, 0x10, 0x50, 0, 9, 0},
		{3, 0x10, 0x50, 0, 7}, nil,
	} {
		b, binding, n, _ := macroFixture(t, [][]byte{r})
		err := b.sendChunksBound(context.Background(), binding, fixtureChunks(3), noMacroWait)
		if err == nil || !strings.Contains(err.Error(), fmt.Sprintf("status %x", r)) || !strings.Contains(err.Error(), "possible device partial mutation; no rollback or retry") || len(n.writes) != 3 || !n.isClosed() {
			t.Fatalf("status %x: %v", r, err)
		}
	}
	b, binding, n, _ := macroFixture(t, nil)
	n.readWait = make(chan struct{})
	if err := b.sendChunksBound(context.Background(), binding, fixtureChunks(3), noMacroWait); !IsErrorKind(err, Timeout) || !n.isClosed() {
		t.Fatalf("missing ACK: %v", err)
	}
}
func TestChunksCancellation(t *testing.T) {
	for _, after := range []int{0, 1, 2, 3} {
		b, binding, n, _ := macroFixture(t, [][]byte{macroACK})
		ctx, cancel := context.WithCancel(context.Background())
		if after == 0 {
			cancel()
		}
		n.onWrite = func(i int) {
			if i == after {
				cancel()
			}
		}
		err := b.sendChunksBound(ctx, binding, fixtureChunks(3), func(ctx context.Context, _ time.Duration) error { return ctx.Err() })
		cancel()
		if !errors.Is(err, context.Canceled) || len(n.writes) != after {
			t.Fatalf("cancel %d: %v writes %d", after, err, len(n.writes))
		}
		if after > 0 && (!n.isClosed() || !strings.Contains(err.Error(), "partial mutation")) {
			t.Fatal("lost mutation warning or close")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if !errors.Is(waitMacroChunk(ctx, time.Hour), context.Canceled) {
		t.Fatal("pacing not cancellable")
	}
}
func TestChunksQueuedRevalidationAndSerialization(t *testing.T) {
	for _, stale := range []bool{false, true} {
		b, binding, n, root := macroFixture(t, [][]byte{macroACK})
		release, err := b.acquireIO(context.Background(), false)
		if err != nil {
			t.Fatal(err)
		}
		done := make(chan error, 1)
		go func() { done <- b.sendChunksBound(context.Background(), binding, fixtureChunks(3), noMacroWait) }()
		deadline := time.After(time.Second)
		for {
			b.ioMu.Lock()
			pending := b.commandPending
			b.ioMu.Unlock()
			if pending {
				break
			}
			select {
			case <-deadline:
				t.Fatal("not queued")
			case <-time.After(time.Millisecond):
			}
		}
		if stale {
			writeFixtureFile(t, filepath.Join(root, "sys/bus/usb/devices/1-4/serial"), "B\n")
		}
		if len(n.writes) != 0 {
			t.Fatal("overlapping write")
		}
		release()
		err = <-done
		if stale {
			if !errors.Is(err, mouse.ErrStaleBinding) || len(n.writes) != 0 {
				t.Fatalf("queued stale: %v", err)
			}
		} else if err != nil || len(n.writes) != 3 {
			t.Fatalf("queued upload: %v", err)
		}
		unlock, err := b.beginCommand(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		unlock()
	}
}
func TestChunksStaleBeforeOpen(t *testing.T) {
	b, binding, n, _ := macroFixture(t, nil)
	binding.ID.Serial = "B"
	if err := b.sendChunksBound(context.Background(), binding, fixtureChunks(3), noMacroWait); !errors.Is(err, mouse.ErrStaleBinding) || len(n.writes) != 0 {
		t.Fatalf("stale: %v", err)
	}
}
