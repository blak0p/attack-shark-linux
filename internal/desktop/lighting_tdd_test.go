package desktop

import (
	"context"
	"errors"
	"testing"

	"github.com/blak0p/attack-shark-linux/internal/mouse"
	"github.com/blak0p/attack-shark-linux/internal/transport"
	"github.com/blak0p/attack-shark-linux/internal/x6"
)

func TestLightingStageUsesPerDevicePendingStateWithoutIO(t *testing.T) {
	command := &lightingCommandFake{}
	service := newLightingService(t, command)

	initial := service.GetLightingSnapshot()
	if initial.Pending != (x6.LightingSelection{Mode: x6.LightingFixed, TemplateID: x6.LightingTemplateFixedGreen}) || initial.Applied != nil {
		t.Fatalf("initial lighting snapshot = %#v; want fixed-green pending and no applied state", initial)
	}
	staged := service.StageLighting(x6.LightingSelection{Mode: x6.LightingBreathingDPI, TemplateID: x6.LightingTemplateBreathingDPIThree})
	if staged.Pending.TemplateID != x6.LightingTemplateBreathingDPIThree || staged.Revision != 1 || command.calls != 0 {
		t.Fatalf("staged lighting = %#v, writes=%d; want pending revision and zero I/O", staged, command.calls)
	}
}

func TestLightingApplyWritesOnceAndCommitsOnlyAfterAcknowledgement(t *testing.T) {
	command := &lightingCommandFake{}
	service := newLightingService(t, command)
	selection := x6.LightingSelection{Mode: x6.LightingColorBreathing, TemplateID: x6.LightingTemplateColorBreathingOne}
	service.StageLighting(selection)

	got := service.ApplyLighting()
	if command.calls != 1 || got.Applied == nil || *got.Applied != selection || got.Firmware != "success" || got.Error.Code != "" {
		t.Fatalf("ApplyLighting() = %#v, writes=%d; want one acknowledged applied selection", got, command.calls)
	}
}

func TestLightingApplyFailureAndRevisionChangeDoNotCommit(t *testing.T) {
	t.Run("failure remains visible", func(t *testing.T) {
		command := &lightingCommandFake{err: errors.New("ack timeout")}
		service := newLightingService(t, command)
		service.StageLighting(x6.LightingSelection{Mode: x6.LightingNeon, TemplateID: x6.LightingTemplateNeonOne})

		got := service.ApplyLighting()
		if command.calls != 1 || got.Applied != nil || got.Firmware != "failed" || got.Error.Code != ApplyFailed {
			t.Fatalf("failed ApplyLighting() = %#v, writes=%d; want visible failure without applied commit", got, command.calls)
		}
	})

	t.Run("changed revision cannot commit", func(t *testing.T) {
		command := &lightingCommandFake{started: make(chan struct{}), release: make(chan struct{})}
		service := newLightingService(t, command)
		service.StageLighting(x6.LightingSelection{Mode: x6.LightingNeon, TemplateID: x6.LightingTemplateNeonOne})
		result := make(chan LightingSnapshot, 1)
		go func() { result <- service.ApplyLighting() }()
		<-command.started
		service.StageLighting(x6.LightingSelection{Mode: x6.LightingBreathingDPI, TemplateID: x6.LightingTemplateBreathingDPIOne})
		close(command.release)
		got := <-result
		if command.calls != 1 || got.Applied != nil || got.Error.Code != ApplyFailed {
			t.Fatalf("stale ApplyLighting() = %#v, writes=%d; want rejected applied commit", got, command.calls)
		}
	})
}

func newLightingService(t *testing.T, command *lightingCommandFake) *Service {
	t.Helper()
	registry, err := mouse.NewProfileRegistry(x6.NewProfile())
	if err != nil {
		t.Fatal(err)
	}
	candidate := transport.Candidate{VendorID: 0x1D57, ProductID: 0xFA60, Serial: "lighting", Path: "/dev/hidraw-lighting"}
	service := New(statusFake{}, &writerFake{}, appliedStoreFake{applied: x6.DefaultDPIConfig()}).
		AttachInventory(mouse.NewTargetedService(registry, inventorySourceFake{candidates: []transport.Candidate{candidate}}, command))
	service.RefreshInventory(context.Background())
	return service
}

type lightingCommandFake struct {
	calls   int
	err     error
	started chan struct{}
	release chan struct{}
}

func (f *lightingCommandFake) SendAndAwaitBound(_ context.Context, _ mouse.Binding, _ []byte, continueReading func([]byte) bool) error {
	f.calls++
	if f.started != nil {
		close(f.started)
		<-f.release
	}
	if f.err != nil {
		return f.err
	}
	continueReading([]byte{0x03, 0x10, 0x50, 0x00, 0x05})
	return nil
}
