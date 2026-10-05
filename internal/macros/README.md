# Local library and offline X6 codec

The library validates local names and event data independently of hardware.
A locally valid macro is not necessarily representable by the X6 codec.

## Supported codec API

- `X6Click{Button: MouseLeft | MouseRight, Repeat: int}` selects the exact
  observed single press/release template. It intentionally has no delay,
  arbitrary event list, slot, name or ID fields.
- `EncodeX6Block` / `DecodeX6Block` use a 128-byte logical block.
- `EncodeX6Reports` / `DecodeX6Reports` use three ordered 64-byte reports
  with headers `09 40 05 00`, `09 40 05 01`, `09 0c 05 02`, logical chunks
  60/60/8 bytes and 52 final zero-padding bytes.
- `X6Upload{Destination: byte, Click: X6Click}` with `EncodeX6Upload` /
  `DecodeX6Upload` carries a checked destination through framing and roundtrip.
  Only existing remap groups **1, 2, 3, 5, 6, 7, 8** are admitted; mixed IDs
  are rejected. Legacy report APIs remain strict destination05 wrappers.
- `x6.MacroDestinationForButton` is the canonical checked logical-button lookup:
  buttons1–7 map to groups1/2/3/7/8/5/6, not their logical button numbers.
  `x6.MacroAssignment{Config: RemapConfig, Button: uint8}` and
  `EncodeMacroAssignmentReport` validate both inputs, preserve unrelated pending
  remaps and overlay `12 00 <group>` with a recomputed report08 checksum.
  Macro assignment is separate from the generic remap action catalog.
- All functions return errors for unsupported input, allocate independent
  output buffers and perform no storage or transport operations.

The additive checksum covers bytes 0–125 and is stored big-endian at 126–127.
Decoding verifies checksum **and** the complete template, including fixed zero
bytes, matching button codes and framing/padding. A valid checksum alone does
not authorize another layout. No overflow rule for future layouts is inferred.

## Evidence versus extrapolation

Source: [`captures/0x09-macro`](../../captures/0x09-macro/README.md), with five
preserved binary fixtures and exact physical reports in `contract.json`.
Tests read those fixtures and compare every selected captured upload byte.

The decided UI range is **1–255**. Captures prove left repeat bytes **1, 2, 3,
5** and right repeat byte **1**. Every other accepted button/repeat combination
is interpolation/extrapolation of those bytes, not verified device acceptance
or playback. Tests exercise all supported byte values, including checksum carry
at 255, without claiming physical-device evidence.

Destination05 is capture-backed for DPI+ (logical button6/group5). The new
DPI-minus capture proves destination06 (logical button7/group6): report08
frame6637 selects `12 00 06`, then report09 frames6713/6717/6719 use headers
`09 40 06 00`, `09 40 06 01`, `09 0c 06 02`. Its body exactly matches
`left-repeat-1.bin` (checksum0267, SHA256
`07bc932f27c5d4431971f1e9406b33f0ea458c30fcd8a91526af8d7e2f2e4d8d`).
Original `macro_dpi-.pcapng` SHA256:
`d6df914d931ae7f591a9fec909287e684dcb5007f6491f363ca33671e8aa2d62`.
Other mapped destinations are explicitly user-authorized extrapolation, not
capture-backed or physically tested. There is no fixed05 destination rule,
slot allocation or claim of independent simultaneous macro assignments.

Timing/delay units, playback count, persistence, larger event layouts, field
width beyond this template and slot rules remain unknown.
The codec neither maps local `DelayMS` values to hardware nor assumes zero means
an observed hardware delay. Future assignment integration must separately decide
whether local events fit this supported boundary; it must not silently discard
unsupported events or timing. This API only selects a captured template, leaving
local-library validation and device application separate.
