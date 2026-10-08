# X6 report 0x09 macro evidence (XM-4a)

Passive capture evidence only: no replay or device writes. XM-7a adds seven
byte-preserved PCAP copies; the original XM-4a fixtures/contract remain unchanged.
`contract.json` records original sizes/SHA-256, exact physical reports, one-based
`tshark frame.number` provenance, fixture hashes and zero-based byte differences.
The five `.bin` fixtures are **128-byte logical blocks**, not wire reports.
Originals remain external at `/home/alejandro/x6-capturas/` and were read only.
The XM-7a copies and their independent manifest are described below.

## Captured contract

Each selected upload has three 64-byte reports with headers `09 40 05 00`,
`09 40 05 01`, `09 0c 05 02`. Strip each four-byte header, retain respectively
60, 60 and 8 bytes, then concatenate. The final report's remaining 52 bytes
are observed zero padding, excluded from the logical block. Block ID `05` is
observed; no X3 slot or block-ID semantics are imported.

The observed checksum is the sum of bytes `[0:126]`, stored big-endian at
`[126:128]`. These fixtures do not establish an overflow rule for other blocks.

| Fixture | Original stem | One-based frames | Checksum |
| --- | --- | --- | --- |
| left-repeat-1.bin | macro_izq | 11837 / 12489 / 12543 | 0267 |
| right-repeat-1.bin | macro_derecho | 10693 / 10753 / 10783 | 0269 |
| left-repeat-2.bin | macro_izq_new_loop2 | 10861 / 10945 / 10947 | 0268 |
| left-repeat-2.bin (same bytes) | macro_izq_loop3 | 13477 / 13481 / 13483 | 0268 |
| left-repeat-3.bin | macro_izq_loop3 | 16745 / 16863 / 17983 | 0269 |
| left-repeat-3.bin (same bytes) | macro_izq_loop3 | 18407 / 18411 / 18413 | 0269 |
| left-repeat-5.bin | macro_izq_loop5 | 11595 / 11683 / 11713 | 026b |

Relative to left-repeat-1, captured left values 2, 3 and 5 change only offset
4 and checksum byte 127. Right-repeat-1 changes event-code bytes 27 and 29
from `f1` to `f2` and checksum byte 127 from `67` to `69`. Nonzero base bytes
are 4=`01`, 25=`02`, 26=`01`, 27=`f1`, 28=`81`, 29=`f1`, 126=`02`, 127=`67`.
Left/right and repeat labels reflect the supplied capture scenarios; the
byte comparisons alone do not establish full event or timing semantics.

Negative control `macro_izq_loop2.pcapng` has no report 09 in the extracted
`usb.data_fragment` field; report 08 at frame 6331 is recorded in JSON.
Its filename is **not** evidence for repeat 2. This is a field-scoped observation,
not a general claim about every possible USB representation in the capture.

## Boundaries

- Captured repeat bytes: **1, 2, 3, 5**. The vendor app UI maximum **255** is
  user-confirmed, not capture-derived; no 255 capture is available.
- Values between/beyond those captures are interpolation/extrapolation, not
  verified protocol acceptance. Full field width/range is not proven.
- Hardware timing, delay units, persistence, execution count and replay remain
  unknown. No device was contacted; captured uploads do not prove playback.
- Local names, IDs and other vendor metadata are not reconstructed or invented.
- This is an evidence subset, not an exhaustive enumeration of all uploads.
  Report 08 mappings and unrelated traffic are intentionally not fixtures.

## Read-only reproduction and readback

From the repository root, with external originals and tshark available, run:

