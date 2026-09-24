# Security Policy

## Supported Versions

| Version | Supported |
| ------- | --------- |
| main (unreleased) | :warning: Beta — development branch; remote release gates remain |

Security fixes land on `main` during development. Signed RC prereleases use
`vMAJOR.MINOR.PATCH-rc.N` tags and remain prereleases; stable publication as
`v1.2.0` is a separate remote gate and is not claimed by this document.

## Reporting a Vulnerability

If you discover a security vulnerability, please report it responsibly:

1. **Do NOT** open a public GitHub issue.
2. Send a private report via [GitHub Security Advisories](https://github.com/blak0p/attack-shark-linux/security/advisories).
3. Or contact the maintainer directly.

Please include:

- Description of the vulnerability
- Steps to reproduce (device model, dongle firmware, OS/distro)
- Potential impact
- Suggested fix (if any)

We aim to acknowledge reports within 72 hours and to respond with a triage
assessment as soon as possible.

## Scope

This project talks to real hardware over hidraw. The following are security
relevant and in scope:

- **hidraw device validation** — the app must only open nodes that match the
  validated X6 dongle (`1d57:fa60`, expected interface/endpoint, physical USB
  path, report descriptor). A matching VID/PID alone is not sufficient.
- **udev policy** — `packaging/udev/` must keep devices non-world-writable
  (`0660`) and grant access only to the active local seat (`uaccess`). Never
  run the app as root; never change the rules to `0666`.
- **Report payloads** — configuration writes (`SET_REPORT`) must be built from
  validated inputs; no unchecked user data should reach the device.
- **Credentials and secrets** — nothing sensitive may ever be committed.
- **Release and update verification** — stable-default and opt-in RC
  installation and AppImage updates require signed release metadata; updates also require explicit user approval
  and are confined to the user-local AppImage. The updater does not install or
  modify udev rules.

## Security Best Practices

When contributing:

- Never commit sensitive data (keys, tokens, credentials, capture material from
  the proprietary app).
- Run `go vet` and the test suite before opening a PR.
- Validate all user inputs before they become report bytes.
- Follow the principle of least privilege for device access.

## Known Limitations

- The app is pre-release; behavior may change without notice.
- The stable-default installer (with `--beta` for signed RCs) and x86_64
  AppImage are release artifacts; no stable candidate means normal installation
  fails rather than selecting an RC. Live
  installer/update rehearsal evidence and stable publication are separately
  gated and are not claimed here. Updates remain within their installed channel.
- Macros, on-device profiles, and DEB/RPM/AUR packages or repositories are
  deferred. Local per-device persistence is distinct from on-device profiles.