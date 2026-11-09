package desktop

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/blak0p/attack-shark-linux/internal/mouse"
	"github.com/blak0p/attack-shark-linux/internal/transport"
	"github.com/blak0p/attack-shark-linux/internal/x6"
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
	registry, err := mouse.NewProfileRegistry(x6.NewProfile())
	if err != nil {
		t.Fatal(err)
	}
	candidate := transport.Candidate{VendorID: 0x1D57, ProductID: 0xFA60, Serial: "stage-led", Path: "/dev/hidraw-stage-led"}
	service := New(statusFake{}, &writerFake{}, appliedStoreFake{applied: x6.DefaultDPIConfig()}).
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
		selection x6.LightingSelection
	}{
		{"static dpi", x6.LightingSelection{Mode: x6.LightingStaticDPI, TemplateID: x6.LightingTemplateStaticDPIDefault}},
		{"breathing dpi", x6.LightingSelection{Mode: x6.LightingBreathingDPI, TemplateID: x6.LightingTemplateBreathingDPIOne}},
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
	service.StageLighting(x6.LightingSelection{Mode: x6.LightingStaticDPI, TemplateID: x6.LightingTemplateStaticDPIDefault})
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
	service.StageLighting(x6.LightingSelection{Mode: x6.LightingStaticDPI, TemplateID: x6.LightingTemplateStaticDPIDefault})
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
