# Release v1.2.0 — Stable AppImage

## Objective

Publish `v1.2.0` as one stable GitHub Latest release with a Linux AppImage, its USB and hidraw udev rule assets, a user-local curl installer, and a signed self-update path.

## Authorized Scope

- Package Wails with `wails3 package GOOS=linux`, a `.desktop` entry, and the source icon `/home/alejandro/Descargas/attack-shark-x6.svg`.
- Attach both canonical udev rules to the release and install them only through an explicit-privilege installer flow.
- Render distinct `EACCES` udev-recovery and `ENOENT` disconnected-dongle feedback.
- Keep macros and on-device profiles visibly unavailable without implementing their protocol features.
- Add a startup, user-approved AppImage update with an Ed25519-signed manifest; add a curl installer, version/release automation, README, and explicit stable-release limits.

## Constraints

- ODD continuity record; strict TDD is active from `openspec/config.yaml` using `cd frontend && npm ci && npm run build && cd .. && go test ./...`.
- Never hand-edit generated bindings; regenerate both trees only after reflected contract changes.
- The updater changes only the user-owned AppImage under `~/.local/share/attack-shark-x6/`; it never elevates or replaces udev rules. The installer uses `sudo` only for udev install/reload after explicit confirmation.
- No hardware write, root app execution, tag, push, release, system udev mutation, or GitHub-secret provisioning without the separately required approval.
- Preserve unrelated untracked `.pi/`, `.rtk/`, `.opencode/skills/ui-ux-pro-max/scripts/__pycache__/`, and `design/` paths.
- Do not release the known ineffective `99-...` udev policy: confirm the canonical policy/order before release assets are assembled.

## Decisions

- [x] REL-DEC-1: Ed25519-signed manifest; app validates it before replacement.
- [x] REL-DEC-2: user-local installation and desktop entry.
- [x] REL-DEC-3: GitHub Actions signs automatically using a private-key secret; provisioning it is a separate explicit remote action.

## Work Units

- [ ] REL-1: Version source and signed update contract; strict TDD. Independently verified; awaiting feature-branch and user-approved work-unit commit.
- [ ] REL-2: AppImage packaging, `.desktop`, and icon assets.
- [ ] REL-3: Release workflow with AppImage, both rules, signed metadata, latest stable release notes.
- [ ] REL-4: Idempotent curl installer; user-approved udev installation/reload, manual fallback.
- [ ] REL-5: EACCES/ENOENT desktop/UI propagation and tests.
- [ ] REL-6: Explicit disabled macro/profile UI and tests.
- [ ] REL-7: README, prerequisites, security/release docs and limits.
- [ ] REL-8: After every source/doc task is ready for `main`, rehearse a local-only update from `v1.2.0-rc.1` to `v1.2.0-rc.2` using locally signed artifacts and an isolated XDG/home. Do not publish an RC or let a production build use a local/HTTP feed.
- [ ] REL-9: Full checks, clean-like package inspection, local RC rehearsal evidence, then explicit publication approval.

## Acceptance and Checks

- Clean-like `wails3 package GOOS=linux` produces AppImage with correct desktop name/icon/category.
- The release has AppImage plus both canonical `.rules`; `v1.2.0` is annotated, Latest, and not prerelease; notes separate working features from macros, on-device profiles, and DEB/RPM/AUR.
- AppImage updates require signed manifest validation and user approval; curl install is idempotent and provides a manual fallback.
- A local-only RC rehearsal proves the update UI with `rc.1` → `rc.2`, an isolated home/XDG environment, and a signed local test feed. It must not create a GitHub prerelease or weaken the production feed policy.
- Run targeted Go/Vitest, `make check`, configured Go/frontend checks, clean-like package build, and artifact inspection. System udev testing requires separate approval and rollback plan.

## Progress

- REL-1 initial writer: `internal/update/contract.go` and `contract_test.go`; RED for missing symbols, GREEN 20 tests, then 21 focused; configured full suite passed 344 Go tests.
- First independent review found three defects: uint64 overflow in prerelease comparisons, optional approval API, and normalized version text in signed bytes.
- Correction: arbitrary-length prerelease numbers compare by digit length then lexically; only the approval-gated API returns a replacement candidate; signed bytes preserve exact version text. Correction RED failed while `VerifyManifest` returned a release; GREEN passed 23 focused tests; configured full suite passed 346 Go tests.
- Native assessment returned no output and is unassessable; its high-risk plan mandated a second independent verification. It passed: 23 focused tests and `git diff --check` passed with no findings. REL-1 now awaits an approved feature-branch commit; default delivery is ask-on-risk.
