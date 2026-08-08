//go:build linux

// Package hidlinux: HidrawBackend implements the passive status transport
// entirely over sysfs and kernel-backed /dev/hidraw nodes. It never claims a
// USB interface, so it never detaches usbhid and never interrupts mouse input.
package hidlinux

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/alejandro/attack-shark-linux/internal/transport"
)

type hidrawNode interface {
	io.ReadCloser
	// SendFeatureReport writes one feature report (report ID first byte) via
	// the HIDIOCSFEATURE ioctl. It returns the number of bytes transferred.
	SendFeatureReport(report []byte) (int, error)
}

type hidrawNodeOpener interface {
	OpenNode(path string) (hidrawNode, error)
}

type osHidrawNodeOpener struct{}

// OpenNode opens the hidraw node read-write: feature reports require a writable
// fd. Opening a kernel usbhid-backed node never claims the interface.
func (osHidrawNodeOpener) OpenNode(path string) (hidrawNode, error) {
	file, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return nil, err
	}
	return &osHidrawNode{file: file}, nil
}

// osHidrawNode wraps *os.File so feature reports reach the node through
// HIDIOCSFEATURE while reads and close stay on the kernel file.
type osHidrawNode struct{ file *os.File }

func (n *osHidrawNode) Read(p []byte) (int, error) { return n.file.Read(p) }

func (n *osHidrawNode) Close() error { return n.file.Close() }

// SendFeatureReport issues HIDIOCSFEATURE(len) with the report buffer. buf[0]
// is the report ID, so the ioctl size includes the ID byte, matching the kernel
// hidraw_ioctl contract (buf[0] passed as report_number, size as report length).
func (n *osHidrawNode) SendFeatureReport(report []byte) (int, error) {
	if len(report) == 0 {
		return 0, errors.New("hidraw feature report is empty")
	}
	request := hidrawFeatureReportRequest(len(report))
	count, _, errno := syscall.Syscall(syscall.SYS_IOCTL, n.file.Fd(), uintptr(request), uintptr(unsafe.Pointer(&report[0])))
	if errno != 0 {
		return 0, errno
	}
	return int(count), nil
}

// hidrawFeatureReportRequest builds the HIDIOCSFEATURE(len) ioctl number:
// _IOC(_IOC_WRITE|_IOC_READ, 'H', 0x06, len) = (3<<30) | ('H'<<8) | 0x06 | (len<<16).
func hidrawFeatureReportRequest(length int) int {
	return (3 << 30) | ('H' << 8) | 0x06 | (length << 16)
}

// HidrawBackend discovers the X6 through sysfs and reads its status reports
// from the matching /dev/hidraw node. sysRoot and devRoot are injectable so
// tests run against fixture trees instead of the live system.
type HidrawBackend struct {
	mu          sync.Mutex
	sources     map[string]Candidate
	sysRoot     string
	devRoot     string
	readTimeout time.Duration
	opener      hidrawNodeOpener
}

func NewHidrawBackend() *HidrawBackend {
	return &HidrawBackend{
		sources:     make(map[string]Candidate),
		sysRoot:     "/sys",
		devRoot:     "/dev",
		readTimeout: statusReadDeadline,
		opener:      osHidrawNodeOpener{},
	}
}

// Enumerate scans sysfs for X6 USB devices without opening or claiming them.
func (b *HidrawBackend) Enumerate(ctx context.Context, match transport.Match) ([]transport.Candidate, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	dirs, err := filepath.Glob(filepath.Join(b.sysRoot, "bus/usb/devices/*"))
	if err != nil {
		return nil, classify(err)
	}

	b.mu.Lock()
	b.sources = make(map[string]Candidate)
	b.mu.Unlock()

	result := make([]transport.Candidate, 0, len(dirs))
	for _, dir := range dirs {
		name := filepath.Base(dir)
		if !strings.Contains(name, "-") || strings.Contains(name, ":") {
			continue
		}
		candidate, ok := candidateFromSysfs(b.sysRoot, name)
		if !ok || candidate.VendorID != match.VendorID || candidate.ProductID != match.ProductID {
			continue
		}
		key := candidateKey(candidate)
		if key == "" {
			continue
		}
		b.mu.Lock()
		b.sources[key] = candidate
		b.mu.Unlock()
		result = append(result, transport.Candidate{Path: key, VendorID: candidate.VendorID, ProductID: candidate.ProductID, Connection: transport.Dongle})
	}
	if len(result) == 0 {
		return nil, &Error{Kind: NotFound, Err: errors.New("validated X6 VID:PID was not found")}
	}
	return result, nil
}

