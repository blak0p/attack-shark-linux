#!/usr/bin/env python3
"""Parse USBPcap captures of the Attack Shark X6 configuration interface.

Extracts the SET_REPORT / GET_REPORT control transfers and the interrupt
status (ACK / heartbeat) reports from a pcapng file produced by USBPcap on
the Windows host (VirtualBox USB passthrough).

Usage:
    python3 tools/parse_usbpcap.py captures/dpi.pcapng
    python3 tools/parse_usbpcap.py --json captures/dpi.pcapng

Layout (observed on USBPcap pcapng):
    [28 bytes USBPcap/linux usbmon header]
    [8 bytes control setup  bmRequestType bRequest wValue wIndex wLength]
    [2 bytes wLength LE]
    [report payload]

The vendor configuration endpoint is interface 2, report type feature (0x03).
"""

import argparse
import json
import struct
import sys

from scapy.all import rdpcap

REPORT_TYPE_FEATURE = 0x03
IFACE_CONFIG = 0x02

OUT_CLASS = 0x21
IN_CLASS = 0xA1
SET_REPORT = 0x09
GET_REPORT = 0x01


def parse_pcap(path):
    packets = rdpcap(path)
    events = []
    for i, pkt in enumerate(packets):
        d = bytes(pkt)
        # Search for a class-interface control setup: OUT (0x21) or IN (0xA1)
        for idx in range(len(d) - 8):
            bm = d[idx]
            if bm not in (OUT_CLASS, IN_CLASS):
                continue
            b_req = d[idx + 1]
            w_value = struct.unpack("<H", d[idx + 2 : idx + 4])[0]
            w_index = struct.unpack("<H", d[idx + 4 : idx + 6])[0]
            w_length = struct.unpack("<H", d[idx + 6 : idx + 8])[0]

            report_type = (w_value >> 8) & 0xFF
            report_id = w_value & 0xFF
            if report_type != REPORT_TYPE_FEATURE or w_index != IFACE_CONFIG:
                continue

            name = "SET_REPORT" if b_req == SET_REPORT else (
                "GET_REPORT" if b_req == GET_REPORT else f"bReq={b_req:02x}"
            )
            direction = "HOST->DEV" if bm == OUT_CLASS else "DEV->HOST"

            # Report payload starts right after the 8-byte setup. The report
            # id is wValue's low byte (already captured as report_id).
            data = d[idx + 8 :]

            events.append(
                {
                    "frame": i,
                    "dir": direction,
                    "name": name,
                    "report_id": report_id,
                    "wLength": w_length,
                    "payload_len": len(data),
                    "data": data,
                }
            )
            break

        # Interrupt status reports (ACK / heartbeat)
        for marker, label in ((b"\x03\x10\x50", "ACK"), (b"\x03\x10\x40", "HEARTBEAT")):
            idx = d.find(marker)
            if idx >= 0:
                events.append(
                    {
                        "frame": i,
                        "dir": "DEV->HOST",
                        "name": label,
                        "report_id": None,
                        "wLength": None,
                        "payload_len": None,
                        "data": d[idx : idx + 5],
                    }
                )
                break
    return events


def fmt_bytes(b):
    return " ".join(f"{x:02x}" for x in b)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("pcap")
    ap.add_argument("--json", action="store_true")
    args = ap.parse_args()

    events = parse_pcap(args.pcap)
    if args.json:
        out = []
        for e in events:
            o = dict(e)
            o["data_hex"] = fmt_bytes(e["data"])
            del o["data"]
            out.append(o)
        print(json.dumps(out, indent=2))
        return

    for e in events:
        extra = ""
        if e["name"] in ("SET_REPORT", "GET_REPORT"):
            extra = f" report_id=0x{e['report_id']:02x} wLength={e['wLength']}"
        print(
            f"frame {e['frame']:5d}  {e['dir']:9s} {e['name']}{extra:30s} "
            f"= {fmt_bytes(e['data'])}"
        )


if __name__ == "__main__":
    main()
