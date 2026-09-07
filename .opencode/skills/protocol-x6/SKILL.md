---
name: protocol-x6
description: "Trigger: encoding/decoding X6 HID reports, adding a new protocol feature (remap 0x08, lighting, polling, sleep), defining a CommandOperation, computing checksums, writing protocol tests. Keep protocol code pure and hardware-free."
license: MIT
metadata:
  author: blak0p
  version: "1.0"
---

## Activation Contract
Load when editing `internal/protocol/x6`, `internal/x6` report/operation code, or
adding a new X6 command/report type.

## Hard Rules
- `internal/protocol/x6` is **pure and hardware-free**: no `hidlinux`, no
  `transport`, no syscalls, no `wails`. It holds only facts about the wire format.
- **Encoders** return `([]byte, error)` and use an explicit length constant
  (e.g. `DPIReportLength = 52`). Return an error on any invalid field rather than
  producing a malformed report.
- **Decoders** return `(T, bool)`. Return `false` for any report that is not
  exactly the expected shape; never panic or partially decode.
- Every command kind exposes an **Operation adapter** in `internal/x6` via
  `New*Operation() mouse.CommandOperation` with `Validate(value any)`,
  `Encode(value any)`, and `MatchesACK(report []byte)`
  (`internal/x6/polling.go:41`, `internal/x6/lighting.go:83`). The adapter wraps
  the pure `protocol` functions and type-asserts.
- Checksums are computed over a fixed byte range and split into high/low bytes
  (`internal/protocol/x6/dpi.go:45`). Preserve the exact range and byte order
  when adding a report.
- Validation lives in two places: field sanity in `protocol.Encode*`, and
  semantic validation in the `Operation.Validate` (often delegating to
  `protocol.Validate*`).
- Tests are table-driven and **hardware-free**: assert exact report bytes/length
  and reject invalid inputs (see `internal/x6/dpi_tdd_test.go`).

## Frontier bar
- A frontier model adds a feature by (1) pure encode/decode in `protocol/x6`,
  (2) a typed `Operation` adapter in `x6`, (3) tests asserting byte-exact output.
- It never puts I/O or device knowledge in `protocol/x6`.
- It keeps `(T, bool)` decode contracts strict and documents the report ID and
  length constants.
- It validates both directions and includes a "rejects invalid input" case.

## Decision Gates
| You need to… | Pattern |
| Add a report type | `Encode*Report` + `DPIReportLength`-style constant in `protocol/x6` |
| Decode a dongle push | `Decode*Report(...) (T, bool)` returning false on mismatch |
| Expose command to `mouse`/`desktop` | `New*Operation() mouse.CommandOperation` in `x6` |
| Verify ACK | `Matches*ACK(report []byte) bool` |
| Compute checksum | sum fixed range, split hi/lo bytes |

## Execution Steps
1. In `protocol/x6`, add the encoder (with length constant + checksum) and decoder
   (`(T, bool)`).
2. In `x6`, add `New*Operation()` returning a `mouse.CommandOperation` whose
   methods delegate to `protocol` after a type assertion.
3. Add field/semantic validation where it belongs (`protocol.Encode*` vs
   `Operation.Validate`).
4. Add table-driven, hardware-free tests asserting exact bytes and rejection.
5. Run `go test ./internal/protocol/... ./internal/x6/...`.

## Output Contract
Report the report ID/length added, the `Operation` adapter, checksum range, and
the tests added. Confirm `protocol/x6` has zero hardware imports.

## References
- [references/protocol-x6.md](references/protocol-x6.md) — real encode/decode/operation excerpts.
