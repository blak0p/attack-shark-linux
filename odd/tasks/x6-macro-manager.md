# X6 Macro Manager

## Objective
Provide an app-shared local macro library and an integrated manager, initially uploading verified left/right mouse macros with fixed repetitions to X6 devices.

## Problem and rationale
The app currently exposes a closed hardware remap catalog, without local macro definitions or editing. Vendor UI reference describes library management separately from button assignment. Local names and editing are app data, not firmware capabilities.

## Authorized scope
- Shared app library; per-device assignments remain separate.
- UI visual reference: /home/alejandro/Descargas/Interfaz de macros del controlador.png. Use its master/detail list-left/editor-right layout and cards with the app's existing theme. Do not import its speed slider, extra playback modes, arbitrary limits or unverified controls. Distinguish local save from device apply.
- Named library CRUD, viewing/editing ordered press/release events and delays.
- PDF-inspired recording and import/export, adapted to the existing UI with explicit supported boundaries.
- Initial hardware actions: left/right clicks; fixed-repeat numeric assignment only.
- Extensible model for other mouse events and keyboard after evidence validation.
- Preserve originals and copy useful protocol evidence into report-based capture directories when the evidence task runs.

## Constraints and non-goals
- No toggle or hold execution modes; no unsupported firmware semantics inferred from X3.
- Separate local storage from existing device configuration and HID writes.
- Delay units, complete repeat field width/range, event limits and persistence/playback are unverified. User reports the vendor X6 app accepts 255 and refuses higher input; this establishes a UI upper bound, not protocol or playback acceptance.
- Preserve pre-existing dirty task notes and untracked metadata/design directories.
- No publishing, PR, merge, device writes or automatic destructive actions.
- Branch: feat/x6-macro-manager, created from feat/refactor-integration-v1-3-0 at 280d604.
- User explicitly authorized work-unit commits for this feature on 2026-10-04; no push or unrelated files.

## Work units
- [x] XM-1: Define and persist a shared local macro library with stable IDs, validated events, CRUD, and atomic JSON storage. Commit: f7b8b491e98b118b40dd5a0e7de3da012856daad.
- [x] XM-2a: Expose shared library CRUD via desktop service, initialize a single instance in startup and regenerate bindings with tests. Commit: 27022dc839adf8356e0ed3b702d6c3e5b3517c07.
- [x] XM-2b: Integrate list/create/rename/delete/view in the workspace manager UI using the approved visual reference and tests. Commit: a9e9acec55ee81ac164d3a55b4cdb4285953a479.

XM-2 was split into desktop and UI units to avoid one oversized multi-area change; its user-facing scope is unchanged.
- [x] XM-3a: Edit ordered left/right press/release events and nonnegative local delays with save/error UX and tests. Commit: 6b92b0c634fbeb63bb41f49f88120d9969ca597a.
- [x] XM-3b: Add explicitly armed editor-scoped browser recording of left/right press/release pairs with monotonic measured delays, lifecycle cleanup and tests; never global/hardware capture. Commit: ac94fa3f98a42c89093a39fbe0c040d090192fa1.
- [x] XM-3c: Export one versioned JSON macro without ID and import it as a validated new local copy with a fresh ID, preserving existing macros; test invalid input and errors. Commit: 4c73a3c1d951a0dab1739a3fa41732778d9e9353.

XM-3 was split into three reviewable work units because editor, recording and file exchange are distinct behaviors. Recording only captures actions inside a visible editor area; measured delays are local input data, not verified X6 playback timing.
- [x] XM-4a: Preserve selected decoded report 0x09 blocks and source hashes/frame provenance from X6 macro captures in a report-based evidence directory, without modifying originals. Commit: ea69e434b0c85e5644232d5066cbaec094488471.
- [x] XM-4b: Implement a pure offline capture-backed left/right macro encoder/decoder and exact-byte tests. Accept the decided UI range 1–255 for the observed one-byte template while documenting interpolation beyond captured values and rejecting unsupported timing/event layouts; no device writes. Commit: 9ec790eec00326af784f731297286a36c7088819.

XM-4 was split into evidence and codec work units to keep binary protocol inference reviewable. The user confirmed 255 as the vendor app UI maximum; captured payloads currently prove values 1, 2, 3 and 5. A 255 capture, if supplied, will test the extrapolation rather than reopen the UI-range decision.
- [ ] XM-5: Integrate per-device fixed-repeat assignment and staged apply/upload, with error handling and mocked transport tests. Clearly label the numeric UI range as 1–255 (user-observed vendor app maximum); distinguish UI allowance from capture-verified device behavior.
- [ ] XM-6: Run end-to-end UI checks, backend/frontend suites and builds; review the bounded candidates and document physical-device checks still pending.

