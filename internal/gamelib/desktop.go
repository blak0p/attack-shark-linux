package gamelib

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var envAssignment = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*=`)

// wrapperExecutables start something else; their name says nothing about the
// process the user will actually run, so entries using them are not listed.
var wrapperExecutables = map[string]bool{
	"steam": true, "bottles-cli": true, "bottles": true, "lutris": true,
	"heroic": true, "sh": true, "bash": true, "xdg-open": true, "wine": true,
	"flatpak": true, "gamescope": true, "gamemoderun": true, "systemd-run": true,
}

func desktopDirs(opts Options) []string {
	var dirs []string
	for _, d := range opts.DataDirs {
		dirs = append(dirs, filepath.Join(d, "applications"))
	}
	if opts.Home != "" {
		dirs = append(dirs,
			filepath.Join(opts.Home, ".local", "share", "applications"),
			filepath.Join(opts.Home, ".local", "share", "flatpak", "exports", "share", "applications"),
		)
	}
	return dirs
}

func scanDesktop(opts Options) []Game {
	var games []Game
	for _, dir := range desktopDirs(opts) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !strings.HasSuffix(e.Name(), ".desktop") {
				continue
			}
			if g, ok := desktopApp(filepath.Join(dir, e.Name())); ok {
				games = append(games, g)
			}
		}
	}
	return games
}

func desktopApp(path string) (Game, bool) {
	data, err := readSmallFile(path)
	if err != nil {
		return Game{}, false
	}
	var name, exec, typ, categories string
	hidden, inEntry := false, false
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[") {
			inEntry = line == "[Desktop Entry]"
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if !inEntry || !found {
			continue
		}
		switch strings.TrimSpace(key) {
		case "Name":
			name = strings.TrimSpace(value)
		case "Exec":
			exec = strings.TrimSpace(value)
		case "Categories":
			categories = strings.TrimSpace(value)
		case "Type":
			typ = strings.TrimSpace(value)
		case "Hidden", "NoDisplay":
			if strings.TrimSpace(value) == "true" {
				hidden = true
			}
		}
	}
	if hidden || typ != "Application" || name == "" {
		return Game{}, false
	}
	exe := executableFromExec(exec)
	if exe == "" || wrapperExecutables[strings.ToLower(exe)] {
		return Game{}, false
	}
	isGame := false
	for _, c := range strings.Split(categories, ";") {
		if c == "Game" {
			isGame = true
		}
	}
	return Game{Name: name, Source: "App", Executable: exe, IsGame: isGame}, true
}

// executableFromExec returns the base name of the program an Exec line starts.
func executableFromExec(exec string) string {
	args := splitExec(exec)
	for len(args) > 0 {
		a := args[0]
		if a == "env" || envAssignment.MatchString(a) {
			args = args[1:]
			continue
		}
		break
	}
	if len(args) == 0 {
		return ""
	}
	exe := filepath.Base(args[0])
	if exe == "flatpak" {
		return flatpakExecutable(args[1:])
	}
	return exe
}

// flatpakExecutable is best effort: the --command value when present, else the
// last segment of the application ID.
func flatpakExecutable(args []string) string {
	if len(args) == 0 || args[0] != "run" {
		return ""
	}
	var id string
	for _, a := range args[1:] {
		if cmd, ok := strings.CutPrefix(a, "--command="); ok {
			return filepath.Base(cmd)
		}
		if id == "" && !strings.HasPrefix(a, "-") && !strings.HasPrefix(a, "%") {
			id = a
		}
	}
	if i := strings.LastIndex(id, "."); i >= 0 {
		return id[i+1:]
	}
	return id
}

// splitExec splits an Exec line on spaces, honouring double quotes, and drops
// field codes such as %f and %U.
func splitExec(exec string) []string {
	var args []string
	var cur strings.Builder
	inQuote, started := false, false
	flush := func() {
		if started {
			if s := cur.String(); !(len(s) == 2 && s[0] == '%') {
				args = append(args, s)
			}
		}
		cur.Reset()
		started = false
	}
	for i := 0; i < len(exec); i++ {
		c := exec[i]
		switch {
		case c == '"':
			inQuote, started = !inQuote, true
		case c == '\\' && inQuote && i+1 < len(exec):
			i++
			cur.WriteByte(exec[i])
		case (c == ' ' || c == '\t') && !inQuote:
			flush()
		default:
			cur.WriteByte(c)
			started = true
		}
	}
	flush()
	return args
}
