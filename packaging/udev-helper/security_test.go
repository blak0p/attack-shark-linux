package helper_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestAuthenticatedRejections(t *testing.T) {
	for _, name := range []string{"directive", "group", "nul", "digest", "history", "signature-bits", "signature-tamper", "unknown-field", "payload-size"} {
		t.Run(name, func(t *testing.T) {
			f := setup(t)
			lines := strings.Split(string(f.wire), "\n")
			switch name {
			case "directive", "group", "nul", "payload-size":
				policy := "RUN+=\"/tmp/evil\"\n"
				if name == "group" {
					policy = "GROUP=\"users\"\n"
				}
				if name == "nul" {
					policy = "# hidden\x00\n"
				}
				if name == "payload-size" {
					policy = strings.Repeat("#x\n", 22000)
				}
				digest := sha256.Sum256([]byte(policy))
				lines[2] = fmt.Sprintf("sha256=%x", digest)
				lines[4] = "payload=" + base64.StdEncoding.EncodeToString([]byte(policy))
			case "digest":
				lines[2] = "sha256=" + strings.Repeat("0", 64)
			case "history":
				lines[3] = "previous=" + strings.Repeat("0", 63)
			case "unknown-field":
				lines[3] = "unknown=value"
			}
			key := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{42}, 32))
			signed := strings.Join(lines[:5], "\n") + "\n"
			sig := base64.StdEncoding.EncodeToString(ed25519.Sign(key, []byte(signed)))
			if name == "signature-bits" { // Change unused base64 padding bits only.
				alphabet := "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
				i := strings.IndexByte(alphabet, sig[85])
				sig = sig[:85] + string(alphabet[i+1]) + sig[86:]
			}
			if name == "signature-tamper" {
				sig = "A" + sig[1:]
			}
			input := []byte(signed + "signature=" + sig + "\n")
			initial := snapshot(t, f.root)
			expect(t, f, input, "", "error: invalid policy\n", 1, "1.2.3")
			unchanged(t, f, initial)
			untouched(t, f, nil)
		})
	}
}

func TestHelpAndUnsafeTargetMode(t *testing.T) {
	f := setup(t)
	expect(t, f, nil, "usage: attack-shark-linux-udev-helper VERSION < signed-policy\n", "", 0, "--help")
	untouched(t, f, nil)
	put(t, f.target, "# custom\n", 0666)
	if err := os.Chmod(f.target, 0666); err != nil {
		t.Fatal(err)
	}
	initial := snapshot(t, f.root)
	expect(t, f, f.wire, "", "error: unsafe target\n", 1, "1.2.3")
	unchanged(t, f, initial)
	untouched(t, f, []byte("# custom\n"))
}
