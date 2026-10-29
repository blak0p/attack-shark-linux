package main

import (
	"os"
	"path/filepath"

	"github.com/blak0p/attack-shark-linux/internal/desktop"
	"github.com/blak0p/attack-shark-linux/internal/gamelib"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// gamePickerOptions is the pure description of the native dialog.
type gamePickerOptions struct {
	title       string
	directories bool
	files       bool
}

func gamePickerOptionsFor() gamePickerOptions {
	return gamePickerOptions{title: "Choose the game folder", directories: true}
}

// newGamePicker opens the native GTK folder chooser. Cancelling returns ("", nil).
func newGamePicker(app *application.App) func() (string, error) {
	return func() (string, error) {
		opts := gamePickerOptionsFor()
		dialog := app.Dialog.OpenFile().
			SetTitle(opts.title).
			CanChooseDirectories(opts.directories).
			CanChooseFiles(opts.files)
		// Parent the chooser to the app window so it stays on top of the app.
		if window := app.Window.Current(); window != nil {
			dialog.AttachToWindow(window)
		}
		return dialog.PromptForSingleSelection()
	}
}

// scanInstalledGames adapts gamelib.Scan to the desktop catalog.
func scanInstalledGames() []desktop.InstalledGame {
	scanned := gamelib.Scan(gamelib.DefaultOptions())
	out := make([]desktop.InstalledGame, 0, len(scanned))
	for _, g := range scanned {
		out = append(out, desktop.InstalledGame{Name: g.Name, Source: g.Source, Folder: g.Folder, Executable: g.Executable, IsGame: g.IsGame})
	}
	return out
}

// chooseLauncherPath picks the path the Games panel should show: the AppImage
// file when APPIMAGE is absolute and exists (symlinks kept), else the running
// executable as returned by the lookup.
func chooseLauncherPath(appImage string, exists func(string) bool, executable func() (string, error)) (string, error) {
	if filepath.IsAbs(appImage) && exists(appImage) {
		return appImage, nil
	}
	return executable()
}

func launcherPath() (string, error) {
	return chooseLauncherPath(os.Getenv("APPIMAGE"), func(p string) bool {
		_, err := os.Stat(p)
		return err == nil
	}, os.Executable)
}
