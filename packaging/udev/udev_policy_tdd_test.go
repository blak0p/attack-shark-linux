package udev

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const (
	policyFile = "60-attack-shark-x6-hidraw.rules"
	policyLine = `SUBSYSTEM=="hidraw", ATTRS{idVendor}=="1d57", ATTRS{idProduct}=="fa60", TAG+="uaccess", MODE="0660"`
)

func TestPackagedUdevPolicyContract(t *testing.T) {
	packageDir := packageDirectory(t)
	policyPath := filepath.Join(packageDir, policyFile)
	activeLines := activeRuleLines(t, policyPath)

	if policyFile >= "73-seat-late.rules" {
		t.Fatalf("policy filename %q must sort before 73-seat-late.rules", policyFile)
	}
	if len(activeLines) != 1 || activeLines[0] != policyLine {
		t.Fatalf("active policy lines = %q, want exactly %q", activeLines, policyLine)
	}

	policyText := readFile(t, policyPath)
	for _, forbidden := range []string{"SUBSYSTEM==\"usb\"", "OWNER="} {
		if strings.Contains(policyText, forbidden) {
			t.Errorf("policy contains forbidden token %q", forbidden)
		}
	}
	for _, obsoleteFile := range []string{
		"99-attack-shark-x6.rules",
		"99-attack-shark-x6-hidraw.rules",
	} {
		if _, err := os.Stat(filepath.Join(packageDir, obsoleteFile)); !os.IsNotExist(err) {
			t.Errorf("obsolete policy %q still exists (stat error: %v)", obsoleteFile, err)
		}
	}

	repositoryRoot := filepath.Dir(filepath.Dir(packageDir))
	documentation := readFile(t, filepath.Join(repositoryRoot, "docs", "linux-usb-prerequisites.md"))
	for _, required := range []string{
		"packaging/udev/60-attack-shark-x6-hidraw.rules",
		"/etc/udev/rules.d/60-attack-shark-x6-hidraw.rules",
	} {
		if !strings.Contains(documentation, required) {
			t.Errorf("documentation does not reference %q", required)
		}
	}
	for _, staleReference := range []string{
		"99-attack-shark-x6.rules",
		"99-attack-shark-x6-hidraw.rules",
	} {
		if strings.Contains(documentation, staleReference) {
			t.Errorf("documentation contains stale %s reference", staleReference)
		}
	}
}

func packageDirectory(t *testing.T) string {
	t.Helper()
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locating test source with runtime.Caller")
	}
	return filepath.Dir(sourceFile)
}

func activeRuleLines(t *testing.T, path string) []string {
	t.Helper()
	var active []string
	for _, line := range strings.Split(readFile(t, path), "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			active = append(active, line)
		}
	}
	return active
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(contents)
}
