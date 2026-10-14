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
	if len(n.writes) != 3 && !(len(n.writes) == 1 && n.writes[0][0] == 8) && !(len(n.writes) >= 4 && n.writes[0][0] == 8) {
		return 0, errors.New("read before all chunks")
	}
	return n.commandHidrawNode.Read(p)
}

func TestCompositeLocalCapturedPackets(t *testing.T) {
	b, binding, n, _ := macroFixture(t, [][]byte{{3, 0x10, 0x50, 0, 8}, macroACK})
	events := []macros.Event{{Type: macros.Keyboard, KeyCode: "ShiftLeft", Action: macros.Down, DelayMS: 100}, {Type: macros.Keyboard, KeyCode: "KeyA", Action: macros.Down, DelayMS: 100}, {Type: macros.Keyboard, KeyCode: "KeyA", Action: macros.Up, DelayMS: 100}, {Type: macros.Keyboard, KeyCode: "ShiftLeft", Action: macros.Up, DelayMS: 100}}
	items := []mouse.MacroLocalItem{{Button: 6, Events: events, Repeat: 1}}
	p, err := b.sendX6MultiMacroLocalAssignmentBound(context.Background(), binding, x6.DefaultRemapConfig(), items, noMacroWait)
	if err != nil || p.Upload != mouse.MacroUploadConfirmed || len(n.writes) != 4 {
		t.Fatal(p, err, n.writes)
	}
	// Expected captured block assembled independently, not through codec helpers.
	block := make([]byte, 128)
	block[4] = 1
	block[25] = 4
	copy(block[26:], []byte{0x0b, 0xe1, 0x0b, 4, 0x8b, 4, 0x81, 0xe1})
	block[126] = 2
	block[127] = 0xf1
	for i, size := range []int{60, 60, 8} {
		want := make([]byte, 64)
		want[0] = 9
		want[1] = 0x40
		if i == 2 {
			want[1] = 12
		}
		want[2] = 5
		want[3] = byte(i)
		start := i * 60
		copy(want[4:], block[start:start+size])
		if !reflect.DeepEqual(n.writes[i+1], want) {
			t.Fatalf("packet %d: %x want %x", i, n.writes[i+1], want)
		}
	}
}

func TestX6OfficialHIDFDSTenEventReports(t *testing.T) {
	b, binding, node, _ := macroFixture(t, [][]byte{{3, 0x10, 0x50, 0, 8}, macroACK})
	var events []macros.Event
	for i, key := range []string{"KeyF", "KeyD", "KeyS", "KeyF", "KeyD"} {
		events = append(events, macros.Event{Type: macros.Keyboard, KeyCode: key, Action: macros.Down, DelayMS: int64(100 * (i + 1))}, macros.Event{Type: macros.Keyboard, KeyCode: key, Action: macros.Up, DelayMS: int64(20 + 10*i)})
	}
	events[9].DelayMS = 600
	before := append([]macros.Event(nil), events...)
	config := x6.DefaultRemapConfig()
	config.Buttons[3].Action = x6.RemapFire
	p, err := b.sendX6MultiMacroLocalAssignmentBound(context.Background(), binding, config, []mouse.MacroLocalItem{{Button: 7, Events: events, Repeat: 3, Mode: 2}}, noMacroWait)
	if err != nil || p != (mouse.MacroProgress{Assignment: mouse.MacroAssignmentACKConfirmed, Upload: mouse.MacroUploadConfirmed}) || len(node.writes) != 4 || !reflect.DeepEqual(events, before) {
		t.Fatal(p, err, node.writes)
	}
	association, err := x6.EncodeMultiMacroAssignmentReport(config, []uint8{7})
	if err != nil || !reflect.DeepEqual(node.writes[0], association) {
		t.Fatal("ordinary remap overlay lost", err)
	}
	// Literal expected official IDs/order/timing and checksum; not mapper-derived.
	block := make([]byte, 128)
	block[0], block[4], block[25] = 2, 3, 10
	copy(block[26:], []byte{0x0b, 9, 0x83, 9, 0x15, 7, 0x83, 7, 0x1f, 0x16, 0x85, 0x16, 0x29, 9, 0x85, 9, 0x33, 7, 0x81, 7})
	block[126], block[127] = 3, 0xa7
	for i, size := range []int{60, 60, 8} {
		want := make([]byte, 64)
		want[0], want[1], want[2], want[3] = 9, 0x40, 6, byte(i)
		if i == 2 {
			want[1] = 0x0c
		}
		copy(want[4:], block[i*60:i*60+size])
		if !reflect.DeepEqual(node.writes[i+1], want) {
			t.Fatalf("HID report %d: %x want %x", i, node.writes[i+1], want)
		}
	}
}

