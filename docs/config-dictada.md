# X6 — Config dictada por el usuario (baseline real, desde la app oficial)

> ARCHIVO DE TRABAJO en vivo. El usuario dicta la config que tiene cargada en la
> app oficial (la misma que se reenvía con SET_REPORT al abrir). Se valida contra
> la codificación ya decodificada en `docs/protocol-captures.md`. Esto se usará
> como **plantilla factory/por defecto** para el futuro botón reset y para el
> archivo de perfil del PC.

## Dictado en sesión (incompleto — en progreso)

| Sección | Valor dictado | Encoding esperado (referencia) |
|---|---|---|
| DPI 1 | 800 | low 0x0f |
| DPI 2 | 1200 | low 0x17 |
| DPI 3 | 1600 | low 0x1f |
| DPI 4 | 3200 | low 0x3f |
| DPI 5 | 5600 | low 0x6f |
| DPI 6 | 26000 | low 0x07, high 0x02 |
| DPI 7 | (no definido) | low 0x00 |
| DPI 8 | (no definido) | low 0x00 |
| Modo de luz | fijo | `0x10` (steady) |
| Tasa de sondeo | 100 Hz | `0x06`: byte[3] = `0x0a` (1000/100) |
| Sueño (normal) | 0.5 min | `0x05` byte[9] = `0x01` (min × 2) |
| Sueño profundo | 10 min | `0x05` bytes 4/5/11 = `03 a8 01` |
| Distancia de elevación | 1 mm | `0x04` byte[7] = `0x00` |
| Tiempo de respuesta | 8 ms | `0x05` byte[10] = `0x04` (ms / 2) |
| Control de corrugación | OFF (guan) | `0x04` byte[4] = `0x00` |
| Captura de ángulo | OFF (guan) | `0x04` byte[3] = byte[6] = `0x00` |
| Motion Sync | ON | **byte sin mapear — captura pendiente** |

## Botones (numeración de la app vs wire groups)

La numeración de la app **no coincide 1:1** con el índice de grupo de wire
más allá del botón 3 (evidencia: el botón "DPI+" de la app remapea el wire g5).

| Botón (app) | Acción física | Wire group | Factory (0x08) |
|---|---|---|---|
| 1 | Click izquierdo | g1 | `0x02` |
| 2 | Click derecho | g2 | `0x03` |
| 3 | Medio (rueda) | g3 | `0x04` |
| 4 | Adelante (primer botón lateral) | g7 | `0x06` |
| 5 | Atrás (último botón lateral) | g8 | `0x05` |
| 6 | DPI+ (arriba centro, debajo rueda) | g5 | `0x0e` |
| 7 | DPI− (debajo de +) | g6 | `0x0f` |

> **No hay botón DPI Cycle.** Los botones son solo 7 (g1-g3 y g5-g8; el g4 es
> una celda de config con `0x0d` sin botón físico). Mapeo app→wire **no lineal**:
> app4→g7, app5→g8, app6→g5, app7→g6 (confirmado por capturas: el botón "6" de
> la app remapea el wire g5). DPI+/DPI− ciclan entre los 6 niveles (800-26000)
> definidos.
>
> El apartado **DPI** del menú de asignación de botones contiene **DPI Cycle
> (`0x0d`), DPI+ (`0x0e`), DPI− (`0x0f`)** — capturado en
> `btn6_dpi_cycle_plus_minus.pcapng`. Qué niveles entran en el ciclo se define
> con el **stage enable mask** del `0x04` (byte[5], factory `0x3f` = niveles 1-6).

## Pendientes / próximos pasos

- ~~**Barra de DPI**: no capturado.~~ **RESUELTO** — captura `barra_dpi.pcapng`
  (order stage 1→6). Encoding confirmado en vivo: `DPI = (byte + 1) × 50`,
  16-bit big-endian (low/high en `0x04` bytes 8-23), cada nivel independiente.
- **Macros** (report `0x09`): pospuesto hasta hacer el creador de macros.
- **Motion Sync**: toggle sin byte mapeado — captura pendiente.
- Con lo actual: el **remap** (`0x08`) ya es implementable desde Linux.

## Validaciones

- DPI: coincide exacto con los lows del payload factory `0f 17 1f 3f 6f 07 00 00`.
- Sueño normal 0.5 min → byte[9] = `0x01` (formato min × 2, barra 0.5-60).
- Sueño profundo 10 min → `(10+0.5)×16 = 0xa8` → bytes `03 a8 01`.
- Tasa 100 Hz difiere del factory reset capturado (1000 Hz) → el ratón persiste
  lo último que le mandaron; la plantilla factory real de la app NO es el reset.
