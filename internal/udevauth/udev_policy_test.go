package udevauth

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/blak0p/attack-shark-linux/internal/update"
)

type policyHTTP func(*http.Request) (*http.Response, error)

func (f policyHTTP) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestPolicyCheck(t *testing.T) {
	key, private, _ := ed25519.GenerateKey(nil)
	encoded := base64.StdEncoding.EncodeToString(key)
	envelope, _ := NewPolicy("1.2.0")
	// Simulate a future official release unknown to this binary's embed/history.
	envelope.payload = append(envelope.payload, []byte("# future official policy\n")...)
	envelope.digest = sum(envelope.payload)
	envelope.previous = strings.Repeat("a", 64)
	signed, _ := envelope.SignedBytes()
	wire, _ := envelope.Marshal(ed25519.Sign(private, signed))
	manifest := update.Manifest{Version: "1.2.0", URL: "https://github.com/blak0p/attack-shark-linux/releases/download/v1.2.0/app.AppImage", SHA256: fmt.Sprintf("%x", sha256.Sum256([]byte("app")))}
	payload, _ := manifest.SignedBytes()
	manifest.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(private, payload))
	wrongVersion, _ := NewPolicy("1.2.1")
	wrongSigned, _ := wrongVersion.SignedBytes()
	wrongWire, _ := wrongVersion.Marshal(ed25519.Sign(private, wrongSigned))
	_, otherPrivate, _ := ed25519.GenerateKey(nil)
	otherWire, _ := envelope.Marshal(ed25519.Sign(otherPrivate, signed))
	for _, tc := range []struct {
		name, current, body, public string
		status                      int
		unavailable, fail           bool
	}{
		{"wrong policy key", "1.2.0", string(otherWire), encoded, 200, false, true},
		{"tampered payload", "1.2.0", strings.Replace(string(wire), "payload=", "payload=a", 1), encoded, 200, false, true},
		{"same version", "1.2.0", string(wire), encoded, 200, false, false},
		{"combined", "1.1.0", string(wire), encoded, 200, false, false},
		{"legacy", "1.2.0", "", encoded, 404, true, false},
		{"server failure", "1.2.0", "", encoded, 500, false, true},
		{"oversize", "1.2.0", strings.Repeat("x", MaxEnvelopeBytes+1), encoded, 200, false, true},
		{"tampered history", "1.2.0", strings.Replace(string(wire), "previous=", "previous=a", 1), encoded, 200, false, true},
		{"tampered digest", "1.2.0", strings.Replace(string(wire), "sha256=", "sha256=a", 1), encoded, 200, false, true},
		{"signed wrong version", "1.2.0", string(wrongWire), encoded, 200, false, true},
		{"other channel", "1.2.0-rc.1", string(wire), encoded, 200, false, true},
		{"wrong expected version", "1.3.0", string(wire), encoded, 200, false, true},
		{"wrong key", "1.2.0", string(wire), base64.StdEncoding.EncodeToString(make([]byte, 32)), 200, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			transport := update.NewGitHubTransport(&http.Client{Transport: policyHTTP(func(r *http.Request) (*http.Response, error) {
				calls++
				if r.URL.String() != "https://github.com/blak0p/attack-shark-linux/releases/download/v1.2.0/udev-policy.txt" {
					t.Fatalf("URL %s", r.URL)
				}
				if _, ok := r.Context().Deadline(); !ok {
					t.Fatal("missing deadline")
				}
				return &http.Response{StatusCode: tc.status, Status: http.StatusText(tc.status), Body: io.NopCloser(strings.NewReader(tc.body))}, nil
			})})
			result, authenticatedWire, err := CheckPolicyWithWire(context.Background(), transport, manifest, tc.current, tc.public)
			if tc.fail || tc.unavailable {
				if authenticatedWire != nil {
					t.Fatal("returned rejected wire")
				}
			} else {
				if string(authenticatedWire) != tc.body {
					t.Fatal("wire changed")
				}
				before := string(result.Bytes)
				authenticatedWire[0] ^= 1
				if string(result.Bytes) != before {
					t.Fatal("wire aliases policy")
				}
			}
			legacy, legacyErr := CheckPolicy(context.Background(), transport, manifest, tc.current, tc.public)
			if fmt.Sprint(legacyErr) != fmt.Sprint(err) || (legacy == nil) != (result == nil) {
				t.Fatalf("legacy mismatch: %v %v / %v %v", legacy, legacyErr, result, err)
			}
			if legacy != nil && (legacy.Version != result.Version || string(legacy.Bytes) != string(result.Bytes) || strings.Join(legacy.Previous, ",") != strings.Join(result.Previous, ",")) {
				t.Fatal("legacy policy changed")
			}
			if (err != nil) != tc.fail {
				t.Fatalf("result=%+v err=%v", result, err)
			}
			if tc.fail {
				if result != nil {
					t.Fatal("rejected data returned")
				}
				return
			}
			if tc.unavailable {
				if result != nil {
					t.Fatal("legacy claims support")
				}
				return
			}
			if calls != 2 || result.Version != "1.2.0" || string(result.Bytes) != string(envelope.payload) || strings.Join(result.Previous, ",") != envelope.previous {
				t.Fatalf("result=%+v calls=%d", result, calls)
			}
		})
	}
}