func TestX6OfficialHIDAliasesBeforeOpen(t *testing.T) {
	key := func(code string, action macros.Action) macros.Event {
		return macros.Event{Type: macros.Keyboard, KeyCode: code, Action: action, DelayMS: 10}
	}
	for _, events := range [][]macros.Event{
		{key("Enter", macros.Down), key("NumpadEnter", macros.Up)},
		{key("Enter", macros.Down), key("NumpadEnter", macros.Down), key("Enter", macros.Up), key("NumpadEnter", macros.Up)},
		{key("Enter", macros.Down), key("Enter", macros.Down), key("Enter", macros.Up)},
		{key("__proto__", macros.Down), key("__proto__", macros.Up)},
	} {
		b, binding, node, _ := macroFixture(t, nil)
		before := append([]macros.Event(nil), events...)
		p, err := b.sendX6MultiMacroLocalAssignmentBound(context.Background(), binding, x6.DefaultRemapConfig(), []mouse.MacroLocalItem{{Button: 7, Events: events, Repeat: 1}}, noMacroWait)
		if err == nil || p != (mouse.MacroProgress{}) || len(node.writes) != 0 || b.opener.(*countingHidrawOpener).count != 0 || !reflect.DeepEqual(events, before) {
			t.Fatal(p, err, node.writes)
		}
	}
	// Sequential use of the inferential aliases remains encodable at raw 0x28.
	b, binding, node, _ := macroFixture(t, [][]byte{{3, 0x10, 0x50, 0, 8}, macroACK})
	events := []macros.Event{key("Enter", macros.Down), key("Enter", macros.Up), key("NumpadEnter", macros.Down), key("NumpadEnter", macros.Up)}
	p, err := b.sendX6MultiMacroLocalAssignmentBound(context.Background(), binding, x6.DefaultRemapConfig(), []mouse.MacroLocalItem{{Button: 7, Events: events, Repeat: 1}}, noMacroWait)
	if err != nil || p.Upload != mouse.MacroUploadConfirmed || !reflect.DeepEqual(node.writes[1][30:38], []byte{1, 0x28, 0x81, 0x28, 1, 0x28, 0x81, 0x28}) {
		t.Fatal(p, err, node.writes)
	}
}

