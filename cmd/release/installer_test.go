package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestInstallerEnforcesSignedCanonicalReleasePolicy(t *testing.T) {
	installerBytes, err := os.ReadFile(filepath.Join("..", "..", "install.sh"))
	if err != nil {
		t.Fatal(err)
	}
	installer := string(installerBytes)

	for _, required := range []string{
		"#!/bin/sh",
		"https://api.github.com/repos/blak0p/attack-shark-linux/releases",
		`^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)-rc\.(0|[1-9][0-9]*)$`,
		"attack-shark-linux-x86_64.AppImage",
		"update-manifest.json",
		"DOWNLOAD_ROOT=\"https://github.com/blak0p/attack-shark-linux/releases/download\"",
		"openssl pkeyutl -verify -pubin -rawin",
		"sha256sum",
		"install_dir=\"$account_home/.local/share/attack-shark-x6\"",
		"install_path=\"$install_dir/$APPIMAGE_NAME\"",
		"Exec=$install_path",
		"--install-udev",
		"udevadm control --reload-rules",
		"udevadm trigger",
		"__ATTACK_SHARK_RELEASE_PUBLIC_KEY__",
	} {
		if !strings.Contains(installer, required) {
			t.Errorf("installer must contain %q", required)
		}
	}
	for _, forbidden := range []string{"git clone", "git checkout"} {
		if strings.Contains(installer, forbidden) {
			t.Errorf("installer must not contain %q", forbidden)
		}
	}
	udevFlow := strings.Index(installer, `if [ "$install_udev" = true ]; then`)
	udevCommand := strings.LastIndex(installer, "sudo udevadm control --reload-rules")
	if udevFlow == -1 || udevCommand == -1 || udevCommand < udevFlow {
		t.Error("udev commands must occur only in the explicit opt-in flow")
	}
}

func TestInstallerCorrectsIndependentREL4BDefects(t *testing.T) {
	installerBytes, err := os.ReadFile(filepath.Join("..", "..", "install.sh"))
	if err != nil {
		t.Fatal(err)
	}
	installer := string(installerBytes)

	for _, required := range []string{
		`account_home=$(getent passwd "$(id -u)" | awk -F: 'NR == 1 { print $6 }')`,
		`case "$account_home" in`,
		`install_dir="$account_home/.local/share/attack-shark-x6"`,
		`desktop_dir="$account_home/.local/share/applications"`,
		`manual_udev_fallback "$udev_url"`,
		`curl --fail --location --silent --show-error '$rule_url' | sudo install -Dm0644 /dev/stdin /etc/udev/rules.d/60-attack-shark-x6-hidraw.rules`,
		`API_URL="https://api.github.com/repos/blak0p/attack-shark-linux/releases"`,
		`releases_page=1`,
		`?per_page=100&page=$releases_page`,
		`[ "$release_count" -eq 0 ] && break`,
		`LC_ALL=C`,
		`decimal_is_longer()`,
	} {
		if !strings.Contains(installer, required) {
			t.Errorf("installer REL-4B correction must contain %q", required)
		}
	}
	for _, forbidden := range []string{
		`$HOME/.local/share`,
		`${XDG_DATA_HOME:-$HOME/.local/share}`,
		`[ ${#left_number} -gt ${#right_number} ]`,
	} {
		if strings.Contains(installer, forbidden) {
			t.Errorf("installer REL-4B correction must not contain %q", forbidden)
		}
	}
	if calls := strings.Count(installer, `manual_udev_fallback "$udev_url"`); calls != 3 {
		t.Errorf("every noninteractive, declined, and failed udev path needs a manual fallback; got %d", calls)
	}
}

func TestInstallerSelectsNewestSignedStableUnderPOSIXShell(t *testing.T) {
	output, home, err := runInstallerFixtureForChannel(t, false, false, "v1.2.0-rc.19", "v1.2.0")
	if err != nil {
		t.Fatalf("stable installer failed unexpectedly: %v; output: %s", err, output)
	}
	if !strings.Contains(output, "signed stable v1.2.0") {
		t.Fatalf("installer did not select the newest signed stable: %s", output)
	}
	if _, err := os.Stat(filepath.Join(home, ".local", "share", "attack-shark-x6", "attack-shark-linux-x86_64.AppImage")); err != nil {
		t.Fatalf("installer did not atomically install the selected stable artifact: %v", err)
	}
}

func TestInstallerSelectsNewestStableAcrossPages(t *testing.T) {
	output, _, err := runInstallerFixtureForChannel(t, false, false, "v1.2.9", "v1.10.0")
	if err != nil || !strings.Contains(output, "signed stable v1.10.0") {
		t.Fatalf("expected greatest stable across pages: %v; output: %s", err, output)
	}
}

