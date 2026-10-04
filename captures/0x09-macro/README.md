# X6 report 0x09 macro evidence (XM-4a)

Passive capture evidence only: no encoder, replay, device writes or copied PCAPs.
`contract.json` records original sizes/SHA-256, exact physical reports, one-based
`tshark frame.number` provenance, fixture hashes and zero-based byte differences.
The five `.bin` fixtures are **128-byte logical blocks**, not wire reports.
Originals remain external at `/home/alejandro/x6-capturas/` and were read only.

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
