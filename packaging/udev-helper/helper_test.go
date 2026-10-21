package helper_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/blak0p/attack-shark-linux/internal/udevauth"
	packaged "github.com/blak0p/attack-shark-linux/packaging/udev"
)

type fixture struct {
	root, script, target, counter, id, udev string
	wire                                    []byte
}

func put(t *testing.T, p, s string, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(p, []byte(s), mode); err != nil {
		t.Fatal(err)
	}
}
func setup(t *testing.T) fixture {
	t.Helper()
	raw, err := os.ReadFile("helper.sh")
	if err != nil {
		t.Fatal(err)
	}
	f := fixture{root: t.TempDir()}
	f.script = filepath.Join(f.root, "helper")
	f.target = filepath.Join(f.root, "rules", "policy")
	f.counter = filepath.Join(f.root, "calls")
	f.id = filepath.Join(f.root, "id")
	f.udev = filepath.Join(f.root, "udevadm")
	for _, dir := range []string{"rules", "lock", "sys", "sys/usb", "sys/usb/x6"} {
		if err := os.Mkdir(filepath.Join(f.root, dir), 0700); err != nil {
			t.Fatal(err)
		}
	}
	put(t, filepath.Join(f.root, "sys/usb/x6/idVendor"), "1d57\n", 0600)
	put(t, filepath.Join(f.root, "sys/usb/x6/idProduct"), "fa60\n", 0600)
	for _, device := range []struct{ name, vendor, product string }{{"unrelated", "ffff", "fa60"}, {"other-product", "1d57", "ffff"}} {
		dir := filepath.Join(f.root, "sys/usb", device.name)
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
		put(t, filepath.Join(dir, "idVendor"), device.vendor+"\n", 0600)
		put(t, filepath.Join(dir, "idProduct"), device.product+"\n", 0600)
	}
	put(t, filepath.Join(f.root, "lock", "helper.lock"), "", 0600)
	put(t, f.id, "#!/bin/sh\nprintf '0\\n'\n", 0700)
	put(t, f.udev, "#!/bin/sh\nprintf '%s\\n' \"$*\" >> '"+f.counter+"'\n", 0700)
	// Preserve real mode/type checks; only map this unprivileged fixture owner's
	// UID to root. Production has no ownership or filesystem override switch.
	stat := filepath.Join(f.root, "stat")
	put(t, stat, "#!/bin/sh\n/usr/bin/stat \"$@\" | /usr/bin/sed 's/^"+strings.TrimSpace(uid())+":/0:/'\n", 0700)
	key := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{42}, 32))
	der, err := x509.MarshalPKIXPublicKey(key.Public())
	if err != nil {
		t.Fatal(err)
	}
	pub := string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}))
	replacements := strings.NewReplacer(
		"@RELEASE_PUBLIC_KEY_PEM@", strings.TrimSuffix(pub, "\n"),
		"/usr/local/libexec/attack-shark-linux-udev-helper", f.script,
		"/etc/udev/rules.d/60-attack-shark-x6-hidraw.rules", f.target,
		"/run/attack-shark-linux/helper.lock", filepath.Join(f.root, "lock", "helper.lock"),
		"/run/attack-shark-linux", filepath.Join(f.root, "lock"),
		"TRUSTED_PARENTS='/ /etc /etc/udev /etc/udev/rules.d /run /run/attack-shark-linux /usr /usr/local /usr/local/libexec'", "TRUSTED_PARENTS='"+f.root+" "+filepath.Join(f.root, "rules")+" "+filepath.Join(f.root, "lock")+"'",
		"TRUSTED_PARENTS='/ /etc /etc/udev /etc/udev/rules.d /run /usr /usr/local /usr/local/libexec'", "TRUSTED_PARENTS='"+f.root+" "+filepath.Join(f.root, "rules")+"'",
		"/usr/bin/id", f.id, "/usr/bin/stat", stat, "/usr/bin/udevadm", f.udev,
		"/sys/bus/usb/devices", filepath.Join(f.root, "sys/usb"), "/sys/devices", filepath.Join(f.root, "sys"),
	)
	put(t, f.script, replacements.Replace(string(raw)), 0700)
	e, err := udevauth.NewPolicy("1.2.3")
	if err != nil {
		t.Fatal(err)
	}
	signed, _ := e.SignedBytes()
	f.wire, err = e.Marshal(ed25519.Sign(key, signed))
	if err != nil {
		t.Fatal(err)
	}
	return f
}
func uid() string { out, _ := exec.Command("/usr/bin/id", "-u").Output(); return string(out) }
func run(t *testing.T, f fixture, input []byte, args ...string) (string, string, int) {
	t.Helper()
	cmd := exec.Command("/bin/sh", append([]string{f.script}, args...)...)
	cmd.Stdin = bytes.NewReader(input)
	var out, errout bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errout
	err := cmd.Run()
	code := 0
	if err != nil {
		if e, ok := err.(*exec.ExitError); ok {
			code = e.ExitCode()
		} else {
			t.Fatal(err)
		}
	}
	return out.String(), errout.String(), code
}
func expect(t *testing.T, f fixture, input []byte, wantOut, wantErr string, wantCode int, args ...string) {
	t.Helper()
	out, errout, code := run(t, f, input, args...)
	if out != wantOut || errout != wantErr || code != wantCode {
		t.Fatalf("stdout=%q stderr=%q exit=%d", out, errout, code)
	}
}
func untouched(t *testing.T, f fixture, before []byte) {
	t.Helper()
	after, err := os.ReadFile(f.target)
	if before == nil {
		if !os.IsNotExist(err) {
			t.Fatalf("target created: %v", err)
		}
	} else if err != nil || !bytes.Equal(before, after) {
		t.Fatal("target changed")
	}
	if _, err := os.Stat(f.counter); !os.IsNotExist(err) {
		t.Fatal("utility invoked")
	}
	entries, err := os.ReadDir(filepath.Join(f.root, "lock"))
	if err != nil || len(entries) != 1 {
		t.Fatalf("scratch remains: %v %v", entries, err)
	}
}

