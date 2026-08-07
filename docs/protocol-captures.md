# X6 Protocol — Captured Evidence

Decoded reports captured from the official Attack Shark X6 Windows app (USBPcap
inside a VirtualBox VM, USB passthrough of dongle `1D57:FA60`).

Raw captures live in `captures/`. Every claim here is backed by a `.pcapng`.

## Transport (confirmed)

| Field | Value |
|---|---|
| Endpoint | Interface 2 (vendor config), feature report type `0x03` |
| Control setup | `bmRequestType=0x21` (host→dev, class, interface), `bRequest=0x09` (SET_REPORT), `wValue=0x03<id>` |
| ACK | interrupt `0x83`, report `0x03`: `03 10 50 00 <report_id>` |

The ACK echoes the confirmed report id in byte `[4]`.

## USBPcap packet layout

```
[28 bytes USBPcap/usbmon header]
[8 bytes control setup]     bmRequestType bRequest wValue wIndex wLength
[report payload]            starts with the report id byte
```

## Report 0x04 — full configuration (52 B, authoritative)

Sent at startup and re-sent after **every** change (DPI, active stage, RGB,
polling). Always the complete payload with a fresh checksum.

Layout (offsets from byte 0 = report id):

| Offset | Size | Meaning |
|---|---|---|
| 0 | 1 | `0x04` report id |
| 1 | 1 | `0x38` protocol field (56); it is **not** the USB transfer length |
| 2 | 1 | `0x01` fixed |
| 3 | 1 | **angle control** (0=off, 1=on), duplicated in [6] |
| 4 | 1 | **ripple control** (0=off, 1=on) |
| 5 | 1 | stage enable mask (bits 1-8) |
| 6 | 1 | **angle control** (duplicate of [3], same `config[0x944]`) |
| 7 | 1 | lift of distance (0=1MM, 1=2MM) |
| 8..15 | 8 | DPI lows of stages 1-8 (`(DPI/50 - 1) & 0xFF`) |
| 16..23 | 8 | DPI highs of stages 1-8 (`(DPI/50 - 1) >> 8`) |
| 24 | 1 | **active stage** (1-8) |
| 25..48 | 24 | per-stage BGR colors, 3 B each (8 stages) — stays fixed when switching stage |
| 49 | 1 | `0x01` fixed |
| 50-51 | 2 | checksum `sum([3..49])`, big-endian |

### Authoritative feature-report contract

The complete `0x04` feature report is **exactly 52 bytes**: offsets `[0:52]`.
The USBPcap control setup records `wLength=52` and `payload_len=52` for
`dpi.pcapng` frame 1870 and all inspected DPI-bar writes (frames 2148–10100 in
`barra_dpi.pcapng`). `0x38` at byte `[1]` is a device protocol field whose value
is 56; it does **not** authorize a 56-byte HID write. There is no trailing
padding at offsets `[52:56]`. The checksum is the big-endian 16-bit sum of
bytes `[3:50]`, stored at `[50:52]`. Future encoders and live writes MUST send
only this 52-byte contract and wait for `03 10 50 00 04` on interrupt `0x83`.

The machine-readable fixture `captures/0x04-config/report04-contract.json`
records this contract and the two capture sources.

### Evidence: startup payload

```
04 38 01 00 00 3f 00 01 0f 17 1f 3f 6f 07 00 00
00 00 00 00 00 00 00 02 00 00 04 ff 00 00 00 ff
00 00 00 ff ff ff 00 00 ff ff ff 00 ff ff 40 00
ff ff ff 01 0e 74
```

Checksum verified: `sum(bytes[3:50]) == 0x0e74`. Active stage `0x04` → stage 4.

### Evidence: DPI/stage change (same session)

```
04 38 01 00 00 3f 00 01 0f 17 1f 3f 6f 07 00 00
00 00 00 00 00 00 00 02 00 00 03 ff 00 00 00 ff
00 00 00 ff ff ff 00 00 ff ff ff 00 ff ff 40 00
ff ff ff 01 0e 73
```

