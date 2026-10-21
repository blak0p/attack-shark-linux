//go:build linux

package udevpolicy

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	packaged "github.com/blak0p/attack-shark-linux/packaging/udev"
	"golang.org/x/sys/unix"
)

func TestInstalledSourceFixedPath(t *testing.T) {
	// Construction alone must perform no host reads.
	s, ok := InstalledSource().(installedSource)
	if !ok || s.path != InstalledPath {
		t.Fatalf("unexpected installed source: %#v", s)
	}
}

func TestInstalledSourceInspect(t *testing.T) {
	for _, name := range []string{"current", "missing", "symlink", "directory", "fifo", "oversize", "custom"} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "policy.rules")
			want := Error
			var original []byte
			switch name {
			case "current", "custom", "oversize":
				original = []byte(packaged.Policy())
				switch name {
				case "current":
					want = Current
				case "custom":
					original = append(original, []byte("# local\n")...)
					want = Custom
				case "oversize":
					original = bytes.Repeat([]byte("x"), MaxPolicyBytes+10)
				}
				if err := os.WriteFile(path, original, 0400); err != nil {
					t.Fatal(err)
				}
			case "missing":
				want = Missing
			case "symlink":
				if err := os.Symlink("absent-target", path); err != nil {
					t.Fatal(err)
				}
			case "directory":
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
			case "fifo":
				if err := unix.Mkfifo(path, 0600); err != nil {
					t.Fatal(err)
				}
			}
			s := installedSource{path: path}
			before, err := os.ReadDir(dir)
			if err != nil {
				t.Fatal(err)
			}
			got := Inspect(s)
			if got.State != want || (got.Err != nil) != (want == Error) {
				t.Fatalf("Inspect = %+v, want %s", got, want)
			}
			after, err := os.ReadDir(dir)
			if err != nil || len(after) != len(before) {
				t.Fatalf("directory changed: %v", err)
			}
			if original != nil {
				info, err := os.Stat(path)
				if err != nil || info.Mode().Perm() != 0400 {
					t.Fatalf("mode changed: %v", err)
				}
				data, err := s.Read(7)
				if err != nil || len(data) != 7 {
					t.Fatalf("bounded read = %d, %v", len(data), err)
				}
				onDisk, err := os.ReadFile(path)
				if err != nil || !bytes.Equal(original, onDisk) || !bytes.Equal(data, original[:7]) {
					t.Fatalf("content changed: %v", err)
				}
				bounded, err := s.Read(MaxPolicyBytes * 2)
				if err != nil || len(bounded) > MaxPolicyBytes+1 {
					t.Fatalf("adapter size cap = %d, %v", len(bounded), err)
				}
			}
		})
	}
}

// These replacements happen after a successful Lstat. Read must independently
// reject unsafe final components, without blocking on a FIFO or following links.
func TestInstalledSourceReadRechecksFinalComponent(t *testing.T) {
	for _, kind := range []string{"symlink", "directory", "fifo", "missing"} {
		t.Run(kind, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "policy.rules")
			if err := os.WriteFile(path, []byte(packaged.Policy()), 0400); err != nil {
				t.Fatal(err)
			}
			s := installedSource{path: path}
			mode, err := s.Lstat()
			if err != nil || !mode.IsRegular() {
				t.Fatalf("Lstat = %v, %v", mode, err)
			}
			if err := os.Rename(path, filepath.Join(dir, "original")); err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "symlink":
				if err := os.Symlink("original", path); err != nil {
					t.Fatal(err)
				}
			case "directory":
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
			case "fifo":
				if err := unix.Mkfifo(path, 0600); err != nil {
					t.Fatal(err)
				}
			}
			if data, err := s.Read(MaxPolicyBytes + 1); err == nil || len(data) != 0 {
				t.Fatalf("unsafe read = %q, %v", data, err)
			}
			if kind == "symlink" {
				mode, err := s.Lstat()
				if err != nil || mode&fs.ModeSymlink == 0 {
					t.Fatalf("Lstat followed symlink: %v, %v", mode, err)
				}
			}
		})
	}
}
