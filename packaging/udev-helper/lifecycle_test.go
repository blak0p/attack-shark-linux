package helper_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/blak0p/attack-shark-linux/internal/udevauth"
	packaged "github.com/blak0p/attack-shark-linux/packaging/udev"
)

func nextPolicy(f fixture, history, payload string) []byte {
	lines := strings.Split(string(f.wire), "\n")
	lines[2] = fmt.Sprintf("sha256=%x", sha256.Sum256([]byte(payload)))
	lines[3] = "previous=" + history
	lines[4] = "payload=" + base64.StdEncoding.EncodeToString([]byte(payload))
	signed := strings.Join(lines[:5], "\n") + "\n"
	key := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{42}, 32))
	return []byte(signed + "signature=" + base64.StdEncoding.EncodeToString(ed25519.Sign(key, []byte(signed))) + "\n")
}
func TestFutureOfficialMigration(t *testing.T) {
	for _, name := range []string{"current", "old", "custom", "unsigned-history"} {
		t.Run(name, func(t *testing.T) {
			f := setup(t)
			history := []string{udevauth.PreviousSHA256, fmt.Sprintf("%x", sha256.Sum256([]byte(packaged.Policy())))}
			sort.Strings(history)
			payload := packaged.Policy() + "# Next official release fixture.\n"
			wire := nextPolicy(f, strings.Join(history, ","), payload)
			installed := packaged.Policy()
			if name == "old" {
				data, err := os.ReadFile("../../internal/udevpolicy/testdata/fa60-official.rules")
				if err != nil {
					t.Fatal(err)
				}
				installed = string(data)
			}
			if name == "custom" {
				installed += "# local edit\n"
			}
			if name == "unsigned-history" {
				wire = bytes.Replace(wire, []byte(history[0]), []byte(strings.Repeat("0", 64)), 1)
			}
			put(t, f.target, installed, 0644)
			if name == "custom" || name == "unsigned-history" {
				initial := snapshot(t, f.root)
				want := "error: custom policy\n"
				if name == "unsigned-history" {
					want = "error: invalid policy\n"
				}
				expect(t, f, wire, "", want, 1, "1.2.3")
				unchanged(t, f, initial)
				untouched(t, f, []byte(installed))
				return
			}
			expect(t, f, wire, "installed\n", "", 0, "1.2.3")
			data, err := os.ReadFile(f.target)
			if err != nil || string(data) != payload {
				t.Fatal("migration did not install exact future policy")
			}
			calls, err := os.ReadFile(f.counter)
			want := "control --reload-rules\ntrigger --action=change --subsystem-match=usb --parent-match=" + filepath.Join(f.root, "sys/usb/x6") + "\ntrigger --action=change --subsystem-match=hidraw --parent-match=" + filepath.Join(f.root, "sys/usb/x6") + "\n"
			if err != nil || string(calls) != want {
				t.Fatalf("calls=%q err=%v", calls, err)
			}
		})
	}
}
func TestRuntimeRecreation(t *testing.T) {
	for _, name := range []string{"directory", "lock"} {
		t.Run(name, func(t *testing.T) {
			f := setup(t)
			source := filepath.Join(f.root, "lock")
			if name == "lock" {
				source = filepath.Join(source, "helper.lock")
			}
			if err := os.Rename(source, filepath.Join(f.root, "saved-runtime")); err != nil {
				t.Fatal(err)
			}
			initial := snapshot(t, f.root)
			// Reboot recovery must work even for a rejected policy. Its only durable
			// effect is private runtime metadata, never the target or udev effects.
			expect(t, f, []byte("invalid\n"), "", "error: invalid policy\n", 1, "1.2.3")
			for path, mode := range map[string]os.FileMode{filepath.Join(f.root, "lock"): 0700, filepath.Join(f.root, "lock/helper.lock"): 0600} {
				st, err := os.Lstat(path)
				if err != nil || st.Mode().Perm() != mode || st.Mode()&os.ModeSymlink != 0 {
					t.Fatalf("unsafe recreated runtime: %s %v", path, err)
				}
			}
			after := snapshot(t, f.root)
			delete(after, filepath.Join(f.root, "lock/helper.lock"))
			if name == "directory" {
				delete(after, filepath.Join(f.root, "lock"))
			}
			if !reflect.DeepEqual(after, initial) {
				t.Fatal("rejection changed more than private runtime metadata")
			}
			untouched(t, f, nil)
			put(t, f.target, packaged.Policy(), 0644)
			initial = snapshot(t, f.root)
			expect(t, f, f.wire, "current\n", "", 0, "1.2.3")
			unchanged(t, f, initial)
			untouched(t, f, []byte(packaged.Policy()))
		})
	}
}
func TestConcurrentRuntimeCreation(t *testing.T) {
	f := setup(t)
	if err := os.Rename(filepath.Join(f.root, "lock"), filepath.Join(f.root, "saved-runtime")); err != nil {
		t.Fatal(err)
	}
	var commands []*exec.Cmd
	var outputs, errorsOut [2]bytes.Buffer
	// Both unprivileged copied scripts remain owned by this foreground test;
	// Wait always joins every process, even when the other invocation fails.
	for i := 0; i < 2; i++ {
		cmd := exec.Command("/bin/sh", f.script, "1.2.3")
		cmd.Stdin = strings.NewReader("invalid\n")
		cmd.Stdout = &outputs[i]
		cmd.Stderr = &errorsOut[i]
		if err := cmd.Start(); err != nil {
			for _, started := range commands {
				_ = started.Wait()
			}
			t.Fatal(err)
		}
		commands = append(commands, cmd)
	}
	for i, cmd := range commands {
		err := cmd.Wait()
		exit, ok := err.(*exec.ExitError)
		if !ok || exit.ExitCode() != 1 || outputs[i].String() != "" || errorsOut[i].String() != "error: invalid policy\n" {
			t.Errorf("concurrent helper stdout=%q stderr=%q err=%v", outputs[i].String(), errorsOut[i].String(), err)
		}
	}
	untouched(t, f, nil)
	for path, mode := range map[string]os.FileMode{filepath.Join(f.root, "lock"): 0700, filepath.Join(f.root, "lock/helper.lock"): 0600} {
		st, err := os.Lstat(path)
		if err != nil || st.Mode().Perm() != mode || st.Mode()&os.ModeSymlink != 0 {
			t.Fatalf("unsafe recreated runtime: %s %v", path, err)
		}
	}
}

