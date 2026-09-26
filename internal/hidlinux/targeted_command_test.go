//go:build linux

package hidlinux

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/blak0p/attack-shark-linux/internal/mouse"
	protocol "github.com/blak0p/attack-shark-linux/internal/protocol/x6"
	"github.com/blak0p/attack-shark-linux/internal/transport"
	"github.com/blak0p/attack-shark-linux/internal/x6"
)

type targetedHidrawInventory struct{ *HidrawBackend }

func (i targetedHidrawInventory) Enumerate(ctx context.Context) ([]transport.Candidate, error) {
	return i.HidrawBackend.Enumerate(ctx, transport.X6Match())
}

type routedHidrawOpener struct {
	mu     sync.Mutex
	nodes  map[string]*commandHidrawNode
	opened []string
}

func (o *routedHidrawOpener) OpenNode(path string) (hidrawNode, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.opened = append(o.opened, path)
	node, ok := o.nodes[path]
	if !ok {
		return nil, os.ErrNotExist
	}
	return node, nil
}

func (o *routedHidrawOpener) paths() []string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return append([]string(nil), o.opened...)
}

func addSecondTargetedHidrawFixture(t *testing.T, root string) {
	t.Helper()
	usb := filepath.Join(root, "sys/bus/usb/devices/1-5")
	writeFixtureFile(t, filepath.Join(usb, "idVendor"), "1d57\n")
	writeFixtureFile(t, filepath.Join(usb, "idProduct"), "fa60\n")
	writeFixtureFile(t, filepath.Join(usb, "serial"), "second-x6\n")
	for number, endpoint := range []string{"0x81", "0x82", "0x83", "0x84"} {
		n := string(rune('0' + number))
		iface := filepath.Join(usb, "1-5:1."+n)
		writeFixtureFile(t, filepath.Join(iface, "bInterfaceNumber"), n+"\n")
		writeFixtureFile(t, filepath.Join(iface, "bInterfaceClass"), "03\n")
		writeFixtureFile(t, filepath.Join(iface, "ep_"+endpoint[2:], "bEndpointAddress"), endpoint+"\n")
	}
	hid := filepath.Join(root, "sys/bus/hid/devices/0003:1D57:FA60.0002")
	writeFixtureFile(t, filepath.Join(hid, "uevent"), "HID_NAME=Attack Shark X6\nHID_ID=0003:00001D57:0000FA60\nHID_PHYS=usb-0000:0d:00.0-5/input2\n")
	writeFixtureFile(t, filepath.Join(hid, "report_descriptor"), string([]byte{0x05, 0x01, 0x09, 0x80, 0x85, 0x04}))
	class := filepath.Join(root, "sys/class/hidraw/hidraw4")
	if err := os.MkdirAll(class, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(hid, filepath.Join(class, "device")); err != nil {
		t.Fatal(err)
	}
}

// A real targeted service and hidraw backend share the fake sysfs inventory;
// only the node opener and the device's replies are replaced.
func TestTargetedPollingRoutesToBoundHidrawNode(t *testing.T) {
	root := fixtureRoot(t)
	addSecondTargetedHidrawFixture(t, root)
	node := newCommandHidrawNode([][]byte{
		{0x03, 0x10, 0x50, 0x00, 0x04}, // unrelated DPI ACK
		{0x03, 0x10, 0x50, 0x00, 0x06}, // polling ACK
	})
	path := filepath.Join(root, "dev/hidraw4")
	defaultNode := newCommandHidrawNode(nil)
	opener := &routedHidrawOpener{nodes: map[string]*commandHidrawNode{filepath.Join(root, "dev/hidraw3"): defaultNode, path: node}}
	backend := &HidrawBackend{sysRoot: filepath.Join(root, "sys"), devRoot: filepath.Join(root, "dev"), readTimeout: 30 * time.Millisecond, opener: opener}
	registry, err := mouse.NewProfileRegistry(x6.NewProfile())
	if err != nil {
		t.Fatal(err)
	}
	service := mouse.NewTargetedService(registry, targetedHidrawInventory{backend}, backend)
	devices, err := service.Refresh(context.Background())
	if err != nil || len(devices) != 2 || !devices[0].Eligible || !devices[1].Eligible {
		t.Fatalf("Refresh() = %v, %v", devices, err)
	}
	if err := service.Select(mouse.DeviceID{VendorID: 0x1d57, ProductID: 0xfa60, Serial: "second-x6"}); err != nil {
		t.Fatal(err)
	}
	binding, ok := service.Selection()
	if !ok || binding.Path != "1:1-5" {
		t.Fatalf("Selection() = %#v, %t", binding, ok)
	}
	if err := service.ApplyOperationBound(context.Background(), binding, x6.NewPollingOperation(), x6.PollingRate500); err != nil {
		t.Fatalf("ApplyOperationBound() = %v", err)
	}
	expected, err := protocol.EncodePollingReport(protocol.PollingRate500)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(node.written(), expected) || len(opener.paths()) != 1 || opener.paths()[0] != path || len(defaultNode.written()) != 0 {
		t.Fatalf("report=%x opened=%v default report=%x; want only selected %s with report %x", node.written(), opener.paths(), defaultNode.written(), path, expected)
	}
	if !node.isClosed() {
		t.Fatal("command node left open")
	}
}

func TestTargetedDPIRoutesToSelectedHidrawNode(t *testing.T) {
	root := fixtureRoot(t)
	addSecondTargetedHidrawFixture(t, root)
	path := filepath.Join(root, "dev/hidraw4")
	node := newCommandHidrawNode([][]byte{
		{0x03, 0x10, 0x50, 0x00, 0x06}, // polling ACK cannot finish DPI
		{0x03, 0x10, 0x50, 0x00, 0x04}, // DPI ACK
	})
	defaultNode := newCommandHidrawNode(nil)
	opener := &routedHidrawOpener{nodes: map[string]*commandHidrawNode{filepath.Join(root, "dev/hidraw3"): defaultNode, path: node}}
	backend := &HidrawBackend{sysRoot: filepath.Join(root, "sys"), devRoot: filepath.Join(root, "dev"), readTimeout: 30 * time.Millisecond, opener: opener}
	registry, err := mouse.NewProfileRegistry(x6.NewProfile())
	if err != nil {
		t.Fatal(err)
	}
	service := mouse.NewTargetedService(registry, targetedHidrawInventory{backend}, backend)
	devices, err := service.Refresh(context.Background())
	if err != nil || len(devices) != 2 || !devices[0].Eligible || !devices[1].Eligible {
		t.Fatalf("Refresh() = %v, %v", devices, err)
	}
	if err := service.Select(mouse.DeviceID{VendorID: 0x1d57, ProductID: 0xfa60, Serial: "second-x6"}); err != nil {
		t.Fatal(err)
	}
	binding, ok := service.Selection()
	if !ok || binding.Path != "1:1-5" {
		t.Fatalf("Selection() = %#v, %t", binding, ok)
	}
	config := x6.DefaultDPIConfig()
	config.DPI[0] = 1600
	config.ActiveStage = 2
	expected, err := x6.EncodeDPIReport(config)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.ApplyOperationBound(context.Background(), binding, x6.NewDPIOperation(), config); err != nil {
		t.Fatalf("ApplyOperationBound() = %v", err)
	}
	if !bytes.Equal(node.written(), expected) || len(opener.paths()) != 1 || opener.paths()[0] != path || len(defaultNode.written()) != 0 {
		t.Fatalf("report=%x opened=%v default report=%x; want only selected %s with report %x", node.written(), opener.paths(), defaultNode.written(), path, expected)
	}
	if !node.isClosed() {
		t.Fatal("command node left open")
	}
	node.mu.Lock()
	reads := node.readCount
	node.mu.Unlock()
	if reads != 2 {
		t.Fatalf("node reads = %d; want polling ACK ignored before DPI ACK", reads)
	}
	invalid := config
	invalid.DPI[0] = 55
	if err := service.ApplyOperationBound(context.Background(), binding, x6.NewDPIOperation(), invalid); err == nil {
		t.Fatal("invalid typed DPI config was accepted")
	}
	if len(opener.paths()) != 1 || !bytes.Equal(node.written(), expected) {
		t.Fatalf("invalid DPI config reached node: opened=%v report=%x", opener.paths(), node.written())
	}
}

func TestTargetedPollingDoesNotAcceptUnrelatedACK(t *testing.T) {
	root := fixtureRoot(t)
	node := newCommandHidrawNode([][]byte{{0x03, 0x10, 0x50, 0x00, 0x04}})
	opener := &countingHidrawOpener{path: filepath.Join(root, "dev/hidraw3"), node: node}
	backend := &HidrawBackend{sysRoot: filepath.Join(root, "sys"), devRoot: filepath.Join(root, "dev"), readTimeout: 20 * time.Millisecond, opener: opener}
	registry, err := mouse.NewProfileRegistry(x6.NewProfile())
	if err != nil {
		t.Fatal(err)
	}
	service := mouse.NewTargetedService(registry, targetedHidrawInventory{backend}, backend)
	if _, err := service.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	binding, ok := service.Selection()
	if !ok {
		t.Fatal("no selected binding")
	}
	err = service.ApplyOperationBound(context.Background(), binding, x6.NewPollingOperation(), x6.PollingRate500)
	if err == nil {
		t.Fatal("unrelated DPI ACK completed polling command")
	}
	node.mu.Lock()
	reads := node.readCount
	node.mu.Unlock()
	if reads != 2 {
		t.Fatalf("node reads = %d; want unrelated ACK consumed then another read", reads)
	}
	if opener.opens() != 1 || len(node.written()) == 0 {
		t.Fatalf("command was not sent to bound node: opens=%d report=%x", opener.opens(), node.written())
	}
}

func TestTargetedPollingFailsClosedBeforeNodeOpen(t *testing.T) {
	for _, tc := range []struct {
		name      string
		mutate    func(*mouse.Binding, string)
		rate      x6.PollingRate
		wantStale bool
	}{
		{name: "invalid rate", rate: x6.PollingRate(42)},
		{name: "stale serial", rate: x6.PollingRate500, wantStale: true, mutate: func(_ *mouse.Binding, root string) {
			writeFixtureFile(t, filepath.Join(root, "sys/bus/usb/devices/1-4/serial"), "replacement\n")
		}},
		{name: "stale binding path", rate: x6.PollingRate500, wantStale: true, mutate: func(b *mouse.Binding, _ string) { b.Path = "1:1-9" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := fixtureRoot(t)
			node := newCommandHidrawNode(nil)
			opener := &countingHidrawOpener{path: filepath.Join(root, "dev/hidraw3"), node: node}
			backend := &HidrawBackend{sysRoot: filepath.Join(root, "sys"), devRoot: filepath.Join(root, "dev"), readTimeout: 20 * time.Millisecond, opener: opener}
			registry, err := mouse.NewProfileRegistry(x6.NewProfile())
			if err != nil {
				t.Fatal(err)
			}
			service := mouse.NewTargetedService(registry, targetedHidrawInventory{backend}, backend)
			if _, err := service.Refresh(context.Background()); err != nil {
				t.Fatal(err)
			}
			binding, ok := service.Selection()
			if !ok {
				t.Fatal("no selected binding")
			}
			if tc.mutate != nil {
				tc.mutate(&binding, root)
			}
			if tc.name == "stale serial" {
				fresh, err := backend.Enumerate(context.Background(), transport.X6Match())
				if err != nil || len(fresh) != 1 || fresh[0].Serial != "replacement" || binding.ID.Serial == fresh[0].Serial {
					t.Fatalf("fresh inventory = %#v, %v; selected binding = %#v; want changed serial", fresh, err, binding)
				}
			}
			err = service.ApplyOperationBound(context.Background(), binding, x6.NewPollingOperation(), tc.rate)
			if err == nil || (tc.wantStale && !errors.Is(err, mouse.ErrStaleBinding)) {
				t.Fatalf("ApplyOperationBound() = %v; want rejection", err)
			}
			if opener.opens() != 0 || len(node.written()) != 0 {
				t.Fatalf("rejected command reached node: opens=%d report=%x", opener.opens(), node.written())
			}
		})
	}
}
