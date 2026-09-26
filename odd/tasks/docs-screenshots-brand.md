# Update Documentation, Screenshots Assets and Gentle-AI Brand

Repository-relative file locator: `odd/tasks/docs-screenshots-brand.md`

## Objective

Update `README.md` and repository documentation with the official "Built with Gentle-AI" badge, establish a structured screenshot asset directory, create a Linux configurator User Guide (`docs/user-guide.md`) with UI walkthroughs and screenshot anchors, and update cross-document navigation.

## Authorized Scope

- Add the official Gentle-AI badge to `README.md`.
- Establish `docs/assets/screenshots/` with designated image paths for key UI views (DPI, Polling, Lighting, Button Remap, Sleep/Power).
- Create `docs/user-guide.md` covering all desktop configurator features and settings with screenshot placements.
- Update documentation index table in `README.md` to link all user and developer guides.
- Coordinate screenshot filenames with user and verify rendered links.

## Constraints

- ODD workflow on feature branch `docs/screenshots-brand`.
- Strict TDD mode active in project configuration; for pure documentation tasks, run Markdown link/lint and test suite verification (`npm test` and `go test ./...` if affected).
- Conventional commits for every work unit. No AI attribution or Co-Authored-By lines in commit messages.
- Preserve existing release, protocol, and prerequisite documentation accuracy.

## Work Units

- [x] DOCS-1: Add Gentle-AI badge, hero banner layout, and screenshot gallery section to `README.md`.
- [x] DOCS-2: Create `docs/assets/screenshots/` directory structure and write `docs/user-guide.md` with UI feature breakdown and screenshot targets.
- [x] DOCS-3: Update documentation catalog in `README.md` and cross-references.
- [x] DOCS-4: Integrate user screenshots into `docs/assets/screenshots/`, verify paths and run repository verification checks.

## Acceptance and Checks

- README renders the Gentle-AI badge image and link correctly.
- `docs/user-guide.md` accurately documents the Linux Wails/React GUI features.
- All internal Markdown links resolve without dead references.
- `git diff --check` and relevant repository checks pass cleanly.
- Visual inspection approved by user via preview server.

## Progress

- Branch `docs/screenshots-brand` created from `main`.
- All 5 user-provided screenshots ingested into `docs/assets/screenshots/`.
- `README.md` updated with Gentle-AI brand badge, hero preview, visual screenshot gallery, and user guide link.
- `docs/user-guide.md` created with comprehensive feature breakdown for all 5 UI views.
- Verified cleanly with `git diff --check` and test suites.
- Approved by maintainer.
