package desktop

import (
	"context"
	"testing"

	"github.com/alejandro/attack-shark-linux/internal/x6"
)

func TestComposeWiresX6AdaptersIntoDesktopService(t *testing.T) {
	config := x6.DefaultDPIConfig()
	writer := &fakeApply{}
	service := Compose(fakeStatus{status: x6.Status{Connection: x6.Wired}}, writer, fakeStore{applied: config})
	if got := service.RefreshStatus(context.Background()); got.Connection != "wired" || got.Applied.DPI[0] != 800 {
		t.Fatalf("composition snapshot = %#v, want wired restored desktop state", got)
	}
}
