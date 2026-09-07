---
name: hidraw-safety
description: "Trigger: reading from or writing to the mouse, opening hidraw nodes, sending feature reports, handling udev/permissions, USB interface claims. Enforce hardware-safety: passive reads, writes on demand, least privilege."
license: MIT
metadata:
  author: blak0p
  version: "1.0"
---

## Activation Contract
Load when touching `internal/hidlinux`, any write path (`Apply*`, `Stage*`),
hidraw node opening, or udev/permission handling.

## Hard Rules
- **Reads are passive; writes happen only on explicit user demand.** Status reads
  and the always-on listener are read-only. A configuration reaches the device
  only through `ApplyDPI`/`ApplyLighting`/`StagePollingRate`→apply, never
  implicitly or on a timer without a prior user `Stage*`.
- **Never claim, detach, reset, rebind, or otherwise take a USB interface from
  the kernel.** Use the kernel-backed validated `/dev/hidrawN` node. Opening it
  `O_RDWR` does not detach `usbhid` (`internal/hidlinux/hidraw_passive.go:39`).
- Feature reports go through `HIDIOCSFEATURE` on the opened node; `buf[0]` is the
  report ID (`internal/hidlinux/hidraw_passive.go:57`).
- Open only the **validated** node the inventory selected — never enumerate-and-
  grab the first match. The binding carries the exact path/revision and is
  revalidated before each operation (`mouse.TargetedCommand.SendAndAwaitBound`).
- **Least privilege:** the udev policy grants the active seat user via
  `TAG+="uaccess"` with mode `0660`. Never set `0666` (world-writable) and never
  run the app as root. Document this loudly wherever permissions are handled.
- Writes are idempotent and bounded: send one report, await its ACK
  (`MatchesACK`), time out via `context` (not a bare sleep).
- No proprietary material (the official Windows app, captured firmware) is ever
  committed — local research reference only.

## Frontier bar
- A frontier model treats the device as a shared, unowned peripheral: it reads
  without side effects and writes only what the UI explicitly asked for.
- It never surprises the user's cursor/input by detaching the kernel driver.
- It fails closed on permission errors with a clear `PermissionDenied` code and a
  pointer to the udev setup, never silently retrying as root.
- It validates the binding on every operation so a re-plugged device can't receive
  a stale write.

## Decision Gates
| You need to… | Do this |
| Read live status | passive interrupt-in / feature read on the validated node |
| Send a config | `SendAndAwaitBound` with the captured `Binding` + ACK check |
| Open the node | `os.OpenFile(path, os.O_RDWR, 0)` on the selected path |
| Handle no access | return `PermissionDenied`, log udev hint, never root |

## Execution Steps
1. Confirm the operation is user-demanded (trace back to a `Stage*`/`Apply*`).
2. Use the binding's exact path; revalidate it is still current.
3. Open `O_RDWR`, send the feature report with `buf[0]` = report ID, await ACK
   under a `context` deadline.
4. On `os.ErrPermission`, map to `PermissionDenied` and surface the udev hint.
5. Run `go build ./...`; manual device tests only with the udev policy installed.

## Output Contract
Report the node opened, the demand path that authorized the write, the ACK/timeout
handling, and confirm no USB interface is claimed and no root is assumed.

## References
- [references/hidraw-safety.md](references/hidraw-safety.md) — real hardware-safety excerpts.
