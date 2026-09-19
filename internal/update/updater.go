package update

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"os/user"
	"strings"

	"golang.org/x/sys/unix"
)

const (
	installedAppImageRelativePath = ".local/share/attack-shark-x6/attack-shark-linux-x86_64.AppImage"
	installedAppImageName         = "attack-shark-linux-x86_64.AppImage"
	pendingMarkerName             = installedAppImageName + ".pending"
	backupAppImageName            = installedAppImageName + ".backup"
	temporaryAppImageName         = installedAppImageName + ".download"
)

var (
	ErrNoUpdate             = errors.New("no newer update is available")
	ErrNoRCUpdate           = errors.New("no release candidate update is available")
	ErrApprovalRequired     = errors.New("user approval is required before replacement")
	ErrUpdateAlreadyApplied = errors.New("verified update has already been applied")

	// currentAccountHome is private so tests can use a temporary account directory.
	// Production authority always comes from the current OS account, not HOME.
	currentAccountHome = accountHome
)

// Transport is the network boundary for the updater. Callers may provide a local
// fake; this package neither constructs URLs nor couples channel selection to a network.
type Transport interface {
	Manifests(context.Context) ([]Manifest, error)
	Download(context.Context, string) (io.ReadCloser, error)
}

// Updater verifies signed manifests and replaces only the installed user AppImage.
type Updater struct {
	CurrentVersion string
	PublicKey      string
	Transport      Transport
}

// VerifiedUpdate is returned only after a signed manifest has passed channel and
// version eligibility checks. It is deliberately consumed by Apply.
type VerifiedUpdate struct {
	// Release is a display-only copy of the verified manifest release. Apply
	// consumes the private immutable copy below, never caller-modifiable fields.
	Release  Release
	release  Release
	verified bool
	applied  bool
}

// Check obtains candidates through the injected transport and returns the newest
// signed update on the installed version's channel.
func (u *Updater) Check(ctx context.Context) (*VerifiedUpdate, error) {
	if u.Transport == nil {
		return nil, errors.New("update transport is required")
	}
	channel, err := channelForVersion(u.CurrentVersion)
	if err != nil {
		return nil, err
	}
	manifests, err := u.Transport.Manifests(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch update manifests: %w", err)
	}

	var selected *Manifest
	for index := range manifests {
		candidate := &manifests[index]
		candidateChannel, err := channelForVersion(candidate.Version)
		if err != nil {
			return nil, fmt.Errorf("invalid candidate version: %w", err)
		}
		if candidateChannel != channel {
			continue
		}
		comparison, err := CompareVersions(candidate.Version, u.CurrentVersion)
		if err != nil {
			return nil, fmt.Errorf("invalid candidate version: %w", err)
		}
		if comparison <= 0 {
			continue
		}
		if selected == nil {
			selected = candidate
			continue
		}
		comparison, err = CompareVersions(candidate.Version, selected.Version)
		if err != nil {
			return nil, fmt.Errorf("compare candidate versions: %w", err)
		}
		if comparison > 0 {
			selected = candidate
		}
	}
	if selected == nil {
		if channel == channelRC {
			return nil, ErrNoRCUpdate
		}
		return nil, ErrNoUpdate
	}
	if err := VerifyManifest(*selected, u.CurrentVersion, u.PublicKey); err != nil {
		return nil, fmt.Errorf("verify selected update: %w", err)
	}
	release := Release{Version: selected.Version, URL: selected.URL, SHA256: selected.SHA256}
	return &VerifiedUpdate{Release: release, release: release, verified: true}, nil
}

