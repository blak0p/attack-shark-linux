// Package udevauth defines bounded, domain-separated release artifacts. It does
// not install files, elevate, or alter the legacy AppImage signature contract.
package udevauth

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/blak0p/attack-shark-linux/internal/update"
	packaged "github.com/blak0p/attack-shark-linux/packaging/udev"
)

const (
	PolicyDomain       = "attack-shark-udev-policy-v1"
	ScriptDomain       = "attack-shark-udev-helper-v1"
	MaxEnvelopeBytes   = 96 * 1024
	MaxPolicyBytes     = 64 * 1024
	MaxScriptBytes     = 64 * 1024
	MaxPreviousDigests = 16
	// InitialUnifiedSHA256 preserves the first official fa60+fa61 policy even
	// after the canonical embed changes. Append outgoing official digests to
	// KnownOfficialDigests before subsequent canonical-policy changes.
	InitialUnifiedSHA256 = "c2862bc932a3454f16f2759510d23dc408b1ce9f015b1dce576cb4f58d2274be"
	// PreviousSHA256 identifies T1's checked-in fa60-only official snapshot.
	PreviousSHA256 = "d7e14b66e339ab394c00c1881759443068001245ed163f7a49e511b264ee32f6"
)

var digestPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
var rulePattern = regexp.MustCompile(`^SUBSYSTEM=="hidraw", ATTRS\{idVendor\}=="1d57", ATTRS\{idProduct\}=="fa6[01]", TAG\+="uaccess", MODE="0660"$`)

// Envelope is a signing input. Marshal requires a detached Ed25519 signature.
// Policy constructors always obtain current bytes from the canonical embed.
// The fields are private so callers cannot accidentally sign a different policy.
type Envelope struct {
	domain, version, digest, previous string
	payload                           []byte
}
type Policy struct {
	Version  string
	Bytes    []byte
	Previous []string
}

// KnownOfficialDigests returns a fresh, sorted, unique release-signing history.
// Its sources are checked-in official digests and the canonical embedded policy,
// never renderer input or runtime git. Keep outgoing policy digests here when
// editing the canonical policy; historical raw current-policy copies are unneeded.
func KnownOfficialDigests() []string {
	known := []string{PreviousSHA256, InitialUnifiedSHA256, sum([]byte(packaged.Policy()))}
	sort.Strings(known)
	result := make([]string, 0, len(known))
	for _, digest := range known {
		if len(result) == 0 || result[len(result)-1] != digest {
			result = append(result, digest)
		}
	}
	return result
}

func NewPolicy(version string) (Envelope, error) {
	data := []byte(packaged.Policy())
	e := Envelope{PolicyDomain, version, sum(data), strings.Join(KnownOfficialDigests(), ","), data}
	_, err := e.SignedBytes()
	return e, err
}

// NewScriptManifest binds final rendered bytes, not the unconfigured template.
func NewScriptManifest(version string, script []byte) (Envelope, error) {
	if len(script) == 0 || len(script) > MaxScriptBytes {
		return Envelope{}, fmt.Errorf("invalid script size")
	}
	e := Envelope{domain: ScriptDomain, version: version, digest: sum(script)}
	_, err := e.SignedBytes()
	return e, err
}

func validVersion(v string) bool {
	if len(v) == 0 || len(v) > 80 || strings.ContainsAny(v, "\r\n= ") {
		return false
	}
	_, err := update.CompareVersions(v, v)
	return err == nil
}
func sum(b []byte) string { return fmt.Sprintf("%x", sha256.Sum256(b)) }
func validPolicy(b []byte) bool {
	if len(b) == 0 || len(b) > MaxPolicyBytes || b[len(b)-1] != '\n' {
		return false
	}
	count := 0
	for _, line := range strings.Split(string(b[:len(b)-1]), "\n") {
		for _, c := range line {
			if c < 32 || c == 127 {
				return false
			}
		}
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}
		if !rulePattern.MatchString(line) {
			return false
		}
		count++
	}
	return count > 0
}
func validHistory(history string) bool {
	items := strings.Split(history, ",")
	if len(items) > MaxPreviousDigests {
		return false
	}
	for i, digest := range items {
		if !digestPattern.MatchString(digest) || (i > 0 && items[i-1] >= digest) {
			return false
		}
	}
	return true
}

