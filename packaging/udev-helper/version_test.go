package helper_test

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/blak0p/attack-shark-linux/internal/udevauth"
	packaged "github.com/blak0p/attack-shark-linux/packaging/udev"
)

// Sign directly with the deterministic fixture key to exercise the executable
// against authenticated malformed versions that the production signer refuses.
func versionWire(f fixture, version string) []byte {
	lines := strings.Split(string(f.wire), "\n")
	lines[1] = "version=" + version
	signed := strings.Join(lines[:5], "\n") + "\n"
	key := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{42}, 32))
	return []byte(signed + "signature=" + base64.StdEncoding.EncodeToString(ed25519.Sign(key, []byte(signed))) + "\n")
}

func TestVersionRejections(t *testing.T) {
	for _, tc := range []struct{ name, signed, expected string }{
		{"octal-argument", "1.2.3", `\061.2.3`},
		{"hex-argument", "1.2.3", `\x31.2.3`},
		{"leading-zero", "01.2.3", "01.2.3"},
		{"empty-prerelease-identifier", "1.2.3-a..b", "1.2.3-a..b"},
		{"numeric-prerelease-leading-zero", "1.2.3-a.01", "1.2.3-a.01"},
		{"repeated-build-separator", "1.2.3+build+bad", "1.2.3+build+bad"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := setup(t)
			put(t, f.target, packaged.Policy(), 0644)
			initial := snapshot(t, f.root)
			wire := versionWire(f, tc.signed)
			key := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{42}, 32))
			if _, err := udevauth.NewPolicy(tc.expected); err == nil {
				t.Fatal("Go signer accepted invalid expected version")
			}
			if _, err := udevauth.VerifyPolicy(wire, tc.expected, key.Public().(ed25519.PublicKey)); err == nil {
				t.Fatal("Go verifier accepted invalid expected version")
			}
			out, errout, code := run(t, f, wire, tc.expected)
			unchanged(t, f, initial)
			untouched(t, f, []byte(packaged.Policy()))
			if out != "" || errout != "error: invalid policy\n" || code != 1 {
				t.Fatalf("stdout=%q stderr=%q exit=%d", out, errout, code)
			}
		})
	}
}

func TestValidVersionPreservation(t *testing.T) {
	for _, version := range []string{"1.2.3", "v1.2.3-a.0+build.01"} {
		t.Run(version, func(t *testing.T) {
			f := setup(t)
			put(t, f.target, packaged.Policy(), 0644)
			initial := snapshot(t, f.root)
			expect(t, f, versionWire(f, version), "current\n", "", 0, version)
			unchanged(t, f, initial)
			untouched(t, f, []byte(packaged.Policy()))
		})
	}
}
