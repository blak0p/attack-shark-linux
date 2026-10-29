package main

import (
	"errors"
	"testing"
)

func TestGamePickerOptions(t *testing.T) {
	opts := gamePickerOptionsFor()
	if opts.title != "Choose the game folder" || !opts.directories || opts.files {
		t.Fatalf("options = %+v", opts)
	}
}

func TestChooseLauncherPath(t *testing.T) {
	exe := func() (string, error) { return "/usr/bin/x6", nil }
	failing := func() (string, error) { return "", errors.New("no exe") }
	yes := func(string) bool { return true }
	no := func(string) bool { return false }
	cases := []struct {
		name     string
		env      string
		exists   func(string) bool
		lookup   func() (string, error)
		want     string
		wantFail bool
	}{
		{"APPIMAGE set and exists", "/home/u/.local/x6.AppImage", yes, exe, "/home/u/.local/x6.AppImage", false},
		{"APPIMAGE set but missing", "/home/u/gone.AppImage", no, exe, "/usr/bin/x6", false},
		{"relative APPIMAGE ignored", "x6.AppImage", yes, exe, "/usr/bin/x6", false},
		{"unset uses executable", "", yes, exe, "/usr/bin/x6", false},
		{"lookup error", "", yes, failing, "", true},
		{"missing APPIMAGE and lookup error", "/gone", no, failing, "", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := chooseLauncherPath(c.env, c.exists, c.lookup)
			if (err != nil) != c.wantFail || got != c.want {
				t.Fatalf("got %q, %v", got, err)
			}
		})
	}
}
