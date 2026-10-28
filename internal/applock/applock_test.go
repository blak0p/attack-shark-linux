package applock

import (
	"path/filepath"
	"testing"
)

func TestAcquireMarksLockHeldUntilReleased(t *testing.T) {
	path := filepath.Join(t.TempDir(), "run", "app.lock")

	if held, err := IsHeld(path); err != nil || held {
		t.Fatalf("IsHeld before acquire = %v, %v; want false, nil", held, err)
	}

	lock, err := Acquire(path)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	if held, err := IsHeld(path); err != nil || !held {
		t.Fatalf("IsHeld while acquired = %v, %v; want true, nil", held, err)
	}

	if err := lock.Release(); err != nil {
		t.Fatalf("Release: %v", err)
	}
	if held, err := IsHeld(path); err != nil || held {
		t.Fatalf("IsHeld after release = %v, %v; want false, nil", held, err)
	}
}

func TestSecondAcquireFailsWithErrHeld(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.lock")
	first, err := Acquire(path)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	defer first.Release()

	if _, err := Acquire(path); err != ErrHeld {
		t.Fatalf("second Acquire error = %v; want ErrHeld", err)
	}
}

func TestIsHeldDoesNotCreateTheLockFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "missing", "app.lock")
	if held, err := IsHeld(path); err != nil || held {
		t.Fatalf("IsHeld = %v, %v; want false, nil", held, err)
	}
	if _, err := Acquire(filepath.Join(dir, "x.lock")); err != nil {
		t.Fatal(err)
	}
	if matches, _ := filepath.Glob(filepath.Join(dir, "missing", "*")); len(matches) != 0 {
		t.Fatalf("IsHeld created files: %v", matches)
	}
}
