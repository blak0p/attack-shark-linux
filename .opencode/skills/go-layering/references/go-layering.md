# go-layering — boundary references (real repo excerpts)

## 1. `desktop` never imports `wails`

`internal/desktop/service.go:105` defines the emitter as an interface; only `cmd`
knows about Wails:

```go
// internal/desktop/service.go:105
// EventSink pushes live status updates to the frontend. The desktop service
// never imports Wails: the application wiring supplies the real emitter.
type EventSink interface {
	Emit(event string, payload any)
}
```

The concrete bridge lives in the composition root:

```go
// cmd/x6configurator/main.go:23
type wailsEventSink struct{ app *application.App }

func (s wailsEventSink) Emit(name string, payload any) { s.app.Event.Emit(name, payload) }
```

## 2. `desktop` receives the backend as an interface, not a concrete type

`cmd/x6configurator/main.go:91` shows the backend (a `*hidlinux.HidrawBackend`)
is handed to `composeDesktopService`, which only sees
`desktop.StatusReader`/`desktop.DPIWriter`:

```go
func composeDesktopService(dataDir string, backend *hidlinux.HidrawBackend) (*desktop.Service, *x6.Service) {
	...
	status := x6.NewService(backend)          // backend satisfies x6.StatusReader
	writer := x6.NewCommandService(backend)   // backend satisfies x6.DPIWriter
	...
}
```

## 3. `mouse` depends on interfaces, not on `hidlinux`

`internal/mouse/service.go:49` defines the contracts the inventory needs;
`hidlinux` implements them without `mouse` ever importing it:

```go
// internal/mouse/service.go:49
type InventorySource interface {
	Enumerate(context.Context) ([]transport.Candidate, error)
}
type TargetedCommand interface {
	SendAndAwaitBound(context.Context, Binding, []byte, func([]byte) bool) error
}
```

## 4. `protocol/x6` is pure and hardware-free

`internal/protocol/x6/dpi.go:1` declares a hardware-free package; no `hidlinux`,
no `transport`, no syscalls:

```go
// internal/protocol/x6/dpi.go:1
// Package x6 contains pure, hardware-free facts about the X6 protocol.
package x6
```

`internal/x6/dpi.go:16` re-exports and adapts without touching hidraw:

```go
func EncodeDPIReport(config DPIConfig) ([]byte, error) {
	report, err := protocol.EncodeDPIReport(config)
	...
}
```

## 5. `wails` is a `cmd`-only dependency

`go.mod:13` lists `github.com/wailsapp/wails/v3` as an indirect dependency; the
import appears only in `cmd/x6configurator/main.go:17`. No `internal/` package
imports it.