```sh
python -B - <<'PY'
import hashlib, json, subprocess
from pathlib import Path
out = Path('captures/0x09-macro')
doc = json.loads((out / 'contract.json').read_text())
sha = lambda b: hashlib.sha256(b).hexdigest()
for entry in doc['blocks']:
    b = (out / entry['file']).read_bytes()
    assert len(b) == entry['bytes'] == 128 and sha(b) == entry['sha256']
    assert sum(b[:126]) == int.from_bytes(b[126:], 'big')
    assert b[126:].hex() == entry['checksum_be_hex']
    assert b[4] == entry['observed_repeat_byte']
    assert {str(i): f'{v:02x}' for i, v in enumerate(b) if v} == entry['nonzero_offsets']
    base = (out / 'left-repeat-1.bin').read_bytes()
    assert [{'offset': i, 'before': f'{base[i]:02x}', 'after': f'{v:02x}'}
            for i, v in enumerate(b) if v != base[i]] == entry['diff_from_left_repeat_1']
for source in doc['sources']:
    p = Path(doc['source_root']) / source['file']
    original = p.read_bytes()
    assert len(original) == source['bytes'] and sha(original) == source['sha256']
    for upload in source['uploads']:
        expr = ' || '.join(f'frame.number == {n}' for n in upload['frames'])
        rows = subprocess.check_output(['tshark', '-r', str(p), '-Y', expr,
            '-T', 'fields', '-e', 'frame.number', '-e', 'usb.data_fragment'], text=True).splitlines()
        assert len(rows) == 3
        assert [int(row.split('\t')[0]) for row in rows] == upload['frames']
        reports = [bytes.fromhex(row.split('\t')[1]) for row in rows]
        assert [r.hex() for r in reports] == upload['reports_hex']
        assert all(len(r) == 64 for r in reports)
        assert [r[:4].hex() for r in reports] == doc['extraction']['headers_hex']
        assert reports[2][12:] == bytes(52)
        block = b''.join(r[4:4+n] for r, n in zip(reports, [60, 60, 8]))
        assert block == (out / upload['fixture']).read_bytes()
    if 'negative_control' in source:
        rows = subprocess.check_output(['tshark', '-r', str(p), '-Y', 'usb.data_fragment',
            '-T', 'fields', '-e', 'frame.number', '-e', 'usb.data_fragment'], text=True).splitlines()
        fragments = [(int(n), bytes.fromhex(h)) for n, h in (row.split('\t') for row in rows)]
        assert not [n for n, b in fragments if b and b[0] == 9]
        for report in source['negative_control']['report08']:
            assert (report['frame'], bytes.fromhex(report['report_hex'])) in fragments
    assert sha(p.read_bytes()) == source['sha256']
print('PASS: 5 fixture hashes/checksums/diffs; 6 original hashes/sizes; 7 frame groups; negative control')
PY
git diff --check
git status --short
```

No meaningful RED applies to passive evidence preservation. The reproduction
assertions verify extraction/readback rather than implementing behavior.

## XM-7a: complete-click sequence captures

`new-sequences.json` records seven originals' filenames, sizes, before/after
SHA-256 and copy hashes, plus all nine report09 groups with one-based frames,
wire bytes, logical blocks, checksums and ordered decoded event records.

| External original | Repository copy |
| --- | --- |
| izq_der_macro.pcapng | left-then-right-clicks.pcapng |
| der_izq_macro.pcapng | right-then-left-clicks.pcapng |
| adelante_atras.pcapng | forward-then-back-clicks.pcapng |
| medio_adelante_macro.pcapng | middle-then-forward-clicks.pcapng |
| adelante_macro.pcapng | forward-click.pcapng |
| atras.pcapng | back-click.pcapng |
| macro_boton_medio(no_se_cual_es_realmetne_).pcapng | middle-click.pcapng |

**izq_der contains three uploads**: earlier single-left at
13303/13315/13335, then identical mixed left/right uploads at
15873/16051/16165 and 16169/16173/16175. Duplication does not prove retry safety.
The middle original filename expresses uncertainty; its label reflects the
supplied scenario and code f3, not independently verified physical playback.

All groups address destination06, independently of event action codes, with
headers 09400600/09400601/090c0602 and 60+60+8 logical bytes.
Offset4 is repeat01; offset25 counts two or four two-byte event records starting
at offset26. States01/81 alternate press/release; f1/f2/f3/f4/f5 denote
left/right/middle/back/forward in this bounded capture-backed interpretation.
The checksum sums bytes0:126 and is stored big-endian at126:128. Wire padding
is zero. No source admission, timing, persistence or playback claim changes.

### Read-only new-sequence reproduction

Run from the repository root with tshark and the external originals available:

