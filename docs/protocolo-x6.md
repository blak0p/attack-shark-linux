# Protocolo HID del Attack Shark X6 — Identificación (read-only)

> **Estado**: protocolo decodificado desde `X6.exe` (Ghidra) y **validado en vivo**
> sobre el dongle real (SET_REPORT aceptado + ACK). Sin CLI aún.
> **Fecha**: 2026-08-07. **Modelo**: Attack Shark X6 (base de carga magnética,
> sensor PAW3395, 26000 DPI).

---

## 1. Resumen ejecutivo

El Attack Shark **X6** habla el **mismo protocolo que el X3/R1/X11**: mismo dongle
`0x1D57:0xFA60`, misma interfaz de configuración, mismo transporte `SET_REPORT`
feature. La prueba definitiva: **`hiddriver_1.dll` es byte a byte idéntica entre
X3 y X6** (sha256 `f9efbf0630218b3e13e894b132d77ceb16113acef1bc841e25af8ce57eac39f4`).
Lo que cambia es la lógica de la app (`X6.exe`), en particular el layout del
report DPI.

Hallazgos validados en vivo (X6, 2026-08-07):

- ✅ **Batería**: se lee "empujada" por el dongle en interrupt `0x83`, report `0x03`,
  `byte[4] × 10 = %`. En la prueba dio 100 % (`0x0A`).
- ✅ **Configuración (DPI + RGB + flags + checksum)**: report `0x04` de 56 B, envío
  por `SET_REPORT` feature `wValue=0x0304`, interfaz 2. **Aceptado (rc=56)** y
  confirmado con **ACK** `03 10 50 00 04` por interrupt.
- ❌ Config **no legible** por read-only (`GET_REPORT` → STALL), igual que el X3.
  La app debe mantener estado canónico.

---

## 2. Identificación del dispositivo

| Campo | Valor |
|---|---|
| Vendor ID | `0x1D57` |
| Product ID | `0xFA60` (dongle 2.4 GHz) |
| Nombre USB | `Beken 2.4G Wireless Device` |
| Chipset / MCU | Beken BK3633 |
| Sensor | PixArt PAW3395 |
| Interfaz de config | Interface **2** (usage page 1, usage `0x80`) |

Interfaces HID del dongle:

| Interface | Uso | Endpoint IN |
|---|---|---|
| 0 | Teclado (boot) | `0x81` |
| 1 | Mouse | `0x82` |
| **2** | **Configuración (vendor)** | **`0x83`** |
| 3 | Consumidor / teclado | `0x84` |

---

## 3. Transporte de configuración

```
SET_REPORT (feature):
  bmRequestType = 0x21   (host→device, class, interface)
  bRequest      = 0x09   (SET_REPORT)
  wValue        = 0x0304 (report type 0x03 = feature, report ID 0x04)
  wIndex        = 2      (interfaz de configuración)
```

- **ACK**: el dongle responde en interrupt `0x83`, report `0x03`, `byte[2] == 0x50`.
  (En el X6, la app oficial espera el ACK tras cada SET_REPORT.)
- **Checksum**: suma aditiva de 16 bits de `[3..49]` (los 47 bytes de datos),
  byte alto en `[50]`, byte bajo en `[51]` (big-endian).

---

## 4. Report de configuración `0x04` (56 B) — DPI/RGB/flags

Layout del builder `FUN_004143a0` en `X6.exe` (validado en vivo):

| Offset | Tamaño | Significado | Fuente en config |
|---|---|---|---|
| 0 | 1 | `0x04` report ID | fijo |
| 1 | 1 | `0x38` largo (56) | fijo |
| 2 | 1 | `0x01` | fijo |
| 3 | 1 | ? | `config[0x944]` |
| 4 | 1 | ? | `config[0x940]` |
| 5 | 1 | **mask de stages 1-8 habilitados** (bit por stage) | `config[0x904..0x920]` |
| 6 | 1 | ? | `config[0x944]` (repetido) |
| 7 | 1 | **lift of distance** (0=1MM, 1=2MM) | `config[0x948]` |
| 8..15 | 8 | **DPI lows** de stages 1-8 (`(DPI/50 - 1) & 0xFF`) | `config[0x8c4+i*4]` |
| 16..23 | 8 | **DPI highs** (`(DPI/50 - 1) >> 8`) | `config[0x8c4+i*4]` |
| 24 | 1 | **stage activo + 1** (1-8) | `config[0x934]` |
| 25..48 | 24 | **8 colores RGB** de 24 bits (BGR), 3 B c/u | `config[0x960+i*4]` |
| 49 | 1 | `0x01` | fijo |
| 50-51 | 2 | checksum `sum([3..49])` big-endian | calculado |

