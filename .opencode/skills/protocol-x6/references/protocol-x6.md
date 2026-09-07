# protocol-x6 — wire-format references (real repo excerpts)

## 1. Pure, hardware-free package

`internal/protocol/x6/dpi.go:1` — no hidraw, no transport, no syscalls:

```go
// Package x6 contains pure, hardware-free facts about the X6 protocol.
package x6
```

## 2. Encoder returns `([]byte, error)` with a length constant

`internal/protocol/x6/dpi.go:6` declares the constant; `:24` builds the report
and computes the checksum over a fixed range:

```go
const DPIReportLength = 52

func EncodeDPIReport(config DPIConfig) ([]byte, error) {
	if config.StageMask == 0 || config.LiftDistance > 1 || config.ActiveStage < 1 || config.ActiveStage > 8 {
		return nil, fmt.Errorf("invalid DPI configuration controls")
	}
	report := make([]byte, DPIReportLength)
	report[0], report[1], report[2] = 0x04, 0x38, 0x01
	...
	checksum := 0
	for _, value := range report[3:50] {
		checksum += int(value)
	}
	report[50], report[51] = byte(checksum>>8), byte(checksum)
	return report, nil
}
```

## 3. Decoder returns `(T, bool)` and rejects mismatches

`internal/protocol/x6/dpi.go:76` — strict shape check, `false` on anything else:

```go
func DecodeStatusReport(report []byte) (StatusReport, bool) {
	if len(report) != 5 || report[0] != 0x03 || report[1] != 0x10 {
		return StatusReport{}, false
	}
	switch report[2] {
	case 0x40:
		return StatusReport{Battery: int(report[4]) * 10, BatteryAvailable: true}, true
	case 0x10:
		return StatusReport{ActiveStage: report[3], StageAvailable: true}, true
	default:
		return StatusReport{}, false
	}
}
```

## 4. Operation adapter in `x6` wraps the pure functions

`internal/x6/polling.go:41` adapts the protocol to `mouse.CommandOperation`:

```go
func NewPollingOperation() mouse.CommandOperation { return pollingOperation{} }

type pollingOperation struct{}

func (pollingOperation) Validate(value any) error {
	rate, ok := value.(PollingRate)
	if !ok {
		return fmt.Errorf("X6 polling rate has type %T", value)
	}
	return protocol.ValidatePollingRate(rate)
}

func (pollingOperation) Encode(value any) ([]byte, error) {
	rate, ok := value.(PollingRate)
	if !ok {
		return nil, fmt.Errorf("X6 polling rate has type %T", value)
	}
	return protocol.EncodePollingReport(rate)
}

func (pollingOperation) MatchesACK(report []byte) bool { return protocol.MatchesPollingACK(report) }
```

Lighting follows the same shape (`internal/x6/lighting.go:83`):
`NewLightingOperation() mouse.CommandOperation` with `Validate`/`Encode`/
`MatchesACK`.

## 5. Type aliases keep the domain thin

`internal/x6/dpi.go:10` re-exports the protocol type so callers use `x6.DPIConfig`
without importing `protocol` directly:

```go
type DPIConfig = protocol.DPIConfig
```

## 6. Hardware-free, byte-exact tests

`internal/x6/dpi_tdd_test.go:5` asserts exact length and header bytes and a
rejection case — no device required:

```go
func TestEncodeDPIReportProducesCompleteValidatedReport(t *testing.T) {
	...
	report, err := EncodeDPIReport(config)
	if err != nil { t.Fatalf("EncodeDPIReport() error = %v", err) }
	if len(report) != 52 { t.Fatalf("report length = %d, want 52", len(report)) }
	if report[0] != 0x04 || report[1] != 0x38 || report[24] != 4 { ... }
}
```