func localUploadItems() []mouse.MacroLocalItem {
	return []mouse.MacroLocalItem{
		{Button: 6, Events: []macros.Event{{Type: macros.MouseLeft, Action: macros.Down}, {Type: macros.MouseLeft, Action: macros.Up}}, Repeat: 1},
		{Button: 7, Events: []macros.Event{{Type: macros.Keyboard, KeyCode: "KeyA", Action: macros.Down, DelayMS: 100}, {Type: macros.Keyboard, KeyCode: "KeyA", Action: macros.Up, DelayMS: 100}}, Repeat: 2, Mode: 1},
	}
}
func TestCompositeLocalAllItemsBeforeIO(t *testing.T) {
	for _, mutation := range []string{"empty", "config", "duplicate", "destination", "unsupported", "unbalanced", "overflow", "oversize", "repeat", "mode", "stale mouse identity"} {
		t.Run(mutation, func(t *testing.T) {
			b, binding, n, _ := macroFixture(t, nil)
			items := localUploadItems()
			config := x6.DefaultRemapConfig()
			switch mutation {
			case "empty":
				items = nil
			case "config":
				config.Buttons[0].Action = "invalid"
			case "duplicate":
				items[1].Button = 6
			case "destination":
				items[1].Button = 8
			case "unsupported":
				items[1].Events[0].KeyCode = "F13"
			case "unbalanced":
				items[1].Events = items[1].Events[:1]
			case "overflow":
				items[1].Events[1].DelayMS = 51200
			case "oversize":
				items[1].Events = nil
				for i := 0; i < 24; i++ {
					items[1].Events = append(items[1].Events, macros.Event{Type: macros.Keyboard, KeyCode: "KeyF", Action: macros.Down}, macros.Event{Type: macros.Keyboard, KeyCode: "KeyF", Action: macros.Up})
				}
			case "repeat":
				items[1].Repeat = 256
			case "mode":
				items[1].Mode = 3
			case "stale mouse identity":
				items[0].Events[0].KeyCode = "KeyA"
			}
			p, err := b.sendX6MultiMacroLocalAssignmentBound(context.Background(), binding, config, items, noMacroWait)
			if err == nil || p != (mouse.MacroProgress{}) || len(n.writes) != 0 || b.opener.(*countingHidrawOpener).count != 0 {
				t.Fatal(p, err, n.writes)
			}
		})
	}
}
func TestCompositeLocalMixedOrderAndPartialProgress(t *testing.T) {
	for at := 0; at <= 7; at++ {
		t.Run(fmt.Sprint(at), func(t *testing.T) {
			b, binding, n, _ := macroFixture(t, [][]byte{{3, 0x10, 0x50, 0, 8}, macroACK})
			n.failAt = at
			n.short = true
			config := x6.DefaultRemapConfig()
			config.Buttons[3].Action = x6.RemapFire
			waits := 0
			p, err := b.sendX6MultiMacroLocalAssignmentBound(context.Background(), binding, config, localUploadItems(), func(ctx context.Context, d time.Duration) error {
				waits++
				if d != x6MacroSpacing || len(n.writes) != waits+1 {
					t.Fatal("pacing/order")
				}
				return nil
			})
			if at > 0 {
				if err == nil || len(n.writes) != at || !strings.Contains(err.Error(), "partial mutation") {
					t.Fatal(p, err)
				}
				if at == 1 {
					if p != (mouse.MacroProgress{Assignment: mouse.MacroAssignmentUnknown}) {
						t.Fatal(p)
					}
				} else if p != (mouse.MacroProgress{Assignment: mouse.MacroAssignmentACKConfirmed, Upload: mouse.MacroUploadPossiblyPartial}) {
					t.Fatal(p)
				}
				return
			}
			if err != nil || len(n.writes) != 7 || waits != 5 || p.Upload != mouse.MacroUploadConfirmed {
				t.Fatal(p, err)
			}
			wantAssociation := []byte{8, 0x3b, 1, 2, 0, 0, 3, 0, 0, 4, 0, 0, 0x0d, 0, 0, 0x12, 0, 5, 0x12, 0, 6, 8, 0, 0, 5, 0, 0, 0x3c, 0, 0, 1, 0, 0, 1, 0, 0, 1, 0, 0, 1, 0, 0, 1, 0, 0, 1, 0, 0, 1, 0, 0, 0x0a, 0, 0, 9, 0, 0, 0, 0xa8}
			if !reflect.DeepEqual(n.writes[0], wantAssociation) {
				t.Fatalf("association: %x want %x", n.writes[0], wantAssociation)
			}
			// First item is the unchanged legacy click; second retains timed order/mode/repeat.
			if !reflect.DeepEqual(n.writes[1][30:34], []byte{1, 0xf1, 0x81, 0xf1}) || !reflect.DeepEqual(n.writes[4][:4], []byte{9, 0x40, 6, 0}) || n.writes[4][4] != 1 || n.writes[4][8] != 2 || !reflect.DeepEqual(n.writes[4][30:34], []byte{0x0b, 4, 0x81, 4}) {
				t.Fatalf("mixed payload mismatch: %x", n.writes)
			}
		})
	}
}
func TestCompositeLocalACKAndCancellationGuards(t *testing.T) {
	for _, ack := range [][]byte{{3, 0x10, 0x50, 0, 9}, {3, 0x10, 0x50, 1, 8}, {3, 0x10, 0x1e, 0, 0xaa}} {
		b, binding, n, _ := macroFixture(t, [][]byte{ack})
		p, err := b.sendX6MultiMacroLocalAssignmentBound(context.Background(), binding, x6.DefaultRemapConfig(), localUploadItems(), noMacroWait)
		if err == nil || len(n.writes) != 1 || p != (mouse.MacroProgress{Assignment: mouse.MacroAssignmentUnknown}) {
			t.Fatal(p, err)
		}
	}
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
				p, err := b.sendX6MultiMacroLocalAssignmentBound(ctx, binding, x6.DefaultRemapConfig(), localUploadItems(), func(ctx context.Context, _ time.Duration) error { return ctx.Err() })
				if err == nil || len(n.writes) != phase || p.Upload == mouse.MacroUploadConfirmed {
					t.Fatal(p, err, len(n.writes))
				}
				if phase == 0 && p != (mouse.MacroProgress{}) {
					t.Fatal(p)
				}
				if phase > 0 && !strings.Contains(err.Error(), "partial mutation") {
					t.Fatal(err)
				}
			})
		}
	}
	for _, ack := range [][]byte{{3, 0x10, 0x50, 1, 9}, {3, 0x10, 0x1e, 0, 0xaa}} {
		b, binding, n, _ := macroFixture(t, [][]byte{{3, 0x10, 0x50, 0, 8}, ack})
		p, err := b.sendX6MultiMacroLocalAssignmentBound(context.Background(), binding, x6.DefaultRemapConfig(), localUploadItems(), noMacroWait)
		if err == nil || len(n.writes) != 7 || p.Upload != mouse.MacroUploadPossiblyPartial {
			t.Fatal(p, err)
		}
	}
}
func TestCompositeLocalQueuedFreezeAndRevalidation(t *testing.T) {
	for _, stale := range []bool{false, true} {
		t.Run(fmt.Sprint(stale), func(t *testing.T) {
			b, binding, n, root := macroFixture(t, [][]byte{{3, 0x10, 0x50, 0, 8}, macroACK})
			items := localUploadItems()
			config := x6.DefaultRemapConfig()
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
				p, err := b.sendX6MultiMacroLocalAssignmentBound(context.Background(), binding, config, items, noMacroWait)
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
			config.Buttons[0].Action = x6.RemapOff
			items[1].Events[0].KeyCode = "KeyB"
			items[1].Repeat = 255
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
			} else {
				if got.err != nil || n.writes[0][3] != 2 || n.writes[4][8] != 2 || n.writes[4][31] != 4 {
					t.Fatal(got, n.writes)
				}
			}
		})
	}
}

