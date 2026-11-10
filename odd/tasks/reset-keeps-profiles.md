# Emergency reset keeps profiles and app data
Follows T9 of `odd/tasks/per-game-profiles.md` (S8, L5, L22). Branch `feat/reset-keeps-profiles` in worktree `.git/wt/reset-profiles`, based on `origin/main` at `09a2ba9`.
Delivery: local work-unit commits only; push, PR and merge are the user's decisions.

## Specs

- S1 — The reset touches only the mouse. User: "porque hace tanto reset el reset deberia ser solo el raton no tray y autostart ni watcher eso es configuradcion de la app otra cosa es un boolean si o no eso no se lo tiene qeu llevar el reset". Verify the watcher rules, tray and autostart settings survive a successful reset unchanged.
- S2 — Macros and other profiles survive. User: "los macros no se tienen que borrar porque da igual que el mouse se resetee el macro deberia poder conservarse y yo cro que los perfiles tambien menos el que hiciste el reset porque lo hiciste por algo" and "pero porque se tendiran que borrar los perfiles al hacer un reset? al igual que las macros no?". Verify the macro library and every non-active profile survive unchanged, and no profile is deleted.
- S3 — The active profile goes to the reset baseline, whole payload. User chose "Resetear todo el payload (Recomendado)": remap, lighting and macros return to default, matching what was written to the mouse. Verify the active profile JSON holds the documented reset baseline (DPI, polling 1000, sleep, response time, lighting, default remap, no macro assignments), written only after all four lanes acknowledge; a write failure reports `persistence_failed` with retry available.
- S4 — No separate total reset. The user rejected deleting profiles or macros in any reset (S2 quote). There is a single reset.
- S5 — Same behavior from the CLI/headless reset as from the GUI.
- S6 — Copy and docs tell the truth: no text claims the reset clears local profiles or settings.

## Tasks

- T1 [S1-S5] route: inline; status: done (RED observed: watcher-v1.json deleted by reset; GREEN: go test ./..., -race on desktop and cmd, go vet); commit: `ede73f5`. Backend: replace the whole-directory purge with a reset persister that rewrites the active profile and the device record to the baseline; headless reset uses the same composition; update tests (`profiles_composition_test.go` pinned the old purge). Risk: item 1 (stops deleting and rewrites stored data), item 3 (changes the reset contract).
- T2 [S6] route: inline; status: done (frontend npm test 682 passed); commit: `b2b56fe`. UI copy (en/es), docs, and frontend tests for the new reset wording.
- T3 [S1-S6] route: inline + independent verify (high risk); status: in progress (verifier running). Full checks and close.

## Log

L1 — Original request: "quiero que hagas en un worktree quiero primero que planeemos de como conectamos los perfiles a el reset".
L2 — Planning: today `PurgeAll` removes the whole data dir (profiles, watcher-v1.json, macro library, tray/autostart), `profiles_composition_test.go:16` pins that `profiles/` is empty after reset, and the UI does not refresh profiles after reset.
L3 — User answers: S1 and S2 quotes; payload: "Resetear todo el payload (Recomendado)"; on the remaining data: "colectivo y una opcion para hacer reset total de el raton con bastante precaucion quiero decir macros perfiles y ya autostart es solo el toggle para eso sin reset eso fuera".
L4 — User: "pero porque se tendiran que borrar los perfiles al hacer un reset? al igual que las macros no?". The total-reset option is dropped (S4). Plan accepted: "me cierra hazlo bajo .git porfa".
L5 — Implementation: `StatePurger`/`PurgeAll` removed; `EmergencyReset` takes a `ResetPersister`, implemented by `Service.PersistReset` (device record + whole active profile payload). A damaged profile store is never overwritten; the reset then reports a retryable `persistence_failed`, pinned by a test. Legacy `applied-dpi.json`/`factory-defaults.json` are left untouched. The headless reset now uses `composeDesktopService` and `desktop.RunReset` (a package function, kept out of the Wails bindings). `openspec/changes/*` keeps the historical purge wording on purpose; `openspec/specs` never mentioned it. S5 is unproven on hardware.
L6 — Independent verify: S1-S4 and S6 met; S5 blocker: a headless reset on un-migrated v1 data saved the device record without running the migration, so every later GUI start reported `migration_failed` (the old purge deleted the legacy files). Fix: `runHeadlessReset` selects the mouse first (runs the migration, sends no commands). RED then GREEN with `TestHeadlessResetMigratesLegacyStateBeforeSavingTheBaseline`.
L7 — User: "si da pass pr y realese" and "y hay variso bugs arreglados e" (the release notes must include the fixes since v1.8.0).
