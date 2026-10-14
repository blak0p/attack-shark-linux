# Safe local macro admission and X6 upload backend

The backend admits supported balanced keyboard/timed takes before any device I/O, preserving legacy complete-click behavior. Transport acknowledgement is not measured physical playback. The frontend assignment and presentation layer follows in the next stacked PR; the editor below describes the existing local recorder.

## Local keyboard storage

`Event{Type: Keyboard, Action: Down | Up, DelayMS: int64, KeyCode: string}`
stores an opaque physical key identity, suitable for browser `KeyboardEvent.code`.
Keyboard events require a nonblank trimmed identity but preserve the accepted
string verbatim, including whitespace and unknown/unverified identities. There
is no key allowlist, firmware mapping, or device-support admission. Mouse events
reject every nonempty `KeyCode`, including whitespace-only strings.

Actions remain down/up and local delays remain nonnegative. Validation does not
mutate input; CRUD preserves order, independent copies, and atomic persistence.
JSON uses optional `key_code`; mouse-only serialization omits it and existing
mouse documents remain readable. Schema version stays **1**: this is additive
compatibility for the current reader, not forward compatibility with older
binaries that reject unknown fields or event types.

Frontend contracts, Wails conversion, and version-1 single-macro import/export
preserve the same opaque identity and per-event delay contract. Unknown fields
still reject. The timeline labels keyboard identity and down/up separately; only
one or two recognized complete zero-delay mouse clicks without nonempty key
fields display assignment eligibility. This display does not admit an upload.

## Editor-scoped recording

Choose a delay policy, then **Start recording** in the macro editor. Only the
focused capture area receives keyboard and five mouse-button down/up events.
**Stop recording**, Tab navigation, or focus loss appends the take to the draft;
**Save to library** persists it. The buffered event count and stopped append
message distinguish recording from saving. Nothing is played or sent to a device.

| Take policy | Local event delays |
|---|---|
| Minimum | 0 ms on every event; not a firmware minimum claim |
| Default | 10 ms on every event |
| Recorded | Rounded monotonic milliseconds between down events, on the previous down; every up remains 0 |

For down A at 0, up A at 30, down B at 120, up B at 180, the delays are
`[120, 0, 0, 0]`. Neither releases nor stopping sample an interval. The final
press stays 0 until another press arrives. Codec terminal 10 is a separate rule.
Policy selection is frozen during a take and never rewrites earlier events.
There is no post-recording policy editor or schema metadata migration.

Keyboard identity is the opaque browser `event.code`, not text or a layout guess.
Modifiers are distinct identities; repeated downs and unmatched ups are ignored.
Tab is reserved for normal navigation, not captured. No global event listeners
or editable-field capture exist. Recorder buttons are outside the capture area.
Stopping automatically appends releases for still-held identities in press order,
using 0 (minimum/recorded) or 10 (default), never release-time measurements.
These local balancing events are not evidence of physical input or playback.

Recording locks library mutations, import, selection, save and deletion. Loading,
reading, saving and delete confirmation prevent a take from starting. A synchronous
session token protects against same-tick races and stale service delivery. Unmount
finishes into the same still-live draft when available; service replacement
invalidates the token and cannot deliver into a different library. The take buffer
is cleared on each stop/cleanup. Existing saved/imported events are deep-copied
and appended to, not normalized. The complete-click composer remains unchanged.

## Assignment safety and transport

`AdmitX6LocalBlock` and `EncodeX6LocalUpload` accept the **104 exact physical
identities** below (103 distinct raw IDs, including one explicit alias) and five
existing mouse types. Complete balanced takes may overlap modifiers and mix
keyboard/mouse events in order. Balance is checked by raw encoded identity and
exact source identity: simultaneous Enter/NumpadEnter downs and mismatched alias
releases reject; sequential complete alias uses remain encodable. Unmatched
releases, repeated downs and still-held inputs reject. No upload event
is dropped, reordered or synthesized. Nonempty mouse `key_code` rejects ambiguity.
Opaque unsupported/padded keys and delays outside encoding bounds remain local.

The forward limits below apply before any association or upload write, including
validation of terminal source delay. Repeat is 1–255 and mode is 0–2. Every item,
destination, duplicate button and ordinary config is checked before first I/O.
Caller events/config are frozen before serialization. Invalid requests preserve
pending drafts, revision and progress except the public error outcome.

Desktop Apply checks library freshness and command capability before changing
pending state, rechecks binding/drafts at dispatch, and rejects stale completion
without hiding observed transport progress. A library change during I/O does not
alter the frozen payload or authorize local success. Restage to adopt saved edits.

Legacy one/two zero-delay complete clicks still use the original sequence command
API and byte-identical blocks. Timed/keyboard or mixed multi-button batches use
additive `MacroLocalItem`/`TargetedMacroLocalCommand`; missing capability fails
before writes, without falling back to generic report admission. No arbitrary
preencoded block is admitted to transport.

