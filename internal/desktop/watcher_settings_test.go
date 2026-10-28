package desktop

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/blak0p/attack-shark-linux/internal/configstore"
)

const watcherHexA = "0123456789abcdef0123456789abcdef"

func newWatcherService(t *testing.T) (*Service, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "watcher-v1.json")
	return (&Service{}).AttachWatcherSettings(configstore.NewWatcherStore(path)), path
}

func TestWatcherSettingsUnavailableWithoutStore(t *testing.T) {
	s := &Service{}
	calls := 0
	s.SetWatcherChangeHandler(func(configstore.WatcherSettings) { calls++ })
	if got := s.GetWatcherSettings(); got.Error.Code != WatcherUnavailable || got.Enabled || len(got.Rules) != 0 {
		t.Fatalf("get = %#v", got)
	}
	if got := s.SetWatcherSettings(true, nil); got.Error.Code != WatcherUnavailable {
		t.Fatalf("set = %#v", got)
	}
	if calls != 0 {
		t.Fatalf("handler calls = %d", calls)
	}
	if string(WatcherUnavailable) != "watcher_unavailable" || string(InvalidWatcherSettings) != "invalid_watcher_settings" {
		t.Fatal("error code strings changed")
	}
}

func TestWatcherSettingsDefaultsWhenNoFile(t *testing.T) {
	s, path := newWatcherService(t)
	got := s.GetWatcherSettings()
	if got.Enabled || len(got.Rules) != 0 || got.Error.Code != "" {
		t.Fatalf("get = %#v", got)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("get created file: %v", err)
	}
}

func TestSetWatcherSettingsPersistsAndCallsHandlerOnce(t *testing.T) {
	s, path := newWatcherService(t)
	var seen []configstore.WatcherSettings
	s.SetWatcherChangeHandler(func(v configstore.WatcherSettings) { seen = append(seen, v) })
	rules := []WatcherRule{{Executable: "game.exe", ProfileID: watcherHexA}}
	got := s.SetWatcherSettings(true, rules)
	if got.Error.Code != "" || !got.Enabled || !reflect.DeepEqual(got.Rules, rules) {
		t.Fatalf("set = %#v", got)
	}
	want := configstore.WatcherSettings{Enabled: true, Rules: rulesToStore(rules)}
	if len(seen) != 1 || !reflect.DeepEqual(seen[0], want) {
		t.Fatalf("handler = %#v", seen)
	}
	if again := s.GetWatcherSettings(); !reflect.DeepEqual(again, got) {
		t.Fatalf("get = %#v want %#v", again, got)
	}
	if len(seen) != 1 {
		t.Fatal("get called handler")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}

func TestSetWatcherSettingsInvalidKeepsStateAndSkipsHandler(t *testing.T) {
	s, path := newWatcherService(t)
	good := []WatcherRule{{Executable: "game", ProfileID: watcherHexA}}
	if r := s.SetWatcherSettings(true, good); r.Error.Code != "" {
		t.Fatal(r)
	}
	before, _ := os.ReadFile(path)
	calls := 0
	s.SetWatcherChangeHandler(func(configstore.WatcherSettings) { calls++ })
	got := s.SetWatcherSettings(false, []WatcherRule{{Executable: "a/b", ProfileID: watcherHexA}})
	if got.Error.Code != InvalidWatcherSettings {
		t.Fatalf("set = %#v", got)
	}
	if !got.Enabled || !reflect.DeepEqual(got.Rules, good) {
		t.Fatalf("snapshot should show stored settings: %#v", got)
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) || calls != 0 {
		t.Fatalf("file changed or handler called (%d)", calls)
	}
}

func TestWatcherSettingsCorruptFileReportsInvalid(t *testing.T) {
	s, path := newWatcherService(t)
	if err := os.WriteFile(path, []byte("{bad"), 0600); err != nil {
		t.Fatal(err)
	}
	if got := s.GetWatcherSettings(); got.Error.Code != InvalidWatcherSettings {
		t.Fatalf("get = %#v", got)
	}
}

func TestSetWatcherSettingsPersistenceFailure(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "file")
	if err := os.WriteFile(blocker, nil, 0600); err != nil {
		t.Fatal(err)
	}
	s := (&Service{}).AttachWatcherSettings(configstore.NewWatcherStore(filepath.Join(blocker, "w.json")))
	calls := 0
	s.SetWatcherChangeHandler(func(configstore.WatcherSettings) { calls++ })
	if got := s.SetWatcherSettings(true, nil); got.Error.Code != PersistenceFailed || calls != 0 {
		t.Fatalf("set = %#v calls=%d", got, calls)
	}
}

