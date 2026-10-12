package desktop

import (
	"context"
	"github.com/blak0p/attack-shark-linux/internal/x6"
	"testing"
)

func TestCapturedResetReconcilesSettingsAndLighting(t *testing.T) {
	s, _ := newResetRuntime(t, t.TempDir(), &resetRecordingCommand{})
	st := s.currentSettingsState()
	st.normalSleep, st.responseTimeMs = 10, 20
	normal, response := 10.0, 20
	st.retryNormal, st.persistedNormal = &normal, &normal
	st.retryResponse, st.persistedResponse = &response, &response
	st.normalPersistence, st.responsePersistence = "failed", "failed"
	s.StageLighting(x6.LightingSelection{Mode: x6.LightingFixed, TemplateID: x6.LightingTemplateFixedGreen})
	if result := s.ResetToFactory(context.Background()); result.Cleanup.State != "success" {
		t.Fatal(result)
	}
	sleep, debounce, lighting := s.GetNormalSleepSnapshot(), s.GetDebounceSnapshot(), s.GetLightingSnapshot()
	if sleep.Pending != 0.5 || sleep.Applied != 0.5 || sleep.Persisted != nil || sleep.RetryAvailable || sleep.Persistence != "" || sleep.Error.Code != "" {
		t.Fatalf("sleep: %#v", sleep)
	}
	if debounce.Desired != 4 || debounce.Applied != 4 || debounce.Persisted != nil || debounce.RetryAvailable || debounce.Persistence != "" || debounce.Error.Code != "" {
		t.Fatalf("debounce: %#v", debounce)
	}
	if lighting.Pending.Mode != 0x50 || lighting.Pending.TemplateID != "static-dpi-factory-reset" || lighting.Applied == nil || *lighting.Applied != lighting.Pending {
		t.Fatalf("lighting: %#v", lighting)
	}
}

func TestCapturedResetOrdinaryDefaultsUnchanged(t *testing.T) {
	if x6.DefaultDPIConfig().ActiveStage != 4 || x6.DefaultLightingSettings().ResponseTimeMs != 8 {
		t.Fatal("ordinary defaults changed")
	}
}
