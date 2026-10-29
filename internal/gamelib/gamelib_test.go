package gamelib

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mkdir(t *testing.T, path string) string {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func manifest(appid, name, installdir string) string {
	return `"AppState"
{
	"appid"		"` + appid + `"
	"name"		"` + name + `"
	"installdir"		"` + installdir + `"
}
`
}

func names(games []Game) []string {
	out := []string{}
	for _, g := range games {
		out = append(out, g.Name)
	}
	return out
}

// steamFixture mirrors the layout seen on a real machine: a main library under
// the home directory plus a second library on another drive.
func steamFixture(t *testing.T) (home, lib2 string) {
	t.Helper()
	home = t.TempDir()
	lib2 = t.TempDir()
	main := filepath.Join(home, ".local", "share", "Steam")
	write(t, filepath.Join(main, "steamapps", "libraryfolders.vdf"), `"libraryfolders"
{
	"0"
	{
		"path"		"`+main+`"
		"apps"
		{
			"480"		"1906055"
		}
	}
	"1"
	{
		"path"		"`+lib2+`"
	}
}
`)
	write(t, filepath.Join(main, "steamapps", "appmanifest_480.acf"), manifest("480", "Spacewar", "Spacewar"))
	write(t, filepath.Join(main, "steamapps", "appmanifest_228980.acf"), manifest("228980", "Steamworks Common Redistributables", "Steamworks Shared"))
	write(t, filepath.Join(main, "steamapps", "appmanifest_431960.acf"), manifest("431960", "Wallpaper Engine", "wallpaper_engine"))
	mkdir(t, filepath.Join(main, "steamapps", "common", "Spacewar"))
	mkdir(t, filepath.Join(main, "steamapps", "common", "Steamworks Shared"))
	mkdir(t, filepath.Join(main, "steamapps", "common", "wallpaper_engine"))

	sa := filepath.Join(lib2, "steamapps")
	write(t, filepath.Join(sa, "appmanifest_105600.acf"), manifest("105600", "Terraria", "Terraria"))
	write(t, filepath.Join(sa, "appmanifest_3658110.acf"), manifest("3658110", "Proton 10.0", "Proton 10.0"))
	write(t, filepath.Join(sa, "appmanifest_1628350.acf"), manifest("1628350", "Steam Linux Runtime 3.0 (sniper)", "SteamLinuxRuntime_sniper"))
	write(t, filepath.Join(sa, "appmanifest_322170.acf.3448925242.tmp"), manifest("322170", "Geometry Dash", "Geometry Dash"))
	write(t, filepath.Join(sa, "appmanifest_999.acf"), manifest("999", "Ghost Game", "Ghost Game"))
	for _, d := range []string{"Terraria", "Proton 10.0", "SteamLinuxRuntime_sniper", "Geometry Dash"} {
		mkdir(t, filepath.Join(sa, "common", d))
	}
	return home, lib2
}

func TestSteamListsGamesAndSkipsToolsTmpAndMissingFolders(t *testing.T) {
	home, lib2 := steamFixture(t)
	games := Scan(Options{Home: home})
	if got, want := names(games), []string{"Terraria", "Wallpaper Engine"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("names = %v, want %v", got, want)
	}
	terraria := games[0]
	if want := filepath.Join(lib2, "steamapps", "common", "Terraria"); terraria.Folder != want {
		t.Fatalf("folder = %q, want %q", terraria.Folder, want)
	}
	if terraria.Source != "Steam" || terraria.Executable != "" {
		t.Fatalf("game = %+v", terraria)
	}
}

func TestSteamSymlinkedRootIsNotListedTwice(t *testing.T) {
	home, _ := steamFixture(t)
	if err := os.MkdirAll(filepath.Join(home, ".steam"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(home, ".local", "share", "Steam"), filepath.Join(home, ".steam", "steam")); err != nil {
		t.Fatal(err)
	}
	if got, want := names(Scan(Options{Home: home})), []string{"Terraria", "Wallpaper Engine"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("names = %v, want %v", got, want)
	}
}

func TestSteamMalformedAndUnsafeManifestsAreSkipped(t *testing.T) {
	home := t.TempDir()
	main := filepath.Join(home, ".local", "share", "Steam")
	sa := filepath.Join(main, "steamapps")
	write(t, filepath.Join(sa, "libraryfolders.vdf"), `"libraryfolders" { "0" { "path" "`+main+`" } }`)
	write(t, filepath.Join(sa, "appmanifest_1.acf"), `"AppState" { "name" "Broken`)
	write(t, filepath.Join(sa, "appmanifest_2.acf"), manifest("2", "Escape", "../outside"))
	write(t, filepath.Join(sa, "appmanifest_3.acf"), manifest("3", "Nested", "a/b"))
	write(t, filepath.Join(sa, "appmanifest_4.acf"), manifest("4", "", "NoName"))
	write(t, filepath.Join(sa, "appmanifest_5.acf"), manifest("5", "Good", "Good"))
	mkdir(t, filepath.Join(sa, "common", "Good"))
	mkdir(t, filepath.Join(sa, "common", "NoName"))
	mkdir(t, filepath.Join(main, "outside"))
	if got, want := names(Scan(Options{Home: home})), []string{"Good"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("names = %v, want %v", got, want)
	}
}

func TestSteamExtraRootIsScanned(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "steamapps", "libraryfolders.vdf"), `"libraryfolders" { }`)
	write(t, filepath.Join(root, "steamapps", "appmanifest_7.acf"), manifest("7", "Extra", "Extra"))
	mkdir(t, filepath.Join(root, "steamapps", "common", "Extra"))
	games := Scan(Options{Home: t.TempDir(), SteamRoots: []string{root}})
	if got, want := names(games), []string{"Extra"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("names = %v, want %v", got, want)
	}
}

