package desktop

import (
	"testing"

	"github.com/blak0p/attack-shark-linux/internal/device"
	"github.com/blak0p/attack-shark-linux/internal/mouse"
)

// seamModel overrides the rules the service must take from the injected model.
type seamModel struct {
	stubModel
	dpiValidated int
}

func (m *seamModel) ValidateResponseTime(ms int) error {
	if ms != 5 {
		return errSeam
	}
	return nil
}

func (m *seamModel) LightingEffects() []device.LightingEffect {
	return []device.LightingEffect{{Label: "model catalog"}}
}

func (m *seamModel) DPIOperation() mouse.CommandOperation {
	return countingOperation{CommandOperation: m.stubModel.DPIOperation(), count: &m.dpiValidated}
}

type countingOperation struct {
	mouse.CommandOperation
	count *int
}

func (o countingOperation) Validate(value any) error {
	*o.count++
	return o.CommandOperation.Validate(value)
}

type seamError string

func (e seamError) Error() string { return string(e) }

const errSeam = seamError("rejected by injected model")

func TestServiceTakesValidationFromInjectedModel(t *testing.T) {
	service := New(&seamModel{}, statusFake{}, &writerFake{}, appliedStoreFake{applied: device.DefaultDPIConfig()})
	if got := service.StageDebounce(5); got.Error.Code != "" || got.Desired != 5 {
		t.Fatalf("StageDebounce(5) = %+v; want accepted by the injected model", got)
	}
	if got := service.StageDebounce(8); got.Error.Code != InvalidConfiguration {
		t.Fatalf("StageDebounce(8) = %+v; want rejected by the injected model", got)
	}
}

func TestServiceTakesLightingCatalogFromInjectedModel(t *testing.T) {
	service := New(&seamModel{}, statusFake{}, &writerFake{}, appliedStoreFake{applied: device.DefaultDPIConfig()})
	if got := service.GetLightingSnapshot().Effects; len(got) != 1 || got[0].Label != "model catalog" {
		t.Fatalf("Effects = %+v; want the injected model's catalog", got)
	}
}

func TestServiceValidatesDPIThroughInjectedModelOperation(t *testing.T) {
	model := &seamModel{}
	service := New(model, statusFake{}, &writerFake{}, appliedStoreFake{applied: device.DefaultDPIConfig()})
	service.StageDPI(ToDTO(device.DefaultDPIConfig()))
	if model.dpiValidated != 1 {
		t.Fatalf("DPI validations through the model = %d, want 1", model.dpiValidated)
	}
}
