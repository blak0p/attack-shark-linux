# Dependency and toolchain alignment

## Objective, scope and authorization
Update the whole build chain instead of bypassing red Dependabot checks. User selected both Go and Wails upgrades, then preferred latest stable Go over minimum 1.26. Targets: Go 1.27.1, Wails backend/CLI/runtime beta.25, x/sys v0.48.0. Preserve application API/UI/protocol/storage and the normalized public release contract: only version/public-key linker flags; signing seed stays environment-only. No global CLI replacement, host system installs, generated-binding hand edits or blanket red-PR merges.

## Baseline and route
- Isolated dependency-alignment worktree; branch `chore/align-go1271-wails-beta25`, base `origin/main` f2b02ab0fec92c36e320d735a515081a6bc6828d with Browser PR #115. Original checkout and separate refactor HEAD 2dc5b2e preserved.
- Official https://go.dev/dl/?mode=json reports stable Go 1.27.1; host has the same version. Exact pins, not floating latest. Linux amd64 archive checksum: 63d339f0da5ab53635a56f2490a7984dfe12dfcff22ad749f63edaf590168445; verify if downloaded. Current builders use the versioned upstream image.
- Strict TDD ON from `openspec/config.yaml` (`strict_tdd: true`). Runner: `cd frontend && npm ci && npm run build && cd .. && go test ./...`. Dependency downloads, builds, rootless images and unsigned local packaging authorized; no signing/publication/GUI/hardware. Delegated-direct mapping and one multi-file writer; parent owns document, Engram, review and delivery. Forecast 250–400 lines excluding lock churn is advisory. No SDD selected.

## Tasks
- [x] DA-1 (delegated): Alignment RED/GREEN; update module/runtime, CI/release/test/AppImage toolchains and docs; preserve release/security contract.
- [x] DA-2 (delegated): Independent host, isolated CLI/build, actual Ubuntu images, unsigned packaging, fresh extraction and offline read-only suite verified.
- [ ] DA-3 (parent, in progress): Commit/review authorization, approved issue/PR prerequisites, Dependabot reconciliation and delivery plan. Refactor integration is separate. User authorized local commit/review, issue creation/approval and PR; no merge/tag/release. Parent is executing that bounded delivery.

## Original failure evidence
Live GitHub logs: PR #117 compiles/vets, but `main_test.go:532` requires beta.23 packaging alignment while the module upgrades to beta.25; release/AppImage CLI remains beta.23. PR #118 raises Go 1.25 → 1.26 with x/sys .46 → .48; Tests/CodeQL use Go 1.25.14 and GOTOOLCHAIN=local, failing before compilation. Green PRs #116/#119/#120/#121/#122 still require individual compatibility review; runtime #121 is covered by coordinated alignment, not automatic merge permission.

## Implementation and checks
- 15 tracked files, +94/-32: module/sum; frontend manifest/lock; test/CodeQL/release workflows; test Containerfile/runtime guard; AppImage Containerfile; main_test.go; README/CONTRIBUTING/Linux prerequisites/OpenSpec config. Taskfile/generated bindings untouched. Targeted go get/tidy/runtime lock only; unrelated dependencies unchanged.
- Clean RED compiled then failed 15 tests/subtests on old pins; GREEN passed 15. Frontend npm ci/build passed before/after, zero audit vulnerabilities. Unprefixed pkg-config discovery failed; authorized system PKG_CONFIG_PATH resolved it without source workaround. Writer strict runner and Go 477/race 112/frontend 107/vet/diff/format/bash syntax passed.
- Native working assessment unassessable due untracked task doc; unknown writer profile/outcome required independent verification. Verifier inspected the entire diff and repeated Go 477/11 packages, desktop race 112, frontend 107/11 files, vet/diff/bash syntax/gofmt; all passed, no blocking defect.
- A temporary isolated CLI beta.25 was built with Go 1.27.1; prefixed make build passed. Binary metadata confirms Go 1.27.1/Wails beta.25/x/sys .48, CGO Linux amd64. `bin/attack-shark-linux` SHA-256: 2d527509e267ad408219009ff3e0ba592ec5330df90d89964499464f41d1fd2b. Manifest/lock/installed runtime all beta.25. User-global CLI unchanged.
- Rootless Podman 6.1.2 built unique `attack-shark-da-test:go1271-beta25` and `attack-shark-da-appimage:go1271-beta25`. Actual Ubuntu 24.04 images run Go 1.27.1/GOTOOLCHAIN=local; AppImage CLI beta.25, Node 22.23.3, Task 3.53.1. Corrected single-context command used; parent prompt typo never executed. No retries/workarounds.
- Container task package built fresh unsigned `cmd/x6configurator/build/attack-shark-linux-x86_64.AppImage`, 85,760,504 bytes, mode 0755, SHA-256 cefeeba4aeacf21cfa569684216bad05b7b27d1202d4cefa324cf9990d9e4c95. Fresh AppDir WebKit helpers executable; follow-up `TestPackagedAppImageWebKitProcessesAreExecutable` passed 1 test and confirms executable helpers inside final-image extraction without GUI.
- Offline Go suite passed all 11 packages with network disabled, read-only container/repository and tmpfs caches (GOPROXY/GOSUMDB off); individual test count not enumerated. Git scope/index/HEAD unchanged; generated bindings, parent doc and global CLI checksums preserved.

## Limitations
String/whitespace alignment tests are brittle and comments can satisfy them; lock-version assertion is unscoped. Tests omit test-image PATH/GOTOOLCHAIN assertions, though actual image verified. Inherited amd64 AppImage assumptions remain. CLI upstream x/sys .46 differs from application .48, not a missed pin. Nonfatal unsigned packaging warnings: AppStream/GPG/copyright/strip/rpath. Remote CI/CodeQL, browser E2E, GUI/runtime/hardware and signing unrun. Build logs remain private local verification evidence.

## Next step
User authorized commit/review, issue creation/approval and PR. Complete duplicate/form/label checks before publication and exact native committed-candidate review. This coordinated PR covers Dependabot #117/#118/#121; only after merge and green CI reconcile them as superseded (not authorized to close now). Other #116/#119/#120/#122 remain separate and unintegrated. Exact release version/scope/notes/destination not granted yet. Keep partial manual refactor approval distinct from unexecuted new-dependency GUI/hardware tests. Preserve sibling worktrees.

## Commit and review
Functional work unit committed as `a9c892e753070a00ad6cb8b4aaa21babeade338f`, 16 files +126/-32 including this task record. Native high-risk four-lens review against the main baseline approved and exact acknowledgement completed under `review-34356c92caeb7b04`. Informational R2-001 at main_test.go:567-569 is nonblocking follow-up, not a correction route. Before authorized push/PR, private home paths and temporary CLI/log names were removed from this public document without changing implementation or test evidence. Issue/form/label prerequisites and remote CI remain pending.