func TestHeroicEpicUsesTitleAndInstallPath(t *testing.T) {
	home := t.TempDir()
	game := mkdir(t, filepath.Join(t.TempDir(), "rocketleague"))
	write(t, filepath.Join(home, ".config", "heroic", "legendaryConfig", "legendary", "installed.json"), `{
  "Sugar": {"app_name": "Sugar", "title": "Rocket League®", "install_path": "`+game+`", "executable": "Binaries/Win64/Launcher.exe", "platform": "Windows"},
  "Gone": {"app_name": "Gone", "title": "Unmounted", "install_path": "/nonexistent/path", "executable": "x.exe"},
  "Blank": {"app_name": "Blank", "title": "", "install_path": "`+game+`"}
}`)
	games := Scan(Options{Home: home})
	want := []Game{{Name: "Rocket League®", Source: "Heroic", Folder: game}}
	if !reflect.DeepEqual(games, want) {
		t.Fatalf("games = %+v, want %+v", games, want)
	}
}

func TestHeroicFlatpakConfigIsRead(t *testing.T) {
	home := t.TempDir()
	game := mkdir(t, filepath.Join(t.TempDir(), "g"))
	write(t, filepath.Join(home, ".var", "app", "com.heroicgameslauncher.hgl", "config", "heroic", "legendaryConfig", "legendary", "installed.json"),
		`{"A": {"title": "Flat Game", "install_path": "`+game+`"}}`)
	if got, want := names(Scan(Options{Home: home})), []string{"Flat Game"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("names = %v, want %v", got, want)
	}
}

func TestHeroicEmptyOrBrokenFilesYieldNothing(t *testing.T) {
	home := t.TempDir()
	h := filepath.Join(home, ".config", "heroic")
	write(t, filepath.Join(h, "legendaryConfig", "legendary", "installed.json"), `{}`)
	write(t, filepath.Join(h, "gog_store", "installed.json"), `{}`)
	write(t, filepath.Join(h, "sideload_apps", "library.json"), `not json`)
	if games := Scan(Options{Home: home}); len(games) != 0 {
		t.Fatalf("games = %+v, want none", games)
	}
}

func TestHeroicGogUsesLibraryTitle(t *testing.T) {
	home := t.TempDir()
	game := mkdir(t, filepath.Join(t.TempDir(), "witcher"))
	h := filepath.Join(home, ".config", "heroic", "gog_store")
	write(t, filepath.Join(h, "installed.json"), `{"installed": [{"appName": "123", "install_path": "`+game+`"}]}`)
	write(t, filepath.Join(h, "library.json"), `{"games": [{"app_name": "123", "title": "The Witcher"}]}`)
	games := Scan(Options{Home: home})
	if len(games) != 1 || games[0].Name != "The Witcher" || games[0].Folder != game || games[0].Source != "Heroic" {
		t.Fatalf("games = %+v", games)
	}
}

func desktop(name, exec string, extra ...string) string {
	lines := []string{"[Desktop Entry]", "Type=Application", "Name=" + name, "Exec=" + exec}
	return strings.Join(append(lines, extra...), "\n") + "\n"
}

