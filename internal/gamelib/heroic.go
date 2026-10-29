package gamelib

import (
	"encoding/json"
	"os"
	"path/filepath"
)

func heroicConfigDirs(home string) []string {
	if home == "" {
		return nil
	}
	return []string{
		filepath.Join(home, ".config", "heroic"),
		filepath.Join(home, ".var", "app", "com.heroicgameslauncher.hgl", "config", "heroic"),
	}
}

func scanHeroic(opts Options) []Game {
	var games []Game
	for _, dir := range heroicConfigDirs(opts.Home) {
		games = append(games, heroicEpic(dir)...)
		games = append(games, heroicGOG(dir)...)
		games = append(games, heroicSideload(dir)...)
	}
	return games
}

func readJSON(path string, v any) bool {
	data, err := readSmallFile(path)
	if err != nil {
		return false
	}
	return json.Unmarshal(data, v) == nil
}

func installedFolder(name, folder string) (Game, bool) {
	if name == "" || !filepath.IsAbs(folder) {
		return Game{}, false
	}
	if info, err := os.Stat(folder); err != nil || !info.IsDir() {
		return Game{}, false
	}
	return Game{Name: name, Source: "Heroic", Folder: filepath.Clean(folder)}, true
}

// heroicEpic reads legendary's installed.json, verified against real data:
// an object keyed by app name with title and install_path.
func heroicEpic(dir string) []Game {
	var installed map[string]struct {
		Title       string `json:"title"`
		InstallPath string `json:"install_path"`
	}
	if !readJSON(filepath.Join(dir, "legendaryConfig", "legendary", "installed.json"), &installed) {
		return nil
	}
	var games []Game
	for _, e := range installed {
		if g, ok := installedFolder(e.Title, e.InstallPath); ok {
			games = append(games, g)
		}
	}
	return games
}

// heroicGOG reads gog_store/installed.json plus library.json for titles. The
// format is unverified against real data (the GOG files were empty when this
// was written); anything unexpected yields no games.
func heroicGOG(dir string) []Game {
	var installed struct {
		Installed []struct {
			AppName     string `json:"appName"`
			InstallPath string `json:"install_path"`
		} `json:"installed"`
	}
	if !readJSON(filepath.Join(dir, "gog_store", "installed.json"), &installed) {
		return nil
	}
	var library struct {
		Games []struct {
			AppName string `json:"app_name"`
			Title   string `json:"title"`
		} `json:"games"`
	}
	titles := map[string]string{}
	if readJSON(filepath.Join(dir, "gog_store", "library.json"), &library) {
		for _, g := range library.Games {
			titles[g.AppName] = g.Title
		}
	}
	var games []Game
	for _, e := range installed.Installed {
		title := titles[e.AppName]
		if title == "" {
			title = filepath.Base(e.InstallPath)
		}
		if g, ok := installedFolder(title, e.InstallPath); ok {
			games = append(games, g)
		}
	}
	return games
}

// heroicSideload reads sideload_apps/library.json. The format is unverified
// against real data (the file was empty when this was written).
func heroicSideload(dir string) []Game {
	var library struct {
		Games []struct {
			Title       string `json:"title"`
			FolderName  string `json:"folder_name"`
			IsInstalled bool   `json:"is_installed"`
		} `json:"games"`
	}
	if !readJSON(filepath.Join(dir, "sideload_apps", "library.json"), &library) {
		return nil
	}
	var games []Game
	for _, e := range library.Games {
		if !e.IsInstalled {
			continue
		}
		if g, ok := installedFolder(e.Title, e.FolderName); ok {
			games = append(games, g)
		}
	}
	return games
}
