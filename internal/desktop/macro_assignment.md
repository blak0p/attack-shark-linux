# Desktop macro assignment contract

## Local draft → explicit Apply remap

The shared saved library remains independent of device assignments. For XM-5c,
list saved macros by name in Button remapping, then call
`StageMacroAssignment(id, button, repeat)`. There is no standalone upload action.
Logical buttons 1–7 are accepted; fixed repeat is an integer from 1 through 255.
Admission requires exactly two left or right events, same button, down then up,
both with zero local delay. Unsupported sequences/timing are rejected without
changing saved data or replacing a valid draft.

`GetRemapSnapshot` and `GetMacroAssignmentSnapshot` return the ordinary remap
snapshot plus `MacroPending`, `MacroApplied`, and `MacroProgress`. All event
slices are defensive copies. A draft records saved ID/name, target button,
repeat, and exact local event snapshot. One draft per selected device replaces
the previous draft when moved; it is not an inventory of independent slots.

- `StageRemap(config)` changes only ordinary pending fields, without hardware I/O.
- `ClearMacroAssignment()` removes the overlay, retaining ordinary pending fields.
  Use this when returning the macro target to an ordinary action.
- `DiscardRemap()` restores ordinary applied fields and clears the macro draft.
  It never resurrects a previously applied macro.
- None of staging, clearing, discarding or snapshot reads writes hardware.
  Selection follows existing desktop semantics; admission requires a selected
  binding. Library CRUD remains usable without any selection.

## Apply and evidence

`ApplyRemap(config)` uses the captured selected binding and existing operation
and per-device apply guards. With a macro draft it calls the typed mouse
`ApplyMacroAssignmentBound`: report08 assignment/ACK followed by destination-aware
report09 upload/final status. The existing inventory command supports this via
`TargetedMacroCommand`; the shared hidraw backend already implements that seam,
so composition needs no alternate transport or startup hardware operation.
Without a macro draft, the existing typed ordinary remap operation and local
persistence behavior remain unchanged.

The full ordinary config is preserved beneath the overlay, including unrelated
button fields. The ordinary `Applied` config is not itself a decoded macro
report: consult `MacroApplied` alongside it. Macro completion is not persisted
as an ordinary remap configuration (`Persistence = "not_supported"`), because
that schema cannot describe the assignment/events. No applied library identity
is discovered from hardware; `MacroApplied` is only this session's acknowledged
local attempt.

Library ID/name/events must still match before I/O. Edits or deletion require
explicit restaging. Transport receives a frozen copy, so an in-flight library
mutation can never substitute different events or silently discard timing.
Completion rechecks library contents, binding and draft revision; stale
completion cannot advance local applied state. A mutation during physical I/O
cannot undo the device operation; transport evidence remains visible.

`MacroProgress.Assignment` and `.Upload` retain independent transport evidence,
including report08 ACK followed by potentially partial report09 failure.
Failure retains the draft and previous local applied state, without retry or
rollback. Snapshot reads and persistence retry never upload. Another user
Apply is an explicit new attempt, not a recovery guarantee. Concurrent clearing
or replacement is respected and never resurrected by stale completion.

Success requires assignment ACK and upload final status, and means transport
confirmation only—not physical playback, persistence or actual readback.
Captured destinations 05/06 are evidence-backed; other existing button groups
remain authorized extrapolation. Independent simultaneous firmware slots,
physical trigger behavior and repeat acceptance/playback remain unverified.
