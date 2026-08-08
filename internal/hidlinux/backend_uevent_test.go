package hidlinux

import (
	"strings"
	"testing"
)

func TestParseHIDID(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantVid uint16
		wantPid uint16
		wantOK  bool
	}{
		{"x6 dongle", "0003:00001D57:0000FA60", 0x1D57, 0xFA60, true},
		{"keyboard", "0003:00001038:00002232", 0x1038, 0x2232, true},
		{"bluetooth type", "0005:00001234:00005678", 0x1234, 0x5678, true},
		{"wrong arity", "0003:00001D57", 0, 0, false},
		{"non-hex", "0003:zzzz:0000FA60", 0, 0, false},
		{"empty", "", 0, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vid, pid, ok := parseHIDID(tt.input)
			if ok != tt.wantOK || vid != tt.wantVid || pid != tt.wantPid {
				t.Fatalf("parseHIDID(%q) = (%#x,%#x,%v), want (%#x,%#x,%v)", tt.input, vid, pid, ok, tt.wantVid, tt.wantPid, tt.wantOK)
			}
		})
	}
}

func TestPortFromHIDPhys(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"root hub port", "usb-0000:0d:00.0-4/input2", "4"},
		{"nested hub", "usb-0000:0d:00.0-6.4/input3", "6.4"},
		{"no input suffix", "usb-0000:0d:00.0-4", "4"},
		{"no dash", "input2", ""},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := portFromHIDPhys(tt.input); got != tt.want {
				t.Fatalf("portFromHIDPhys(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestPortFromPortPath(t *testing.T) {
	tests := []struct{ input, want string }{
		{"1-4", "4"},
		{"1-6.4", "6.4"},
		{"7-2", "2"},
		{"no-dash", "dash"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := portFromPortPath(tt.input); got != tt.want {
			t.Fatalf("portFromPortPath(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestHIDUEventMatches(t *testing.T) {
	x6 := Candidate{VendorID: 0x1D57, ProductID: 0xFA60, Bus: 1, PortPath: "1-4"}
	keyboard := Candidate{VendorID: 0x1038, ProductID: 0x2232, Bus: 1, PortPath: "1-6.4"}

	x6KbUevent := "DRIVER=hid-generic\nHID_ID=0003:00001D57:0000FA60\nHID_PHYS=usb-0000:0d:00.0-4/input0\n"
	x6VendorUevent := "DRIVER=hid-generic\nHID_ID=0003:00001D57:0000FA60\nHID_PHYS=usb-0000:0d:00.0-4/input2\n"
	kbUevent := "DRIVER=hid-generic\nHID_ID=0003:00001038:00002232\nHID_PHYS=usb-0000:0d:00.0-6.4/input3\n"

	tests := []struct {
		name   string
		uevent string
		cand   Candidate
		want   bool
	}{
		{"x6 vendor interface matches", x6VendorUevent, x6, true},
		{"x6 keyboard interface does not match", x6KbUevent, x6, false},
		{"keyboard uevent on wrong interface rejected", kbUevent, keyboard, false},
		{"keyboard uevent does not match x6", kbUevent, x6, false},
		{"x6 uevent does not match keyboard", x6VendorUevent, keyboard, false},
		{"missing HID_ID", "DRIVER=hid-generic\n", x6, false},
		{"wrong vid/pid in HID_ID", strings.Replace(x6VendorUevent, "00001D57", "0000BEEF", 1), x6, false},
		{"missing HID_PHYS rejected", "HID_ID=0003:00001D57:0000FA60\n", x6, false},
		{"wrong port rejected", "HID_ID=0003:00001D57:0000FA60\nHID_PHYS=usb-0000:0d:00.0-5/input2\n", x6, false},
		{"non-input suffix rejected", "HID_ID=0003:00001D57:0000FA60\nHID_PHYS=usb-0000:0d:00.0-4/somewhere\n", x6, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hidUEventMatches([]byte(tt.uevent), tt.cand); got != tt.want {
				t.Fatalf("hidUEventMatches(%q, %+v) = %v, want %v", tt.uevent, tt.cand, got, tt.want)
			}
		})
	}
}
