# hidraw-safety — hardware-safety references (real repo excerpts)

## 1. Passive, kernel-backed, never claims the USB interface

`internal/hidlinux/hidraw_passive.go:1` states the contract in the package doc:

```go
//go:build linux

// Package hidlinux: HidrawBackend implements the passive status transport
// entirely over sysfs and kernel-backed /dev/hidraw nodes. It never claims a
// USB interface, so it never detaches usbhid and never interrupts mouse input.
package hidlinux
```

## 2. Opening `O_RDWR` does not detach usbhid

`internal/hidlinux/hidraw_passive.go:39` — the opener uses the kernel node, not
libusb claim/detach:

```go
// OpenNode opens the hidraw node read-write: feature reports require a writable
// fd. Opening a kernel usbhid-backed node never claims the interface.
func (osHidrawNodeOpener) OpenNode(path string) (hidrawNode, error) {
	file, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return nil, err
	}
	return &osHidrawNode{file: file}, nil
}
```

## 3. Feature reports via `HIDIOCSFEATURE`, `buf[0]` is the report ID

`internal/hidlinux/hidraw_passive.go:57`:

```go
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
```

## 4. Composition root documents "no USB taken"

`cmd/x6configurator/main.go:88` reiterates the invariant at the wiring site:

```go
// composeDesktopService shares one HidrawBackend between status and Apply.
// Both operations use the validated vendor hidraw node; no USB interface is
// claimed, detached, reset, rebound, or otherwise taken from the kernel.
func composeDesktopService(dataDir string, backend *hidlinux.HidrawBackend) (*desktop.Service, *x6.Service) {
```

## 5. Writes are user-demanded and binding-validated

`internal/mouse/service.go:265` revalidates the immutable `Binding` on every
operation; a re-plugged device cannot receive a stale write:

```go
func (s *TargetedService) ApplyOperationBound(ctx context.Context, binding Binding, operation CommandOperation, value any) error {
	selected, state, _, err := s.selectedState()
	if err != nil || selected != binding || !s.bindingCurrent(ctx, binding) || operation == nil {
		return ErrStaleBinding
	}
	...
	return s.command.SendAndAwaitBound(ctx, binding, report, func(report []byte) bool { return !operation.MatchesACK(report) })
}
```

## 6. udev policy is `uaccess`/`0660`, never `0666`/root

Per `README.md:30` and `docs/linux-usb-prerequisites.md`: the policy grants the
active seat user via `TAG+="uaccess"` with device mode `0660`. Do **not** run as
root and do **not** make the node world-writable (`0666`).
