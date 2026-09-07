# Proposal: Emergency Reset and Remap Confirmation

## Intent

Require explicit confirmation for every X6 configuration write and make `ResetToFactory` use one failure-safe reset path. Close four blockers: the missing remap assertion, absent `Service.ApplyRemap` coverage, absent real-service reset/restart coverage, and implicit `StageDPI` writes.

## Scope

### In Scope
- Keep DPI and polling staging local; writes require explicit apply or confirmed reset.
- Make `ResetToFactory` delegate to `EmergencyReset.Run`, writing documented DPI, 1000 Hz polling, and default remap in order through the validated binding.
- Purge all persisted X6 state only after all three ACKs; preserve it after any failed ACK.
- Add service-level remap tests for selection, validation, targeting, ACK-gated state, persistence failure, and retry.
- Add deterministic runtime tests for ordering, cancellation, restart, failed ACK preservation, and late callbacks.

### Out of Scope
- Protocol format changes or hardware-dependent tests.
- Lighting reset, USB ownership, permission changes, or automatic retry.
- External research; it was explicitly deselected after admission was unavailable.

## Capabilities

### New Capabilities
- `explicit-device-configuration`: Local staging, explicit confirmation, selected-binding writes, ACK-gated state, and persistence.
- `complete-factory-reset`: Confirmed three-lane reset orchestration, pending-write quiescence, ordered ACK handling, and success-gated purge.

### Modified Capabilities
None; no source specs currently exist.

## Approach

Adopt exploration Approach 1. Remove scheduler-triggered writes from staging. Reuse pure X6 operations and `ApplyOperationBound`; keep orchestration in `EmergencyReset`, quiesce callbacks before writes, and expose it through `Service.ResetToFactory` without crossing existing boundaries.

## Affected Areas

| Area | Impact | Description |
|---|---|---|
| `internal/desktop/service.go`, `sync.go` | Modified | Explicit apply/reset boundaries; local staging only. |
| `internal/desktop/emergency_reset.go` | Modified | Authoritative reset delegation and lifecycle. |
| `internal/desktop/*_test.go` | Modified | Four blocker-closing test groups. |
| `frontend/src`, generated bindings | Modified | Explicit confirmation UX and regenerated contract. |

## Risks

| Risk | Likelihood | Mitigation |
|---|---|---|
| Stale callback writes after confirmation/reset | Medium | Cancel schedules, invalidate revisions, test deterministic races. |
| Partial reset leaves mixed hardware state | Medium | Stop on failed ACK and preserve persisted recovery state. |

## Rollback Plan

Revert service/UI changes and bindings together; retain persisted state and disable reset if guarantees regress.

## Dependencies

- Existing X6 operations, validated binding command path, scheduler seams, and state purger.

## Success Criteria

- [ ] Staging plus scheduler advancement performs zero hardware writes until explicit confirmation.
- [ ] Remap service scenarios prove validation, targeting, ACK gating, and persistence recovery.
- [ ] Reset tests prove ordered three-lane ACKs, cancellation/restart behavior, and no purge on any failed ACK.
- [ ] Successful confirmed reset purges state only after all three ACKs.
