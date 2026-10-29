// Package gamelib lists the games and apps installed on this machine so a
// profile rule can be chosen by name instead of typing an executable.
//
// Steam and Heroic are read from their own registries (appmanifest files and
// installed.json); other apps come from .desktop entries. Everything is
// read-only, and a broken or missing file only removes its own entries.
package gamelib

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Game is one installed game or app. Games from a store have a Folder; apps
// from a .desktop entry have an Executable name instead.
type Game struct {
	Name       string
	Source     string // "Steam", "Heroic" or "App"
	Folder     string
	Executable string
	// IsGame is set for apps whose .desktop Categories include Game, so the
	// picker can show games and emulators before ordinary apps.
	IsGame bool
}

// Options names the places to look. Nothing outside them is read.
type Options struct {
	Home       string
	DataDirs   []string // XDG data dirs; their "applications" folders are scanned
	SteamRoots []string // extra Steam install roots
}

// DefaultOptions uses the current user's home and the XDG data dirs.
func DefaultOptions() Options {
	home, _ := os.UserHomeDir()
	dataDirs := filepath.SplitList(os.Getenv("XDG_DATA_DIRS"))
	if len(dataDirs) == 0 {
		dataDirs = []string{"/usr/local/share", "/usr/share"}
	}
	return Options{
		Home:     home,
		DataDirs: append(dataDirs, "/var/lib/flatpak/exports/share"),
	}
}

// Scan returns the installed games and apps sorted by name, never nil.
func Scan(opts Options) []Game {
	all := append(scanSteam(opts), scanHeroic(opts)...)
	all = append(all, scanDesktop(opts)...)
	sort.SliceStable(all, func(i, j int) bool {
		a, b := strings.ToLower(all[i].Name), strings.ToLower(all[j].Name)
		if a != b {
			return a < b
		}
		return all[i].Source < all[j].Source
	})
	seen := map[string]bool{}
	games := []Game{}
	for _, g := range all {
		key := "folder:" + g.Folder
		if g.Folder == "" {
			key = "exe:" + g.Executable
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		games = append(games, g)
	}
	return games
}
