# Apply Progress

## RED

- Added protocol, desktop, and panel coverage before production edits.
- `go test ./internal/protocol/x6 ./internal/x6 ./internal/desktop` failed as expected because the five new constants were undefined.
- Focused frontend test execution initially could not run because `vitest` was unavailable; this environment was restored before the final frontend validation.

## GREEN and Triangulation

- Added the five closed actions and Button 1 validation before pending/applied/revision/config changes, encoding, persistence, transport, and device I/O. Established desktop semantics may still publish the typed invalid-configuration failure status.
- Verified all 30 Buttons 2–7/action encodings at the protocol layer and all five Button 1 rejections in protocol and desktop-boundary tests.
- Focused frontend tests now verify Mouse Controls category order, Button 1 pointer and trigger-keyboard blocking, eligible staging, and targeted DPI-marker clearing: 14 tests passed.
- `cd frontend && npm test` passed: 9 files, 89 tests. `cd frontend && npm run build` passed using the repository's configured Vite script.

## Diff budget

- Source/test diff: 137 changed lines (120 additions, 17 deletions), below the 400-line limit.
- No generated bindings, transport, HID, persistence, reset, or report infrastructure was changed.
