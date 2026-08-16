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

	"github.com/alejandro/attack-shark-linux/internal/mouse"
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
	mu             sync.Mutex
	sources        map[string]Candidate
	sourceIndexes  map[string]int
	sysRoot        string
	devRoot        string
	readTimeout    time.Duration
	opener         hidrawNodeOpener
	ioOnce         sync.Once
	ioMu           sync.Mutex
	ioBusy         bool
	commandPending bool
	ioWake         chan struct{}
	activeNode     hidrawNode
	activePath     string
	listener       func([]byte) bool
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
	b.sourceIndexes = make(map[string]int)
	b.mu.Unlock()

	result := make([]transport.Candidate, 0, len(dirs))
	candidateIndex := 0
	for _, dir := range dirs {
		name := filepath.Base(dir)
		if !strings.Contains(name, "-") || strings.Contains(name, ":") {
			continue
		}
		candidate, ok := candidateFromSysfs(b.sysRoot, name)
		if !ok {
			continue
		}
		index := candidateIndex
		candidateIndex++
		profileMatch := candidate.VendorID == match.VendorID && candidate.ProductID == match.ProductID
		interfaceNumber, endpoint := inventoryInterfaceFacts(candidate)
		if !profileMatch {
			inventoryDiagnosticf("event=enumeration candidate_index=%d vid_pid=%04x:%04x interface_number=%d endpoint=0x%02x hid_usage=unknown serial_present=%t hidraw_basename=unknown profile_match=false profile_validation=not_applicable eligibility=false warning=vid_pid_mismatch selected_binding_present=false", index, candidate.VendorID, candidate.ProductID, interfaceNumber, endpoint, candidate.Serial != "")
			continue
		}
		key := candidateKey(candidate)
		if key == "" {
			inventoryDiagnosticf("event=enumeration candidate_index=%d vid_pid=%04x:%04x interface_number=%d endpoint=0x%02x hid_usage=unknown serial_present=%t hidraw_basename=%s profile_match=true profile_validation=not_checked eligibility=false warning=invalid_physical_path selected_binding_present=false", index, candidate.VendorID, candidate.ProductID, interfaceNumber, endpoint, candidate.Serial != "", "unknown")
			continue
		}
		b.mu.Lock()
		b.sources[key] = candidate
		b.sourceIndexes[key] = index
		b.mu.Unlock()
		result = append(result, transport.Candidate{Path: key, VendorID: candidate.VendorID, ProductID: candidate.ProductID, Serial: candidate.Serial, Connection: transport.Dongle})
		inventoryDiagnosticf("event=enumeration candidate_index=%d vid_pid=%04x:%04x interface_number=%d endpoint=0x%02x hid_usage=unknown serial_present=%t hidraw_basename=%s profile_match=true profile_validation=not_checked eligibility=pending warning=none selected_binding_present=false", index, candidate.VendorID, candidate.ProductID, interfaceNumber, endpoint, candidate.Serial != "", b.inventoryHidrawBasename(candidate))
	}
	if len(result) == 0 {
		return nil, &Error{Kind: NotFound, Err: errors.New("validated X6 VID:PID was not found")}
	}
	return result, nil
}

