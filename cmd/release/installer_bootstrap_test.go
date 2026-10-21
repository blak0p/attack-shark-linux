package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/blak0p/attack-shark-linux/internal/update"
	packaged "github.com/blak0p/attack-shark-linux/packaging/udev"
)

// Admission only: authentication assets must never be consumed by normal install.
func TestInstallerAssetSetAdmission(t *testing.T) {
	legacy := []string{"attack-shark-linux-x86_64.AppImage", "attack-shark-linux-x86_64.AppImage.sha256", "update-manifest.json", "60-attack-shark-x6-hidraw.rules", "install.sh"}
	complete := append(append([]string{}, legacy...), "attack-shark-linux-udev-helper", "udev-helper-manifest.txt", "udev-policy.txt")
	for _, tc := range []struct {
		name             string
		names            []string
		valid            bool
		badURL           bool
		duplicateRelease bool
	}{
		{name: "legacy", names: legacy, valid: true},
		{name: "complete", names: complete, valid: true},
		{name: "incomplete", names: complete[:7]},
		{name: "duplicate", names: append(append([]string{}, complete[:7]...), "udev-helper-manifest.txt")},
		{name: "unknown", names: append(append([]string{}, complete[:7]...), "other-policy.txt")},
		{name: "extra", names: append(append([]string{}, complete...), "extra.txt")},
		{name: "missing-base", names: complete[1:]},
		{name: "wrong-url", names: complete, badURL: true},
		{name: "ambiguous-release", names: complete, duplicateRelease: true},
	} {
		for _, route := range []string{"stable", "beta", "manual-udev"} {
			t.Run(tc.name+"/"+route, func(t *testing.T) {
				temp := t.TempDir()
				home, bin := filepath.Join(temp, "home"), filepath.Join(temp, "bin")
				if err := os.MkdirAll(home, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.MkdirAll(bin, 0755); err != nil {
					t.Fatal(err)
				}
				sentinel := filepath.Join(home, "existing")
				if err := os.WriteFile(sentinel, []byte("unchanged"), 0644); err != nil {
					t.Fatal(err)
				}
				script, err := os.ReadFile("../../install.sh")
				if err != nil {
					t.Fatal(err)
				}
				installer := filepath.Join(temp, "install.sh")
				if err := os.WriteFile(installer, bytes.ReplaceAll(script, []byte("__ATTACK_SHARK_RELEASE_PUBLIC_KEY__"), []byte("AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=")), 0755); err != nil {
					t.Fatal(err)
				}
				tag, channel := "v1.2.3", "stable"
				args := []string{installer}
				if route == "beta" {
					tag, channel = "v1.2.3-rc.1", "RC"
					args = append(args, "--beta")
				}
				if route == "manual-udev" {
					args = append(args, "--install-udev")
				}
				root := "https://github.com/blak0p/attack-shark-linux/releases/download/" + tag
				assets := []map[string]string{}
				for _, name := range tc.names {
					assets = append(assets, map[string]string{"name": name, "browser_download_url": root + "/" + name})
				}
				if tc.badURL {
					assets[len(assets)-1]["browser_download_url"] = "https://example.invalid/policy"
				}
				release := map[string]any{"tag_name": tag, "prerelease": route == "beta", "draft": false, "assets": assets}
				releases := []any{release}
				if tc.duplicateRelease {
					releases = append(releases, release)
				}
				releaseBytes, err := json.Marshal(releases)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(temp, "releases"), releaseBytes, 0600); err != nil {
					t.Fatal(err)
				}
				app := []byte("#!/bin/sh\nmkdir -p squashfs-root\nprintf '<svg>fixture</svg>\\n' > squashfs-root/.DirIcon\n")
				if err := os.WriteFile(filepath.Join(temp, "app"), app, 0755); err != nil {
					t.Fatal(err)
				}
				sum := sha256.Sum256(app)
				manifest, err := json.Marshal(map[string]string{"version": strings.TrimPrefix(tag, "v"), "url": root + "/" + legacy[0], "sha256": hex.EncodeToString(sum[:]), "signature": "AAAA"})
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(temp, "manifest"), manifest, 0600); err != nil {
					t.Fatal(err)
				}
				writeInstallerStub(t, bin, "id", "#!/bin/sh\nprintf '1000\\n'\n")
				writeInstallerStub(t, bin, "getent", "#!/bin/sh\nprintf 'fixture:x:1000:1000::%s:/bin/sh\\n' \"$FIXTURE/home\"\n")
				writeInstallerStub(t, bin, "openssl", "#!/bin/sh\nprintf 'verify\\n' >> \"$FIXTURE/verifications\"\nexit 0\n")
				for _, name := range []string{"sudo", "pkexec", "udevadm", "attack-shark-linux-udev-helper"} {
					writeInstallerStub(t, bin, name, "#!/bin/sh\nprintf 'unexpected privileged call\\n' >> \"$FIXTURE/privileged\"\nexit 99\n")
				}
				writeInstallerStub(t, bin, "curl", `#!/bin/sh
output=
for arg do
 if [ "$output_next" = yes ]; then output=$arg; output_next=; continue; fi
 case "$arg" in -o) output_next=yes ;; http*) url=$arg ;; esac
done
case "$url" in
 *'&page=1') cat "$FIXTURE/releases"; exit 0 ;;
 *'&page='*) printf '[]'; exit 0 ;;
 */update-manifest.json) printf 'manifest\n' >> "$FIXTURE/downloads"; cat "$FIXTURE/manifest" > "$output" ;;
 */attack-shark-linux-x86_64.AppImage) printf 'app\n' >> "$FIXTURE/downloads"; cat "$FIXTURE/app" > "$output" ;;
 *) printf 'unexpected download\n' >> "$FIXTURE/downloads"; exit 99 ;;
esac
`)
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				cmd := exec.CommandContext(ctx, "/bin/sh", args...)
				cmd.Env = append(os.Environ(), "PATH="+bin+":"+os.Getenv("PATH"), "FIXTURE="+temp, "TMPDIR="+temp)
				cmd.Stdin = strings.NewReader("")
				var stdout, stderr bytes.Buffer
				cmd.Stdout, cmd.Stderr = &stdout, &stderr
				err = cmd.Run()
				if ctx.Err() != nil {
					t.Fatal(ctx.Err())
				}
				wantOut, wantErr, wantDownloads, wantVerify := "", "install.sh: No valid signed GitHub "+channel+" release is available.\n", "", ""
				installPath := filepath.Join(home, ".local/share/attack-shark-x6", legacy[0])
				if tc.valid {
					if err != nil {
						t.Fatalf("exit: %v; stdout=%q stderr=%q", err, stdout.String(), stderr.String())
					}
					wantOut = fmt.Sprintf("Installed %s from signed %s %s\n", installPath, channel, tag)
					wantErr, wantDownloads, wantVerify = "", "manifest\napp\n", "verify\n"
					if route == "manual-udev" {
						wantErr = "Skipping udev installation because input is noninteractive.\nInstall the canonical udev rule manually, then reload and trigger rules before reconnecting the dongle:\ncurl --fail --location --silent --show-error '" + root + "/" + legacy[3] + "' | sudo install -Dm0644 /dev/stdin /etc/udev/rules.d/60-attack-shark-x6-hidraw.rules\nsudo udevadm control --reload-rules\nsudo udevadm trigger\n"
					}
					got, e := os.ReadFile(installPath)
					if e != nil || !bytes.Equal(got, app) {
						t.Fatalf("installed app: %q, %v", got, e)
					}
					info, e := os.Stat(installPath)
					if e != nil || info.Mode().Perm() != 0755 {
						t.Fatalf("app permissions: %v %v", info, e)
					}
				} else {
					exit, ok := err.(*exec.ExitError)
					if !ok || exit.ExitCode() != 1 {
						t.Fatalf("exit=%v, want 1", err)
					}
					entries, e := os.ReadDir(home)
					if e != nil || len(entries) != 1 || entries[0].Name() != "existing" {
						t.Fatalf("rejected release changed home: %v %v", entries, e)
					}
				}
				if stdout.String() != wantOut || stderr.String() != wantErr {
					t.Errorf("stdout=%q stderr=%q; want %q / %q", stdout.String(), stderr.String(), wantOut, wantErr)
				}
				for file, want := range map[string]string{"downloads": wantDownloads, "verifications": wantVerify, "privileged": "", "home/existing": "unchanged"} {
					got, e := os.ReadFile(filepath.Join(temp, file))
					if e != nil && !os.IsNotExist(e) {
						t.Fatal(e)
					}
					if string(got) != want {
						t.Errorf("%s=%q, want %q", file, got, want)
					}
				}
				// No helper/rule payload is requested and no installed authentication file appears.
				for _, name := range complete[5:] {
					if _, e := os.Stat(filepath.Join(home, ".local/share/attack-shark-x6", name)); !os.IsNotExist(e) {
						t.Errorf("unexpected installed auth asset %s: %v", name, e)
					}
				}
			})
		}
	}
}

