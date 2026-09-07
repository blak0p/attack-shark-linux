```yaml
schema: gentle-ai.verify-result/v1
evidence_revision: sha256:ef21258107909777443ea75fe0204dbae6cf36d3745e9880db9d192319805b17
verdict: pass
blockers: 0
critical_findings: 0
requirements: 1/1
scenarios: 2/2
test_command: "go test ./internal/desktop -run 'Test.*Polling' -count=1 && go test ./internal/desktop -run 'Test.*(Remap|EmergencyReset)' -count=1 && go test ./internal/desktop -run 'TestResetRuntime' -count=1 && go test -race ./internal/desktop -run 'Test.*(Polling|Remap|EmergencyReset|ResetRuntime)' -count=1 && go test ./... -count=1 && npm --prefix frontend test -- --run"
test_exit_code: 0
test_output_hash: sha256:816cda8fae5014a7cccb1c59e620f3497c854f209f45487a88f26fb00e6b7d48
build_command: "go vet ./... && go build ./... && npm --prefix frontend run build -- --outDir /tmp/opencode/emergency-reset-remap-evidence-remediation-vite-dist --emptyOutDir"
build_exit_code: 0
build_output_hash: sha256:6cb77800ca0acf7c8c8623b9fbf4d475d08743c08d46c1a36fb12c7359c58668
```

## Verification Report

**Change**: emergency-reset-remap-evidence-remediation
**Version**: N/A
**Mode**: Standard
**Native attempt token**: `sha256:a59721672e0ed5dceeab466dfef36db837fe6e4d708a95cc0e5cee51abd25454`
**Candidate identity**: `sha256:5b03c2c80975ceedc5272f031d8a3ae72349437b5704703e103e69d8af47c978`
**Candidate tree**: `c22355cfcb43c20cb2d3a09ab59ce490ffcb6905`

### Completeness

| Metric | Value |
|---|---:|
| Tasks total | 11 |
| Tasks complete | 11 |
| Tasks incomplete | 0 |
| Requirements compliant | 1/1 |
| Scenarios compliant | 2/2 |

All eleven tasks are checked and carry `✅ Written` / `✅ Passed` evidence where required. The retrieved remediation specification contains one requirement and two scenarios.

### Build & Tests Execution

| Command | Exit | Result | Exact combined-output SHA-256 |
|---|---:|---|---|
| `go test ./internal/desktop -run 'Test.*Polling' -count=1 && go test ./internal/desktop -run 'Test.*(Remap\|EmergencyReset)' -count=1 && go test ./internal/desktop -run 'TestResetRuntime' -count=1 && go test -race ./internal/desktop -run 'Test.*(Polling\|Remap\|EmergencyReset\|ResetRuntime)' -count=1 && go test ./... -count=1 && npm --prefix frontend test -- --run` | 0 | Four focused/race Go checks passed; eight Go packages passed uncached; five frontend files and 68 tests passed. | `sha256:816cda8fae5014a7cccb1c59e620f3497c854f209f45487a88f26fb00e6b7d48` |
| `go vet ./... && go build ./... && npm --prefix frontend run build -- --outDir /tmp/opencode/emergency-reset-remap-evidence-remediation-vite-dist --emptyOutDir` | 0 | Vet and Go build passed; Vite transformed 64 modules and wrote only to `/tmp/opencode`. | `sha256:6cb77800ca0acf7c8c8623b9fbf4d475d08743c08d46c1a36fb12c7359c58668` |
| `go test ./internal/desktop -coverprofile=/tmp/opencode/emergency-reset-remap-evidence-remediation-desktop.cover -count=1` | 0 | Desktop package passed with 85.1% statement coverage. | Coverage command output recorded independently. |
| `git diff --check de450e159d3386dad28be57dac1806ff1193e1cc c22355cfcb43c20cb2d3a09ab59ce490ffcb6905` | 0 | No whitespace errors in the bounded remediation delta. | Empty output. |

All executed checks were hardware-free. Relevant tests use recording commands, manual schedulers, and `t.TempDir`; no hidraw node was opened, no USB interface was claimed, and no root privilege was assumed. The frontend build was redirected to `/tmp/opencode`, so verification did not modify repository build artifacts.

**Coverage**: 85.1% of statements in `internal/desktop`; no project threshold is declared. Frontend per-file coverage is not configured.

### Spec Compliance Matrix

| Requirement | Scenario | Runtime evidence | Result |
|---|---|---|---|
| Preserve the existing product capability scope | Predecessor requirements remain authoritative | Read-only candidate-tree assertion `git diff --quiet de450e159d3386dad28be57dac1806ff1193e1cc c22355cfcb43c20cb2d3a09ab59ce490ffcb6905 -- openspec/changes/emergency-reset-remap-confirmation internal/protocol internal/hidlinux frontend/src/wails-service.ts` exited 0; all Go and frontend suites passed. | ✅ COMPLIANT |
| Preserve the existing product capability scope | Remediation remains bounded | The same tree assertion found no protocol, hardware, facade, or predecessor-artifact delta. The complete baseline diff is confined to polling DTO propagation, focused service correction/tests, reset/remap/lifecycle evidence, generated assets, and change-local evidence. | ✅ COMPLIANT |

