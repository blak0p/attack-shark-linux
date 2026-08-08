//go:build linux

package hidlinux

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/gousb"
)

type gousbContext interface {
	Close() error
	OpenDevices(func(*gousb.DeviceDesc) bool) ([]*gousb.Device, error)
}

// GousbBackend provides the production Linux implementation of the adapter seams.
// Constructing it only creates a libusb context; device I/O is operation-driven.
type GousbBackend struct {
	context  gousbContext
	close    sync.Once
	closeErr error
}

func NewGousbBackend() *GousbBackend {
	return newGousbBackend(gousb.NewContext())
}

func newGousbBackend(context gousbContext) *GousbBackend {
	return &GousbBackend{context: context}
}

// NewGousbAdapter composes the one backend into the shared status and command adapter.
func NewGousbAdapter(backend *GousbBackend) *Adapter {
	return NewStatusAdapter(backend, backend, backend)
}

func (b *GousbBackend) Close() error {
	b.close.Do(func() { b.closeErr = b.context.Close() })
	return b.closeErr
}

func (b *GousbBackend) Enumerate(ctx context.Context) ([]Candidate, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	devices, err := b.context.OpenDevices(func(desc *gousb.DeviceDesc) bool {
		return uint16(desc.Vendor) == x6VendorID && uint16(desc.Product) == x6ProductID
	})
	candidates := make([]Candidate, 0, len(devices))
	for _, device := range devices {
		candidates = append(candidates, candidateFromGousb(device.Desc))
		_ = device.Close()
	}
	return candidates, err
}

func (b *GousbBackend) Open(ctx context.Context, candidate Candidate) (Device, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	devices, err := b.context.OpenDevices(func(desc *gousb.DeviceDesc) bool {
		return sameGousbCandidate(desc, candidate)
	})
	if err != nil {
		for _, device := range devices {
			_ = device.Close()
		}
		return nil, err
	}
	if len(devices) != 1 {
		for _, device := range devices {
			_ = device.Close()
		}
		return nil, ErrDeviceDisconnected
	}
	if err := devices[0].SetAutoDetach(true); err != nil {
		_ = devices[0].Close()
		return nil, err
	}
	return &gousbDevice{device: devices[0]}, nil
}

func (b *GousbBackend) ReportDescriptor(ctx context.Context, candidate Candidate) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	dirs, err := filepath.Glob("/sys/bus/hid/devices/*")
	if err != nil {
		return nil, err
	}
	// Primary: match HID devices through their uevent attributes. This works on
	// kernels and containers where the legacy "device" symlink is not created.
	for _, dir := range dirs {
		data, err := os.ReadFile(filepath.Join(dir, "uevent"))
		if err != nil || !hidUEventMatches(data, candidate) {
			continue
		}
		descriptor, err := os.ReadFile(filepath.Join(dir, "report_descriptor"))
		if err == nil {
			return descriptor, nil
		}
	}
	// Fallback: legacy kernels expose a "device" symlink pointing at the
	// physical USB path; resolve it and match on the port path.
	needle := "/" + candidate.PortPath + ":"
	for _, dir := range dirs {
		resolved, err := filepath.EvalSymlinks(filepath.Join(dir, "device"))
		if err != nil || !strings.Contains(resolved, needle) {
			continue
		}
		return os.ReadFile(filepath.Join(dir, "report_descriptor"))
	}
	return nil, fmt.Errorf("HID report descriptor for %s: %w", candidate.PortPath, os.ErrNotExist)
}

// hidUEventMatches reports whether a HID device uevent belongs to the candidate
// on the adapter's HID interface. The dongle exposes several HID interfaces
// (keyboard, consumer, vendor); only the interface claimed by the adapter
// carries the System Control usage 1/0x80, so matching by VID/PID alone is not
// enough.
func hidUEventMatches(data []byte, candidate Candidate) bool {
	fields := make(map[string]string)
	for _, line := range strings.Split(string(data), "\n") {
		key, value, ok := strings.Cut(line, "=")
		if ok {
			fields[key] = value
		}
	}
	hidID, ok := fields["HID_ID"]
	if !ok {
		return false
	}
	vid, pid, ok := parseHIDID(hidID)
	if !ok || vid != candidate.VendorID || pid != candidate.ProductID {
		return false
	}
	physPort := portFromHIDPhys(fields["HID_PHYS"])
	candidatePort := portFromPortPath(candidate.PortPath)
	if physPort == "" || candidatePort == "" || physPort != candidatePort {
		return false
	}
	// The adapter claims HID interface 2; require that exact interface number.
	physInterface := interfaceFromHIDPhys(fields["HID_PHYS"])
	return physInterface == hidInterface
}

// interfaceFromHIDPhys extracts the input number from a HID_PHYS uevent value
// like "usb-0000:0d:00.0-4/input2" (2).
func interfaceFromHIDPhys(value string) int {
	slash := strings.LastIndexByte(value, '/')
	if slash < 0 {
		return -1
	}
	rest := value[slash+1:]
	if !strings.HasPrefix(rest, "input") {
		return -1
	}
	n, err := strconv.Atoi(strings.TrimPrefix(rest, "input"))
	if err != nil {
		return -1
	}
	return n
}