const setupApproval = "USB setup requires administrator authorization; trust this installer and its embedded release key.\n"
const setupPreFailure = "usb-setup: invalid authenticated input\ninstall.sh: USB setup validation failed before authorization.\n"
const setupRootFailure = "usb-setup: invalid authenticated input; helper unchanged; policy not applied\ninstall.sh: USB setup transaction failed; inspect status above (no automatic retry).\n"

type usbSetupFixture struct {
	root, installer, helper, policy, bin string
	assets                               map[string][]byte
}

func writeUSBFixture(t *testing.T, path string, data []byte, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, data, mode); err != nil {
		t.Fatal(err)
	}
}

func newUSBSetupFixture(t *testing.T, tag string) usbSetupFixture {
	t.Helper()
	f := usbSetupFixture{root: t.TempDir()}
	f.bin = filepath.Join(f.root, "bin")
	f.helper = filepath.Join(f.root, "usr/local/libexec/attack-shark-linux-udev-helper")
	f.policy = filepath.Join(f.root, "rules/policy")
	f.installer = filepath.Join(f.root, "install.sh")
	for _, dir := range []string{"bin", "home", "run", "rules", "sys", "sys/usb", "sys/usb/x6"} {
		if err := os.MkdirAll(filepath.Join(f.root, dir), 0700); err != nil {
			t.Fatal(err)
		}
	}
	writeUSBFixture(t, filepath.Join(f.root, "sys/usb/x6/idVendor"), []byte("1d57\n"), 0600)
	writeUSBFixture(t, filepath.Join(f.root, "sys/usb/x6/idProduct"), []byte("fa60\n"), 0600)
	writeUSBFixture(t, filepath.Join(f.root, "home/appimage"), []byte("existing AppImage"), 0755)
	writeUSBFixture(t, filepath.Join(f.root, "id"), []byte("#!/bin/sh\nprintf '0\\n'\n"), 0700)
	stat := "#!/bin/sh\n/usr/bin/stat \"$@\" | /usr/bin/sed 's/^" + fmt.Sprint(os.Getuid()) + ":/0:/'\n"
	writeUSBFixture(t, filepath.Join(f.root, "stat"), []byte(stat), 0700)
	writeUSBFixture(t, filepath.Join(f.root, "udevadm"), []byte("#!/bin/sh\nprintf '%s\\n' \"$*\" >> '"+f.root+"/udev-calls'\n"), 0700)
	template, err := os.ReadFile("../../packaging/udev-helper/helper.sh")
	if err != nil {
		t.Fatal(err)
	}
	// Instrument only the signed fixture template, before signing final bytes.
	template = []byte(strings.NewReplacer(
		"/usr/local/libexec/attack-shark-linux-udev-helper", f.helper,
		"/etc/udev/rules.d/60-attack-shark-x6-hidraw.rules", f.policy,
		"/run/attack-shark-linux", f.root+"/run/attack-shark-linux",
		"TRUSTED_PARENTS='/ /etc /etc/udev /etc/udev/rules.d /run /usr /usr/local /usr/local/libexec'", "TRUSTED_PARENTS='"+f.root+" "+f.root+"/rules "+f.root+"/run "+f.root+"/usr "+f.root+"/usr/local "+f.root+"/usr/local/libexec'",
		"/usr/bin/id", f.root+"/id", "/usr/bin/stat", f.root+"/stat", "/usr/bin/udevadm", f.root+"/udevadm",
		"/sys/bus/usb/devices", f.root+"/sys/usb", "/sys/devices", f.root+"/sys",
	).Replace(string(template)))
	seed := bytes.Repeat([]byte{42}, 32) // Public deterministic test key; never a release key.
	f.assets, err = BuildAuthArtifacts(tag, template, base64.StdEncoding.EncodeToString(seed))
	if err != nil {
		t.Fatal(err)
	}
	key := ed25519.NewKeyFromSeed(seed)
	public := base64.StdEncoding.EncodeToString(key.Public().(ed25519.PublicKey))
	script, err := os.ReadFile("../../install.sh")
	if err != nil {
		t.Fatal(err)
	}
	// Python UID/namespace seams exist only in this copied installer, never in production.
	script = []byte(strings.NewReplacer(
		"__ATTACK_SHARK_RELEASE_PUBLIC_KEY__", public,
		"ROOT_UID = 0", "ROOT_UID = "+fmt.Sprint(os.Getuid()),
		"RUNTIME = '/run'", "RUNTIME = '"+f.root+"/run'",
		"ROOT_PARENTS = ('/', '/run')", "ROOT_PARENTS = ('"+f.root+"', '"+f.root+"/run')",
		"INSTALL_PARENTS = ('/', '/usr', '/usr/local', '/usr/local/libexec')", "INSTALL_PARENTS = ('"+f.root+"', '"+f.root+"/usr', '"+f.root+"/usr/local', '"+f.root+"/usr/local/libexec')",
		"/usr/local/libexec/attack-shark-linux-udev-helper", f.helper,
		"/usr/bin/sudo", f.bin+"/sudo",
	).Replace(string(script)))
	writeUSBFixture(t, f.installer, script, 0700)
	for name, data := range f.assets {
		writeUSBFixture(t, filepath.Join(f.root, name), data, 0600)
	}
	rootURL := "https://github.com/blak0p/attack-shark-linux/releases/download/" + tag
	names := []string{"attack-shark-linux-x86_64.AppImage", "attack-shark-linux-x86_64.AppImage.sha256", "update-manifest.json", "60-attack-shark-x6-hidraw.rules", "install.sh", HelperAsset, HelperManifestAsset, PolicyAsset}
	assets := []map[string]string{}
	for _, name := range names {
		assets = append(assets, map[string]string{"name": name, "browser_download_url": rootURL + "/" + name})
	}
	releases, _ := json.Marshal([]any{map[string]any{"tag_name": tag, "prerelease": strings.Contains(tag, "-rc."), "draft": false, "assets": assets}})
	writeUSBFixture(t, filepath.Join(f.root, "releases"), releases, 0600)
	manifest := update.Manifest{Version: strings.TrimPrefix(tag, "v"), URL: rootURL + "/attack-shark-linux-x86_64.AppImage", SHA256: strings.Repeat("a", 64)}
	// The legacy manifest signature is real too, independently of auth envelopes.
	manifest.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(key, []byte("version="+manifest.Version+"\nurl="+manifest.URL+"\nsha256="+manifest.SHA256+"\n")))
	wire, _ := json.Marshal(manifest)
	writeUSBFixture(t, filepath.Join(f.root, "manifest"), wire, 0600)
	writeInstallerStub(t, f.bin, "id", "#!/bin/sh\nprintf '1000\\n'\n")
	writeInstallerStub(t, f.bin, "getent", "#!/bin/sh\nprintf 'fixture:x:1000:1000::%s:/bin/sh\\n' \"$FIXTURE/home\"\n")
	writeInstallerStub(t, f.bin, "sudo", `#!/bin/sh
printf 'authorize\n' >> "$FIXTURE/authorizations"
if [ -f "$FIXTURE/deny" ]; then printf 'fixture: authorization denied\n' >&2; exit 1; fi
if [ -f "$FIXTURE/mutate" ]; then
 for arg do source=$previous; previous=$arg; done
 case "$(cat "$FIXTURE/mutate")" in
 tamper) printf 'tamper\n' >> "$source/attack-shark-linux-udev-helper" ;;
 symlink) mv "$source/attack-shark-linux-udev-helper" "$source/saved"; ln -s "$source/saved" "$source/attack-shark-linux-udev-helper" ;;
 fifo) mv "$source/attack-shark-linux-udev-helper" "$source/saved"; mkfifo "$source/attack-shark-linux-udev-helper" ;;
 policy) printf 'tamper\n' >> "$source/udev-policy.txt" ;;
 esac
fi
[ "$1" = -- ] && shift
exec "$@"
`)
	writeInstallerStub(t, f.bin, "curl", `#!/bin/sh
output=
for arg do
 if [ "$next" = yes ]; then output=$arg; next=; continue; fi
 case "$arg" in -o) next=yes ;; http*) url=$arg ;; esac
done
case "$url" in
 *'&page=1') cat "$FIXTURE/releases"; exit 0 ;;
 *'&page='*) printf '[]'; exit 0 ;;
 */update-manifest.json) source=manifest ;;
 */attack-shark-linux-udev-helper) source=attack-shark-linux-udev-helper ;;
 */udev-helper-manifest.txt) source=udev-helper-manifest.txt ;;
 */udev-policy.txt) source=udev-policy.txt ;;
 *) printf 'unexpected\n' >> "$FIXTURE/downloads"; exit 99 ;;
esac
printf '%s\n' "$source" >> "$FIXTURE/downloads"
cat "$FIXTURE/$source" > "$output"
`)
	return f
}

