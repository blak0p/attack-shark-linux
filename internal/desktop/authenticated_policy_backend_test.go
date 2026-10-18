package desktop

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/blak0p/attack-shark-linux/internal/udevauth"
	"github.com/blak0p/attack-shark-linux/internal/update"
)

type authenticatedHTTP func(*http.Request) (*http.Response, error)

func (f authenticatedHTTP) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type absentPolicy struct{}

func (absentPolicy) Lstat() (fs.FileMode, error) { return 0, fs.ErrNotExist }
func (absentPolicy) Read(int) ([]byte, error)    { panic("absent policy read") }

type authenticatedFixture struct {
	service                                             *Service
	backend                                             *authenticatedPolicyBackend
	wire                                                []byte
	lists, manifests, policies, helpers, apps, launches int
	status                                              int
	invalid, failList                                   bool
	applyError                                          error
	installed                                           bool
}

func newAuthenticatedFixture(t *testing.T, current string) *authenticatedFixture {
	t.Helper()
	public, private, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	manifest := update.Manifest{Version: "1.2.0", URL: "https://github.com/blak0p/attack-shark-linux/releases/download/v1.2.0/app.AppImage", SHA256: strings.Repeat("a", 64)}
	payload, err := manifest.SignedBytes()
	if err != nil {
		t.Fatal(err)
	}
	manifest.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(private, payload))
	envelope, err := udevauth.NewPolicy("1.2.0")
	if err != nil {
		t.Fatal(err)
	}
	payload, err = envelope.SignedBytes()
	if err != nil {
		t.Fatal(err)
	}
	wire, err := envelope.Marshal(ed25519.Sign(private, payload))
	if err != nil {
		t.Fatal(err)
	}
	f := &authenticatedFixture{wire: wire, status: 200}
	transport := update.NewGitHubTransport(&http.Client{Transport: authenticatedHTTP(func(r *http.Request) (*http.Response, error) {
		var body string
		status := 200
		switch {
		case r.URL.Host == "api.github.com":
			f.lists++
			if f.failList {
				return nil, errors.New("offline")
			}
			body = `[{"assets":[{"name":"update-manifest.json","browser_download_url":"https://github.com/blak0p/attack-shark-linux/releases/download/v1.2.0/update-manifest.json"}]}]`
		case strings.HasSuffix(r.URL.Path, "update-manifest.json"):
			f.manifests++
			copy := manifest
			if f.invalid {
				copy.Signature = base64.StdEncoding.EncodeToString(make([]byte, 64))
			}
			data, _ := json.Marshal(copy)
			body = string(data)
		case strings.HasSuffix(r.URL.Path, "/v1.2.0/udev-policy.txt"):
			f.policies++
			body, status = string(f.wire), f.status
		default:
			t.Fatalf("unexpected URL %s", r.URL)
		}
		return &http.Response{StatusCode: status, Status: http.StatusText(status), Body: io.NopCloser(strings.NewReader(body))}, nil
	})})
	f.service = &Service{}
	ConfigureUpdater(f.service, &update.Updater{CurrentVersion: current, PublicKey: base64.StdEncoding.EncodeToString(public), Transport: transport}, func() error { f.launches++; return nil })
	state := f.service.updateState()
	configureAuthenticatedPolicyUpdates(f.service, state, transport, absentPolicy{}, func(ctx context.Context, p *udevauth.Policy, wire []byte) (PolicyApplyResult, error) {
		f.helpers++
		if !reflect.DeepEqual(wire, f.wire) || p.Version != "1.2.0" {
			t.Fatal("changed signed helper input")
		}
		return PolicyApplyResult{Installed: f.installed}, f.applyError
	})
	f.backend = state.unified.backend.(*authenticatedPolicyBackend)
	state.apply = func(context.Context, *update.VerifiedUpdate, bool) error { f.apps++; return nil }
	if f.lists+f.helpers+f.policies != 0 {
		t.Fatal("construction performed I/O")
	}
	return f
}

func TestAuthenticatedComposition(t *testing.T) {
	for _, current := range []string{"1.1.0", "1.2.0"} {
		t.Run(current, func(t *testing.T) {
			f := newAuthenticatedFixture(t, current)
			info, err := f.service.CheckUnifiedUpdates(context.Background())
			app := current == "1.1.0"
			want := &UnifiedUpdateInfo{PolicyState: "missing", AppAvailable: app}
			if app {
				want.Version = "1.2.0"
			}
			if err != nil || !reflect.DeepEqual(info, want) {
				t.Fatalf("%+v %v want %+v", info, err, want)
			}
			lists := 1
			if !app {
				lists = 2
			}
			if f.lists != lists || f.manifests != lists || f.policies != 1 || f.helpers != 0 {
				t.Fatalf("counts %+v", f)
			}
			f.installed = true
			info, err = f.service.ApplyUnifiedUpdate(context.Background())
			if err != nil || !info.PolicyInstalled || info.AppInstalled != app || f.helpers != 1 {
				t.Fatalf("apply %+v %v", info, err)
			}
			_, err = f.service.ApplyUnifiedUpdate(context.Background())
			if err != nil || f.helpers != 1 {
				t.Fatalf("retry repeated helper: %v %d", err, f.helpers)
			}
		})
	}
}

