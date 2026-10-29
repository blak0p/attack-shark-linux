package autostart

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func entryPath(dir string) string { return filepath.Join(dir, "attack-shark-linux.desktop") }

func TestEnableWritesTrayModeEntry(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "autostart") // missing directory must be created
	if err := Set(dir, "/opt/App Dir/attack-shark.AppImage", true); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(entryPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{
		"[Desktop Entry]\n",
		"Type=Application\n",
		"Name=Attack Shark X6 Configurator\n",
		`Exec="/opt/App Dir/attack-shark.AppImage" --tray` + "\n",
		"X-GNOME-Autostart-enabled=true\n",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("entry misses %q:\n%s", want, text)
		}
	}
	if !Enabled(dir) {
		t.Fatal("Enabled must report true after Set(true)")
	}
}

func TestExecQuotesSpecialCharacters(t *testing.T) {
	dir := t.TempDir()
	if err := Set(dir, `/tmp/we"ird$`+"`"+`\path`, true); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(entryPath(dir))
	want := `Exec="/tmp/we\\"ird\\$\\` + "`" + `\\\\path" --tray`
	if !strings.Contains(string(data), want) {
		t.Fatalf("exec not escaped per the desktop entry spec, want %q in:\n%s", want, data)
	}
}

func TestDisableRemovesEntryAndIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	if err := Set(dir, "/usr/bin/x", true); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ { // second call: nothing to remove, still no error
		if err := Set(dir, "/usr/bin/x", false); err != nil {
			t.Fatalf("disable #%d: %v", i, err)
		}
	}
	if Enabled(dir) {
		t.Fatal("Enabled must report false after Set(false)")
	}
	if _, err := os.Stat(entryPath(dir)); !os.IsNotExist(err) {
		t.Fatalf("entry must be gone, stat err = %v", err)
	}
}

func TestEnabledFalseWhenMissingOrHidden(t *testing.T) {
	dir := t.TempDir()
	if Enabled(dir) {
		t.Fatal("no file means disabled")
	}
	os.WriteFile(entryPath(dir), []byte("[Desktop Entry]\nType=Application\nHidden=true\n"), 0o644)
	if Enabled(dir) {
		t.Fatal("Hidden=true entry (disabled by the user's session settings) means disabled")
	}
}

func TestEnableRejectsRelativeLauncherAndLeavesNothing(t *testing.T) {
	dir := t.TempDir()
	if err := Set(dir, "attack-shark-linux", true); err == nil {
		t.Fatal("a relative launcher path would break at login and must be rejected")
	}
	if Enabled(dir) {
		t.Fatal("a rejected enable must not create an entry")
	}
}

func TestEnableReplacesStaleEntry(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(entryPath(dir), []byte("old"), 0o644)
	if err := Set(dir, "/new/path", true); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(entryPath(dir))
	if !strings.Contains(string(data), `Exec="/new/path" --tray`) || strings.Contains(string(data), "old") {
		t.Fatalf("stale entry not replaced:\n%s", data)
	}
}
