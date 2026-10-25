package configstore_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/blak0p/attack-shark-linux/internal/configstore"
	"github.com/blak0p/attack-shark-linux/internal/mouse"
)

func TestProfilesLifecycle(t *testing.T) {
	root := filepath.Join(t.TempDir(), "profiles")
	s := configstore.NewProfileStore(root)
	a := mouse.DeviceID{VendorID: 1, ProductID: 2, Serial: "../../a"}
	b := mouse.DeviceID{VendorID: 1, ProductID: 2, Serial: "b"}
	seed := json.RawMessage(`{"lighting":{"mode":2},"macros":[1]}`)
	first, err := s.Bootstrap(a, "x6", seed)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Profiles) != 1 || first.Notice == "" || first.Profiles[0].Name != "Automatic profile" || first.Profiles[0].CreatedAt == "" {
		t.Fatal(first)
	}
	again, err := s.Bootstrap(a, "x6", json.RawMessage(`bad`))
	if err != nil || !reflect.DeepEqual(first, again) {
		t.Fatal(again, err)
	}
	other, err := s.Bootstrap(b, "x6", json.RawMessage(`{}`))
	if err != nil || other.ActiveID == first.ActiveID {
		t.Fatal(other, err)
	}
	clone, err := s.Create(a, "x6", "Gaming")
	if err != nil || clone.ActiveID == first.ActiveID {
		t.Fatal(clone, err)
	}
	got, err := s.Read(a, "x6", clone.ActiveID)
	if err != nil || string(got.Configuration) != string(seed) {
		t.Fatal(got, err)
	}
	dup, err := s.Duplicate(a, "x6", first.ActiveID, "Copy")
	if err != nil || dup.ActiveID != clone.ActiveID || len(dup.Profiles) != 3 {
		t.Fatal(dup, err)
	}
	if err := s.SaveActive(a, "x6", json.RawMessage(`{"lighting":3,"macros":[]}`)); err != nil {
		t.Fatal(err)
	}
	s = configstore.NewProfileStore(root)
	saved, err := s.Read(a, "x6", clone.ActiveID)
	if err != nil || string(saved.Configuration) != `{"lighting":3,"macros":[]}` {
		t.Fatal(saved, err)
	}
	if _, err = s.Delete(a, "x6", clone.ActiveID); err == nil {
		t.Fatal("deleted active")
	}
	renamed, err := s.Rename(a, "x6", first.ActiveID, "Original")
	if err != nil || renamed.Profiles[0].Name != "Original" {
		t.Fatal(renamed, err)
	}
	if _, err = s.Switch(a, "x6", first.ActiveID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Delete(a, "x6", clone.ActiveID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.DismissNotice(a, "x6"); err != nil {
		t.Fatal(err)
	}
	final, err := configstore.NewProfileStore(root).List(a, "x6")
	if err != nil || final.Notice != "" {
		t.Fatal(final, err)
	}
	unchanged, err := s.List(b, "x6")
	if err != nil || !reflect.DeepEqual(other, unchanged) {
		t.Fatal(unchanged, err)
	}
	before := tree(t, root)
	for _, name := range []string{"", "../escape", "a/b", "a\\b", "\n"} {
		if _, err = s.Create(a, "x6", name); err == nil {
			t.Fatalf("accepted %q", name)
		}
	}
	if _, err = s.Switch(a, "x6", "../escape"); err == nil {
		t.Fatal("accepted ID")
	}
	if err = s.SaveActive(a, "x6", json.RawMessage(`null`)); err == nil {
		t.Fatal("accepted null")
	}
	if _, err = s.List(a, "wrong"); err == nil {
		t.Fatal("accepted binding")
	}
	if _, err = s.List(mouse.DeviceID{}, "x6"); err == nil {
		t.Fatal("accepted identity")
	}
	if !reflect.DeepEqual(before, tree(t, root)) {
		t.Fatal("rejection changed files")
	}
}

func TestProfilesInvalidAndPreservation(t *testing.T) {
	root := filepath.Join(t.TempDir(), "profiles")
	s := configstore.NewProfileStore(root)
	id := mouse.DeviceID{VendorID: 1, ProductID: 2}
	for _, seed := range []string{`bad`, `null`, `[]`, `1`} {
		if _, err := s.Bootstrap(id, "x6", json.RawMessage(seed)); err == nil {
			t.Fatal(seed)
		}
	}
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatal("invalid seed wrote root", err)
	}
	state, err := s.Bootstrap(id, "x6", json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Delete(id, "x6", state.ActiveID); err == nil {
		t.Fatal("deleted last")
	}
	files := tree(t, root)
	for path := range files {
		if filepath.Ext(path) != ".json" {
			continue
		}
		original := files[path]
		for _, bad := range []string{`{`, `{"version":999}`} {
			if err := os.WriteFile(path, []byte(bad), 0600); err != nil {
				t.Fatal(err)
			}
			before := tree(t, root)
			if _, err := s.Bootstrap(id, "x6", json.RawMessage(`{}`)); err == nil {
				t.Fatal("overwrote malformed", path)
			}
			if !reflect.DeepEqual(before, tree(t, root)) {
				t.Fatal("changed malformed files")
			}
		}
		if err := os.WriteFile(path, []byte(original), 0600); err != nil {
			t.Fatal(err)
		}
	}
}
func tree(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			b, e := os.ReadFile(path)
			if e != nil {
				return e
			}
			out[path] = string(b)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}
