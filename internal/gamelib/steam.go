package gamelib

import (
	"os"
	"path/filepath"
	"strings"
)

func scanSteam(opts Options) []Game {
	var games []Game
	for _, lib := range steamLibraries(opts) {
		entries, err := os.ReadDir(filepath.Join(lib, "steamapps"))
		if err != nil {
			continue
		}
		for _, e := range entries {
			name := e.Name()
			if !strings.HasPrefix(name, "appmanifest_") || !strings.HasSuffix(name, ".acf") {
				continue
			}
			if g, ok := steamGame(lib, filepath.Join(lib, "steamapps", name)); ok {
				games = append(games, g)
			}
		}
	}
	return games
}

func steamGame(lib, manifestPath string) (Game, bool) {
	data, err := readSmallFile(manifestPath)
	if err != nil {
		return Game{}, false
	}
	root, err := parseVDF(string(data))
	if err != nil {
		return Game{}, false
	}
	state := root.node("appstate")
	appid, name, dir := state.str("appid"), state.str("name"), state.str("installdir")
	if name == "" || dir == "" || dir == "." || dir == ".." || strings.ContainsAny(dir, `/\`) {
		return Game{}, false
	}
	if isSteamTool(appid, name) {
		return Game{}, false
	}
	folder := filepath.Join(lib, "steamapps", "common", dir)
	if info, err := os.Stat(folder); err != nil || !info.IsDir() {
		return Game{}, false
	}
	return Game{Name: name, Source: "Steam", Folder: folder}, true
}

// isSteamTool reports Steam entries that are runtimes or shared files, not games.
func isSteamTool(appid, name string) bool {
	if appid == "228980" || appid == "480" {
		return true
	}
	for _, prefix := range []string{"Proton", "Steam Linux Runtime", "Steamworks Common Redistributables"} {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

// steamLibraries returns every distinct Steam library directory: the known
// install roots plus the extra libraries listed in their libraryfolders.vdf.
func steamLibraries(opts Options) []string {
	roots := append([]string{}, opts.SteamRoots...)
	if opts.Home != "" {
		roots = append(roots,
			filepath.Join(opts.Home, ".local", "share", "Steam"),
			filepath.Join(opts.Home, ".steam", "steam"),
			filepath.Join(opts.Home, ".var", "app", "com.valvesoftware.Steam", ".local", "share", "Steam"),
		)
	}
	seen := map[string]bool{}
	var libs []string
	add := func(path string) bool {
		path = canonical(path)
		if path == "" || seen[path] {
			return false
		}
		seen[path] = true
		libs = append(libs, path)
		return true
	}
	for _, root := range roots {
		if info, err := os.Stat(filepath.Join(root, "steamapps")); err != nil || !info.IsDir() {
			continue
		}
		add(root)
		data, err := readSmallFile(filepath.Join(root, "steamapps", "libraryfolders.vdf"))
		if err != nil {
			continue
		}
		tree, err := parseVDF(string(data))
		if err != nil {
			continue
		}
		for _, v := range tree.node("libraryfolders") {
			switch entry := v.(type) {
			case vdfNode:
				if p := entry.str("path"); p != "" {
					add(p)
				}
			case string: // older format: the value is the path itself
				if filepath.IsAbs(entry) {
					add(entry)
				}
			}
		}
	}
	return libs
}

func canonical(path string) string {
	if path == "" {
		return ""
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	return filepath.Clean(path)
}
