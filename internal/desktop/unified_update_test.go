package desktop

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"reflect"
	"testing"

	"github.com/blak0p/attack-shark-linux/internal/udevauth"
	"github.com/blak0p/attack-shark-linux/internal/update"
	packaged "github.com/blak0p/attack-shark-linux/packaging/udev"
)

type policyFixture struct {
	data        []byte
	missing     bool
	calls       int
	failure     error
	installed   bool
	candidate   *udevauth.Policy
	checkError  error
	readError   error
	unavailable bool
	onApply     func()
	onCheck     func(context.Context)
	checks      int
	received    *udevauth.Policy
}

func (p *policyFixture) Lstat() (fs.FileMode, error) {
	if p.missing {
		return 0, fs.ErrNotExist
	}
	return 0644, nil
}
func (p *policyFixture) Read(int) ([]byte, error) { return p.data, p.readError }
func (p *policyFixture) Check(ctx context.Context) (*udevauth.Policy, error) {
	p.checks++
	if p.onCheck != nil {
		p.onCheck(ctx)
	}
	if p.checkError != nil || p.unavailable {
		return nil, p.checkError
	}
	if p.candidate != nil {
		return p.candidate, nil
	}
	return &udevauth.Policy{Version: "1.2.3", Bytes: []byte("remote")}, nil
}
func (p *policyFixture) Apply(_ context.Context, candidate *udevauth.Policy) (PolicyApplyResult, error) {
	p.calls++
	copy := *candidate
	copy.Bytes = append([]byte(nil), candidate.Bytes...)
	copy.Previous = append([]string(nil), candidate.Previous...)
	p.received = &copy
	if string(candidate.Bytes) != "remote" {
		return PolicyApplyResult{}, errors.New("candidate was replaced")
	}
	if p.onApply != nil {
		p.onApply()
	}
	if p.installed || p.failure == nil {
		p.data = append([]byte(nil), candidate.Bytes...)
		p.missing = false
	}
	return PolicyApplyResult{Installed: p.installed || p.failure == nil}, p.failure
}

