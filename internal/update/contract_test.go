package update

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"strings"
	"testing"
)

func signedManifest(t *testing.T, version, downloadURL, digest string, privateKey ed25519.PrivateKey) Manifest {
	t.Helper()
	manifest := Manifest{
		Version: version,
		URL:     downloadURL,
		SHA256:  digest,
	}
	payload, err := manifest.SignedBytes()
	if err != nil {
		t.Fatalf("SignedBytes() error = %v", err)
	}
	manifest.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(privateKey, payload))
	return manifest
}

func TestVerifyManifestAcceptsNewerSignedAppImage(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	manifest := signedManifest(t, "v1.2.0", "https://github.com/example/app/releases/download/v1.2.0/app.AppImage", strings.Repeat("a", 64), privateKey)

	if err := VerifyManifest(manifest, "1.1.9", base64.StdEncoding.EncodeToString(publicKey)); err != nil {
		t.Fatalf("VerifyManifest() error = %v", err)
	}

	release, err := ReplacementEligible(manifest, "1.1.9", base64.StdEncoding.EncodeToString(publicKey), true)
	if err != nil {
		t.Fatalf("ReplacementEligible() error = %v", err)
	}
	if release.Version != "v1.2.0" {
		t.Errorf("release.Version = %q, want v1.2.0", release.Version)
	}
	if release.URL != manifest.URL || release.SHA256 != manifest.SHA256 {
		t.Errorf("release = %#v, want manifest download fields", release)
	}
}

func TestVerifyManifestRejectsUnsafeOrInvalidRelease(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	encodedPublicKey := base64.StdEncoding.EncodeToString(publicKey)
	valid := signedManifest(t, "1.2.0", "https://example.com/app.AppImage", strings.Repeat("a", 64), privateKey)

	tests := []struct {
		name       string
		manifest   Manifest
		current    string
		publicKey  string
		wantSubstr string
	}{
		{name: "malformed signature", manifest: Manifest{Version: valid.Version, URL: valid.URL, SHA256: valid.SHA256, Signature: "not-base64"}, current: "1.0.0", publicKey: encodedPublicKey, wantSubstr: "signature"},
		{name: "wrong signature", manifest: func() Manifest { m := valid; m.Version = "1.2.1"; return m }(), current: "1.0.0", publicKey: encodedPublicKey, wantSubstr: "signature"},
		{name: "missing public key", manifest: valid, current: "1.0.0", publicKey: "", wantSubstr: "public key"},
		{name: "invalid public key", manifest: valid, current: "1.0.0", publicKey: "not-base64", wantSubstr: "public key"},
		{name: "not an upgrade", manifest: valid, current: "1.2.0", publicKey: encodedPublicKey, wantSubstr: "newer"},
		{name: "invalid current version", manifest: valid, current: "nope", publicKey: encodedPublicKey, wantSubstr: "current version"},
		{name: "invalid release version", manifest: func() Manifest { m := valid; m.Version = "nope"; return m }(), current: "1.0.0", publicKey: encodedPublicKey, wantSubstr: "version"},
		{name: "insecure URL", manifest: func() Manifest { m := valid; m.URL = "http://example.com/app.AppImage"; return m }(), current: "1.0.0", publicKey: encodedPublicKey, wantSubstr: "HTTPS"},
		{name: "non AppImage URL", manifest: func() Manifest { m := valid; m.URL = "https://example.com/app.tar.gz"; return m }(), current: "1.0.0", publicKey: encodedPublicKey, wantSubstr: "AppImage"},
		{name: "uppercase digest", manifest: func() Manifest { m := valid; m.SHA256 = strings.Repeat("A", 64); return m }(), current: "1.0.0", publicKey: encodedPublicKey, wantSubstr: "SHA-256"},
		{name: "short digest", manifest: func() Manifest { m := valid; m.SHA256 = "abc"; return m }(), current: "1.0.0", publicKey: encodedPublicKey, wantSubstr: "SHA-256"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := VerifyManifest(test.manifest, test.current, test.publicKey)
			if err == nil || !strings.Contains(err.Error(), test.wantSubstr) {
				t.Fatalf("VerifyManifest() error = %v, want %q", err, test.wantSubstr)
			}
		})
	}
}

func TestReplacementEligibleRequiresUserApproval(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	manifest := signedManifest(t, "1.2.0", "https://example.com/app.AppImage", strings.Repeat("a", 64), privateKey)

	if _, err := ReplacementEligible(manifest, "1.0.0", base64.StdEncoding.EncodeToString(publicKey), false); err == nil || !strings.Contains(err.Error(), "approval") {
		t.Fatalf("ReplacementEligible() error = %v, want approval error", err)
	}
	if _, err := ReplacementEligible(manifest, "1.0.0", base64.StdEncoding.EncodeToString(publicKey), true); err != nil {
		t.Fatalf("ReplacementEligible() error = %v", err)
	}
}

func TestVerifyManifestBindsExactVersionText(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	manifest := signedManifest(t, "v1.2.0", "https://example.com/app.AppImage", strings.Repeat("a", 64), privateKey)
	manifest.Version = "1.2.0"

	err = VerifyManifest(manifest, "1.0.0", base64.StdEncoding.EncodeToString(publicKey))
	if err == nil || !strings.Contains(err.Error(), "signature") {
		t.Fatalf("VerifyManifest() error = %v, want signature error after exact version text changes", err)
	}
}

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		left, right string
		want        int
	}{
		{"1.2.0", "1.2.0", 0},
		{"v1.2.1", "1.2.0", 1},
		{"1.10.0", "1.2.0", 1},
		{"1.2.0-beta.2", "1.2.0-beta.10", -1},
		{"1.2.0", "1.2.0-rc.1", 1},
		{"1.2.0+build.7", "1.2.0+build.8", 0},
		{"1.2.0-beta.999999999999999999999", "1.2.0-beta.1000000000000000000000", -1},
	}
	for _, test := range tests {
		t.Run(test.left+"_"+test.right, func(t *testing.T) {
			got, err := CompareVersions(test.left, test.right)
			if err != nil || got != test.want {
				t.Fatalf("CompareVersions(%q, %q) = %d, %v; want %d, nil", test.left, test.right, got, err, test.want)
			}
		})
	}
}
