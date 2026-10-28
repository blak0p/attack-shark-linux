package configstore_test

import (
	"bytes"
	"encoding/json"
	"errors"
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

var uniqueID = mouse.DeviceID{VendorID: 1, ProductID: 2, Serial: "unique"}

func uniqueStore(t *testing.T) (*configstore.ProfileStore, string, configstore.ProfileSnapshot) {
	t.Helper()
	root := filepath.Join(t.TempDir(), "profiles")
	s := configstore.NewProfileStore(root)
	snap, err := s.Bootstrap(uniqueID, "x6", json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	snap, err = s.Create(uniqueID, "x6", "Work")
	if err != nil {
		t.Fatal(err)
	}
	return s, root, snap
}

func indexBytes(t *testing.T, root string) []byte {
	t.Helper()
	m, err := filepath.Glob(filepath.Join(root, "*", "index.json"))
	if err != nil || len(m) != 1 {
		t.Fatal(m, err)
	}
	raw, err := os.ReadFile(m[0])
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestDuplicateNamesRejectedAndIndexUntouched(t *testing.T) {
	s, root, snap := uniqueStore(t)
	other := snap.Profiles[0].ID
	before := indexBytes(t, root)
	calls := map[string]func() error{
		"create":    func() error { _, err := s.Create(uniqueID, "x6", "Work"); return err },
		"duplicate": func() error { _, err := s.Duplicate(uniqueID, "x6", other, "Work"); return err },
		"rename":    func() error { _, err := s.Rename(uniqueID, "x6", other, "Work"); return err },
	}
	for name, call := range calls {
		if err := call(); !errors.Is(err, configstore.ErrDuplicateProfileName) {
			t.Fatalf("%s: %v", name, err)
		}
		if !bytes.Equal(before, indexBytes(t, root)) {
			t.Fatalf("%s changed index.json", name)
		}
	}
}

func TestRenameToOwnNameAndCaseDifferenceAllowed(t *testing.T) {
	s, _, snap := uniqueStore(t)
	if _, err := s.Rename(uniqueID, "x6", snap.ActiveID, "Work"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Create(uniqueID, "x6", "work"); err != nil {
		t.Fatal(err)
	}
}

func TestUniqueNamesKeepValidOperations(t *testing.T) {
	s, _, snap := uniqueStore(t)
	if _, err := s.Create(uniqueID, "x6", "Game"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Duplicate(uniqueID, "x6", snap.ActiveID, "Copy"); err != nil {
		t.Fatal(err)
	}
	got, err := s.Rename(uniqueID, "x6", snap.ActiveID, "Renamed")
	if err != nil || len(got.Profiles) != 4 {
		t.Fatal(got, err)
	}
	if _, err = s.Create(uniqueID, "x6", ""); !errors.Is(err, configstore.ErrInvalidProfile) {
		t.Fatal(err)
	}
}

func TestExistingDuplicateNamesStillLoadAndSwitch(t *testing.T) {
	s, root, snap := uniqueStore(t)
	m, _ := filepath.Glob(filepath.Join(root, "*", "index.json"))
	var index map[string]any
	if err := json.Unmarshal(indexBytes(t, root), &index); err != nil {
		t.Fatal(err)
	}
	for _, p := range index["profiles"].([]any) {
		p.(map[string]any)["name"] = "Same"
	}
	raw, _ := json.Marshal(index)
	if err := os.WriteFile(m[0], raw, 0600); err != nil {
		t.Fatal(err)
	}
	s = configstore.NewProfileStore(root)
	listed, err := s.List(uniqueID, "x6")
	if err != nil || len(listed.Profiles) != 2 || listed.Profiles[0].Name != listed.Profiles[1].Name {
		t.Fatal(listed, err)
	}
	other := listed.Profiles[0].ID
	if other == snap.ActiveID {
		other = listed.Profiles[1].ID
	}
	if got, err := s.Switch(uniqueID, "x6", other); err != nil || got.ActiveID != other {
		t.Fatal(got, err)
	}
}
