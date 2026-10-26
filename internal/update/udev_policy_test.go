package update

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestPolicyReleaseEligibilityAndLegacyVerification(t *testing.T) {
	public, private, _ := ed25519.GenerateKey(nil)
	key := base64.StdEncoding.EncodeToString(public)
	for _, tc := range []struct {
		name, version, url string
		reject             bool
	}{
		{"canonical", "1.2.0", "https://github.com/blak0p/attack-shark-linux/releases/download/v1.2.0/app.AppImage", false},
		{"foreign repository", "1.2.0", "https://github.com/other/repo/releases/download/v1.2.0/app.AppImage", true},
		{"wrong tag", "1.2.0", "https://github.com/blak0p/attack-shark-linux/releases/download/v1.3.0/app.AppImage", true},
		{"query", "1.2.0", "https://github.com/blak0p/attack-shark-linux/releases/download/v1.2.0/app.AppImage?q=1", true},
		{"older", "1.0.0", "https://github.com/blak0p/attack-shark-linux/releases/download/v1.0.0/app.AppImage", true},
		{"rc", "1.2.0-rc.1", "https://github.com/blak0p/attack-shark-linux/releases/download/v1.2.0-rc.1/app.AppImage", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := Manifest{Version: tc.version, URL: tc.url, SHA256: strings.Repeat("a", 64)}
			payload, err := m.SignedBytes()
			if err != nil {
				t.Fatal(err)
			}
			m.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(private, payload))
			version, err := PolicyVersion(m, "1.1.0", key)
			if (err != nil) != tc.reject {
				t.Fatalf("version=%q err=%v", version, err)
			}
			if tc.reject && version != "" {
				t.Fatal("rejected version returned")
			}
			if tc.name == "canonical" {
				if err := VerifyManifest(m, "1.1.0", key); err != nil {
					t.Fatal(err)
				}
				if err := VerifyManifest(m, "1.2.0", key); err == nil {
					t.Fatal("old upgrade-only contract changed")
				}
			}
		})
	}
}

func TestPolicyEnvelopeBoundsAndFailures(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
	}{
		{"oversized", 200, "12345"}, {"trailing data", 200, "1234x"}, {"forbidden", 403, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			closed := false
			tr := NewGitHubTransport(&http.Client{Transport: roundTripper(func(r *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: tc.status, Body: &policyBody{Reader: strings.NewReader(tc.body), closed: &closed}}, nil
			})})
			wire, err := tr.PolicyEnvelope(context.Background(), "1.2.0", 4)
			if err == nil || wire != nil || !closed {
				t.Fatalf("wire=%q err=%v closed=%v", wire, err, closed)
			}
		})
	}
}

type policyBody struct {
	io.Reader
	closed *bool
}

func (b *policyBody) Close() error { *b.closed = true; return nil }
