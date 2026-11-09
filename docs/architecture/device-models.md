# Device models

The application is split into generic code and per-model drivers. Generic code
never imports a model; the composition root injects one.

## Layers

| Layer | Packages | Knows about |
|---|---|---|
| Generic | `internal/transport`, `internal/hidlinux`, `internal/mouse`, `internal/device`, `internal/macros`, `internal/desktop`, `internal/configstore` | HID transport, profiles/inventory, the model-neutral settings domain, the `device.Model` seam, the Wails-facing service, persistence |
| Model | `internal/attackshark/<model>`, `.../protocol`, `.../macrocodec` | Report encoding, ACK matching, VID/PID, HID usage, key table, validation limits, lighting catalog |
| Composition root | `cmd/x6configurator` | Builds the profiles, `Model()`, status/command services and Wails app; the only place that names the X6 |

Key seams:

- `mouse.ProfileRegistry` holds the registered `mouse.Profile`s (VID/PID, `HIDFacts`, feature reports). Generic code gets device identity from it, never from literals.
- `device.Model` is what `desktop.Service` needs from a driver: operations, validation, lighting effects, macro admission/encoding, `Name()` and `KeyCodes()`.
- `desktop.Service.DeviceInfo()` is the one Wails method that hands the frontend the model name and macro key table. The frontend hard-codes neither.

## Boundary tests

`internal/architecture` (`go test ./internal/architecture/ -v`) fails when:

- a package outside `internal/attackshark/...` and `cmd/...` imports `internal/attackshark/...` (the allowlist of known violations is empty);
- generic code contains model USB identity literals (`0x1d57`, `0xfa60`, ...);
- the legacy packages `internal/x6` or `internal/protocol/x6` reappear.

## Adding a model

1. Create `internal/attackshark/<model>` (plus `protocol`, and `macrocodec` if it has macros). Keep protocol and codec packages free of I/O.
2. Add a `mouse.Profile` constructor: VID/PID match, `HIDFacts`, feature reports. See `x6.NewProfile`.
3. Implement `device.Model` (`Name`, `KeyCodes`, operations, validation, macro methods) behind a `Model()` constructor. Test the exact bytes.
4. Register it in `cmd/x6configurator`: pass the profile to `mouse.NewProfileRegistry` and inject the model into `desktop.Compose`. Nothing outside `cmd` may import the new package.
5. Add udev rules for the new VID/PID under `packaging/udev` and update the policy tests.
6. Run `go test ./...` and `cd frontend && npm test`.

Persisted values (profile IDs, `model` name, configstore keys) are part of the
user's data; do not change them for an existing model.

## Open problem: shared VID/PID

The Attack Shark X11 SE dongle reports `1d57:fa60`, the same as the X6 dongle.
`mouse.ProfileRegistry` rejects duplicate VID/PID registrations, so a second
model cannot be registered for that ID today. How a model is resolved for a
shared ID (extra HID facts, a probe command, or a user choice) is undecided and
not solved here. The constraint kept for now: identity comes only from the
registered profiles, so a resolver can later be added in one place.
