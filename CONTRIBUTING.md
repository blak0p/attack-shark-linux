# Contributing to attack-shark-linux

Thanks for your interest! We're glad you're here.

Please note that this project is governed by a [Code of Conduct](CODE_OF_CONDUCT.md).
By participating, you agree to uphold its terms.

## Table of Contents

- [What we're working on](#what-were-working-on)
- [Good first issues](#good-first-issues)
- [Setup](#setup)
- [Development workflow](#development-workflow)
- [Tests](#tests)
- [Project structure](#project-structure)
- [Commit conventions](#commit-conventions)
- [Pull request process](#pull-request-process)
- [Review process](#review-process)
- [Reporting issues](#reporting-issues)
- [Security](#security)
- [Getting help](#getting-help)

## What we're working on

Check [GitHub Issues](https://github.com/blak0p/attack-shark-linux/issues) for
open tasks. The project is a Linux desktop configurator for the Attack Shark X6
gaming mouse (Go + Wails v3 backend, React frontend). High-level directions:

- Protocol coverage: remap (`0x08`), lighting, polling, sleep
- Macro manager
- Installer / packaging (not available yet)
- Live status features (battery, DPI switching)

## Good first issues

If you're new to the project, look for issues labeled
[`good first issue`](https://github.com/blak0p/attack-shark-linux/labels/good%20first%20issue).
These are smaller, well-scoped tasks that don't require deep knowledge of the
codebase.

## Setup

**Requirements:** Go 1.25+ · Node.js · the X6 dongle (for manual device testing).

```sh
git clone https://github.com/blak0p/attack-shark-linux.git
cd attack-shark-linux
```

The hidraw tests run against a fake device — no hardware needed:

```sh
go build ./...
go test ./...
```

To test against the real dongle, install the udev policy first
([docs/linux-usb-prerequisites.md](docs/linux-usb-prerequisites.md)).

## Development workflow

1. **Pick an issue** or create one to discuss your change first. New issues are
   auto-labeled `status:pending-approval`; a maintainer reviews and approves
   them (`status:approved`) before implementation.
2. **Create a branch** from `main`: `git checkout -b feat/my-change`
3. **Make changes** with tests
4. **Run tests locally** before committing
5. **Open a PR** with a clear description and a link to the issue
6. **Address review feedback** if any

Branch naming:

| Pattern | Example |
|---------|---------|
| `feat/<name>` | `feat/macro-manager` |
| `fix/<name>` | `fix/hidraw-node-lifecycle` |
| `docs/<name>` | `docs/protocol-x6` |
| `chore/<name>` | `chore/update-deps` |

## Tests

**Tests must pass before merge.** CI enforces this automatically.

```sh
# Backend
go test ./...        # unit tests (fake hidraw, no device needed)
go vet ./...         # static analysis

# Frontend
(cd frontend && npm ci && npm test)
```

### Test conventions

- Backend unit tests live next to the code under test as `*_tdd_test.go`
  (behavior-first), plus focused integration tests where relevant.
- The `probe` build tag gates hardware-probe tooling — do not run it in CI.
- hidraw tests use a fake device; tests that need the real dongle are manual.

## Project structure

```
attack-shark-linux/
├── cmd/x6configurator/       # Wails app entry point + embedded frontend dist
├── frontend/                 # React + Vite UI
├── internal/
│   ├── configstore/          # Versioned per-device config persistence
│   ├── desktop/              # Desktop service, sync coordinator
│   ├── hidlinux/             # hidraw adapter, uevent, passive status
│   ├── mouse/                # Device profile + inventory
│   ├── protocol/             # Protocol-domain helpers
│   ├── transport/            # HID candidate matching
│   └── x6/                   # X6 DPI config, status events, reports
├── packaging/udev/           # udev access policy
├── captures/                 # Raw .pcapng protocol evidence
└── docs/                     # Protocol, UI map, capture plan
```

## Commit conventions

```
type: short description (max 72 chars)

Optional body — explain WHY, not WHAT.
```

Types: `feat` `fix` `refactor` `chore` `test` `docs` `perf` `ci`

Breaking changes: add `!` after type (`feat!:`) or `BREAKING CHANGE:` in the
body.

## Pull request process

1. Branch from `main`
2. Make changes with tests
3. Run the checks locally (see [Tests](#tests)) — all must pass
4. Open a PR using the [PR template](.github/PULL_REQUEST_TEMPLATE.md) and link
   the issue it addresses (`Closes #123`)

### PR checklist (required)

- [ ] `go build ./...` compiles without errors
- [ ] `go test ./...` unit tests pass
- [ ] `go vet ./...` passes
- [ ] `(cd frontend && npm test)` passes if the frontend changed
- [ ] Followed [conventional commits](https://www.conventionalcommits.org/)
- [ ] Updated documentation if applicable

**If tests fail, the PR will not be merged.** No exceptions.

## Review process

- PRs need at least **one approval** from a maintainer
- PRs must reference an approved issue (`status:approved`) — CI enforces it
- All conversations must be **resolved** before merge
- **Stale reviews are dismissed** automatically when new commits are pushed
- Reviews focus on correctness, test coverage, and hardware-safety (no writes
  beyond what the UI asked for)

## Reporting issues

Use [GitHub Issues](https://github.com/blak0p/attack-shark-linux/issues).
Include:

- OS/distro and Go version
- Whether the udev policy is installed (`lsusb -d 1d57:fa60`)
- Device details (dongle firmware, serial availability) if relevant
- Steps to reproduce
- Expected vs actual behavior

New issues are auto-labeled `status:pending-approval`.

## Security

For security vulnerabilities, see [SECURITY.md](SECURITY.md) — **do not** open
a public issue. Never attach proprietary capture material from the official
Windows app to public reports.

## Getting help

- **Discussions**: [github.com/blak0p/attack-shark-linux/discussions](https://github.com/blak0p/attack-shark-linux/discussions)
- **Issues**: For bugs and feature requests