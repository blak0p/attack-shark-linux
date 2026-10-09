package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMacroStartupPersistence(t *testing.T) {
	dir := t.TempDir()
	first := newDesktopService(dir)
	// Existing service initialization may seed factory defaults. Macro CRUD must
	// preserve that config rather than asserting startup never created it.
	factoryPath := filepath.Join(dir, "factory-defaults.json")
	factoryBefore, factoryErr := os.ReadFile(factoryPath)
	m, err := first.CreateMacro("offline", nil)
	if err != nil {
		t.Fatal(err)
	}
	second := newDesktopService(dir)
	got, err := second.ReadMacro(m.ID)
	if err != nil || got.Name != m.Name {
		t.Fatalf("reload: %#v %v", got, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "macros-v1.json")); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"devices-v2.json", "applied-dpi.json"} {
		if _, err := os.Stat(filepath.Join(dir, name)); !os.IsNotExist(err) {
			t.Fatalf("macro CRUD touched %s: %v", name, err)
		}
	}
	factoryAfter, err := os.ReadFile(factoryPath)
	if (factoryErr == nil) != (err == nil) || string(factoryBefore) != string(factoryAfter) {
		t.Fatal("macro CRUD changed factory config")
	}
}

func TestMacroStartupCorruptPreserved(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "macros-v1.json")
	original := []byte("corrupt original")
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	s := newDesktopService(dir)
	if _, err := s.ListMacros(); err == nil {
		t.Fatal("corrupt library silently replaced")
	}
	if _, err := s.CreateMacro("draft", nil); err == nil {
		t.Fatal("corrupt library writable")
	}
	contents, err := os.ReadFile(path)
	if err != nil || string(contents) != string(original) {
		t.Fatalf("original changed: %q %v", contents, err)
	}
}
