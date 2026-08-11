package main

import (
	"context"
	"embed"
	"log"
	"os"
	"path/filepath"

	"github.com/alejandro/attack-shark-linux/internal/configstore"
	"github.com/alejandro/attack-shark-linux/internal/desktop"
	"github.com/alejandro/attack-shark-linux/internal/hidlinux"
	"github.com/alejandro/attack-shark-linux/internal/x6"
	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed frontend/dist
var assets embed.FS

// wailsEventSink bridges the desktop service to the Wails event manager so the
// frontend can subscribe to live status updates.
type wailsEventSink struct{ app *application.App }

func (s wailsEventSink) Emit(name string, payload any) { s.app.Event.Emit(name, payload) }

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
	store := configstore.New(
		filepath.Join(dataDir, "applied-dpi.json"),
		filepath.Join(dataDir, "factory-defaults.json"),
	)
	if backend == nil {
		backend = hidlinux.NewHidrawBackend()
	}
	status := x6.NewService(backend)
	writer := x6.NewCommandService(backend)
	return desktop.Compose(status, writer, store), status
}