func TestCompositeMultiSequenceOrderAndProgress(t *testing.T) {
	b, binding, n, _ := macroFixture(t, [][]byte{{3, 0x10, 0x50, 0, 8}, macroACK})
	cfg := x6.DefaultRemapConfig()
	s1 := macros.X6Sequence{Buttons: []macros.EventType{macros.MouseLeft, macros.MouseRight}, Repeat: 1}
	s2 := macros.X6Sequence{Buttons: []macros.EventType{macros.MouseMiddle, macros.MouseBack}, Repeat: 2}
	items := []mouse.MacroSequenceItem{
		{Button: 6, Sequence: s1},
		{Button: 7, Sequence: s2},
	}
	p, err := b.sendX6MultiMacroSequenceAssignmentBound(context.Background(), binding, cfg, items, noMacroWait)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	report, _ := x6.EncodeMultiMacroAssignmentReport(cfg, []uint8{6, 7})
	chunks1, _ := macros.EncodeX6SequenceUpload(macros.X6SequenceUpload{Destination: 5, Sequence: s1})
	chunks2, _ := macros.EncodeX6SequenceUpload(macros.X6SequenceUpload{Destination: 6, Sequence: s2})
	wantWrites := append([][]byte{report}, chunks1...)
	wantWrites = append(wantWrites, chunks2...)
	if !reflect.DeepEqual(n.writes, wantWrites) {
		t.Fatalf("writes mismatch:\ngot %d writes\nwant %d writes", len(n.writes), len(wantWrites))
	}
	if p.Assignment != mouse.MacroAssignmentACKConfirmed || p.Upload != mouse.MacroUploadConfirmed {
		t.Fatalf("unexpected progress: %+v", p)
	}
}

