package desktop

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/blak0p/attack-shark-linux/internal/device"
	"github.com/blak0p/attack-shark-linux/internal/mouse"
	"github.com/blak0p/attack-shark-linux/internal/transport"
)

type stageLEDFake struct {
	reports      [][]byte
	failLighting bool
}

func (f *stageLEDFake) SendAndAwaitBound(_ context.Context, _ mouse.Binding, report []byte, continueReading func([]byte) bool) error {
	f.reports = append(f.reports, append([]byte(nil), report...))
	if report[0] == 0x05 && f.failLighting {
		return errors.New("ack timeout")
	}
	continueReading([]byte{0x03, 0x10, 0x50, 0x00, report[0]})
	return nil
}

func newStageLEDService(t *testing.T, fake *stageLEDFake) *Service {
	t.Helper()
	registry, err := mouse.NewProfileRegistry(newStubProfile())
	if err != nil {
		t.Fatal(err)
	}
	candidate := transport.Candidate{VendorID: 0x1D57, ProductID: 0xFA60, Serial: "stage-led", Path: "/dev/hidraw-stage-led"}
	service := New(testModel(), statusFake{}, &writerFake{}, appliedStoreFake{applied: device.DefaultDPIConfig()}).
		AttachInventory(mouse.NewTargetedService(registry, inventorySourceFake{candidates: []transport.Candidate{candidate}}, fake))
	service.RefreshInventory(context.Background())
	return service
}

func stageChange(service *Service, stage byte) {
	pending := service.GetSnapshot().Pending
	pending.ActiveStage = stage
	service.StageDPI(pending)
}

func TestStageChangeResendsLightingOnlyForDPIFollowingModes(t *testing.T) {
	for _, tc := range []struct {
		name      string
		selection device.LightingSelection
	}{
		{"static dpi", device.LightingSelection{Mode: device.LightingStaticDPI, TemplateID: device.LightingTemplateStaticDPIDefault}},
		{"breathing dpi", device.LightingSelection{Mode: device.LightingBreathingDPI, TemplateID: device.LightingTemplateBreathingDPIOne}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fake := &stageLEDFake{}
			service := newStageLEDService(t, fake)
			service.StageLighting(tc.selection)
			service.ApplyLighting()
			lightingReport := fake.reports[0]
			fake.reports = nil

			stageChange(service, 3)
			if got := service.ApplyDPI(context.Background()); got.Firmware != "success" {
				t.Fatalf("ApplyDPI() = %#v", got)
			}
			if len(fake.reports) != 2 || fake.reports[0][0] != 0x04 || !bytes.Equal(fake.reports[1], lightingReport) {
				t.Fatalf("reports = %x; want 0x04 then the lighting report %x", fake.reports, lightingReport)
			}
		})
	}
}

func TestStageChangeDoesNotResendLightingForFixed(t *testing.T) {
	fake := &stageLEDFake{}
	service := newStageLEDService(t, fake)
	service.ApplyLighting()
	fake.reports = nil
	stageChange(service, 3)
	service.ApplyDPI(context.Background())
	if len(fake.reports) != 1 || fake.reports[0][0] != 0x04 {
		t.Fatalf("reports = %x; want only 0x04", fake.reports)
	}
}

func TestStageChangeWithDefaultPendingLightingWritesOnlyDPI(t *testing.T) {
	fake := &stageLEDFake{}
	service := newStageLEDService(t, fake)
	stageChange(service, 3)
	service.ApplyDPI(context.Background())
	if len(fake.reports) != 1 || fake.reports[0][0] != 0x04 {
		t.Fatalf("reports = %x; want only 0x04", fake.reports)
	}
}

func TestDPIValueChangeWithoutStageChangeDoesNotResendLighting(t *testing.T) {
	fake := &stageLEDFake{}
	service := newStageLEDService(t, fake)
	service.StageLighting(device.LightingSelection{Mode: device.LightingStaticDPI, TemplateID: device.LightingTemplateStaticDPIDefault})
	service.ApplyLighting()
	fake.reports = nil
	pending := service.GetSnapshot().Pending
	pending.DPI[0] = 1600
	service.StageDPI(pending)
	service.ApplyDPI(context.Background())
	if len(fake.reports) != 1 || fake.reports[0][0] != 0x04 {
		t.Fatalf("reports = %x; want only 0x04", fake.reports)
	}
}

