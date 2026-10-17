package main

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	"github.com/blak0p/attack-shark-linux/internal/udevauth"
)

func TestStageAuthenticatedArtifacts(t *testing.T) {
	template, err := os.ReadFile("../../packaging/udev-helper/helper.sh")
	if err != nil {
		t.Fatal(err)
	}
	seed, _ := base64.StdEncoding.DecodeString(testSeed)
	key := ed25519.NewKeyFromSeed(seed).Public().(ed25519.PublicKey)
	artifacts, err := BuildAuthArtifacts("v1.2.3", template, testSeed)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(artifacts[HelperAsset], []byte("@RELEASE_PUBLIC_KEY_PEM@")) {
		t.Fatal("unrendered helper")
	}
	if err := udevauth.VerifyScript(artifacts[HelperManifestAsset], artifacts[HelperAsset], "1.2.3", key); err != nil {
		t.Fatal(err)
	}
	if _, err := udevauth.VerifyPolicy(artifacts[PolicyAsset], "1.2.3", key); err != nil {
		t.Fatal(err)
	}
	if err := udevauth.VerifyScript(artifacts[PolicyAsset], artifacts[HelperAsset], "1.2.3", key); err == nil {
		t.Fatal("domain confusion")
	}
	input := filepath.Join(t.TempDir(), canonicalAppImage)
	if err := os.WriteFile(input, []byte("fake AppImage"), 0755); err != nil {
		t.Fatal(err)
	}
	legacy, enhanced := t.TempDir(), t.TempDir()
	url := "https://github.com/blak0p/attack-shark-linux/releases/download/v1.2.3/" + canonicalAppImage
	if err := stage("v1.2.3", input, legacy, url, testSeed); err != nil {
		t.Fatal(err)
	}
	if err := stageWithHelper("v1.2.3", input, enhanced, url, testSeed, "../../packaging/udev-helper/helper.sh"); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{canonicalAppImage, canonicalAppImage + ".sha256", "update-manifest.json"} {
		a, _ := os.ReadFile(filepath.Join(legacy, name))
		b, _ := os.ReadFile(filepath.Join(enhanced, name))
		if !bytes.Equal(a, b) {
			t.Fatal("legacy bytes changed", name)
		}
	}
	for name, want := range artifacts {
		got, err := os.ReadFile(filepath.Join(enhanced, name))
		if err != nil || !bytes.Equal(got, want) {
			t.Fatal("staged artifact mismatch", name, err)
		}
	}
	second, err := BuildAuthArtifacts("v1.2.3", template, testSeed)
	if err != nil {
		t.Fatal(err)
	}
	for name, data := range artifacts {
		if !bytes.Equal(data, second[name]) {
			t.Fatal("nondeterministic", name)
		}
	}
	for _, tc := range []struct {
		tag, seed string
		template  []byte
	}{
		{"bad", testSeed, template}, {"v1.2.3", "", template}, {"v1.2.3", "invalid", template},
		{"v1.2.3", testSeed, []byte("unconfigured")}, {"v1.2.3", testSeed, append(bytes.Clone(template), []byte("@RELEASE_PUBLIC_KEY_PEM@")...)},
	} {
		out := t.TempDir()
		sentinel := filepath.Join(out, HelperAsset)
		if err := os.WriteFile(sentinel, []byte("existing"), 0600); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(t.TempDir(), "template")
		if err := os.WriteFile(path, tc.template, 0600); err != nil {
			t.Fatal(err)
		}
		if err := stageWithHelper(tc.tag, "missing", out, "", tc.seed, path); err == nil {
			t.Fatal("accepted invalid input")
		}
		entries, _ := os.ReadDir(out)
		got, _ := os.ReadFile(sentinel)
		if len(entries) != 1 || string(got) != "existing" {
			t.Fatal("rejection changed artifacts")
		}
	}
}
