//go:build linux

package hidlinux

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/blak0p/attack-shark-linux/internal/macros"
	"github.com/blak0p/attack-shark-linux/internal/mouse"
	"github.com/blak0p/attack-shark-linux/internal/protocol/x6"
)

type macroNode struct {
	*commandHidrawNode
	writes  [][]byte
	failAt  int
	short   bool
	onWrite func(int)
}

func (n *macroNode) SendFeatureReport(r []byte) (int, error) {
	n.writes = append(n.writes, append([]byte(nil), r...))
	if n.onWrite != nil {
		n.onWrite(len(n.writes))
	}
	if len(n.writes) == n.failAt {
		if n.short {
			return len(r) - 1, nil
		}
		return 0, errors.New("write fault")
	}
	return len(r), nil
}
func (n *macroNode) Read(p []byte) (int, error) {
	if len(n.writes) != 3 && !(len(n.writes) == 1 && n.writes[0][0] == 8) && !(len(n.writes) == 4 && n.writes[0][0] == 8) {
		return 0, errors.New("read before all chunks")
	}
	return n.commandHidrawNode.Read(p)
}
func macroFixture(t *testing.T, reports [][]byte) (*HidrawBackend, mouse.Binding, *macroNode, string) {
	t.Helper()
	root := fixtureRoot(t)
	writeFixtureFile(t, filepath.Join(root, "sys/bus/usb/devices/1-4/serial"), "A\n")
	n := &macroNode{commandHidrawNode: newCommandHidrawNode(reports)}
	b := &HidrawBackend{sysRoot: filepath.Join(root, "sys"), devRoot: filepath.Join(root, "dev"), readTimeout: 20 * time.Millisecond, opener: &countingHidrawOpener{path: filepath.Join(root, "dev/hidraw3"), node: n}}
	binding := mouse.Binding{ID: mouse.DeviceID{VendorID: 0x1d57, ProductID: 0xfa60, Serial: "A"}, Path: "1:1-4"}
	return b, binding, n, root
}
func noMacroWait(context.Context, time.Duration) error { return nil }

var macroACK = []byte{3, 0x10, 0x50, 0, 9}
var macroClick = macros.X6Click{Button: macros.MouseLeft, Repeat: 1}

func TestCompositeSequenceOrderAndProgress(t *testing.T) {
	b, binding, n, _ := macroFixture(t, [][]byte{{3, 0x10, 0x50, 0, 8}, macroACK})
	a := x6.MacroAssignment{Config: x6.DefaultRemapConfig(), Button: 7}
	s := macros.X6Sequence{Buttons: []macros.EventType{macros.MouseLeft, macros.MouseRight}, Repeat: 2}
	p, err := b.sendX6MacroSequenceAssignmentBound(context.Background(), binding, a, s, noMacroWait)
	r, _ := x6.EncodeMacroAssignmentReport(a)
	chunks, _ := macros.EncodeX6SequenceUpload(macros.X6SequenceUpload{Destination: 6, Sequence: s})
	if err != nil || !reflect.DeepEqual(n.writes, append([][]byte{r}, chunks...)) || p.Upload != mouse.MacroUploadConfirmed {
		t.Fatal(p, err, n.writes)
	}
}

