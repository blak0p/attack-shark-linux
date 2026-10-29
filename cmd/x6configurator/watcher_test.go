package main

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/blak0p/attack-shark-linux/internal/configstore"
	"github.com/blak0p/attack-shark-linux/internal/desktop"
	"github.com/blak0p/attack-shark-linux/internal/procwatch"
)

type runRecord struct {
	rules   []procwatch.Rule
	stopped chan struct{}
}

type fakeRunner struct {
	mu   sync.Mutex
	runs []*runRecord
}

func (f *fakeRunner) run(ctx context.Context, rules []procwatch.Rule) {
	rec := &runRecord{rules: rules, stopped: make(chan struct{})}
	f.mu.Lock()
	f.runs = append(f.runs, rec)
	f.mu.Unlock()
	<-ctx.Done()
	close(rec.stopped)
}

func (f *fakeRunner) started() []*runRecord {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]*runRecord(nil), f.runs...)
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func stopped(rec *runRecord) bool {
	select {
	case <-rec.stopped:
		return true
	default:
		return false
	}
}

var gameSettings = configstore.WatcherSettings{
	Enabled: true,
	Rules:   []configstore.WatcherRule{{Executable: "game.exe", ProfileID: idGame}},
}

func TestWatcherControllerStartsEnabledRulesInOrder(t *testing.T) {
	runner := &fakeRunner{}
	c := newWatcherController(runner.run)
	defer c.Stop()

	c.Apply(configstore.WatcherSettings{Enabled: true, Rules: []configstore.WatcherRule{
		{Executable: "a", ProfileID: idWork}, {Executable: "b", ProfileID: idGame},
	}})

	waitFor(t, "the watcher to start", func() bool { return len(runner.started()) == 1 })
	want := []procwatch.Rule{{Executable: "a", ProfileID: idWork}, {Executable: "b", ProfileID: idGame}}
	if got := runner.started()[0].rules; !reflect.DeepEqual(got, want) {
		t.Fatalf("rules = %v, want %v", got, want)
	}
}

func TestWatcherControllerDoesNotStartWhenDisabledOrEmpty(t *testing.T) {
	runner := &fakeRunner{}
	c := newWatcherController(runner.run)
	defer c.Stop()

	c.Apply(configstore.WatcherSettings{Enabled: false, Rules: gameSettings.Rules})
	c.Apply(configstore.WatcherSettings{Enabled: true})
	time.Sleep(50 * time.Millisecond)

	if n := len(runner.started()); n != 0 {
		t.Fatalf("%d watcher runs started, want none", n)
	}
}

func TestWatcherControllerReapplyStopsThePreviousRun(t *testing.T) {
	runner := &fakeRunner{}
	c := newWatcherController(runner.run)
	defer c.Stop()

	c.Apply(gameSettings)
	waitFor(t, "first run", func() bool { return len(runner.started()) == 1 })
	c.Apply(configstore.WatcherSettings{Enabled: true, Rules: []configstore.WatcherRule{{Executable: "other", ProfileID: idWork}}})
	waitFor(t, "second run", func() bool { return len(runner.started()) == 2 })

	if !stopped(runner.started()[0]) {
		t.Fatal("the previous run is still active")
	}
}

func TestWatcherControllerDisablingStopsTheRun(t *testing.T) {
	runner := &fakeRunner{}
	c := newWatcherController(runner.run)
	defer c.Stop()

	c.Apply(gameSettings)
	waitFor(t, "run", func() bool { return len(runner.started()) == 1 })
	c.Apply(configstore.WatcherSettings{Enabled: false, Rules: gameSettings.Rules})

	waitFor(t, "the run to stop", func() bool { return stopped(runner.started()[0]) })
	if n := len(runner.started()); n != 1 {
		t.Fatalf("%d runs, want only the first", n)
	}
}

func TestWatcherControllerStopEndsTheRun(t *testing.T) {
	runner := &fakeRunner{}
	c := newWatcherController(runner.run)

	c.Apply(gameSettings)
	waitFor(t, "run", func() bool { return len(runner.started()) == 1 })
	c.Stop()

	waitFor(t, "the run to stop", func() bool { return stopped(runner.started()[0]) })
}

func TestProfileSwitcherAdapter(t *testing.T) {
	sw := newFakeSwitcher()
	adapter := serviceProfileSwitcher{service: sw}

	if got := adapter.ActiveID(); got != idWork {
		t.Fatalf("ActiveID = %q, want %q", got, idWork)
	}
	if err := adapter.Switch(idGame); err != nil {
		t.Fatalf("Switch: %v", err)
	}
	if want := []string{idGame}; !reflect.DeepEqual(sw.calls(), want) {
		t.Fatalf("switches = %v, want %v", sw.calls(), want)
	}

	sw.switchResult = map[string]desktop.ProfileSnapshot{idWork: {Error: desktop.Error{Code: desktop.ApplyFailed}}}
	err := adapter.Switch(idWork)
	if err == nil || !errors.Is(err, errSwitchRefused) {
		t.Fatalf("Switch error = %v, want errSwitchRefused", err)
	}

	sw.snapshot = desktop.ProfileSnapshot{Error: desktop.Error{Code: desktop.ProfileUnavailable}}
	if got := adapter.ActiveID(); got != "" {
		t.Fatalf("ActiveID with unavailable profiles = %q, want empty", got)
	}
}

func TestProfileSwitcherAdapterLogsEachSwitch(t *testing.T) {
	var buf bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	defer slog.SetDefault(previous)

	sw := newFakeSwitcher()
	adapter := serviceProfileSwitcher{service: sw}
	if err := adapter.Switch(idGame); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); !strings.Contains(got, "process watcher switched profile") || !strings.Contains(got, idGame) {
		t.Fatalf("success not logged: %q", got)
	}

	buf.Reset()
	sw.switchResult = map[string]desktop.ProfileSnapshot{idWork: {Error: desktop.Error{Code: desktop.ApplyFailed}}}
	if err := adapter.Switch(idWork); err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(buf.String(), "switched profile") {
		t.Fatalf("a refused switch must not be logged as success: %q", buf.String())
	}
}

func TestWatcherControllerPassesFolderToProcwatch(t *testing.T) {
	runner := &fakeRunner{}
	c := newWatcherController(runner.run)
	defer c.Stop()

	c.Apply(configstore.WatcherSettings{Enabled: true, Rules: []configstore.WatcherRule{
		{Folder: "/games/T", Name: "T", ProfileID: idGame}, {Executable: "b", ProfileID: idWork},
	}})

	waitFor(t, "the watcher to start", func() bool { return len(runner.started()) == 1 })
	want := []procwatch.Rule{{Folder: "/games/T", ProfileID: idGame}, {Executable: "b", ProfileID: idWork}}
	if got := runner.started()[0].rules; !reflect.DeepEqual(got, want) {
		t.Fatalf("rules = %v, want %v", got, want)
	}
}
