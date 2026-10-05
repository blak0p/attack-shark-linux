# Bound X6 macro upload

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
