package x6

import "fmt"

const PollingReportLength = 9

// PollingRate is one of the X6's supported report frequencies in hertz.
type PollingRate uint16

const (
	PollingRate125  PollingRate = 125
	PollingRate250  PollingRate = 250
	PollingRate500  PollingRate = 500
	PollingRate1000 PollingRate = 1000
)

// ValidatePollingRate accepts only frequencies represented by the X6 0x06 command.
func ValidatePollingRate(rate PollingRate) error {
	switch rate {
	case PollingRate125, PollingRate250, PollingRate500, PollingRate1000:
		return nil
	default:
		return fmt.Errorf("unsupported polling rate %d Hz", rate)
	}
}

// EncodePollingReport creates the exact nine-byte X6 polling-rate command.
func EncodePollingReport(rate PollingRate) ([]byte, error) {
	if err := ValidatePollingRate(rate); err != nil {
		return nil, err
	}

	divisor := byte(1000 / rate)
	return []byte{0x06, 0x09, 0x01, divisor, ^divisor, 0x00, 0x00, 0x00, 0x00}, nil
}

// MatchesPollingACK accepts only the acknowledgement for a polling-rate command.
func MatchesPollingACK(report []byte) bool {
	return len(report) == 5 && report[0] == 0x03 && report[1] == 0x10 && report[2] == 0x50 && report[3] == 0x00 && report[4] == 0x06
}