HID uses one ordinary-remap overlay report08, ordered destination-specific
report09 batches (60/60/8), captured framing and the existing assignment/final
upload ACK mechanism under the same ownership lock. Cancellation and stale-target
checks remain active between writes. Assignment-unknown/upload-possibly-partial
states retain evidence; failure never retries or rolls back hardware.

Transport ACK success means host-side transaction confirmation only. The audited
ordinary-key mapping is implemented; inverse timing, physical acceptance and
independent firmware slots remain outside this software integration.

Both `internal/macros/models.ts` binding mirrors were narrowly updated together
for `Keyboard` and optional `key_code`. Full generation via the `bindings` task
in `cmd/x6configurator/Taskfile.yml` rewrites the complete binding tree, so it was
not run in this bounded change. The constructor preserves absent mouse key fields.

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

## Ordered complete-click sequences (XM-7c)

`X6Sequence{Buttons: []EventType{...}, Repeat: int, Mode: uint8}` and
`EncodeX6SequenceBlock` / `DecodeX6SequenceBlock` admit exactly one or two
ordered complete clicks with the captured zero-delay layout. Accepted actions
are `MouseLeft`, `MouseRight`, and `EventType("mouse_middle")`,
`EventType("mouse_back")`, `EventType("mouse_forward")` (codes f1–f5).
`X6SequenceUpload{Destination: byte, Sequence: X6Sequence}` and
`EncodeX6SequenceUpload` / `DecodeX6SequenceUpload` preserve checked addressing.

The separate API leaves positional `X6Click` and `X6Upload` literals unchanged.
Legacy APIs retain their left/right single-click admission; they reject sequences
rather than silently reducing them. Legacy click APIs remain fixed-mode only;
nonfixed blocks/uploads are rejected rather than silently losing the mode.
Zero clicks, three or more clicks, non-button actions, partial transitions and
timing representations are outside this API. Decode checks the exact template,
including all unused bytes, paired transitions, checksum and report padding.
No local-event or delay conversion is provided.

Exact-byte tests read committed `new-sequences.json`: destination06 single
middle/back/forward, left, mixed left/right, right/left, forward/back,
middle/forward and duplicate uploads. These captures have repeat1 only.
Other one/two-button combinations and repeat2–255 are authorized layout
extrapolation, not proof of device playback or timing units.

## Autoclicker playback modes

`Mode` defaults to **0 (fixed repeat)**. **1 (continuous)** means press to start,
press again to stop; **2 (while held)** means playback while the trigger is held.
These are authorized semantics, not physically verified playback behavior.
Other mode values are rejected by both encoding and decoding.

Mode occupies logical byte **0**; repeat remains byte **4**, with the existing
1–255 admission for every mode. Mode participates in the additive big-endian
checksum at bytes **126–127**. Uploads keep the same **60/60/8** chunk framing,
including final header `09 0c <destination> 02`. No local timer or device I/O is
introduced by the codec. Existing capture fixtures remain fixed mode.

## Raw timed events: forward offline encoding only

`EncodeX6TimedBlock([]X6TimedEvent, repeat, mode)` creates a logical block
without changing the legacy mouse codec or local library. This raw offline API
alone is not transport admission; the separate local-event gate owns assignment.
Each raw event carries `Code byte`, `Action` (Down/Up), and `DelayMS int64`.
Codes are copied verbatim: this API provides no keyboard mapping or support list.

Delay belongs **after the preceding event**, represented by that event's pair.
The terminal event always encodes 10, regardless of its valid source delay.
For unsigned16 source delays, zero normalizes to 1; the writer rounds delay/10
(half up), then ORs with 01 for down or 81 for up. At **1271 and above**, it
uses quotient `floor(delay/200)` and remainder `delay%200` (zero becomes 1),
encoding the remainder on the event and appending `[quotient, 03]`.

| Rejection boundary | Policy |
|---|---|
| Source events | Require 1–46 |
| Source delays, including terminal | Require 0–65535 and quotient at most 255; 51200 rejects |
| Encoded extensions | At most 10, never silently drop later extensions |
| Output pairs | At most 50, leaving checksum bytes untouched |
| Repeat / mode | Require 1–255 / 0–2 |

These are **host safeguards**, enforced by admission, not firmware capacity
claims. The official UI recorder stops when its weighted count (raw events plus
one for each delay >1280) exceeds 45, with at most 46 raw events; saving requires
weighted count <48. Its writer instead extends at >=1271, with a 100-byte pair
buffer, only ten proper markers, malformed duplicate-event behavior thereafter,
and quotient-byte truncation above 255. We do not reproduce that loss/overflow:
46 source / 50 encoded pairs / 10 extensions / quotient <=255 remain independently
validated. An oversize take can still be saved/exported locally; rejection never
truncates the draft or silently edits actions, order or source delays.

Every rejection returns nil output and preserves the input. The block uses mode
at byte 0, repeat at byte 4, pair count at byte 25, pairs at byte 26 and the
existing additive checksum at bytes 126–127. Unused bytes remain zero.