func TestCompositeSequenceGuards(t *testing.T) {
	a := x6.MacroAssignment{Config: x6.DefaultRemapConfig(), Button: 7}
	s := macros.X6Sequence{Buttons: []macros.EventType{macros.MouseLeft, macros.MouseRight}, Repeat: 1}
	for _, invalid := range []macros.X6Sequence{{}, {Buttons: s.Buttons, Repeat: 256}, {Buttons: []macros.EventType{"unknown"}, Repeat: 1}, {Buttons: append(s.Buttons, macros.MouseLeft), Repeat: 1}} {
		b, binding, n, _ := macroFixture(t, nil)
		p, err := b.sendX6MacroSequenceAssignmentBound(context.Background(), binding, a, invalid, noMacroWait)
		if err == nil || len(n.writes) != 0 || p != (mouse.MacroProgress{}) {
			t.Fatal(p, err)
		}
	}
	for at := 1; at <= 4; at++ {
		b, binding, n, _ := macroFixture(t, [][]byte{{3, 0x10, 0x50, 0, 8}, macroACK})
		n.failAt, n.short = at, true
		p, err := b.sendX6MacroSequenceAssignmentBound(context.Background(), binding, a, s, noMacroWait)
		if err == nil || len(n.writes) != at || !strings.Contains(err.Error(), "partial mutation") {
			t.Fatal(p, err)
		}
		if at == 1 && (p.Assignment != mouse.MacroAssignmentUnknown || p.Upload != mouse.MacroUploadNotStarted) {
			t.Fatal(p)
		}
		if at > 1 && (p.Assignment != mouse.MacroAssignmentACKConfirmed || p.Upload != mouse.MacroUploadPossiblyPartial) {
			t.Fatal(p)
		}
	}
	for _, stale := range []bool{false, true} {
		b, binding, n, root := macroFixture(t, [][]byte{{3, 0x10, 0x50, 0, 8}, macroACK})
		ctx, cancel := context.WithCancel(context.Background())
		n.onWrite = func(i int) {
			if i == 2 {
				if stale {
					writeFixtureFile(t, filepath.Join(root, "sys/bus/usb/devices/1-4/serial"), "B\n")
				} else {
					cancel()
				}
			}
		}
		p, err := b.sendX6MacroSequenceAssignmentBound(ctx, binding, a, s, noMacroWait)
		cancel()
		if err == nil || len(n.writes) != 2 || p.Upload != mouse.MacroUploadPossiblyPartial {
			t.Fatal(p, err)
		}
	}
	for _, ack := range [][]byte{{3, 0x10, 0x50, 0, 9}, {3, 0x10, 0x1e, 0, 0xaa}} {
		b, binding, n, _ := macroFixture(t, [][]byte{ack})
		p, err := b.sendX6MacroSequenceAssignmentBound(context.Background(), binding, a, s, noMacroWait)
		if err == nil || len(n.writes) != 1 || p.Upload != mouse.MacroUploadNotStarted {
			t.Fatal(p, err)
		}
	}
}

func TestCompositeMacroOrderAndProgress(t *testing.T) {
	for button := uint8(1); button <= 7; button++ {
		b, binding, n, _ := macroFixture(t, [][]byte{{3, 0x10, 0x40, 1, 10}, {3, 0x10, 0x50, 0, 8}, macroACK})
		assignment := x6.MacroAssignment{Config: x6.DefaultRemapConfig(), Button: button}
		progress, err := b.sendX6MacroAssignmentBound(context.Background(), binding, assignment, macroClick, noMacroWait)
		report, _ := x6.EncodeMacroAssignmentReport(assignment)
		destination, _ := x6.MacroDestinationForButton(button)
		chunks, _ := macros.EncodeX6Upload(macros.X6Upload{Destination: destination, Click: macroClick})
		want := append([][]byte{report}, chunks...)
		if err != nil || !reflect.DeepEqual(n.writes, want) || progress.Assignment != mouse.MacroAssignmentACKConfirmed || progress.Upload != mouse.MacroUploadConfirmed {
			t.Fatalf("button %d: %+v %v", button, progress, err)
		}
	}
}

func TestCompositeMacroFailures(t *testing.T) {
	for at := 1; at <= 4; at++ {
		for _, short := range []bool{false, true} {
			b, binding, n, _ := macroFixture(t, [][]byte{{3, 0x10, 0x50, 0, 8}, macroACK})
			n.failAt, n.short = at, short
			p, err := b.sendX6MacroAssignmentBound(context.Background(), binding, x6.MacroAssignment{Config: x6.DefaultRemapConfig(), Button: 6}, macroClick, noMacroWait)
			if err == nil || !strings.Contains(err.Error(), "partial mutation") || len(n.writes) != at {
				t.Fatalf("boundary %d: %+v %v", at, p, err)
			}
			if at == 1 && p.Assignment != mouse.MacroAssignmentUnknown {
				t.Fatal(p)
			}
			if at > 1 && (p.Assignment != mouse.MacroAssignmentACKConfirmed || p.Upload != mouse.MacroUploadPossiblyPartial) {
				t.Fatal(p)
			}
		}
	}
	for _, a := range []x6.MacroAssignment{{}, {Config: x6.DefaultRemapConfig(), Button: 8}} {
		if _, err := (&HidrawBackend{}).SendX6MacroAssignmentBound(context.Background(), mouse.Binding{}, a, macroClick); err == nil {
			t.Fatal("invalid assignment")
		}
	}
}

