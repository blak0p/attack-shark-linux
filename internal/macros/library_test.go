package macros

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
)

func openTest(t *testing.T) (*Library, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "macros.json")
	lib, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	return lib, path
}

func TestCRUDReloadAndCopies(t *testing.T) {
	lib, path := openTest(t)
	if len(lib.List()) != 0 {
		t.Fatal("missing file should be empty")
	}
	draft, err := lib.Create("Draft", nil)
	if err != nil || draft.ID == "" {
		t.Fatalf("draft: %v %v", draft, err)
	}
	events := []Event{{Type: MouseLeft, Action: Down, DelayMS: 0}, {Type: MouseRight, Action: Up, DelayMS: 17}}
	updated, err := lib.Update(draft.ID, "Renamed without a length cap", events)
	if err != nil || updated.ID != draft.ID {
		t.Fatalf("update: %v %v", updated, err)
	}
	events[0].DelayMS = 999
	updated.Events[0].DelayMS = 999
	list := lib.List()
	list[0].Events[0].DelayMS = 999
	got, err := lib.Read(draft.ID)
	if err != nil || got.Events[0].DelayMS != 0 {
		t.Fatalf("alias: %v %v", got, err)
	}
	got.Events[0].DelayMS = 999
	reloaded, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	want := Macro{ID: draft.ID, Name: "Renamed without a length cap", Events: []Event{{Type: MouseLeft, Action: Down}, {Type: MouseRight, Action: Up, DelayMS: 17}}}
	got, _ = reloaded.Read(draft.ID)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("reload: %#v", got)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("permissions: %v %v", info, err)
	}
	if err := reloaded.Delete(draft.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := reloaded.Read(draft.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	reloaded, err = Open(path)
	if err != nil || len(reloaded.List()) != 0 {
		t.Fatalf("delete reload: %v", err)
	}
}

func TestValidationDoesNotMutate(t *testing.T) {
	lib, path := openTest(t)
	m, _ := lib.Create("original", nil)
	before, _ := os.ReadFile(path)
	cases := []struct {
		name   string
		events []Event
	}{
		{" \t", nil},
		{"valid", []Event{{Type: "keyboard", Action: Down}}},
		{"valid", []Event{{Type: MouseLeft, Action: "click"}}},
		{"valid", []Event{{Type: MouseLeft, Action: Down, DelayMS: -1}}},
	}
	for _, tc := range cases {
		if _, err := lib.Create(tc.name, tc.events); err == nil {
			t.Fatal("invalid create accepted")
		}
		if _, err := lib.Update(m.ID, tc.name, tc.events); err == nil {
			t.Fatal("invalid update accepted")
		}
	}
	if _, err := lib.Update("missing", "valid", nil); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if err := lib.Delete("missing"); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) || len(lib.List()) != 1 {
		t.Fatal("error mutated store")
	}
	got, _ := lib.Read(m.ID)
	if got.Name != "original" {
		t.Fatal("error mutated memory")
	}
}

func TestRejectInvalidStoresWithoutOverwrite(t *testing.T) {
	for _, text := range []string{
		`not json`, `{"version":2,"macros":[]}`, `{"version":1,"macros":[{"id":"x","name":"one"},{"id":"x","name":"two"}]}`,
		`{"version":1,"macros":[{"id":"","name":"one"}]}`, `{"version":1,"macros":[{"id":"x","name":" "}]}`,
		`{"version":1,"macros":[{"id":"x","name":"one","events":[{"type":"mouse_left","action":"down","delay_ms":-1}]}]}`,
		`{"version":1,"macros":[]} {}`, `{"version":1,"macros":[],"unexpected":true}`,
	} {
		t.Run(text, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "macros.json")
			if err := os.WriteFile(path, []byte(text), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := Open(path); err == nil {
				t.Fatal("invalid store accepted")
			}
			got, _ := os.ReadFile(path)
			if string(got) != text {
				t.Fatal("invalid store overwritten")
			}
		})
	}
}

func TestFailedPersistenceDoesNotAdvance(t *testing.T) {
	lib, path := openTest(t)
	m, _ := lib.Create("original", nil)
	// A directory cannot be replaced by the atomic file rename.
	directory := filepath.Join(filepath.Dir(path), "destination")
	if err := os.Mkdir(directory, 0700); err != nil {
		t.Fatal(err)
	}
	lib.path = directory
	if _, err := lib.Create("new", nil); err == nil {
		t.Fatal("create succeeded")
	}
	if _, err := lib.Update(m.ID, "changed", nil); err == nil {
		t.Fatal("update succeeded")
	}
	if err := lib.Delete(m.ID); err == nil {
		t.Fatal("delete succeeded")
	}
	got, _ := lib.Read(m.ID)
	if got.Name != "original" || len(lib.List()) != 1 {
		t.Fatal("failed save advanced state")
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil || len(entries) != 2 {
		t.Fatalf("temporary files leaked: %v %v", entries, err)
	}
}

func TestConcurrentAccess(t *testing.T) {
	lib, _ := openTest(t)
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m, err := lib.Create("same name allowed", nil)
			if err != nil {
				t.Error(err)
				return
			}
			if _, err := lib.Update(m.ID, "updated", []Event{{Type: MouseLeft, Action: Down}}); err != nil {
				t.Error(err)
			}
			lib.List()
			if _, err := lib.Read(m.ID); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	seen := map[string]bool{}
	for _, m := range lib.List() {
		if seen[m.ID] {
			t.Fatal("duplicate ID")
		}
		seen[m.ID] = true
	}
	if len(seen) != 12 {
		t.Fatal("lost concurrent writes")
	}
}
