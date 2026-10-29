package desktop

import (
	"encoding/json"
	"errors"
	"reflect"
	"sort"
	"testing"
)

func keysOf(t *testing.T, v any) []string {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func TestListInstalledGamesKeepsCatalogOrder(t *testing.T) {
	in := []InstalledGame{
		{Name: "Zed", Source: "Steam", Folder: "/g/zed", Executable: "zed.exe", IsGame: true},
		{Name: "Alpha", Source: "App", Executable: "alpha"},
	}
	snap := (&Service{}).AttachGameCatalog(func() []InstalledGame { return in }).ListInstalledGames()
	if snap.Error.Code != "" || !reflect.DeepEqual(snap.Games, in) {
		t.Fatalf("snapshot = %+v", snap)
	}
}

func TestListInstalledGamesUnavailableAndNeverNil(t *testing.T) {
	snap := (&Service{}).ListInstalledGames()
	if snap.Error.Code != GamesUnavailable || snap.Games == nil || len(snap.Games) != 0 {
		t.Fatalf("snapshot = %+v", snap)
	}
	snap = (&Service{}).AttachGameCatalog(func() []InstalledGame { return nil }).ListInstalledGames()
	if snap.Error.Code != "" || snap.Games == nil {
		t.Fatalf("empty catalog snapshot = %+v", snap)
	}
}

func TestGameJSONKeysArePascalCase(t *testing.T) {
	if got, want := keysOf(t, GameListSnapshot{}), []string{"Error", "Games"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("GameListSnapshot keys = %v", got)
	}
	if got, want := keysOf(t, InstalledGame{}), []string{"Executable", "Folder", "IsGame", "Name", "Source"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("InstalledGame keys = %v", got)
	}
	if got, want := keysOf(t, PickResult{}), []string{"Cancelled", "Error", "Name", "Path"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("PickResult keys = %v", got)
	}
}

func pickerReturning(path string, err error) func() (string, error) {
	return func() (string, error) { return path, err }
}

func TestPickGameFolder(t *testing.T) {
	r := (&Service{}).AttachGamePicker(pickerReturning("/games/Hades//", nil)).PickGameFolder()
	want := PickResult{Path: "/games/Hades", Name: "Hades"}
	if r != want {
		t.Fatalf("result = %+v", r)
	}
}

func TestPickCancelled(t *testing.T) {
	r := (&Service{}).AttachGamePicker(pickerReturning("", nil)).PickGameFolder()
	if r != (PickResult{Cancelled: true}) {
		t.Fatalf("result = %+v", r)
	}
}

func TestPickFailures(t *testing.T) {
	cases := map[string]func() (string, error){
		"error":    pickerReturning("/x", errors.New("boom")),
		"relative": pickerReturning("games/Hades", nil),
	}
	for name, p := range cases {
		r := (&Service{}).AttachGamePicker(p).PickGameFolder()
		if r != (PickResult{Error: Error{Code: PickFailed}}) {
			t.Fatalf("%s: result = %+v", name, r)
		}
	}
}

func TestPickWithoutPicker(t *testing.T) {
	if r := (&Service{}).PickGameFolder(); r != (PickResult{Error: Error{Code: GamesUnavailable}}) {
		t.Fatalf("result = %+v", r)
	}
}

func TestSecondPickWhileOpenIsRejected(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	calls := 0
	s := (&Service{}).AttachGamePicker(func() (string, error) {
		calls++
		close(entered)
		<-release
		return "/games/Hades", nil
	})
	done := make(chan PickResult)
	go func() { done <- s.PickGameFolder() }()
	<-entered
	second := s.PickGameFolder()
	if second != (PickResult{Error: Error{Code: PickFailed}}) {
		t.Fatalf("second = %+v", second)
	}
	close(release)
	if first := <-done; first.Path != "/games/Hades" {
		t.Fatalf("first = %+v", first)
	}
	if calls != 1 {
		t.Fatalf("picker opened %d times", calls)
	}
}
