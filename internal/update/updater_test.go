package update

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

type fakeTransport struct {
	manifests  []Manifest
	artifact   []byte
	err        error
	downloads  int
	onDownload func()
}

func (f *fakeTransport) Manifests(context.Context) ([]Manifest, error) { return f.manifests, f.err }
func (f *fakeTransport) Download(context.Context, string) (io.ReadCloser, error) {
	f.downloads++
	if f.onDownload != nil {
		f.onDownload()
	}
	return io.NopCloser(bytes.NewReader(f.artifact)), f.err
}

func updaterForTest(t *testing.T, current string, manifests []Manifest, artifact []byte, publicKey ed25519.PublicKey) (*Updater, *fakeTransport, string) {
	t.Helper()
	home := t.TempDir()
	transport := &fakeTransport{manifests: manifests, artifact: artifact}
	originalHome := currentAccountHome
	currentAccountHome = func() (string, error) { return home, nil }
	t.Cleanup(func() { currentAccountHome = originalHome })
	return &Updater{
		CurrentVersion: current,
		PublicKey:      base64.StdEncoding.EncodeToString(publicKey),
		Transport:      transport,
	}, transport, home
}

func updaterManifest(t *testing.T, version string, artifact []byte, privateKey ed25519.PrivateKey) Manifest {
	t.Helper()
	digest := sha256.Sum256(artifact)
	return signedManifest(t, version, "https://example.com/attack-shark-linux-x86_64.AppImage", hex.EncodeToString(digest[:]), privateKey)
}

func TestUpdaterFiltersChannelsAndReportsAbsentRC(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	stableArtifact, rcArtifact := []byte("stable"), []byte("rc")
	stable := updaterManifest(t, "1.2.0", stableArtifact, privateKey)
	rc := updaterManifest(t, "1.2.0-rc.2", rcArtifact, privateKey)

	t.Run("stable only selects a newer stable", func(t *testing.T) {
		updater, _, _ := updaterForTest(t, "1.1.0", []Manifest{rc, stable}, stableArtifact, publicKey)
		update, err := updater.Check(context.Background())
		if err != nil {
			t.Fatalf("Check() error = %v", err)
		}
		if update.Release.Version != stable.Version {
			t.Fatalf("version = %q, want %q", update.Release.Version, stable.Version)
		}
	})
	t.Run("RC only selects a newer RC", func(t *testing.T) {
		updater, _, _ := updaterForTest(t, "1.2.0-rc.1", []Manifest{stable, rc}, rcArtifact, publicKey)
		update, err := updater.Check(context.Background())
		if err != nil {
			t.Fatalf("Check() error = %v", err)
		}
		if update.Release.Version != rc.Version {
			t.Fatalf("version = %q, want %q", update.Release.Version, rc.Version)
		}
	})
	t.Run("RC reports no RC", func(t *testing.T) {
		updater, _, _ := updaterForTest(t, "1.2.0-rc.1", []Manifest{stable}, stableArtifact, publicKey)
		_, err := updater.Check(context.Background())
		if !errors.Is(err, ErrNoRCUpdate) {
			t.Fatalf("Check() error = %v, want ErrNoRCUpdate", err)
		}
	})
}

func TestUpdaterGatesSignatureApprovalAndUpgrade(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	artifact := []byte("new AppImage")
	manifest := updaterManifest(t, "1.2.0", artifact, privateKey)

	t.Run("signature fails before download", func(t *testing.T) {
		invalid := manifest
		invalid.Signature = "invalid"
		updater, transport, _ := updaterForTest(t, "1.1.0", []Manifest{invalid}, artifact, publicKey)
		if _, err := updater.Check(context.Background()); err == nil {
			t.Fatal("Check() succeeded with an invalid signature")
		}
		if transport.downloads != 0 {
			t.Fatalf("downloads = %d, want 0", transport.downloads)
		}
	})
	t.Run("unverified input is rejected before download", func(t *testing.T) {
		updater, transport, _ := updaterForTest(t, "1.1.0", nil, artifact, publicKey)
		err := updater.Apply(context.Background(), &VerifiedUpdate{Release: Release{URL: manifest.URL, SHA256: manifest.SHA256}}, true)
		if err == nil || !strings.Contains(err.Error(), "not a verified update") {
			t.Fatalf("Apply() error = %v, want verified update error", err)
		}
		if transport.downloads != 0 {
			t.Fatalf("downloads = %d, want 0", transport.downloads)
		}
	})
	t.Run("approval is required after verification", func(t *testing.T) {
		updater, transport, home := updaterForTest(t, "1.1.0", []Manifest{manifest}, artifact, publicKey)
		update, err := updater.Check(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if err := updater.Apply(context.Background(), update, false); !errors.Is(err, ErrApprovalRequired) {
			t.Fatalf("Apply() error = %v, want ErrApprovalRequired", err)
		}
		if transport.downloads != 0 {
			t.Fatalf("downloads = %d, want 0", transport.downloads)
		}
		if _, err := os.Stat(filepath.Join(home, ".local", "share", "attack-shark-x6", "attack-shark-linux-x86_64.AppImage")); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("target unexpectedly exists: %v", err)
		}
	})
	t.Run("equal and older releases are rejected", func(t *testing.T) {
		for _, version := range []string{"1.1.0", "1.0.9"} {
			candidate := updaterManifest(t, version, artifact, privateKey)
			updater, _, _ := updaterForTest(t, "1.1.0", []Manifest{candidate}, artifact, publicKey)
			if _, err := updater.Check(context.Background()); !errors.Is(err, ErrNoUpdate) {
				t.Fatalf("Check(%s) error = %v, want ErrNoUpdate", version, err)
			}
		}
	})
}

