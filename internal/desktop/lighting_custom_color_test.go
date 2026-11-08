package desktop

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/blak0p/attack-shark-linux/internal/attackshark/x6"
)

func TestLightingCustomColorStagesAppliesAndWritesRGB(t *testing.T) {
	command := &lightingCommandFake{}
	service := newLightingService(t, command)
	selection := x6.LightingSelection{Mode: x6.LightingFixed, TemplateID: x6.LightingTemplateFixedGreen, Color: "#FF7F00"}

	staged := service.StageLighting(selection)
	if staged.Pending != selection || staged.Error.Code != "" || command.calls != 0 {
		t.Fatalf("StageLighting(%#v) = %#v, writes=%d; want pending custom color without I/O", selection, staged, command.calls)
	}
	got := service.ApplyLighting()
	if command.calls != 1 || got.Applied == nil || *got.Applied != selection || got.Error.Code != "" {
		t.Fatalf("ApplyLighting() = %#v, writes=%d; want applied custom color", got, command.calls)
	}
	if report := command.reports[0]; len(report) < 9 || !bytes.Equal(report[6:9], []byte{0xff, 0x7f, 0x00}) {
		t.Fatalf("written report = % x; want RGB ff 7f 00 in bytes 6-8", report)
	}
}

func TestLightingInvalidCustomColorIsRejectedWithoutChangingPending(t *testing.T) {
	command := &lightingCommandFake{}
	service := newLightingService(t, command)
	before := service.GetLightingSnapshot()

	got := service.StageLighting(x6.LightingSelection{Mode: x6.LightingNeon, TemplateID: x6.LightingTemplateNeonOne, Color: "#FF0000"})
	if got.Error.Code != InvalidConfiguration || got.Pending != before.Pending || got.Revision != before.Revision || command.calls != 0 {
		t.Fatalf("StageLighting(neon+color) = %#v; want InvalidConfiguration and unchanged pending %#v", got, before.Pending)
	}
}

func TestLightingSelectionJSONOmitsEmptyColorForStoredProfiles(t *testing.T) {
	plain, _ := json.Marshal(x6.LightingSelection{Mode: x6.LightingFixed, TemplateID: x6.LightingTemplateFixedGreen})
	if strings.Contains(string(plain), "Color") {
		t.Fatalf("plain selection JSON = %s; want no Color key", plain)
	}
	var decoded x6.LightingSelection
	custom := x6.LightingSelection{Mode: x6.LightingBreathing, TemplateID: x6.LightingTemplateBreathingGreen, Color: "#123456"}
	encoded, _ := json.Marshal(custom)
	if err := json.Unmarshal(encoded, &decoded); err != nil || decoded != custom {
		t.Fatalf("custom selection round trip = %#v, %v; want %#v", decoded, err, custom)
	}
}
