# Cascading Remap Selector

## Objective
Replace the flat grouped remap popup with a two-level cascading selector: a category menu with **Basic** and **Multimedia**, and an adjacent submenu containing only the selected category's actions.

## Problem and Why
The current popup shows all remap actions in one list, separated only by group labels. The requested interaction is a compact category-first menu so users intentionally enter Basic or Multimedia before choosing an action.

## Scope
- Refactor the frontend-only `GnomeSelect` interaction and styles into an accessible cascading menu.
- Keep `ButtonRemapPanel`'s `onStage(button, action)` contract and all backend/protocol behavior unchanged.
- Preserve Button 1's visible but unavailable Multimedia actions and Buttons 2–7 eligibility.
- Add focused component and panel tests for pointer, keyboard, focus, close, and disabled-action behavior.

## Constraints
- Strict TDD is active from `openspec/config.yaml`.
- Run observed RED, then GREEN, TRIANGULATE, and REFACTOR evidence.
- Do not edit generated bindings, `frontend/src/wails-service.ts`, backend, protocol, transport, HID, or hardware paths.
- No live-device, hidraw, root, or USB operations.
- Work only in this isolated worktree and branch `feat/cascading-remap-selector`, based on `feat/multimedia-button-remapping`.
- The follow-up PR should target `feat/multimedia-button-remapping` because the UI depends on its unmerged action catalog.

## Interaction Decisions
- Opening the trigger presents only the Basic and Multimedia category entries; no category submenu is selected initially.
- Pointer click and hover can open a category submenu; click must work without hover.
- The submenu opens adjacent to the category menu and flips/repositions near the viewport edge.
- Arrow keys navigate the current menu level; Right/Enter opens a category, Left returns to the category list, Escape closes the submenu then the full selector, and Enter/Space selects an enabled action.
- The component uses coherent nested `menu` / `menuitem` semantics, visible focus styling, outside-click close, and focus restoration to the trigger.
- Events inside the selector must not trigger `ButtonRemapPanel` Apply or Discard shortcuts.

## Checklist

- [x] CR-1: Added isolated `GnomeSelect` tests for category-first rendering, portal placement, scroll repositioning, pointer behavior, keyboard traversal, closing, focus restoration, and disabled actions.
- [x] CR-2: Updated `ButtonRemapPanel` tests for Basic/Multimedia category navigation, Button 1 rejection, Buttons 2–7 staging, and Apply/Discard shortcut isolation.
- [x] CR-3: Implemented the smallest cascading menu state and ARIA behavior in `frontend/src/components/panels/GnomeSelect.tsx`.
- [x] CR-4: Updated `controls.css` for portal-safe category/submenu layout, focus, active state, disabled state, and viewport width constraints.
- [x] CR-5: Confirmed no `ButtonRemapPanel.tsx` adaptation was needed; its existing category metadata and staging callback contract remain intact.
- [x] CR-6: Focused selector/panel tests, full frontend tests, `go test ./...`, `go vet ./...`, and the configured production build passed; generated build output was restored and excluded from the candidate.

## Acceptance Criteria
- A selector opens to exactly Basic and Multimedia categories, and each category exposes only its own ordered actions in an adjacent submenu.
- Pointer and keyboard users can open, navigate, select, close, and return focus predictably.
- Button 1 Multimedia actions remain visible but cannot stage; eligible buttons can stage them.
- Selecting an action calls the existing staging callback once and does not perform device I/O.
- Apply, Discard, and DPI-marker behavior remains intact and keyboard events from the selector do not invoke panel shortcuts.
- No backend or generated-binding files change.

## Progress and Verification Evidence
- 2026-09-16: Read-only implementation map completed in the parent workspace. Candidate implementation files are `GnomeSelect.tsx`, `controls.css`, `ButtonRemapPanel.tsx`, and component/panel tests. Backend changes are not indicated.
- 2026-09-16: A bounded frontend writer added `GnomeSelect.test.tsx`, adapted panel coverage, implemented cascading-menu interaction in `GnomeSelect.tsx`, and updated selector CSS. `ButtonRemapPanel.tsx` and backend/generated paths remained unchanged.
- 2026-09-16: Strict-TDD execution is blocked by missing frontend executables in this fresh worktree: both focused and full `npm test` fail with exit 127 (`vitest: command not found`), and the frontend build fails with exit 127 (`vite: command not found`). No checklist item is accepted until these tests execute.
- 2026-09-16: `go test ./...` passed (313 tests in 8 packages) and `go vet ./...` passed. `go build ./...` passed before the frontend build failed because Vite is unavailable.
- 2026-09-16: `npm ci` restored 102 packages with no reported vulnerabilities. Focused and full frontend suites then executed but each had one failing assertion: the new test expects `Basic` / `Multimedia` while the rendered category labels intentionally include the submenu indicator (`Basic▸` / `Multimedia▸`).
- 2026-09-16: Manual desktop inspection found an enabled selector whose popup was visually clipped. The correction portals the popup to `document.body` with fixed viewport positioning and scroll repositioning, rather than relying on z-index inside a scrollable ancestor.
- 2026-09-16: Observed strict-TDD evidence: focused RED failed while the popup stayed inside the wrapper; focused GREEN passed (9 tests); triangulation added scroll-repositioning coverage and passed (10 tests). Focused panel/selector verification passed (10 tests); full frontend verification passed (86 tests); `go test ./...` passed (313 tests in 8 packages); `go vet ./...` passed.
- 2026-09-16: The configured frontend production build passed but rewrote tracked `cmd/x6configurator/frontend/dist/**` assets and added new hashed asset files. The maintainer explicitly authorized their restoration; the generated output is clean and excluded from the candidate.
- 2026-09-16: The maintainer manually verified the rebuilt desktop app: the category menu is compact and visible, and the adjacent action submenu adapts to available screen space.

## Next Step
Inspect and review the clean source/test/ODD candidate, then commit and open the stacked follow-up PR.