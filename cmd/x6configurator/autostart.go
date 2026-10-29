package main

import (
	"github.com/blak0p/attack-shark-linux/internal/autostart"
	"github.com/blak0p/attack-shark-linux/internal/desktop"
)

// attachAutostart lets the Games panel switch the login start on and off. The
// entry points at the launcher path the panel already shows, so an AppImage
// keeps starting from the file the user runs.
func attachAutostart(service *desktop.Service, launcher func() (string, error)) {
	service.AttachAutostart(
		func() bool {
			dir, err := autostart.Dir()
			return err == nil && autostart.Enabled(dir)
		},
		func(enabled bool) error {
			dir, err := autostart.Dir()
			if err != nil {
				return err
			}
			path := ""
			if enabled {
				if path, err = launcher(); err != nil {
					return err
				}
			}
			return autostart.Set(dir, path, enabled)
		},
	)
}