func TestRuntimeCreationValidatesWinner(t *testing.T) {
	for _, name := range []string{"safe", "unsafe"} {
		t.Run(name, func(t *testing.T) {
			f := setup(t)
			dir := filepath.Join(f.root, "lock")
			if err := os.Rename(dir, filepath.Join(f.root, "saved-runtime")); err != nil {
				t.Fatal(err)
			}
			// A fixed mkdir-process seam deterministically simulates another helper
			// winning creation, including a nonzero mkdir status for our invocation.
			stub := filepath.Join(f.root, "mkdir")
			mode := "0700"
			if name == "unsafe" {
				mode = "0755"
			}
			put(t, stub, "#!/bin/sh\n/usr/bin/mkdir -m "+mode+" -- '"+dir+"'\nexit 1\n", 0700)
			raw, err := os.ReadFile(f.script)
			if err != nil {
				t.Fatal(err)
			}
			put(t, f.script, strings.ReplaceAll(string(raw), "/usr/bin/mkdir", stub), 0700)
			want := "error: invalid policy\n"
			if name == "unsafe" {
				want = "error: unsafe lock\n"
			}
			expect(t, f, []byte("invalid\n"), "", want, 1, "1.2.3")
			if name == "safe" {
				untouched(t, f, nil)
			} else {
				if _, err := os.Lstat(filepath.Join(dir, "helper.lock")); !os.IsNotExist(err) {
					t.Fatal("unsafe winning directory used")
				}
				if _, err := os.Stat(f.target); !os.IsNotExist(err) {
					t.Fatal("target created")
				}
			}
		})
	}
}

func TestUnsafeRuntimeObjects(t *testing.T) {
	for _, name := range []string{"directory-symlink", "directory-file", "lock-directory", "lock-owner", "directory-owner"} {
		t.Run(name, func(t *testing.T) {
			f := setup(t)
			dir := filepath.Join(f.root, "lock")
			lock := filepath.Join(dir, "helper.lock")
			if name == "directory-symlink" || name == "directory-file" {
				saved := filepath.Join(f.root, "saved-runtime")
				if err := os.Rename(dir, saved); err != nil {
					t.Fatal(err)
				}
				if name == "directory-symlink" {
					if err := os.Symlink(saved, dir); err != nil {
						t.Fatal(err)
					}
				} else {
					put(t, dir, "unsafe", 0600)
				}
			}
			if name == "lock-directory" {
				if err := os.Rename(lock, filepath.Join(f.root, "saved-lock")); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(lock, 0700); err != nil {
					t.Fatal(err)
				}
			}
			if name == "lock-owner" || name == "directory-owner" {
				bad := lock
				if name == "directory-owner" {
					bad = dir
				}
				stat := filepath.Join(f.root, "stat")
				raw, err := os.ReadFile(stat)
				if err != nil {
					t.Fatal(err)
				}
				put(t, stat, "#!/bin/sh\ncase \"$*\" in *'"+bad+"') printf '1000:700\\n'; exit 0;; esac\n"+strings.TrimPrefix(string(raw), "#!/bin/sh\n"), 0700)
			}
			initial := snapshot(t, f.root)
			expect(t, f, f.wire, "", "error: unsafe lock\n", 1, "1.2.3")
			unchanged(t, f, initial)
			if _, err := os.Stat(f.target); !os.IsNotExist(err) {
				t.Fatal("target changed")
			}
			if _, err := os.Stat(f.counter); !os.IsNotExist(err) {
				t.Fatal("udev invoked")
			}
		})
	}
}
