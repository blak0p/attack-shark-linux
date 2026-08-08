package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestWailsConfigurationBuildsFrontendWithoutInvokingNativeBuild(t *testing.T) {
	contents, err := os.ReadFile("wails.json")
	if err != nil {
		t.Fatalf("read Wails configuration: %v", err)
	}

	var config struct {
		Frontend struct {
			Build string `json:"build"`
		} `json:"frontend"`
	}
	if err := json.Unmarshal(contents, &config); err != nil {
		t.Fatalf("parse Wails configuration: %v", err)
	}
	if config.Frontend.Build != "npm run build" {
		t.Fatalf("frontend build = %q, want npm run build", config.Frontend.Build)
	}
	if strings.Contains(config.Frontend.Build, "wails3 build") {
		t.Fatal("frontend build must not invoke wails3 build recursively")
	}
}

func TestNewDesktopServiceUsesDurableAppliedState(t *testing.T) {
	service := newDesktopService(t.TempDir())

	snapshot := service.GetSnapshot()
	if snapshot.Pending.DPI[0] != 800 || snapshot.Applied.DPI[0] != 800 {
		t.Fatalf("initial snapshot = %#v, want the default persisted DPI configuration", snapshot)
	}
}

func TestWailsBuildTaskDoesNotRecursivelyInvokeWails(t *testing.T) {
	contents, err := os.ReadFile("Taskfile.yml")
	if err != nil {
		t.Fatalf("read Wails build task: %v", err)
	}
	if strings.Contains(string(contents), "wails3 build") {
		t.Fatal("Wails build task must not recursively invoke wails3 build")
	}
	if !strings.Contains(string(contents), "go build") {
		t.Fatal("Wails build task must compile the composition root directly")
	}
}
