package main

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/blak0p/attack-shark-linux/internal/desktop"
)

const (
	idWork = "0123456789abcdef0123456789abcdef"
	idGame = "fedcba9876543210fedcba9876543210"
)

type fakeSwitcher struct {
	snapshot desktop.ProfileSnapshot
	// switchResult maps a profile ID to the snapshot SwitchProfile returns.
	switchResult map[string]desktop.ProfileSnapshot
	block        chan struct{}
	// refreshDelay and profilesDelay slow the pre-switch steps.
	refreshDelay  time.Duration
	profilesDelay time.Duration
	mu            sync.Mutex
	switched      []string
}

func (f *fakeSwitcher) RefreshInventory(context.Context) desktop.Inventory {
	time.Sleep(f.refreshDelay)
	return desktop.Inventory{}
}

func (f *fakeSwitcher) GetProfiles() desktop.ProfileSnapshot {
	time.Sleep(f.profilesDelay)
	return f.snapshot
}

func (f *fakeSwitcher) SwitchProfile(id string) desktop.ProfileSnapshot {
	f.mu.Lock()
	f.switched = append(f.switched, id)
	f.mu.Unlock()
	if f.block != nil {
		<-f.block
	}
	return f.switchResult[id]
}

func (f *fakeSwitcher) calls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.switched...)
}

func newFakeSwitcher() *fakeSwitcher {
	return &fakeSwitcher{
		snapshot: desktop.ProfileSnapshot{
			ActiveID: idWork,
			Profiles: []desktop.ProfileSummary{{ID: idWork, Name: "Work"}, {ID: idGame, Name: "Game"}},
		},
	}
}

func wrapperFixture(sw *fakeSwitcher) (wrapperDependencies, *bytes.Buffer) {
	stderr := &bytes.Buffer{}
	return wrapperDependencies{
		lockHeld:    func() (bool, error) { return false, nil },
		newSwitcher: func() (profileSwitcher, error) { return sw, nil },
		timeout:     2 * time.Second,
		stderr:      stderr,
	}, stderr
}

