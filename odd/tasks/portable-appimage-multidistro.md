# Portable AppImage and Multi-Distro CI Verification

Repository-relative file locator: `odd/tasks/portable-appimage-multidistro.md`

## Objective

Ensure `attack-shark-linux-x86_64.AppImage` executes portably across any Linux distribution without crashing on WebKitGTK helper paths, provide complete desktop integration including the SVG icon, and add a multi-distribution verification matrix in CI.

## Problem and evidence

1. **WebKit startup crash on non-Debian distributions:**
   - In production observation #4796, running `v1.2.0` on CachyOS (Arch-based) fails immediately with:
     `Unable to spawn a new child process ... /usr/lib/x86_64-linux-gnu/webkitgtk-6.0/WebKitNetworkProcess (No existe el fichero o el directorio)` followed by SIGTRAP.
   - The AppImage is built in Ubuntu 24.04 and packages `WebKitNetworkProcess` and `WebKitWebProcess` under `$APPDIR/usr/lib/x86_64-linux-gnu/webkitgtk-6.0/`, but WebKit's default lookup path looks on the host filesystem when `WEBKIT_EXEC_PATH` is not explicitly set.
2. **Missing SVG icon in desktop integration:**
   - In `install.sh`, the generated `~/.local/share/applications/attack-shark-x6.desktop` lacks an `Icon=` key, and the SVG icon is not installed into the user's icon directory (`~/.local/share/icons/hicolor/scalable/apps/`).
3. **No multi-distribution testing in CI:**
   - Workflows only verify execution on `ubuntu-latest`, allowing distribution-specific path assumptions to escape to release assets.

## Scope

- **Packaging runtime hook:** In `cmd/x6configurator/Taskfile.yml` and AppImage build assets, inject an AppRun runtime hook that sets `WEBKIT_EXEC_PATH="$APPDIR/usr/lib/x86_64-linux-gnu/webkitgtk-6.0"` (or AppDir-relative equivalent).
- **Desktop & Icon integration:** Update `install.sh` and release asset pipeline to install `attack-shark-x6.svg` into `$account_home/.local/share/icons/hicolor/scalable/apps/attack-shark-x6.svg` (and `$install_dir/attack-shark-x6.svg`), and write `Icon=attack-shark-x6` into the desktop entry.
- **Installer tests:** Update `cmd/release/installer_test.go` and release tooling to validate desktop icon configuration and asset contracts.
- **Multi-distro CI workflow:** Add a container-based matrix verification job (Arch Linux, Fedora, Ubuntu) to validate AppImage execution.

## Constraints and non-goals

- Do not require host-level installation of WebKitGTK development packages on end-user machines.
- Maintain existing cryptographic signing, release manifest format, and least-privilege udev policies.
- Keep `install.sh` POSIX-compatible and shellcheck clean.
- Do not modify HID communication or device mapping logic.

## Delivery strategy

- Forecast: ~180–250 authored changed lines across packaging, scripts, workflow, and Go tests.
- Strategy: `ask-on-risk`; within heuristic threshold.
- Effective TDD mode: Go tests (`go test ./cmd/release/...`, `go test ./...`) and shellcheck.

## Tasks

- [x] **PORT-1 — Inject WEBKIT_EXEC_PATH runtime hook in AppImage packaging.** (Route: delegated)
- [x] **PORT-2 — Fix install.sh SVG icon installation and desktop entry.** (Route: delegated)
- [x] **PORT-3 — Add multi-distro container matrix verification to CI workflow.** (Route: delegated)
- [x] **PORT-4 — Full verification and work-unit commits.** (Route: direct)

## Acceptance criteria and checks

- AppImage packages an AppRun hook exporting `WEBKIT_EXEC_PATH` pointing to the bundled WebKitGTK helpers inside `$APPDIR`.
- `install.sh` installs the SVG icon and includes `Icon=attack-shark-x6` in the desktop entry.
- `cmd/release/installer_test.go` enforces the icon and desktop contract.
- CI includes a multi-distro smoke-test matrix (Arch, Fedora, Ubuntu) verifying AppImage portability.
- All Go unit tests, frontend tests, and shell validations pass.

## Current progress and next step

- PORT-1 complete: `packaging/appimage/prepare-appdir.sh` ensures `WebKitNetworkProcess` and `WebKitWebProcess` are executable (0755), compiles `packaging/appimage/webkit-relocate.c` into `$APPDIR/usr/lib/libwebkit-relocate.so` (intercepting `execve`/`execv`/`execvp`/`execvpe`/`posix_spawn`/`posix_spawnp` to redirect hardcoded Debian `/usr/lib/x86_64-linux-gnu/webkitgtk-6.0/*` helper paths to `$APPDIR`), wraps binary AppRun with `AppRun.wrapper` exporting `LD_PRELOAD`, installs executable hook `apprun-hooks/01-webkit-exec-path.sh`, and updates `cmd/x6configurator/Taskfile.yml` packaging tasks (`package`, `package:release`, `package:container`, `package:container:release`). Verified empirically on Arch Linux container and via `cmd/release/appdir_portable_test.go`.
- PORT-2 complete: `install.sh` extracts the SVG icon from AppImage (`.DirIcon` / `attack-shark-x6.svg`), installs it into `$account_home/.local/share/icons/hicolor/scalable/apps/attack-shark-x6.svg` and `$install_dir/attack-shark-x6.svg` with 0644 permissions, adds `Icon=attack-shark-x6` to `.desktop`, and all `cmd/release/installer_test.go` contract tests pass.
- PORT-3 complete: structured `.github/workflows/release.yml` into a 3-stage pipeline (`build`, `smoke-test-distros`, `release`). The `smoke-test-distros` job validates the packaged AppImage across a Docker matrix (`ubuntu:24.04`, `archlinux:latest`, `fedora:latest`) using `packaging/appimage/smoke-test.sh` under `xvfb-run` with timeout detection, asserting no WebKit helper process spawn crash. GitHub release publication is strictly conditioned on `needs: [build, smoke-test-distros]` and uses canonical asset paths. Enforced and validated via `TestReleaseWorkflowEnforcesMultiDistroMatrixSmokeTest` in `cmd/release/workflow_test.go` and `actionlint`.
- PORT-4 complete: full test suites passed (`go test ./cmd/release/...`, `frontend vitest`, `actionlint`, POSIX shell check), podman containers pruned, all work ready for conventional commit on `fix/portable-appimage-multidistro`.

