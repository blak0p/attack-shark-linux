package udevauth_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/blak0p/attack-shark-linux/internal/udevauth"
	packaged "github.com/blak0p/attack-shark-linux/packaging/udev"
)

func fixture(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey, []byte) {
	t.Helper()
	key := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{42}, ed25519.SeedSize))
	e, err := udevauth.NewPolicy("1.2.3")
	if err != nil {
		t.Fatal(err)
	}
	signed, err := e.SignedBytes()
	if err != nil {
		t.Fatal(err)
	}
	wire, err := e.Marshal(ed25519.Sign(key, signed))
	if err != nil {
		t.Fatal(err)
	}
	return key.Public().(ed25519.PublicKey), key, wire
}

func TestPolicyPublicContract(t *testing.T) {
	pub, _, wire := fixture(t)
	policy, err := udevauth.VerifyPolicy(wire, "1.2.3", pub)
	if err != nil || string(policy.Bytes) != packaged.Policy() || strings.Join(policy.Previous, ",") != strings.Join(udevauth.KnownOfficialDigests(), ",") {
		t.Fatalf("policy=%+v err=%v", policy, err)
	}
	// Equality is intentional: a udev-only install does not require a newer app.
	if _, err := udevauth.VerifyPolicy(wire, "1.2.4", pub); err == nil {
		t.Fatal("wrong release accepted")
	}
}

func TestPolicyRejections(t *testing.T) {
	pub, key, wire := fixture(t)
	wrong := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{7}, ed25519.SeedSize)).Public().(ed25519.PublicKey)
	cases := map[string][]byte{
		"tamper":        bytes.Replace(wire, []byte("version=1.2.3"), []byte("version=1.2.4"), 1),
		"extra-data":    append(bytes.Clone(wire), '\n'),
		"unknown-field": bytes.Replace(wire, []byte("version="), []byte("unknown="), 1),
		"oversize":      bytes.Repeat([]byte{'x'}, udevauth.MaxEnvelopeBytes+1),
		"domain":        bytes.Replace(wire, []byte(udevauth.PolicyDomain), []byte(udevauth.ScriptDomain), 1),
		"digest":        bytes.Replace(wire, []byte("sha256="), []byte("sha256=f"), 1),
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := udevauth.VerifyPolicy(input, "1.2.3", pub); err == nil {
				t.Fatal("accepted")
			}
		})
	}
	if _, err := udevauth.VerifyPolicy(wire, "1.2.3", wrong); err == nil {
		t.Fatal("wrong key accepted")
	}
	// Even an authenticated signer cannot introduce arbitrary directives.
	lines := strings.Split(string(wire), "\n")
	for _, rule := range []string{"RUN+=\"/tmp/evil\"\n", "SUBSYSTEM==\"hidraw\", OWNER=\"nobody\"\n", "# comment\x00\n"} {
		sum := sha256.Sum256([]byte(rule))
		lines[2] = fmt.Sprintf("sha256=%x", sum)
		lines[4] = "payload=" + base64.StdEncoding.EncodeToString([]byte(rule))
		signed := strings.Join(lines[:5], "\n") + "\n"
		bad := []byte(signed + "signature=" + base64.StdEncoding.EncodeToString(ed25519.Sign(key, []byte(signed))) + "\n")
		if _, err := udevauth.VerifyPolicy(bad, "1.2.3", pub); err == nil {
			t.Fatal("directive accepted")
		}
	}
	_, _, wire = fixture(t)
	lines = strings.Split(string(wire), "\n")
	lines[3] = "previous=" + strings.Repeat("0", 64)
	signed := strings.Join(lines[:5], "\n") + "\n"
	bad := []byte(signed + "signature=" + base64.StdEncoding.EncodeToString(ed25519.Sign(key, []byte(signed))) + "\n")
	if _, err := udevauth.VerifyPolicy(bad, "1.2.3", pub); err != nil {
		t.Fatalf("authenticated history rejected: %v", err)
	}
}

func TestOfficialHistoryAndBounds(t *testing.T) {
	previous, err := os.ReadFile("../udevpolicy/testdata/fa60-official.rules")
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprintf("%x", sha256.Sum256(previous)) != udevauth.PreviousSHA256 {
		t.Fatal("official fixture drift")
	}
	for _, version := range []string{"", "1.2", "01.2.3", "1.2.3\n", strings.Repeat("1", 81)} {
		if _, err := udevauth.NewPolicy(version); err == nil {
			t.Fatalf("accepted version %q", version)
		}
	}
	for _, script := range [][]byte{nil, bytes.Repeat([]byte{'x'}, udevauth.MaxScriptBytes+1)} {
		if _, err := udevauth.NewScriptManifest("1.2.3", script); err == nil {
			t.Fatal("invalid script size accepted")
		}
	}
	pub, _, wire := fixture(t)
	if _, err := udevauth.VerifyPolicy(wire, "1.2.3", pub[:31]); err == nil {
		t.Fatal("short public key accepted")
	}
	e, _ := udevauth.NewPolicy("1.2.3")
	if _, err := e.Marshal(nil); err == nil {
		t.Fatal("missing signature accepted")
	}
}

func TestScriptManifestPublicContract(t *testing.T) {
	pub, key, policy := fixture(t)
	script := []byte("#!/bin/sh\nexit 0\n")
	e, err := udevauth.NewScriptManifest("1.2.3", script)
	if err != nil {
		t.Fatal(err)
	}
	signed, err := e.SignedBytes()
	if err != nil {
		t.Fatal(err)
	}
	wire, err := e.Marshal(ed25519.Sign(key, signed))
	if err != nil {
		t.Fatal(err)
	}
	if err := udevauth.VerifyScript(wire, script, "1.2.3", pub); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		wire, script []byte
		version      string
	}{
		{wire, append(bytes.Clone(script), 'x'), "1.2.3"}, {policy, script, "1.2.3"}, {wire, script, "1.2.4"}, {append(bytes.Clone(wire), 'x'), script, "1.2.3"},
	} {
		if err := udevauth.VerifyScript(tc.wire, tc.script, tc.version, pub); err == nil {
			t.Fatal("bad script accepted")
		}
	}
}