func (e Envelope) SignedBytes() ([]byte, error) {
	if !validVersion(e.version) || !digestPattern.MatchString(e.digest) {
		return nil, fmt.Errorf("invalid artifact metadata")
	}
	text := e.domain + "\nversion=" + e.version + "\nsha256=" + e.digest + "\n"
	switch e.domain {
	case PolicyDomain:
		if !validHistory(e.previous) || !validPolicy(e.payload) || sum(e.payload) != e.digest {
			return nil, fmt.Errorf("invalid policy")
		}
		text += "previous=" + e.previous + "\npayload=" + base64.StdEncoding.EncodeToString(e.payload) + "\n"
	case ScriptDomain:
	default:
		return nil, fmt.Errorf("invalid artifact domain")
	}
	return []byte(text), nil
}
func (e Envelope) Marshal(signature []byte) ([]byte, error) {
	signed, err := e.SignedBytes()
	if err != nil {
		return nil, err
	}
	if len(signature) != ed25519.SignatureSize {
		return nil, fmt.Errorf("invalid signature size")
	}
	return append(signed, []byte("signature="+base64.StdEncoding.EncodeToString(signature)+"\n")...), nil
}
func parse(wire []byte, domain, expected string, key ed25519.PublicKey) (Envelope, error) {
	fail := func() (Envelope, error) { return Envelope{}, fmt.Errorf("invalid authenticated artifact") }
	if len(wire) > MaxEnvelopeBytes || len(key) != ed25519.PublicKeySize || !validVersion(expected) {
		return fail()
	}
	lines := strings.Split(string(wire), "\n")
	n := 4
	if domain == PolicyDomain {
		n = 6
	}
	if len(lines) != n+1 || lines[n] != "" || lines[0] != domain {
		return fail()
	}
	field := func(i int, prefix string) string {
		if !strings.HasPrefix(lines[i], prefix) {
			return ""
		}
		return strings.TrimPrefix(lines[i], prefix)
	}
	e := Envelope{domain: domain, version: field(1, "version="), digest: field(2, "sha256=")}
	if e.version != expected {
		return fail()
	}
	if domain == PolicyDomain {
		e.previous = field(3, "previous=")
		var err error
		encoded := field(4, "payload=")
		e.payload, err = base64.StdEncoding.Strict().DecodeString(encoded)
		if err != nil || base64.StdEncoding.EncodeToString(e.payload) != encoded {
			return fail()
		}
	}
	signature, err := base64.StdEncoding.Strict().DecodeString(field(n-1, "signature="))
	if err != nil {
		return fail()
	}
	canonical, err := e.Marshal(signature)
	if err != nil || !bytes.Equal(canonical, wire) {
		return fail()
	}
	signed, err := e.SignedBytes()
	if err != nil || !ed25519.Verify(key, signed, signature) {
		return fail()
	}
	return e, nil
}

// VerifyPolicy requires exact release-version equality, not an upgrade ordering.
// Same-version udev-only installation is supported; aliases (v1 versus 1) differ.
func VerifyPolicy(wire []byte, expectedVersion string, key ed25519.PublicKey) (Policy, error) {
	e, err := parse(wire, PolicyDomain, expectedVersion, key)
	if err != nil {
		return Policy{}, err
	}
	return Policy{e.version, bytes.Clone(e.payload), strings.Split(e.previous, ",")}, nil
}
func VerifyScript(wire, script []byte, expectedVersion string, key ed25519.PublicKey) error {
	e, err := parse(wire, ScriptDomain, expectedVersion, key)
	if err != nil {
		return err
	}
	if len(script) == 0 || len(script) > MaxScriptBytes || sum(script) != e.digest {
		return fmt.Errorf("script digest mismatch")
	}
	return nil
}
