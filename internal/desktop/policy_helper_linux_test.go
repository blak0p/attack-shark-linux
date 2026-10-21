package desktop

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/blak0p/attack-shark-linux/internal/udevauth"
)

func TestInstalledPolicyExecution(t *testing.T) {
	for _, tc := range []struct {
		name, out, diagnostic string
		code                  int
		installed, fail       bool
	}{
		{"current", "current\n", "", 0, false, false},
		{"installed", "installed\n", "", 0, true, false},
		{"reload", "installed\n", "error: installed-but-reload-failed\n", 3, true, true},
		{"trigger", "installed\n", "error: installed-but-trigger-failed\n", 4, true, true},
		{"denied", "", "Not authorized\n", 126, false, true},
		{"dismissed", "", "", 127, false, true},
		{"invalid", "", "error: invalid policy\n", 1, false, true},
		{"unexpected", "surprise\n", "", 0, false, true},
		{"overflow", "installed\n" + strings.Repeat("x", helperOutputLimit), "", 0, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			h := policyHelper{inspect: func(string) (helperFile, error) { return helperFile{directory: true, executable: true}, nil }, run: func(ctx context.Context, command string, args []string, input []byte, out, diagnostic io.Writer) (int, error) {
				calls++
				if command != "/usr/bin/pkexec" || strings.Join(args, "|") != "--disable-internal-agent|/usr/local/libexec/attack-shark-linux-udev-helper|1.2.3" || string(input) != "signed\n" {
					t.Fatalf("invocation %q %q %q", command, args, input)
				}
				_, _ = io.WriteString(out, tc.out)
				_, _ = io.WriteString(diagnostic, tc.diagnostic)
				return tc.code, nil
			}}
			// Fixed executable leaves are regular, ancestors are directories.
			original := h.inspect
			h.inspect = func(path string) (helperFile, error) {
				m, e := original(path)
				m.directory = path != "/usr/bin/pkexec" && path != installedPolicyHelper
				return m, e
			}
			got, err := h.apply(context.Background(), &udevauth.Policy{Version: "1.2.3", Bytes: []byte("policy")}, []byte("signed\n"))
			if got.Installed != tc.installed || (err != nil) != tc.fail || calls != 1 {
				t.Fatalf("result=%+v error=%v calls=%d", got, err, calls)
			}
		})
	}
}

func TestInstalledPolicyRejectsBeforeInvocation(t *testing.T) {
	for _, name := range []string{"nil", "version", "empty", "oversize", "missing", "unsafe", "directory-leaf", "regular-ancestor", "not-executable", "canceled", "deadline"} {
		t.Run(name, func(t *testing.T) {
			p := &udevauth.Policy{Version: "1.2.3", Bytes: []byte("policy")}
			wire := []byte("signed\n")
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			calls := 0
			h := policyHelper{inspect: func(path string) (helperFile, error) {
				if name == "missing" {
					return helperFile{}, errors.New("missing")
				}
				if name == "unsafe" {
					return helperFile{unsafe: true}, nil
				}
				m := helperFile{directory: path != "/usr/bin/pkexec" && path != installedPolicyHelper, executable: true}
				if name == "directory-leaf" {
					m.directory = true
				}
				if name == "regular-ancestor" {
					m.directory = false
				}
				if name == "not-executable" {
					m.executable = false
				}
				return m, nil
			}, run: func(context.Context, string, []string, []byte, io.Writer, io.Writer) (int, error) {
				calls++
				return 0, nil
			}}
			switch name {
			case "nil":
				p = nil
			case "version":
				p.Version = "--help"
			case "empty":
				wire = nil
			case "oversize":
				wire = make([]byte, udevauth.MaxEnvelopeBytes+1)
			case "canceled":
				cancel()
			case "deadline":
				var stop context.CancelFunc
				ctx, stop = context.WithDeadline(ctx, time.Now().Add(-time.Second))
				defer stop()
			}
			got, err := h.apply(ctx, p, wire)
			if got.Installed || err == nil || calls != 0 {
				t.Fatalf("result=%+v error=%v calls=%d", got, err, calls)
			}
		})
	}
}

func TestInstalledPolicyCancellationRetainsWrite(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	h := policyHelper{inspect: func(path string) (helperFile, error) {
		return helperFile{directory: path != "/usr/bin/pkexec" && path != installedPolicyHelper, executable: true}, nil
	}, run: func(_ context.Context, _ string, _ []string, _ []byte, out, _ io.Writer) (int, error) {
		io.WriteString(out, "installed\n")
		cancel()
		return -1, context.Canceled
	}}
	got, err := h.apply(ctx, &udevauth.Policy{Version: "1.2.3", Bytes: []byte("policy")}, []byte("signed\n"))
	if !got.Installed || !errors.Is(err, context.Canceled) {
		t.Fatalf("%+v %v", got, err)
	}
}
