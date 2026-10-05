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

Timing/delay units, playback count, persistence, larger event layouts, field
width beyond this template, slot rules and report08 assignments remain unknown.
The codec neither maps local `DelayMS` values to hardware nor assumes zero means
an observed hardware delay. Future assignment integration must separately decide
whether local events fit this supported boundary; it must not silently discard
unsupported events or timing. This API only selects a captured template, leaving
local-library validation and device application separate.
