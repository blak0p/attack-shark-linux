package macros

import (
	"encoding/json"
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

func keyboardEvents(t *testing.T, text string) []Event {
	t.Helper()
	var events []Event
	if err := json.Unmarshal([]byte(text), &events); err != nil {
		t.Fatal(err)
	}
	return events
}

func TestKeyboardLocalCRUD(t *testing.T) {
	lib, path := openTest(t)
	text := `[{"type":"keyboard","action":"down","delay_ms":17,"key_code":" KeyUnverified-Ω "},{"type":"mouse_left","action":"up","delay_ms":0},{"type":"keyboard","action":"up","delay_ms":9223372036854775807,"key_code":" KeyUnverified-Ω "}]`
	events := keyboardEvents(t, text)
	before := append([]Event{}, events...)
	created, err := lib.Create("mixed", events)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(events, before) {
		t.Fatal("create mutated input")
	}
	encoded, err := json.Marshal(created.Events)
	if err != nil || string(encoded) != text {
		t.Fatalf("lossy round trip: %s %v", encoded, err)
	}
	events[0] = Event{}
	created.Events[0] = Event{}
	got, err := lib.Read(created.ID)
	if err != nil || !reflect.DeepEqual(got.Events, before) {
		t.Fatalf("create alias: %#v %v", got, err)
	}
	updated, err := lib.Update(created.ID, "updated", before)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, keyboardEvents(t, text)) {
		t.Fatal("update mutated input")
	}
	updated.Events[0] = Event{}
	listed := lib.List()
	listed[0].Events[0] = Event{}
	got.Events[0] = Event{}
	before[0] = Event{}
	reloaded, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	got, err = reloaded.Read(created.ID)
	if err != nil || got.Name != "updated" || !reflect.DeepEqual(got.Events, keyboardEvents(t, text)) {
		t.Fatalf("reload: %#v %v", got, err)
	}
	if err := reloaded.Delete(created.ID); err != nil {
		t.Fatal(err)
	}
	reloaded, err = Open(path)
	if err != nil || len(reloaded.List()) != 0 {
		t.Fatalf("delete reload: %v", err)
	}
}

func TestKeyboardLocalValidation(t *testing.T) {
	for _, text := range []string{
		`[{"type":"keyboard","action":"down","delay_ms":0}]`,
		`[{"type":"keyboard","action":"down","delay_ms":0,"key_code":" \t"}]`,
		`[{"type":"keyboard","action":"click","delay_ms":0,"key_code":"KeyA"}]`,
		`[{"type":"keyboard","action":"up","delay_ms":-1,"key_code":"KeyA"}]`,
		`[{"type":"mouse_left","action":"down","delay_ms":0,"key_code":"KeyA"}]`,
		`[{"type":"mouse_right","action":"up","delay_ms":0,"key_code":" "}]`,
	} {
		t.Run(text, func(t *testing.T) {
			lib, path := openTest(t)
			original, err := lib.Create("original", nil)
			if err != nil {
				t.Fatal(err)
			}
			contents, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			events := keyboardEvents(t, text)
			before := append([]Event{}, events...)
			if _, err := lib.Create("bad", events); err == nil {
				t.Fatal("invalid create accepted")
			}
			if _, err := lib.Update(original.ID, "bad", events); err == nil {
				t.Fatal("invalid update accepted")
			}
			if !reflect.DeepEqual(events, before) {
				t.Fatal("validation mutated input")
			}
			after, err := os.ReadFile(path)
			if err != nil || string(after) != string(contents) || !reflect.DeepEqual(lib.List(), []Macro{original}) {
				t.Fatal("validation mutated library")
			}
			invalid := `{"version":1,"macros":[{"id":"x","name":"invalid","events":` + text + `}]}`
			if err := os.WriteFile(path, []byte(invalid), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := Open(path); err == nil {
				t.Fatal("invalid stored event accepted")
			}
			after, err = os.ReadFile(path)
			if err != nil || string(after) != invalid {
				t.Fatal("open changed invalid file")
			}
		})
	}
}

func TestKeyboardLocalLegacyAndAdmission(t *testing.T) {
	_, path := openTest(t)
	const old = `{"version":1,"macros":[{"id":"old","name":"mouse","events":[{"type":"mouse_left","action":"down","delay_ms":0},{"type":"mouse_left","action":"up","delay_ms":17}]}]}`
	if err := os.WriteFile(path, []byte(old), 0600); err != nil {
		t.Fatal(err)
	}
	lib, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	m, err := lib.Read("old")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := lib.Update(m.ID, m.Name, m.Events); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(path)
	if err != nil || string(contents) != old {
		t.Fatalf("mouse JSON changed: %s %v", contents, err)
	}
	for _, buttons := range [][]EventType{{"keyboard"}, {MouseLeft, "keyboard"}} {
		sequence := X6Sequence{Buttons: buttons, Repeat: 1}
		if block, err := EncodeX6SequenceBlock(sequence); err == nil || block != nil {
			t.Fatal("keyboard block admitted")
		}
		if reports, err := EncodeX6SequenceUpload(X6SequenceUpload{Destination: 1, Sequence: sequence}); err == nil || reports != nil {
			t.Fatal("keyboard upload admitted")
		}
	}
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
		`{"version":1,"macros":[{"id":"x","name":"one","events":[{"type":"mouse_arbitrary","action":"down","delay_ms":0}]}]}`,
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

func TestStoredMouseTypesRoundtrip(t *testing.T) {
	lib, path := openTest(t)
	types := []EventType{"mouse_left", "mouse_right", "mouse_middle", "mouse_back", "mouse_forward"}
	for _, kind := range types {
		t.Run(string(kind), func(t *testing.T) {
			events := []Event{{Type: kind, Action: Down, DelayMS: 17}, {Type: kind, Action: Up, DelayMS: 23}}
			created, err := lib.Create("stored", events)
			if err != nil {
				t.Fatal(err)
			}
			updated, err := lib.Update(created.ID, "updated", events)
			if err != nil {
				t.Fatal(err)
			}
			loaded, err := Open(path)
			if err != nil {
				t.Fatal(err)
			}
			got, err := loaded.Read(created.ID)
			if err != nil || !reflect.DeepEqual(got, updated) {
				t.Fatalf("roundtrip: %#v %v", got, err)
			}
		})
	}
}

func TestVersionOneLegacyMouseStore(t *testing.T) {
	_, path := openTest(t)
	text := `{"version":1,"macros":[{"id":"legacy","name":"Old","events":[{"type":"mouse_left","action":"down","delay_ms":0},{"type":"mouse_right","action":"up","delay_ms":19}]}]}`
	if err := os.WriteFile(path, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	lib, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	got, err := lib.Read("legacy")
	if err != nil || got.Events[0].Type != MouseLeft || got.Events[1].Type != MouseRight || got.Events[1].DelayMS != 19 {
		t.Fatalf("legacy: %#v %v", got, err)
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
