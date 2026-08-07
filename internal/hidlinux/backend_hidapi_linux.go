//go:build linux && hidapi

package hidlinux

import "github.com/sstallion/go-hid"

import "github.com/alejandro/attack-shark-linux/internal/x6"

type goHIDBackend struct{}

func NewPassiveTransport() (x6.PassiveInputTransport, error) {
	return newPassiveAdapter(goHIDBackend{}), nil
}
func NewCommandTransport(path string) (x6.CommandTransport, error) {
	return newCommandAdapterAtPath(goHIDCommandBackend{}, path), nil
}
func (goHIDBackend) Enumerate(vid, pid uint16, visit func(deviceInfo) error) error {
	return hid.Enumerate(vid, pid, func(info *hid.DeviceInfo) error {
		return visit(deviceInfo{path: info.Path, vendorID: info.VendorID, productID: info.ProductID, usagePage: info.UsagePage, usage: info.Usage, interfaceNumber: info.InterfaceNbr})
	})
}
func (goHIDBackend) OpenPath(path string) (inputDevice, error) { return hid.OpenPath(path) }

type goHIDCommandBackend struct{}

func (goHIDCommandBackend) OpenPath(path string) (commandDevice, error) { return hid.OpenPath(path) }
