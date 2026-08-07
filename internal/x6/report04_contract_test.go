package x6

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

type report04Fixture struct {
	FeatureReportLength int    `json:"featureReportLength"`
	LengthByte          int    `json:"lengthByte"`
	Checksum            string `json:"checksum"`
	Padding             string `json:"padding"`
}

func TestReport04CaptureContract(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "captures", "0x04-config", "report04-contract.json"))
	if err != nil {
		t.Fatal(err)
	}

	var fixture report04Fixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}

	for _, tt := range []struct {
		name string
		got  interface{}
		want interface{}
	}{
		{"feature transfer length", fixture.FeatureReportLength, 52},
		{"protocol length byte", fixture.LengthByte, 0x38},
		{"checksum", fixture.Checksum, "sum(bytes[3:50]) big-endian at [50:52]"},
		{"trailing padding", fixture.Padding, "none"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Fatalf("%s = %v, want %v", tt.name, tt.got, tt.want)
			}
		})
	}
}
