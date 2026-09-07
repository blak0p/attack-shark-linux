package configstore

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// StatePurger removes the dedicated application state namespace only after a
// physical reset has completed. It refuses symlinked namespaces.
type syncCloser interface {
	Sync() error
	Close() error
}

type statePurgeOps struct {
	lstat     func(string) (os.FileInfo, error)
	rename    func(string, string) error
	mkdir     func(string, os.FileMode) error
	open      func(string) (syncCloser, error)
	removeAll func(string) error
}

func defaultStatePurgeOps() statePurgeOps {
	return statePurgeOps{
		lstat:     os.Lstat,
		rename:    os.Rename,
		mkdir:     os.Mkdir,
		open:      func(path string) (syncCloser, error) { return os.Open(path) },
		removeAll: os.RemoveAll,
	}
}

type StatePurger struct {
	dir string
	ops statePurgeOps
}

func NewStatePurger(dir string) StatePurger {
	return newStatePurgerWithOps(dir, defaultStatePurgeOps())
}

func newStatePurgerWithOps(dir string, ops statePurgeOps) StatePurger {
	return StatePurger{dir: filepath.Clean(dir), ops: ops}
}

func (p StatePurger) PurgeAll() error {
	info, err := p.ops.lstat(p.dir)
	if os.IsNotExist(err) {
		return os.MkdirAll(p.dir, 0o700)
	}
	if err != nil {
		return fmt.Errorf("inspect state directory: %w", err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("state directory is not a real directory")
	}
	parent := filepath.Dir(p.dir)
	quarantine := p.dir + ".reset-quarantine"
	if _, err := p.ops.lstat(quarantine); err == nil {
		return fmt.Errorf("reset quarantine already exists")
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect reset quarantine: %w", err)
	}
	if err := p.ops.rename(p.dir, quarantine); err != nil {
		return fmt.Errorf("quarantine state: %w", err)
	}
	if err := p.ops.mkdir(p.dir, 0o700); err != nil {
		return p.restore(quarantine, fmt.Errorf("recreate state directory: %w", err))
	}
	dir, err := p.ops.open(parent)
	if err != nil {
		return p.restore(quarantine, fmt.Errorf("open state parent: %w", err))
	}
	if syncErr := dir.Sync(); syncErr != nil {
		closeErr := dir.Close()
		return p.restore(quarantine, errors.Join(fmt.Errorf("sync state parent: %w", syncErr), closeErr))
	}
	if err := dir.Close(); err != nil {
		return p.restore(quarantine, fmt.Errorf("close state parent: %w", err))
	}
	if err := p.ops.removeAll(quarantine); err != nil {
		return p.restore(quarantine, fmt.Errorf("remove reset quarantine: %w", err))
	}
	return nil
}

func (p StatePurger) restore(quarantine string, cleanupErr error) error {
	if err := p.ops.removeAll(p.dir); err != nil {
		return errors.Join(cleanupErr, fmt.Errorf("remove recreated state directory: %w", err))
	}
	if err := p.ops.rename(quarantine, p.dir); err != nil {
		return errors.Join(cleanupErr, fmt.Errorf("restore state quarantine: %w", err))
	}
	return cleanupErr
}
