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
- Delay units, complete repeat field width/range, event limits and persistence/playback are unverified.
- Preserve pre-existing dirty task notes and untracked metadata/design directories.
- No publishing, PR, merge, device writes or automatic destructive actions.
- Branch: feat/x6-macro-manager, created from feat/refactor-integration-v1-3-0 at 280d604.
- User explicitly authorized work-unit commits for this feature on 2026-10-04; no push or unrelated files.

## Work units
- [x] XM-1: Define and persist a shared local macro library with stable IDs, validated events, CRUD, and atomic JSON storage. Commit: f7b8b491e98b118b40dd5a0e7de3da012856daad.
- [ ] XM-2a: Expose shared library CRUD via desktop service, initialize a single instance in startup and regenerate bindings with tests. Status: in_progress; user approved the exact ten additional generator dependency/event paths on 2026-10-04.
- [ ] XM-2b: Integrate list/create/rename/delete/view in the workspace manager UI using the approved visual reference and tests.

XM-2 was split into desktop and UI units to avoid one oversized multi-area change; its user-facing scope is unchanged.
- [ ] XM-3: Add event editor, scoped recording, and validated import/export with tests and user-facing errors.
- [ ] XM-4: Preserve capture evidence and implement a capture-backed encoder/decoder for verified left/right events and fixed-repeat values; document unsupported timing/ranges.
- [ ] XM-5: Integrate per-device fixed-repeat assignment and staged apply/upload, with error handling and mocked transport tests.
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
- X3 published MIT protocol is analogous framing only; blockID08 and slot rules are not substituted for observed X6 blockID05.

## Progress and verification evidence
- Exploration and offline decoding completed; no device operations or source changes before task tracking.
- Library scope confirmed by user: shared.
- XM-1: internal/macros/library.go, library_test.go, domain_test.go added (~498 lines). API Open/List/Create/Read/Update/Delete; local left/right down/up events with nonnegative delay_ms, stable IDs, atomic versioned JSON persistence, defensive copies, single-instance concurrency.
- XM-1 RED: missing API compile failure observed by writer; GREEN: 15 tests passed. go test -race ./internal/macros -count=1, go vet ./internal/macros and git diff --check passed. Parent spot check go test ./internal/macros -count=1 passed (15 tests).
- XM-1 review: review-dd6d8cab4e0cdb01 approved; exact acknowledgement burned authority for target sha256:fb73c13b90ca8d05fc33cb873f4930471cdcb702975d66e77361e61c0f3409c3. Informational advisory R3-premature-completion concerns pre-existing unrelated refactor task note; not edited by this feature.
- XM-1 limitations: no cross-process coordination or directory-entry crash durability guarantee; no UI/startup/device wiring. Full repo suites/build/browser/physical checks pending later units. Commit: f7b8b491e98b118b40dd5a0e7de3da012856daad (source/tests plus feature tracking only; unrelated changes preserved).
- Engram mirror and visible todo must stay synchronized after each task transition.

## Next step
Resume XM-2a with approved generated output paths. Existing Podman image localhost/attack-shark-beta26-appimage:verify-mupfv4rx verified to contain Go1.27.1, wails3, GTK4 4.14.5, WebKitGTK2.52.6 and libsoup3.4.4. User prefers reuse with --rm; inventory contained images, no retained containers. Writer authorized offline startup-package check with read-only repository/host module-cache mounts and tmpfs build cache. Generation may use same existing image with the repository mount writable only for approved paths. No host installs, new image builds/pulls or cleanup/prune. Image module-cache completeness unknown; host cached modules mounted read-only. Separate frontend binding root still needs usage assessment. No device writes or push.
