package desktop

import (
	"errors"
	"reflect"
	"testing"
)

type fakeAutostart struct {
	on   bool
	err  error
	sets []bool
}

func (f *fakeAutostart) attach(s *Service) *Service {
	return s.AttachAutostart(func() bool { return f.on }, func(v bool) error {
		f.sets = append(f.sets, v)
		if f.err != nil {
			return f.err
		}
		f.on = v
		return nil
	})
}

func TestGetAutostartReportsAttachedState(t *testing.T) {
	f := &fakeAutostart{on: true}
	if got := f.attach(&Service{}).GetAutostart(); !got.Enabled || got.Error.Code != "" {
		t.Fatalf("snapshot = %+v", got)
	}
}

func TestSetAutostartEnabledStoresAndReturnsNewState(t *testing.T) {
	f := &fakeAutostart{}
	s := f.attach(&Service{})
	if got := s.SetAutostartEnabled(true); !got.Enabled || got.Error.Code != "" {
		t.Fatalf("enable = %+v", got)
	}
	if got := s.SetAutostartEnabled(false); got.Enabled || got.Error.Code != "" {
		t.Fatalf("disable = %+v", got)
	}
	if !reflect.DeepEqual(f.sets, []bool{true, false}) {
		t.Fatalf("set calls = %v", f.sets)
	}
}

func TestSetAutostartFailureKeepsStoredState(t *testing.T) {
	f := &fakeAutostart{on: true, err: errors.New("read-only home")}
	got := f.attach(&Service{}).SetAutostartEnabled(false)
	if got.Error.Code != AutostartUnavailable || !got.Enabled {
		t.Fatalf("failed disable must report the unchanged state and the error, got %+v", got)
	}
}

func TestAutostartUnavailableWithoutAttachment(t *testing.T) {
	s := &Service{}
	if got := s.GetAutostart(); got.Enabled || got.Error.Code != AutostartUnavailable {
		t.Fatalf("get = %+v", got)
	}
	if got := s.SetAutostartEnabled(true); got.Enabled || got.Error.Code != AutostartUnavailable {
		t.Fatalf("set = %+v", got)
	}
	if AutostartUnavailable != "autostart_unavailable" {
		t.Fatal("error code string is part of the frontend contract")
	}
}

func TestAutostartSnapshotKeysArePascalCase(t *testing.T) {
	if got, want := keysOf(t, AutostartSnapshot{}), []string{"Enabled", "Error"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("keys = %v", got)
	}
}
