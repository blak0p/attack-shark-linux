# snapshot-dto — DTO references (real repo excerpts)

## 1. Desktop owns its DTOs, separate from domain types

`internal/desktop/service.go:31` defines the API `DPIConfig`, distinct from
`x6.DPIConfig`:

```go
type DPIConfig struct {
	AngleControl, RippleControl bool
	StageMask, LiftDistance     byte
	DPI                         [8]int
	ActiveStage                 byte
	Colors                      [8][3]byte
}
```

The snapshot bundles applied/pending/factory + live fields
(`internal/desktop/service.go:47`):

```go
type Snapshot struct {
	Connection     string
	Battery        *int
	Applied        DPIConfig
	Pending        DPIConfig
	Factory        DPIConfig
	Revision       uint64
	Error          Error
	Firmware       string
	Persistence    string
	RetryAvailable bool
	ObservedStage  *int
	ObservedDPI    *int
}
```

`*int` distinguishes "no observation yet" from a zero value.

## 2. Explicit, centralized conversion

`internal/desktop/service.go:1052` is the only place DPI domain↔DTO happens:

```go
func ToDTO(config x6.DPIConfig) DPIConfig {
	return DPIConfig{config.AngleControl, config.RippleControl, config.StageMask, config.LiftDistance, config.DPI, config.ActiveStage, config.Colors}
}
func fromDTO(config DPIConfig) x6.DPIConfig {
	return x6.DPIConfig{AngleControl: config.AngleControl, RippleControl: config.RippleControl, StageMask: config.StageMask, LiftDistance: config.LiftDistance, DPI: config.DPI, ActiveStage: config.ActiveStage, Colors: config.Colors}
}
```

## 3. Snapshots are taken under lock, returned as value copies

`internal/desktop/service.go:996` copies while holding the lock, then returns:

```go
func snapshotOf(state *deviceState) Snapshot {
	state.mu.Lock()
	defer state.mu.Unlock()
	return snapshotLocked(state)
}

func snapshotLocked(state *deviceState) Snapshot {
	return Snapshot{Connection: string(state.connection), Battery: state.battery, Applied: ToDTO(state.applied), Pending: ToDTO(state.pending), Factory: ToDTO(state.factory), Revision: state.revision, Error: state.err, Firmware: state.firmware, Persistence: state.persistence, RetryAvailable: state.retry != nil, ObservedStage: state.observedStage, ObservedDPI: state.observedDPI}
}
```

## 4. Event payloads are immutable copies

`internal/desktop/service.go:1028` deep-copies the applied lighting selection so
the emitted `LightingSnapshot` cannot alias internal state:

```go
func lightingSnapshotLocked(state *lightingState) LightingSnapshot {
	var applied *x6.LightingSelection
	if state.applied != nil {
		copy := *state.applied
		applied = &copy
	}
	return LightingSnapshot{Pending: state.pending, Applied: applied, Effects: x6.LightingEffects(), Revision: state.revision, Firmware: state.firmware, Error: state.err}
}
```

And `x6.LightingEffects()` deep-copies the catalog (`internal/x6/lighting.go:72`):

```go
func LightingEffects() []LightingEffect {
	effects := make([]LightingEffect, len(lightingEffects))
	for i, effect := range lightingEffects {
		effects[i] = effect
		effects[i].SpeedVariants = append([]LightingSpeedVariant(nil), effect.SpeedVariants...)
		effects[i].ColorTemplates = append([]LightingColorTemplate(nil), effect.ColorTemplates...)
	}
	return effects
}
```