Only bytes `[24]` (stage `0x04`→`0x03`, i.e. stage 4→3) and `[51]` (checksum
`0x74`→`0x73`) changed. The checksum delta matches the byte change: a stage
decrement of 1 subtracts exactly 1 from the additive sum.

### Evidence: DPI bar value change (barra_dpi.pcapng)

Moving the DPI bar per stage (stage 1→6) re-sends `0x04` changing only that
stage's 16-bit DPI field (and the checksum). Each level is independent:

```
lows   75 00 b7 3f 6f 07 00 00      # stage1 5900 set first
highs  00 00 00 00 00 02 00 00
lows   75 8b b7 3f 6f 07 00 00      # stage2 -> 0x8b = 7000
lows   75 8b 45 3f 6f 07 00 00      # stage3 -> 0x45 = 3500
lows   75 8b 45 ae 6f 07 00 00      # stage4 -> 0xae = 8750
lows   75 8b 45 ae 1f 07 00 00      # stage5 -> 0x011f = 14400 (highs 01)
lows   75 8b 45 ae 1f bf 00 00      # stage6 -> 0xbf = 9600 (highs 00)
```

Decoding `DPI = (raw + 1) × 50` with `raw = high<<8 | low`:
`0x75`→5900, `0x8b`→7000, `0xb7`→9200, `0x45`→3500, `0xae`→8750,
`0x011f`→14400, `0xbf`→9600, `0x207`→26000. **Encoding confirmed in both
directions** (write + what the dongle stores).

### Evidence: ripple control (ripple_control.pcapng)

Toggling "ripple control" (corrugation) to ON changes only byte `[4]` from
`0x00` to `0x01` (and the checksum `0x0e72`→`0x0e73`). Factory reset restores
`0x00`:

```
04 38 01 00 01 3f 00 01 ... 0e 73     # ripple ON,  byte[4]=0x01
04 38 01 00 00 3f 00 01 ... 0e 72     # factory OFF, byte[4]=0x00
```

### Evidence: angle control (angle.pcapng)

Toggling "angle control" ON changes bytes `[3]` **and** `[6]` together from
`0x00` to `0x01` (checksum `0x0e72`→`0x0e74`, delta +2). Both bytes mirror
the same `config[0x944]` flag (Ghidra), so byte `[6]` is a duplicate of
byte `[3]`:

```
04 38 01 01 00 3f 01 01 ... 0e 74     # angle ON,  byte[3]=byte[6]=0x01
04 38 01 00 00 3f 00 01 ... 0e 72     # factory OFF, byte[3]=byte[6]=0x00
```

### Evidence: cycling every stage (stage.pcapng)

Switching the active stage in the app re-sends the full `0x04` with only `[24]`
(active stage) and `[51]` (checksum) changing. Byte `[24]` is the stage number
**directly** (1-based) — not offset:

| frame | byte [24] | checksum |
|---|---|---|
| 3912 | `0x01` | `0x0e71` |
| 4064 | `0x02` | `0x0e72` |
| 4342 | `0x03` | `0x0e73` |
| 4510 | `0x04` | `0x0e74` |
| 4992 | `0x05` | `0x0e75` |
| 5220 | `0x06` | `0x0e76` |
| 5640 | `0x03` | `0x0e73` |

Each stage increment adds exactly 1 to the checksum, confirming
`checksum = sum(bytes[3:50])` endian-independent (additive).

> Note: at frames 5640-5644 the app sent two identical `0x04` payloads back to
> back before the single ACK (5648). The device ACKs once. Re-sends appear on
> UI interactions; a single SET_REPORT is sufficient.

### ACK

```
03 10 50 00 04
```

## Report 0x0c — startup handshake (6 B)

```
0c 0a 01 fe 01 fe
```

Sent first at startup. Meaning unknown yet; expected to be an init/handshake
request.