func runUSBSetup(t *testing.T, f usbSetupFixture, args ...string) (string, string, int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "/bin/sh", append([]string{f.installer}, args...)...)
	cmd.Env = append(os.Environ(), "FIXTURE="+f.root, "PATH="+f.bin+":"+os.Getenv("PATH"), "TMPDIR="+f.root)
	var out, errout bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errout
	err := cmd.Run()
	if ctx.Err() != nil {
		t.Fatal(ctx.Err())
	}
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

func TestInstallerUSBSetup(t *testing.T) {
	for _, name := range []string{"valid", "beta", "existing-private-parents", "denied", "missing-python", "old-release", "wrong-key", "wrong-version", "wrong-digest", "policy-key", "policy-digest", "domain", "extra-field", "oversize", "root-tamper", "root-symlink", "root-fifo", "root-policy", "unsafe-parent", "unsafe-helper", "custom", "rule-symlink", "reload-failure"} {
		t.Run(name, func(t *testing.T) {
			tag := "v1.2.3"
			args := []string{"--setup-usb-permissions"}
			if name == "beta" {
				tag = "v1.2.3-rc.1"
				args = []string{"--beta", "--setup-usb-permissions"}
			}
			f := newUSBSetupFixture(t, tag)
			wantOut, wantErr, wantCode := "", setupPreFailure, 1
			authorized, installed, policyInstalled := false, false, false
			switch name {
			case "valid", "beta", "existing-private-parents":
				if name == "existing-private-parents" {
					if err := os.MkdirAll(filepath.Dir(f.helper), 0700); err != nil {
						t.Fatal(err)
					}
				}
				authorized, installed, policyInstalled = true, true, true
				wantOut = "helper-installed\ninstalled\nUSB permissions setup complete.\n"
				wantErr = setupApproval
				wantCode = 0
			case "denied":
				writeUSBFixture(t, f.root+"/deny", nil, 0600)
				authorized = true
				wantErr = setupApproval + "fixture: authorization denied\ninstall.sh: USB setup transaction failed; inspect status above (no automatic retry).\n"
			case "missing-python":
				raw, _ := os.ReadFile(f.installer)
				writeUSBFixture(t, f.installer, bytes.ReplaceAll(raw, []byte("[ -x /usr/bin/python3 ]"), []byte("[ -x "+f.root+"/missing-python ]")), 0700)
				wantErr = "install.sh: USB setup requires /usr/bin/python3. Install Python 3 with your distro package manager, then rerun --setup-usb-permissions. No packages were installed.\n"
			case "old-release":
				var releases []map[string]any
				raw, _ := os.ReadFile(f.root + "/releases")
				json.Unmarshal(raw, &releases)
				releases[0]["assets"] = releases[0]["assets"].([]any)[:5]
				raw, _ = json.Marshal(releases)
				writeUSBFixture(t, f.root+"/releases", raw, 0600)
				wantErr = "install.sh: Selected release has no authenticated USB setup assets; use a release that publishes them. No AppImage changes made.\n"
			case "wrong-key", "policy-key":
				asset := HelperManifestAsset
				if name == "policy-key" {
					asset = PolicyAsset
				}
				raw := f.assets[asset]
				lines := bytes.Split(raw, []byte{'\n'})
				signed := bytes.Join(lines[:len(lines)-2], []byte{'\n'})
				signed = append(signed, '\n')
				key := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{7}, 32))
				lines[len(lines)-2] = []byte("signature=" + base64.StdEncoding.EncodeToString(ed25519.Sign(key, signed)))
				writeUSBFixture(t, f.root+"/"+asset, bytes.Join(lines, []byte{'\n'}), 0600)
			case "wrong-version":
				writeUSBFixture(t, f.root+"/"+HelperManifestAsset, bytes.ReplaceAll(f.assets[HelperManifestAsset], []byte("version=1.2.3"), []byte("version=1.2.4")), 0600)
			case "wrong-digest":
				writeUSBFixture(t, f.root+"/"+HelperAsset, append(bytes.Clone(f.assets[HelperAsset]), 'x'), 0600)
			case "policy-digest":
				raw := bytes.Clone(f.assets[PolicyAsset])
				start := bytes.Index(raw, []byte("sha256=")) + 7
				raw[start] = '0'
				writeUSBFixture(t, f.root+"/"+PolicyAsset, raw, 0600)
			case "domain":
				writeUSBFixture(t, f.root+"/"+HelperManifestAsset, bytes.ReplaceAll(f.assets[HelperManifestAsset], []byte("attack-shark-udev-helper-v1"), []byte("attack-shark-udev-policy-v1")), 0600)
			case "extra-field":
				writeUSBFixture(t, f.root+"/"+PolicyAsset, append(bytes.Clone(f.assets[PolicyAsset]), []byte("extra=x\n")...), 0600)
			case "oversize":
				writeUSBFixture(t, f.root+"/"+HelperAsset, bytes.Repeat([]byte{'x'}, 65537), 0600)
			case "root-tamper", "root-symlink", "root-fifo", "root-policy":
				mutation := strings.TrimPrefix(name, "root-")
				if mutation == "tamper" {
					mutation = "tamper"
				}
				writeUSBFixture(t, f.root+"/mutate", []byte(mutation), 0600)
				authorized = true
				wantErr = setupApproval + setupRootFailure
			case "unsafe-parent":
				os.Chmod(f.root+"/run", 0777)
				authorized = true
				wantErr = setupApproval + "usb-setup: unsafe parent; helper unchanged; policy not applied\ninstall.sh: USB setup transaction failed; inspect status above (no automatic retry).\n"
			case "unsafe-helper":
				os.MkdirAll(filepath.Dir(f.helper), 0700)
				os.Symlink(f.root+"/home/appimage", f.helper)
				authorized = true
				wantErr = setupApproval + "usb-setup: unsafe helper; helper unchanged; policy not applied\ninstall.sh: USB setup transaction failed; inspect status above (no automatic retry).\n"
			case "custom", "rule-symlink":
				authorized, installed = true, true
				wantOut = "helper-installed\n"
				reason := "custom policy"
				if name == "custom" {
					writeUSBFixture(t, f.policy, []byte("GROUP=\"users\"\n"), 0644)
				} else {
					os.Symlink(f.root+"/home/appimage", f.policy)
					reason = "unsafe target"
				}
				wantErr = setupApproval + "error: " + reason + "\nusb-setup: helper installed; policy application failed (see helper output)\ninstall.sh: USB setup transaction failed; inspect status above (no automatic retry).\n"
			case "reload-failure":
				authorized, installed, policyInstalled = true, true, true
				writeUSBFixture(t, f.root+"/udevadm", []byte("#!/bin/sh\nprintf '%s\\n' \"$*\" >> '"+f.root+"/udev-calls'\nexit 1\n"), 0700)
				wantOut = "helper-installed\ninstalled\n"
				wantErr = setupApproval + "error: installed-but-reload-failed\nusb-setup: helper installed; policy application failed (see helper output)\ninstall.sh: USB setup transaction failed; inspect status above (no automatic retry).\n"
			}
			beforePolicy, _ := os.ReadFile(f.policy)
			out, errout, code := runUSBSetup(t, f, args...)
			if out != wantOut || errout != wantErr || code != wantCode {
				t.Errorf("stdout=%q stderr=%q exit=%d; want %q / %q / %d", out, errout, code, wantOut, wantErr, wantCode)
			}
			auth, _ := os.ReadFile(f.root + "/authorizations")
			wantAuth := ""
			if authorized {
				wantAuth = "authorize\n"
			}
			if string(auth) != wantAuth {
				t.Errorf("authorizations=%q want %q", auth, wantAuth)
			}
			app, _ := os.ReadFile(f.root + "/home/appimage")
			if string(app) != "existing AppImage" {
				t.Error("AppImage changed")
			}
			home, _ := os.ReadDir(f.root + "/home")
			if len(home) != 1 {
				t.Error("setup wrote application files")
			}
			if installed {
				raw, e := os.ReadFile(f.helper)
				if e != nil || !bytes.Equal(raw, f.assets[HelperAsset]) {
					t.Errorf("installed helper mismatch: %v", e)
				}
				st, e := os.Stat(f.helper)
				if e != nil || st.Mode().Perm() != 0755 || st.Sys().(*syscall.Stat_t).Uid != uint32(os.Getuid()) {
					t.Error("helper ownership/mode")
				}
			} else if name != "unsafe-helper" {
				if _, e := os.Lstat(f.helper); !os.IsNotExist(e) {
					t.Errorf("helper changed: %v", e)
				}
			}
			policy, _ := os.ReadFile(f.policy)
			if policyInstalled {
				if string(policy) != packaged.Policy() {
					t.Error("wrong policy")
				}
			} else if !bytes.Equal(policy, beforePolicy) {
				t.Error("policy changed")
			}
			calls, _ := os.ReadFile(f.root + "/udev-calls")
			wantCalls := ""
			if policyInstalled {
				wantCalls = "control --reload-rules\n"
				if name != "reload-failure" {
					wantCalls += "trigger --action=change --subsystem-match=usb --parent-match=" + f.root + "/sys/usb/x6\ntrigger --action=change --subsystem-match=hidraw --parent-match=" + f.root + "/sys/usb/x6\n"
				}
			}
			if string(calls) != wantCalls {
				t.Errorf("udev calls=%q want %q", calls, wantCalls)
			}
			downloads, _ := os.ReadFile(f.root + "/downloads")
			expectedDownloads := "manifest\n" + HelperAsset + "\n" + HelperManifestAsset + "\n" + PolicyAsset + "\n"
			if name == "missing-python" {
				expectedDownloads = ""
			}
			if name == "old-release" {
				expectedDownloads = "manifest\n"
			}
			if string(downloads) != expectedDownloads {
				t.Errorf("downloads=%q want %q", downloads, expectedDownloads)
			}
			if name == "valid" || name == "beta" || name == "existing-private-parents" {
				// Setup internally uses umask 077 regardless of the caller's umask.
				// Only newly created public helper ancestors become traversable.
				wantMode := os.FileMode(0755)
				if name == "existing-private-parents" {
					wantMode = 0700
				}
				for _, relative := range []string{"usr", "usr/local", "usr/local/libexec"} {
					info, err := os.Lstat(filepath.Join(f.root, relative))
					if err != nil {
						t.Fatal(err)
					}
					if !info.IsDir() || info.Mode().Perm() != wantMode || info.Sys().(*syscall.Stat_t).Uid != uint32(os.Getuid()) {
						t.Errorf("%s ownership/mode: %v; want directory %04o", relative, info, wantMode)
					}
				}
				for _, relative := range []string{"run", "home"} {
					info, err := os.Stat(filepath.Join(f.root, relative))
					if err != nil || info.Mode().Perm() != 0700 {
						t.Errorf("private %s permissions changed: %v / %v", relative, info, err)
					}
				}
			}
			entries, _ := os.ReadDir(f.root + "/run")
			for _, entry := range entries {
				if strings.HasPrefix(entry.Name(), ".attack-shark-bootstrap-") {
					t.Error("root scratch remains")
				}
			}
		})
	}
}
