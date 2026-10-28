package main

import "testing"

func TestGamePickerOptions(t *testing.T) {
	folder := gamePickerOptionsFor(true, "/home/u")
	if folder.title != "Choose the game folder" || !folder.directories || folder.files || folder.startDir != "" {
		t.Fatalf("folder options = %+v", folder)
	}
	file := gamePickerOptionsFor(false, "/home/u")
	if file.title != "Choose the game executable" || file.directories || !file.files || file.startDir != "/home/u" {
		t.Fatalf("file options = %+v", file)
	}
}
