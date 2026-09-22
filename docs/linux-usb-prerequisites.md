# Linux USB Build and Device Access Prerequisites

The Linux HID adapter uses kernel-backed hidraw nodes for both passive status and
DPI Apply. It does not use gousb/libusb, claim USB interfaces, or detach kernel
drivers. Install the canonical packaged udev policy so the active local-seat user can
access the Attack Shark X6 hidraw node without running the app as root or making
the device world-writable. The release asset is
`60-attack-shark-x6-hidraw.rules`; do not substitute an obsolete `99-` rule.

## Quick path: install the udev rule

1. From the repository root, copy the shipped rule and reload udev:

   ```sh
   sudo install -Dm0644 packaging/udev/60-attack-shark-x6-hidraw.rules /etc/udev/rules.d/60-attack-shark-x6-hidraw.rules
   sudo udevadm control --reload-rules
   sudo udevadm trigger
   ```

2. Unplug and reconnect the dongle. The rule matches only `1d57:fa60`, uses
   `TAG+="uaccess"` for the active seat user, and keeps device mode `0660`.

The policy never requires root and must not be changed to world-writable `0666`.

## Build prerequisite

The release packaging toolchain uses Wails
`github.com/wailsapp/wails/v3 v3.0.0-beta.23`; this document does not upgrade
it. Build and run the fake-only hidraw tests with:

```sh
# Generate the assets embedded by cmd/x6configurator before Go checks or builds.
(cd frontend && npm ci && npm run build)
go test ./internal/hidlinux -run 'TestHidraw(SendAndAwait|ReadInterruptIN|Enumerate|ValidateDescriptor)' -count=1
go build ./cmd/x6configurator
```

## Non-logind alternative: static group policy

`uaccess` is the preferred policy because it grants access to the active local
seat user. On systems without logind/uaccess support, use a static group instead:

```sh
sudo groupadd --system attack-shark-x6
sudo usermod -aG attack-shark-x6 "$USER"
sudo install -Dm0644 packaging/udev/60-attack-shark-x6-hidraw.rules /etc/udev/rules.d/60-attack-shark-x6-hidraw.rules
```

Then replace the installed rule's final action with this group policy, reload
udev, and replug the dongle:

```udev
SUBSYSTEM=="hidraw", ATTRS{idVendor}=="1d57", ATTRS{idProduct}=="fa60", GROUP="attack-shark-x6", MODE="0660"
```

```sh
sudo udevadm control --reload-rules
sudo udevadm trigger
```

Log out and back in after joining the group. The tradeoff is deliberate: the
group grants static membership-based access, whereas `uaccess` grants access per
active local seat.

## Release installer boundary

The signed RC installer can install the AppImage under the current OS account's
user-local data directory. Its optional udev flow is separate, requires explicit
approval before `sudo`, and otherwise provides a manual fallback. The AppImage
updater never elevates and never changes this udev policy.

## Permission-denied recovery

The application maps `os.ErrPermission` to the UI error code
`permission_denied`. This means Linux denied access to the validated dongle; it
does not mean the app should be run as root. Check, in order:

- The rule is installed at `/etc/udev/rules.d/60-attack-shark-x6-hidraw.rules`.
- Rules were reloaded, the trigger command ran, and the dongle was replugged.
- For group policy, the user belongs to `attack-shark-x6` and has logged in again.
- The dongle identity still appears as `1d57:fa60`.

## Troubleshooting

Confirm the USB identity:

```sh
lsusb -d 1d57:fa60
```

Inspect udev attributes after locating an X6 hidraw node (replace the
placeholder with the hidraw node reported by your system):

```sh
udevadm info -a -n /dev/hidrawN
```

The adapter reads status or acknowledgements from the vendor hidraw node only
after validating the X6 VID/PID, physical USB path, interface `2`, endpoint
`0x83`, and the HID report descriptor. A matching VID/PID alone is not
sufficient for the application's device validation.