func TestDesktopEntriesGiveNameAndExecutable(t *testing.T) {
	home := t.TempDir()
	data := t.TempDir()
	apps := filepath.Join(data, "applications")
	write(t, filepath.Join(apps, "cemu.desktop"), desktop("Cemu", "cemu %f", "Categories=Game;Emulator;"))
	write(t, filepath.Join(apps, "wine.desktop"), desktop("Prefix Game", "env WINEPREFIX=/home/u/.wine /opt/game/game.exe"))
	write(t, filepath.Join(apps, "xemu.desktop"), desktop("xemu", "env FOO=1 /usr/bin/xemu %U"))
	write(t, filepath.Join(apps, "hidden.desktop"), desktop("Hidden", "hidden", "Hidden=true"))
	write(t, filepath.Join(apps, "nodisplay.desktop"), desktop("NoDisplay", "nodisplay", "NoDisplay=true"))
	write(t, filepath.Join(apps, "link.desktop"), "[Desktop Entry]\nType=Link\nName=Link\nURL=https://x\n")
	write(t, filepath.Join(apps, "steam.desktop"), desktop("Some Steam Game", "steam steam://rungameid/1"))
	write(t, filepath.Join(apps, "bottle.desktop"), desktop("Bottle Game", `bottles-cli run -p "A" -b "B" -- %u`))
	write(t, filepath.Join(apps, "lutris.desktop"), desktop("Lutris Game", "Lutris lutris:rungameid/1"))
	write(t, filepath.Join(apps, "sysd.desktop"), desktop("Systemd Game", "systemd-run --user sober"))
	write(t, filepath.Join(apps, "noexec.desktop"), "[Desktop Entry]\nType=Application\nName=NoExec\n")
	write(t, filepath.Join(apps, "bad.desktop"), "garbage")
	write(t, filepath.Join(apps, "localized.desktop"), "[Desktop Entry]\nType=Application\nName[es]=Hola\nName=Localized\nExec=\"/opt/my app/bin/loc\" --flag\n")
	games := Scan(Options{Home: home, DataDirs: []string{data}})
	want := []Game{
		{Name: "Cemu", Source: "App", Executable: "cemu", IsGame: true},
		{Name: "Localized", Source: "App", Executable: "loc"},
		{Name: "Prefix Game", Source: "App", Executable: "game.exe"},
		{Name: "xemu", Source: "App", Executable: "xemu"},
	}
	if !reflect.DeepEqual(games, want) {
		t.Fatalf("games = %+v, want %+v", games, want)
	}
}

func TestDesktopFlatpakUsesCommandOrAppIDTail(t *testing.T) {
	data := t.TempDir()
	apps := filepath.Join(data, "applications")
	write(t, filepath.Join(apps, "a.desktop"), desktop("With Command", "/usr/bin/flatpak run --branch=stable --arch=x86_64 --command=/app/bin/mycmd com.example.With %U"))
	write(t, filepath.Join(apps, "b.desktop"), desktop("No Command", "/usr/bin/flatpak run --branch=stable app.xemu.xemu"))
	got := Scan(Options{Home: t.TempDir(), DataDirs: []string{data}})
	want := []Game{
		{Name: "No Command", Source: "App", Executable: "xemu"},
		{Name: "With Command", Source: "App", Executable: "mycmd"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("games = %+v, want %+v", got, want)
	}
}

func TestDesktopDuplicateExecutablesAreListedOnce(t *testing.T) {
	data := t.TempDir()
	apps := filepath.Join(data, "applications")
	write(t, filepath.Join(apps, "one.desktop"), desktop("One", "same"))
	write(t, filepath.Join(apps, "two.desktop"), desktop("Two", "same"))
	if got := Scan(Options{Home: t.TempDir(), DataDirs: []string{data}}); len(got) != 1 {
		t.Fatalf("games = %+v, want one", got)
	}
}

func TestResultsAreSortedByNameIgnoringCase(t *testing.T) {
	home, _ := steamFixture(t)
	data := t.TempDir()
	write(t, filepath.Join(data, "applications", "z.desktop"), desktop("alpha tool", "alpha"))
	got := names(Scan(Options{Home: home, DataDirs: []string{data}}))
	want := []string{"alpha tool", "Terraria", "Wallpaper Engine"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("names = %v, want %v", got, want)
	}
}

func TestScanOfAnEmptyHomeReturnsNothingAndDoesNotPanic(t *testing.T) {
	if got := Scan(Options{Home: t.TempDir()}); len(got) != 0 {
		t.Fatalf("games = %+v", got)
	}
	if got := Scan(Options{}); got == nil {
		t.Fatal("Scan(Options{}) must return an empty slice, not nil")
	}
}

func TestParseVDFHandlesEscapesCommentsAndNesting(t *testing.T) {
	n, err := parseVDF("// comment\n\"Root\" { \"Key\" \"a \\\"b\\\" \\\\ c\" \"Inner\" { \"x\" \"1\" } }")
	if err != nil {
		t.Fatal(err)
	}
	root := n["root"].(vdfNode)
	if root["key"] != `a "b" \ c` {
		t.Fatalf("key = %q", root["key"])
	}
	if root["inner"].(vdfNode)["x"] != "1" {
		t.Fatalf("inner = %v", root["inner"])
	}
	for _, bad := range []string{`"a" {`, `}`, `"a"`, `"a" "b`, `{ }`} {
		if _, err := parseVDF(bad); err == nil {
			t.Fatalf("parseVDF(%q) = nil error", bad)
		}
	}
}
