package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
