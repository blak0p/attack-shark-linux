package device

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDeviceConfigKeepsPersistedJSONShape(t *testing.T) {
	contents, err := json.Marshal(DefaultDeviceConfig())
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"pollingRate":1000`, `"normalSleepMinutes":0.5`, `"responseTimeMs":8`, `"StageMask":63`} {
		if !strings.Contains(string(contents), key) {
			t.Errorf("persisted JSON %s lacks %s", contents, key)
		}
	}
	if strings.Contains(string(contents), `"remap"`) {
		t.Errorf("persisted JSON %s must omit an unset remap", contents)
	}
}

func TestDeviceConfigUnmarshalSuppliesDefaultsForOldRecords(t *testing.T) {
	var got DeviceConfig
	if err := json.Unmarshal([]byte(`{"StageMask":63,"ActiveStage":2}`), &got); err != nil {
		t.Fatal(err)
	}
	want := DefaultDeviceConfig()
	want.ActiveStage = 2
	if got != want {
		t.Fatalf("DeviceConfig = %+v, want %+v", got, want)
	}
}
