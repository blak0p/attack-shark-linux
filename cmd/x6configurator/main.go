package main

import (
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

func main() {
	dataDir, err := os.UserConfigDir()
	if err != nil {
		log.Fatalf("locate user configuration directory: %v", err)
	}

	backend := hidlinux.NewGousbBackend()
	adapter := hidlinux.NewGousbAdapter(backend)
	service := composeDesktopService(filepath.Join(dataDir, "attack-shark-linux"), adapter)
	app := application.New(application.Options{
		Name: "Attack Shark X6 Configurator",
		OnShutdown: func() {
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
	return composeDesktopService(dataDir, hidlinux.NewStatusAdapter(nil, nil, nil))
}

func composeDesktopService(dataDir string, adapter *hidlinux.Adapter) *desktop.Service {
	store := configstore.New(
		filepath.Join(dataDir, "applied-dpi.json"),
		filepath.Join(dataDir, "factory-defaults.json"),
	)
	status, writer := x6.NewDesktopServices(adapter)
	return desktop.Compose(status, writer, store)
}