func TestUnifiedUpdates(t *testing.T) {
	for _, name := range []string{"both", "policy only", "app only", "neither", "custom", "denial", "partial"} {
		t.Run(name, func(t *testing.T) {
			p := &policyFixture{missing: true}
			app := name != "policy only" && name != "neither" && name != "custom"
			if name == "app only" || name == "neither" {
				p.missing = false
				p.data = []byte("remote")
			}
			if name == "custom" {
				p.missing = false
				p.data = []byte("local")
			}
			if name == "denial" || name == "partial" {
				p.failure = errors.New("denied")
				p.installed = name == "partial"
			}
			s := &Service{}
			applies, launches := 0, 0
			ConfigureUpdater(s, nil, func() error { launches++; return nil })
			s.update.check = func(context.Context) (*update.VerifiedUpdate, error) {
				if !app {
					return nil, update.ErrNoUpdate
				}
				return &update.VerifiedUpdate{Release: update.Release{Version: "1.2.3"}}, nil
			}
			s.update.apply = func(context.Context, *update.VerifiedUpdate, bool) error { applies++; return nil }
			ConfigureUnifiedUpdates(s, p, p)
			original := string(p.data)
			if got, err := s.ApplyUnifiedUpdate(context.Background()); got != nil || err == nil || err.Error() != "check for updates before applying" || p.calls != 0 || applies != 0 || launches != 0 || string(p.data) != original {
				t.Fatalf("unchecked result=%+v error=%v counters=%d/%d/%d bytes=%q", got, err, p.calls, applies, launches, p.data)
			}
			info, err := s.CheckUnifiedUpdates(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			want := UnifiedUpdateInfo{PolicyState: "missing", AppAvailable: app}
			if app {
				want.Version = "1.2.3"
			}
			if name == "app only" || name == "neither" {
				want.PolicyState = "current"
			}
			if name == "custom" {
				want.PolicyState = "custom"
			}
			if !reflect.DeepEqual(*info, want) || p.calls != 0 || applies != 0 || launches != 0 {
				t.Fatalf("check: %+v want %+v counters %d/%d/%d", info, want, p.calls, applies, launches)
			}
			info, err = s.ApplyUnifiedUpdate(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			wantPolicy := name == "both" || name == "policy only" || name == "denial" || name == "partial"
			wantCalls := 0
			if wantPolicy {
				wantCalls = 1
			}
			wantApps := 0
			if app {
				wantApps = 1
			}
			wantLaunch := 0
			if app && p.failure == nil {
				wantLaunch = 1
			}
			want.AppInstalled = app
			want.PolicyInstalled = wantPolicy && p.failure == nil || name == "partial"
			if p.failure != nil {
				want.PolicyError = "denied"
			}
			if p.calls != wantCalls || applies != wantApps || launches != wantLaunch || !reflect.DeepEqual(*info, want) {
				t.Fatalf("apply: %+v counters %d/%d/%d", info, p.calls, applies, launches)
			}
			expectedBytes := original
			if want.PolicyInstalled {
				expectedBytes = "remote"
			}
			if string(p.data) != expectedBytes {
				t.Fatalf("stored bytes=%q want=%q", p.data, expectedBytes)
			}
			if p.failure != nil {
				if info.PolicyError != "denied" {
					t.Fatalf("policy error: %+v", info)
				}
				p.failure = nil
				info, err = s.ApplyUnifiedUpdate(context.Background())
				if err != nil {
					t.Fatal(err)
				}
				expected := 2
				if name == "partial" {
					expected = 1
				}
				want.PolicyInstalled = true
				if name != "partial" {
					want.PolicyError = ""
				}
				if p.calls != expected || applies != 1 || launches != 1 || !reflect.DeepEqual(*info, want) {
					t.Fatalf("retry: %+v counters %d/%d/%d", info, p.calls, applies, launches)
				}
			}
		})
	}
}

func TestUnifiedPolicyInspectionAndRevalidation(t *testing.T) {
	for _, name := range []string{"signed history", "custom after check", "current after check", "unavailable", "authentication error", "inspection error", "candidate mutation", "partial recheck"} {
		t.Run(name, func(t *testing.T) {
			p := &policyFixture{missing: true}
			expectedState, expectedError, wantCalls := "missing", "", 1
			switch name {
			case "signed history":
				p.missing = false
				p.data = []byte(packaged.Policy())
				sum := sha256.Sum256(p.data)
				p.candidate = &udevauth.Policy{Version: "1.2.3", Bytes: []byte("remote"), Previous: []string{fmt.Sprintf("%x", sum)}}
				expectedState = "outdated"
			case "unavailable":
				p.unavailable = true
				expectedState = "unavailable"
				wantCalls = 0
			case "authentication error":
				p.checkError = errors.New("signature rejected")
				expectedState = "error"
				expectedError = "signature rejected"
				wantCalls = 0
			case "inspection error":
				p.missing = false
				p.readError = errors.New("read rejected")
				expectedState = "error"
				expectedError = "read rejected"
				wantCalls = 0
			case "candidate mutation":
				p.candidate = &udevauth.Policy{Version: "1.2.3", Bytes: []byte("remote"), Previous: []string{"signed history"}}
			case "partial recheck":
				p.failure = errors.New("reload failed")
				p.installed = true
			}
			s := &Service{}
			ConfigureUpdater(s, nil, nil)
			ConfigureUnifiedUpdates(s, p, p)
			checked, err := s.CheckUnifiedUpdates(context.Background())
			if err != nil || !reflect.DeepEqual(*checked, UnifiedUpdateInfo{PolicyState: expectedState, PolicyError: expectedError}) || p.calls != 0 {
				t.Fatalf("check=%+v err=%v calls=%d", checked, err, p.calls)
			}
			if name == "custom after check" {
				p.missing = false
				p.data = []byte("custom")
				expectedState = "custom"
				wantCalls = 0
			}
			if name == "current after check" {
				p.missing = false
				p.data = []byte("remote")
				expectedState = "current"
				wantCalls = 0
			}
			if name == "candidate mutation" {
				p.candidate.Bytes[0] = 'X'
				p.candidate.Previous[0] = "replaced history"
			}
			applied, err := s.ApplyUnifiedUpdate(context.Background())
			want := UnifiedUpdateInfo{PolicyState: expectedState, PolicyError: expectedError, PolicyInstalled: wantCalls == 1}
			if name == "partial recheck" {
				want.PolicyError = "reload failed"
			}
			if err != nil || p.calls != wantCalls || !reflect.DeepEqual(*applied, want) {
				t.Fatalf("apply=%+v err=%v calls=%d", applied, err, p.calls)
			}
			if name == "candidate mutation" && !reflect.DeepEqual(p.received, &udevauth.Policy{Version: "1.2.3", Bytes: []byte("remote"), Previous: []string{"signed history"}}) {
				t.Fatalf("detached candidate changed: %+v", p.received)
			}
			if name == "partial recheck" {
				again, err := s.CheckUnifiedUpdates(context.Background())
				want.PolicyState = "current"
				if err != nil || !reflect.DeepEqual(*again, want) || p.calls != 1 {
					t.Fatalf("lost partial result: %+v err=%v calls=%d", again, err, p.calls)
				}
			}
		})
	}
}

func TestUnifiedSerializationCancellationAndLegacyInvalidation(t *testing.T) {
	p := &policyFixture{missing: true}
	s := &Service{}
	ConfigureUpdater(s, nil, nil)
	ConfigureUnifiedUpdates(s, p, p)
	started, release := make(chan struct{}), make(chan struct{})
	s.update.check = func(ctx context.Context) (*update.VerifiedUpdate, error) {
		close(started)
		<-release
		return nil, update.ErrNoUpdate
	}
	checked := make(chan error, 1)
	go func() { _, err := s.CheckUnifiedUpdates(context.Background()); checked <- err }()
	<-started
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	applied := make(chan error, 1)
	go func() { _, err := s.ApplyUnifiedUpdate(ctx); applied <- err }()
	close(release)
	if err := <-checked; err != nil {
		t.Fatal(err)
	}
	if err := <-applied; !errors.Is(err, context.Canceled) || p.calls != 0 {
		t.Fatalf("canceled apply=%v calls=%d", err, p.calls)
	}
	s.update.check = func(context.Context) (*update.VerifiedUpdate, error) { return nil, update.ErrNoUpdate }
	if _, err := s.CheckForUpdate(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ApplyUnifiedUpdate(context.Background()); err == nil || err.Error() != "check for updates before applying" || p.calls != 0 {
		t.Fatalf("stale apply=%v calls=%d", err, p.calls)
	}
	if _, err := s.CheckUnifiedUpdates(context.Background()); err != nil {
		t.Fatal(err)
	}
	s.update.check = func(ctx context.Context) (*update.VerifiedUpdate, error) { return nil, ctx.Err() }
	if _, err := s.CheckUnifiedUpdates(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled check=%v", err)
	}
	if _, err := s.ApplyUnifiedUpdate(context.Background()); err == nil || err.Error() != "check for updates before applying" || p.calls != 0 {
		t.Fatalf("canceled check retained approval: %v calls=%d", err, p.calls)
	}
}

func assertUnified(t *testing.T, got *UnifiedUpdateInfo, err error, want UnifiedUpdateInfo, wantError string) {
	t.Helper()
	text := ""
	if err != nil {
		text = err.Error()
	}
	if got == nil || *got != want || text != wantError {
		t.Fatalf("result=%+v error=%q want=%+v error=%q", got, text, want, wantError)
	}
}

func TestUnifiedLegacyEmptyCheckInvalidatesApproval(t *testing.T) {
	p := &policyFixture{missing: true}
	s := &Service{}
	ConfigureUpdater(s, nil, nil)
	ConfigureUnifiedUpdates(s, p, p)
	want := UnifiedUpdateInfo{PolicyState: "missing"}
	got, err := s.CheckUnifiedUpdates(context.Background())
	assertUnified(t, got, err, want, "")
	if old, err := s.CheckForUpdate(context.Background()); old != nil || err != nil {
		t.Fatalf("empty legacy check=%+v %v", old, err)
	}
	got, err = s.ApplyUnifiedUpdate(context.Background())
	if got != nil || err == nil || err.Error() != "check for updates before applying" || p.calls != 0 || !p.missing || len(p.data) != 0 {
		t.Fatalf("empty legacy check retained approval: %+v %v calls=%d bytes=%q", got, err, p.calls, p.data)
	}
}

func TestUnifiedCancellationBetweenEffects(t *testing.T) {
	for _, stage := range []string{"check", "policy", "app"} {
		t.Run(stage, func(t *testing.T) {
			p := &policyFixture{missing: true}
			s := &Service{}
			apps, launches := 0, 0
			appBytes := "installed app"
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			ConfigureUpdater(s, nil, func() error { launches++; return nil })
			s.update.check = func(context.Context) (*update.VerifiedUpdate, error) {
				if stage == "check" {
					cancel()
				}
				return &update.VerifiedUpdate{Release: update.Release{Version: "1.2.3"}}, nil
			}
			s.update.apply = func(context.Context, *update.VerifiedUpdate, bool) error {
				apps++
				appBytes = "updated app"
				if stage == "app" {
					cancel()
				}
				return nil
			}
			if stage == "policy" {
				p.onApply = cancel
			}
			ConfigureUnifiedUpdates(s, p, p)
			checked, err := s.CheckUnifiedUpdates(ctx)
			if stage == "check" {
				if checked != nil || !errors.Is(err, context.Canceled) || p.checks != 0 || p.calls != 0 || apps != 0 || launches != 0 || !p.missing || len(p.data) != 0 || appBytes != "installed app" {
					t.Fatalf("canceled check=%+v err=%v counters=%d/%d/%d/%d bytes=%q", checked, err, p.checks, p.calls, apps, launches, p.data)
				}
				rejected, err := s.ApplyUnifiedUpdate(context.Background())
				if rejected != nil || err == nil || err.Error() != "check for updates before applying" {
					t.Fatalf("approval survived canceled check: %+v %v", rejected, err)
				}
				return
			}
			want := UnifiedUpdateInfo{Version: "1.2.3", AppAvailable: true, PolicyState: "missing"}
			assertUnified(t, checked, err, want, "")
			got, err := s.ApplyUnifiedUpdate(ctx)
			want.PolicyInstalled = true
			want.AppInstalled = stage == "app"
			assertUnified(t, got, err, want, "context canceled")
			wantApps := 0
			if stage == "app" {
				wantApps = 1
			}
			expectedApp := "installed app"
			if stage == "app" {
				expectedApp = "updated app"
			}
			if apps != wantApps || launches != 0 || p.calls != 1 || p.missing || string(p.data) != "remote" || appBytes != expectedApp {
				t.Fatalf("side effect after cancellation: %d/%d/%d bytes=%q", p.calls, apps, launches, p.data)
			}
			p.onApply = nil
			got, err = s.ApplyUnifiedUpdate(context.Background())
			want.AppInstalled = true
			assertUnified(t, got, err, want, "")
			if p.calls != 1 || apps != 1 || launches != 1 || string(p.data) != "remote" || appBytes != "updated app" {
				t.Fatalf("retry counters=%d/%d/%d bytes=%q", p.calls, apps, launches, p.data)
			}
		})
	}
}

func TestUnifiedPartialResultsAcrossRecheckAndRetry(t *testing.T) {
	for _, failure := range []string{"policy", "app", "reload", "relaunch"} {
		t.Run(failure, func(t *testing.T) {
			p := &policyFixture{missing: true}
			s := &Service{}
			apps, launches := 0, 0
			failing := true
			appBytes := "installed app"
			if failure == "policy" || failure == "reload" {
				p.failure = errors.New(failure + " failed")
				p.installed = failure == "reload"
			}
			ConfigureUpdater(s, nil, func() error {
				launches++
				if failing && failure == "relaunch" {
					return errors.New("relaunch failed")
				}
				return nil
			})
			s.update.check = func(context.Context) (*update.VerifiedUpdate, error) {
				return &update.VerifiedUpdate{Release: update.Release{Version: "1.2.3"}}, nil
			}
			s.update.apply = func(context.Context, *update.VerifiedUpdate, bool) error {
				apps++
				if failing && failure == "app" {
					return errors.New("app failed")
				}
				appBytes = "updated app"
				return nil
			}
			ConfigureUnifiedUpdates(s, p, p)
			want := UnifiedUpdateInfo{Version: "1.2.3", AppAvailable: true, PolicyState: "missing"}
			got, err := s.CheckUnifiedUpdates(context.Background())
			assertUnified(t, got, err, want, "")
			got, err = s.ApplyUnifiedUpdate(context.Background())
			want.AppInstalled = failure != "app"
			want.PolicyInstalled = failure != "policy"
			switch failure {
			case "policy", "reload":
				want.PolicyError = failure + " failed"
			case "app":
				want.AppError = "app failed"
			case "relaunch":
				want.RelaunchError = "relaunch failed"
			}
			assertUnified(t, got, err, want, "")
			wantLaunches := 0
			if failure == "relaunch" {
				wantLaunches = 1
			}
			expectedData := "remote"
			if failure == "policy" {
				expectedData = ""
			}
			expectedApp := "updated app"
			if failure == "app" {
				expectedApp = "installed app"
			}
			if apps != 1 || p.calls != 1 || launches != wantLaunches || string(p.data) != expectedData || p.missing != (failure == "policy") || appBytes != expectedApp {
				t.Fatalf("first counters=%d/%d/%d bytes=%q", p.calls, apps, launches, p.data)
			}
			canceled, cancel := context.WithCancel(context.Background())
			p.onCheck = func(context.Context) { cancel() }
			p.checkError = context.Canceled
			interrupted, interruptedErr := s.CheckUnifiedUpdates(canceled)
			if interrupted != nil || !errors.Is(interruptedErr, context.Canceled) || p.calls != 1 || apps != 1 || launches != wantLaunches || string(p.data) != expectedData || appBytes != expectedApp {
				t.Fatalf("canceled recheck=%+v %v", interrupted, interruptedErr)
			}
			p.onCheck = nil
			p.checkError = nil
			got, err = s.CheckUnifiedUpdates(context.Background())
			if want.PolicyInstalled {
				want.PolicyState = "current"
			}
			assertUnified(t, got, err, want, "")
			if apps != 1 || p.calls != 1 || launches != wantLaunches || string(p.data) != expectedData || appBytes != expectedApp {
				t.Fatal("check caused a write")
			}
			failing = false
			p.failure = nil
			got, err = s.ApplyUnifiedUpdate(context.Background())
			want.AppInstalled = true
			want.PolicyInstalled = true
			want.AppError = ""
			want.RelaunchError = ""
			if failure != "reload" {
				want.PolicyError = ""
			}
			assertUnified(t, got, err, want, "")
			wantApps, wantPolicies := 1, 1
			if failure == "app" {
				wantApps = 2
			}
			if failure == "policy" {
				wantPolicies = 2
			}
			if p.calls != wantPolicies || apps != wantApps || launches != wantLaunches+1 || p.missing || string(p.data) != "remote" || appBytes != "updated app" {
				t.Fatalf("retry counters=%d/%d/%d bytes=%q", p.calls, apps, launches, p.data)
			}
		})
	}
}

func TestUnifiedCompetingPublicOperations(t *testing.T) {
	for _, operation := range []string{"unified apply", "legacy apply", "legacy check", "unified check", "canceled apply", "canceled check"} {
		t.Run(operation, func(t *testing.T) {
			p := &policyFixture{missing: true}
			s := &Service{}
			apps, launches := 0, 0
			entered, release := make(chan struct{}), make(chan struct{})
			p.onApply = func() { close(entered); <-release }
			ConfigureUpdater(s, nil, func() error { launches++; return nil })
			s.update.check = func(context.Context) (*update.VerifiedUpdate, error) {
				return &update.VerifiedUpdate{Release: update.Release{Version: "1.2.3"}}, nil
			}
			s.update.apply = func(context.Context, *update.VerifiedUpdate, bool) error { apps++; return nil }
			ConfigureUnifiedUpdates(s, p, p)
			want := UnifiedUpdateInfo{Version: "1.2.3", AppAvailable: true, PolicyState: "missing"}
			got, err := s.CheckUnifiedUpdates(context.Background())
			assertUnified(t, got, err, want, "")
			type result struct {
				info *UnifiedUpdateInfo
				err  error
			}
			first := make(chan result, 1)
			second := make(chan result, 1)
			ready := make(chan struct{})
			go func() { info, err := s.ApplyUnifiedUpdate(context.Background()); first <- result{info, err} }()
			<-entered
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			go func() {
				close(ready)
				switch operation {
				case "legacy apply":
					second <- result{nil, s.ApplyVerifiedUpdate(context.Background())}
				case "legacy check":
					info, err := s.CheckForUpdate(context.Background())
					if err == nil && (info == nil || info.Version != "1.2.3") {
						err = errors.New("legacy result changed")
					}
					second <- result{nil, err}
				case "unified check", "canceled check":
					info, err := s.CheckUnifiedUpdates(ctx)
					second <- result{info, err}
				default:
					info, err := s.ApplyUnifiedUpdate(ctx)
					second <- result{info, err}
				}
			}()
			<-ready
			if operation == "canceled apply" || operation == "canceled check" {
				cancel()
			}
			close(release)
			a, b := <-first, <-second
			want.AppInstalled = true
			want.PolicyInstalled = true
			assertUnified(t, a.info, a.err, want, "")
			expectedLaunches := 1
			switch operation {
			case "unified apply":
				expectedLaunches = 2
				assertUnified(t, b.info, b.err, want, "")
			case "legacy apply":
				expectedLaunches = 2
				if b.info != nil || b.err != nil {
					t.Fatalf("legacy apply=%+v %v", b.info, b.err)
				}
			case "legacy check":
				if b.info != nil || b.err != nil {
					t.Fatalf("legacy check=%+v %v", b.info, b.err)
				}
			case "unified check":
				want.PolicyState = "current"
				assertUnified(t, b.info, b.err, want, "")
			case "canceled apply", "canceled check":
				if b.info != nil || !errors.Is(b.err, context.Canceled) {
					t.Fatalf("canceled waiting apply=%+v %v", b.info, b.err)
				}
			}
			if p.calls != 1 || apps != 1 || launches != expectedLaunches || p.missing || string(p.data) != "remote" {
				t.Fatalf("competing counters=%d/%d/%d bytes=%q", p.calls, apps, launches, p.data)
			}
			if operation == "legacy apply" || operation == "legacy check" {
				got, err = s.ApplyUnifiedUpdate(context.Background())
				if got != nil || err == nil || err.Error() != "check for updates before applying" || p.calls != 1 || apps != 1 || launches != expectedLaunches || string(p.data) != "remote" {
					t.Fatalf("legacy approval invalidation=%+v %v", got, err)
				}
			}
		})
	}
}

func TestUnifiedRelaunchCancellationReturnsCompletedResult(t *testing.T) {
	p := &policyFixture{missing: true}
	s := &Service{}
	apps, launches := 0, 0
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ConfigureUpdater(s, nil, func() error { launches++; cancel(); return nil })
	s.update.check = func(context.Context) (*update.VerifiedUpdate, error) {
		return &update.VerifiedUpdate{Release: update.Release{Version: "9.0.0"}}, nil
	}
	s.update.apply = func(context.Context, *update.VerifiedUpdate, bool) error { apps++; return nil }
	ConfigureUnifiedUpdates(s, p, p)
	want := UnifiedUpdateInfo{Version: "9.0.0", AppAvailable: true, PolicyState: "missing"}
	got, err := s.CheckUnifiedUpdates(ctx)
	assertUnified(t, got, err, want, "")
	if p.checks != 1 || p.calls != 0 || apps != 0 || launches != 0 || !p.missing || len(p.data) != 0 {
		t.Fatalf("check counters=%d/%d/%d/%d bytes=%q missing=%t", p.checks, p.calls, apps, launches, p.data, p.missing)
	}
	got, err = s.ApplyUnifiedUpdate(ctx)
	want.AppInstalled, want.PolicyInstalled = true, true
	if got == nil || *got != want || !errors.Is(err, context.Canceled) {
		t.Errorf("apply result=%+v error=%v want=%+v and context.Canceled", got, err, want)
	}
	if p.checks != 1 || p.calls != 1 || apps != 1 || launches != 1 || p.missing || string(p.data) != "remote" {
		t.Errorf("apply counters=%d/%d/%d/%d bytes=%q missing=%t", p.checks, p.calls, apps, launches, p.data, p.missing)
	}
}

func TestPreservedUpdatePublicBehavior(t *testing.T) {
	s := &Service{}
	applies, launches := 0, 0
	ConfigureUpdater(s, nil, func() error { launches++; return errors.New("launch failed") })
	s.update.check = func(context.Context) (*update.VerifiedUpdate, error) {
		return &update.VerifiedUpdate{Release: update.Release{Version: "1.2.3"}}, nil
	}
	s.update.apply = func(_ context.Context, v *update.VerifiedUpdate, approved bool) error {
		if v == nil || !approved {
			t.Fatal("lost verified approval")
		}
		applies++
		return nil
	}
	info, err := s.CheckForUpdate(context.Background())
	if err != nil || info.Version != "1.2.3" {
		t.Fatalf("check=%+v err=%v", info, err)
	}
	for i := 0; i < 2; i++ {
		if err := s.ApplyVerifiedUpdate(context.Background()); err == nil || err.Error() != "launch failed" {
			t.Fatalf("apply=%v", err)
		}
	}
	if applies != 1 || launches != 2 {
		t.Fatalf("counters=%d/%d", applies, launches)
	}
	if s.GetApplicationVersion() != update.CurrentVersion {
		t.Fatal("compiled version changed")
	}
	if err := RecoverUpdates(s); err != nil {
		t.Fatalf("recovery=%v", err)
	}
	t.Logf("version=%s check=%s apply=launch failed twice counters=%d/%d recovery=nil", s.GetApplicationVersion(), info.Version, applies, launches)
}
