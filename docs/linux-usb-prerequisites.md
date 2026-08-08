# Linux USB Build and Device Access Prerequisites

The Linux HID adapter uses `github.com/google/gousb v1.1.3` with the system
`libusb-1.0` development files. Install the packaged udev policy so the active
local-seat user can access the Attack Shark X6 dongle without running the app as
root or making the device world-writable.

## Quick path: install libusb and the udev rule

1. Install the distribution prerequisite:

   ```sh
   # Fedora
   sudo dnf install libusb1-devel

   # Debian/Ubuntu
   sudo apt install libusb-1.0-0-dev
   ```

2. From the repository root, copy the shipped rule and reload udev:

   ```sh
   sudo install -Dm0644 packaging/udev/99-attack-shark-x6.rules /etc/udev/rules.d/99-attack-shark-x6.rules
   sudo udevadm control --reload-rules
   sudo udevadm trigger
   ```

3. Unplug and reconnect the dongle. The rule matches only `1d57:fa60`, uses
   `TAG+="uaccess"` for the active seat user, and keeps device mode `0660`.

The policy never requires root and must not be changed to world-writable `0666`.

## Build prerequisite

Wails remains pinned to `github.com/wailsapp/wails/v3 v3.0.0-beta.5`; this
document does not upgrade it. Verify that libusb is available before building:

```sh
pkg-config --modversion libusb-1.0
go test -tags gousb_prerequisite ./internal/hidlinux -run TestGousbFoundationUsesPinnedModule -count=1
go build ./cmd/x6configurator
```

## Non-logind alternative: static group policy

`uaccess` is the preferred policy because it grants access to the active local
seat user. On systems without logind/uaccess support, use a static group instead:

```sh
sudo groupadd --system attack-shark-x6
sudo usermod -aG attack-shark-x6 "$USER"
sudo install -Dm0644 packaging/udev/99-attack-shark-x6.rules /etc/udev/rules.d/99-attack-shark-x6.rules
```

Then replace the installed rule's final action with this group policy, reload
udev, and replug the dongle:

```udev
SUBSYSTEM=="usb", ATTRS{idVendor}=="1d57", ATTRS{idProduct}=="fa60", GROUP="attack-shark-x6", MODE="0660"
```

```sh
sudo udevadm control --reload-rules
sudo udevadm trigger
```

Log out and back in after joining the group. The tradeoff is deliberate: the
group grants static membership-based access, whereas `uaccess` grants access per
active local seat.

## Permission-denied recovery

The application maps `os.ErrPermission` to the UI error code
`permission_denied`. This means Linux denied access to the validated dongle; it
does not mean the app should be run as root. Check, in order:

- The rule is installed at `/etc/udev/rules.d/99-attack-shark-x6.rules`.
- Rules were reloaded, the trigger command ran, and the dongle was replugged.
- For group policy, the user belongs to `attack-shark-x6` and has logged in again.
- The dongle identity still appears as `1d57:fa60`.

## Troubleshooting

Confirm the USB identity:

```sh
lsusb -d 1d57:fa60
```

Inspect udev attributes after locating the matching device node (replace the
placeholder with the bus/device path reported by your system):

```sh
udevadm info -a -n /dev/bus/usb/BBB/DDD
```

The adapter only reads status or acknowledgements from endpoint `0x83` after it
has claimed HID interface `2`. A matching VID/PID alone is not sufficient for
the application's device validation.
