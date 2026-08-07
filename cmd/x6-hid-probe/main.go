//go:build hidapi

// x6-hid-probe is an opt-in hardware capability probe. It is not used by the
// application and requires both a flag and environment gate before writing.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/alejandro/attack-shark-linux/internal/hidlinux"
	"github.com/sstallion/go-hid"
)

const (
	x6VendorID  = 0x1D57
	x6ProductID = 0xFA60
)

func main() {
	var reportHex string
	var timeout time.Duration
	var explicitWrite bool
	flag.StringVar(&reportHex, "report-hex", "", "complete feature report bytes in hexadecimal")
	flag.DurationVar(&timeout, "ack-timeout", 2*time.Second, "bounded acknowledgement timeout")
	flag.BoolVar(&explicitWrite, "i-understand-this-writes", false, "allow one feature-report write")
	flag.Parse()

	if !hidlinux.ProbeEnabled(explicitWrite, os.Getenv("ATTACK_SHARK_X6_HARDWARE")) {
		fatal("hardware write disabled: require --i-understand-this-writes and ATTACK_SHARK_X6_HARDWARE=1")
	}
	if reportHex == "" || timeout <= 0 {
		fatal("report-hex and a positive ack-timeout are required")
	}
	report, err := decodeHex(reportHex)
	if err != nil || len(report) == 0 {
		fatal("report-hex must contain complete hexadecimal bytes")
	}

	device, err := openInterface2()
	if err != nil {
		fatal(err.Error())
	}
	defer device.Close()

	n, err := device.SendFeatureReport(report)
	if err != nil {
		fatal(fmt.Sprintf("SendFeatureReport failed: %v", err))
	}
	if n != len(report) {
		fatal(fmt.Sprintf("SendFeatureReport wrote %d bytes, want %d", n, len(report)))
	}
	if err := awaitACK(device, report[0], timeout); err != nil {
		fatal(err.Error())
	}
	fmt.Printf("probe succeeded: interface=2 write=%d ACK=03 10 50 00 %02x\n", n, report[0])
}

func openInterface2() (*hid.Device, error) {
	var path string
	err := hid.Enumerate(x6VendorID, x6ProductID, func(info *hid.DeviceInfo) error {
		if info.InterfaceNbr == 2 && info.UsagePage == 1 && info.Usage == 0x80 {
			path = info.Path
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("enumerate X6: %w", err)
	}
	if path == "" {
		return nil, errors.New("no X6 interface 2 configuration path found")
	}
	device, err := hid.OpenPath(path)
	if err != nil {
		return nil, fmt.Errorf("open interface 2 (check udev permissions): %w", err)
	}
	return device, nil
}

func awaitACK(device *hid.Device, reportID byte, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	buffer := make([]byte, 64)
	for remaining := time.Until(deadline); remaining > 0; remaining = time.Until(deadline) {
		n, err := device.ReadWithTimeout(buffer, remaining)
		if errors.Is(err, hid.ErrTimeout) {
			continue
		}
		if err != nil {
			return fmt.Errorf("read ACK: %w", err)
		}
		if hidlinux.MatchesProbeACK(buffer[:n], reportID) {
			return nil
		}
	}
	return fmt.Errorf("timed out waiting %s for ACK of report 0x%02x", timeout, reportID)
}

func decodeHex(value string) ([]byte, error) {
	fields := strings.Fields(value)
	result := make([]byte, len(fields))
	for i, field := range fields {
		if _, err := fmt.Sscanf(field, "%02x", &result[i]); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func fatal(message string) {
	fmt.Fprintln(os.Stderr, message)
	os.Exit(1)
}
