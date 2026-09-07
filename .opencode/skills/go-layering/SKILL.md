---
name: go-layering
description: "Trigger: adding a package, changing imports, wiring dependencies, splitting layers, respecting package boundaries. Enforce the dependency direction of the Go+Wails backend."
license: MIT
metadata:
  author: blak0p
  version: "1.0"
---

## Activation Contract
Load when creating a package, editing imports, wiring `cmd/x6configurator/main.go`, or moving logic between layers.

## Hard Rules — dependency direction
The graph is acyclic and points **inward to the domain, outward to the App**:

```
transport  (generic HID identities, no hardware)
   ├── protocol/x6  (pure X6 protocol facts, no hardware)
   ├── hidlinux     (hidraw implementation; depends on transport + mouse interfaces)
   ├── x6           (domain: DPI/lighting/polling config + reports)
   ├── mouse        (profiles, inventory, targeted command)
   ├── desktop      (service, sync coordinator — the Wails-facing contract)
   └── cmd/x6configurator  (composition root: wires everything + Wails App)
```

| Package | MAY import | MUST NOT import |
|---|---|---|
| `transport` | stdlib only | `hidlinux`, `x6`, `mouse`, `desktop`, `wails` |
| `protocol/x6` | stdlib only | `hidlinux`, `transport`, `mouse`, `desktop`, `wails` |
| `hidlinux` | `transport`, `mouse` | `x6` (domain), `desktop`, `wails` |
| `x6` | `protocol/x6`, `mouse` | `hidlinux`, `transport`, `desktop`, `wails` |
| `mouse` | `transport`, `x6` (operations) | `hidlinux`, `desktop`, `wails` |
| `desktop` | `x6`, `mouse`, `configstore`, `transport`, `hidlinux` (types only) | `wails` — use the `EventSink` interface |
| `cmd/x6configurator` | everything + `wailsapp/wails/v3` | — |

## Frontier bar
- A frontier model never reaches "upward" or "sideways" for a concrete type when an interface will do. `desktop` receives the backend as `desktop.StatusReader`/`desktop.DPIWriter`, not as `*hidlinux.HidrawBackend`.
- `desktop` contains **zero** `wailsapp/wails/v3` imports. The real event emitter is injected in `cmd` via the `EventSink` interface (`internal/desktop/service.go:105`).
- `x6` and `protocol/x6` are hardware-free and testable without a device. Any HID/ioctl detail stays in `hidlinux`.
- When a cycle seems necessary, extract a local interface or pass a function/seam — do not import the higher layer.

## Decision Gates
| You need to… | Do this |
| Send live updates to the UI | define/use `EventSink` in `desktop`; implement it in `cmd` |
| Call hardware from `x6`/`mouse` | depend on a `transport`/`mouse` interface, not `hidlinux` |
| Add protocol knowledge | put it in `protocol/x6` (pure); adapt via an `x6` Operation |
| Wire concrete backends | only in `cmd/x6configurator/main.go` |

## Execution Steps
1. Decide which layer owns the new behavior.
2. Check the table above; if the import would cross a "MUST NOT" edge, inject an interface instead.
3. Keep `protocol/x6` and `x6` free of I/O; keep `desktop` free of `wails`.
4. Do all concrete wiring (backend, App, window, emitter) in `cmd/x6configurator/main.go`.
5. Run `go build ./...` and `go vet ./...`.

## Output Contract
Report packages touched, the import direction honored, any new interface introduced to avoid a forbidden import, and confirm `desktop` has no `wails` import.

## References
- [references/go-layering.md](references/go-layering.md) — real boundary excerpts.