func TestWrapperAppliesProfileRunsCommandAndRestores(t *testing.T) {
	sw := newFakeSwitcher()
	deps, stderr := wrapperFixture(sw)

	got := runWrapper("Game", []string{"sh", "-c", "exit 3"}, deps)

	if got != 3 {
		t.Fatalf("exit = %d, want the command's 3", got)
	}
	if want := []string{idGame, idWork}; !reflect.DeepEqual(sw.calls(), want) {
		t.Fatalf("switches = %v, want %v", sw.calls(), want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("unexpected stderr: %q", stderr)
	}
}

func TestWrapperAlreadyActiveProfileIsRewritten(t *testing.T) {
	sw := newFakeSwitcher()
	deps, _ := wrapperFixture(sw)

	runWrapper("Work", []string{"true"}, deps)

	if want := []string{idWork, idWork}; !reflect.DeepEqual(sw.calls(), want) {
		t.Fatalf("switches = %v, want %v", sw.calls(), want)
	}
}

func TestWrapperAcceptsRawProfileID(t *testing.T) {
	sw := newFakeSwitcher()
	deps, _ := wrapperFixture(sw)

	runWrapper(idGame, []string{"true"}, deps)

	if len(sw.calls()) == 0 || sw.calls()[0] != idGame {
		t.Fatalf("switches = %v, want first %s", sw.calls(), idGame)
	}
}

func TestWrapperLeavesMouseAloneWhenAppHoldsLock(t *testing.T) {
	sw := newFakeSwitcher()
	deps, _ := wrapperFixture(sw)
	deps.lockHeld = func() (bool, error) { return true, nil }
	deps.newSwitcher = func() (profileSwitcher, error) {
		t.Fatal("switcher must not be built while the app holds the lock")
		return nil, nil
	}

	if got := runWrapper("Game", []string{"sh", "-c", "exit 4"}, deps); got != 4 {
		t.Fatalf("exit = %d, want 4", got)
	}
}

func TestWrapperWarnsAndStillLaunchesOnProfileFailures(t *testing.T) {
	tests := []struct {
		name string
		ref  string
		prep func(*fakeSwitcher, *wrapperDependencies)
		warn string
	}{
		{name: "unknown name", ref: "Nope", prep: func(*fakeSwitcher, *wrapperDependencies) {}, warn: "Nope"},
		{name: "corrupt store", ref: "Game", warn: "profile", prep: func(sw *fakeSwitcher, _ *wrapperDependencies) {
			sw.snapshot = desktop.ProfileSnapshot{Error: desktop.Error{Code: desktop.InvalidProfile}}
		}},
		{name: "switch refused", ref: "Game", warn: "switch", prep: func(sw *fakeSwitcher, _ *wrapperDependencies) {
			sw.switchResult = map[string]desktop.ProfileSnapshot{idGame: {Error: desktop.Error{Code: desktop.ProfileUnavailable}}}
		}},
		{name: "no device service", ref: "Game", warn: "unavailable", prep: func(_ *fakeSwitcher, d *wrapperDependencies) {
			d.newSwitcher = func() (profileSwitcher, error) { return nil, errors.New("unavailable") }
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sw := newFakeSwitcher()
			deps, stderr := wrapperFixture(sw)
			tt.prep(sw, &deps)

			if got := runWrapper(tt.ref, []string{"sh", "-c", "exit 5"}, deps); got != 5 {
				t.Fatalf("exit = %d, want the command's 5", got)
			}
			if !strings.Contains(stderr.String(), "warning") || !strings.Contains(stderr.String(), tt.warn) {
				t.Fatalf("stderr = %q, want warning mentioning %q", stderr, tt.warn)
			}
		})
	}
}

func TestWrapperNeverDelaysCommandPastTimeout(t *testing.T) {
	sw := newFakeSwitcher()
	sw.block = make(chan struct{})
	defer close(sw.block)
	deps, stderr := wrapperFixture(sw)
	deps.timeout = 50 * time.Millisecond

	start := time.Now()
	got := runWrapper("Game", []string{"true"}, deps)

	if got != 0 {
		t.Fatalf("exit = %d, want 0", got)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("wrapper took %s with a blocked mouse", elapsed)
	}
	if !strings.Contains(stderr.String(), "timed out") {
		t.Fatalf("stderr = %q, want a timeout warning", stderr)
	}
	// The abandoned switch is still writing to the mouse: a restore must not
	// run concurrently with it.
	if want := []string{idGame}; !reflect.DeepEqual(sw.calls(), want) {
		t.Fatalf("switches = %v, want only the abandoned %v", sw.calls(), want)
	}
	if !strings.Contains(stderr.String(), "restor") {
		t.Fatalf("stderr = %q, want a warning that the restore was skipped", stderr)
	}
}

func TestWrapperPreLaunchBudgetIsTotalNotPerStep(t *testing.T) {
	sw := newFakeSwitcher()
	sw.refreshDelay = 150 * time.Millisecond
	sw.profilesDelay = 150 * time.Millisecond
	deps, stderr := wrapperFixture(sw)
	deps.timeout = 200 * time.Millisecond

	start := time.Now()
	runWrapper("Game", []string{"true"}, deps)

	if elapsed := time.Since(start); elapsed > 330*time.Millisecond {
		t.Fatalf("pre-launch work took %s, want it bounded by one %s budget", elapsed, deps.timeout)
	}
	if len(sw.calls()) != 0 || !strings.Contains(stderr.String(), "timed out") {
		t.Fatalf("switches = %v, stderr = %q; want no switch and a timeout warning", sw.calls(), stderr)
	}
}

func TestWrapperRestoresAfterApplyFailedSwitch(t *testing.T) {
	sw := newFakeSwitcher()
	sw.switchResult = map[string]desktop.ProfileSnapshot{idGame: {Error: desktop.Error{Code: desktop.ApplyFailed}}}
	deps, stderr := wrapperFixture(sw)

	runWrapper("Game", []string{"true"}, deps)

	// apply_failed may leave the device half-written, so the previous profile
	// is written back.
	if want := []string{idGame, idWork}; !reflect.DeepEqual(sw.calls(), want) {
		t.Fatalf("switches = %v, want %v", sw.calls(), want)
	}
	if !strings.Contains(stderr.String(), "switch failed") {
		t.Fatalf("stderr = %q, want the switch failure", stderr)
	}
}

func TestWrapperDoesNotRestoreWhenNothingWasWritten(t *testing.T) {
	sw := newFakeSwitcher()
	sw.switchResult = map[string]desktop.ProfileSnapshot{idGame: {Error: desktop.Error{Code: desktop.ProfileUnavailable}}}
	deps, _ := wrapperFixture(sw)

	runWrapper("Game", []string{"true"}, deps)

	if want := []string{idGame}; !reflect.DeepEqual(sw.calls(), want) {
		t.Fatalf("switches = %v, want %v", sw.calls(), want)
	}
}

func TestWrapperSkipsRestoreWhenAppTookTheMouse(t *testing.T) {
	sw := newFakeSwitcher()
	deps, _ := wrapperFixture(sw)
	checks := 0
	deps.lockHeld = func() (bool, error) {
		checks++
		return checks > 1, nil // free before launch, held by the app afterwards
	}

	runWrapper("Game", []string{"true"}, deps)

	if want := []string{idGame}; !reflect.DeepEqual(sw.calls(), want) {
		t.Fatalf("switches = %v, want %v", sw.calls(), want)
	}
}

func TestWrapperDoesNotForwardSIGINT(t *testing.T) {
	sw := newFakeSwitcher()
	deps, _ := wrapperFixture(sw)
	signals := make(chan os.Signal, 2)
	deps.signals = signals
	go func() {
		time.Sleep(150 * time.Millisecond)
		// The terminal already delivers Ctrl-C to the command; forwarding it too
		// would interrupt the command twice.
		signals <- syscall.SIGINT
		time.Sleep(150 * time.Millisecond)
		signals <- syscall.SIGTERM
	}()

	got := runWrapper("Game", []string{"sleep", "30"}, deps)

	if want := 128 + int(syscall.SIGTERM); got != want {
		t.Fatalf("exit = %d, want %d (SIGINT must not reach the command)", got, want)
	}
}

func TestWrapperReportsMissingCommandAndStillRestores(t *testing.T) {
	sw := newFakeSwitcher()
	deps, stderr := wrapperFixture(sw)

	got := runWrapper("Game", []string{"/definitely/not/a/command"}, deps)

	if got != exitCommandNotFound {
		t.Fatalf("exit = %d, want %d", got, exitCommandNotFound)
	}
	if !strings.Contains(stderr.String(), "/definitely/not/a/command") {
		t.Fatalf("stderr = %q, want the command named", stderr)
	}
	if want := []string{idGame, idWork}; !reflect.DeepEqual(sw.calls(), want) {
		t.Fatalf("switches = %v, want %v", sw.calls(), want)
	}
}

func TestWrapperForwardsSignalAndRestores(t *testing.T) {
	sw := newFakeSwitcher()
	deps, _ := wrapperFixture(sw)
	signals := make(chan os.Signal, 1)
	deps.signals = signals
	go func() {
		time.Sleep(200 * time.Millisecond)
		signals <- syscall.SIGTERM
	}()

	got := runWrapper("Game", []string{"sleep", "30"}, deps)

	if want := 128 + int(syscall.SIGTERM); got != want {
		t.Fatalf("exit = %d, want %d", got, want)
	}
	if want := []string{idGame, idWork}; !reflect.DeepEqual(sw.calls(), want) {
		t.Fatalf("switches = %v, want %v", sw.calls(), want)
	}
}

func TestQuietDiagnosticsSilencesLogsAndRestores(t *testing.T) {
	previous := slog.Default()
	restore := quietDiagnostics()
	if slog.Default().Enabled(context.Background(), slog.LevelError) {
		t.Fatal("slog still enabled while the wrapper is quiet")
	}
	restore()
	if slog.Default() != previous {
		t.Fatal("slog default was not restored")
	}
}

func TestWrapperSignalBeforeLaunchAbortsAndRestores(t *testing.T) {
	sw := newFakeSwitcher()
	sw.refreshDelay = 200 * time.Millisecond
	deps, _ := wrapperFixture(sw)
	signals := make(chan os.Signal, 1)
	deps.signals = signals
	marker := filepath.Join(t.TempDir(), "ran")
	go func() {
		time.Sleep(50 * time.Millisecond)
		signals <- syscall.SIGTERM
	}()

	got := runWrapper("Game", []string{"touch", marker}, deps)

	if want := 128 + int(syscall.SIGTERM); got != want {
		t.Fatalf("exit = %d, want %d", got, want)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("the command ran although the launch was aborted")
	}
	if want := []string{idGame, idWork}; !reflect.DeepEqual(sw.calls(), want) {
		t.Fatalf("switches = %v, want %v", sw.calls(), want)
	}
}
