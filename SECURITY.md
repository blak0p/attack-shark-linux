# Security Policy

## Supported Versions

| Version | Supported |
| ------- | --------- |
| main (unreleased) | :warning: Beta — no installer or tagged release yet |

Until the first release, security fixes land on `main` and are described in the
release notes of the first tagged version.

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

## Security Best Practices

When contributing:

- Never commit sensitive data (keys, tokens, credentials, capture material from
  the proprietary app).
- Run `go vet` and the test suite before opening a PR.
- Validate all user inputs before they become report bytes.
- Follow the principle of least privilege for device access.

## Known Limitations

- The app is pre-release; behavior may change without notice.
- No installer is shipped yet — see [README](README.md) for building from source.