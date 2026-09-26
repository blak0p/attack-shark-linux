package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/blak0p/attack-shark-linux/internal/update"
)

const testSeed = "AAECAwQFBgcICQoLDA0ODxAREhMUFRYXGBkaGxwdHh8="

func TestNormalizeTagAcceptsOnlyReleaseTags(t *testing.T) {
	for _, tag := range []string{"v1.2.3", "v1.2.3-rc.0", "v1.2.3-rc.7"} {
		if got, err := normalizeTag(tag); err != nil || got != tag {
			t.Errorf("normalizeTag(%q) = %q, %v; want unchanged accepted tag", tag, got, err)
		}
	}
	for _, tag := range []string{"1.2.3", "v01.2.3", "v1.2.3-rc.01", "v1.2.3-rc.1+build", "v1.2.3-beta.1", "v1.2.3-rc", "v1.2.3-rc.-1"} {
		if _, err := normalizeTag(tag); err == nil {
			t.Errorf("normalizeTag(%q) unexpectedly succeeded", tag)
		}
	}
}

func TestValidateAnnotatedTagAcceptsOnlyAnnotatedNormalizedReleaseTags(t *testing.T) {
	repository := t.TempDir()
	runGit := func(args ...string) {
		t.Helper()
		command := exec.Command("git", args...)
		command.Dir = repository
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, output)
		}
	}
	runGit("init")
	runGit("-c", "user.name=Release Test", "-c", "user.email=release-test@example.invalid", "commit", "--allow-empty", "-m", "initial")
	runGit("-c", "user.name=Release Test", "-c", "user.email=release-test@example.invalid", "tag", "-a", "v1.2.3", "-m", "stable")
	runGit("tag", "v1.2.3-rc.1")

	if err := validateAnnotatedTag("v1.2.3", repository); err != nil {
		t.Fatalf("validate annotated stable tag: %v", err)
	}
	if err := validateAnnotatedTag("v1.2.3-rc.1", repository); err == nil {
		t.Fatal("validate lightweight RC tag unexpectedly succeeded")
	}
}

func TestStageWritesCanonicalDeterministicSignedArtifacts(t *testing.T) {
	inputDir := t.TempDir()
	appImage := filepath.Join(inputDir, canonicalAppImage)
	contents := []byte("non-secret test AppImage bytes\n")
	if err := os.WriteFile(appImage, contents, 0o755); err != nil {
		t.Fatal(err)
	}
	outputDir := filepath.Join(t.TempDir(), "release")
	url := "https://github.com/blak0p/attack-shark-linux/releases/download/v1.2.3/" + canonicalAppImage
	if err := stage("v1.2.3", appImage, outputDir, url, testSeed); err != nil {
		t.Fatalf("stage() error = %v", err)
	}

	staged, err := os.ReadFile(filepath.Join(outputDir, canonicalAppImage))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(staged, contents) {
		t.Fatalf("staged AppImage = %q, want %q", staged, contents)
	}
	digest := sha256.Sum256(contents)
	wantSidecar := fmtDigest(digest[:]) + "  " + canonicalAppImage + "\n"
	if got, err := os.ReadFile(filepath.Join(outputDir, canonicalAppImage+".sha256")); err != nil || string(got) != wantSidecar {
		t.Fatalf("digest sidecar = %q, %v; want %q", got, err, wantSidecar)
	}

	manifestBytes, err := os.ReadFile(filepath.Join(outputDir, "update-manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest update.Manifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Version != "1.2.3" || manifest.URL != url || manifest.SHA256 != fmtDigest(digest[:]) {
		t.Fatalf("manifest = %#v", manifest)
	}
	seed, _ := base64.StdEncoding.DecodeString(testSeed)
	publicKey := ed25519.NewKeyFromSeed(seed).Public().(ed25519.PublicKey)
	if err := update.VerifyManifest(manifest, "1.2.2", base64.StdEncoding.EncodeToString(publicKey)); err != nil {
		t.Fatalf("update.VerifyManifest() = %v", err)
	}
	if bytes.Contains(manifestBytes, []byte(testSeed)) {
		t.Fatal("manifest must not contain the signing seed")
	}

	first := append([]byte(nil), manifestBytes...)
	if err := stage("v1.2.3", appImage, outputDir, url, testSeed); err != nil {
		t.Fatalf("second stage() error = %v", err)
	}
	second, err := os.ReadFile(filepath.Join(outputDir, "update-manifest.json"))
	if err != nil || !bytes.Equal(first, second) {
		t.Fatalf("manifest is not deterministic: %v", err)
	}
}