func TestUpdaterAppliesOnlyImmutableVerifiedManifest(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	verifiedArtifact := []byte("verified AppImage")
	mutableArtifact := []byte("caller-controlled AppImage")
	manifest := updaterManifest(t, "1.2.0", verifiedArtifact, privateKey)
	updater, transport, home := updaterForTest(t, "1.1.0", []Manifest{manifest}, mutableArtifact, publicKey)
	target := filepath.Join(home, ".local", "share", "attack-shark-x6", "attack-shark-linux-x86_64.AppImage")
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("old AppImage"), 0o755); err != nil {
		t.Fatal(err)
	}

	update, err := updater.Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	mutableDigest := sha256.Sum256(mutableArtifact)
	update.Release.URL = "https://attacker.invalid/attack-shark.AppImage"
	update.Release.SHA256 = hex.EncodeToString(mutableDigest[:])
	update.Release.Version = "999.999.999"
	if err := updater.Apply(context.Background(), update, true); err == nil || !strings.Contains(err.Error(), "SHA-256") {
		t.Fatalf("Apply() error = %v, want signed manifest digest rejection", err)
	}
	if transport.downloads != 1 {
		t.Fatalf("downloads = %d, want 1", transport.downloads)
	}
}

func TestUpdaterIgnoresForgedHome(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	artifact := []byte("new AppImage")
	manifest := updaterManifest(t, "1.2.0", artifact, privateKey)
	updater, _, accountHome := updaterForTest(t, "1.1.0", []Manifest{manifest}, artifact, publicKey)
	forgedHome := t.TempDir()
	t.Setenv("HOME", forgedHome)
	accountTarget := filepath.Join(accountHome, installedAppImageRelativePath)
	if err := os.MkdirAll(filepath.Dir(accountTarget), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(accountTarget, []byte("old AppImage"), 0o755); err != nil {
		t.Fatal(err)
	}

	update, err := updater.Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := updater.Apply(context.Background(), update, true); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	contents, err := os.ReadFile(accountTarget)
	if err != nil || !bytes.Equal(contents, artifact) {
		t.Fatalf("account target = %q, %v", contents, err)
	}
	if _, err := os.Stat(filepath.Join(forgedHome, installedAppImageRelativePath)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("forged HOME target was used: %v", err)
	}
}

func TestUpdaterCannotEscapeReplacedParent(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	artifact := []byte("new AppImage")
	manifest := updaterManifest(t, "1.2.0", artifact, privateKey)
	updater, transport, home := updaterForTest(t, "1.1.0", []Manifest{manifest}, artifact, publicKey)
	target := filepath.Join(home, installedAppImageRelativePath)
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("old AppImage"), 0o755); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	outsideTarget := filepath.Join(outside, "attack-shark-linux-x86_64.AppImage")
	if err := os.WriteFile(outsideTarget, []byte("outside AppImage"), 0o755); err != nil {
		t.Fatal(err)
	}
	installDir := filepath.Dir(target)
	transport.onDownload = func() {
		if err := os.Rename(installDir, installDir+".original"); err != nil {
			t.Fatalf("move install directory: %v", err)
		}
		if err := os.Symlink(outside, installDir); err != nil {
			t.Fatalf("replace install directory: %v", err)
		}
	}

	update, err := updater.Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := updater.Apply(context.Background(), update, true); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	contents, err := os.ReadFile(outsideTarget)
	if err != nil || !bytes.Equal(contents, []byte("outside AppImage")) {
		t.Fatalf("outside target = %q, %v", contents, err)
	}
}

