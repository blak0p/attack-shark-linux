package main

import (
	"path/filepath"

	"github.com/blak0p/attack-shark-linux/internal/desktop"
	"github.com/blak0p/attack-shark-linux/internal/macros"
)

// Open once at composition, independently of per-device configuration. Keep
// startup usable if local macro data cannot be loaded; macro methods report it.
func macroLibraryOption(dataDir string) desktop.Option {
	library, err := macros.Open(filepath.Join(dataDir, "macros-v1.json"))
	return desktop.WithMacroLibrary(library, err)
}
