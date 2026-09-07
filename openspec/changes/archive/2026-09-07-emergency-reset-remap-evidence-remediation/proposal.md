# Proposal: Emergency Reset/Remap Evidence Remediation

## Intent

Close the eight critical findings from `emergency-reset-remap-confirmation`. Correct polling apply without a validated selection and add hardware-free runtime evidence for the existing contracts.

## Scope

### In Scope

- Return a typed polling selection error and propagate minimum DTO/binding changes.
- Add remap/polling service assertions for validation, targeting, ACK, persistence, and retry.
- Prove exact reset lanes, no lighting, and state preservation for every failed lane.
- Use a deterministic real `Service`/`TargetedService` harness for quiescence, late callbacks, restart, reselection, and fresh confirmation.
- Record canonical Strict TDD evidence during later apply/verify phases.

### Out of Scope

- Protocol changes, hardware access, lighting reset, automatic retry, or UI redesign.
- Unrelated refactoring or the `ApplyRemap` context warning.
- External research and predecessor-artifact edits in this phase.

## Capabilities

### New Capabilities

None. This change introduces no product capability.

### Modified Capabilities

None. Blocked-predecessor requirements remain unchanged; this change corrects compliance and evidence.

## Approach

Fix polling selection propagation, extend focused tests, and compose a hardware-free runtime harness with deterministic command/scheduler fakes and temporary persistence. Regenerate Wails bindings only through the supported generator.

## Affected Areas

| Area | Impact | Description |
|---|---|---|
| `internal/desktop/service.go` | Modified | Surface polling selection failure. |
| `internal/desktop/*_test.go` | Modified | Add bounded runtime evidence. |
| `frontend/src/{desktop-contract.ts,wails-service.ts}` | Conditional | Propagate the error contract. |
| `cmd/x6configurator/frontend/bindings/` | Conditional | Regenerate bindings. |
| Apply/verify artifacts | Modified later | Record canonical evidence. |

## Risks

| Risk | Likelihood | Mitigation |
|---|---|---|
| DTO drift across Go/Wails/TypeScript | Medium | Generate bindings and assert the facade. |
| Harness tests discovery, not reset | Medium | Assert exact bindings and writes. |
| Timing-sensitive callbacks | Medium | Use controlled schedulers without sleeps. |

## Rollback Plan

Revert the polling contract, generated bindings, and tests together. No data or protocol migration is involved.

## Dependencies

- Blocked predecessor specs and canonical verification report.
- Wails generator if the polling DTO changes.

## Success Criteria

- [ ] Polling apply without a validated selection returns the typed selection error and performs no write.
- [ ] Deterministic tests close all eight findings without hardware.
- [ ] The real-service harness proves no late/pre-confirmation writes or premature purge.
- [ ] Later verification reports canonical evidence and complete compliance for these findings.