**Compliance summary**: 2/2 scenarios compliant; 1/1 requirement complete.

### Correctness (Static and Runtime Evidence)

| Finding remediated | Status | Concrete evidence |
|---|---|---|
| Polling invalid selection fails closed | ✅ Implemented and tested | `Service.ApplyPollingRate` returns `failPolling(..., SelectionRequired)` for absent selection and maps stale binding to the same typed code. `TestApplyPollingRateRejectsInvalidSelectionWithoutIO` covers absent, stale, and invalid selections with unchanged state and zero command/persistence calls. |
| Remap validation, targeting, ACK, persistence, and repeat idempotence | ✅ Implemented and tested | `TestApplyRemapValidatesBindingACKAndPersistence` covers invalid/missing/stale selection, exact binding, ACK failure/success, and persistence failure/success. The caller-driven repeat test proves persistence-only recovery without a second device write. |
| Quiescence rejects retained callbacks | ✅ Implemented and tested | `Service.Quiesce` cancels both coordinators and increments DPI/polling revisions. `TestResetRuntimeQuiescesLateCallbacksBeforeResetWrites` manually fires retained callbacks after reset and observes exactly the three reset writes. |
| Reset uses exact ordered lanes and excludes lighting | ✅ Implemented and tested | `EmergencyReset.Run` orders documented DPI, 1000 Hz polling, and default remap. `TestEmergencyResetPurgesOnlyAfterAllLanesSucceed` compares all encoded reports in order and rejects report ID `0x05`. |
| Every physical-lane failure preserves recovery bytes | ✅ Implemented and tested | `TestEmergencyResetReportsRetryableLaneFailures` exercises failures at lanes 1–3 and compares the seeded recovery file byte-for-byte after each failure. |
| Restart/reselection requires fresh confirmation | ✅ Implemented and tested | `TestResetRuntimeRestartRequiresFreshConfirmedResetAfterFailure` rebuilds real services over one temporary state directory, proves reselection performs zero writes and preserves recovery state, then proves a fresh confirmed three-lane reset purges state. |
| Canonical evidence remains change-local | ✅ Implemented | Task/apply evidence is under this remediation change; the read-only baseline assertion found no predecessor-artifact delta. |

### Coherence (Design)

| Decision | Followed? | Notes |
|---|---|---|
| Add typed polling error without changing the method signature | ✅ Yes | `PollingSnapshot.Error` is mapped from `pollingState.err`; both generated model roots are byte-identical and include `Error`; the TypeScript desktop contract requires it. |
| Reject invalid selection before I/O | ✅ Yes | Focused tests observe zero command and persistence calls for absent, stale, and invalid selections. |
| Use real service seams with deterministic hardware-free fakes | ✅ Yes | Runtime tests compose `desktop.Service`, `mouse.TargetedService`, manual scheduler callbacks, recording commands, and temporary persistence without sleeps or hardware. |
| Purge only after all three ACK-gated lanes | ✅ Yes | `EmergencyReset.Run` reaches `PurgeAll` only after three successful `ApplyOperationBound` calls; failures at each lane preserve state. |
| Keep retry caller-driven and idempotent | ✅ Yes | Repeating `ApplyRemap` after persistence failure calls persistence again but does not emit another device write; no automatic retry path was added. |
| Preserve protocol, hardware, lighting, facade, and predecessor boundaries | ✅ Yes | Baseline-to-candidate tree assertion exited 0 for all named out-of-scope paths. No new package or forbidden import edge was introduced. |

### Issues Found

**CRITICAL**: None.

**WARNING**: None.

**SUGGESTION**: None.

### Verification Evidence Preimage

The exact UTF-8 preimage below includes its terminal newline and is 892 bytes:

```yaml
schema: gentle-ai.sdd-verification-evidence/v1
change: emergency-reset-remap-evidence-remediation
attempt_token: sha256:a59721672e0ed5dceeab466dfef36db837fe6e4d708a95cc0e5cee51abd25454
candidate_identity: sha256:5b03c2c80975ceedc5272f031d8a3ae72349437b5704703e103e69d8af47c978
candidate_tree: c22355cfcb43c20cb2d3a09ab59ce490ffcb6905
verdict: pass
mode: standard
test_exit_code: 0
test_output_hash: sha256:816cda8fae5014a7cccb1c59e620f3497c854f209f45487a88f26fb00e6b7d48
build_exit_code: 0
build_output_hash: sha256:6cb77800ca0acf7c8c8623b9fbf4d475d08743c08d46c1a36fb12c7359c58668
desktop_coverage: 85.1%
scope_baseline_tree: de450e159d3386dad28be57dac1806ff1193e1cc
scope_candidate_tree: c22355cfcb43c20cb2d3a09ab59ce490ffcb6905
out_of_scope_baseline_diff: none
generated_desktop_models_identical: true
requirements: 1/1
scenarios: 2/2
blockers: 0
critical_findings: 0
hardware_access: none
```

The preimage hashes to `sha256:ef21258107909777443ea75fe0204dbae6cf36d3745e9880db9d192319805b17`.

### Verdict

**PASS**

All 11 tasks are complete, both specification scenarios have executable evidence, the focused/race/full test suites and builds pass, and inspection found no critical findings, warnings, hardware access, or out-of-scope remediation delta.
