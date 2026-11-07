package update

import "testing"

func TestBuildLabelDescribesVersionAndInstallSource(t *testing.T) {
	original := []string{CurrentVersion, PackageManager, Distribution, Branch, Commit}
	t.Cleanup(func() {
		CurrentVersion, PackageManager, Distribution, Branch, Commit = original[0], original[1], original[2], original[3], original[4]
	})
	tests := []struct {
		name                                           string
		version, manager, distribution, branch, commit string
		want                                           string
	}{
		{name: "release AppImage", version: "1.8.0", want: "1.8.0"},
		{name: "stable AUR", version: "1.8.0", manager: "pacman", distribution: "AUR", want: "1.8.0 (AUR, pacman)"},
		{name: "git AUR", version: "1.8.0.r1.g2bf7932", manager: "pacman", distribution: "AUR", branch: "main", commit: "2bf7932", want: "main 2bf7932 (AUR, pacman)"},
		{name: "manager without distribution", version: "1.8.0", manager: "pacman", want: "1.8.0 (pacman)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			CurrentVersion, PackageManager, Distribution, Branch, Commit = tt.version, tt.manager, tt.distribution, tt.branch, tt.commit
			if got := BuildLabel(); got != tt.want {
				t.Fatalf("BuildLabel() = %q, want %q", got, tt.want)
			}
		})
	}
}
