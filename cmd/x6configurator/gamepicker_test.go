package main

import "testing"

func TestGamePickerOptions(t *testing.T) {
	opts := gamePickerOptionsFor()
	if opts.title != "Choose the game folder" || !opts.directories || opts.files {
		t.Fatalf("options = %+v", opts)
	}
}
