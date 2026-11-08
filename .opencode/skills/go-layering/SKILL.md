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
   ├── hidlinux     (hidraw implementation; depends on transport + mouse interfaces)
   ├── mouse        (profiles, registry, inventory, targeted command, Batch)
   ├── macros       (model-neutral local macro library)
   ├── device       (settings domain + device.Model seam; depends on mouse, macros)
   ├── attackshark/<model>[/protocol|/macrocodec]  (per-model drivers; implement device.Model)
   ├── desktop      (service, sync coordinator — the Wails-facing contract; takes a device.Model)
   └── cmd/x6configurator  (composition root: the only place that wires a model + Wails App)
```

| Package | MAY import | MUST NOT import |
|---|---|---|
| `transport` | stdlib only | `hidlinux`, `attackshark/...`, `mouse`, `desktop`, `wails` |
| `attackshark/<model>/protocol`, `/macrocodec` | stdlib, `macros` | `hidlinux`, `transport`, `mouse`, `desktop`, `wails` |
| `attackshark/<model>` | own `protocol`/`macrocodec`, `device`, `macros`, `mouse` | `hidlinux`, `desktop`, `wails` |
| `hidlinux` | `transport`, `mouse` | `attackshark/...`, `desktop`, `wails` |
| `mouse` | `transport` | `attackshark/...`, `hidlinux`, `desktop`, `wails` |
| `device` | `mouse`, `macros` | `attackshark/...`, `hidlinux`, `desktop`, `wails` |
| `desktop` | `device`, `mouse`, `macros`, `configstore`, `transport`, `hidlinux` (types only) | `attackshark/...`, `wails` — use the `EventSink` interface |
| `cmd/x6configurator` | everything + `wailsapp/wails/v3` | — |

`internal/architecture` enforces that nothing outside `attackshark/...` and `cmd/...` imports a model. See `docs/architecture/device-models.md`.

## Frontier bar
- A frontier model never reaches "upward" or "sideways" for a concrete type when an interface will do. `desktop` receives the model as `device.Model` and the backend as `desktop.StatusReader`/`desktop.DPIWriter`, not as `*hidlinux.HidrawBackend`.
- `desktop` contains **zero** `wailsapp/wails/v3` imports. The real event emitter is injected in `cmd` via the `EventSink` interface (`internal/desktop/service.go:105`).
- `attackshark/<model>` and its `protocol` are hardware-free and testable without a device. Any HID/ioctl detail stays in `hidlinux`.
- When a cycle seems necessary, extract a local interface or pass a function/seam — do not import the higher layer.

## Decision Gates
| You need to… | Do this |
| Send live updates to the UI | define/use `EventSink` in `desktop`; implement it in `cmd` |
| Call hardware from a model/`mouse` | depend on a `transport`/`mouse` interface, not `hidlinux` |
| Add protocol knowledge | put it in `attackshark/<model>/protocol` (pure); adapt via the model's Operation |
| Wire concrete backends and models | only in `cmd/x6configurator/main.go` |

## Execution Steps
1. Decide which layer owns the new behavior.
2. Check the table above; if the import would cross a "MUST NOT" edge, inject an interface instead.
3. Keep `attackshark/<model>/protocol` and the model package free of I/O; keep `desktop` free of `wails`.
4. Do all concrete wiring (model, backend, App, window, emitter) in `cmd/x6configurator/main.go`.
5. Run `go build ./...` and `go vet ./...`.

## Output Contract
Report packages touched, the import direction honored, any new interface introduced to avoid a forbidden import, and confirm `desktop` has no `wails` import.

## References
- [references/go-layering.md](references/go-layering.md) — real boundary excerpts.
