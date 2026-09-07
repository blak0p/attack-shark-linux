---
name: error-handling
description: "Trigger: returning errors from the desktop service or domain, mapping failures to the UI, defining error codes, wrapping/classifying errors. Follow the typed error taxonomy."
license: MIT
metadata:
  author: blak0p
  version: "1.0"
---

## Activation Contract
Load when a function in `internal/desktop`, `internal/x6`, `internal/mouse`, or
`internal/hidlinux` returns an error that may reach the UI, or when adding a new
failure mode.

## Hard Rules
- The desktop layer reports failures as a typed `Error{ Code ErrorCode }`, never
  as a raw Go `error` string. `ErrorCode` is a closed string enum
  (`internal/desktop/service.go:16`).
- Every error path funnels through `errorCode(err, isStatus)` so the same root
  cause always maps to the same code (`internal/desktop/service.go:1058`).
- Use package **sentinel errors** with `errors.New` and compare with
  `errors.Is` — never by substring. Domain sentinels: `mouse.ErrSelectionRequired`,
  `mouse.ErrStaleBinding`, `mouse.ErrRevisionChanged`, `os.ErrPermission`,
  `x6.IsErrorKind(err, x6.PersistFailure)`, `x6.IsErrorKind(err, x6.ReadFailure)`,
  `x6.IsErrorKind(err, x6.AckFailure)`.
- Wrap at the boundary with `%w` so the chain survives, but the **UI only sees the
  `ErrorCode`**. Do not leak internal error text or stack to the frontend.
- Classify transport failure flavor for logs: `hidlinux.IsErrorKind(err, hidlinux.Timeout)`
  or `hidlinux.DiagnosticOperation(err)` before falling back to `x6.IsErrorKind(err, x6.AckFailure)`
  (`internal/desktop/service.go:1080`).
- Domain errors are typed (`x6.ServiceError{Kind, Err}`) so `IsErrorKind` can
  inspect them without exporting every cause.

## Frontier bar
- A frontier model never returns `error` types across the Wails boundary; it maps
  to `ErrorCode` deterministically.
- It preserves the wrapped chain for `errors.Is`/`As` even while showing only the
  code to users.
- It adds a new `ErrorCode` constant (and maps it in `errorCode`) rather than
  overloading an existing one.
- It logs with structured context (`slog.Error(..., "classification", ...)`) but
  never spills internals into the UI payload.

## Decision Gates
| Failure origin | Map to |
| No device / inventory unavailable | `DeviceUnavailable` |
| Permission on hidraw | `PermissionDenied` |
| Status read failed | `StatusReadFailed` (status path) |
| Invalid staged config | `InvalidConfiguration` |
| Apply/write failed | `ApplyFailed` |
| Persistence failed | `PersistenceFailed` |
| No selection | `SelectionRequired` |
| Binding changed under us | `StaleBinding` |
| Ambiguous device identity | `AmbiguousIdentity` |
| Legacy migration failed | `MigrationFailed` |

## Execution Steps
1. Identify the failure and choose (or add) the correct `ErrorCode`.
2. Return/store `Error{Code: ...}` in the relevant snapshot/state.
3. If wrapping a dependency error, use `%w`; classify via `errors.Is`/`IsErrorKind`.
4. Log with `slog` + classification; keep internals out of the UI payload.
5. Run `go build ./...`.

## Output Contract
Report the `ErrorCode` chosen, where it is set, the sentinel/`IsErrorKind` used to
classify, and confirm no raw error text crosses to the frontend.

## References
- [references/error-handling.md](references/error-handling.md) — real taxonomy excerpts.
