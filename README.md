# Attack Shark Linux

<a href="https://github.com/Gentleman-Programming/gentle-ai">
  <img width="220" src="https://raw.githubusercontent.com/Gentleman-Programming/gentle-ai/main/docs/assets/brand/built-with-gentle-ai.png" alt="Built with Gentle-AI" />
</a>

Linux desktop configurator for the **Attack Shark X6** gaming mouse, built with
Go + [Wails v3](https://wails.io) (backend) and React + Vite (frontend).

![Attack Shark Linux - Performance View](docs/assets/screenshots/ui-performance.png)

> **Releases:** Linux x86_64 AppImages and a user-local installer are available
> from [GitHub Releases](https://github.com/blak0p/attack-shark-linux/releases).
> The installer verifies signed release metadata and the AppImage digest; check
> the release page for currently available stable and RC versions. Not
> affiliated with or endorsed by Attack Shark.

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

## Screenshots

See [docs/user-guide.md](docs/user-guide.md) for detailed feature descriptions and setting guides.

| Performance (DPI, Polling, Sleep) | Lighting (RGB Effects & Colors) |
|:---:|:---:|
| <a href="docs/assets/screenshots/ui-performance.png"><img src="docs/assets/screenshots/ui-performance.png" width="440" alt="Performance View" /></a> | <a href="docs/assets/screenshots/ui-lighting.png"><img src="docs/assets/screenshots/ui-lighting.png" width="440" alt="Lighting View" /></a> |

| Controls (Sensor & Debounce) | Button Remapping |
|:---:|:---:|
| <a href="docs/assets/screenshots/ui-controls.png"><img src="docs/assets/screenshots/ui-controls.png" width="440" alt="Controls View" /></a> | <a href="docs/assets/screenshots/ui-button-remapping.png"><img src="docs/assets/screenshots/ui-button-remapping.png" width="440" alt="Button Remapping View" /></a> |

<details>
<summary><b>Device & Maintenance View</b></summary>
<br>

<a href="docs/assets/screenshots/ui-device.png"><img src="docs/assets/screenshots/ui-device.png" width="700" alt="Device View" /></a>
</details>

## Prerequisites

- Linux with an X6 dongle (`1d57:fa60`) or X6 USB cable (`1d57:fa61`).
  Both use the existing validated command protocol; Bluetooth and other models
  are not supported. The actual PID remains part of each device's saved identity.
- With one validated cable and one validated dongle connected, the cable is
  selected automatically for status and configuration; no device selector is
  needed for this pair. Their saved identities remain separate. Multiple cables
  or receivers remain ambiguous rather than choosing the first device.
- The packaged udev policy installed — see
  [docs/linux-usb-prerequisites.md](docs/linux-usb-prerequisites.md).

> **Do not** run the app as root and **do not** change the udev rules to
> world-writable mode `0666`. The policy grants the active seat user access via
> `TAG+="uaccess"` with device mode `0660`.

## Release channels and installation

Normal installation selects the newest valid signed stable GitHub release.
For a published stable release, fetch its installer asset through the latest
stable release URL, without `--beta`:

```sh
(
  installer=$(mktemp) || exit 1
  trap 'rm -f "$installer"' 0
  curl --fail --location --output "$installer" \
    https://github.com/blak0p/attack-shark-linux/releases/latest/download/install.sh &&
    sh "$installer" --install-udev
)
```

If no signed stable release is available, normal installation fails rather than
falling back to an RC or source from `main`. To rehearse an RC instead, download
`install.sh` from a trusted RC asset on the
[GitHub releases page](https://github.com/blak0p/attack-shark-linux/releases),
then explicitly select the beta channel:

```sh
sh ./install.sh --beta --install-udev
```

Use `--install-udev` on the first interactive install so the X6 dongle can be
accessed without running the app as root. The installer asks for confirmation
before using `sudo` to install and reload the canonical
`60-attack-shark-x6-hidraw.rules`; declining or running without a terminal
leaves the rule untouched and prints manual instructions. The installer selects
the newest valid signed release in the chosen channel; no version needs to be
hardcoded. Obtain `install.sh` only from a trusted release asset. The AppImage
and absolute desktop entry are installed under the current OS account's
user-local data directories. This legacy/manual rule installation does not install
the authenticated update helper; repeat installs may omit `--install-udev`.

The installed AppImage checks for a newer signed release and requires explicit
user approval before replacement. An RC installation updates only to a newer
signed RC; a stable installation updates only to a newer signed stable release.
AppImage replacement is confined to
`~/.local/share/attack-shark-x6/attack-shark-linux-x86_64.AppImage` and never elevates.
Startup checks produce one combined notice for AppImage and USB permissions.
Review **Device → Updates**, then explicitly **Apply updates**. Only missing or
recognized outdated official rules are eligible; custom rules are preserved.
USB policy updates use one system authorization through a separately installed,
authenticated helper; the app never collects a password or bootstraps the helper.
See [trusted initial setup](docs/linux-usb-prerequisites.md#one-time-authenticated-usb-permissions-setup)
and [update outcomes](docs/user-guide.md#4-updates). The signed RC.4-to-RC.5 rehearsal
verified AppImage replacement in an isolated account; the maintainer observed
the in-app update card and relaunch. Real-device installation requires its own
udev authorization and validation.

## Optional workaround: fast macro clicks with libinput

If the kernel receives every click but fast macro clicks are merged on the
desktop, libinput's mechanical debounce may be filtering them. This workaround
was tested on **CachyOS with libinput 1.32.0-1.1**, with user-confirmed results
for both the USB receiver and cable. The [upstream report](https://gitlab.freedesktop.org/libinput/libinput/-/work_items/1349)
tracks the problem; this local workaround is **not an upstream fix**.

> **Optional, temporary local workaround:** this disables mechanical debounce
> for the matched USB identities, potentially exposing switch bounce. The receiver
> identity may be shared by other devices. `ModelBouncingKeys` is an internal,
> version-dependent libinput quirk, not a stable setting or an installer default.

### Apply

1. Run `sudo libinput list-devices` and find the current mouse event node with
   pointer capability. Replace `/dev/input/eventX` below with that node; it can
   change after reconnecting or switching between receiver and cable.
2. If `/etc/libinput` is absent, create it with `sudo mkdir -p /etc/libinput`.
   If `/etc/libinput/local-overrides.quirks` already exists, back it up to a
   separate, unused backup path before editing (for example,
   `sudo cp -n /etc/libinput/local-overrides.quirks /etc/libinput/local-overrides.quirks.before-x6`).
   Keep that backup for rollback; do not overwrite an older backup.
3. Open `sudoedit /etc/libinput/local-overrides.quirks` and append these two
   sections, preserving all existing entries. Do not edit `/usr/share/libinput`.

```ini
[Attack Shark X6 USB receiver fast clicks]
MatchUdevType=mouse
MatchBus=usb
MatchVendor=0x1D57
MatchProduct=0xFA60
ModelBouncingKeys=1

[Attack Shark X6 USB cable fast clicks]
MatchUdevType=mouse
MatchBus=usb
MatchVendor=0x1D57
MatchProduct=0xFA61
ModelBouncingKeys=1
```

### Verify and activate

Check the active connection's event node, then test the same rapid-click macro:

```sh
sudo libinput quirks list /dev/input/eventX
sudo libinput debug-events --device /dev/input/eventX
```

The quirks output should include `ModelBouncingKeys=1`; the event stream should
show separate button press/release events for each click rather than merged
clicks. Stop the diagnostic with Ctrl+C. Repeat for the other connection using
its current event node if you use both receiver and cable.

These diagnostics create a **new libinput context**; success there does not
reload your running desktop compositor. Save your work, then log out and back
in to activate the override in the desktop. No reboot is needed.

### Undo

Use `sudoedit /etc/libinput/local-overrides.quirks` to remove **only the two
sections added above**, preserving all other entries. Alternatively, restore
your backup if no unrelated edits have been made since it was taken. Do not
blindly delete the whole file. Save your work and log out and back in again.

## Per-game profiles (launch wrapper)

Switch the mouse to a saved profile while a game runs, then switch back:

```sh
attack-shark-linux --profile "Name" -- <command> [args...]
```

For Steam, set the game's launch option to:

```sh
attack-shark-linux --profile "Name" -- %command%
```

The Games panel in the app shows this command with the full path of the
launcher, so it works even when `attack-shark-linux` is not on your `PATH`. For
the AppImage, the path is the AppImage file itself; if you move the AppImage the
path changes, so copy the command again. If the path cannot be determined, the
panel falls back to the bare `attack-shark-linux` form shown above.

`Name` is the exact, case-sensitive profile name; a 32-character profile ID also
works. The wrapper applies the profile, runs the command, waits for it, restores
the profile that was active before, and exits with the command's exit code
(127 if the command does not exist, 126 if it cannot be started). SIGTERM and
SIGHUP are forwarded to the command; Ctrl-C reaches it through the terminal. A
signal received before the command starts cancels the launch and puts the
profile back.

- If the desktop app is open, the wrapper does not touch the mouse: the app owns
  it, and the command simply runs.
- If the profile cannot be found, is ambiguous, or the switch fails, the wrapper
  prints a warning to stderr and launches the command anyway. Detection, reading
  the profiles and the switch share one 8-second budget before the launch, and
  the restore afterwards has its own 8 seconds, so the game is never held back
  longer than that.
- A switch that fails part-way (`apply_failed`) or times out is followed by a
  restore of the previous profile, since the mouse may be half written. The
  restore is skipped if the app was opened during the game.
- Switching to the already-active profile rewrites it.

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

Requirements: Go 1.27.1, Node.js 22 (for the frontend), and [Task](https://taskfile.dev).
The backend, CLI, and frontend runtime are pinned to Wails v3.0.0.
Install the matching CLI with `go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0`.
CI and both Ubuntu builders use Go 1.27.1; update the complete chain together,
not just `go.mod`. The builders copy the explicit upstream Go image toolchain
while retaining Ubuntu's GTK4 and WebKitGTK 6 native dependencies.

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
| [docs/user-guide.md](docs/user-guide.md) | Complete user guide and UI walkthrough with screenshots |
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