```sh
python -B - <<'PY'
import hashlib, json, subprocess
from pathlib import Path
out = Path('captures/0x09-macro')
doc = json.loads((out / 'new-sequences.json').read_text())
sha = lambda b: hashlib.sha256(b).hexdigest()
assert len(doc['sources']) == 7
for s in doc['sources']:
    original = Path(doc['source_root']) / s['original_file']
    before = original.read_bytes()
    assert len(before) == s['bytes']
    assert sha(before) == s['sha256'] == s['original_sha256_after'] == s['copy_sha256']
    assert (out / s['file']).read_bytes() == before
    for p in (original, out / s['file']):
        rows = subprocess.check_output(['tshark', '-r', str(p), '-Y',
            'usb.data_fragment', '-T', 'fields', '-e', 'frame.number',
            '-e', 'usb.data_fragment'], text=True).splitlines()
        reports = [(int(n), bytes.fromhex(h)) for n, h in
                   (r.split('\t') for r in rows)]
        reports = [(n, b) for n, b in reports if b[0] == 9]
        assert len(reports) == 3 * len(s['uploads'])
        for i, u in enumerate(s['uploads']):
            group = reports[3*i:3*i+3]
            wire = [b for _, b in group]
            assert [n for n, _ in group] == u['frames']
            assert [b.hex() for b in wire] == u['reports_hex']
            assert all(len(b) == 64 for b in wire)
            assert [b[:4].hex() for b in wire] == doc['extraction']['headers_hex']
            assert u['destination_hex'] == '06' and wire[2][12:] == bytes(52)
            b = b''.join(r[4:4+n] for r, n in zip(wire, [60,60,8]))
            assert len(b) == 128 and b.hex() == u['logical_block_hex']
            assert sha(b) == u['logical_block_sha256']
            assert sum(b[:126]) == int.from_bytes(b[126:], 'big')
            assert b[126:].hex() == u['checksum_be_hex']
            assert b[4] == u['repeat_byte'] == 1
            assert b[25] == u['event_count'] == len(u['events']) and b[25] in (2,4)
            for j, e in enumerate(u['events']):
                state, code = b[26+2*j:28+2*j]
                assert state == (1 if j % 2 == 0 else 129)
                assert e['state_hex'] == f'{state:02x}' and e['code_hex'] == f'{code:02x}'
                assert e['action'] == doc['event_codes'][e['code_hex']]
                assert e['transition'] == ('press' if state == 1 else 'release')
                if j % 2:
                    assert e['code_hex'] == u['events'][j-1]['code_hex']
    assert original.read_bytes() == before
u = doc['sources'][0]['uploads']
assert len(u) == 3 and u[0]['event_count'] == 2
assert u[1]['event_count'] == u[2]['event_count'] == 4
assert u[1]['logical_block_hex'] == u[2]['logical_block_hex']
assert sum(len(s['uploads']) for s in doc['sources']) == 9
print('PASS: seven original/copy hashes/sizes; nine frame groups, destination06, events and checksums')
PY
git diff --check
```

## XM-8a: multi-macro assignment capture (two-macros.pcapng)

`two-macros.pcapng` proves that the Attack Shark X6 firmware supports multiple simultaneous macro button assignments in report 0x08 and independent report 0x09 sequence uploads.

| External original | Repository copy | Bytes | SHA-256 |
| --- | --- | --- | --- |
| `/home/alejandro/x6-capturas/2 macros.pcapng` | `two-macros.pcapng` | 1148748 | `1121cf4b6b28b60939dce02a1515361f6e1ae120ff97c35c7d5821c7bd5cdc30` |

### Observed frames

- **Frame 7155 (Report 0x08)**: Single macro assignment to Button 6 (dest 05, bytes `12 00 05` at offset 15..17).
- **Frames 7353, 8309, 8313 (Report 0x09)**: Macro upload targeting destination 05 (`09 40 05 00`, `09 40 05 01`, `09 0c 05 02`).
- **Frame 11647 (Report 0x08)**: Multi-macro assignment with **both Button 6 and Button 7 active**:
  - Offset 15..17 (Group 5, Button 6): `12 00 05`
  - Offset 18..20 (Group 6, Button 7): `12 00 06`
  - Checksum at offset 57..58: `00 a6` (sum of bytes 3..56 is 166 = `0x00a6`).
  - Wire bytes (59):
    `08 3b 01 02 00 00 03 00 00 04 00 00 0d 00 00 12 00 05 12 00 06 06 00 00 05 00 00 3c 00 00 01 00 00 01 00 00 01 00 00 01 00 00 01 00 00 01 00 00 01 00 00 0a 00 00 09 00 00 00 a6`
- **Frames 12427, 12561, 12801 (Report 0x09)**: Macro upload targeting destination 06 (`09 40 06 00`, `09 40 06 01`, `09 0c 06 02`).