// Apply downloads a previously verified update only after explicit approval and
// atomically replaces the fixed installed AppImage path.
func (u *Updater) Apply(ctx context.Context, update *VerifiedUpdate, approved bool) error {
	if !approved {
		return ErrApprovalRequired
	}
	if update == nil || !update.verified {
		return errors.New("update is not a verified update")
	}
	if update.applied {
		return ErrUpdateAlreadyApplied
	}
	if u.Transport == nil {
		return errors.New("update transport is required")
	}
	directory, err := openInstallationDirectory()
	if err != nil {
		return err
	}
	defer directory.Close()
	if err := recoverInstallation(directory); err != nil {
		return err
	}

	mode, err := installedAppImageMode(directory)
	if err != nil {
		return err
	}
	contents, err := u.Transport.Download(ctx, update.release.URL)
	if err != nil {
		return fmt.Errorf("download AppImage: %w", err)
	}
	defer contents.Close()

	if err := removeAt(directory, temporaryAppImageName); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove stale temporary AppImage: %w", err)
	}
	file, err := openTemporary(directory, mode)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = removeAt(directory, temporaryAppImageName)
		}
	}()

	digest := sha256.New()
	if _, err := io.Copy(io.MultiWriter(file, digest), contents); err != nil {
		file.Close()
		return fmt.Errorf("write temporary AppImage: %w", err)
	}
	if fmt.Sprintf("%x", digest.Sum(nil)) != update.release.SHA256 {
		file.Close()
		return errors.New("downloaded AppImage SHA-256 does not match signed manifest")
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return fmt.Errorf("sync temporary AppImage: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close temporary AppImage: %w", err)
	}
	if err := syncUpdateDirectory(directory); err != nil {
		return err
	}
	if err := writePending(directory); err != nil {
		return err
	}
	if err := renameAt(directory, installedAppImageName, backupAppImageName); err != nil {
		return fmt.Errorf("backup installed AppImage: %w", err)
	}
	if err := syncUpdateDirectory(directory); err != nil {
		return err
	}
	if err := renameAt(directory, temporaryAppImageName, installedAppImageName); err != nil {
		return fmt.Errorf("replace installed AppImage: %w", err)
	}
	if err := syncUpdateDirectory(directory); err != nil {
		return err
	}
	committed = true
	if err := recoverInstallation(directory); err != nil {
		return err
	}
	update.applied = true
	return nil
}

// Recover finalizes a completed replacement or restores a backup interrupted
// before the new AppImage was renamed into place. It is safe to call repeatedly.
func (u *Updater) Recover() error {
	directory, err := openInstallationDirectory()
	if err != nil {
		return err
	}
	defer directory.Close()
	return recoverInstallation(directory)
}

type updateChannel uint8

const (
	channelStable updateChannel = iota
	channelRC
)

func channelForVersion(raw string) (updateChannel, error) {
	parsed, err := parseVersion(raw)
	if err != nil {
		return 0, err
	}
	if len(parsed.pre) == 0 {
		return channelStable, nil
	}
	if len(parsed.pre) == 2 && parsed.pre[0] == "rc" && isDigits(parsed.pre[1]) {
		return channelRC, nil
	}
	return 0, errors.New("version must be stable or an rc prerelease")
}

func accountHome() (string, error) {
	current, err := user.Current()
	if err != nil {
		return "", fmt.Errorf("resolve current OS account: %w", err)
	}
	if current.HomeDir == "" {
		return "", errors.New("current OS account home is empty")
	}
	return current.HomeDir, nil
}