// Snapshot every fixture path, without following symlinks, so rejection proves
// zero filesystem effects rather than merely an unchanged destination.
func snapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	result := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		value := info.Mode().String()
		if d.Type()&os.ModeSymlink != 0 {
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			value += target
		} else if d.Type().IsRegular() {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			value += string(data)
		}
		result[path] = value
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}
func unchanged(t *testing.T, f fixture, before map[string]string) {
	t.Helper()
	if !reflect.DeepEqual(before, snapshot(t, f.root)) {
		t.Fatal("rejected input changed fixture filesystem")
	}
}
func TestRejectedInputs(t *testing.T) {
	for _, name := range []string{"tamper", "wrong-key", "domain", "oversize", "extra-data", "custom", "group", "symlink", "unsafe-parent", "root", "custom-path", "lock-symlink", "nonregular", "unsafe-owner", "lock-busy", "wrong-version", "missing-newline", "unconfigured", "lock-mode", "lock-directory-mode"} {
		t.Run(name, func(t *testing.T) {
			f := setup(t)
			input := bytes.Clone(f.wire)
			args := []string{"1.2.3"}
			want := "error: invalid policy\n"
			code := 1
			var before []byte
			switch name {
			case "tamper":
				input = bytes.Replace(input, []byte("version=1.2.3"), []byte("version=1.2.4"), 1)
			case "wrong-key":
				key := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{7}, 32))
				e, _ := udevauth.NewPolicy("1.2.3")
				signed, _ := e.SignedBytes()
				input, _ = e.Marshal(ed25519.Sign(key, signed))
			case "domain":
				input = bytes.Replace(input, []byte(udevauth.PolicyDomain), []byte(udevauth.ScriptDomain), 1)
			case "oversize":
				input = bytes.Repeat([]byte{'x'}, udevauth.MaxEnvelopeBytes+1)
			case "extra-data":
				input = append(input, 'x')
			case "custom", "group":
				before = []byte("# custom\n")
				if name == "group" {
					before = []byte("GROUP=\"users\"\n")
				}
				put(t, f.target, string(before), 0644)
				want = "error: custom policy\n"
			case "symlink":
				dest := filepath.Join(f.root, "original")
				put(t, dest, "custom", 0644)
				if err := os.Symlink(dest, f.target); err != nil {
					t.Fatal(err)
				}
				before = []byte("custom")
				want = "error: unsafe target\n"
			case "unsafe-parent":
				if err := os.Chmod(filepath.Join(f.root, "rules"), 0777); err != nil {
					t.Fatal(err)
				}
				want = "error: unsafe parent\n"
			case "root":
				put(t, f.id, "#!/bin/sh\necho 1000\n", 0700)
				want = "error: root required\n"
			case "custom-path":
				args = append(args, "/tmp/evil")
				want = "usage: attack-shark-linux-udev-helper VERSION < signed-policy\n"
				code = 2
			case "nonregular":
				if err := os.Mkdir(f.target, 0700); err != nil {
					t.Fatal(err)
				}
				want = "error: unsafe target\n"
			case "unsafe-owner":
				put(t, filepath.Join(f.root, "stat"), "#!/bin/sh\necho 1000:700\n", 0700)
				want = "error: unsafe parent\n"
			case "lock-mode":
				if err := os.Chmod(filepath.Join(f.root, "lock", "helper.lock"), 0644); err != nil {
					t.Fatal(err)
				}
				want = "error: unsafe lock\n"
			case "lock-directory-mode":
				if err := os.Chmod(filepath.Join(f.root, "lock"), 0755); err != nil {
					t.Fatal(err)
				}
				want = "error: unsafe lock\n"
			case "lock-busy":
				stub := filepath.Join(f.root, "flock")
				put(t, stub, "#!/bin/sh\nexit 1\n", 0700)
				raw, _ := os.ReadFile(f.script)
				put(t, f.script, strings.ReplaceAll(string(raw), "/usr/bin/flock", stub), 0700)
				want = "error: lock unavailable\n"
			case "wrong-version":
				args = []string{"1.2.4"}
			case "missing-newline":
				input = input[:len(input)-1]
			case "unconfigured":
				raw, _ := os.ReadFile(f.script)
				start := strings.Index(string(raw), "-----BEGIN PUBLIC KEY-----")
				end := strings.Index(string(raw), "-----END PUBLIC KEY-----") + len("-----END PUBLIC KEY-----")
				put(t, f.script, string(raw[:start])+"@RELEASE_PUBLIC_KEY_PEM@"+string(raw[end:]), 0700)
			case "lock-symlink": // Rename preserves the existing file; no host resources touched.
				lock := filepath.Join(f.root, "lock", "helper.lock")
				if err := os.Rename(lock, filepath.Join(f.root, "saved-lock")); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(filepath.Join(f.root, "saved-lock"), lock); err != nil {
					t.Fatal(err)
				}
				want = "error: unsafe lock\n"
			}
			initial := snapshot(t, f.root)
			expect(t, f, input, "", want, code, args...)
			unchanged(t, f, initial)
			if name == "nonregular" {
				if _, err := os.Stat(f.counter); !os.IsNotExist(err) {
					t.Fatal("utility invoked")
				}
			} else {
				untouched(t, f, before)
			}
		})
	}
}
func TestCurrentMissingOldAndReloadFailure(t *testing.T) {
	for _, name := range []string{"current", "missing", "old", "reload-failure", "trigger-failure"} {
		t.Run(name, func(t *testing.T) {
			f := setup(t)
			if name == "current" {
				put(t, f.target, packaged.Policy(), 0644)
				before, _ := os.Stat(f.target)
				expect(t, f, f.wire, "current\n", "", 0, "1.2.3")
				untouched(t, f, []byte(packaged.Policy()))
				after, _ := os.Stat(f.target)
				if !os.SameFile(before, after) {
					t.Fatal("current rewritten")
				}
				return
			}
			if name == "old" {
				old, err := os.ReadFile("../../internal/udevpolicy/testdata/fa60-official.rules")
				if err != nil {
					t.Fatal(err)
				}
				put(t, f.target, string(old), 0644)
			}
			if name == "trigger-failure" {
				put(t, f.udev, "#!/bin/sh\nprintf '%s\\n' \"$*\" >> '"+f.counter+"'\n[ \"$1\" = control ]\n", 0700)
				expect(t, f, f.wire, "installed\n", "error: installed-but-trigger-failed\n", 4, "1.2.3")
			} else if name == "reload-failure" {
				put(t, f.udev, "#!/bin/sh\nprintf '%s\\n' \"$*\" >> '"+f.counter+"'\nexit 1\n", 0700)
				expect(t, f, f.wire, "installed\n", "error: installed-but-reload-failed\n", 3, "1.2.3")
			} else {
				expect(t, f, f.wire, "installed\n", "", 0, "1.2.3")
			}
			data, err := os.ReadFile(f.target)
			if err != nil || string(data) != packaged.Policy() {
				t.Fatal("wrong policy")
			}
			st, _ := os.Stat(f.target)
			if st.Mode().Perm() != 0644 {
				t.Fatal("wrong mode")
			}
			calls, err := os.ReadFile(f.counter)
			if err != nil {
				t.Fatal(err)
			}
			want := "control --reload-rules\n"
			if name != "reload-failure" {
				want += "trigger --action=change --subsystem-match=usb --parent-match=" + filepath.Join(f.root, "sys/usb/x6") + "\n"
				if name != "trigger-failure" {
					want += "trigger --action=change --subsystem-match=hidraw --parent-match=" + filepath.Join(f.root, "sys/usb/x6") + "\n"
				}
			}
			if string(calls) != want {
				t.Fatalf("calls=%q", calls)
			}
		})
	}
}