func TestAuthenticatedRejection(t *testing.T) {
	for _, mode := range []string{"legacy", "signature", "offline", "policy signature"} {
		t.Run(mode, func(t *testing.T) {
			f := newAuthenticatedFixture(t, "1.1.0")
			if _, err := f.service.CheckUnifiedUpdates(context.Background()); err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "legacy":
				f.status = 404
			case "signature":
				f.invalid = true
			case "offline":
				f.failList = true
			case "policy signature":
				f.wire[0] ^= 1
			}
			info, err := f.service.CheckUnifiedUpdates(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			want := "error"
			if mode == "legacy" {
				want = "unavailable"
			}
			if info.PolicyState != want || (info.PolicyError == "") != (mode == "legacy") {
				t.Fatalf("%+v", info)
			}
			if (mode == "signature" || mode == "offline") && (info.AppAvailable || info.AppError == "" || f.policies != 1) {
				t.Fatalf("independent selection %+v %d", info, f.policies)
			}
			_, _ = f.service.ApplyUnifiedUpdate(context.Background())
			if f.helpers != 0 || f.backend.policy != nil || f.backend.wire != nil {
				t.Fatal("rejected recheck retained authority")
			}
		})
	}
}

func TestAuthenticatedRetainedAuthority(t *testing.T) {
	for _, mode := range []string{"history", "bytes", "version", "wire"} {
		t.Run(mode, func(t *testing.T) {
			f := newAuthenticatedFixture(t, "1.2.0")
			_, err := f.service.CheckUnifiedUpdates(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			u := f.service.updateState().unified
			if u.candidate == nil {
				t.Fatal("missing authenticated candidate")
			}
			switch mode {
			case "history":
				u.candidate.Previous = append(u.candidate.Previous, strings.Repeat("b", 64))
			case "bytes":
				u.candidate.Bytes[0] ^= 1
			case "version":
				u.candidate.Version = "1.3.0"
			case "wire":
				f.backend.wire[0] ^= 1
			}
			info, err := f.service.ApplyUnifiedUpdate(context.Background())
			if err != nil || info.PolicyError == "" || info.PolicyInstalled || f.helpers != 0 {
				t.Fatalf("%+v %v helper=%d", info, err, f.helpers)
			}
		})
	}
}

func TestAuthenticatedLegacyPreservation(t *testing.T) {
	for _, current := range []string{"1.1.0", "1.2.0"} {
		t.Run(current, func(t *testing.T) {
			f := newAuthenticatedFixture(t, current)
			info, err := f.service.CheckForUpdate(context.Background())
			if err != nil || (info != nil) != (current == "1.1.0") || f.lists != 1 || f.policies != 0 || f.helpers != 0 {
				t.Fatalf("legacy check %+v %v counts %+v", info, err, f)
			}
			if err := f.service.ApplyVerifiedUpdate(context.Background()); err != nil {
				t.Fatal(err)
			}
			if err := f.service.ApplyVerifiedUpdate(context.Background()); err != nil {
				t.Fatal(err)
			}
			if f.apps != 1 || f.launches != 2 || f.helpers != 0 {
				t.Fatalf("legacy rerun counts %+v", f)
			}
			ConfigureUpdater(f.service, nil, nil)
			if _, err := f.service.CheckUnifiedUpdates(context.Background()); err == nil {
				t.Fatal("replacement retained unified state")
			}
		})
	}
}

func TestAuthenticatedPartialAndDenial(t *testing.T) {
	for _, installed := range []bool{false, true} {
		t.Run(fmt.Sprint(installed), func(t *testing.T) {
			f := newAuthenticatedFixture(t, "1.1.0")
			_, _ = f.service.CheckUnifiedUpdates(context.Background())
			f.installed, f.applyError = installed, errors.New("authorization or reload failed")
			info, err := f.service.ApplyUnifiedUpdate(context.Background())
			if err != nil || info.PolicyInstalled != installed || info.PolicyError != f.applyError.Error() || !info.AppInstalled || f.apps != 1 || f.helpers != 1 || f.launches != 0 {
				t.Fatalf("%+v %v counts %+v", info, err, f)
			}
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			if _, err := f.service.ApplyUnifiedUpdate(ctx); !errors.Is(err, context.Canceled) || f.helpers != 1 {
				t.Fatal("cancelled apply had effects")
			}
			_, _ = f.service.ApplyUnifiedUpdate(context.Background())
			want := 2
			if installed {
				want = 1
			}
			if f.helpers != want || f.apps != 1 {
				t.Fatalf("retry helper=%d app=%d", f.helpers, f.apps)
			}
		})
	}
}
