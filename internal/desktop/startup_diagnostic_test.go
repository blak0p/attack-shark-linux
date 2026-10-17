package desktop

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/blak0p/attack-shark-linux/internal/mouse"
	"github.com/blak0p/attack-shark-linux/internal/transport"
	"github.com/blak0p/attack-shark-linux/internal/x6"
)

func TestStartupDiagnostics(t *testing.T) {
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	defer slog.SetDefault(previous)
	registry, err := mouse.NewProfileRegistry(x6.NewProfile(), x6.NewWiredProfile())
	if err != nil {
		t.Fatal(err)
	}
	service := New(statusFake{status: x6.Status{Connection: "wired"}}, &writerFake{}, appliedStoreFake{}).
		AttachInventory(mouse.NewTargetedService(registry, validatedInventorySource{candidates: []transport.Candidate{{VendorID: 0x1d57, ProductID: 0xfa61, Path: "/private/device", Serial: "private-serial"}}, valid: true}, nil))
	before := service.GetSnapshot()
	status := service.RefreshStatus(context.Background())
	inventory := service.RefreshInventory(context.Background())
	after := service.RefreshStatus(context.Background())
	if inventory.Selected == nil || len(inventory.Devices) != 1 || !inventory.Devices[0].Eligible || status.Connection != "wired" || after.Connection != "wired" || after.Error.Code != "" || after.Pending != before.Pending {
		t.Fatal("wired startup failed through fake seams")
	}
	for _, want := range []string{"operation=RefreshStatus phase=entry", "operation=RefreshStatus phase=status_returned", "operation=RefreshStatus phase=complete error_code=\"\"", "operation=RefreshInventory phase=entry", "operation=RefreshInventory phase=complete error_code=\"\" eligible_count=1 selected_binding_present=true"} {
		if !strings.Contains(logs.String(), want) {
			t.Errorf("missing %q in %s", want, logs.String())
		}
	}
	if strings.Contains(logs.String(), "private") {
		t.Fatal("diagnostics leaked identity/path")
	}
	logs.Reset()
	unavailable := New(statusFake{err: context.DeadlineExceeded}, &writerFake{}, appliedStoreFake{})
	if unavailable.RefreshStatus(context.Background()).Error.Code != DeviceUnavailable || unavailable.RefreshInventory(context.Background()).Error.Code != DeviceUnavailable {
		t.Fatal("error mapping changed")
	}
	if strings.Count(logs.String(), "phase=complete error_code=device_unavailable") != 2 {
		t.Fatalf("missing error completion: %s", logs.String())
	}
}