This is traced **host arithmetic**, not proof of firmware acceptance or physical
timing. In particular, 51199 is arithmetically encodable, not playback-verified.
OR encoding loses information; no inverse timing decoder is provided. Local
editor recording does not establish firmware timing or playback acceptance.
Assignment uses the separate strict local-event gate. Legacy APIs retain their
original template limitations.

## Official ordinary-key physical identities

`MapX6LocalEvents([]Event)` returns independent `[]X6TimedEvent` values. It
validates local type/action/identity and nonnegative delays, preserves order and
source timing **after each event**, and does not overwrite terminal timing.
`EncodeX6TimedBlock` remains responsible for unsigned16, capacity, extension,
repeat/mode limits and the traced terminal 10 rule. Empty local sequences map to
an empty slice; they still cannot be encoded as a timed block.

The canonical [`x6keys.json`](x6keys.json) is embedded by Go and imported directly
by frontend `x6keys.ts`. Independent expected-row/range tests in both languages
assert every identity/code and the exact cardinality; neither derives expected
values from the JSON under test. Own-property lookup rejects `__proto__`,
`constructor` and `toString`, as well as unknown/lowercase/padded identities.

| Category | Exact browser physical identities | Official raw IDs |
|---|---|---|
| Letters (26) | `KeyA`–`KeyZ` | `04`–`1d` |
| Digits (10) | `Digit1`–`Digit9`, `Digit0` | `1e`–`27` |
| Basic controls (5) | Enter, Escape, Backspace, Tab, Space | `28`–`2c` |
| OEM positions (11) | Minus, Equal, BracketLeft/Right, Backslash; Semicolon, Quote, Backquote, Comma, Period, Slash | `2d`–`31`; `33`–`38` |
| CapsLock (1) | CapsLock | `39` |
| Function keys (12) | F1–F12 | `3a`–`45` |
| Navigation/locks (14) | PrintScreen, ScrollLock, Pause, Insert, Home, PageUp, Delete, End, PageDown, ArrowRight/Left/Down/Up, NumLock | `46`–`53` |
| Numpad (15) | Divide, Multiply, Subtract, Add; Numpad1–9, Numpad0, NumpadDecimal | `54`–`57`; `59`–`63` |
| Context menu (1) | ContextMenu | `65` |
| Sided modifiers (8) | ControlLeft, ShiftLeft, AltLeft, MetaLeft; corresponding Right identities | `e0`–`e7` |
| Explicit inferential alias (1) | NumpadEnter → VK_RETURN, same as Enter | `28`, **not** `58` |
| Existing mouse buttons | Five mouse event types, unchanged | `f1`–`f5` |

Provenance: parent-supplied static decoding of official X6.exe SHA256
`154b825d700231f980883376f332fe78b93d295aa5ce8ef839ab41341aba8f6e`,
converter `0x401780`, 112 12-byte VK/event/scan rows at `0x41cef0`
(PE file offset `0x1b4f0`). Parent independently decoded all 1344 bytes. Existing
capture-backed A/Ctrl/Shift uploads remain indexed in
[`keyboard-captures.json`](../../captures/0x09-macro/keyboard-captures.json).
No external executable is launched by the mapper/tests.

**Physical position, not typed-character parity:** named OEM positions are the
explicit browser-code/official VK identity seam (VK_BD/BB/DB/DD/DC,
BA/DE/C0/BC/BE/BF respectively). Official Windows conversion remaps scans through
MapVirtualKeyW for raw IDs `04`–`45`; Linux uses these physical identities rather
than inferring VK from `event.key` or the active layout's typed character. This
is not a claim of equivalent text on arbitrary layouts or observed playback.
Browser/OS-reserved keys may never reach the focused recorder.

**Declared exceptions:** F13–F24, ISO OEM102/IntlBackslash, media/browser VK
A6–B7 and generic Shift/Ctrl/Alt have no supported official ordinary rows.
VK_SEPARATOR (6C → `58`) and OEM8 (DF → `38`, colliding with OEMslash) have
no demonstrated browser physical-code seam and are deliberately not invented.
NumpadEnter is an explicit inferential VK_RETURN alias, not evidence for the
separator row. IME VK F0–F4/F7–F8 have static same-byte rows but no demonstrated
browser translation and collide with mouse F1–F5: they remain unsupported.
Local storage retains all these identities losslessly; mapping rejects with nil
output. Alias physical behavior is not verified beyond this static policy.

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

Physical timing/delay units, playback count, device persistence, firmware layout
capacity and slot rules remain unverified. The traced host forward arithmetic
encodes local `DelayMS`; zero does not imply a measured zero hardware delay.
Software bounds protect against host truncation/overflow, not proven firmware
limits. Legacy template decoders intentionally remain strict: OR timing is
noninvertible, so no timed inverse decoder or readback guarantee is offered.
