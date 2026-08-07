#!/usr/bin/env python3
"""Decode the X6 button remap report (0x08, 59 B) from captured evidence.

Usage:
    python3 tools/decode_remap.py captures/0x08-remap/btn6_forward.pcapng
    python3 tools/decode_remap.py --all captures/0x08-remap/
"""

import argparse
import struct
import sys
from pathlib import Path

from scapy.all import rdpcap

OUT_CLASS = 0x21
SET_REPORT = 0x09
REPORT_TYPE_FEATURE = 0x03
IFACE_CONFIG = 0x02

ACTIONS = {
    0x01: "Button Off",
    0x02: "Left Click",
    0x03: "Right Click",
    0x04: "Middle Button",
    0x05: "Backward",
    0x06: "Forward",
    0x07: "Double Click",
    0x08: "Fire Button",
    0x09: "Scroll Up",
    0x0a: "Scroll Down",
    0x0d: "DPI Cycle",
    0x0e: "DPI+",
    0x0f: "DPI-",
    0x10: "Easy Aim",
    0x11: "Shortcut",
    0x15: "Media Player",
    0x16: "Previous Track",
    0x17: "Next Track",
    0x18: "Play / Pause",
    0x19: "Stop",
    0x1a: "Mute",
    0x1b: "Volume +",
    0x1c: "Volume -",
    0x1d: "Browser: Calculator",
    0x1e: "Browser: Email",
    0x1f: "Browser: Favorites",
    0x20: "Browser: Forward",
    0x21: "Browser: Backward",
    0x22: "Browser: Stop",
    0x23: "Browser: My Computer",
    0x24: "Browser: Refresh",
    0x25: "Browser: Home",
    0x26: "Browser: Search",
    0x3c: "UNKNOWN",
}

MODS = {0x01: "Ctrl", 0x02: "Shift", 0x04: "Alt", 0x08: "Win"}


def action_name(aid, p1, p2):
    name = ACTIONS.get(aid, f"UNKNOWN 0x{aid:02x}")
    if aid == 0x10:
        return f"{name} (level {p1})"
    if aid == 0x11:
        mods = ", ".join(m for bit, m in MODS.items() if p1 & bit) or "none"
        return f"{name} mods={mods} key=0x{p2:02x}"
    return name


def find_reports(path):
    packets = rdpcap(str(path))
    reports = []
    for pkt in packets:
        d = bytes(pkt)
        for idx in range(len(d) - 8):
            if d[idx] != OUT_CLASS:
                continue
            if d[idx + 1] != SET_REPORT:
                continue
            w_value = struct.unpack("<H", d[idx + 2 : idx + 4])[0]
            w_index = struct.unpack("<H", d[idx + 4 : idx + 6])[0]
            if (w_value >> 8) != REPORT_TYPE_FEATURE or w_index != IFACE_CONFIG:
                continue
            rid = w_value & 0xFF
            if rid != 0x08:
                continue
            payload = d[idx + 8 :]
            reports.append(payload)
    return reports


def decode(payload):
    assert payload[0] == 0x08, "not a 0x08 report"
    groups = [payload[i : i + 3] for i in range(3, 57, 3)]
    checksum = struct.unpack(">H", payload[57:59])[0]
    calc = sum(payload[3:57]) & 0xFFFF
    ok = checksum == calc
    return groups, checksum, calc, ok


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("path")
    ap.add_argument("--all", action="store_true", help="treat path as a dir, decode every 0x08")
    args = ap.parse_args()

    paths = sorted(Path(args.path).glob("*.pcapng")) if args.all else [Path(args.path)]
    for path in paths:
        for payload in find_reports(path):
            if len(payload) < 59:
                continue
            groups, checksum, calc, ok = decode(payload)
            status = "OK" if ok else f"FAIL sum={calc:04x}"
            print(f"=== {path.name} === checksum {checksum:04x} [{status}]")
            for i, g in enumerate(groups, 1):
                aid, p1, p2 = g
                if aid == 0x01 and i in (10, 11, 12, 13, 14, 15, 16):
                    continue  # factory-empty slots
                print(f"  g{i:02d}: {action_name(aid, p1, p2)}")
            print()
        else:
            pass


if __name__ == "__main__":
    main()
