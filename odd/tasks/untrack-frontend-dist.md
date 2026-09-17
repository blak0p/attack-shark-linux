# Untrack Generated Frontend Dist

## Objective

Remove the generated `cmd/x6configurator/frontend/dist/**` output from version control while keeping it available whenever the application is built. Future frontend output must be ignored, and frontend generation must happen before Go build, test, vet, Wails build, and release operations that depend on the embedded assets.

The maintainer-approved change must preserve `cmd/x6configurator/main.go`'s `//go:embed frontend/dist` contract and the existing Taskfile and CI behavior. The clean-checkout-like validation must prove that a checkout without tracked `dist` files can generate the assets before dependent Go operations.

## Repository Evidence and Expected Surfaces

The implementation is expected to touch only the following documentation/configuration/build-guidance surfaces, plus the three tracked generated files explicitly listed under the destructive boundary:

| Surface | Evidence and expected change |
|---|---|
| `.gitignore` | Currently force-includes `cmd/x6configurator/frontend/dist/` and its contents; replace that exception with an ignore rule for generated output. |
| `Makefile` | Defines `frontend-build`, `go-test`, `test`, `build`, and `check`; make dependent Go/build/check paths generate frontend assets first without changing their intended commands. |
| `cmd/x6configurator/Taskfile.yml` | `build` already depends on `build:frontend`; preserve this dependency and its `go build -o ../../bin/attack-shark-linux .` command. |
| `.github/workflows/test.yml` | Already builds frontend assets before `go build ./...`, `go vet ./...`, and `go test ./...`; preserve and make the ordering explicit/robust after `dist` is untracked. |
| `.github/workflows/release.yml` | Installs frontend dependencies, then runs `make check`; preserve release verification ordering and ensure the frontend is generated before all dependent operations. |
| `README.md` | Building and Testing sections currently show separate/manual frontend and Go commands; document the required generation-first workflow. |
| `CONTRIBUTING.md` | Setup, tests, and PR checklist currently show direct Go commands; document that frontend generation precedes Go build/test/release checks. |
| `docs/linux-usb-prerequisites.md` | Its build prerequisite includes `go build ./cmd/x6configurator`; document the frontend-generation prerequisite for that command. |
| `cmd/x6configurator/wails.json` | Configures `frontend.dir` and `npm run build`; preserve the existing Wails frontend build configuration and do not introduce recursive Wails builds. |
| `openspec/config.yaml` | Defines repository test, vet, and build commands; update configuration so its build/test contract reflects frontend generation before Go operations. |
| `.opencode/skills/wails-backend/SKILL.md` | Agent guidance currently describes `//go:embed frontend/dist` and runs Go build before the frontend build; update the guidance to require generation first while preserving the embedding contract. |
| `.opencode/skills/wails-backend/references/wails-backend.md` | Agent reference currently documents the embedded output and Wails configuration; update the build-order guidance without changing the runtime embedding model. |
| `cmd/x6configurator/main.go` and `cmd/x6configurator/main_test.go` | Preserve unchanged as the embedding implementation and its runtime/configuration assertions; they are evidence and compatibility boundaries, not expected edit surfaces. |
| `cmd/x6configurator/frontend/dist/index.html` | Currently tracked generated output; remove only under the destructive boundary below. |
| `cmd/x6configurator/frontend/dist/assets/index-CU8R5TZP.css` | Currently tracked generated output; remove only under the destructive boundary below. |
| `cmd/x6configurator/frontend/dist/assets/index-DG8SNgAN.js` | Currently tracked generated output; remove only under the destructive boundary below. |

No agent-guidance files were found at repository root; the Wails skill and its reference above are the evidence-backed agent-guidance surfaces.

## Ordered Checklist

