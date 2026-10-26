package udevauth_test

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/blak0p/attack-shark-linux/internal/udevauth"
	packaged "github.com/blak0p/attack-shark-linux/packaging/udev"
)

func signedHistory(t *testing.T, history, payload string) ([]byte, ed25519.PublicKey) {
	t.Helper()
	pub, key, wire := fixture(t)
	lines := strings.Split(string(wire), "\n")
	lines[2] = fmt.Sprintf("sha256=%x", sha256.Sum256([]byte(payload)))
	lines[3] = "previous=" + history
	lines[4] = "payload=" + base64.StdEncoding.EncodeToString([]byte(payload))
	signed := strings.Join(lines[:5], "\n") + "\n"
	return []byte(signed + "signature=" + base64.StdEncoding.EncodeToString(ed25519.Sign(key, []byte(signed))) + "\n"), pub
}
func TestFutureOfficialPolicy(t *testing.T) {
	current := fmt.Sprintf("%x", sha256.Sum256([]byte(packaged.Policy())))
	history := []string{udevauth.PreviousSHA256, current}
	sort.Strings(history)
	payload := packaged.Policy() + "# Next official release fixture.\n"
	wire, pub := signedHistory(t, strings.Join(history, ","), payload)
	p, err := udevauth.VerifyPolicy(wire, "1.2.3", pub)
	if err != nil || string(p.Bytes) != payload || strings.Join(p.Previous, ",") != strings.Join(history, ",") {
		t.Fatalf("future official policy rejected: %v", err)
	}
	_, _, wire = fixture(t)
	if !strings.Contains(string(wire), "previous="+strings.Join(history, ",")+"\n") {
		t.Fatal("signer omits current official digest from future migration history")
	}
}
func TestHistoryCanonicalRejections(t *testing.T) {
	a, b := strings.Repeat("0", 64), strings.Repeat("f", 64)
	cases := map[string]string{"empty": "", "duplicate": a + "," + a, "unsorted": b + "," + a, "uppercase": strings.Repeat("A", 64), "short": a[:63], "trailing": a + ",", "space": a + ", " + b, "too-many": ""}
	var many []string
	for i := 0; i < 17; i++ {
		many = append(many, fmt.Sprintf("%064x", i))
	}
	cases["too-many"] = strings.Join(many, ",")
	for name, history := range cases {
		t.Run(name, func(t *testing.T) {
			wire, pub := signedHistory(t, history, packaged.Policy())
			if _, err := udevauth.VerifyPolicy(wire, "1.2.3", pub); err == nil {
				t.Fatal("noncanonical history accepted")
			}
		})
	}
	// The maximum canonical list remains valid, including close numeric-looking
	// hashes that shell awk must compare lexically rather than as floating numbers.
	many = nil
	for i := 0; i < 16; i++ {
		many = append(many, strings.Repeat("1", 62)+fmt.Sprintf("%02d", i))
	}
	wire, pub := signedHistory(t, strings.Join(many, ","), packaged.Policy())
	if _, err := udevauth.VerifyPolicy(wire, "1.2.3", pub); err != nil {
		t.Fatalf("maximum history rejected: %v", err)
	}
	// History is release-key authority, not a local hardcoded verifier allowlist.
	wire, pub = signedHistory(t, a, packaged.Policy())
	if _, err := udevauth.VerifyPolicy(wire, "1.2.3", pub); err != nil {
		t.Fatalf("signed history rejected: %v", err)
	}
}
