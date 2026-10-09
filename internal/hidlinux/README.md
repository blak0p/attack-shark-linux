# Bound X6 macro upload

The explicit composite `SendX6MacroAssignmentBound(ctx, binding, assignment,
click)` (and mouse `ApplyMacroAssignmentBound`) validates typed remap and click
before I/O, derives the destination from the assignment button, and owns one
command lock/node for report08, exact `0310500008`, three report09 chunks and
final `0310500009`. Identity, descriptor, path and cancellation are checked
before writes and the upload completion phase. No generic report09 bypass is
provided. Legacy block05 upload remains unchanged.

The additive `SendX6MacroSequenceAssignmentBound` and mouse
`ApplyMacroSequenceAssignmentBound` accept `macros.X6Sequence`: one or two
complete zero-delay clicks from the five captured action codes. Both composite
APIs share the same serialized transport implementation, status handling and
partial-progress rules. Destination comes only from `assignment.Button`, never
from the sequence's actions. Invalid sequences fail before ownership or writes;
legacy click admission and ordinary remapping are unchanged. Desktop admission
and UI wiring are separate work; transport success does not prove playback.

Returned `MacroProgress` has independent assignment (not started, unknown,
ACK confirmed) and upload (not started, possibly partial, confirmed) evidence.
Assignment becomes unknown before its write attempt; an observed08 ACK remains
confirmed even if later upload fails. Errors after that attempt warn about
partial mutation. Neither confirmation establishes physical playback or
persistence. Destinations05/06 are capture-backed; other mapped button groups
are authorized extrapolation, not proof of independent simultaneous slots.
Desktop staging/composition and physical validation remain separate work.

`HidrawBackend.SendX6MacroBound(ctx, binding, macros.X6Click)` encodes the strict
single-button template before I/O. It acquires command ownership, revalidates the
captured candidate identity, serial and status descriptor, and opens only that
binding's node. Three report09 chunks share one command lock and one node.
Generic feature-report admission remains unchanged; report08 assignment and
service/UI integration are not part of this API.

Chunks have a cancellable 1.2-second gap. This is conservative engineering
policy, **not a validated firmware minimum**; captures showed
811.726–1190.951ms. Tests inject the internal wait function without real pacing.
The bounded final-status deadline starts only after all three writes; there is
no per-chunk ACK wait. Reports consumed by the command reach the existing
same-binding listener dispatch. Exact `0310500009` means normal upload status
only, not playback, readback or persistence. Known five-byte heartbeats and
unrelated ACKs are ignored within that deadline; unknown or truncated statuses
fail closed with diagnostic bytes.

Errors after any attempted write warn of possible device partial mutation.
There is no rollback claim or automatic retry. Cancellation releases command
ownership and closes the command node. All tests use fixture sysfs and mocked
nodes, never a physical device.