## Report 0x05 — lighting mode (13 B)

Sent at startup and re-sent when the lighting **mode**, color or speed
changes.

| Offset | Size | Meaning |
|---|---|---|
| 0 | 1 | `0x05` report id |
| 1 | 1 | `0x0f` length (15) |
| 2 | 1 | `0x01` fixed |
| 3 | 1 | **light mode** (see table) |
| 4 | 1 | deep-sleep high: `0x03 + 0x10 × wraps` |
| 5 | 1 | **deep-sleep low**: `(min + 0.5) × 16` mod 256 |
| 6..8 | 3 | **color BGR** (e.g. `00 ff 00` green) |
| 9 | 1 | **sleep timer** = minutes × 2 (bar 0.5-60 min, 0.5 steps) |
| 10 | 1 | **response time** = ms / 2 (factory 8 ms → `0x04`) |
| 11 | 1 | deep-sleep wraps: `0x01 + wraps` (also shifts on color pick) |
| 12 | 1 | checksum `sum(bytes[3:11]) & 0xff` |

### Lighting modes

| Value | Mode |
|---|---|
| `0x00` | Off |
| `0x10` | Steady (constant light) |
| `0x20` | Breathing |
| `0x50` | Startup mode (unknown yet) |

### Observed payloads

```
05 0f 01 00 03 a8 00 ff 00 01 04 01 af     # off, mode=0x00
05 0f 01 10 03 a8 00 ff 00 01 04 01 bf     # steady, mode=0x10
05 0f 01 20 03 a8 00 ff 00 01 04 01 cf     # breathing base, speed=03 color=00ff00
05 0f 01 20 03 a8 f9 5e fe 01 04 03 25     # breathing, color changed -> f95efe
05 0f 01 20 03 a8 00 7f ff 01 04 02 4e     # breathing, color changed -> 007fff
05 0f 01 20 03 a8 00 ff ff 01 04 02 ce     # breathing, color changed -> 00ffff
05 0f 01 20 04 a8 00 ff ff 01 04 02 cf     # breathing, speed 03 -> 04
05 0f 01 20 03 a8 00 ff ff 01 04 02 ce     # breathing, speed back to 03
05 0f 01 50 03 a8 00 ff 00 01 04 01 ff     # startup, mode=0x50
05 0f 01 20 23 d8 00 ff ff 0c 04 03 29     # sleep timer 6 min (0x0c/2)
05 0f 01 20 23 d8 00 ff ff 2c 04 03 49     # sleep timer 22 min (0x2c/2)
05 0f 01 20 23 d8 00 ff ff 17 04 03 34     # sleep timer 11.5 min (0x17/2)
05 0f 01 20 23 d8 00 ff ff 3c 04 03 59     # sleep timer 30 min (0x3c/2)
05 0f 01 20 23 d8 00 ff ff 1d 04 03 3a     # sleep timer 14.5 min (0x1d/2)
05 0f 01 20 23 d8 00 ff ff 26 04 03 43     # sleep timer 19 min (0x26/2) - exact
05 0f 01 20 23 d8 00 ff ff 2b 04 03 48     # sleep timer 21.5 min (0x2b/2) - exact
05 0f 01 20 23 d8 00 ff ff 0d 04 03 2a     # sleep timer 6.5 min (0x0d/2) - exact
```

### Sleep timer

Byte [9] is the **sleep (normal) timer** in **minutes × 2**: the app bar runs
0.5-60 min in 0.5 min steps, so byte = minutes × 2 (e.g. 19 min → `0x26`).
Confirmed with exact values 19, 21.5, 6.5 min.

### Response time

Byte [10] is the **response time in ms / 2**: the app bar goes from 8 ms
factory in 2 ms steps, so byte = ms / 2 (8 ms → `0x04`). Confirmed by factory
reset (`response_time_factory.pcapng`).

