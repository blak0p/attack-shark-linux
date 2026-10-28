// Package applock tracks whether the desktop app is running through an
// advisory flock on a lock file. The kernel releases the lock when the owning
// process exits, even after a crash, so a stale file never reads as "running".
package applock

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

// ErrHeld means another process already holds the lock.
var ErrHeld = errors.New("applock: lock already held")

// Lock is an acquired exclusive lock.
type Lock struct {
	file *os.File
}

// Acquire takes the exclusive lock at path without blocking, creating the file
// and its parent directory when needed.
func Acquire(path string) (*Lock, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		file.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, ErrHeld
		}
		return nil, err
	}
	return &Lock{file: file}, nil
}

// Release drops the lock. Closing the file releases the flock.
func (l *Lock) Release() error {
	return l.file.Close()
}

// IsHeld reports whether another process holds the lock at path. It never
// creates the file: a missing file means nobody holds it.
func IsHeld(path string) (bool, error) {
	file, err := os.OpenFile(path, os.O_RDWR, 0)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer file.Close()
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return true, nil
		}
		return false, err
	}
	return false, nil
}