func TestInstallerFailsWithoutStableInsteadOfSelectingRC(t *testing.T) {
	output, _, err := runInstallerFixtureForChannel(t, false, false, "v1.2.0-rc.19", "v1.2.0-rc.20")
	if err == nil {
		t.Fatalf("stable installer selected an RC when no stable exists: %s", output)
	}
	if !strings.Contains(output, "No valid signed GitHub stable release is available.") {
		t.Fatalf("stable installer did not explain stable absence: %s", output)
	}
}

func TestInstallerBetaDoesNotSelectStable(t *testing.T) {
	output, _, err := runInstallerFixtureForChannel(t, false, true, "v1.2.0", "v1.2.1")
	if err == nil {
		t.Fatalf("beta installer selected a stable release: %s", output)
	}
	if !strings.Contains(output, "No valid signed GitHub RC release is available.") {
		t.Fatalf("beta installer did not explain RC absence: %s", output)
	}
}

func TestInstallerSelectsGreatestRCAndStopsAtExhaustionUnderPOSIXShell(t *testing.T) {
	output, home, err := runInstallerFixture(t, false, "v1.2.0-rc.11", "v1.2.0-rc.19")
	if err != nil {
		t.Fatalf("installer failed unexpectedly: %v; output: %s", err, output)
	}
	if !strings.Contains(output, "signed RC v1.2.0-rc.19") {
		t.Fatalf("installer did not select the greater equal-length RC under /bin/sh: %s", output)
	}
	if _, err := os.Stat(filepath.Join(home, ".local", "share", "attack-shark-x6", "attack-shark-linux-x86_64.AppImage")); err != nil {
		t.Fatalf("installer did not atomically install the selected artifact: %v", err)
	}
}

func TestInstallerSelectsGloballyNewestArbitraryPrecisionRCUnderPOSIXShell(t *testing.T) {
	for _, fixture := range []struct {
		name     string
		first    string
		second   string
		selected string
	}{
		{
			name:     "unequal length numeric identifiers",
			first:    "v1.2.0-rc.999",
			second:   "v1.2.0-rc.1000",
			selected: "v1.2.0-rc.1000",
		},
		{
			name:     "decimal identifiers beyond shell integer precision",
			first:    "v1.2.0-rc.18446744073709551616",
			second:   "v1.2.0-rc.18446744073709551617",
			selected: "v1.2.0-rc.18446744073709551617",
		},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			output, _, err := runInstallerFixture(t, false, fixture.first, fixture.second)
			if err != nil {
				t.Fatalf("installer failed unexpectedly: %v; output: %s", err, output)
			}
			if !strings.Contains(output, "signed RC "+fixture.selected) {
				t.Fatalf("installer did not select the globally newest arbitrary-precision RC %s: %s", fixture.selected, output)
			}
		})
	}
}

func TestInstallerRejectsRepeatedNonemptyReleasePage(t *testing.T) {
	output, _, err := runInstallerFixture(t, true, "v1.2.0-rc.11", "v1.2.0-rc.19")
	if err == nil {
		t.Fatalf("installer accepted a repeated nonempty release page: %s", output)
	}
	if !strings.Contains(output, "release discovery did not terminate") {
		t.Fatalf("installer did not fail closed for a repeated nonempty release page: %s", output)
	}
}

func runInstallerFixture(t *testing.T, repeatPages bool, firstTag, secondTag string) (string, string, error) {
	t.Helper()
	return runInstallerFixtureForChannel(t, repeatPages, true, firstTag, secondTag)
}

func runInstallerFixtureForChannel(t *testing.T, repeatPages, beta bool, firstTag, secondTag string) (string, string, error) {
	return runInstallerFixtureWithKey(t, repeatPages, beta, true, firstTag, secondTag)
}

func TestInstallerRenderedKeyDoesNotRejectItself(t *testing.T) {
	for _, beta := range []bool{false, true} {
		t.Run(map[bool]string{false: "stable", true: "RC"}[beta], func(t *testing.T) {
			tag := "v1.2.0"
			if beta {
				tag = "v1.2.0-rc.2"
			}
			older := "v1.1.0"
			if beta {
				older = "v1.2.0-rc.1"
			}
			output, _, err := runInstallerFixtureWithKey(t, false, beta, true, older, tag)
			if err != nil || !strings.Contains(output, "signed "+map[bool]string{false: "stable", true: "RC"}[beta]+" "+tag) {
				t.Fatalf("globally rendered installer rejected signed release: %v; output: %s", err, output)
			}
		})
	}
}

func TestInstallerRejectsUnrenderedKey(t *testing.T) {
	output, _, err := runInstallerFixtureWithKey(t, false, true, false, "v1.2.0-rc.2", "v1.2.0-rc.2")
	if err == nil || !strings.Contains(output, "installer public key was not injected") {
		t.Fatalf("unrendered installer must fail before release discovery: %v; output: %s", err, output)
	}
}

