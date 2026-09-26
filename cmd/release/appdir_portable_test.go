package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrepareAppDirWrapsAppRunAndInjectsWebKitHook(t *testing.T) {
	tempDir := t.TempDir()

	// Reconstruct minimal AppDir structure
	appDir := filepath.Join(tempDir, "test.AppDir")
	webkitDir := filepath.Join(appDir, "usr", "lib", "x86_64-linux-gnu", "webkitgtk-6.0")
	if err := os.MkdirAll(webkitDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create dummy WebKit processes with restrictive permissions
	networkProc := filepath.Join(webkitDir, "WebKitNetworkProcess")
	webProc := filepath.Join(webkitDir, "WebKitWebProcess")
	if err := os.WriteFile(networkProc, []byte("binary-content"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(webProc, []byte("binary-content"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create dummy ELF AppRun
	appRunPath := filepath.Join(appDir, "AppRun")
	dummyELF := []byte{0x7f, 'E', 'L', 'F', 0x02, 0x01, 0x01, 0x00}
	if err := os.WriteFile(appRunPath, dummyELF, 0o755); err != nil {
		t.Fatal(err)
	}

	// Locate prepare-appdir.sh
	prepareScript, err := filepath.Abs(filepath.Join("..", "..", "packaging", "appimage", "prepare-appdir.sh"))
	if err != nil {
		t.Fatal(err)
	}

	// Run prepare-appdir.sh
	cmd := exec.Command("/bin/sh", prepareScript, appDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("prepare-appdir.sh failed: %v, output: %s", err, output)
	}

	// Verify WebKit process permissions
	for _, procPath := range []string{networkProc, webProc} {
		info, err := os.Stat(procPath)
		if err != nil {
			t.Fatalf("failed to stat %s: %v", procPath, err)
		}
		if info.Mode()&0o111 == 0 {
			t.Errorf("expected %s to be executable, got mode %v", procPath, info.Mode())
		}
	}

	// Verify libwebkit-relocate.so was compiled and is present
	shimPath := filepath.Join(appDir, "usr", "lib", "libwebkit-relocate.so")
	shimInfo, err := os.Stat(shimPath)
	if err != nil {
		t.Fatalf("libwebkit-relocate.so was not created: %v", err)
	}
	if shimInfo.Mode()&0o111 == 0 {
		t.Errorf("expected %s to be executable, got mode %v", shimPath, shimInfo.Mode())
	}

	// Verify AppRun was wrapped
	appRunWrappedPath := filepath.Join(appDir, "AppRun.wrapped")
	if _, err := os.Stat(appRunWrappedPath); err != nil {
		t.Fatalf("AppRun.wrapped was not created: %v", err)
	}

	// Verify AppRun is a script
	appRunBytes, err := os.ReadFile(appRunPath)
	if err != nil {
		t.Fatal(err)
	}
	appRunContent := string(appRunBytes)
	if !strings.HasPrefix(appRunContent, "#!/bin/sh") {
		t.Errorf("expected AppRun to be a shell script, got %s", appRunContent)
	}
	if !strings.Contains(appRunContent, "apprun-hooks") {
		t.Errorf("expected AppRun script to reference apprun-hooks, got %s", appRunContent)
	}
	if !strings.Contains(appRunContent, "libwebkit-relocate.so") {
		t.Errorf("expected AppRun script to reference libwebkit-relocate.so, got %s", appRunContent)
	}
	if !strings.Contains(appRunContent, "LD_PRELOAD") {
		t.Errorf("expected AppRun script to export LD_PRELOAD, got %s", appRunContent)
	}

	// Verify hook script content and execution
	hookPath := filepath.Join(appDir, "apprun-hooks", "01-webkit-exec-path.sh")
	hookBytes, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatalf("failed to read hook script: %v", err)
	}
	hookContent := string(hookBytes)
	if !strings.Contains(hookContent, "WEBKIT_EXEC_PATH") {
		t.Errorf("expected hook to contain WEBKIT_EXEC_PATH, got %s", hookContent)
	}
	if !strings.Contains(hookContent, "usr/lib/x86_64-linux-gnu/webkitgtk-6.0") {
		t.Errorf("expected hook to contain webkit helper path, got %s", hookContent)
	}

	// Replace AppRun.wrapped with a test script verifying WEBKIT_EXEC_PATH and LD_PRELOAD are exported
	testWrappedScript := "#!/bin/sh\nprintf '%s|%s' \"$WEBKIT_EXEC_PATH\" \"$LD_PRELOAD\"\n"
	if err := os.WriteFile(appRunWrappedPath, []byte(testWrappedScript), 0o755); err != nil {
		t.Fatal(err)
	}

	runCmd := exec.Command(appRunPath)
	runOutput, err := runCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("AppRun execution failed: %v, output: %s", err, runOutput)
	}

	parts := strings.Split(string(runOutput), "|")
	if len(parts) != 2 {
		t.Fatalf("unexpected output format from AppRun: %s", string(runOutput))
	}

	expectedPath := filepath.Join(appDir, "usr", "lib", "x86_64-linux-gnu", "webkitgtk-6.0")
	if parts[0] != expectedPath {
		t.Errorf("expected WEBKIT_EXEC_PATH=%q, got %q", expectedPath, parts[0])
	}
	if !strings.Contains(parts[1], "libwebkit-relocate.so") {
		t.Errorf("expected LD_PRELOAD to contain libwebkit-relocate.so, got %q", parts[1])
	}
}