// ValidateDescriptor checks every identity field against sysfs before any hidraw access.
func (b *HidrawBackend) ValidateDescriptor(ctx context.Context, source transport.Candidate, wanted transport.InputDescriptor) (transport.InputSource, error) {
	if err := ctx.Err(); err != nil {
		return transport.InputSource{}, err
	}
	b.mu.Lock()
	candidate, ok := b.sources[source.Path]
	b.mu.Unlock()
	if !ok {
		return transport.InputSource{}, &Error{Kind: NotFound, Err: errors.New("discovered device is no longer present")}
	}
	if wanted.InterfaceNumber != hidInterface || wanted.UsagePage != 1 || wanted.Usage != 0x80 || wanted.EndpointAddress != statusEndpoint {
		return transport.InputSource{}, &Error{Kind: Mismatch, Err: errors.New("unsupported status descriptor request")}
	}
	if !matchesX6Identity(candidate) || !hasPhysicalPath(candidate) || !hasValidatedInterface(candidate.Interfaces) {
		return transport.InputSource{}, &Error{Kind: Mismatch, Err: errors.New("candidate does not match the validated X6 path")}
	}
	descriptor, err := b.reportDescriptor(candidate)
	if err != nil {
		return transport.InputSource{}, classify(err)
	}
	if !hasX6TopLevelUsage(descriptor) {
		return transport.InputSource{}, &Error{Kind: Mismatch, Err: errors.New("HID report descriptor does not expose usage 1/0x80")}
	}
	return transport.InputSource{Path: source.Path}, nil
}

// ReadInterruptIN performs one bounded, passive status read through hidraw.
func (b *HidrawBackend) ReadInterruptIN(ctx context.Context, source transport.InputSource, use func([]byte) bool) error {
	b.mu.Lock()
	candidate, ok := b.sources[source.Path]
	b.mu.Unlock()
	if !ok {
		return &Error{Kind: NotFound, Err: errors.New("validated device is no longer present")}
	}
	path, err := b.hidrawPath(candidate)
	if err != nil {
		return err
	}
	node, err := b.opener.OpenNode(path)
	if err != nil {
		return classify(err)
	}
	defer node.Close()

	bounded, cancel := context.WithTimeout(ctx, b.readTimeout)
	defer cancel()
	buffer := make([]byte, 64)
	for {
		count, readErr := b.readNode(bounded, node, buffer)
		if readErr != nil {
			return readErr
		}
		if !use(buffer[:count]) {
			return nil
		}
	}
}

type nodeRead struct {
	count int
	err   error
}

// readNode reads one report, bounded by the context. A blocking hidraw read is
// released on timeout by closing the node from the timeout path.
func (b *HidrawBackend) readNode(ctx context.Context, node hidrawNode, buffer []byte) (int, error) {
	result := make(chan nodeRead, 1)
	go func() {
		count, err := node.Read(buffer)
		result <- nodeRead{count: count, err: err}
	}()
	select {
	case read := <-result:
		if read.err != nil {
			return 0, classify(read.err)
		}
		return read.count, nil
	case <-ctx.Done():
		_ = node.Close()
		return 0, &Error{Kind: Timeout, Err: ctx.Err()}
	}
}

func (b *HidrawBackend) reportDescriptor(candidate Candidate) ([]byte, error) {
	hidDir, err := b.hidrawDeviceDir(candidate)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(filepath.Join(hidDir, "report_descriptor"))
}