## Acceptance criteria and checks
- Local library shared across devices and independent of HID/device configuration.
- Editing, saving and import do not write to hardware.
- Stable identities, deterministic validation and corrupt-file protection.
- No toggle/hold controls. Initial uploads use only proven protocol semantics.
- Applicable behavior work uses observed RED/GREEN tests, then refactoring with focused checks.
- Unit checks per task; closure commands: go test ./..., go vet ./..., cd frontend && npm test, cd frontend && npm run build, cd frontend && npm run e2e, git diff --check.
- Browser checks cover mocked UI; no claim of real Wails recording/device playback without physical evidence.
- Tests/docs accompany each work unit. Review size estimates are advisory, not hard limits; avoid one oversized multi-area change.

## Protocol evidence
- Reference: /home/alejandro/Descargas/attack-shark-x6-gestor-macros.pdf (5 pages; UI source, not protocol specification).
- Original macro.pcapng: report09 chunks 14313/14317/14319; five left click-like pairs, event count10, checksum0bff.
- macro_izq.pcapng: map11833, chunks11837/12489/12543; logical block128bytes with nonzero offsets4=01,25=02,26=01,27=f1,28=81,29=f1,126=02,127=67.
- macro_derecho.pcapng: map10643, chunks10693/10753/10783; only event code offsets27/29 changes f1->f2 and checksum0267->0269.
- macro_izq_loop2.pcapng: only unchanged report08 at6331, no macro upload; cannot establish counter.
- macro_izq_new_loop2.pcapng: map10857/13945 unchanged; chunks10861/10945/10947; block offset4=02 and checksum0268, all other bytes unchanged.
- Headers: 09 40 05 00 / 09 40 05 01 / 09 0c 05 02. Physical report lengths64, logical data60+60+8, trailing padding excluded. Checksum BE sum of block0:126 stored126:128.
- New external originals: /home/alejandro/x6-capturas/macro_izq_loop3.pcapng (SHA256 62e77e4611362fd52072ef2213e86adce5fdc7da1d98c8b4c52e014ce1734ad9) and macro_izq_loop5.pcapng (SHA256 8da51a14ebff158f04f49e64f0d83d3186a0081bba541b4736253f6baef08bdc). Read-only decoded report09 chunks for value3: one-based frames16745/16863/17983 and18407/18411/18413; value5:11595/11683/11713. Each 128-byte logical block differs from count1 only at offset4=03/05 and checksum127=69/6b, BE checksums0269/026b validated. Loop3 also contains prior value2 at13477/13481/13483. Parser indexes are zero-based; frame citations here are one-based. Originals not yet copied into repository.
- X3 published MIT protocol is analogous framing only; blockID08 and slot rules are not substituted for observed X6 blockID05.