// parseHIDID parses a HID_ID uevent value like "0003:00001D57:0000FA60"
// into vendor and product IDs.
func parseHIDID(value string) (vid, pid uint16, ok bool) {
	parts := strings.Split(value, ":")
	if len(parts) != 3 {
		return 0, 0, false
	}
	parsedVID, err := strconv.ParseUint(parts[1], 16, 32)
	if err != nil {
		return 0, 0, false
	}
	parsedPID, err := strconv.ParseUint(parts[2], 16, 32)
	if err != nil {
		return 0, 0, false
	}
	return uint16(parsedVID), uint16(parsedPID), true
}

// portFromHIDPhys extracts the physical port from a HID_PHYS uevent value like
// "usb-0000:0d:00.0-4/input2" (port "4") or "usb-0000:0d:00.0-6.4/input3" ("6.4").
func portFromHIDPhys(value string) string {
	if value == "" {
		return ""
	}
	if slash := strings.LastIndexByte(value, '/'); slash >= 0 {
		value = value[:slash]
	}
	dash := strings.LastIndexByte(value, '-')
	if dash < 0 {
		return ""
	}
	return value[dash+1:]
}

// portFromPortPath extracts the port from a candidate port path like "1-4" (port "4")
// or "1-6.4" (port "6.4").
func portFromPortPath(portPath string) string {
	dash := strings.IndexByte(portPath, '-')
	if dash < 0 {
		return portPath
	}
	return portPath[dash+1:]
}

type gousbDevice struct{ device *gousb.Device }

func (d *gousbDevice) OpenConfiguration(context.Context) (Configuration, error) {
	for number, descriptor := range d.device.Desc.Configs {
		for _, intf := range descriptor.Interfaces {
			for _, setting := range intf.AltSettings {
				if setting.Number == hidInterface && setting.Alternate == 0 {
					config, err := d.device.Config(number)
					if err != nil {
						return nil, err
					}
					return &gousbConfiguration{device: d.device, config: config}, nil
				}
			}
		}
	}
	return nil, ErrCandidateMismatch
}

func (d *gousbDevice) Close() error { return d.device.Close() }

type gousbConfiguration struct {
	device *gousb.Device
	config *gousb.Config
}

func (c *gousbConfiguration) Claim(_ context.Context, number, alternate int) (Claim, error) {
	intf, err := c.config.Interface(number, alternate)
	if err != nil {
		return nil, err
	}
	return &gousbClaim{device: c.device, intf: intf}, nil
}

func (c *gousbConfiguration) Close() error { return c.config.Close() }

type gousbClaim struct {
	device *gousb.Device
	intf   *gousb.Interface
}

func (c *gousbClaim) ControlTransfer(ctx context.Context, requestType, request uint8, value, index uint16, payload []byte) (int, error) {
	if len(payload) != dpiReportLength {
		return 0, fmt.Errorf("DPI SET_REPORT length = %d, want %d", len(payload), dpiReportLength)
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	c.device.ControlTimeout = remainingTimeout(ctx)
	return c.device.Control(requestType, request, value, index, payload)
}

func (c *gousbClaim) ReadInterruptIN(ctx context.Context, endpoint uint8, use func([]byte) bool) error {
	if endpoint != statusEndpoint {
		return ErrCandidateMismatch
	}
	in, err := c.intf.InEndpoint(int(endpoint & 0x0f))
	if err != nil {
		return err
	}
	buffer := make([]byte, 64)
	for {
		count, err := in.ReadContext(ctx, buffer)
		if err != nil {
			return err
		}
		if !use(buffer[:count]) {
			return nil
		}
	}
}

func (c *gousbClaim) Close() error {
	c.intf.Close()
	return nil
}

func candidateFromGousb(desc *gousb.DeviceDesc) Candidate {
	interfaces := make([]InterfaceDescriptor, 0)
	for _, config := range desc.Configs {
		for _, intf := range config.Interfaces {
			for _, setting := range intf.AltSettings {
				endpoints := make([]uint8, 0, len(setting.Endpoints))
				for address := range setting.Endpoints {
					endpoints = append(endpoints, uint8(address))
				}
				interfaces = append(interfaces, InterfaceDescriptor{Number: setting.Number, AlternateSetting: setting.Alternate, Class: uint8(setting.Class), Endpoints: endpoints})
			}
		}
	}
	path := make([]string, len(desc.Path))
	for index, port := range desc.Path {
		path[index] = fmt.Sprint(port)
	}
	return Candidate{VendorID: uint16(desc.Vendor), ProductID: uint16(desc.Product), Bus: uint8(desc.Bus), PortPath: fmt.Sprintf("%d-%s", desc.Bus, strings.Join(path, ".")), Interfaces: interfaces}
}

func sameGousbCandidate(desc *gousb.DeviceDesc, candidate Candidate) bool {
	actual := candidateFromGousb(desc)
	return actual.VendorID == candidate.VendorID &&
		actual.ProductID == candidate.ProductID &&
		actual.Bus == candidate.Bus &&
		actual.PortPath == candidate.PortPath
}

func remainingTimeout(ctx context.Context) time.Duration {
	deadline, ok := ctx.Deadline()
	if !ok {
		return statusReadDeadline
	}
	remaining := time.Until(deadline)
	if remaining <= 0 {
		return time.Nanosecond
	}
	return remaining
}

var _ Discovery = (*GousbBackend)(nil)
var _ USB = (*GousbBackend)(nil)
var _ Sysfs = (*GousbBackend)(nil)
