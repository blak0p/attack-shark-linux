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

	backend := hidlinux.NewGousbBackend()
	adapter := hidlinux.NewGousbAdapter(backend)
	service, passive := composeDesktopService(filepath.Join(dataDir, "attack-shark-linux"), adapter)
	listenCtx, stopListening := context.WithCancel(context.Background())
	app := application.New(application.Options{
		Name: "Attack Shark X6 Configurator",
		OnShutdown: func() {
			stopListening()
			if err := backend.Close(); err != nil {
				log.Printf("close USB context: %v", err)
			}
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

// composeDesktopService wires the always-on status listener through HidrawBackend
// (kernel-backed /dev/hidraw, zero USB claims, no mouse-lock) and leaves the
// explicit DPI Apply on the gousb adapter, which owns the only remaining claim.
func composeDesktopService(dataDir string, command *hidlinux.Adapter) (*desktop.Service, *x6.Service) {
	store := configstore.New(
		filepath.Join(dataDir, "applied-dpi.json"),
		filepath.Join(dataDir, "factory-defaults.json"),
	)
	status := x6.NewService(hidlinux.NewHidrawBackend())
	writer := x6.NewCommandService(command)
	return desktop.Compose(status, writer, store), status
}