// ProfileValid checks the profile's HID facts from sysfs without opening a
// hidraw node. Inventory can therefore expose incompatible receivers safely.
func (b *HidrawBackend) ProfileValid(ctx context.Context, source transport.Candidate, facts mouse.HIDFacts) bool {
	if ctx.Err() != nil {
		return false
	}
	b.mu.Lock()
	candidate, ok := b.sources[source.Path]
	index := b.sourceIndexes[source.Path]
	b.mu.Unlock()
	interfaceNumber, endpoint := inventoryInterfaceFacts(candidate)
	hidrawBasename := "unknown"
	if ok {
		hidrawBasename = b.inventoryHidrawBasename(candidate)
	}
	if !ok || facts.StatusInput.InterfaceNumber != hidInterface || facts.StatusInput.UsagePage != 1 || facts.StatusInput.Usage != 0x80 || facts.StatusInput.EndpointAddress != statusEndpoint {
		inventoryDiagnosticf("event=profile_validation candidate_index=%d vid_pid=%04x:%04x interface_number=%d endpoint=0x%02x hid_usage=0x%04x/0x%02x serial_present=%t hidraw_basename=%s profile_match=%t profile_validation=rejected eligibility=false warning=profile_interface_mismatch selected_binding_present=false", index, candidate.VendorID, candidate.ProductID, interfaceNumber, endpoint, facts.StatusInput.UsagePage, facts.StatusInput.Usage, candidate.Serial != "", hidrawBasename, ok && matchesX6Identity(candidate))
		return false
	}
	if !matchesX6Identity(candidate) || !hasPhysicalPath(candidate) || !hasValidatedInterface(candidate.Interfaces) {
		inventoryDiagnosticf("event=profile_validation candidate_index=%d vid_pid=%04x:%04x interface_number=%d endpoint=0x%02x hid_usage=0x%04x/0x%02x serial_present=%t hidraw_basename=%s profile_match=false profile_validation=rejected eligibility=false warning=profile_interface_mismatch selected_binding_present=false", index, candidate.VendorID, candidate.ProductID, interfaceNumber, endpoint, facts.StatusInput.UsagePage, facts.StatusInput.Usage, candidate.Serial != "", hidrawBasename)
		return false
	}
	descriptor, err := b.reportDescriptor(candidate)
	valid := err == nil && hasX6TopLevelUsage(descriptor)
	warning := "none"
	if !valid {
		warning = "hid_usage_mismatch"
	}
	usagePage, usageID := facts.StatusInput.UsagePage, facts.StatusInput.Usage
	if valid {
		usagePage, usageID = 1, 0x80
	}
	inventoryDiagnosticf("event=profile_validation candidate_index=%d vid_pid=%04x:%04x interface_number=%d endpoint=0x%02x hid_usage=0x%04x/0x%02x serial_present=%t hidraw_basename=%s profile_match=true profile_validation=%t eligibility=%t warning=%s selected_binding_present=false", index, candidate.VendorID, candidate.ProductID, interfaceNumber, endpoint, usagePage, usageID, candidate.Serial != "", hidrawBasename, valid, valid, warning)
	return valid
}

// ValidateDescriptor checks every identity field against sysfs before any hidraw access.
func (b *HidrawBackend) ValidateDescriptor(ctx context.Context, source transport.Candidate, wanted transport.InputDescriptor) (transport.InputSource, error) {
	if err := ctx.Err(); err != nil {
		return transport.InputSource{}, err
	}
	b.mu.Lock()
	candidate, ok := b.sources[source.Path]
	index := b.sourceIndexes[source.Path]
	b.mu.Unlock()
	if !ok {
		inventoryDiagnosticf("event=descriptor_validation candidate_index=%d vid_pid=unknown interface_number=%d endpoint=0x%02x hid_usage=0x%04x/0x%02x serial_present=false hidraw_basename=unknown profile_match=false profile_validation=rejected eligibility=false warning=unknown_candidate selected_binding_present=false", index, wanted.InterfaceNumber, wanted.EndpointAddress, wanted.UsagePage, wanted.Usage)
		return transport.InputSource{}, &Error{Kind: NotFound, Err: errors.New("discovered device is no longer present")}
	}
	interfaceNumber, endpoint := inventoryInterfaceFacts(candidate)
	hidrawBasename := b.inventoryHidrawBasename(candidate)
	if wanted.InterfaceNumber != hidInterface || wanted.UsagePage != 1 || wanted.Usage != 0x80 || wanted.EndpointAddress != statusEndpoint {
		inventoryDiagnosticf("event=descriptor_validation candidate_index=%d vid_pid=%04x:%04x interface_number=%d endpoint=0x%02x hid_usage=0x%04x/0x%02x serial_present=%t hidraw_basename=%s profile_match=true profile_validation=rejected eligibility=false warning=descriptor_mismatch selected_binding_present=false", index, candidate.VendorID, candidate.ProductID, interfaceNumber, endpoint, wanted.UsagePage, wanted.Usage, candidate.Serial != "", hidrawBasename)
		return transport.InputSource{}, &Error{Kind: Mismatch, Err: errors.New("unsupported status descriptor request")}
	}
	if !matchesX6Identity(candidate) || !hasPhysicalPath(candidate) || !hasValidatedInterface(candidate.Interfaces) {
		inventoryDiagnosticf("event=descriptor_validation candidate_index=%d vid_pid=%04x:%04x interface_number=%d endpoint=0x%02x hid_usage=0x%04x/0x%02x serial_present=%t hidraw_basename=%s profile_match=false profile_validation=rejected eligibility=false warning=profile_interface_mismatch selected_binding_present=false", index, candidate.VendorID, candidate.ProductID, interfaceNumber, endpoint, wanted.UsagePage, wanted.Usage, candidate.Serial != "", hidrawBasename)
		return transport.InputSource{}, &Error{Kind: Mismatch, Err: errors.New("candidate does not match the validated X6 path")}
	}
	descriptor, err := b.reportDescriptor(candidate)
	if err != nil {
		return transport.InputSource{}, classify(err)
	}
	if !hasX6TopLevelUsage(descriptor) {
		inventoryDiagnosticf("event=descriptor_validation candidate_index=%d vid_pid=%04x:%04x interface_number=%d endpoint=0x%02x hid_usage=0x%04x/0x%02x serial_present=%t hidraw_basename=%s profile_match=true profile_validation=rejected eligibility=false warning=hid_usage_mismatch selected_binding_present=false", index, candidate.VendorID, candidate.ProductID, interfaceNumber, endpoint, wanted.UsagePage, wanted.Usage, candidate.Serial != "", hidrawBasename)
		return transport.InputSource{}, &Error{Kind: Mismatch, Err: errors.New("HID report descriptor does not expose usage 1/0x80")}
	}
	inventoryDiagnosticf("event=descriptor_validation candidate_index=%d vid_pid=%04x:%04x interface_number=%d endpoint=0x%02x hid_usage=0x%04x/0x%02x serial_present=%t hidraw_basename=%s profile_match=true profile_validation=valid eligibility=true warning=none selected_binding_present=false", index, candidate.VendorID, candidate.ProductID, interfaceNumber, endpoint, wanted.UsagePage, wanted.Usage, candidate.Serial != "", hidrawBasename)
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
	b.mu.Lock()
	b.activeNode = node
	b.activePath = source.Path
	b.listener = use
	b.mu.Unlock()
	defer func() {
		b.mu.Lock()
		sameNode := b.activeNode == node
		if sameNode {
			b.activeNode = nil
			b.activePath = ""
			b.listener = nil
		}
		b.mu.Unlock()
		_ = node.Close()
	}()

	bounded, cancel := context.WithTimeout(ctx, b.readTimeout)
	defer cancel()
	buffer := make([]byte, 64)
	for {
		release, acquireErr := b.acquireIO(bounded, false)
		if acquireErr != nil {
			return acquireErr
		}
		count, readErr := b.readNode(bounded, node, buffer)
		if readErr != nil {
			release()
			return readErr
		}
		if count == 0 {
			release()
			continue
		}
		if !use(buffer[:count]) {
			release()
			return nil
		}
		release()
	}
}

