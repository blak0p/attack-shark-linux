// Package udevpolicy provides read-only installed-policy classification.
// Its optional Linux source reads the fixed installed path; it never installs
// or reloads rules.
package udevpolicy

import (
	_ "embed"
	"errors"
	"fmt"
	"io/fs"

	packaged "github.com/blak0p/attack-shark-linux/packaging/udev"
)

const InstalledPath = "/etc/udev/rules.d/60-attack-shark-x6-hidraw.rules"
const MaxPolicyBytes = 64 * 1024

type State string

const (
	Current  State = "current"
	Missing  State = "missing"
	Outdated State = "outdated"
	Custom   State = "custom"
	Error    State = "error"
)

// Result reports an exact-content classification. Err is non-nil only for Error.
type Result struct {
	State State
	Err   error
}

// Source is a read-only seam for InstalledPath, not a renderer-selected path.
// Lstat must not follow symlinks. Read must return at most limit bytes, and
// must reject symlinks and nonregular files even if replaced since Lstat.
// The Linux InstalledSource adapter enforces these guarantees on the opened
// descriptor independently of the preceding Lstat.
// No write or process capabilities are requested or used.
type Source interface {
	Lstat() (fs.FileMode, error)
	Read(limit int) ([]byte, error)
}

// Snapshot of the official fa60-only policy at 31665a9; never consult git at runtime.
//go:embed testdata/fa60-official.rules
var previousPolicy string

// Inspect compares exact bytes, preserving all unrecognized policies as Custom.
// Even harmless local comment edits are custom: recognition is deliberately
// conservative so a future installer cannot overwrite local directives.
func Inspect(source Source) Result {
	fail := func(err error) Result { return Result{State: Error, Err: err} }
	if source == nil {
		return fail(errors.New("udev policy source is nil"))
	}
	mode, err := source.Lstat()
	if errors.Is(err, fs.ErrNotExist) {
		return Result{State: Missing}
	}
	if err != nil {
		return fail(fmt.Errorf("inspect udev policy: %w", err))
	}
	if !mode.IsRegular() {
		return fail(errors.New("udev policy is not a regular file"))
	}
	data, err := source.Read(MaxPolicyBytes + 1)
	if err != nil {
		return fail(fmt.Errorf("read udev policy: %w", err))
	}
	if len(data) > MaxPolicyBytes {
		return fail(errors.New("udev policy exceeds size limit"))
	}
	switch string(data) {
	case packaged.Policy():
		return Result{State: Current}
	case previousPolicy:
		return Result{State: Outdated}
	default:
		return Result{State: Custom}
	}
}
