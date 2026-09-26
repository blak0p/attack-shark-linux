// Command release stages signed, repository-local AppImage release artifacts.
package main

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/blak0p/attack-shark-linux/internal/update"
)

const (
	canonicalAppImage = "attack-shark-linux-x86_64.AppImage"
	signingSeedEnv    = "ATTACK_SHARK_RELEASE_SIGNING_SEED"
)

var releaseTag = regexp.MustCompile(`^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-rc\.(0|[1-9][0-9]*))?$`)

func main() {
	if len(os.Args) < 2 {
		fatal(errors.New("usage: release <public-key|stage>"))
	}

	switch os.Args[1] {
	case "public-key":
		publicKey, err := publicKeyFromSeed(os.Getenv(signingSeedEnv))
		if err != nil {
			fatal(err)
		}
		fmt.Println(publicKey)
	case "stage":
		flags := flag.NewFlagSet("stage", flag.ContinueOnError)
		flags.SetOutput(io.Discard)
		tag := flags.String("tag", "", "annotated release tag")
		appImage := flags.String("appimage", "", "canonical AppImage path")
		outputDir := flags.String("output-dir", "", "release artifact directory")
		url := flags.String("url", "", "HTTPS release URL")
		if err := flags.Parse(os.Args[2:]); err != nil {
			fatal(errors.New("invalid stage arguments"))
		}
		if err := validateAnnotatedTag(*tag, "."); err != nil {
			fatal(err)
		}
		if err := stage(*tag, *appImage, *outputDir, *url, os.Getenv(signingSeedEnv)); err != nil {
			fatal(err)
		}
	default:
		fatal(errors.New("usage: release <public-key|stage>"))
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "release:", err)
	os.Exit(1)
}

func normalizeTag(tag string) (string, error) {
	if !releaseTag.MatchString(tag) {
		return "", errors.New("tag must be normalized vMAJOR.MINOR.PATCH or vMAJOR.MINOR.PATCH-rc.N")
	}
	return tag, nil
}

func validateAnnotatedTag(tag, repository string) error {
	if _, err := normalizeTag(tag); err != nil {
		return err
	}
	command := exec.Command("git", "cat-file", "-e", "refs/tags/"+tag+"^{tag}")
	command.Dir = repository
	if output, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("tag %q must exist locally as an annotated tag: %s", tag, strings.TrimSpace(string(output)))
	}
	return nil
}

func publicKeyFromSeed(encodedSeed string) (string, error) {
	seed, err := base64.StdEncoding.DecodeString(encodedSeed)
	if err != nil || len(seed) != ed25519.SeedSize {
		return "", errors.New("release signing seed must be a base64-encoded 32-byte Ed25519 seed")
	}
	privateKey := ed25519.NewKeyFromSeed(seed)
	return base64.StdEncoding.EncodeToString(privateKey.Public().(ed25519.PublicKey)), nil
}

func stage(tag, appImage, outputDir, url, encodedSeed string) error {
	normalizedTag, err := normalizeTag(tag)
	if err != nil {
		return err
	}
	if filepath.Base(appImage) != canonicalAppImage {
		return fmt.Errorf("AppImage must be named %s", canonicalAppImage)
	}
	if outputDir == "" {
		return errors.New("release output directory is required")
	}
	seed, err := base64.StdEncoding.DecodeString(encodedSeed)
	if err != nil || len(seed) != ed25519.SeedSize {
		return errors.New("release signing seed must be a base64-encoded 32-byte Ed25519 seed")
	}

	contents, err := os.ReadFile(appImage)
	if err != nil {
		return fmt.Errorf("read AppImage: %w", err)
	}
	digest := sha256.Sum256(contents)
	manifest := update.Manifest{
		Version: strings.TrimPrefix(normalizedTag, "v"),
		URL:     url,
		SHA256:  fmtDigest(digest[:]),
	}
	payload, err := manifest.SignedBytes()
	if err != nil {
		return fmt.Errorf("build manifest: %w", err)
	}
	manifest.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(ed25519.NewKeyFromSeed(seed), payload))
	manifestBytes, err := json.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("encode manifest: %w", err)
	}

	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("create release directory: %w", err)
	}
	if err := os.WriteFile(filepath.Join(outputDir, canonicalAppImage), contents, 0o755); err != nil {
		return fmt.Errorf("stage AppImage: %w", err)
	}
	if err := os.WriteFile(filepath.Join(outputDir, canonicalAppImage+".sha256"), []byte(fmtDigest(digest[:])+"  "+canonicalAppImage+"\n"), 0o644); err != nil {
		return fmt.Errorf("write digest sidecar: %w", err)
	}
	if err := os.WriteFile(filepath.Join(outputDir, "update-manifest.json"), append(manifestBytes, '\n'), 0o644); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}
	return nil
}

func fmtDigest(digest []byte) string {
	return hex.EncodeToString(digest)
}