// SendAndAwait implements transport.CommandTransport entirely through the
// validated vendor hidraw node. Command ownership is granted before feature
// write and ACK read; the listener cannot start another read until ownership is
// released. Reports consumed while Apply owns the node are synchronously sent
// to the listener callback, so status events are not silently discarded.
func (b *HidrawBackend) SendAndAwait(ctx context.Context, payload []byte, continueReading func([]byte) bool) error {
	if len(payload) != dpiReportLength {
		return &Error{Kind: Mismatch, Err: fmt.Errorf("DPI feature report length = %d, want %d", len(payload), dpiReportLength)}
	}
	if err := ctx.Err(); err != nil {
		return classify(err)
	}
	if _, err := b.Enumerate(ctx, transport.X6Match()); err != nil {
		return &diagnosticError{operation: "discovery", err: err}
	}
	candidate, err := b.commandCandidate()
	if err != nil {
		return &diagnosticError{operation: "discovery", err: err}
	}
	if _, err := b.ValidateDescriptor(ctx, transport.Candidate{Path: candidateKey(candidate)}, transport.InputDescriptor{
		InterfaceNumber: hidInterface,
		UsagePage:       1,
		Usage:           0x80,
		EndpointAddress: statusEndpoint,
	}); err != nil {
		return &diagnosticError{operation: "discovery", err: err}
	}
	path, err := b.hidrawPath(candidate)
	if err != nil {
		return &diagnosticError{operation: "discovery", err: err}
	}

	release, err := b.beginCommand(ctx)
	if err != nil {
		return &diagnosticError{operation: "ack_failure", err: err}
	}
	defer release()

	bounded, cancel := context.WithTimeout(ctx, b.readTimeout)
	defer cancel()
	// Commands own a separate node from the always-on listener. The listener
	// may close its node while its bounded read shuts down.
	node, err := b.opener.OpenNode(path)
	if err != nil {
		return &diagnosticError{operation: "transfer", err: classify(err)}
	}
	defer node.Close()

	count, err := node.SendFeatureReport(payload)
	if err != nil {
		return &diagnosticError{operation: "transfer", err: classify(err)}
	}
	if count != len(payload) {
		return &diagnosticError{operation: "transfer", err: fmt.Errorf("DPI feature report wrote %d bytes, want %d", count, len(payload))}
	}

	buffer := make([]byte, 64)
	for {
		count, err := b.readNode(bounded, node, buffer)
		if err != nil {
			return &diagnosticError{operation: "ack_failure", err: err}
		}
		report := append([]byte(nil), buffer[:count]...)
		b.dispatchListenerReport(candidateKey(candidate), report)
		if !continueReading(report) {
			return nil
		}
	}
}

