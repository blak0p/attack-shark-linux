package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/blak0p/attack-shark-linux/internal/udevauth"
)

const (
	HelperAsset         = "attack-shark-linux-udev-helper"
	HelperManifestAsset = "udev-helper-manifest.txt"
	PolicyAsset         = "udev-policy.txt"
)

// BuildAuthArtifacts renders and signs separate artifacts in memory. Version is
// bound to the normalized release tag without its v prefix, as in the AppImage
// manifest. No output is written on invalid template, key, or signing metadata.
func BuildAuthArtifacts(tag string, template []byte, encodedSeed string) (map[string][]byte, error) {
	if _, err := normalizeTag(tag); err != nil {
		return nil, err
	}
	public, err := publicKeyFromSeed(encodedSeed)
	if err != nil {
		return nil, err
	}
	const placeholder = "@RELEASE_PUBLIC_KEY_PEM@"
	if bytes.Count(template, []byte(placeholder)) != 1 {
		return nil, errors.New("helper template must contain exactly one public-key placeholder")
	}
	key, err := base64.StdEncoding.DecodeString(public)
	if err != nil {
		return nil, err
	}
	der, err := x509.MarshalPKIXPublicKey(ed25519.PublicKey(key))
	if err != nil {
		return nil, err
	}
	rendered := bytes.Replace(template, []byte(placeholder), bytes.TrimSpace(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})), 1)
	version := strings.TrimPrefix(tag, "v")
	script, err := udevauth.NewScriptManifest(version, rendered)
	if err != nil {
		return nil, err
	}
	policy, err := udevauth.NewPolicy(version)
	if err != nil {
		return nil, err
	}
	seed, err := base64.StdEncoding.DecodeString(encodedSeed)
	if err != nil {
		return nil, err
	}
	private := ed25519.NewKeyFromSeed(seed)
	sign := func(e udevauth.Envelope) ([]byte, error) {
		payload, err := e.SignedBytes()
		if err != nil {
			return nil, err
		}
		return e.Marshal(ed25519.Sign(private, payload))
	}
	manifest, err := sign(script)
	if err != nil {
		return nil, err
	}
	envelope, err := sign(policy)
	if err != nil {
		return nil, err
	}
	return map[string][]byte{HelperAsset: rendered, HelperManifestAsset: manifest, PolicyAsset: envelope}, nil
}

func stageWithHelper(tag, appImage, outputDir, url, seed, templatePath string) error {
	if templatePath == "" {
		return stage(tag, appImage, outputDir, url, seed)
	}
	template, err := os.ReadFile(templatePath)
	if err != nil {
		return err
	}
	artifacts, err := BuildAuthArtifacts(tag, template, seed)
	if err != nil {
		return err
	}
	if err := stage(tag, appImage, outputDir, url, seed); err != nil {
		return err
	}
	for _, name := range []string{HelperAsset, HelperManifestAsset, PolicyAsset} {
		// Non-executable until authenticated and explicitly installed by bootstrap.
		if err := os.WriteFile(filepath.Join(outputDir, name), artifacts[name], 0644); err != nil {
			return err
		}
	}
	return nil
}
