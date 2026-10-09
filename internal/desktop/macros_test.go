package desktop

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/blak0p/attack-shark-linux/internal/macros"
)

func TestMacroOfflineCRUD(t *testing.T) {
	path := filepath.Join(t.TempDir(), "macros-v1.json")
	lib, err := macros.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	// A bare service has no backend, inventory, or selected device. Any device
	// dependency in these methods would panic rather than pass this test.
	s := &Service{}
	WithMacroLibrary(lib, nil)(s)
	// A second device-facing service sharing the app dependency sees the same
	// library immediately, without separate per-device copies or reloads.
	peer := &Service{}
	WithMacroLibrary(lib, nil)(peer)
	events := []macros.Event{{Type: macros.MouseLeft, Action: macros.Down, DelayMS: 12}}
	m, err := s.CreateMacro("click", events)
	if err != nil {
		t.Fatal(err)
	}
	got, err := peer.ReadMacro(m.ID)
	if err != nil || !reflect.DeepEqual(got, m) {
		t.Fatalf("read: %#v %v", got, err)
	}
	m, err = s.UpdateMacro(m.ID, "renamed", nil)
	if err != nil {
		t.Fatal(err)
	}
	reopened, err := macros.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	other := &Service{macroLibrary: reopened}
	list, err := other.ListMacros()
	if err != nil || !reflect.DeepEqual(list, []macros.Macro{m}) {
		t.Fatalf("reload: %#v %v", list, err)
	}
	if _, err := s.CreateMacro(" ", nil); err == nil {
		t.Fatal("accepted blank name")
	}
	if _, err := s.UpdateMacro(m.ID, "bad", []macros.Event{{DelayMS: -1}}); err == nil {
		t.Fatal("accepted invalid event")
	}
	if err := other.DeleteMacro(m.ID); err != nil {
		t.Fatal(err)
	}
	reopened, err = macros.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(reopened.List()) != 0 {
		t.Fatal("delete not persisted")
	}
	if _, err := other.ReadMacro(m.ID); !errors.Is(err, macros.ErrNotFound) {
		t.Fatalf("missing read: %v", err)
	}
	if err := other.DeleteMacro(m.ID); !errors.Is(err, macros.ErrNotFound) {
		t.Fatalf("missing delete: %v", err)
	}
}

func TestMacroUnavailable(t *testing.T) {
	cause := errors.New("corrupt macro data")
	for _, s := range []*Service{{}, {macroLibraryError: cause}} {
		if _, err := s.ListMacros(); err == nil {
			t.Fatal("list hid unavailable library")
		}
		if _, err := s.ReadMacro("id"); err == nil {
			t.Fatal("read hid unavailable library")
		}
		if _, err := s.CreateMacro("name", nil); err == nil {
			t.Fatal("create hid unavailable library")
		}
		if _, err := s.UpdateMacro("id", "name", nil); err == nil {
			t.Fatal("update hid unavailable library")
		}
		if err := s.DeleteMacro("id"); err == nil {
			t.Fatal("delete hid unavailable library")
		}
	}
	s := &Service{macroLibraryError: cause}
	if _, err := s.ListMacros(); !errors.Is(err, cause) {
		t.Fatalf("lost cause: %v", err)
	}
}

func TestMacroPersistenceError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "macros-v1.json")
	lib, err := macros.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	s := &Service{macroLibrary: lib}
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateMacro("draft", nil); err == nil {
		t.Fatal("hid persistence error")
	}
	list, err := s.ListMacros()
	if err != nil || len(list) != 0 {
		t.Fatalf("failed mutation became visible: %v %v", list, err)
	}
}
