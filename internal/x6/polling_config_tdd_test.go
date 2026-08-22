package x6

import (
	"encoding/json"
	"testing"
)

func TestDeviceConfigDefaultsMissingPollingAndPreservesDPI(t *testing.T) {
	legacy := DefaultDPIConfig()
	legacy.DPI[0] = 1600
	contents, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	var got DeviceConfig
	if err := json.Unmarshal(contents, &got); err != nil {
		t.Fatal(err)
	}
	if got.PollingRate != PollingRate1000 || got.DPI[0] != 1600 {
		t.Fatalf("legacy config = %#v; want 1000 Hz and unchanged DPI", got)
	}
}