**Clave del DPI**: a diferencia del X3 (que usa un byte de mapa con
`DPI = (índice-1) × 50`), el **X6 guarda DPI/50 directo** en la config interna y
en el wire manda `DPI/50 - 1` en 16-bit LE. Por eso los DPI de fábrica son
`0x10/0x18/0x20/0x40/0x70/0x208` (= 800/1200/1600/3200/5600/26000), que en el
wire se convierten a `0x0F/0x17/0x1F/0x3F/0x6F/0x207`.

### 4.1 Config de fábrica X6 (init `FUN_00415280`)

| Campo | Valor |
|---|---|
| DPI stages 1-6 | 800, 1200, 1600, 3200, 5600, 26000 |
| Stages habilitados | 1,1,1,1,1,1,0,0 |
| Colores | 0xff, 0xff00, 0xff0000, 0xffff, 0xffff00, 0xff00ff, 0x40ff, 0xffffff |
| Polling | 1000 Hz |
| Lift of distance | 2 MM |
| Stage activo | 2 (el segundo, `0x934=1`) |

### 4.2 Payload de fábrica reconstruido y validado

```
04 38 01 00 00 3f 00 01
0f 17 1f 3f 6f 07 00 00    ← lows: (800/50-1)=0x0f, ..., (26000/50-1)=0x207
00 00 00 00 00 00 00 02    ← highs
00 00 02                   ← stage activo + 1 = 2, más 2 bytes de los colores
ff 00 00 00 ff 00 00 00 ff ff ff 00 00 ff ff ff 00 ff ff 40 00 ff ff ff
01
0e 72                      ← checksum
```

En bytes completos:

```
04 38 01 00 00 3f 00 01 0f 17 1f 3f 6f 07 00 00
00 00 00 00 00 00 00 02 00 00 02 ff 00 00 00 ff
00 00 00 ff ff ff 00 00 ff ff ff 00 ff ff 40 00
ff ff ff 01 0e 72
```

> `SET_REPORT` de este payload: **rc=56 OK**, ACK `03 10 50 00 04`.

---

## 5. Report de status (única lectura directa)

Interrupt IN `0x83`, report input `0x03` (5 B):

| Byte | Significado |
|---|---|
| 0 | `0x03` report ID (status) |
| 1 | `0x10` evento/sub-status |
| 2 | `0x40` heartbeat · `0x10` botón DPI · `0x50` ACK |
| 3 | stage DPI / perfil |
| 4 | `byte × 10 = % batería` |

Evidencia en vivo (X6):

```
idle:  03 10 40 01 0a   (heartbeat, batería 100 %)
ack:   03 10 50 00 04   (tras SET_REPORT de config)
```

---

## 6. Metodología y archivos

| Archivo | Rol |
|---|---|
| `x6probe/probe.py` | lectura read-only del dongle (batería/status) |
| `x6probe/factory_reset.py` | envío del payload de fábrica (validado) |
| `x6ghidra/` | proyecto Ghidra: report_builder.txt (FUN_004143a0), init_config.txt (FUN_00415280), reset_handler.txt (FUN_00410460), hid_wrapper.txt (FUN_00413940) |
| `official-app/` | app Windows del X6 a adaptar (fuente de los textos y layout de la UI) |

Pendiente de decodificar (fases siguientes): report `0x05` (sleep/key response),
`0x06` (polling), `0x08` (remapeo de botones), `0x09` (macro). El X3 doc de la
familia (protocolo-x3.md) los describe como candidatos de la misma estructura.

---

## 7. Conclusión para el driver

- El X6 reutiliza el **transport layer del X3/R1** (SET_REPORT feature 0x0304,
  iface 2, ACK por 0x83). La capa de transporte se puede compartir.
- La **capa de report DPI difiere** (16-bit `DPI/50 - 1` vs byte de mapa del X3);
  cada modelo necesita su builder.
- Batería/status: lectura pasiva por interrupt; la app mantiene estado canónico
  de la config tras cada escritura.
