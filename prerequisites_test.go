package attacksharklinux_test

import (
	"os"
	"strings"
	"testing"
)

func TestDesktopPrerequisitesArePinned(t *testing.T) {
	for _, tt := range []struct {
		name string
		file string
		want []string
	}{
		{"Go and Wails", "go.mod", []string{"go 1.25", "github.com/wailsapp/wails/v3 v3.0.0-beta.5"}},
		{"Wails configuration", "config.yml", []string{"version: '3'", "react: 18.2.0", "typescript: 5.2.2", "vite: 8.0.5"}},
		{"Linux build prerequisites", "Taskfile.yml", []string{"libgtk-3-dev", "libwebkit2gtk-4.1-dev", "webkit2gtk4.1-devel"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			data, err := os.ReadFile(tt.file)
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range tt.want {
				if !strings.Contains(string(data), want) {
					t.Fatalf("%s does not contain %q", tt.file, want)
				}
			}
		})
	}
}