func TestCompositeMacroFinalStatusAndWaitFailure(t *testing.T) {
	assignment := x6.MacroAssignment{Config: x6.DefaultRemapConfig(), Button: 6}
	for _, status := range [][]byte{{3, 0x10, 0x1e, 0, 0xaa}, {3, 0x10, 0x50, 1, 9}, nil} {
		b, binding, n, _ := macroFixture(t, [][]byte{{3, 0x10, 0x50, 0, 8}, status})
		p, err := b.sendX6MacroAssignmentBound(context.Background(), binding, assignment, macroClick, noMacroWait)
		if err == nil || len(n.writes) != 4 || p.Assignment != mouse.MacroAssignmentACKConfirmed || p.Upload != mouse.MacroUploadPossiblyPartial {
			t.Fatal(p, err)
		}
	}
	for boundary := 1; boundary <= 2; boundary++ {
		b, binding, n, _ := macroFixture(t, [][]byte{{3, 0x10, 0x50, 0, 8}})
		waits := 0
		p, err := b.sendX6MacroAssignmentBound(context.Background(), binding, assignment, macroClick, func(context.Context, time.Duration) error {
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
	if _, err := b.sendX6MacroAssignmentBound(context.Background(), binding, assignment, macroClick, noMacroWait); err != nil || seen != 4 {
		t.Fatal(seen, err)
	}
}

func TestCompositeMacroWaitOwnership(t *testing.T) {
	b, binding, n, _ := macroFixture(t, [][]byte{{3, 0x10, 0x50, 0, 8}, macroACK})
	waits := 0
	_, err := b.sendX6MacroAssignmentBound(context.Background(), binding, x6.MacroAssignment{Config: x6.DefaultRemapConfig(), Button: 6}, macroClick, func(ctx context.Context, d time.Duration) error {
		waits++
		if d != x6MacroSpacing || len(n.writes) != waits+1 {
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

func TestCompositeMacroPhaseGuards(t *testing.T) {
	for _, phase := range []int{1, 2, 3, 4} {
		for _, stale := range []bool{false, true} {
			b, binding, n, root := macroFixture(t, [][]byte{{3, 0x10, 0x50, 0, 8}, macroACK})
			ctx, cancel := context.WithCancel(context.Background())
			n.onWrite = func(i int) {
				if i == phase {
					if stale {
						writeFixtureFile(t, filepath.Join(root, "sys/bus/usb/devices/1-4/serial"), "B\n")
					} else {
						cancel()
					}
				}
			}
			p, err := b.sendX6MacroAssignmentBound(ctx, binding, x6.MacroAssignment{Config: x6.DefaultRemapConfig(), Button: 7}, macroClick, noMacroWait)
			cancel()
			if err == nil || len(n.writes) != phase || !strings.Contains(err.Error(), "partial mutation") {
				t.Fatalf("phase %d stale %t: %+v %v", phase, stale, p, err)
			}
			unlock, e := b.beginCommand(context.Background())
			if e != nil {
				t.Fatal(e)
			}
			unlock()
		}
	}
	for _, ack := range [][]byte{{3, 0x10, 0x1e, 0, 0xaa}, {3, 0x10, 0x50, 1, 8}, {3, 0x10, 0x50, 0, 9}} {
		b, binding, n, _ := macroFixture(t, [][]byte{ack})
		p, err := b.sendX6MacroAssignmentBound(context.Background(), binding, x6.MacroAssignment{Config: x6.DefaultRemapConfig(), Button: 6}, macroClick, noMacroWait)
		if err == nil || len(n.writes) != 1 || p.Assignment != mouse.MacroAssignmentUnknown || p.Upload != mouse.MacroUploadNotStarted {
			t.Fatalf("%+v %v", p, err)
		}
	}
	b, binding, n, _ := macroFixture(t, nil)
	n.readWait = make(chan struct{})
	if _, err := b.sendX6MacroAssignmentBound(context.Background(), binding, x6.MacroAssignment{Config: x6.DefaultRemapConfig(), Button: 6}, macroClick, noMacroWait); !IsErrorKind(err, Timeout) {
		t.Fatal(err)
	}
}

func TestMacroInvalidBeforeIO(t *testing.T) {
	for _, click := range []macros.X6Click{{}, {Button: macros.MouseLeft, Repeat: 256}, {Button: macros.MouseRight, Repeat: 0}} {
		if err := (&HidrawBackend{}).SendX6MacroBound(context.Background(), mouse.Binding{}, click); err == nil {
			t.Fatal("invalid template accepted")
		}
	}
}
func TestMacroBatchAndDispatch(t *testing.T) {
	for _, button := range []macros.EventType{macros.MouseLeft, macros.MouseRight} {
		for _, repeat := range []int{1, 255} {
			t.Run(fmt.Sprintf("%s/%d", button, repeat), func(t *testing.T) {
				// Captured idle heartbeat: docs/protocol-x6.md, status report.
				reports := [][]byte{{3, 0x10, 0x40, 1, 0x0a}, {3, 0x10, 0x50, 0, 4}, macroACK}
				b, binding, n, _ := macroFixture(t, reports)
				var seen [][]byte
				b.activePath = binding.Path
				b.listener = func(r []byte) bool { seen = append(seen, r); return true }
				waits := 0
				click := macros.X6Click{Button: button, Repeat: repeat}
				err := b.sendX6MacroBound(context.Background(), binding, click, func(ctx context.Context, d time.Duration) error {
					waits++
					if d != 1200*time.Millisecond || len(n.writes) != waits {
						t.Fatal("incorrect pacing")
					}
					return nil
				})
				want, _ := macros.EncodeX6Reports(click)
				if err != nil || !reflect.DeepEqual(n.writes, want) || !reflect.DeepEqual(seen, reports) || waits != 2 || !n.isClosed() {
					t.Fatalf("batch failed: %v", err)
				}
				if supportedFeatureReport(want[0]) {
					t.Fatal("generic report09 admitted")
				}
			})
		}
	}
}
func TestMacroWriteFailuresNoRetry(t *testing.T) {
	for i := 1; i <= 3; i++ {
		for _, short := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/%t", i, short), func(t *testing.T) {
				b, binding, n, _ := macroFixture(t, nil)
				n.failAt, n.short = i, short
				err := b.sendX6MacroBound(context.Background(), binding, macroClick, noMacroWait)
				if err == nil || !strings.Contains(err.Error(), "possible device partial mutation") || len(n.writes) != i || !n.isClosed() {
					t.Fatalf("failure not contained: %v", err)
				}
			})
		}
	}
}
func TestMacroStatusFailsClosed(t *testing.T) {
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
		err := b.sendX6MacroBound(context.Background(), binding, macroClick, noMacroWait)
		if err == nil || !strings.Contains(err.Error(), fmt.Sprintf("status %x", r)) || !strings.Contains(err.Error(), "possible device partial mutation; no rollback or retry") || len(n.writes) != 3 || !n.isClosed() {
			t.Fatalf("status %x: %v", r, err)
		}
	}
	b, binding, n, _ := macroFixture(t, nil)
	n.readWait = make(chan struct{})
	if err := b.sendX6MacroBound(context.Background(), binding, macroClick, noMacroWait); !IsErrorKind(err, Timeout) || !n.isClosed() {
		t.Fatalf("missing ACK: %v", err)
	}
}
func TestMacroCancellation(t *testing.T) {
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
		err := b.sendX6MacroBound(ctx, binding, macroClick, func(ctx context.Context, _ time.Duration) error { return ctx.Err() })
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
func TestMacroQueuedRevalidationAndSerialization(t *testing.T) {
	for _, stale := range []bool{false, true} {
		b, binding, n, root := macroFixture(t, [][]byte{macroACK})
		release, err := b.acquireIO(context.Background(), false)
		if err != nil {
			t.Fatal(err)
		}
		done := make(chan error, 1)
		go func() { done <- b.sendX6MacroBound(context.Background(), binding, macroClick, noMacroWait) }()
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
func TestMacroStaleBeforeOpen(t *testing.T) {
	b, binding, n, _ := macroFixture(t, nil)
	binding.ID.Serial = "B"
	if err := b.sendX6MacroBound(context.Background(), binding, macroClick, noMacroWait); !errors.Is(err, mouse.ErrStaleBinding) || len(n.writes) != 0 {
		t.Fatalf("stale: %v", err)
	}
}
