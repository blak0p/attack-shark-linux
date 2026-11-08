package x6

import (
	"context"
	"errors"
	"testing"

	"github.com/blak0p/attack-shark-linux/internal/transport"
)

type preferenceTransport struct {
	candidates []transport.Candidate
	invalid    string
	reads      []string
	fail       bool
	cancel     context.CancelFunc
}

func (f *preferenceTransport) Enumerate(context.Context, transport.Match) ([]transport.Candidate, error) {
	return f.candidates, nil
}
func (f *preferenceTransport) ValidateDescriptor(_ context.Context, c transport.Candidate, _ transport.InputDescriptor) (transport.InputSource, error) {
	if c.Path == f.invalid {
		return transport.InputSource{}, errors.New("invalid")
	}
	return transport.InputSource{Path: c.Path}, nil
}
func (f *preferenceTransport) ReadInterruptIN(_ context.Context, s transport.InputSource, use func([]byte) bool) error {
	f.reads = append(f.reads, s.Path)
	if f.cancel != nil {
		defer f.cancel()
	}
	if f.fail {
		return errors.New("read failed")
	}
	use([]byte{3, 0x10, 0x40, 1, 5})
	return nil
}

// Passive status reads do not authorize selecting a configuration target.
func TestStatusPreservesFirstValidatedDongleWithTwoDongles(t *testing.T) {
	first := transport.Candidate{Path: "d1", VendorID: 0x1d57, ProductID: 0xfa60, Connection: transport.Dongle}
	second := first
	second.Path = "d2"
	f := &preferenceTransport{candidates: []transport.Candidate{first, second}}
	status, err := NewService(f).Status(context.Background())
	want := Status{Connection: Dongle, BatteryPercent: 50, BatteryAvailable: true}
	if err != nil || status != want || len(f.reads) != 1 || f.reads[0] != "d1" {
		t.Fatalf("Status()=%+v, %v; reads=%v; want %+v, nil; reads=[d1]", status, err, f.reads, want)
	}
}

func TestWiredPreferenceStatusAndListen(t *testing.T) {
	dongle := transport.Candidate{Path: "dongle", VendorID: 0x1d57, ProductID: 0xfa60, Connection: transport.Dongle}
	wired := transport.Candidate{Path: "wired", VendorID: 0x1d57, ProductID: 0xfa61, Connection: transport.Wired}
	for _, tc := range []struct {
		name          string
		candidates    []transport.Candidate
		invalid, want string
	}{
		{"paired", []transport.Candidate{dongle, wired}, "", "wired"},
		{"dongle", []transport.Candidate{dongle}, "", "dongle"},
		{"wired", []transport.Candidate{wired}, "", "wired"},
		{"invalid cable", []transport.Candidate{dongle, wired}, "wired", "dongle"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &preferenceTransport{candidates: tc.candidates, invalid: tc.invalid}
			status, err := NewService(f).Status(context.Background())
			if err != nil || len(f.reads) != 1 || f.reads[0] != tc.want || string(status.Connection) != tc.want {
				t.Fatalf("status=%+v err=%v reads=%v", status, err, f.reads)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			f.reads = nil
			f.cancel = cancel
			var events []StatusEvent
			if err := NewService(f).Listen(ctx, func(e StatusEvent) { events = append(events, e) }); err != nil || len(events) != 1 || string(events[0].Connection) != tc.want || len(f.reads) != 1 || f.reads[0] != tc.want {
				t.Fatalf("listen err=%v events=%v reads=%v", err, events, f.reads)
			}
		})
	}
	f := &preferenceTransport{candidates: []transport.Candidate{dongle, wired}, fail: true}
	if _, err := NewService(f).Status(context.Background()); err == nil || len(f.reads) != 1 || f.reads[0] != "wired" {
		t.Fatalf("failed cable must not fall through: %v %v", err, f.reads)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f.reads = nil
	f.cancel = cancel
	if err := NewService(f).Listen(ctx, func(StatusEvent) { t.Fatal("failed read emitted event") }); err != nil || len(f.reads) != 1 || f.reads[0] != "wired" {
		t.Fatalf("failed listener fell through: %v %v", err, f.reads)
	}
	second := wired
	second.Path = "other cable"
	f = &preferenceTransport{candidates: []transport.Candidate{wired, second, dongle}}
	// Multiple devices prevent automatic mutation selection, not passive reads.
	status, err := NewService(f).Status(context.Background())
	if err != nil || status != (Status{Connection: Dongle, BatteryPercent: 50, BatteryAvailable: true}) || len(f.reads) != 1 || f.reads[0] != "dongle" {
		t.Fatalf("legacy passive status=%+v reads=%v err=%v", status, f.reads, err)
	}
}
