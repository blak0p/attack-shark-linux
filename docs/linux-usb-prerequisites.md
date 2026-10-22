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

2. Unplug and reconnect the dongle or cable. The two exact hidraw rules match
   only the X6 dongle `1d57:fa60` and cable `1d57:fa61`, use
   `TAG+="uaccess"` for the active seat user, and keeps device mode `0660`.

The policy never requires root and must not be changed to world-writable `0666`.

## Build prerequisite

The release packaging toolchain uses Go 1.27.1 and Wails
`github.com/wailsapp/wails/v3 v3.0.0-beta.28`, with the CLI and frontend runtime
pinned to the same Wails version. Both Ubuntu builders copy Go 1.27.1 from the
versioned upstream image rather than installing Ubuntu's older Go package.
GTK4 and WebKitGTK 6 remain required native build dependencies.
Build and run the fake-only hidraw tests with:

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
SUBSYSTEM=="hidraw", ATTRS{idVendor}=="1d57", ATTRS{idProduct}=="fa61", GROUP="attack-shark-x6", MODE="0660"
```

```sh
sudo udevadm control --reload-rules
sudo udevadm trigger
```

Log out and back in after joining the group. The tradeoff is deliberate: the
group grants static membership-based access, whereas `uaccess` grants access per
active local seat.

## Release installer boundary

The release-asset installer selects signed stable releases by default and signed
RC releases only with `--beta`. Without a stable candidate, normal installation
fails instead of falling back to an RC or `main`. It installs the AppImage under
the current OS account's user-local data directory. Its optional udev flow is
separate, requires explicit approval before `sudo`, and otherwise provides a
manual fallback. The AppImage
updater never elevates; optional authenticated policy application uses only the
fixed installed helper. Updates remain in the installed stable or RC channel.

## One-time authenticated USB permissions setup

For releases publishing the signed helper and policy assets, use a **trusted,
versioned release installer** to set up future USB permission updates:

```sh
sh ./install.sh --setup-usb-permissions
# To select the signed RC channel instead:
sh ./install.sh --beta --setup-usb-permissions
```

This is separate from AppImage installation and the legacy `--install-udev`
manual-rule flow. Setup requires `/usr/bin/python3` and system OpenSSL. Python 3
is used only by this initial bootstrap, not by normal app use or the installed
shell helper. If Python is missing, install it through your distro package
manager and rerun setup; the installer never installs packages automatically.
Run setup as your normal user: it requests administrator authorization once
through system `sudo`, never collects a password in the app, and never runs an
AppImage as root. Trust in the installer and its embedded release key is required;
a downloaded AppImage is not a root trust anchor.

Setup verifies the selected release and helper/policy signatures before asking
for authorization. The fixed bootstrap program then copies bounded regular files
without following final symlinks, verifies that root-private copy again, and
installs only the authenticated helper at its canonical root-owned path. The
helper validates and applies the signed policy independently. Newly created
root-owned public helper parent directories are set to `0755`, even under the
bootstrap's private `077` umask. Existing directory modes are never changed;
private staging and runtime directories remain private. Custom and static-group
rules are preserved, not overwritten. There is no daemon.

Older releases without the three authentication assets cannot perform setup;
normal AppImage installation remains supported. Setup never changes the
AppImage. If helper installation succeeds but policy application or udev reload
fails, the output reports that partial result. Inspect it before retrying;
no automatic retry is performed. Reconnecting the mouse may still be necessary.
The desktop checks silently at startup. An upper-right Updates notice labels only
the available targets: AppImage, USB permissions (udev), or both. No loading or
up-to-date panel appears. Expand **Update details** for explanations. Policy
eligibility uses backend-verified contents, not a changed release version or raw
manifest. Apply, progress, individual errors and explicit **Relaunch app** remain
available without Device navigation. Failures stay visible. The notice scrolls
when needed; **Check for updates** never applies anything.
**Apply updates** is the single explicit approval action. Construction alone does
not fetch releases or authorize. The UI never bootstraps this helper or collects
a password; initial trusted setup above remains separate.

## Read-only policy library foundation

The Go `packaging/udev` package embeds the canonical rule above directly, without
another current-policy copy. `internal/udevpolicy` classifies injected read-only
policy sources as `current`, `missing`, `outdated`, `custom`, or `error` for the
fixed installed path. Only exact current bytes or the checked-in official
fa60-only snapshot are recognized; static-group rules, added directives, and
other local edits remain custom. Unreadable, symlink, nonregular, or oversized
inputs produce errors, not permission to replace them.

This stage is a read-only library foundation: on Linux,
`Inspect(InstalledSource())` reads only the fixed installed path, using a bounded,
no-follow, nonblocking open and a descriptor regular-file check. It rejects
final-component symlink/nonregular replacements independently of the initial
inspection. No caller-selected installed path or environment override is exposed.
Startup wires this fixed read-only source for automatic unified inspection and
explicit rechecks in the upper-right Updates window. There is no automatic helper installation. Explicit
authenticated setup is described above; it preserves custom policies. Existing
manual and normal AppImage installation paths remain available.

## Authenticated helper contract

`internal/udevauth` adds separate Ed25519 artifacts without changing the legacy
AppImage manifest. `NewPolicy(version)` takes bytes only from the canonical embed
and history only from `KnownOfficialDigests()`, which returns an independent,
sorted copy of checked-in official historical digests plus the canonical digest.
There is no signing API accepting renderer-selected payload or history.
`NewScriptManifest(version, finalScript)` hashes the final key-pinned script.
Both return signing inputs with `SignedBytes()` and `Marshal(signature)`.
`VerifyPolicy(wire, expectedVersion, publicKey)` returns authenticated policy bytes
and recognized previous hashes; `VerifyScript(wire, script, expectedVersion,
publicKey)` verifies the final script. Versions must be semantic versions and
match the expected release **exactly**, including any `v` prefix or metadata.
There is no newer-than requirement: same-version udev-only installation is valid.

The canonical UTF-8/ASCII-compatible wire format is newline-terminated, ordered,
with no extra fields or trailing data. The policy format is:

```text
attack-shark-udev-policy-v1
version=1.2.3
sha256=<64 lowercase hex characters>
previous=<1–16 sorted unique lowercase SHA-256 digests, comma-separated>
payload=<one-line canonical standard base64 of exact rule bytes>
signature=<one-line canonical standard base64 Ed25519 signature>
```

The signature covers all preceding lines, including their final newline.
The script manifest instead uses domain `attack-shark-udev-helper-v1`, followed
by `version`, `sha256`, and `signature` only. Envelope size is at most 96 KiB;
policy and final script sizes are at most 64 KiB. `previous` has no spaces,
empty entries, duplicates or trailing comma; sorting is bytewise ascending.
`MaxPreviousDigests` is 16. Verifiers accept bounded canonical history only under
the release signature, not through a frozen local digest allowlist. Unsigned
changes to that list invalidate the whole envelope.

The release signing history retains the fa60-only snapshot digest
`d7e14b66e339ab394c00c1881759443068001245ed163f7a49e511b264ee32f6`
and the initial fa60+fa61 policy digest
`c2862bc932a3454f16f2759510d23dc408b1ce9f015b1dce576cb4f58d2274be`.
The current canonical digest is included automatically (and deduplicated).
**Release maintenance:** before changing the canonical policy again, retain its
outgoing official digest in `KnownOfficialDigests()`; keep the registry bounded,
checked in, and sourced only from official release snapshots/digests. Never
construct history from runtime git, an installed custom file, or renderer input.
No duplicate current raw-policy file is introduced. Adding a new official digest
does not require replacing an already trusted root helper.

### Read-only remote policy checks

`udevauth.CheckPolicy(ctx, transport, manifest, currentVersion, releaseKey)` is
available for backend-selected release manifests. It authenticates the unchanged
AppImage manifest, requires the installed channel and a non-older version, and
fetches only `udev-policy.txt` from the canonical repository and exact release tag.
Same-version checks support USB-only updates. Requests have a 15-second deadline;
the envelope fetch is bounded to 96 KiB and the existing policy verifier validates
its exact version, signature, digest, grammar and signed history. A nil policy
with no error means the artifact is unavailable (HTTP 404), including old releases;
other HTTP, read, size and authentication failures return errors, not support.
This check never applies an AppImage, writes rules, installs a helper or elevates.
Manifest, version and key selection must remain backend-owned, not renderer input.
The existing AppImage check/apply APIs and legacy releases are unchanged.

For remote candidates, the desktop coordinator compares installed bytes to
`VerifyPolicy`'s returned `Policy.Bytes`, or their SHA-256 against its authenticated
`Policy.Previous`. An unrecognized installed policy stays custom. A T1 `current`
result refers only to the local embedded policy: it must not hide an update to a
newer authenticated remote policy. `KnownOfficialDigests()` is a signing source,
not a substitute for verifying remote history.

### Desktop coordinator contract (backend only)

Trusted Go composition calls `ConfigureUnifiedUpdates` after `ConfigureUpdater`,
with a read-only installed source and `PolicyUpdateBackend`. Its `Check` must
return only authenticated T3a candidates. Its `Apply` must independently validate
the signed payload and installed policy under the installed helper lock; the
coordinator's pre-apply inspection is not a replacement for that security check.
Production startup calls the trusted-Go `ConfigureAuthenticatedPolicyUpdates(service)`
after the final `ConfigureUpdater` and before Wails service registration. It pairs
the updater's exact concrete GitHub transport with `InstalledSource()` and
`ApplyInstalledPolicy`; none of these capabilities are Service methods.

An available AppImage uses its `VerifiedUpdate.Manifest()` for policy authentication,
without selecting a second release. Only the no-newer sentinel permits a
`CheckPolicyRelease` lookup for same-version policy availability. Failed app
selection/authentication cannot independently authorize policy. Each recheck clears
previous policy/wire authority. Apply compares the complete detached candidate,
including history, revalidates the retained signature and passes the exact signed
wire to the helper. Legacy AppImage checks do not fetch policy artifacts.

`CheckUnifiedUpdates` returns display-only AppImage and policy availability and
individual errors; it never authorizes or applies. `ApplyUnifiedUpdate` is the
single explicit approval action, with no renderer-selected candidate or inputs.
Legacy check/apply calls invalidate that approval. All operations serialize on
the existing AppImage state lock and use the existing operation deadlines.
Current, custom, unavailable and failed inspections do not invoke policy apply.

Policy and AppImage writes are not atomic. The returned result distinguishes
AppImage replacement, policy installation, policy reload/trigger errors and
relaunch errors. Retries never repeat completed replacement or policy writes.
A policy partial failure defers relaunch on the first action; a subsequent
explicit action can relaunch without repeating the write, retaining the policy
error for display. A check of the same candidate also retains that partial result.
App and relaunch failures also survive rechecks of the same target until the
relevant explicit retry succeeds. Cancellation is checked after acquiring the
shared lock and between unified callbacks: a canceled apply cannot start a later
policy write, AppImage replacement or relaunch. If cancellation follows a write,
`ApplyUnifiedUpdate` returns both its individual result and the context error;
callers must not discard that result. This does not change legacy updater errors
or relaunch behavior.
The UI must display these individual outcomes rather than claim combined success.
AppImage replacement remains user-local. The upper-right Updates window displays independent
app, policy and relaunch errors. A policy write is not proof of successful reload,
and AppImage replacement is not proof of relaunch. Only missing/outdated official
policies are eligible; current, custom, error and unavailable states never solicit
a policy overwrite. Merely constructing the backend does not prove a graphical
authorization agent is available. For read-only user-led validation, inspect any
startup notice, expand Update details, and use Check for updates without approving Apply updates or
running setup. Any host-changing validation requires separate explicit approval.

### Fixed desktop executor (trusted Go only)

`ApplyInstalledPolicy(ctx, authenticatedPolicy, exactSignedWire)` accepts only
backend-authenticated candidates and bounded signed envelopes. It is not exposed
on the Wails Service and does not select release keys. On Linux it validates
root ownership, non-writable ancestors and regular executable leaves, then runs
exactly `/usr/bin/pkexec --disable-internal-agent
/usr/local/libexec/attack-shark-linux-udev-helper VERSION`, with the signed wire
on stdin. A system graphical polkit agent must be available; there is no internal
terminal password agent, sudo fallback, password collection or automatic retry.
Missing/unsafe components require trusted `--setup-usb-permissions` setup and a
distro-provided pkexec/polkit authentication agent; the app installs neither.

There is at most one authorization invocation per call, a two-minute deadline,
and 4 KiB retained per output stream. Child output is drained with bounded
storage; inherited output pipes have a one-second wait bound after process exit.
`current` succeeds without claiming a write; `installed` records a completed
write even when reload/trigger fails, cancellation follows, or output overflows.
Cancellation cannot prove nothing changed if the child died before reporting its
write. Callers must preserve returned results alongside errors and inspect an
uncertain outcome before any explicit retry. Non-Linux calls are unavailable
without process or filesystem operations. The existing trusted-admin concurrency
assumption still applies; these checks do not protect against arbitrary root
writers replacing the executable.

`packaging/udev-helper/helper.sh` is an **unconfigured release template**, not a
bootstrap installer. Its public-key placeholder fails closed. The release
stage pins the release public key and signs the final script digest. The trusted
bootstrap authenticates that script before installation and explicitly installs it root-owned at
`/usr/local/libexec/attack-shark-linux-udev-helper`. After authorization, the
script itself securely recreates only its fixed `/run/attack-shark-linux`
directory (root-owned 0700) and regular `helper.lock` (root-owned 0600) when absent,
including after reboot. Concurrent creators validate the resulting objects;
unsafe pre-existing objects are rejected, never repaired or followed. No boot
service or tmpfiles configuration is required. Explicit bootstrap is available
through the release installer; the combined desktop update UI uses only the
already-installed helper and links to this separate trusted setup guidance.
Do not execute the repository template with administrator privileges.

### Release staging

`release stage -helper-template packaging/udev-helper/helper.sh` adds three
artifacts alongside the unchanged AppImage outputs: `attack-shark-linux-udev-helper`,
`udev-helper-manifest.txt`, and `udev-policy.txt`. Both release Task targets use
this option; omitting it preserves the old stage invocation. The template must
contain exactly one key placeholder. `BuildAuthArtifacts` validates and signs
all authentication artifacts in memory before staging, using the existing seed
and an x509/PEM Ed25519 public key. Invalid key, tag, or template inputs do not
change staged artifacts. Filesystem write failures can still leave incomplete
staging; never publish a failed stage.

Future transport must fetch these fixed assets from an already verified expected
release/tag and verify their exact version (tag without `v`), not trust an unsigned
URL appended to the AppImage manifest. Older five-asset releases remain supported
by the existing installer and AppImage updater. Normal installation accepts either
exactly the five legacy asset names or all eight names (the legacy set plus the
three authentication assets), with unique names and canonical release URLs.
Incomplete, duplicated, unexpected, or ambiguous asset sets are rejected. Normal
installation still downloads only the signed AppImage manifest and AppImage;
it does not download, install, or execute the helper or signed policy. The existing
`--install-udev` manual-rule opt-in remains separate and does not bootstrap a
trusted helper. Do not execute a staged helper as root before trusted
bootstrap authentication and explicit authorization.

The future installed command has one interface:

```text
attack-shark-linux-udev-helper VERSION < signed-policy
attack-shark-linux-udev-helper --help
```

There are no destination, key, utility, environment-root, or command overrides.
The only writable policy destination is
`/etc/udev/rules.d/60-attack-shark-x6-hidraw.rules`. The helper requires trusted
root-owned ancestors without group/world write permission, a safe installed
script and lock, and root execution. It waits at most five seconds for its fixed
lock and at most five seconds for bounded stdin. It independently verifies the
signature with distro OpenSSL before policy installation. Allowed directives are
only the canonical X6 hidraw/uaccess/0660 grammar for vendor `1d57` and products
`fa60`/`fa61`, plus comments and empty lines; RUN, PROGRAM, OWNER, static groups
and arbitrary directives are refused even with a valid signature.

Private runtime directory/lock creation may precede rejection of an invalid
payload. These fixed metadata-only changes are distinct from managed-rule writes
or udev effects: rejection never authorizes either. With runtime objects already
present, rejected input leaves the fixture filesystem entirely unchanged.

Under the helper lock, existing content must exactly match the authenticated
payload or one of its signed previous official digests. Symlink/nonregular, unsafe,
oversized, custom and static-group files are preserved. An identical rule prints
`current` (exit 0), without replacement or udev calls. The later desktop
coordinator must use T1 inspection to skip elevation entirely when current.
A changed rule is staged beside the destination, set to 0644, rechecked and
atomically renamed. It prints `installed`; reload is followed only by USB and
hidraw events under resolved matching USB device parents, never a global trigger.
Each udev command has a ten-second timeout. Replugging may still be necessary.

Success exits 0; invalid invocation exits 2; validation/installation failures
exit 1. After replacement, reload failure exits 3 with
`error: installed-but-reload-failed`; trigger failure exits 4 with
`error: installed-but-trigger-failed`. Both retain stdout `installed`: these are
partial successes, not claims that nothing changed. No persistent service,
password input, sudo cache, Go root helper or root AppImage execution is involved.

**Concurrency assumption:** the fixed root-owned lock serializes helper
invocations. Independent administrator/root tools must not edit the managed rule
concurrently. Existing custom policy is checked before staging and immediately
before rename, but this is not compare-and-swap protection against arbitrary
concurrent root writers. Root actors able to replace trusted directories or the
helper are outside the unprivileged-attacker threat model.

## Permission-denied recovery

The application maps `os.ErrPermission` to the UI error code
`permission_denied`. This means Linux denied access to the validated dongle; it
does not mean the app should be run as root. Check, in order:

- The rule is installed at `/etc/udev/rules.d/60-attack-shark-x6-hidraw.rules`.
- Rules were reloaded, the trigger command ran, and the dongle was replugged.
- For group policy, the user belongs to `attack-shark-x6` and has logged in again.
- The USB identity still appears as `1d57:fa60` (dongle) or `1d57:fa61` (cable).

## Cable and dongle connected together

One validated X6 cable takes priority when exactly one eligible cable and one
eligible dongle are present. Status, listening, and automatic configuration
selection use the cable; a cable read failure does not fall through to dongle
status. An invalid cable descriptor does not hide a valid dongle. The two PIDs
retain separate saved identities; this preference does not prove they are the
same physical mouse. Multiple cables or receivers are not automatically merged.

## Troubleshooting

When launching the app from a terminal, `x6_inventory event=status_read`
records passive status-read outcomes without logging report contents. An
`operation=open` error with `error_kind="permission denied"` identifies a node
access failure. An `operation=read` error with `error_kind="timeout"` means the
node opened but the status operation did not finish before its deadline;
`reports=0` means no nonempty reports reached its callback. Descriptor validation
alone does not establish successful node access or a fresh battery reading.

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
