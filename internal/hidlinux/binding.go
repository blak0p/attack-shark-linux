package hidlinux

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/blak0p/attack-shark-linux/internal/mouse"
	"github.com/blak0p/attack-shark-linux/internal/transport"
)

const (
	pollingReportLength  = 9
	lightingReportLength = 13
)

// SendAndAwaitBound revalidates one captured binding and uses only its matching
// hidraw node. It performs no USB claims, resets, rebinding, or live discovery
// fallback to another device.
func (b *HidrawBackend) SendAndAwaitBound(ctx context.Context, binding mouse.Binding, payload []byte, continueReading func([]byte) bool) error {
	if !supportedFeatureReport(payload) {
		return &Error{Kind: Mismatch, Err: fmt.Errorf("unsupported feature report ID/length = %#x/%d", payloadID(payload), len(payload))}
	}
	if err := ctx.Err(); err != nil {
		return classify(err)
	}
	if _, err := b.Enumerate(ctx, transport.Match{VendorID: binding.ID.VendorID, ProductID: binding.ID.ProductID}); err != nil {
		return mouse.ErrStaleBinding
	}
	b.mu.Lock()
	candidate, ok := b.sources[binding.Path]
	b.mu.Unlock()
	serialMatches := b.serial(candidate) == binding.ID.Serial
	if binding.SessionOnly {
		serialMatches = b.serial(candidate) == ""
	}
	if !ok || candidateKey(candidate) != binding.Path || candidate.VendorID != binding.ID.VendorID || candidate.ProductID != binding.ID.ProductID || !serialMatches {
		return mouse.ErrStaleBinding
	}
	if _, err := b.ValidateDescriptor(ctx, transport.Candidate{Path: binding.Path}, transport.InputDescriptor{InterfaceNumber: hidInterface, UsagePage: 1, Usage: 0x80, EndpointAddress: statusEndpoint}); err != nil {
		return mouse.ErrStaleBinding
	}
	path, err := b.hidrawPath(candidate)
	if err != nil {
		return mouse.ErrStaleBinding
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
		return &diagnosticError{operation: "transfer", err: fmt.Errorf("feature report wrote %d bytes, want %d", count, len(payload))}
	}
	buffer := make([]byte, 64)
	for {
		count, err := b.readNode(bounded, node, buffer)
		if err != nil {
			return &diagnosticError{operation: "ack_failure", err: err}
		}
		report := append([]byte(nil), buffer[:count]...)
		b.dispatchListenerReport(binding.Path, report)
		if !continueReading(report) {
			return nil
		}
	}
}

func supportedFeatureReport(payload []byte) bool {
	return (len(payload) == dpiReportLength && payload[0] == 0x04) ||
		(len(payload) == pollingReportLength && payload[0] == 0x06) ||
		(len(payload) == lightingReportLength && payload[0] == 0x05)
}

func payloadID(payload []byte) byte {
	if len(payload) == 0 {
		return 0
	}
	return payload[0]
}

func (b *HidrawBackend) serial(candidate Candidate) string {
	data, err := os.ReadFile(filepath.Join(b.sysRoot, "bus/usb/devices", candidate.PortPath, "serial"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}
