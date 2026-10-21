package main

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/blak0p/attack-shark-linux/internal/udevauth"
)

func TestStageCLIParseErrors(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "release")
	build := exec.Command("go", "build", "-o", binary, ".")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build release: %v\n%s", err, output)
	}
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"unknown flag", []string{"-unknown"}},
		{"missing tag value", []string{"-tag"}},
		{"short help", []string{"-h"}},
		{"long help", []string{"--help"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cwd := t.TempDir()
			out := filepath.Join(cwd, "output")
			if err := os.Mkdir(out, 0700); err != nil {
				t.Fatal(err)
			}
			for _, path := range []string{filepath.Join(cwd, "sentinel"), filepath.Join(out, "sentinel")} {
				if err := os.WriteFile(path, []byte("unchanged"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			args := append([]string{"stage", "-output-dir", out}, tc.args...)
			command := exec.Command(binary, args...)
			command.Dir = cwd
			var stdout, stderr bytes.Buffer
			command.Stdout, command.Stderr = &stdout, &stderr
			err := command.Run()
			if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 1 {
				t.Errorf("exit = %v, want 1", err)
			}
			if stdout.String() != "" || stderr.String() != "release: invalid stage arguments\n" {
				t.Errorf("stdout = %q, stderr = %q", stdout.String(), stderr.String())
			}
			for _, dir := range []string{cwd, out} {
				entries, err := os.ReadDir(dir)
				wantCount := 1
				if dir == cwd {
					wantCount = 2
				}
				if err != nil || len(entries) != wantCount {
					t.Errorf("entries in %s = %v, %v", dir, entries, err)
				}
				got, err := os.ReadFile(filepath.Join(dir, "sentinel"))
				if err != nil || string(got) != "unchanged" {
					t.Errorf("sentinel = %q, %v", got, err)
				}
			}
		})
	}
}

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