// hidrawDeviceDir finds the sysfs HID device directory matching the candidate
// on the adapter's status interface.
func (b *HidrawBackend) hidrawDeviceDir(candidate Candidate) (string, error) {
	dirs, err := filepath.Glob(filepath.Join(b.sysRoot, "bus/hid/devices/*"))
	if err != nil {
		return "", classify(err)
	}
	for _, dir := range dirs {
		uevent, err := os.ReadFile(filepath.Join(dir, "uevent"))
		if err != nil || !hidUEventMatches(uevent, candidate) {
			continue
		}
		return dir, nil
	}
	return "", &Error{Kind: NotFound, Err: fmt.Errorf("HID device for %s was not found", candidateKey(candidate))}
}

// hidrawPath resolves the /dev/hidrawN node exposing the candidate's status
// interface by following the class symlink back to the HID device directory.
func (b *HidrawBackend) hidrawPath(candidate Candidate) (string, error) {
	hidDir, err := b.hidrawDeviceDir(candidate)
	if err != nil {
		return "", err
	}
	hidTarget, err := filepath.EvalSymlinks(hidDir)
	if err != nil {
		return "", classify(err)
	}
	nodes, err := filepath.Glob(filepath.Join(b.sysRoot, "class/hidraw/hidraw*"))
	if err != nil {
		return "", classify(err)
	}
	for _, node := range nodes {
		resolved, err := filepath.EvalSymlinks(filepath.Join(node, "device"))
		if err != nil || resolved != hidTarget {
			continue
		}
		return filepath.Join(b.devRoot, filepath.Base(node)), nil
	}
	return "", &Error{Kind: NotFound, Err: errors.New("no hidraw node exposes the X6 HID interface")}
}

func candidateFromSysfs(sysRoot, dir string) (Candidate, bool) {
	base := filepath.Join(sysRoot, "bus/usb/devices", dir)
	vendor, err := readHexFile(filepath.Join(base, "idVendor"))
	if err != nil {
		return Candidate{}, false
	}
	product, err := readHexFile(filepath.Join(base, "idProduct"))
	if err != nil {
		return Candidate{}, false
	}
	bus, ok := busFromDeviceName(dir)
	if !ok {
		return Candidate{}, false
	}
	return Candidate{VendorID: vendor, ProductID: product, Bus: bus, PortPath: dir, Interfaces: interfaceDescriptorsFromSysfs(base, dir)}, true
}

func interfaceDescriptorsFromSysfs(base, dir string) []InterfaceDescriptor {
	entries, err := filepath.Glob(filepath.Join(base, dir+":*"))
	if err != nil {
		return nil
	}
	descriptors := make([]InterfaceDescriptor, 0, len(entries))
	for _, entry := range entries {
		number, err := readUintFile(filepath.Join(entry, "bInterfaceNumber"))
		if err != nil {
			continue
		}
		class, err := readHexFile(filepath.Join(entry, "bInterfaceClass"))
		if err != nil {
			continue
		}
		endpoints := make([]uint8, 0, 2)
		endpointDirs, err := filepath.Glob(filepath.Join(entry, "ep_*"))
		if err == nil {
			for _, endpointDir := range endpointDirs {
				if endpoint, err := readHexFile(filepath.Join(endpointDir, "bEndpointAddress")); err == nil {
					endpoints = append(endpoints, uint8(endpoint))
				}
			}
		}
		descriptors = append(descriptors, InterfaceDescriptor{Number: number, AlternateSetting: 0, Class: uint8(class), Endpoints: endpoints})
	}
	return descriptors
}

// busFromDeviceName extracts the USB bus number from a sysfs device name like
// "1-4" or "1-6.4".
func busFromDeviceName(name string) (uint8, bool) {
	dash := strings.IndexByte(name, '-')
	if dash <= 0 {
		return 0, false
	}
	bus, err := strconv.ParseUint(name[:dash], 10, 8)
	if err != nil {
		return 0, false
	}
	return uint8(bus), true
}

func readHexFile(path string) (uint16, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	value := strings.TrimSpace(string(data))
	value = strings.TrimPrefix(value, "0x")
	parsed, err := strconv.ParseUint(value, 16, 32)
	return uint16(parsed), err
}

func readUintFile(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(data)))
}
