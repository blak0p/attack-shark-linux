package hidlinux

// ProbeEnabled requires both a command-line acknowledgement and a dedicated
// environment gate so normal builds and test runs cannot write to hardware.
func ProbeEnabled(explicitWriteFlag bool, environmentGate string) bool {
	return explicitWriteFlag && environmentGate == "1"
}

// MatchesProbeACK accepts only the X6 configuration acknowledgement for reportID.
func MatchesProbeACK(report []byte, reportID byte) bool {
	return len(report) == 5 && report[0] == 0x03 && report[1] == 0x10 && report[2] == 0x50 && report[3] == 0x00 && report[4] == reportID
}