func TestCompositeMultiSequenceGuards(t *testing.T) {
	cfg := x6.DefaultRemapConfig()
	s1 := macros.X6Sequence{Buttons: []macros.EventType{macros.MouseLeft, macros.MouseRight}, Repeat: 1}
	s2 := macros.X6Sequence{Buttons: []macros.EventType{macros.MouseMiddle, macros.MouseBack}, Repeat: 2}
	items := []mouse.MacroSequenceItem{
		{Button: 6, Sequence: s1},
		{Button: 7, Sequence: s2},
	}
	// Empty items rejected
	b, binding, n, _ := macroFixture(t, nil)
	p, err := b.sendX6MultiMacroSequenceAssignmentBound(context.Background(), binding, cfg, nil, noMacroWait)
	if err == nil || len(n.writes) != 0 || p != (mouse.MacroProgress{}) {
		t.Fatalf("empty items should fail: p=%+v err=%v", p, err)
	}
	// Duplicate buttons rejected
	dup := []mouse.MacroSequenceItem{
		{Button: 6, Sequence: s1},
		{Button: 6, Sequence: s2},
	}
	b, binding, n, _ = macroFixture(t, nil)
	p, err = b.sendX6MultiMacroSequenceAssignmentBound(context.Background(), binding, cfg, dup, noMacroWait)
	if err == nil || len(n.writes) != 0 || p != (mouse.MacroProgress{}) {
		t.Fatalf("duplicate button should fail: p=%+v err=%v", p, err)
	}
	// Invalid button rejected
	badButton := []mouse.MacroSequenceItem{
		{Button: 8, Sequence: s1},
	}
	b, binding, n, _ = macroFixture(t, nil)
	p, err = b.sendX6MultiMacroSequenceAssignmentBound(context.Background(), binding, cfg, badButton, noMacroWait)
	if err == nil || len(n.writes) != 0 || p != (mouse.MacroProgress{}) {
		t.Fatalf("invalid button should fail: p=%+v err=%v", p, err)
	}
	// Partial failure during chunk write
	for at := 1; at <= 7; at++ {
		b, binding, n, _ := macroFixture(t, [][]byte{{3, 0x10, 0x50, 0, 8}, macroACK})
		n.failAt, n.short = at, true
		p, err := b.sendX6MultiMacroSequenceAssignmentBound(context.Background(), binding, cfg, items, noMacroWait)
		if err == nil || len(n.writes) != at || !strings.Contains(err.Error(), "partial mutation") {
			t.Fatalf("failAt=%d: expected partial mutation error, got %v (writes=%d)", at, err, len(n.writes))
		}
		if at == 1 && (p.Assignment != mouse.MacroAssignmentUnknown || p.Upload != mouse.MacroUploadNotStarted) {
			t.Fatalf("failAt=1 unexpected progress: %+v", p)
		}
		if at > 1 && (p.Assignment != mouse.MacroAssignmentACKConfirmed || p.Upload != mouse.MacroUploadPossiblyPartial) {
			t.Fatalf("failAt=%d unexpected progress: %+v", at, p)
		}
	}
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

func TestCompositePlaybackModes(t *testing.T) {
	for _, mode := range []uint8{1, 2, 3, 255} {
		b, binding, n, _ := macroFixture(t, [][]byte{{3, 0x10, 0x50, 0, 8}, macroACK})
		a := x6.MacroAssignment{Config: x6.DefaultRemapConfig(), Button: 7}
		s := macros.X6Sequence{Buttons: []macros.EventType{macros.MouseLeft}, Repeat: 1, Mode: mode}
		p, err := b.sendX6MacroSequenceAssignmentBound(context.Background(), binding, a, s, noMacroWait)
		if mode > 2 {
			if err == nil || len(n.writes) != 0 || p != (mouse.MacroProgress{}) {
				t.Fatal(p, err)
			}
			continue
		}
		if err != nil || len(n.writes) != 4 || p.Upload != mouse.MacroUploadConfirmed {
			t.Fatal(p, err)
		}
		got, err := macros.DecodeX6SequenceUpload(n.writes[1:])
		if err != nil || got.Sequence.Mode != mode || got.Sequence.Repeat != 1 || n.writes[3][1] != 12 {
			t.Fatal(got, err)
		}
	}
}

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
