# Attack Shark X6 HID Protocol — Identification (read-only)

> **Status**: protocol decoded from `X6.exe` (Ghidra) and **validated live** on
> a real dongle (SET_REPORT accepted + ACK). No CLI yet.
> **Date**: 2026-08-07. **Model**: Attack Shark X6 (magnetic charging base,
> PAW3395 sensor, 26000 DPI).

---

## 1. Executive summary

The Attack Shark **X6** speaks the **same protocol as the X3/R1/X11**: same
dongle `0x1D57:0xFA60`, same configuration interface, same `SET_REPORT` feature
transport. The definitive proof: **`hiddriver_1.dll` is byte-for-byte identical
between the X3 and X6** (sha256
`f9efbf0630218b3e13e894b132d77ceb16113acef1bc841e25af8ce57eac39f4`). What changes
is the app logic (`X6.exe`), in particular the DPI report layout.

Findings validated live (X6, 2026-08-07):

- ✅ **Battery**: read "pushed" by the dongle on interrupt `0x83`, report `0x03`,
  `byte[4] × 10 = %`. Measured 100 % (`0x0A`).
- ✅ **Configuration (DPI + RGB + flags + checksum)**: 56-byte `0x04` report, sent
  via `SET_REPORT` feature `wValue=0x0304`, interface 2. **Accepted (rc=56)** and
  confirmed with **ACK** `03 10 50 00 04` over interrupt.
- ❌ Config is **not readable** read-only (`GET_REPORT` → STALL), same as the X3.
  The app must keep the canonical state.

---

## 2. Device identification

| Field | Value |
|---|---|
| Vendor ID | `0x1D57` |
| Product ID | `0xFA60` (2.4 GHz dongle) |
| USB name | `Beken 2.4G Wireless Device` |
| Chipset / MCU | Beken BK3633 |
| Sensor | PixArt PAW3395 |
| Config interface | Interface **2** (usage page 1, usage `0x80`) |

Dongle HID interfaces:

| Interface | Usage | IN endpoint |
|---|---|---|
| 0 | Keyboard (boot) | `0x81` |
| 1 | Mouse | `0x82` |
| **2** | **Configuration (vendor)** | **`0x83`** |
| 3 | Consumer / keyboard | `0x84` |

---

## 3. Configuration transport

```
SET_REPORT (feature):
  bmRequestType = 0x21   (host→device, class, interface)
  bRequest      = 0x09   (SET_REPORT)
  wValue        = 0x0304 (report type 0x03 = feature, report ID 0x04)
  wIndex        = 2      (configuration interface)
```

- **ACK**: the dongle replies on interrupt `0x83`, report `0x03`, `byte[2] == 0x50`.
  (On the X6, the official app waits for the ACK after every SET_REPORT.)
- **Checksum**: 16-bit additive sum of `[3..49]` (the 47 data bytes), high byte
  in `[50]`, low byte in `[51]` (big-endian).

---

## 4. Configuration report `0x04` (56 B) — DPI/RGB/flags

Layout of builder `FUN_004143a0` in `X6.exe` (validated live):

| Offset | Size | Meaning | Config source |
|---|---|---|---|
| 0 | 1 | `0x04` report ID | fixed |
| 1 | 1 | `0x38` length (56) | fixed |
| 2 | 1 | `0x01` | fixed |
| 3 | 1 | ? | `config[0x944]` |
| 4 | 1 | ? | `config[0x940]` |
| 5 | 1 | **enabled-stages mask 1-8** (bit per stage) | `config[0x904..0x920]` |
| 6 | 1 | ? | `config[0x944]` (repeated) |
| 7 | 1 | **lift of distance** (0=1MM, 1=2MM) | `config[0x948]` |
| 8..15 | 8 | **DPI lows** of stages 1-8 (`(DPI/50 - 1) & 0xFF`) | `config[0x8c4+i*4]` |
| 16..23 | 8 | **DPI highs** (`(DPI/50 - 1) >> 8`) | `config[0x8c4+i*4]` |
| 24 | 1 | **active stage + 1** (1-8) | `config[0x934]` |
| 25..48 | 24 | **8 RGB colors**, 24-bit (BGR), 3 B each | `config[0x960+i*4]` |
| 49 | 1 | `0x01` | fixed |
| 50-51 | 2 | checksum `sum([3..49])` big-endian | computed |

