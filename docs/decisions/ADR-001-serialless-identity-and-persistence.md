# ADR-001 — Durable identity and persistence for serial-less X6 receivers

| | |
|---|---|
| **Status** | Proposed (pending maintainer approval) |
| **Date** | 2026-08-17 |
| **Scope** | Durable identity + persistence only (no product features) |
| **Split** | Backend and frontend as two separate work units (maintainer decision) |

---

## 1. Context

The maintainer's Attack Shark X6 receiver is `1d57:fa60` with
`serial_present=false` — the device exposes **no USB serial number**.

Today, persistence keys every device record by `VID:PID:serial`
(`internal/mouse/profile.go`). A serial-less receiver therefore **can never be
saved**: inventory classifies it as `session-only (no serial)`
(`internal/mouse/service.go`) and the desktop service skips persistence for
`SessionOnly` bindings by design.

```
Current key ──►  VID : PID : serial
                     │           │
                same for all    ✗ MISSING on this receiver
                     │           │
                     └───────┬───┘
                             ▼
               No durable key  →  nothing is ever saved
```

Live hardware validation (2026-08-17) proved the rest of the stack works:

| Check | Result |
|---|---|
| Passive status (battery heartbeats, DPI stage events) | ✅ live |
| Full `0x04` write (52 B) with firmware ACK | ✅ live |
| Restore of stored baseline with ACK | ✅ live |

The Windows official app never reads configuration back — it only pushes the
complete `0x04` report at startup and after every change, so durable state is
**local by nature**.

> ⚠️ The previous `serialless-x6-persistence` attempt was discarded wholesale.
> The claim-store format (devices-v2.json v3) survives on disk as residue but is
> **not** in `main`; current `main` is schema v2 (`devices` + `migrations`
> only). This ADR redefines the design cleanly.

---

## 2. Decision

Add a **second, coexisting durable identity** for serial-less receivers:

```
Has serial?
     │
     ├─ YES ──►  key = VID : PID : serial        (unchanged, stable)
     │
     └─ NO  ──►  key = VID : PID : topology      (new)
                        │
                        └── bound to a human-confirmed claim
```

### 2.1 Topology is the identity source

The key is the **physical USB path**: `bus` plus the nested `ports` array
(hub-aware), e.g. `bus:1, ports:[1,4]`.

> HID facts and connection type are deliberately **not** part of the key: two
> identical receivers would collide.

### 2.2 A claim is the human bridge

Persistence happens **only after the user explicitly confirms** "this topology
is this receiver" (alias). Without a confirmed claim, nothing is loaded and
nothing is saved.

### 2.3 Never overwrite

A topology that matches no claim is **never loaded and never overwrites**
anything. It is offered as a new claim, or surfaced as ambiguous when more than
one candidate exists.

### 2.4 Port move = no assumption

If the topology changes, the app asks again. It must never silently adopt
another device's configuration.

### 2.5 Fail closed

Any ambiguity, unknown outcome, or missing claim leaves the device in a
session-only state and asks the user. No defaults, no inference.

---

## 3. Work split — backend and frontend

Two **separate work units**, as the maintainer requested. Each keeps its own
tests, its own change, and its own review surface. The frontend depends on the
backend API and never reimplements it.

### 3.1 Backend — identity, store, resolution, service API

| Area | File(s) | Responsibility |
|---|---|---|
| Identity model | `internal/mouse` | Extend `DeviceID` with a topology-derived identity for serial-less candidates (`bus` + nested `ports`); serial identity untouched |
| Store v3 | `internal/configstore` | Evolve `DeviceStore` to v3: `claims` map (`claimID → {alias, validatedProfile, topology}`) beside `devices` (`claimID → DeviceRecord`); load/save/validate by claim |
| Resolution | store or small resolver | exact topology match → load that claim; no match → `new_claim_required` (metadata only, never another device's payload); multiple matches/candidates → `selection_required` |
| Service API | `internal/desktop` | Claim lifecycle: propose from topology, confirm (persists), reject (session-only), load/save bound to the confirmed claim; keep the ACK-before-save invariant |
| Bindings | `cmd/x6configurator` + Wails bindings | Expose the claim API to the frontend |

**Acceptance:** Go tests for identity, store v3, resolution cases, and
ACK-before-save on claims; `go test ./...` green.

### 3.2 Frontend — claim confirmation UI only

| Area | File(s) | Responsibility |
|---|---|---|
| First connection | `frontend/src` | "This receiver is new — confirm and name it" |
| Topology changed / ambiguous | `frontend/src` | Ask which receiver, or whether to re-confirm |
| Confirmed claim | `frontend/src` | Show alias + loaded configuration; persistence feedback after save |

No frontend identity logic, no direct file writes, no persistence rules.

**Acceptance:** frontend tests for the claim flows; `npm test --prefix frontend`
green.

---

## 4. Consequences

- ✅ Maintainer's daily flow: connect on the same port → app loads the saved
  configuration; change DPI → ACK → persisted to that claim.
- ✅ Two identical serial-less receivers never share or overwrite each other's
  configuration.
- ✅ A port move costs one re-confirmation instead of a silent wrong load.
- ✅ Existing serial-bearing receivers keep their current behavior exactly.

## 5. Non-goals

- ❌ Rebind / identical-receiver selection flows beyond the minimal confirm flow.
- ❌ Per-stage color or other product features (identity/persistence only).
- ❌ Any resurrection of the discarded claim-creation UI beyond this ADR.

---

## 6. References

| File | Role |
|---|---|
| `internal/mouse/profile.go` | `DeviceID`, `Validate`, `Key` |
| `internal/mouse/service.go` | Inventory classification, `sessionOnlyID` |
| `internal/configstore/device_store.go` | v2 store, `Save`/`Load` |
| `internal/desktop/service.go` | `DevicePersistence`, ACK-before-save path |
| `docs/protocol-captures.md` | `0x04` write-only protocol evidence |