func TestUpdaterRejectsSymlinkedTargetParents(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	artifact := []byte("new AppImage")
	manifest := updaterManifest(t, "1.2.0", artifact, privateKey)
	updater, _, home := updaterForTest(t, "1.1.0", []Manifest{manifest}, artifact, publicKey)
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(home, ".local")); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(outside, "share", "attack-shark-x6"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outside, "share", "attack-shark-x6", "attack-shark-linux-x86_64.AppImage"), []byte("old AppImage"), 0o755); err != nil {
		t.Fatal(err)
	}

	update, err := updater.Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := updater.Apply(context.Background(), update, true); err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("Apply() error = %v, want symlinked parent rejection", err)
	}
}

func TestUpdaterRestoresExactModeDespiteUmask(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	artifact := []byte("new AppImage")
	manifest := updaterManifest(t, "1.2.0", artifact, privateKey)
	updater, _, home := updaterForTest(t, "1.1.0", []Manifest{manifest}, artifact, publicKey)
	target := filepath.Join(home, ".local", "share", "attack-shark-x6", "attack-shark-linux-x86_64.AppImage")
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("old AppImage"), 0o751); err != nil {
		t.Fatal(err)
	}

	oldUmask := syscall.Umask(0o077)
	defer syscall.Umask(oldUmask)
	update, err := updater.Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := updater.Apply(context.Background(), update, true); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o751 {
		t.Fatalf("mode = %04o, want 0751", got)
	}
}

func TestUpdaterRecoveryKeepsMarkerWhenRestoreIsNotDurable(t *testing.T) {
	home := t.TempDir()
	originalHome := currentAccountHome
	currentAccountHome = func() (string, error) { return home, nil }
	t.Cleanup(func() { currentAccountHome = originalHome })
	target := filepath.Join(home, ".local", "share", "attack-shark-x6", "attack-shark-linux-x86_64.AppImage")
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target+".backup", []byte("old AppImage"), 0o751); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target+".pending", []byte("pending\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	originalSync := syncUpdateDirectory
	t.Cleanup(func() { syncUpdateDirectory = originalSync })
	syncUpdateDirectory = func(*os.File) error { return errors.New("simulated directory sync failure") }
	updater := &Updater{}
	if err := updater.Recover(); err == nil || !strings.Contains(err.Error(), "sync") {
		t.Fatalf("recover() error = %v, want sync failure", err)
	}
	if _, err := os.Lstat(target + ".pending"); err != nil {
		t.Fatalf("pending marker was removed before restored target was durable: %v", err)
	}
	contents, err := os.ReadFile(target)
	if err != nil || !bytes.Equal(contents, []byte("old AppImage")) {
		t.Fatalf("restored target = %q, %v", contents, err)
	}
}

func TestUpdaterConfinementDigestRecoveryAndIdempotency(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	old, replacement := []byte("old AppImage"), []byte("new AppImage")
	manifest := updaterManifest(t, "1.2.0", replacement, privateKey)
	updater, transport, home := updaterForTest(t, "1.1.0", []Manifest{manifest}, replacement, publicKey)
	target := filepath.Join(home, ".local", "share", "attack-shark-x6", "attack-shark-linux-x86_64.AppImage")
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, old, 0o755); err != nil {
		t.Fatal(err)
	}

	update, err := updater.Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := updater.Apply(context.Background(), update, true); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	got, err := os.ReadFile(target)
	if err != nil || !bytes.Equal(got, replacement) {
		t.Fatalf("replacement = %q, %v", got, err)
	}
	info, err := os.Stat(target)
	if err != nil || info.Mode().Perm() != 0o755 {
		t.Fatalf("mode = %v, %v; want 0755", info.Mode(), err)
	}
	if err := updater.Apply(context.Background(), update, true); !errors.Is(err, ErrUpdateAlreadyApplied) {
		t.Fatalf("second Apply() error = %v, want ErrUpdateAlreadyApplied", err)
	}
	if transport.downloads != 1 {
		t.Fatalf("downloads = %d, want 1", transport.downloads)
	}

	backup := target + ".backup"
	pending := target + ".pending"
	if err := os.Rename(target, backup); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pending, []byte("pending"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := updater.Recover(); err != nil {
		t.Fatalf("Recover() error = %v", err)
	}
	got, err = os.ReadFile(target)
	if err != nil || !bytes.Equal(got, replacement) {
		t.Fatalf("recovered replacement = %q, %v", got, err)
	}
	if _, err := os.Stat(pending); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("pending remains: %v", err)
	}

	transport.artifact = []byte("tampered")
	update, err = updater.Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := updater.Apply(context.Background(), update, true); err == nil || !strings.Contains(err.Error(), "SHA-256") {
		t.Fatalf("Apply() error = %v, want SHA-256 mismatch", err)
	}
	got, _ = os.ReadFile(target)
	if !bytes.Equal(got, replacement) {
		t.Fatalf("digest mismatch changed target to %q", got)
	}
}