```
05 0f 01 50 03 a8 00 ff 00 01 04 01 ff     # factory reset, 8 ms (0x04)
05 0f 01 50 03 a8 00 ff 00 01 0e 02 09     # pre-reset state, 28 ms (0x0e)
05 0f 01 50 03 a8 00 ff 00 01 06 02 01     # 12 ms (0x06)
05 0f 01 50 03 a8 00 ff 00 01 10 02 0b     # 32 ms (0x10)
```

### Deep-sleep timer

A 3-byte field (bytes 4, 5, 11) encodes the deep-sleep timer:

```
raw    = (min + 0.5) * 16
wraps  = raw // 256
byte5  = raw % 256
byte4  = 0x03 + 0x10 * wraps
byte11 = 0x01 + wraps
```

Verified on 5 values (4, 10, 22, 30, 45 min) — all bytes match:

| min | byte4 | byte5 | byte11 |
|---|---|---|---|
| 4 | `0x03` | `0x48` | `0x01` |
| 10 (default) | `0x03` | `0xa8` | `0x01` |
| 22 | `0x13` | `0x68` | `0x02` |
| 30 | `0x13` | `0xe8` | `0x02` |
| 45 | `0x23` | `0xd8` | `0x03` |

Byte 11 also shifts on color pick, so it is shared between the deep-sleep
counter and the color selector.

Checksum verified on every payload: always `sum(bytes[3:11]) & 0xff`, no
offset. Field deltas (mode, speed, color) propagate 1:1 to the checksum
(additive).

## Report 0x06 — polling rate (9 B)

Sent at startup and re-sent when the polling rate changes. The rate is encoded
in byte [3]; byte [4] is its 1's complement (`byte[4] = 0xff - byte[3]`).

| Value | Rate |
|---|---|
| `0x01` | 1000 Hz (startup default) |
| `0x02` | 500 Hz |
| `0x04` | 250 Hz |
| `0x08` | 125 Hz |

The value is `byte[3] = 1000 / rate`; byte [4] is its 1's complement.

```
06 09 01 01 fe 00 00 00 00     # 1000 Hz, byte[3]=0x01 (0x01+0xfe=0xff)
06 09 01 02 fd 00 00 00 00     # 500 Hz,  byte[3]=0x02 (0x02+0xfd=0xff)
06 09 01 04 fb 00 00 00 00     # 250 Hz,  byte[3]=0x04 (0x04+0xfb=0xff)
06 09 01 08 f7 00 00 00 00     # 125 Hz,  byte[3]=0x08 (0x08+0xf7=0xff)
```

Layout (offsets from byte 0 = report id):

| Offset | Size | Meaning |
|---|---|---|
| 0 | 1 | `0x06` report id |
| 1 | 1 | `0x09` length (9) |
| 2 | 1 | `0x01` fixed |
| 3 | 1 | **polling rate value** |
| 4 | 1 | `0xff - byte[3]` (1's complement validation) |
| 5..8 | 4 | `0x00` fixed |

Note: report `0x06` does not carry a trailing checksum like `0x04`/`0x05` —
the pair (rate, complement) acts as the integrity check.

## Report 0x08 — button remap (59 B)

```
08 3b 01 02 00 00 03 00 00 04 00 00 0d 00 00 0e 00 00 0f 00 00
06 00 00 05 00 00 3c 00 00 01 00 00 01 00 00 01 00 00 01 00 00
01 00 00 01 00 00 01 00 00 0a 00 00 09 00 00 00 94
```

Layout: 3-byte header `08 3b 01`, then 18 groups of 3 bytes (one per button
slot), then 2-byte additive checksum `sum(bytes[3:57])` big-endian.

Each group's first byte is the **action id** of that button slot; the other
two bytes are per-action **parameters** (`00 00` for simple actions, e.g. Easy
Aim uses `10 00 03` with level in byte 2). Remapping a button to "double click"
sets its action to `0x07`. Verified with clean factory-reset baseline captures:
the only changed byte is the remapped slot's action (and the checksum delta =
action delta).

