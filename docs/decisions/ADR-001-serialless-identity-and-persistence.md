# ADR-001 — Persist serial-less X6 receivers by VID:PID

| | |
|---|---|
| **Status** | Accepted (maintainer decision, 2026-08-18) |
| **Date** | 2026-08-18 |
| **Scope** | Durable persistence for serial-less receivers only (no product features) |
| **Supersedes** | The earlier topology + human-claims proposal for serial-less identity (never approved) |

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

> An earlier proposal keyed serial-less receivers by USB topology (bus + nested
> ports) plus a human-confirmed claim. It was **discarded before approval**: it
> solved a two-identical-receivers scenario that the backend already rejects by
> design (`commandCandidate` refuses more than one validated X6 device) and
> cost a claims store, a confirmation UI, and port-move re-confirmation flows.

---

## 2. Decision

For **serial-less** receivers, persist by `VID:PID` alone — a model-level key.

```
Has serial?
     │
     ├─ YES ──►  key = VID : PID : serial        (unchanged, stable)
     │
     └─ NO  ──►  key = VID : PID                (new, model-level)
```

- `DeviceID.Validate()` accepts an empty serial (VID:PID non-zero);
  `DeviceID.Key()` returns `1d57:fa60` when serial is empty and
  `1d57:fa60:<serial>` otherwise.
- The `sessionOnlyID` path is removed; a serial-less candidate flows through the
  normal eligible path with `ID = {1d57, fa60, ""}` and is persisted exactly like
  a serial-bearing device.
- `Binding.SessionOnly` is always `false`; the field stays on the struct so the
  generated Wails bindings do not need regeneration.
- Existing migration (v1 → `devices-v2.json`) and the ACK-before-save
  persistence path apply to the serial-less device unchanged.

### 2.1 Rationale

- A single receiver is the real usage: one person, one PC, one mouse. Almost
  nobody connects two identical receivers at once, and when they do, they are
  testing, not configuring.
- A replaced or borrowed X6 inherits the saved configuration — reasonable
  behavior, not a bug.
- Port moves stop mattering: the key carries no topology, so nothing needs
  re-confirming.
- Two identical serial-less receivers connected simultaneously would share the
  `1d57:fa60` key. This is an **accepted tradeoff**: the current backend already
  surfaces that situation as `ambiguous identity` and rejects Apply.

---

## 3. Consequences

- ✅ The serial-less X6 now remembers its configuration across reconnects.
- ✅ Change DPI → ACK → persisted under the `1d57:fa60` key.
- ✅ A port move or hub change no longer affects identity.
- ✅ Serial-bearing receivers keep their per-device behavior exactly.
- ⚠️ Two identical serial-less receivers at once share one configuration
  (accepted; rare and already guarded as ambiguous).
- ✅ Net removal of machinery: no claims store, no topology, no confirmation UI.

## 4. Non-goals

- ❌ Per-device identity for serial-less receivers (no topology, no claims).
- ❌ Claim confirmation UI or rebind flows.
- ❌ Per-stage color or other product features (persistence only).
- ❌ Regenerating Wails bindings (the `SessionOnly` field is retained).

---

## 5. References

| File | Role |
|---|---|
| `internal/mouse/profile.go` | `DeviceID`, `Validate`, `Key` (serial optional) |
| `internal/mouse/service.go` | Inventory, serial-less now a regular eligible device |
| `internal/configstore/device_store.go` | v2 store, `Save`/`Load` by `DeviceID.Key()` |
| `internal/desktop/service.go` | `DevicePersistence`, ACK-before-save path |
| `docs/protocol-x6.md` | `0x04` write-only protocol evidence |
| Issue #26 | Approved plan for this change |