- [x] 1. Confirm the baseline: exactly three tracked files exist below `cmd/x6configurator/frontend/dist/**`, `main.go` embeds `frontend/dist`, `frontend/package.json` writes Vite output there, and current Taskfile/CI frontend-before-Go behavior is understood. Baseline RED: `make -n go-test` and `make -n check` ran Go operations before frontend generation.
- [x] 2. Change ignore/configuration policy so generated `cmd/x6configurator/frontend/dist/**` is ignored and no longer force-included, without broadening the ignore rule to source, bindings, or unrelated build output.
- [x] 3. Make the Makefile's dependent workflows generation-first: frontend generation must precede Go build/test/vet/check and any release verification path that invokes them; retain named targets and existing tool commands where possible.
- [x] 4. Preserve `cmd/x6configurator/Taskfile.yml`'s `build:frontend` dependency and existing output command; reconcile only if required to make the documented clean-checkout ordering unambiguous. Preserved unchanged.
- [x] 5. Preserve CI and release behavior while making their frontend-generation-before-Go ordering explicit for `.github/workflows/test.yml` and `.github/workflows/release.yml`. Preserved unchanged: CI builds after `npm ci`, and release invokes the generation-first `make check`.
- [x] 6. Update `README.md`, `CONTRIBUTING.md`, and `docs/linux-usb-prerequisites.md` so build, test, and release-related instructions never imply that Go operations can run before frontend generation.
- [x] 7. Update `openspec/config.yaml`, `cmd/x6configurator/wails.json` only as needed for the generation-first contract; preserve the Wails `npm run build` configuration and non-recursive behavior. `wails.json` preserved unchanged.
- [x] 8. Update the Wails agent skill and reference to teach the same generation-first ordering, while retaining the `//go:embed frontend/dist` runtime contract and generated-binding guidance.
- [x] 9. Remove only the three currently tracked dist files listed above; do not remove generated directories or newly produced files beyond this exact set as part of the source change.
- [ ] 10. Validate a clean-checkout-like sequence: with tracked dist absent, install frontend dependencies, generate frontend output, then run the configured Go build/test/vet and frontend test checks; verify `go test ./cmd/x6configurator` still serves embedded assets and `main.go` remains unchanged. `npm ci && npm run build`, `go build ./...`, `go vet ./...`, `go test ./...`, `go test ./cmd/x6configurator`, and `npm test` passed; `make check` reached Wails build after those checks but cannot complete because `wails3` is not installed.
- [x] 11. Validate the final diff and status: the only deletion candidates are the three listed dist files, generated dist remains ignored after regeneration, Taskfile and CI ordering is preserved, and no unrelated files are changed.

## Acceptance Criteria

- `cmd/x6configurator/frontend/dist/**` is no longer versioned, and future generated output is ignored.
- Exactly these three currently tracked files are removed: `cmd/x6configurator/frontend/dist/index.html`, `cmd/x6configurator/frontend/dist/assets/index-CU8R5TZP.css`, and `cmd/x6configurator/frontend/dist/assets/index-DG8SNgAN.js`.
- Frontend generation is required before Go build/test/vet/check and release verification operations that depend on the embedded tree.
- Makefile, build/test documentation/configuration, CI/release configuration, and the evidence-backed Wails agent guidance describe the same ordering.
- `cmd/x6configurator/main.go` continues to embed `frontend/dist`; no workaround replaces or weakens the embed contract.
- `cmd/x6configurator/Taskfile.yml` retains its frontend build dependency and existing binary build behavior.
- A clean-checkout-like validation generates the ignored assets before dependent Go operations and passes the configured checks.
- No production behavior, frontend source, generated bindings, protocol/transport code, or unrelated generated artifacts are changed.

## Non-goals

- Do not change the Wails asset embedding mechanism or move the frontend output directory.
- Do not redesign frontend build tooling, package scripts, dependency versions, or hashed asset naming.
- Do not regenerate or hand-edit frontend bindings.
- Do not change application, protocol, HID, transport, desktop, or frontend source behavior.
- Do not alter release publishing semantics beyond required verification ordering.
- Do not add a cleanup mechanism that deletes arbitrary build output.

## Destructive Boundary

The only permitted destructive operation in implementation is removing these three currently tracked generated files:

1. `cmd/x6configurator/frontend/dist/index.html`
2. `cmd/x6configurator/frontend/dist/assets/index-CU8R5TZP.css`
3. `cmd/x6configurator/frontend/dist/assets/index-DG8SNgAN.js`

Do not delete any other file, directory, generated asset, source file, binding, configuration, or documentation artifact. Do not commit.

## Verification Plan

- `git ls-files 'cmd/x6configurator/frontend/dist/**'` reports no tracked files after the change.
- Ignore checks confirm generated files under `cmd/x6configurator/frontend/dist/**` are ignored.
- A fresh/clean-like run performs `cd frontend && npm ci && npm run build` before `go build ./...`, `go vet ./...`, and `go test ./...`.
- `cd frontend && npm test` passes, and `go test ./cmd/x6configurator` confirms the embedded runtime entry remains readable after generation.
- `git diff --name-status` and `git status --short` show only the approved documentation/configuration/build-guidance changes and the three approved generated-file deletions.
