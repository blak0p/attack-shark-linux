package desktop

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/blak0p/attack-shark-linux/internal/udevauth"
)

const installedPolicyHelper = "/usr/local/libexec/attack-shark-linux-udev-helper"
const helperOutputLimit = 4096

type helperFile struct{ directory, executable, unsafe bool }
type policyHelper struct {
	inspect func(string) (helperFile, error)
	run     func(context.Context, string, []string, []byte, io.Writer, io.Writer) (int, error)
}

func installedHelper() policyHelper {
	return policyHelper{inspect: inspectHelperFile, run: runPolicyHelper}
}

func inspectHelperFile(path string) (helperFile, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return helperFile{}, err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	mode := info.Mode()
	return helperFile{directory: info.IsDir(), executable: mode.Perm()&0111 != 0, unsafe: !ok || stat.Uid != 0 || mode.Perm()&0022 != 0 || (!info.IsDir() && !mode.IsRegular())}, nil
}

// Root-owned non-writable ancestors prevent unprivileged substitution. As with
// the helper's lock, this is not CAS protection against independent root editors.
func (h policyHelper) apply(ctx context.Context, p *udevauth.Policy, wire []byte) (PolicyApplyResult, error) {
	result := PolicyApplyResult{}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if !validHelperInput(p, wire) {
		return result, fmt.Errorf("invalid bounded authenticated policy input")
	}
	if h.inspect == nil || h.run == nil {
		return result, helperUnavailable()
	}
	for _, path := range []string{"/", "/usr", "/usr/bin", "/usr/local", "/usr/local/libexec", "/usr/bin/pkexec", installedPolicyHelper} {
		m, err := h.inspect(path)
		leaf := path == "/usr/bin/pkexec" || path == installedPolicyHelper
		if err != nil || m.unsafe || m.directory == leaf || (leaf && !m.executable) {
			return result, helperUnavailable()
		}
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	bounded, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	stdout, stderr := &helperOutput{}, &helperOutput{}
	code, err := h.run(bounded, "/usr/bin/pkexec", []string{"--disable-internal-agent", installedPolicyHelper, p.Version}, wire, stdout, stderr)
	// The first complete installed line is durable evidence even if cancellation,
	// later diagnostics or output overflow prevent a successful process completion.
	result.Installed = bytes.HasPrefix(stdout.data, []byte("installed\n"))
	if bounded.Err() != nil {
		return result, bounded.Err()
	}
	if err != nil {
		return result, fmt.Errorf("USB policy helper execution: %w", err)
	}
	if stdout.overflow || stderr.overflow {
		return result, fmt.Errorf("USB policy helper output exceeded limit")
	}
	if code == 0 && len(stderr.data) == 0 && (string(stdout.data) == "current\n" || string(stdout.data) == "installed\n") {
		return result, nil
	}
	if code == 126 || code == 127 {
		return result, fmt.Errorf("USB policy authorization denied or dismissed (exit %d)", code)
	}
	if result.Installed && code == 3 && string(stderr.data) == "error: installed-but-reload-failed\n" {
		return result, fmt.Errorf("installed-but-reload-failed")
	}
	if result.Installed && code == 4 && string(stderr.data) == "error: installed-but-trigger-failed\n" {
		return result, fmt.Errorf("installed-but-trigger-failed")
	}
	return result, fmt.Errorf("USB policy helper failed (exit %d); inspect setup and installed policy before retrying", code)
}

// Always drain child pipes, retaining only a fixed prefix. Returning a short
// write could strand the child or discard an already observed completed write.
type helperOutput struct {
	data     []byte
	overflow bool
}

func (b *helperOutput) Write(p []byte) (int, error) {
	n := len(p)
	remaining := helperOutputLimit - len(b.data)
	if n > remaining {
		b.overflow = true
		p = p[:remaining]
	}
	b.data = append(b.data, p...)
	return n, nil
}

func runPolicyHelper(ctx context.Context, command string, args []string, input []byte, out, diagnostic io.Writer) (int, error) {
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Stdin = bytes.NewReader(input)
	cmd.Stdout = out
	cmd.Stderr = diagnostic
	// Do not inherit loader/shell/OpenSSL overrides from the AppImage environment.
	cmd.Env = []string{"PATH=/usr/sbin:/usr/bin:/sbin:/bin", "LC_ALL=C"}
	// Bound pipe draining if a descendant retains an inherited output descriptor.
	cmd.WaitDelay = time.Second
	err := cmd.Run()
	if err == nil {
		return 0, nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode(), nil
	}
	return -1, err
}