## Progress and verification evidence
- Exploration and offline decoding completed; no device operations or source changes before task tracking.
- Library scope confirmed by user: shared.
- XM-1: internal/macros/library.go, library_test.go, domain_test.go added (~498 lines). API Open/List/Create/Read/Update/Delete; local left/right down/up events with nonnegative delay_ms, stable IDs, atomic versioned JSON persistence, defensive copies, single-instance concurrency.
- XM-1 RED: missing API compile failure observed by writer; GREEN: 15 tests passed. go test -race ./internal/macros -count=1, go vet ./internal/macros and git diff --check passed. Parent spot check go test ./internal/macros -count=1 passed (15 tests).
- XM-1 review: review-dd6d8cab4e0cdb01 approved; exact acknowledgement burned authority for target sha256:fb73c13b90ca8d05fc33cb873f4930471cdcb702975d66e77361e61c0f3409c3. Informational advisory R3-premature-completion concerns pre-existing unrelated refactor task note; not edited by this feature.
- XM-1 limitations: no cross-process coordination or directory-entry crash durability guarantee; no UI/startup/device wiring. Full repo suites/build/browser/physical checks pending later units. Commit: f7b8b491e98b118b40dd5a0e7de3da012856daad (source/tests plus feature tracking only; unrelated changes preserved).
- XM-2a: ListMacros/CreateMacro/ReadMacro/UpdateMacro/DeleteMacro exposed, single macros-v1.json instance injected at startup. Corrupt-store errors preserved; CRUD offline and independent of HID. RED missing APIs/startup injection observed; GREEN 3 desktop + 2 startup macro tests, 154 desktop/domain tests, race tests and vet passed. Native startup tests and binding generation passed in reused Podman image, --rm left no containers. Parent reran 3 desktop macro tests successfully.
- XM-2a native review review-7eeb3a45a86b862b approved and acknowledged (authority burned). Same informational unrelated task-note advisory as XM-1; no source blocker. Commit 27022dc839adf8356e0ed3b702d6c3e5b3517c07. Generated binding refresh includes nine existing remap enum entries; frontend/bindings root unchanged because API adapter consumes command-root bindings.
- XM-2b UI worker mutrkf6u-c-ev2x finished (no active writer). Added Macros workspace with existing-theme responsive master/detail, library create/rename/view/delete+confirmation, ordered read-only event list/delays, loading/error states and offline availability. Adapter uses generated lowercase JSON types. Guards stale async/service responses and duplicate submissions; preserves draft/data on failure. ~520 authored added lines in14files.
- XM-2b observed RED: missing panel/import/adapter plus failed-load false empty state; GREEN final18focusedtests,116frontendtests across12files, frontend build and3ChromiummacrosE2E tests passed (across-device/offlineCRUD,620pxlayout). Parent repeated18focusedtests successfully. Earlier missingApplyRemaptestspy corrected narrowly; final checks passed. Browser mocks do not prove native Wails persistence or physical hardware. No event manipulation/recording/import-export/repeat/assignment controls yet. Native review review-11d5e8af6fc17dd0 approved and acknowledged (authority burned); non-blocking informational R3-lost-library-retry in useMacroLibrary.ts:76 is later follow-up, not a correction for this candidate. Source/tests commit a9e9acec55ee81ac164d3a55b4cdb4285953a479; unrelated refactor task note excluded from commit.
- XM-3a: event add/edit/remove/reorder, nonnegative JS-safe integer local delay editing, validation/save retry; stable ID and name preserved. RED 2 missing-control failures; GREEN 10 focused UI tests, frontend build (73 modules) and parent 10-test repeat passed; git diff --check passed. Native committed-range review review-14b4cc7dd60c84b8 approved and acknowledged (authority burned). R3-delay-rounding and R3-hidden-operation-error are informational, non-blocking follow-ups; not corrections to this candidate. Source/tests and split task plan commit 6b92b0c634fbeb63bb41f49f88120d9969ca597a. E2E, full suites and physical playback remain pending.
- XM-3b: explicitly armed visible recording zone, supported left/right press/release pairs, rounded monotonic elapsed local delays, balanced-session Stop, no global listeners; events remain unsaved until explicit Save. RED 3 missing-zone failures; GREEN 14 focused UI tests, parent 14-test repeat and git diff --check passed. Native committed-range review review-e35a48f9ab9308ff approved and acknowledged (authority burned). Commit ac94fa3f98a42c89093a39fbe0c040d090192fa1. Frontend build skipped because this unit did not authorize generated-output cleanup; typecheck unavailable (no local tsc or frontend tsconfig). E2E, full suites and physical behavior pending.
- XM-3c: strict versioned one-macro JSON validation, 1 MiB local file safety budget, ID-free saved-data export and create-only import with fresh ID; duplicate names append and failures preserve existing data. RED missing codec/import and two UI controls; GREEN final 40 focused tests, parent 40-test repeat and git diff --check passed. Native committed-range review review-8106b0ab9c095987 approved and acknowledged (authority burned); R3-export-budget informational non-blocking follow-up. Commit 4c73a3c1d951a0dab1739a3fa41732778d9e9353. Build skipped for generated-output cleanup scope; broad suites, E2E and physical checks pending.
- XM-4a: captures/0x09-macro contains five 128-byte binary fixtures, contract JSON and read-only reproduction README. Independent verifier reproduced five fixture hashes/checksums/diffs, six external original hashes/sizes, seven frame groups and negative control; original hashes unchanged, git diff --check passed. Passive evidence has no meaningful RED; native executable review skipped for this passive evidence-only unit. Commit ea69e434b0c85e5644232d5066cbaec094488471. No source/device edits. Field width, timing and playback remain unverified.
- Engram mirror and visible todo must stay synchronized after each task transition.

## Current execution
- User authorized continuing XM-4b, XM-5 and XM-6 on 2026-10-05. XM-4b completed with independent verification, commit and native approved acknowledgement. XM-5 is in read-only integration/evidence exploration before safe implementation. Preserve single-writer execution and existing work-unit commit consent. No push or physical device writes.
- XM-4b worker: internal/macros/x6codec.go, x6codec_test.go and README.md. Observed RED missing API compile failure; GREEN 149 tests including exact fixtures, all repeat-byte values 1–255, strict layouts/checksums/framing. Worker go test, race, vet and diff check passed. Independent verifier reproduced 149 tests, race, vet and diff check successfully; five fixtures and seven upload groups checked byte-for-byte. Commit 9ec790eec00326af784f731297286a36c7088819. Native committed-range review review-6b7dd7a88a9570f0 approved and exact acknowledgement burned authority. First reviewer submission was rejected for inconsistent inspection availability; fresh provider-reoffered capture succeeded. Physical acceptance/playback and original PCAP provenance not rechecked by this verifier.
- XM-5 scout: current HID filter rejects report09; existing remap08 is not proof of macro slot mapping. Timing projection, report08 assignment and report09 ACK/partial failure semantics require direct capture evidence inspection before transport implementation. No second writer.

## Next step
XM-3a/b/c closed with native approved acknowledgement and work-unit commits. XM-4a evidence committed and independently reproduced. XM-4b is now complete. Next unit XM-5: inspect original assignment/ACK evidence and establish safe admission for local event timing before staged integration; never silently discard delays or invent macro slot mapping. Preserve unrelated dirty refactor task note, generated cache and untracked metadata/design files. No push or device writes. Full physical checks, frontend build for recording/import-export, timing and count-width gaps remain pending.