func runInstallerFixtureWithKey(t *testing.T, repeatPages, beta, renderKey bool, firstTag, secondTag string) (string, string, error) {
	t.Helper()
	temp := t.TempDir()
	home := filepath.Join(temp, "account-home")
	bin := filepath.Join(temp, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	installerBytes, err := os.ReadFile(filepath.Join("..", "..", "install.sh"))
	if err != nil {
		t.Fatal(err)
	}
	installer := string(installerBytes)
	if renderKey {
		installer = strings.ReplaceAll(installer, "__ATTACK_SHARK_RELEASE_PUBLIC_KEY__", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=")
	}
	installerPath := filepath.Join(temp, "install.sh")
	if err := os.WriteFile(installerPath, []byte(installer), 0o755); err != nil {
		t.Fatal(err)
	}
	writeInstallerStub(t, bin, "id", "#!/bin/sh\nprintf '1000\\n'\n")
	writeInstallerStub(t, bin, "getent", "#!/bin/sh\nprintf 'tester:x:1000:1000::%s:/bin/sh\\n' \"$INSTALLER_TEST_HOME\"\n")
	writeInstallerStub(t, bin, "openssl", "#!/bin/sh\nexit 0\n")
	writeInstallerStub(t, bin, "curl", `#!/bin/sh
output=
expect_output=false
for argument do
  if [ "$expect_output" = true ]; then output=$argument; expect_output=false; continue; fi
  case "$argument" in
    -o) expect_output=true ;;
    http*) url=$argument ;;
  esac
done
case "$url" in
  *'&page=1') value=$INSTALLER_TEST_FIRST_TAG ;;
  *'&page=2') if [ "$INSTALLER_TEST_REPEAT" = true ]; then value=$INSTALLER_TEST_FIRST_TAG; else value=$INSTALLER_TEST_SECOND_TAG; fi ;;
  *'&page='*) if [ "$INSTALLER_TEST_REPEAT" = true ]; then value=$INSTALLER_TEST_FIRST_TAG; else value='[]'; fi ;;
  */update-manifest.json) value=$url ;;
  */attack-shark-linux-x86_64.AppImage) value='fixture-appimage' ;;
esac
if [ -n "$output" ]; then printf '%s' "$value" > "$output"; else printf '%s' "$value"; fi
`)
	writeInstallerStub(t, bin, "jq", `#!/bin/sh
for argument do last=$argument; done
if [ -f "$last" ]; then input=$(cat "$last"); else input=$(cat); fi
case "$*" in
  *'type == '*) case "$input" in '[]') printf '0\n' ;; *) printf '1\n' ;; esac ;;
  *"length == 1") printf 'true\n' ;;
  *".[] | select"*)
    case "$*" in
      *'--arg prerelease true'*) expected=true ;;
      *'--arg prerelease false'*) expected=false ;;
      *) exit 1 ;;
    esac
    case "$input:$expected" in
      *-rc.*:true) printf '%s\n' "$input" ;;
      *-rc.*:false) ;;
      v[0-9]*.[0-9]*.[0-9]*:false) printf '%s\n' "$input" ;;
    esac ;;
  *".version | strings"*) version=${input#*/download/v}; printf '%s\n' "${version%/update-manifest.json}" ;;
  *".url | strings"*) printf '%s\n' "$input" | sed 's/update-manifest.json/attack-shark-linux-x86_64.AppImage/' ;;
  *".sha256 | strings"*) printf '09c1a2f3396015e2a1d6060fd66ed7c9744fdb5b101679121943b9e0fd8424c2\n' ;;
  *".signature | strings"*) printf 'AAAA\n' ;;
esac
`)
	context, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	arguments := []string{installerPath}
	if beta {
		arguments = append(arguments, "--beta")
	}
	command := exec.CommandContext(context, "/bin/sh", arguments...)
	command.Env = append(os.Environ(), "PATH="+bin+":"+os.Getenv("PATH"), "INSTALLER_TEST_HOME="+home, "INSTALLER_TEST_REPEAT="+map[bool]string{true: "true", false: "false"}[repeatPages], "INSTALLER_TEST_FIRST_TAG="+firstTag, "INSTALLER_TEST_SECOND_TAG="+secondTag)
	output, err := command.CombinedOutput()
	if context.Err() != nil {
		t.Fatalf("installer did not terminate under /bin/sh: %v; output: %s", context.Err(), output)
	}
	return string(output), home, err
}

func writeInstallerStub(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestInstallerDefaultsToStableAndAcceptsExplicitBetaFlag(t *testing.T) {
	installerBytes, err := os.ReadFile(filepath.Join("..", "..", "install.sh"))
	if err != nil {
		t.Fatal(err)
	}
	installer := string(installerBytes)
	for _, required := range []string{
		`[ "${1:-}" = "--beta" ]`,
		`[ "$1" = "--install-udev" ]`,
		"Usage: install.sh [--beta] [--install-udev]",
		"No valid signed GitHub $channel release is available.",
	} {
		if !strings.Contains(installer, required) {
			t.Errorf("installer argument contract must contain %q", required)
		}
	}
}