Confirmed physical mapping (via clean per-button captures):

| Group | Factory action | Physical button |
|---|---|---|
| 1 | `0x02` | left |
| 2 | `0x03` | right |
| 3 | `0x04` | wheel click |
| 4 | `0x0d` | DPI Cycle |
| 5 | `0x0e` | DPI+ |
| 6 | `0x0f` | DPI− |
| 7 | `0x06` | forward (adelante) |
| 8 | `0x05` | side back |
| 9 | `0x3c` | ? |
| 17 | `0x0a` | scroll down |
| 18 | `0x09` | scroll up |

> Note: the app's button row numbering does not match the wire group index
> 1:1 past button 3 (e.g. the button shown as "DPI+" row remaps wire group 5).
> The physical mapping above is authoritative.

### Shortcut key encoding (resolved)

The "Assign A Shortcut" dialog blocks free-text entry (no letters accepted), so
the direct capture only gave `0x11 0f 00` (action `0x11`, mods `0x0f`, key
`0x00`). However the **Shortcut submenu** resolved the encoding: every option is
sent as a `0x11` report with a preloaded key combo. The **key byte is the
standard HID keyboard usage** (A=`0x04`, C=`0x06`, X=`0x1b`, Tab=`0x2b`,
F4=`0x3d`, …).

Observed modifier bitmask (checked in this order: Ctrl, Shift, Alt, Win):

| Bit | Modifier |
|---|---|
| `0x01` | Ctrl |
| `0x02` | Shift |
| `0x04` | Alt |
| `0x08` | Win |

Evidence (`btn6_shortcut.pcapng`): assigning a shortcut to button 6 while
checking each modifier in order produced `11 01 00`, `11 02 00`, `11 04 00`,
`11 08 00` (checksums 0x98, 0x99, 0x9b, 0x9f — params count in the additive
sum). `0x0f` = all four modifiers together.

Shortcut submenu (`btn6_shortcut_menu.pcapng`) — every option is `0x11`:

| Option | Report | Combo |
|---|---|---|
| Cut | `11 01 1b` | Ctrl+X |
| Copy | `11 01 06` | Ctrl+C |
| Paste | `11 01 19` | Ctrl+V |
| Open | `11 01 12` | Ctrl+O |
| Save | `11 01 16` | Ctrl+S |
| Find | `11 01 09` | Ctrl+F |
| Redo | `11 01 1c` | Ctrl+Y |
| Select All | `11 01 04` | Ctrl+A |
| Print | `11 01 13` | Ctrl+P |
| Close Window | `11 04 3d` | Alt+F4 |
| Swap Windows | `11 04 2b` | Alt+Tab |
| Show Desktop | `11 08 07` | Win+D |
| Run Command | `11 08 15` | Win+R |
| Lock PC | `11 08 0f` | Win+L |
| Screen Capture | `11 0a 16` | Win+Shift+S |

### Action ids are global (not per-button)

Action ids are **global**: assigning the same action to any button slot writes
the same byte. Confirmed by remapping the button shown as "button 6" to Left
Click: group 5 (`0x0e`) became `0x02`, the same byte the left button uses.
`left_click.pcapng`.

### Confirmed action ids

