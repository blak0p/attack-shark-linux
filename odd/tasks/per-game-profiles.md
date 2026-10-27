# Per-game profiles: launch wrapper and tray watcher
Objective: switch the mouse profile per game without a system service. Two independent deliverables: A) launch wrapper CLI, B) tray app with /proc watcher. Branch: not created yet; delivery strategy `ask-on-risk`; test runner: Go `go test ./...`, frontend Vitest, `make check`. Never write to real hardware in automated checks.

## Specs
S1. Wrapper command (A): `attack-shark-linux --profile <name> -- <command>`. Used as a Steam launch option (`... -- %command%`) and in other launchers. It applies the named profile, runs the command, waits for it, restores the previous profile, and returns the command's exit code. If the profile cannot be resolved or switched, it prints a warning and launches the game anyway; it must never block the game.
S2. Name resolution (A): the profile is selected by name (exact match, case-sensitive). Not found or ambiguous is a warning, not a failure of the game. A raw 32-hex profile ID is accepted as an escape hatch.
S3. Unique profile names (A prerequisite): Create, Rename and Duplicate must reject a name already in use. Existing duplicate names on disk must not break loading.
S4. App-running lock (A and B): the app takes an flock on a lock file at startup and releases it on exit (kernel releases it if the app crashes). The wrapper checks it. App open: the wrapper does not touch the mouse, it only launches the game, because the app owns the mouse. App closed: the wrapper applies the profile itself. Rule from the user: “depende si esta desde la app le hace caso a la app si no nada”.
S5. Switching to the already-active profile rewrites it anyway (accepted by the user: “lo mas facil lo rescribe”).
S6. Tray app with watcher (B, second phase): opt-in setting, off by default. When on, closing the window keeps the app in the system tray (Wails `Linux.DisableQuitOnLastWindowClosed` plus `SystemTray`). A selector in the app maps an executable (or launcher entry) to a profile. The app scans /proc every few seconds, applies the mapped profile when the executable appears and restores the previous one when it exits. Autostart is an opt-in toggle that writes/removes `~/.config/autostart/*.desktop`; nothing is installed system-wide and no systemd unit is used. If no tray exists (Hyprland without Waybar tray module), the window minimizes and the app stays alive.
S7. Memory footprint (decision): accept ~300 MB for the full app, no WebKit purge patch, no separate watcher binary. Trim frontend size before the release.
S8. Existing `reset` stays as is; connecting `reset` to profiles is a later task before the release (see T9).

## Tasks
T1 | S3 | inline | unique profile names in configstore + tests | pending
T2 | S2 | inline | name->id resolver (not found / ambiguous / hex ID) + tests | pending
T3 | S4 | inline | flock lock file taken by the app, checked by CLI + tests | pending
T4 | S1,S5 | inline | CLI parsing `--profile NAME -- cmd`, headless composition via composeDesktopService + RefreshInventory, run command, restore, exit code, help text and README | pending
T5 | S1 | manual | hardware check with the real mouse (never automated) | pending
T6 | S6 | inline | tray + DisableQuitOnLastWindowClosed + minimize fallback, opt-in setting | pending (phase 2)
T7 | S6 | inline | executable->profile selector UI + /proc watcher + restore | pending (phase 2)
T8 | S6 | inline | autostart toggle (.desktop in ~/.config/autostart) | pending (phase 2)
T9 | S8 | inline | connect emergency `reset` to profiles (emergencyReset lacks AttachProfiles) | pending (before release)
T10 | S7 | inline | trim frontend size (about 45 MB at most) | pending (before release)

## Open questions (unverified)
- Q1. Receiver reports `serial_present=false` and `SwitchProfile` refuses SessionOnly bindings (no serial) with ProfileUnavailable; the app already switches profiles, so verify why headless would differ (check in T4).
- Q2. A corrupt saved profile blocks GetProfiles and therefore all switching; decide behavior for the wrapper (warn and launch).
- Q3. Tray visibility on Hyprland depends on the user's bar having a tray module.
- Q4. SwitchProfile uses context.Background() without timeout; the wrapper needs its own timeout so the game is not delayed indefinitely.

## Evidence
- Explorer mvqykxqh: no name lookup exists (internal/configstore/profiles.go:73,106,255,288); names are not unique; `composeDesktopService` (cmd/x6configurator/main.go ~229-290) is already headless with profiles; `emergencyReset` (main.go:96-110) omits AttachProfiles; no single-instance or lock exists; hidraw opens O_RDWR without flock (internal/hidlinux/hidraw_passive.go:41-42); profile store says concurrent processes writing the same root are unsupported.
- PoC /tmp/wails-poc (Wails v3.0.0): tray only 47 MB; window open ~240 MB; Close() does not free WebKit; SIGTERM purge works (120 MB) but was rejected as a patch. Installed AppImage measured at ~300 MB PSS.
- Prior art: gamerat and Autoratbag are daemons; Piper issues #343 and #569 request this feature.

## Log
L1. 2026-10-27 user idea: per-game profiles, auto-switch the mouse profile by running game (Engram obs 5777): “hay que mirarlo bien”.
L2. User constraints: used by real users, “lo mas limpia posible sin ser invasivo no me gustan los servicios”; uses Hyprland; wants “como los software buenos de windows” but very light; later accepted ~300 MB.
L3. User: “seria el selector de profiles por ejecutable juego acceso lo que quiera” and “es con el servicio que mira proc para aplicarlo”; and the wrapper for Steam launch options (“casi todos los launchers lo tienen hacemos las dos”).
L4. User: “depende si esta desde la app le hace caso a la app si no nada”; the wrapper and the app are two independent things.
L5. User: later, “que el reset este conectado a los perfiles” and unique profile names, then release.