func (b *HidrawBackend) commandCandidate() (Candidate, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.sources) == 0 {
		return Candidate{}, &Error{Kind: NotFound, Err: errors.New("no validated X6 device is available for DPI Apply")}
	}
	if len(b.sources) != 1 {
		return Candidate{}, &Error{Kind: Mismatch, Err: errors.New("multiple validated X6 devices are available for DPI Apply")}
	}
	for _, candidate := range b.sources {
		return candidate, nil
	}
	panic("unreachable")
}

func (b *HidrawBackend) dispatchListenerReport(path string, report []byte) {
	b.mu.Lock()
	listener := b.listener
	activePath := b.activePath
	b.mu.Unlock()
	if listener != nil && activePath == path {
		listener(report)
	}
}

func (b *HidrawBackend) initIO() {
	b.ioOnce.Do(func() { b.ioWake = make(chan struct{}) })
}

// acquireIO grants one serialized hidraw I/O turn. Command priority is set by
// beginCommand before waiting, preventing the listener from consuming the ACK
// between the feature write and the command's read.
func (b *HidrawBackend) acquireIO(ctx context.Context, command bool) (func(), error) {
	b.initIO()
	for {
		b.ioMu.Lock()
		if !b.ioBusy && (command || !b.commandPending) {
			b.ioBusy = true
			b.ioMu.Unlock()
			return func() { b.releaseIO() }, nil
		}
		wake := b.ioWake
		b.ioMu.Unlock()

		select {
		case <-ctx.Done():
			return nil, classify(ctx.Err())
		case <-wake:
		}
	}
}

func (b *HidrawBackend) releaseIO() {
	b.initIO()
	b.ioMu.Lock()
	b.ioBusy = false
	close(b.ioWake)
	b.ioWake = make(chan struct{})
	b.ioMu.Unlock()
}

func (b *HidrawBackend) beginCommand(ctx context.Context) (func(), error) {
	b.initIO()
	b.ioMu.Lock()
	b.commandPending = true
	close(b.ioWake)
	b.ioWake = make(chan struct{})
	b.ioMu.Unlock()

	release, err := b.acquireIO(ctx, true)
	if err != nil {
		b.finishCommand()
		return nil, err
	}
	return func() {
		release()
		b.finishCommand()
	}, nil
}

func (b *HidrawBackend) finishCommand() {
	b.initIO()
	b.ioMu.Lock()
	b.commandPending = false
	close(b.ioWake)
	b.ioWake = make(chan struct{})
	b.ioMu.Unlock()
}

type nodeRead struct {
	count int
	err   error
}

const readWorkerShutdownTimeout = 100 * time.Millisecond

// readNode reads one report, bounded by the context. A blocking hidraw read is
// released on timeout by closing the node, and the read worker is joined before
// returning so it cannot outlive the node owner.
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
		shutdown := time.NewTimer(readWorkerShutdownTimeout)
		defer shutdown.Stop()
		select {
		case <-result:
		case <-shutdown.C:
			return 0, &Error{Kind: Timeout, Err: fmt.Errorf("hidraw read worker did not stop after close: %w", ctx.Err())}
		}
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
	serial, _ := os.ReadFile(filepath.Join(base, "serial"))
	return Candidate{VendorID: vendor, ProductID: product, Bus: bus, PortPath: dir, Serial: strings.TrimSpace(string(serial)), Interfaces: interfaceDescriptorsFromSysfs(base, dir)}, true
}

func inventoryInterfaceFacts(candidate Candidate) (int, uint8) {
	for _, descriptor := range candidate.Interfaces {
		for _, endpoint := range descriptor.Endpoints {
			if endpoint == statusEndpoint {
				return descriptor.Number, endpoint
			}
		}
	}
	if len(candidate.Interfaces) > 0 {
		return candidate.Interfaces[0].Number, firstEndpoint(candidate.Interfaces[0].Endpoints)
	}
	return -1, 0
}

func firstEndpoint(endpoints []uint8) uint8 {
	if len(endpoints) == 0 {
		return 0
	}
	return endpoints[0]
}

func (b *HidrawBackend) inventoryHidrawBasename(candidate Candidate) string {
	path, err := b.hidrawPath(candidate)
	if err != nil {
		return "unknown"
	}
	return filepath.Base(path)
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
