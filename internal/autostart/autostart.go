// Package autostart manages the XDG autostart entry that starts the app in
// tray mode at login.
package autostart

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const entryName = "attack-shark-linux.desktop"

// Dir returns the XDG autostart directory for the current user.
func Dir() (string, error) {
	config, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(config, "autostart"), nil
}

// Enabled reports whether dir holds an entry that the session would run.
func Enabled(dir string) bool {
	data, err := os.ReadFile(filepath.Join(dir, entryName))
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "Hidden=true" {
			return false
		}
	}
	return true
}

// Set writes or removes the autostart entry in dir. The entry starts launcher
// with --tray; launcher must be an absolute path.
func Set(dir, launcher string, enabled bool) error {
	path := filepath.Join(dir, entryName)
	if !enabled {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	if !filepath.IsAbs(launcher) {
		return fmt.Errorf("autostart launcher %q is not an absolute path", launcher)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".autostart-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.WriteString(entry(launcher)); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

func entry(launcher string) string {
	return "[Desktop Entry]\n" +
		"Type=Application\n" +
		"Name=Attack Shark X6 Configurator\n" +
		"Comment=Keeps the profile watcher running in the system tray\n" +
		"Exec=" + execArg(launcher) + " --tray\n" +
		"Icon=attack-shark-x6\n" +
		"Terminal=false\n" +
		"X-GNOME-Autostart-enabled=true\n"
}

// execArg quotes one Exec argument. Inside double quotes the reserved
// characters take a backslash; the key-file string then doubles every
// backslash; % becomes %% so it is not read as a field code.
func execArg(path string) string {
	var quoted strings.Builder
	for _, r := range path {
		switch r {
		case '"', '`', '$', '\\':
			quoted.WriteByte('\\')
		case '%':
			quoted.WriteByte('%')
		}
		quoted.WriteRune(r)
	}
	return `"` + strings.ReplaceAll(quoted.String(), `\`, `\\`) + `"`
}
