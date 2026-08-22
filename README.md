# Attack Shark Linux

Linux desktop configurator for the **Attack Shark X6** gaming mouse, built with
Go + [Wails v3](https://wails.io) (backend) and React + Vite (frontend).

> **Status**: Beta. There is no installer yet — build from source (see
> [Building](#building)). Not affiliated with or endorsed by Attack Shark.

## Features

- **Multi-device inventory** over kernel-backed hidraw, with selection by serial.
- **Live status monitoring** — battery heartbeats and physical DPI button
  changes are pushed to the UI as they arrive from the dongle.
- **DPI configuration** — up to 8 stages, 50–26000 in 50-unit steps, with
  per-stage colors and an active-stage circle UI.
- **Apply on demand** — writes the 56-byte configuration report over hidraw
  (`SET_REPORT`), with debounced auto-sync after one second of inactivity.
- **Per-device persistence** — versioned device profiles survive restarts,
  with factory-defaults restore.
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

## Building

Requirements: Go 1.25+, Node.js (for the frontend).

```sh
# Backend + embedded frontend
task build                # taskfile at cmd/x6configurator

# Or manually:
go build ./...
(cd frontend && npm ci && npm run build)
```

## Testing

```sh
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