**DPI key**: unlike the X3 (which uses a map byte with
`DPI = (index-1) × 50`), the **X6 stores DPI/50 directly** in the internal config
and sends `DPI/50 - 1` in 16-bit LE on the wire. That is why the factory DPI
values are `0x10/0x18/0x20/0x40/0x70/0x208` (= 800/1200/1600/3200/5600/26000),
which become `0x0F/0x17/0x1F/0x3F/0x6F/0x207` on the wire.

### 4.1 X6 factory config (init `FUN_00415280`)

| Field | Value |
|---|---|
| DPI stages 1-6 | 800, 1200, 1600, 3200, 5600, 26000 |
| Enabled stages | 1,1,1,1,1,1,0,0 |
| Colors | 0xff, 0xff00, 0xff0000, 0xffff, 0xffff00, 0xff00ff, 0x40ff, 0xffffff |
| Polling | 1000 Hz |
| Lift of distance | 2 MM |
| Active stage | 2 (the second one, `0x934=1`) |

### 4.2 Factory payload reconstructed and validated

```
04 38 01 00 00 3f 00 01
0f 17 1f 3f 6f 07 00 00    ← lows: (800/50-1)=0x0f, ..., (26000/50-1)=0x207
00 00 00 00 00 00 00 02    ← highs
00 00 02                   ← active stage + 1 = 2, plus 2 color bytes
ff 00 00 00 ff 00 00 00 ff ff ff 00 00 ff ff ff 00 ff ff 40 00 ff ff ff
01
0e 72                      ← checksum
```

As full bytes:

```
04 38 01 00 00 3f 00 01 0f 17 1f 3f 6f 07 00 00
00 00 00 00 00 00 00 02 00 00 02 ff 00 00 00 ff
00 00 00 ff ff ff 00 00 ff ff ff 00 ff ff 40 00
ff ff ff 01 0e 72
```

> `SET_REPORT` of this payload: **rc=56 OK**, ACK `03 10 50 00 04`.

---

## 5. Status report (the only direct read)

Interrupt IN `0x83`, input report `0x03` (5 B):

| Byte | Meaning |
|---|---|
| 0 | `0x03` report ID (status) |
| 1 | `0x10` event/sub-status |
| 2 | `0x40` heartbeat · `0x10` DPI button · `0x50` ACK |
| 3 | active stage on the `0x10` event; carries no raw DPI |
| 4 | `byte × 10 = % battery` |

Live evidence (X6):

```
idle:  03 10 40 01 0a   (heartbeat, battery 100 %)
ack:   03 10 50 00 04   (after config SET_REPORT)
```

The `03 10 10 <stage> 00` event is only authoritative for selecting the physical
stage. The Linux UI resolves its DPI exclusively from the stage→DPI mapping
confirmed for that same device; if that mapping is missing, it shows DPI as
unknown. No DPI sent by the mouse is inferred or claimed.

---

## 6. Methodology and files

| File | Role |
|---|---|
| `x6probe/probe.py` | read-only dongle reads (battery/status) |
| `x6probe/factory_reset.py` | sends the factory payload (validated) |
| `x6ghidra/` | Ghidra project: report_builder.txt (FUN_004143a0), init_config.txt (FUN_00415280), reset_handler.txt (FUN_00410460), hid_wrapper.txt (FUN_00413940) |
| `official-app/` | X6 Windows app being adapted (source of UI texts and layout) |

Still to decode (next phases): report `0x05` (sleep/key response), `0x06`
(polling), `0x08` (button remap), `0x09` (macro). The family X3 doc
(protocol-x3.md) describes them as candidates of the same structure.

---

## 7. Conclusion for the driver

- The X6 reuses the **X3/R1 transport layer** (SET_REPORT feature 0x0304, iface
  2, ACK over 0x83). The transport layer can be shared.
- The **DPI report layer differs** (16-bit `DPI/50 - 1` vs the X3 map byte);
  each model needs its own builder.
- Battery/status: passive interrupt reads; the app keeps the canonical config
  state after each write.