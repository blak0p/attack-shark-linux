# SDD Data Briefing — serialless-durable-identity

> This briefing is the SOLE input for the SDD session. Do not re-investigate the
> codebase, do not invent scope. If a fact is missing, ask the maintainer.

## 1. Context

- Project: attack-shark-linux (Wails v3 + Go + React configurator for Attack Shark X6).
- Baseline: `main` at local `a54ff23` (one commit ahead of `origin/main` — ADR-001, not yet pushed). Clean worktree.
- Date: 2026-08-17.
- Why: the maintainer's X6 receiver has no USB serial, so current persistence
  (keyed `VID:PID:serial`) never saves it. The user wants the configuration to
  be saved, restored on reconnect, and versioned feature-by-feature later.

## 2. Current code state (verified at HEAD)

What exists today:

- `internal/mouse/profile.go` — `DeviceID{VendorID, ProductID, Serial}`; `Validate()` fails without serial; `Key()` = `%04x:%04x:%s`.
- `internal/mouse/service.go` — inventory classifies serial-less as
  `session-only (no serial)`; `sessionOnlyID()` derives a `session-<hash>` serial
  from profile HID facts (NOT topology); `Binding.SessionOnly` flag.
- `internal/configstore/device_store.go` — schema **v2** (`devices` +
  `migrations` only, `deviceSchemaVersion = 2`); `Save`/`Load` by `DeviceID.Key()`.
- `internal/desktop/service.go` — `DevicePersistence` interface; ACK-before-save
  invariant: persistence runs only after a successful HID ACK;
  `SessionOnly` bindings skip persistence and migration.
- `cmd/x6configurator/main.go` — composition wires store, device store
  (`devices-v2.json`), migrator, and per-device load/save.

What does NOT exist (do not assume it):

- ❌ Topology-based identity (bus + nested ports) in the identity model.
- ❌ Claim store v3 (`claims` map) in `main` (`deviceEnvelope` has no claims field).
- ❌ Claim resolution (exact match / new / ambiguous).
- ❌ Claim lifecycle API in `internal/desktop` or Wails bindings.
- ❌ Frontend claim confirmation UI.

Discarded prior work:

- `serialless-x6-persistence` (2026-08-17): implemented claims-by-topology v3 +
  claim creation/rebind UI + desktop wiring; PRs #23-#25 and all local work were
  discarded by maintainer decision because the flow contradicted the archived
  scope and got tangled. Surviving residue: `~/.config/attack-shark-linux/
  devices-v2.json` still contains a v3 claim record (`ce9eb22a...`, alias "X6
  Receiver", `topology {bus:1, ports:[4]}`) — **not** in `main`; treat as
  reference evidence only. The v3 claim idea is reused by ADR-001, the
  claim-creation UI is NOT.

## 3. Validated evidence (byte-exact)

Live hardware validation on 2026-08-17, receiver `1d57:fa60` on `hidraw3`.

| Observed behavior | Bytes / value | Condition | Source |
|---|---|---|---|
| Enumerated receiver (profile match) | VID:PID `1d57:fa60`, `serial_present=false`, hidraw `hidraw3` | sysfs enumeration | `/tmp/opencode/x6probe` |
| Passive battery heartbeat | `03 10 40 <pct*10>` (5 B) | ~every 2.1 s idle | x6probe output (29 events, 100%) |
| Physical DPI stage event | `03 10 10 <stage>` (5 B) | DPI button press | x6probe output (stages 2→3→4→5→6) |
| Full configuration write | `04 38 01 ...` 52 B (DPI stage 3 = 1600→1650) | reversible apply | `/tmp/opencode/x6write apply` |
| Firmware ACK after valid write | `03 10 50 00 04` (5 B) | immediate response | x6write output (`apply ACK OK`) |
| Firmware ACK after restore | `03 10 50 00 04` (5 B) | baseline re-send | x6write output (`restore ACK OK`) |
| Claim residue from discarded work | `claim ce9eb22a...` alias "X6 Receiver", `topology {bus:1, ports:[4]}` | disk file | `~/.config/attack-shark-linux/devices-v2.json` |

- Commands: `GOTOOLCHAIN=local GOPROXY=off go build -o /tmp/opencode/x6probe ./cmd/x6probe`
  (probe later removed from source); `x6write apply` / `x6write restore`.
- Read-only evidence: enumeration, battery, stage events, claim file read.
- Write evidence (consent given): reversible +50 DPI on stage 3, then restore.
- NOT observed (unknown, do not guess): whether multiple identical serial-less
  receivers present simultaneously collide in the current session ID; whether
  any topology can be read from sysfs in a hub chain (ports nesting) — must be
  verified during implementation.

## 4. Decisions (ADR links)

- **ADR-001** (`docs/decisions/ADR-001-serialless-identity-and-persistence.md`)
  — Approved, committed `a54ff23` (not yet pushed).
  - Second coexisting identity: `VID:PID:serial` (unchanged) OR
    `VID:PID:topology` (new, bus + nested ports) for serial-less.
  - Claim = human bridge; no persistence without explicit confirmation.
  - Never overwrite: unmatched topology is never loaded or overwritten.
  - Port move = no assumption: ask again.
  - Fail closed: ambiguity/unknown → session-only + ask user.
  - Split: backend and frontend as two separate work units.
- Rules the SDD must respect: ACK-before-save invariant stays; tests with code;
  Go artifacts in English; no generated bindings by `go run` (wails3 CLI must be
  installed via `go install .../cmd/wails3@v3.0.0-beta.5` into GOBIN/PATH).

## 5. Scope

Goals (this change = backend unit of ADR-001):

- Topology-derived identity for serial-less candidates (bus + nested ports).
- `DeviceStore` v3: `claims` map + `devices` keyed by claim; load/save/validate.
- Claim resolution: exact topology match → load; none → `new_claim_required`
  (metadata only); multiple → `selection_required`.
- `internal/desktop` claim lifecycle API: propose/confirm/reject/load/save,
  preserving ACK-before-save.
- `cmd/x6configurator` + Wails bindings exposing the claim API.
- Tests: Go unit tests for identity, store v3, resolution, ACK-before-save on
  claims; `go test ./...` green.

Non-goals (MUST NOT deliver or infer):

- ❌ Frontend claim confirmation UI (separate frontend work unit).
- ❌ Rebind / identical-receiver selection flows beyond minimal confirm.
- ❌ Per-stage colors, polling Hz, or any product feature.
- ❌ Resurrecting the discarded claim-creation UI beyond this briefing.

## 6. Risks and open questions

- `devices-v2.json` on disk is v3 with a claim; the v2 reader may ignore or
  misread it. Migration/backward-compat decision needed before v3 writer lands.
- wails3 CLI unavailable in worktrees: bindings generation requires
  `go install` (never `go run`, which mutates go.mod/go.sum).
- Topology is only stable while the receiver stays on the same port path; a hub
  chain ports array must be verified from sysfs during implementation.
- Main is 1 commit ahead of origin; push ADR-001 before/with delivery.
- Open: what alias default to propose ("X6 Receiver") and whether auto-propose
  is acceptable for a single receiver (ADR-001 allows proposal, requires
  confirmation).

## 7. Handoff

- Suggested change name: `serialless-durable-identity`.
- Constraints: backend unit only (frontend unit separately later); keep
  `VID:PID:serial` path unchanged; ACK-before-save; Go tests with each unit;
  English technical artifacts; do not create PRs/branches without maintainer
  approval; SDD session must start from this briefing and NOT re-investigate.
- Suggested SDD entry: run `sdd-data-briefing` output handoff → then
  `/sdd-new serialless-durable-identity` in a fresh session.