// openInstallationDirectory resolves the OS account's fixed install directory
// one component at a time. Each descriptor is opened with O_NOFOLLOW, so later
// replacement of a pathname parent cannot redirect operations outside it.
func openInstallationDirectory() (*os.File, error) {
	home, err := currentAccountHome()
	if err != nil {
		return nil, fmt.Errorf("resolve user home: %w", err)
	}
	if home == "" {
		return nil, errors.New("user home is empty")
	}
	fd, err := unix.Open(home, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		return nil, fmt.Errorf("open user home: %w", err)
	}
	directory := os.NewFile(uintptr(fd), home)
	for _, component := range strings.Split(".local/share/attack-shark-x6", "/") {
		next, err := unix.Openat(int(directory.Fd()), component, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
		if err != nil {
			if isSymlinkAt(directory, component) {
				directory.Close()
				return nil, fmt.Errorf("installed AppImage parent must not be a symlink: %s", component)
			}
			directory.Close()
			return nil, fmt.Errorf("open installed AppImage parent: %w", err)
		}
		directory.Close()
		directory = os.NewFile(uintptr(next), component)
	}
	return directory, nil
}

func installedAppImageMode(directory *os.File) (os.FileMode, error) {
	var stat unix.Stat_t
	if err := unix.Fstatat(int(directory.Fd()), installedAppImageName, &stat, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		return 0, fmt.Errorf("inspect installed AppImage: %w", err)
	}
	if stat.Mode&unix.S_IFMT != unix.S_IFREG {
		return 0, errors.New("installed AppImage must be a regular file")
	}
	return os.FileMode(stat.Mode).Perm(), nil
}

func openTemporary(directory *os.File, mode os.FileMode) (*os.File, error) {
	fd, err := unix.Openat(int(directory.Fd()), temporaryAppImageName, unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_CLOEXEC|unix.O_NOFOLLOW, uint32(mode.Perm()))
	if err != nil {
		return nil, fmt.Errorf("create temporary AppImage: %w", err)
	}
	file := os.NewFile(uintptr(fd), temporaryAppImageName)
	if err := file.Chmod(mode.Perm()); err != nil {
		file.Close()
		return nil, fmt.Errorf("restore temporary AppImage mode: %w", err)
	}
	return file, nil
}

func recoverInstallation(directory *os.File) error {
	pending, err := existsAt(directory, pendingMarkerName)
	if err != nil {
		return fmt.Errorf("inspect update marker: %w", err)
	}
	if !pending {
		return nil
	}
	target, err := existsAt(directory, installedAppImageName)
	if err != nil {
		return fmt.Errorf("inspect installed AppImage during recovery: %w", err)
	}
	if !target {
		if err := renameAt(directory, backupAppImageName, installedAppImageName); err != nil {
			return fmt.Errorf("restore interrupted AppImage replacement: %w", err)
		}
	} else if err := removeAt(directory, backupAppImageName); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove completed replacement backup: %w", err)
	}
	// Make the restored target (or completed backup cleanup) durable before the
	// marker can be cleared. A sync failure leaves recovery repeatable.
	if err := syncUpdateDirectory(directory); err != nil {
		return err
	}
	if err := removeAt(directory, pendingMarkerName); err != nil {
		return fmt.Errorf("remove update marker: %w", err)
	}
	return syncUpdateDirectory(directory)
}

func writePending(directory *os.File) error {
	fd, err := unix.Openat(int(directory.Fd()), pendingMarkerName, unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0o600)
	if err != nil {
		return fmt.Errorf("create update marker: %w", err)
	}
	marker := os.NewFile(uintptr(fd), pendingMarkerName)
	if _, err := marker.WriteString("pending\n"); err != nil {
		marker.Close()
		return fmt.Errorf("write update marker: %w", err)
	}
	if err := marker.Sync(); err != nil {
		marker.Close()
		return fmt.Errorf("sync update marker: %w", err)
	}
	if err := marker.Close(); err != nil {
		return fmt.Errorf("close update marker: %w", err)
	}
	return syncUpdateDirectory(directory)
}

func existsAt(directory *os.File, name string) (bool, error) {
	var stat unix.Stat_t
	err := unix.Fstatat(int(directory.Fd()), name, &stat, unix.AT_SYMLINK_NOFOLLOW)
	if errors.Is(err, unix.ENOENT) {
		return false, nil
	}
	return err == nil, err
}

func isSymlinkAt(directory *os.File, name string) bool {
	var stat unix.Stat_t
	return unix.Fstatat(int(directory.Fd()), name, &stat, unix.AT_SYMLINK_NOFOLLOW) == nil && stat.Mode&unix.S_IFMT == unix.S_IFLNK
}

func removeAt(directory *os.File, name string) error {
	if err := unix.Unlinkat(int(directory.Fd()), name, 0); err != nil {
		return err
	}
	return nil
}

func renameAt(directory *os.File, oldName, newName string) error {
	return unix.Renameat(int(directory.Fd()), oldName, int(directory.Fd()), newName)
}

var syncUpdateDirectory = syncDirectory

func syncDirectory(directory *os.File) error {
	if err := directory.Sync(); err != nil {
		return fmt.Errorf("sync update directory: %w", err)
	}
	return nil
}
