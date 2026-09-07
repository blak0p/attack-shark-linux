---
name: wails-backend
description: "Trigger: registering a Wails v3 service, adding a binding method, emitting events to the frontend, configuring the window/assets, regenerating Go->TS bindings. Follow the backend integration contract."
license: MIT
metadata:
  author: blak0p
  version: "1.0"
---

## Activation Contract
Load when editing `cmd/x6configurator/main.go`, adding an exported method to a
Wails service (`internal/desktop/service.go`), emitting live events, or changing
`wails.json` / frontend bindings.

## Hard Rules
- Register exactly one backend service through `application.NewService(service)`
  in `application.Options.Services`. The service is a plain Go struct whose
  **exported** methods become JS bindings.
- The service must be Wails-agnostic: it emits events through the `EventSink`
  interface (`desktop.EventSink.Emit(name, payload)`). The concrete emitter
  (`wailsEventSink`) is wired only in `cmd`. Never import `wailsapp/wails/v3`
  from `internal/desktop`.
- Lifecycle: cancel background work in `application.Options.OnShutdown`. The
  status listener is started with a `context.WithCancel` created in `main` and
  stopped on shutdown (`cmd/x6configurator/main.go:53-68`).
- Window + assets: `app.Window.NewWithOptions(application.WebviewWindowOptions{...})`,
  `window.SetURL("/")`, `application.AssetOptions{Handler: application.AssetFileServerFS(assets)}`
  fed by `//go:embed frontend/dist`.
- **Never edit `frontend/bindings/**`** — it is generated. After changing a
  service's exported methods or DTOs, regenerate with `wails generate` (or rely
  on `npm run build` producing fresh bindings via the Wails v3 binding step).
- Event names are stable string contracts consumed by the frontend
  (`mouse:status`, `mouse:configuration`, `mouse:polling-configuration`). Treat
  them as API; do not rename without updating `frontend/src/wails-service.ts`.

## Frontier bar
- A frontier model keeps the App object free of business state: state lives in
  the injected service, the App only owns window/event plumbing.
- It cancels the listener on shutdown (no leaked goroutines) and starts it once.
- It returns typed snapshots/DTOs (see `snapshot-dto`), never raw domain structs.
- It updates `wails-service.ts` together with the backend so the facade and the
  bindings never drift.

## Decision Gates
| You need to… | Pattern |
| Expose a capability to the UI | add exported method on `desktop.Service` |
| Push live data | `EventSink.Emit("mouse:...", payload)` from `desktop` |
| Receive live data in React | `Events.On("mouse:...", e => cb(e.data))` |
| Change method signature | regenerate bindings + update `wails-service.ts` |

## Execution Steps
1. Add/modify the exported method on `desktop.Service`; return a DTO/snapshot.
2. Emit via `s.events.Emit(name, payload)` (guarded by a nil check).
3. In `cmd`, ensure the real emitter is attached (`AttachListener`) and the
   listener is started/stopped with the app lifecycle.
4. Regenerate bindings; update `frontend/src/wails-service.ts` facade.
5. Run `go build ./...` and `(cd frontend && npm run build)`.

## Output Contract
Report the method/binding added, the event name emitted, the lifecycle wiring
touched, and confirm `internal/desktop` has no `wails` import and bindings were
regenerated.

## References
- [references/wails-backend.md](references/wails-backend.md) — real App/service wiring.
