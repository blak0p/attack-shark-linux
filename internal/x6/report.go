package x6

const (
	statusReportID  = 0x03
	heartbeatEvent  = 0x40
	batteryByte     = 4
	maxBatteryLevel = 10
	batteryStep     = 10
)

type ReportStatus struct {
	BatteryPercent   int
	BatteryAvailable bool
}

func DecodeValidatedInputReport(report []byte) ReportStatus {
	if !isHeartbeatReport(report) {
		return ReportStatus{}
	}

	level := report[batteryByte]
	if level > maxBatteryLevel {
		return ReportStatus{}
	}

	return ReportStatus{BatteryPercent: int(level) * batteryStep, BatteryAvailable: true}
}

func isHeartbeatReport(report []byte) bool {
	return len(report) > batteryByte && report[0] == statusReportID && report[1] == heartbeatEvent
}
