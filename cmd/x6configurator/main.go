package main

import (
	"context"
	"embed"
	"errors"
	"log"
	"os"
	"path/filepath"

	"github.com/alejandro/attack-shark-linux/internal/configstore"
	"github.com/alejandro/attack-shark-linux/internal/desktop"
	"github.com/alejandro/attack-shark-linux/internal/hidlinux"
	"github.com/alejandro/attack-shark-linux/internal/mouse"
	"github.com/alejandro/attack-shark-linux/internal/transport"
	"github.com/alejandro/attack-shark-linux/internal/x6"
	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed frontend/dist
var assets embed.FS

// wailsEventSink bridges the desktop service to the Wails event manager so the
// frontend can subscribe to live status updates.
type wailsEventSink struct{ app *application.App }

func (s wailsEventSink) Emit(name string, payload any) { s.app.Event.Emit(name, payload) }

type x6InventoryBackend interface {
	Enumerate(context.Context, transport.Match) ([]transport.Candidate, error)
	mouse.ProfileValidator
}

// x6InventorySource preserves the serial-bearing candidates discovered by hidraw.
type x6InventorySource struct{ backend x6InventoryBackend }

func (s x6InventorySource) Enumerate(ctx context.Context) ([]transport.Candidate, error) {
	return s.backend.Enumerate(ctx, x6.NewProfile().Match())
}

func (s x6InventorySource) ProfileValid(ctx context.Context, candidate transport.Candidate, facts mouse.HIDFacts) bool {
	return s.backend.ProfileValid(ctx, candidate, facts)
}

func main() {
	dataDir, err := os.UserConfigDir()
	if err != nil {
		log.Fatalf("locate user configuration directory: %v", err)
	}

	backend := hidlinux.NewHidrawBackend()
	service, passive := composeDesktopService(filepath.Join(dataDir, "attack-shark-linux"), backend)
	listenCtx, stopListening := context.WithCancel(context.Background())
	app := application.New(application.Options{
		Name: "Attack Shark X6 Configurator",
		OnShutdown: func() {
			stopListening()
		},
		Services: []application.Service{
			application.NewService(service),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
	})

	service.AttachListener(passive, wailsEventSink{app: app})
	service.StartListener(listenCtx)

	window := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:  "Attack Shark X6 Configurator",
		Width:  1024,
		Height: 768,
	})
	window.SetURL("/")
	window.Center()

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

func newDesktopService(dataDir string) *desktop.Service {
	service, _ := composeDesktopService(dataDir, nil)
	return service
}

// composeDesktopService shares one HidrawBackend between status and Apply.
// Both operations use the validated vendor hidraw node; no USB interface is
// claimed, detached, reset, rebound, or otherwise taken from the kernel.
func composeDesktopService(dataDir string, backend *hidlinux.HidrawBackend) (*desktop.Service, *x6.Service) {
	if backend == nil {
		backend = hidlinux.NewHidrawBackend()
	}
	status := x6.NewService(backend)
	writer := x6.NewCommandService(backend)
	registry, _ := mouse.NewProfileRegistry(x6.NewProfile())
	service := composeDesktopServiceWithTargeted(dataDir, status, writer, registry, x6InventorySource{backend: backend}, backend)
	return service, status
}

func composeDesktopServiceWithTargeted(dataDir string, status desktop.StatusReader, writer desktop.DPIWriter, registry *mouse.ProfileRegistry, source mouse.InventorySource, command mouse.TargetedCommand) *desktop.Service {
	store := configstore.New(
		filepath.Join(dataDir, "applied-dpi.json"),
		filepath.Join(dataDir, "factory-defaults.json"),
	)
	inventory := mouse.NewTargetedService(registry, source, command)
	deviceStore := configstore.NewDeviceStore(filepath.Join(dataDir, "devices-v2.json"))
	migrate := func(binding mouse.Binding) error {
		_, err := deviceStore.MigrateV1(
			filepath.Join(dataDir, "applied-dpi.json"), filepath.Join(dataDir, "factory-defaults.json"),
			[]configstore.MigrationTarget{{ID: binding.ID, Profile: binding.ProfileID, Model: "Attack Shark X6"}}, false,
		)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	loadDevice := func(binding mouse.Binding) (x6.DPIConfig, error) {
		var config x6.DPIConfig
		err := deviceStore.Load(binding.ID, binding.ProfileID, &config)
		return config, err
	}
	saveDevice := func(binding mouse.Binding, config x6.DPIConfig) error {
		return deviceStore.Save(binding.ID, binding.ProfileID, "Attack Shark X6", 1, config)
	}
	return desktop.Compose(status, writer, store).AttachInventory(inventory).AttachMigrator(migrate).AttachDevicePersistence(loadDevice, saveDevice)
}