| id | Action | Evidence |
|---|---|---|
| `0x01` | **Button Off** | factory g10-16 + btn6_button_off (global) |
| `0x02` | Left Click | factory baseline + left_click (global) |
| `0x03` | Right Click | factory baseline + btn6_right_click (global) |
| `0x04` | Middle Button | factory baseline + btn6_middle_click (global) |
| `0x05` | Backward (atrás) | factory g8 + btn6_backward (global) |
| `0x06` | **Forward (adelante)** | factory g7 + btn6_forward (global) |
| `0x07` | Double Click | remap_* captures |
| `0x08` | **Fire Button** | btn6_fire_button |
| `0x09` | **Scroll Up** | factory g18 + btn6_scroll_up (global) |
| `0x0a` | **Scroll Down** | factory g17 + btn6_scroll_down (global) |
| `0x0d` | **DPI Cycle** | factory g4 + btn6_dpi_cycle_plus_minus (global) |
| `0x0e` | DPI+ | factory baseline + btn6_dpi_cycle_plus_minus |
| `0x0f` | DPI− | factory baseline + btn6_dpi_cycle_plus_minus |
| `0x10` | **Easy Aim** (param byte 2: level, e.g. `03`) | btn6_easy_aim |
| `0x11` | **Assign A Shortcut** (params: mods + key, e.g. `0f 00`) | btn6_shortcut |
| `0x15` | **Media Player** | btn6_multimedia |
| `0x16` | **Previous Track** | btn6_multimedia |
| `0x17` | **Next Track** | btn6_multimedia |
| `0x18` | **Play / Pause** | btn6_multimedia |
| `0x19` | **Stop** | btn6_multimedia |
| `0x1a` | **Mute** | btn6_multimedia |
| `0x1b` | **Volume +** | btn6_multimedia |
| `0x1c` | **Volume −** | btn6_multimedia |
| `0x1d` | **Browser: Calculator** | btn6_browser |
| `0x1e` | **Browser: Email** | btn6_browser |
| `0x1f` | **Browser: Favorites** (probable, capture glitched to shortcut) | btn6_browser |
| `0x20` | **Browser: Forward** | btn6_browser |
| `0x21` | **Browser: Backward** | btn6_browser |
| `0x22` | **Browser: Stop** | btn6_browser |
| `0x23` | **Browser: My Computer** | btn6_browser |
| `0x24` | **Browser: Refresh** | btn6_browser |
| `0x25` | **Browser: Home** | btn6_browser |
| `0x26` | **Browser: Search** | btn6_browser |
| `0x3c` | ? (group 9 factory) | factory baseline |

### Evidence (each: factory baseline → after remap)

```
08 3b 01 02 00 00 07 00 00 04 00 00 ...  # left -> 07 (remap_left)
08 3b 01 02 00 00 07 00 00 ...            # right -> 07 (remap_right)
08 3b 01 ... 07 00 00 0f 00 00 ...        # DPI+ -> 07 (group 5, remap_dpi)
08 3b 01 ... 0f 00 00 07 00 00 ...        # DPI- -> 07 (group 6, remap_dpi_minus)
08 3b 01 ... 05 00 00 07 00 00 ...        # side back -> 07 (group 8, remap_back)
08 3b 01 ... 02 00 00 07 00 00 ...        # button 6 (g5) -> 02 (left_click)
```

## Interrupt status reports (interface 2)

| Event | Bytes | Meaning |
|---|---|---|
| Heartbeat | `03 10 40 01 0a` | idle, battery 100 % (byte 4 × 10 = %) |
| ACK | `03 10 50 00 <id>` | config report confirmed |

Heartbeats arrive roughly once per second while the app is open.

## Startup sequence (observed order)

1. `0x0c` handshake → ACK `00 0c`
2. `0x04` full config → ACK `00 04`
3. `0x05` → ACK `00 05`
4. `0x06` → ACK `00 06`
5. `0x08` button remap → ACK `00 08`
6. (interactive) `0x04` again on every config change

## Open questions

- Which byte(s) in `0x06` encode the polling rate?
- Full button table semantics of `0x08`.
- Meaning of `0x0c` handshake.
- Whether `0x09` (macro) appears on macro operations.
- Lighting **mode** in `0x05` byte [3] is decoded; color is bytes 6-8 BGR,
  byte [11] is shared between deep-sleep wraps and color pick. Sleep timer =
  byte [9] (min × 2), deep-sleep = bytes 4+5+11 (3-byte encoding above),
  response time = byte [10] (ms / 2). Remaining modes (neon, heartbeat,
  fixed-DPI, breathing-DPI) still need captures. The `0x50` startup mode is
  unassigned.
