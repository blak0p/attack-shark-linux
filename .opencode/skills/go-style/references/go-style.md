# go-style — idiomatic Go references (real repo excerpts)

## 1. Wrap errors at the boundary, classify with sentinels

`internal/desktop/service.go` wraps and classifies instead of string-matching:

```go
// internal/desktop/service.go:1058
func errorCode(err error, status bool) ErrorCode {
	if errors.Is(err, mouse.ErrSelectionRequired) {
		return SelectionRequired
	}
	if errors.Is(err, mouse.ErrStaleBinding) {
		return StaleBinding
	}
	if errors.Is(err, os.ErrPermission) {
		return PermissionDenied
	}
	if x6.IsErrorKind(err, x6.PersistFailure) {
		return PersistenceFailed
	}
	...
}
```

The domain layer preserves the chain with `%w` (`internal/x6/dpi.go:16`):

```go
func EncodeDPIReport(config DPIConfig) ([]byte, error) {
	report, err := protocol.EncodeDPIReport(config)
	if err != nil {
		return nil, &ServiceError{InvalidDPI, fmt.Errorf("%w", err)}
	}
	return report, nil
}
```

## 2. Define local, minimal interfaces at the call site

`internal/hidlinux/hidraw_passive.go` depends on tiny interfaces, not concrete
types, so the OS opener is swappable:

```go
// internal/hidlinux/hidraw_passive.go:26
type hidrawNode interface {
	io.ReadCloser
	SendFeatureReport(report []byte) (int, error)
}

type hidrawNodeOpener interface {
	OpenNode(path string) (hidrawNode, error)
}
```

The same discipline lives in `internal/mouse/service.go:49` (`InventorySource`,
`ProfileValidator`, `TargetedCommand`) and `internal/transport/contracts.go:30`
(`PassiveInputTransport`, `CommandTransport`).

## 3. Context first, honor cancellation

Every I/O path takes `context.Context` as the first argument —
`mouse.TargetedCommand.SendAndAwaitBound`, `x6.Service.Status`,
`desktop.Service.RefreshStatus`, `application` lifecycle in `cmd/.../main.go`.
The process root is the only `context.Background()`/`context.WithCancel` site
(`cmd/x6configurator/main.go:53`).

## 4. Value mutexes, document lock order

`internal/mouse/service.go:65` and `internal/desktop/service.go:135` declare
mutexes as values inside the struct. When two locks exist (state + service), the
order is always **state lock, then service lock**, never reversed
(`internal/desktop/service.go:436`, `internal/mouse/service.go:222`).

## 5. Copy before crossing a boundary

`desktop.LightingEffects()` returns a deep copy so callers cannot mutate the
internal catalog (`internal/x6/lighting.go:72`); `snapshotLocked` copies under
lock before handing data to the UI (see `snapshot-dto`).

## 6. Naming without stutter

`DPIConfig` lives in package `x6` (`internal/x6/dpi.go:10` re-exports
`protocol.DPIConfig` as `x6.DPIConfig`) — not `x6.X6DPIConfig`. Getters are
unprefixed (`Store.Load`, `Service.Selection`).
