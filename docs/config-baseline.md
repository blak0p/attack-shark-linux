# X6 — User-dictated config (real baseline, from the official app)

> LIVE WORKING FILE. The user dictates the config currently loaded in the
> official app (the same one re-sent via SET_REPORT on startup). It is validated
> against the encoding already decoded in `docs/protocol-captures.md`. It will
> be used as the **factory/default template** for the future reset button and
> for the PC profile file.

## Dictated in session (incomplete — in progress)

| Section | Dictated value | Expected encoding (reference) |
|---|---|---|
| DPI 1 | 800 | low 0x0f |
| DPI 2 | 1200 | low 0x17 |
| DPI 3 | 1600 | low 0x1f |
| DPI 4 | 3200 | low 0x3f |
| DPI 5 | 5600 | low 0x6f |
| DPI 6 | 26000 | low 0x07, high 0x02 |
| DPI 7 | (undefined) | low 0x00 |
| DPI 8 | (undefined) | low 0x00 |
| Light mode | fixed | `0x10` (steady) |
| Polling rate | 100 Hz | `0x06`: byte[3] = `0x0a` (1000/100) |
| Sleep (normal) | 0.5 min | `0x05` byte[9] = `0x01` (min × 2) |
| Deep sleep | 10 min | `0x05` bytes 4/5/11 = `03 a8 01` |
| Lift of distance | 1 mm | `0x04` byte[7] = `0x00` |
| Key response time | 8 ms | `0x05` byte[10] = `0x04` (ms / 2) |
| Ripple control | OFF | `0x04` byte[4] = `0x00` |
| Angle snap | OFF | `0x04` byte[3] = byte[6] = `0x00` |
| Motion Sync | ON | **unmapped byte — capture pending** |

## Buttons (app numbering vs wire groups)

The app numbering **does not match 1:1** the wire group index beyond button 3
(evidence: the app's "DPI+" button remaps wire g5).

| Button (app) | Physical action | Wire group | Factory (0x08) |
|---|---|---|---|
| 1 | Left click | g1 | `0x02` |
| 2 | Right click | g2 | `0x03` |
| 3 | Middle (wheel) | g3 | `0x04` |
| 4 | Forward (first side button) | g7 | `0x06` |
| 5 | Backward (last side button) | g8 | `0x05` |
| 6 | DPI+ (center top, under the wheel) | g5 | `0x0e` |
| 7 | DPI− (below +) | g6 | `0x0f` |

> **There is no DPI Cycle button.** There are only 7 buttons (g1-g3 and g5-g8;
> g4 is a config cell with `0x0d` and no physical button). App→wire mapping is
> **non-linear**: app4→g7, app5→g8, app6→g5, app7→g6 (confirmed by captures:
> the app's "6" button remaps wire g5). DPI+/DPI− cycle through the 6 defined
> levels (800-26000).
>
> The **DPI** section of the button assignment menu contains **DPI Cycle
> (`0x0d`), DPI+ (`0x0e`), DPI− (`0x0f`)** — captured in
> `btn6_dpi_cycle_plus_minus.pcapng`. Which levels enter the cycle is defined by
> the **stage enable mask** of `0x04` (byte[5], factory `0x3f` = levels 1-6).

## Pending / next steps

- ~~**DPI bar**: not captured.~~ **RESOLVED** — capture `barra_dpi.pcapng`
  (stage order 1→6). Encoding confirmed live: `DPI = (byte + 1) × 50`, 16-bit
  big-endian (low/high in `0x04` bytes 8-23), each level independent.
- **Macros** (report `0x09`): postponed until the macro creator is built.
- **Motion Sync**: toggle without a mapped byte — capture pending.
- With what we have: **remap** (`0x08`) is already implementable from Linux.

## Validations

- DPI: exact match with the factory payload lows `0f 17 1f 3f 6f 07 00 00`.
- Normal sleep 0.5 min → byte[9] = `0x01` (min × 2 format, bar 0.5-60).
- Deep sleep 10 min → `(10+0.5)×16 = 0xa8` → bytes `03 a8 01`.
- 100 Hz rate differs from the captured factory reset (1000 Hz) → the mouse
  persists whatever it was last sent; the app's real factory template is NOT
  the reset.