func TestFailedLightingResendKeepsDPIApplySuccessful(t *testing.T) {
	fake := &stageLEDFake{}
	service := newStageLEDService(t, fake)
	service.StageLighting(device.LightingSelection{Mode: device.LightingStaticDPI, TemplateID: device.LightingTemplateStaticDPIDefault})
	service.ApplyLighting()
	before := service.GetLightingSnapshot()
	fake.reports, fake.failLighting = nil, true

	stageChange(service, 3)
	got := service.ApplyDPI(context.Background())
	if got.Firmware != "success" || got.Persistence != "success" || got.Error.Code != "" {
		t.Fatalf("ApplyDPI() = %#v; want success despite lighting failure", got)
	}
	if len(fake.reports) != 2 || fake.reports[1][0] != 0x05 {
		t.Fatalf("reports = %x; want 0x04 then failed 0x05", fake.reports)
	}
	after := service.GetLightingSnapshot()
	if after.Revision != before.Revision || after.Pending != before.Pending || after.Firmware != before.Firmware || after.Applied == nil || *after.Applied != *before.Applied || after.Error != before.Error {
		t.Fatalf("lighting snapshot changed: before %#v after %#v", before, after)
	}
}

func TestActiveStageColorChangeRepaintsTheLED(t *testing.T) {
	fake := &stageLEDFake{}
	service := newStageLEDService(t, fake)
	service.StageLighting(device.LightingSelection{Mode: device.LightingStaticDPI, TemplateID: device.LightingTemplateStaticDPIDefault})
	service.ApplyLighting()
	lightingReport := fake.reports[0]
	fake.reports = nil

	pending := service.GetSnapshot().Pending
	pending.Colors[pending.ActiveStage-1] = [3]byte{0x12, 0x34, 0x56}
	service.StageDPI(pending)
	if got := service.ApplyDPI(context.Background()); got.Firmware != "success" {
		t.Fatalf("ApplyDPI() = %#v", got)
	}
	if len(fake.reports) != 2 || fake.reports[0][0] != 0x04 || !bytes.Equal(fake.reports[1], lightingReport) {
		t.Fatalf("reports = %x; want 0x04 then the lighting report %x", fake.reports, lightingReport)
	}
}

func TestAppStageChangeAfterPhysicalPressMovesTheSelectedStage(t *testing.T) {
	fake := &stageLEDFake{}
	service := newStageLEDService(t, fake)
	service.StageLighting(device.LightingSelection{Mode: device.LightingStaticDPI, TemplateID: device.LightingTemplateStaticDPIDefault})
	service.ApplyLighting()
	lightingReport := fake.reports[0]
	selected := service.RefreshInventory(context.Background()).Selected
	service.handleAttributedStatusEvent(StatusEvent{ID: selected.ID, Path: selected.Path, InventoryRevision: selected.InventoryRevision, ActiveStage: intPtr(3)})
	fake.reports = nil

	stageChange(service, 4)
	got := service.ApplyDPI(context.Background())
	if got.ObservedStage == nil || *got.ObservedStage != 4 || got.ObservedDPI == nil || *got.ObservedDPI != got.Applied.DPI[3] {
		t.Fatalf("observed stage = %v, DPI = %v; want 4 and %d", deref(got.ObservedStage), deref(got.ObservedDPI), got.Applied.DPI[3])
	}
	if len(fake.reports) != 2 || fake.reports[0][0] != 0x04 || !bytes.Equal(fake.reports[1], lightingReport) {
		t.Fatalf("reports = %x; want 0x04 then the lighting report %x", fake.reports, lightingReport)
	}
}

func TestAppStageChangeBackToStaleAppliedStageRepaintsTheLED(t *testing.T) {
	fake := &stageLEDFake{}
	service := newStageLEDService(t, fake)
	service.StageLighting(device.LightingSelection{Mode: device.LightingStaticDPI, TemplateID: device.LightingTemplateStaticDPIDefault})
	service.ApplyLighting()
	applied := service.GetSnapshot().Applied.ActiveStage
	selected := service.RefreshInventory(context.Background()).Selected
	other := 1
	if applied == 1 {
		other = 2
	}
	service.handleAttributedStatusEvent(StatusEvent{ID: selected.ID, Path: selected.Path, InventoryRevision: selected.InventoryRevision, ActiveStage: intPtr(other)})
	fake.reports = nil

	stageChange(service, applied)
	service.ApplyDPI(context.Background())
	if len(fake.reports) != 2 || fake.reports[1][0] != 0x05 {
		t.Fatalf("reports = %x; want 0x04 then a lighting repaint", fake.reports)
	}
}

func deref(value *int) any {
	if value == nil {
		return nil
	}
	return *value
}
