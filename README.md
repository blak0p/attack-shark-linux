# Attack Shark Linux

Linux desktop configurator for the **Attack Shark X6** gaming mouse, built with
Go + [Wails v3](https://wails.io) (backend) and React + Vite (frontend).

> **Status**: Beta. The release implementation includes a signed x86_64
> AppImage and a release-asset installer with signed artifact verification.
> Remote RC rehearsal and stable publication remain gated; this repository does not claim a published Latest
> release. Not affiliated with or endorsed by Attack Shark.

## Features

- **Multi-device inventory** over kernel-backed hidraw, with selection by serial.
- **Live status monitoring** — battery heartbeats and physical DPI button
  changes are pushed to the UI as they arrive from the dongle.
- **DPI configuration** — up to 8 stages, 50–26000 in 50-unit steps, with
  per-stage colors and an active-stage circle UI.
- **Apply on demand** — writes the 56-byte configuration report over hidraw
  (`SET_REPORT`), with debounced auto-sync after one second of inactivity.
- **Per-device persistence** — versioned device profiles are stored locally and
  survive restarts, with factory-defaults restore. This is local persistence,
  not on-device profiles.
- **Polling-rate configuration** — select 125, 250, 500, or 1000 Hz; the
  acknowledged choice persists per serial-bearing device, while session-only
  devices retain it only for the current session.
- **Serialless X6 sessions** — identifies and configures dongles without a
  usable serial.

## Prerequisites

- Linux with the X6 dongle connected (`VID:PID 1d57:fa60`).
- The packaged udev policy installed — see
  [docs/linux-usb-prerequisites.md](docs/linux-usb-prerequisites.md).

> **Do not** run the app as root and **do not** change the udev rules to
> world-writable mode `0666`. The policy grants the active seat user access via
> `TAG+="uaccess"` with device mode `0660`.

## Release channels and installation

Normal installation selects the newest valid signed stable GitHub release.
Once a stable release is published, fetch its immutable installer asset through
the latest stable release URL, without `--beta`:

```sh
(
  installer=$(mktemp) || exit 1
  trap 'rm -f "$installer"' 0
  curl --fail --location --output "$installer" \
    https://github.com/blak0p/attack-shark-linux/releases/latest/download/install.sh &&
    sh "$installer"
)
```

Until a stable release exists, this download fails; normal installation never
falls back to an RC or source from `main`. To rehearse an RC instead, download
`install.sh` from a trusted RC asset on the
[GitHub releases page](https://github.com/blak0p/attack-shark-linux/releases),
then explicitly select the beta channel:

```sh
sh ./install.sh --beta
```

The installer selects the newest valid signed release in the chosen channel;
no version needs to be hardcoded. Obtain `install.sh` only from a trusted
release asset. The installer writes the
AppImage and absolute desktop entry under the current OS account's user-local
data directories. Adding `--install-udev` is separate, requires explicit
confirmation before `sudo`, and has a manual fallback. The installer-managed
rule is exactly
`60-attack-shark-x6-hidraw.rules`.

The installed AppImage checks for a newer signed release and requires explicit
user approval before replacement. An RC installation updates only to a newer
signed RC; a stable installation updates only to a newer signed stable release.
Updates are confined to
`~/.local/share/attack-shark-x6/attack-shark-linux-x86_64.AppImage`; the updater
never elevates and never changes udev rules. A stable release and the RC
rehearsal are remote gates, so no live installer, update, or rehearsal evidence
is claimed here.

## Emergency recovery

If an applied configuration leaves the mouse unusable, run the headless factory
reset with the application binary:

```sh
attack-shark-linux reset
```

For an installed AppImage, invoke the same recovery command from its install
directory:

```sh
cd ~/.local/share/attack-shark-x6
./attack-shark-linux-x86_64.AppImage reset
```

This command opens no window. It selects one validated X6 hidraw target and
writes the documented factory configuration, including DPI, polling, and button
remap defaults. Use it only for recovery: it replaces the mouse's current
configuration. Do not run it as root; install the udev policy first.

## Release limits

Macros, on-device profiles, and DEB/RPM/AUR packages or distribution
repositories are deferred. Local per-device persistence does not imply that
profiles are stored on the mouse.

## Building

Requirements: Go 1.25+, Node.js (for the frontend), and [Task](https://taskfile.dev).

```sh
# Generate the embedded frontend before any Go build, test, or vet command.
(cd frontend && npm ci && npm run build)

# Backend + embedded frontend
task --dir cmd/x6configurator build

# Or build the Go packages manually:
go build ./...
```

## Testing

```sh
# Required once per clean checkout, before Go checks:
(cd frontend && npm ci && npm run build)

go test ./...        # backend unit tests (hidraw tests use a fake, no device needed)
go vet ./...         # static analysis
(cd frontend && npm test)   # frontend unit tests (vitest)
```

## Documentation

| Doc | What it covers |
|---|---|
| [docs/linux-usb-prerequisites.md](docs/linux-usb-prerequisites.md) | udev policy, build prerequisites, permission troubleshooting |
| [docs/protocol-x6.md](docs/protocol-x6.md) | HID protocol decoded from `X6.exe` and validated on the dongle |
| [docs/protocol-captures.md](docs/protocol-captures.md) | Captured report evidence (raw `.pcapng` in `captures/`) |
| [docs/app-x6.md](docs/app-x6.md) | Complete UI map of the official Windows app |
| [docs/capture-plan.md](docs/capture-plan.md) | How captures are produced |
| [docs/config-baseline.md](docs/config-baseline.md) | Factory defaults and per-device configuration semantics |

## Security

- Reads are passive; writes happen **on demand** only when you apply a change.
- Never commit the official Windows application or any proprietary material —
  it is local research reference only (see `.gitignore`).
- To report a vulnerability, follow [SECURITY.md](SECURITY.md) — **do not** open
  a public issue.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) and
[CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md). This project is licensed under the
[MIT License](LICENSE).
