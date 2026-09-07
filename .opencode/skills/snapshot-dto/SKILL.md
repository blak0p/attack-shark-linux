---
name: snapshot-dto
description: "Trigger: defining or returning data to the frontend, designing DTOs, converting domain types to API types, copying state for emission. Keep cross-boundary data immutable and decoupled from domain structs."
license: MIT
metadata:
  author: blak0p
  version: "1.0"
---

## Activation Contract
Load when adding a method that returns data to the UI, defining a desktop DTO,
or converting between domain (`x6`/`mouse`) types and the API surface
(`internal/desktop`).

## Hard Rules
- The desktop layer owns its **own** DTOs (`DPIConfig`, `Snapshot`,
  `PollingSnapshot`, `LightingSnapshot`, `Inventory`) — separate from the domain
  types in `x6`/`mouse`. The UI never receives raw domain structs.
- Conversions are explicit and centralized: `ToDTO`/`fromDTO` for DPI
  (`internal/desktop/service.go:1052`), and `*SnapshotOf` builders for the rest.
- Snapshots are taken **under the state lock** (`snapshotLocked`) and returned as
  value copies. The lock is released before any emission.
- Event payloads are immutable copies. `LightingEffects()` deep-copies the catalog
  so callers cannot mutate internal state (`internal/x6/lighting.go:72`).
- `nil` vs pointer semantics are deliberate: `*int`/`*x6.LightingSelection`
  distinguish "absent" from "zero" in `Snapshot`/`LightingSnapshot`.
- Never return a pointer to internal state, a locked struct, or a slice/map that
  aliases mutable internal memory.

## Frontier bar
- A frontier model treats the desktop DTOs as a stable API: additive changes only,
  no reshape of existing fields without a migration plan.
- It copies on read and on emit; aliasing bugs simply cannot occur.
- It keeps domain types (`x6.DPIConfig`) free of UI concerns and never adds JSON
  tags or presentation fields to them for the frontend's sake.
- It documents, per snapshot field, whether `nil` is meaningful.

## Decision Gates
| You need to… | Pattern |
| Expose domain config to UI | add a `*Config`/`*Snapshot` DTO + `ToDTO`/`fromDTO` |
| Read current state safely | `snapshotOf(state)` under `state.mu` |
| Emit a live update | copy snapshot, then `EventSink.Emit` after unlock |
| Represent optional value | use `*int`/`*T`, not sentinel zeros |

## Execution Steps
1. If the field is new, add it to the desktop DTO (never the domain type).
2. Add/or update `ToDTO`/`fromDTO` or the `*SnapshotOf` builder.
3. Take the snapshot under `state.mu`; build a value copy.
4. Unlock, then emit/copy; never hand out the locked struct.
5. Run `go build ./...` and regenerate frontend bindings if the DTO changed.

## Output Contract
Report the DTO/snapshot added or changed, the conversion function used, and
confirm no raw domain struct or locked state crosses the boundary.

## References
- [references/snapshot-dto.md](references/snapshot-dto.md) — real DTO/snapshot excerpts.
