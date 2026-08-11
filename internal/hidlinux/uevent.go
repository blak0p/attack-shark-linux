//go:build linux

package hidlinux

import (
	"strconv"
	"strings"
)

// hidUEventMatches reports whether a HID device uevent belongs to the
// candidate's validated physical path and interface 2.
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
	return interfaceFromHIDPhys(fields["HID_PHYS"]) == hidInterface
}

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

func portFromPortPath(portPath string) string {
	dash := strings.IndexByte(portPath, '-')
	if dash < 0 {
		return portPath
	}
	return portPath[dash+1:]
}