func TestWatcherChangeHandlerMayReadSettings(t *testing.T) {
	s, _ := newWatcherService(t)
	var seen WatcherSettingsSnapshot
	s.SetWatcherChangeHandler(func(configstore.WatcherSettings) { seen = s.GetWatcherSettings() })
	done := make(chan WatcherSettingsSnapshot, 1)
	go func() {
		done <- s.SetWatcherSettings(true, []WatcherRule{{Executable: "game.exe", ProfileID: watcherHexA}})
	}()

	select {
	case got := <-done:
		if got.Error.Code != "" || !seen.Enabled || len(seen.Rules) != 1 {
			t.Fatalf("set = %#v, handler saw %#v", got, seen)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("deadlock: the handler could not read the settings")
	}
}

func TestWatcherSettingsFolderRuleRoundTripAndInvalid(t *testing.T) {
	s, path := newWatcherService(t)
	got := s.SetWatcherSettings(true, []WatcherRule{{Folder: "/games/T/", ProfileID: watcherHexA}})
	want := []WatcherRule{{Folder: "/games/T", Name: "T", ProfileID: watcherHexA}}
	if got.Error.Code != "" || !reflect.DeepEqual(got.Rules, want) {
		t.Fatalf("set = %#v", got)
	}
	if again := s.GetWatcherSettings(); !reflect.DeepEqual(again.Rules, want) {
		t.Fatalf("get = %#v", again)
	}
	before, _ := os.ReadFile(path)
	bad := s.SetWatcherSettings(true, []WatcherRule{{Executable: "a", Folder: "/g", ProfileID: watcherHexA}})
	if bad.Error.Code != InvalidWatcherSettings || !reflect.DeepEqual(bad.Rules, want) {
		t.Fatalf("bad = %#v", bad)
	}
	if after, _ := os.ReadFile(path); string(after) != string(before) {
		t.Fatal("file changed on rejection")
	}
}

func TestWatcherSettingsJSONUsesPascalCaseKeys(t *testing.T) {
	s, _ := newWatcherService(t)
	s.SetWatcherSettings(true, []WatcherRule{{Executable: "game.exe", Name: "G", ProfileID: watcherHexA}})
	encoded, err := json.Marshal(s.GetWatcherSettings())
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct{ Rules []map[string]any }
	if err := json.Unmarshal(encoded, &decoded); err != nil || len(decoded.Rules) != 1 {
		t.Fatalf("decode %s: %v", encoded, err)
	}
	keys := make([]string, 0, 4)
	for k := range decoded.Rules[0] {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if !reflect.DeepEqual(keys, []string{"Executable", "Folder", "Name", "ProfileID"}) {
		t.Fatalf("keys = %v in %s", keys, encoded)
	}
}

func TestSetWatcherSettingsAcceptsFrontendJSON(t *testing.T) {
	s, _ := newWatcherService(t)
	var rules []WatcherRule
	input := `[{"Executable":"game.exe","ProfileID":"` + watcherHexA + `"}]`
	if err := json.Unmarshal([]byte(input), &rules); err != nil {
		t.Fatal(err)
	}
	got := s.SetWatcherSettings(true, rules)
	want := []WatcherRule{{Executable: "game.exe", ProfileID: watcherHexA}}
	if got.Error.Code != "" || !reflect.DeepEqual(got.Rules, want) {
		t.Fatalf("set = %#v", got)
	}
	if again := s.GetWatcherSettings(); !reflect.DeepEqual(again.Rules, want) {
		t.Fatalf("get = %#v", again)
	}
}

func TestSetWatcherSettingsPreservesTray(t *testing.T) {
	s, _ := newWatcherService(t)
	rules := []WatcherRule{{Executable: "game.exe", ProfileID: watcherHexA}}
	if got := s.SetTrayEnabled(true); !got.Tray || got.Error.Code != "" {
		t.Fatalf("tray = %#v", got)
	}
	got := s.SetWatcherSettings(true, rules)
	if got.Error.Code != "" || !got.Tray || !got.Enabled {
		t.Fatalf("set = %#v", got)
	}
	if again := s.GetWatcherSettings(); !again.Tray {
		t.Fatalf("get = %#v", again)
	}
}

func TestSetTrayEnabledKeepsEnabledAndRulesAndCallsHandlerOnce(t *testing.T) {
	s, _ := newWatcherService(t)
	rules := []WatcherRule{{Executable: "game.exe", ProfileID: watcherHexA}}
	s.SetWatcherSettings(true, rules)
	var seen []configstore.WatcherSettings
	s.SetWatcherChangeHandler(func(v configstore.WatcherSettings) { seen = append(seen, v) })
	got := s.SetTrayEnabled(true)
	if got.Error.Code != "" || !got.Tray || !got.Enabled || !reflect.DeepEqual(got.Rules, rules) {
		t.Fatalf("set = %#v", got)
	}
	if len(seen) != 1 || !seen[0].Tray || !seen[0].Enabled || len(seen[0].Rules) != 1 {
		t.Fatalf("handler = %#v", seen)
	}
	if off := s.SetTrayEnabled(false); off.Tray || !off.Enabled || len(off.Rules) != 1 {
		t.Fatalf("off = %#v", off)
	}
}

func TestSetTrayEnabledErrors(t *testing.T) {
	s := &Service{}
	calls := 0
	s.SetWatcherChangeHandler(func(configstore.WatcherSettings) { calls++ })
	if got := s.SetTrayEnabled(true); got.Error.Code != WatcherUnavailable || got.Tray {
		t.Fatalf("unavailable = %#v", got)
	}
	s, path := newWatcherService(t)
	s.SetWatcherChangeHandler(func(configstore.WatcherSettings) { calls++ })
	if err := os.WriteFile(path, []byte("{bad"), 0600); err != nil {
		t.Fatal(err)
	}
	if got := s.SetTrayEnabled(true); got.Error.Code != InvalidWatcherSettings {
		t.Fatalf("invalid = %#v", got)
	}
	if contents, _ := os.ReadFile(path); string(contents) != "{bad" {
		t.Fatalf("file changed: %s", contents)
	}
	if calls != 0 {
		t.Fatalf("handler calls = %d", calls)
	}
}

func TestWatcherSnapshotJSONHasTrayKey(t *testing.T) {
	s, _ := newWatcherService(t)
	s.SetTrayEnabled(true)
	encoded, err := json.Marshal(s.GetWatcherSettings())
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil || decoded["Tray"] != true {
		t.Fatalf("decode %s: %v", encoded, err)
	}
}

func TestAddWatcherChangeHandlerNotifiesEveryListener(t *testing.T) {
	s, _ := newWatcherService(t)
	var first, second, third int
	s.SetWatcherChangeHandler(func(configstore.WatcherSettings) { first++ })
	s.AddWatcherChangeHandler(func(configstore.WatcherSettings) { second++ })
	s.AddWatcherChangeHandler(func(configstore.WatcherSettings) { third++ })
	s.SetTrayEnabled(true)
	s.SetWatcherSettings(true, nil)
	if first != 2 || second != 2 || third != 2 {
		t.Fatalf("calls = %d %d %d", first, second, third)
	}
	s.SetWatcherChangeHandler(nil)
	s.SetTrayEnabled(false)
	if first != 2 || second != 3 || third != 3 {
		t.Fatalf("after clear = %d %d %d", first, second, third)
	}
}
