//go:build linux

package udevpolicy

import (
	"errors"
	"io"
	"io/fs"
	"os"

	"golang.org/x/sys/unix"
)

// InstalledSource returns a read-only Linux source for the fixed system policy.
// Construction performs no I/O; Inspect performs the inspection on demand.
func InstalledSource() Source {
	return installedSource{path: InstalledPath}
}

// The path is private so fixture tests can exercise the real adapter without
// exposing arbitrary installed paths to application/renderer callers.
type installedSource struct {
	path string
}

func (s installedSource) Lstat() (fs.FileMode, error) {
	info, err := os.Lstat(s.path)
	if err != nil {
		return 0, err
	}
	return info.Mode(), nil
}

func (s installedSource) Read(limit int) ([]byte, error) {
	if limit < 0 {
		return nil, errors.New("negative udev policy read limit")
	}
	// Independently protect the final component even after a successful Lstat.
	// NONBLOCK prevents a substituted FIFO from blocking before descriptor Stat.
	fd, err := unix.Open(s.path, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: s.path, Err: err}
	}
	file := os.NewFile(uintptr(fd), s.path)
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("udev policy is not a regular file")
	}
	// Never trust size metadata: a regular file may grow after Stat.
	if limit > MaxPolicyBytes+1 {
		limit = MaxPolicyBytes + 1
	}
	return io.ReadAll(io.LimitReader(file, int64(limit)))
}
