package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReleaseWorkflowEnforcesCanonicalReleasePolicy(t *testing.T) {
	workflowPath := filepath.Join("..", "..", ".github", "workflows", "release.yml")
	workflowBytes, err := os.ReadFile(workflowPath)
	if err != nil {
		t.Fatal(err)
	}
	workflow := string(workflowBytes)

	requireExactOnce(t, workflow, "on:\n  push:\n    tags:\n      - 'v*'\n\npermissions:", "tag-only push trigger")
	requireExactOnce(t, workflow, "permissions:\n  contents: write\n\njobs:", "contents: write-only permission")

	annotatedTag := workflowStep(t, workflow, "Verify annotated tag")
	for _, required := range []string{
		"TAG: ${{ github.ref_name }}",
		`TAG="${TAG#refs/tags/}"`,
		`git for-each-ref --format='%(objecttype)' "refs/tags/$TAG"`,
		`if [[ "$object_type" != "tag" ]]; then`,
		`git for-each-ref --format='%(contents)' "refs/tags/$TAG"`,
	} {
		if !strings.Contains(annotatedTag, required) {
			t.Errorf("annotated-tag validation must contain %q", required)
		}
	}

	packaging := workflowStep(t, workflow, "Package signed release artifacts")
	if !strings.Contains(packaging, "working-directory: cmd/x6configurator\n") {
		t.Error("packaging must run from cmd/x6configurator")
	}
	if !strings.Contains(packaging, "run: task package:container:release\n") {
		t.Error("packaging must invoke the release packaging task exactly")
	}
	if !strings.Contains(packaging, "ATTACK_SHARK_RELEASE_SIGNING_SEED: ${{ secrets.ATTACK_SHARK_RELEASE_SIGNING_SEED }}") {
		t.Error("release signing seed must be available to packaging")
	}
	if strings.Count(workflow, "secrets.ATTACK_SHARK_RELEASE_SIGNING_SEED") != 1 {
		t.Error("release signing seed must be scoped only to the packaging step")
	}

	release := workflowStep(t, workflow, "Create GitHub Release")
	for _, required := range []string{
		`if [[ "$TAG" =~ -rc\. ]]; then`,
		"release_args+=(--prerelease --latest=false)",
		"release_args+=(--latest)",
	} {
		if !strings.Contains(release, required) {
			t.Errorf("release metadata must contain %q", required)
		}
	}

	expectedAssets := []string{
		"cmd/x6configurator/build/release/attack-shark-linux-x86_64.AppImage",
		"cmd/x6configurator/build/release/attack-shark-linux-x86_64.AppImage.sha256",
		"cmd/x6configurator/build/release/update-manifest.json",
		"packaging/udev/60-attack-shark-x6-hidraw.rules",
	}
	assets := releaseAssetArguments(release)
	if !hasExactReleaseAssets(assets, expectedAssets) {
		t.Errorf("release must upload exactly the canonical assets %q, got %q", expectedAssets, assets)
	}
	for _, forbidden := range []string{"99-", "*.AppImage", "build/release/*"} {
		if strings.Contains(release, forbidden) {
			t.Errorf("release assets must not contain %q", forbidden)
		}
	}
}

func requireExactOnce(t *testing.T, contents, want, policy string) {
	t.Helper()
	if strings.Count(contents, want) != 1 {
		t.Errorf("release workflow must have exactly one %s", policy)
	}
}

func TestHasExactReleaseAssetsRejectsNonCanonicalPaths(t *testing.T) {
	expected := []string{"one.AppImage", "one.AppImage.sha256", "update-manifest.json", "60-device.rules"}
	for _, test := range []struct {
		name   string
		assets []string
		want   bool
	}{
		{name: "canonical paths", assets: expected, want: true},
		{name: "wildcard path", assets: []string{"*.AppImage", "one.AppImage.sha256", "update-manifest.json", "60-device.rules"}},
		{name: "obsolete 99 rule", assets: []string{"one.AppImage", "one.AppImage.sha256", "update-manifest.json", "99-device.rules"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := hasExactReleaseAssets(test.assets, expected); got != test.want {
				t.Errorf("hasExactReleaseAssets() = %t, want %t", got, test.want)
			}
		})
	}
}

func hasExactReleaseAssets(assets, expected []string) bool {
	if len(assets) != len(expected) {
		return false
	}
	for index, expectedAsset := range expected {
		if assets[index] != expectedAsset {
			return false
		}
	}
	return true
}

func releaseAssetArguments(step string) []string {
	const command = `          gh release create "$TAG" "${release_args[@]}" \`
	start := strings.Index(step, command)
	if start == -1 {
		return nil
	}

	var assets []string
	for _, line := range strings.Split(step[start+len(command):], "\n") {
		asset := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(line), "\\"))
		if asset != "" {
			assets = append(assets, asset)
		}
	}
	return assets
}

func workflowStep(t *testing.T, workflow, name string) string {
	t.Helper()
	marker := "      - name: " + name + "\n"
	start := strings.Index(workflow, marker)
	if start == -1 {
		t.Fatalf("release workflow is missing step %q", name)
	}
	step := workflow[start:]
	if next := strings.Index(step[len(marker):], "\n      - name: "); next != -1 {
		step = step[:len(marker)+next]
	}
	return step
}
