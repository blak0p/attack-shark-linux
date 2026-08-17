# X6 Capture Plan

How to capture the official Attack Shark X6 Windows app traffic so the Linux
driver can replicate every report with evidence.

## Setup

- Host: Linux.
- Windows runs in a VirtualBox VM with the X6 dongle attached via USB
  passthrough (`1D57:FA60`).
- Inside the VM: Wireshark + USBPcap. Capture on a **USBPcap** interface.
- Shared folder between VM and host: `~/x6-capturas` (host), auto-mounted in
  Windows. This is the drop zone for `.pcapng` files.

## Capture procedure (per scenario)

1. Open Wireshark in the VM, double-click a **USBPcap** interface (start).
2. Open the X6 app, wait ~5 s for the startup burst to finish.
3. Perform **one** action.
4. Stop capture (square button), Save As `captures/<name>.pcapng`.
5. Copy the file into the shared folder; it lands on the host.
6. Move it into the repo `captures/` directory and run:

   ```sh
   python3 tools/parse_usbpcap.py captures/<name>.pcapng
   ```

## Scenario matrix

| # | File name | Action in the app | Reports to expect |
|---|---|---|---|
| 1 | `dpi.pcapng` | Change DPI of a stage | `0x04` re-sent with new DPI + checksum |
| 2 | `stage.pcapng` | Change active stage (1-6) | `0x04` re-sent, stage byte changes |
| 3 | `rgb.pcapng` | Change a lighting **mode** (off, steady, fixed DPI, ...) | `0x04` re-sent, bytes 25-48 (mode/color block) change |
| 4 | `polling.pcapng` | Change polling rate (125/250/500/1000) | `0x06` |
| 5 | `remap.pcapng` | Remap a mouse button to a key | `0x08` |
| 6 | `macro.pcapng` | Record / delete a macro | `0x09` (candidate) |
| 7 | `dpibutton.pcapng` | Press the physical DPI button on the mouse | interrupt status event |
| 8 | `sleep.pcapng` | Leave mouse idle ~2 min, then move it | sleep/wake event |
| 9 | `profile.pcapng` | Add / rename / switch a profile | profile report |
| 10 | `battery.pcapng` | Open Battery panel, watch it refresh | `0x03` interrupt |

> Firmware update is deliberately excluded: it writes device state and carries
> hardware risk. It belongs to the write phase, not read-only capture.

## Analysis workflow

1. Run `tools/parse_usbpcap.py` on the new pcap.
2. Compare the report payloads against `docs/protocol-captures.md`.
3. Note which bytes changed between two SET_REPORTs of the same report: the
   changed byte is the field the action edits.
4. Update `docs/protocol-captures.md` and, when confirmed, `docs/protocol-x6.md`.

## Rules

- One action per capture file.
- Keep the raw `.pcapng` in `captures/` as evidence ("guardar a fuego").
- Documentation lives in `docs/` and is written in **English**.
- Never capture or store the firmware update flow in this phase.
