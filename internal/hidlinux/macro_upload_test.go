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
	if len(n.writes) != 3 {
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
