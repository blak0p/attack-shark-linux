// Package device holds the model-neutral settings domain the application edits
// and persists, and the Model seam through which generic code reaches a
// per-model driver.
package device

import "encoding/json"

// DPIConfig represents the entire documented 0x04 configuration report.
type DPIConfig struct {
	AngleControl, RippleControl bool
	StageMask, LiftDistance     byte
	DPI                         [8]int
	ActiveStage                 byte
	Colors                      [8][3]byte
}

func DefaultDPIConfig() DPIConfig {
	// Factory template dictated by the user (docs/config-baseline.md): six stages
	// ending at the sensor max. Stages 7-8 hold the minimum encodable value but
	// are masked off by StageMask 0x3f. Colors match the factory dwords
	// 0xff/0xff00/0xff0000/0xffff/0xffff00/0xff00ff/0x40ff/0xffffff.
	return DPIConfig{StageMask: 0x3f, LiftDistance: 1, DPI: [8]int{800, 1200, 1600, 3200, 5600, 26000, 50, 50}, ActiveStage: 4, Colors: [8][3]byte{{0xff, 0, 0}, {0, 0xff, 0}, {0, 0, 0xff}, {0xff, 0xff, 0}, {0, 0xff, 0xff}, {0xff, 0, 0xff}, {0xff, 0x40, 0}, {0xff, 0xff, 0xff}}}
}

// DocumentedResetDPIConfig returns the documented factory baseline for report 0x04.
func DocumentedResetDPIConfig() DPIConfig {
	config := DefaultDPIConfig()
	config.ActiveStage = 2
	return config
}

// PollingRate is the typed polling-rate selection in hertz.
type PollingRate uint16

const (
	PollingRate125  PollingRate = 125
	PollingRate250  PollingRate = 250
	PollingRate500  PollingRate = 500
	PollingRate1000 PollingRate = 1000
)

const (
	NormalSleepMinimumMinutes = 0.5
	NormalSleepMaximumMinutes = 60.0
)

const (
	ResponseTimeFactoryMs = 8
	ResponseTimeMinMs     = 2
	ResponseTimeMaxMs     = 25
	ResponseTimeStepMs    = 1
)

// DeviceConfig is the durable X6 configuration. DPI remains flattened for
// backward-compatible reads of existing DPI-only records.
type DeviceConfig struct {
	DPIConfig
	PollingRate        PollingRate  `json:"pollingRate"`
	NormalSleepMinutes float64      `json:"normalSleepMinutes"`
	ResponseTimeMs     int          `json:"responseTimeMs"`
	Remap              *RemapConfig `json:"remap,omitempty"`
}

func DefaultDeviceConfig() DeviceConfig {
	return DeviceConfig{DPIConfig: DefaultDPIConfig(), PollingRate: PollingRate1000, NormalSleepMinutes: NormalSleepMinimumMinutes, ResponseTimeMs: ResponseTimeFactoryMs}
}

// UnmarshalJSON keeps old direct DPI records valid while supplying the polling
// factory default when their new field is absent.
func (c *DeviceConfig) UnmarshalJSON(contents []byte) error {
	*c = DefaultDeviceConfig()
	type plain DeviceConfig
	return json.Unmarshal(contents, (*plain)(c))
}
