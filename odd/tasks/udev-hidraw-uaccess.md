# Udev Hidraw Uaccess Fix

Repository-relative file locator: `odd/tasks/udev-hidraw-uaccess.md`

## Objective

Correct the packaged X6 Linux access policy so systemd processes `uaccess` before its late-seat rule and grants access to the hidraw nodes that the desktop application actually opens.

## Problem and evidence

The packaged file is named `99-attack-shark-x6.rules` and matches `SUBSYSTEM=="usb"`. On this host, `/usr/lib/udev/rules.d/73-seat-late.rules:16` queues its uaccess helper only when the tag already exists. Collective lexical evaluation therefore processes `73-...` before `99-...`, so the late tag cannot satisfy that earlier condition. The application uses `/dev/hidraw*`, not the USB device node.

The issue #77 reporter observed the ordering failure on CachyOS with Hyprland. Read-only host inspection confirmed an X6 USB device and four X6 hidraw nodes. The host's local policy uses separate USB and hidraw rules, but it is customized with `OWNER="alejandro"`; it is not a suitable shipped policy or runtime proof.

## Scope

- Remove the superseded USB-only `99-attack-shark-x6.rules` policy and rename the existing hidraw-specific `99-attack-shark-x6-hidraw.rules` policy to `60-attack-shark-x6-hidraw.rules`, preserving its X6 parent matching, `TAG+="uaccess"`, and `MODE="0660"` contract.
- Update installation, group-policy alternative, recovery, and troubleshooting documentation to use the new path and accurately state the hidraw target.
- Run a maintainer-authorized, reversible live end-to-end acceptance before repository policy edits; prove the candidate grants the active user ACL access to the X6 hidraw nodes and permits non-mutating desktop reads.
- Post one concise clarification to issue #77: the `99 → 60` ordering diagnosis is valid, and the delivered policy must also match hidraw.

## Constraints and non-goals

- Preserve least privilege: do not use `0666`, root execution, broad USB access, or user-specific `OWNER=` policy.
- Do not reload rules, trigger events, replug hardware, modify `/etc` or `/run` udev rules, or run a live udev simulation **unless the maintainer explicitly authorizes the reversible UDEV-2 acceptance procedure**.
- Do not change HID transport, device discovery, Wails bindings, or remapping behavior.
- The live acceptance procedure must use a volatile `/run` candidate and exact rollback. It must temporarily neutralize the host's existing X6-specific `OWNER=` rules, reload udev, require a physical replug, verify hidraw ACLs and non-mutating desktop reads, then restore the original rules and access state even on failure.
- A repository rule contract can be validated statically; live ACL confirmation is separate maintainer-operated evidence and must not be claimed until observed.

## Delivery strategy

- Forecast: approximately 80–140 authored changed lines, including documentation and static contract checks.
- Strategy: `ask-on-risk`; the forecast is below the 400-line delivery heuristic.
- Effective TDD mode: enabled by explicit maintainer instruction. Runner: `go test ./packaging/udev -run '^TestPackagedUdevPolicyContract$' -count=1`. The new adjacent test `packaging/udev/udev_policy_tdd_test.go` must produce observed RED evidence before production policy/documentation edits, then observed GREEN evidence afterward.

## Tasks

- [x] **UDEV-1 — Establish an isolated feature workspace from `origin/main`.** Created branch `fix/udev-hidraw-uaccess` at `e4ea092d8bc71f56b852f4fada310bc82404a24e`; worktree is clean. Evidence: `git worktree add -b fix/udev-hidraw-uaccess … origin/main`. Commit: pending explicit user authorization.
- [x] **UDEV-2 — Run the reversible live end-to-end acceptance.** PASS. The volatile `60-attack-shark-x6-hidraw.rules` candidate produced `uaccess` tags and `user:alejandro:rw-` ACLs on all four X6 hidraw nodes after physical replug. The desktop Device view read interface `dongle` and battery `100%` without applying any device change. Terminal closure interrupted automatic cleanup, but exact manual restoration was then verified read-only: both root-owned baseline rules exist, the candidate and backup are absent, and all four hidraw nodes are `alejandro:root` mode `0660` with active-user read/write access.
- [x] **UDEV-3 — RED, GREEN, and REFACTOR the packaged policy and documentation.** RED observed because the new policy path was absent. GREEN observed after explicit deletion authorization: removed both obsolete `99-...` policies, renamed the correct hidraw policy to `60-attack-shark-x6-hidraw.rules`, and updated the documentation. Focused test passed; `gofmt` completed; `git diff --check` passed.
- [x] **UDEV-4 — Verify delivery evidence and clarify issue #77.** Static contract and diff checks passed. Posted and read back the corrective issue comment: https://github.com/blak0p/attack-shark-linux/issues/77#issuecomment-5717223060. It records both the ordering and hidraw-target requirements and the live acceptance evidence.
- [x] **UDEV-5 — Create the work-unit commit.** Committed as `a516ba24f1635ac54b0ada4957fab58616d1badd` with `fix(udev): grant uaccess to X6 hidraw nodes`. Native assessment was unavailable/schema-incompatible, so independent verification ran and passed the focused policy test and committed-range diff check.

## Acceptance criteria and checks

- The installed filename is lexically earlier than `73-seat-late.rules`.
- The packaged rule matches `SUBSYSTEM=="hidraw"`, constrains the X6 VID/PID via parent attributes, has `TAG+="uaccess"`, and retains `MODE="0660"`.
- The packaged and documented filenames agree; no `99-attack-shark-x6.rules` reference remains.
- No broad USB-only rule, `OWNER=`, `0666`, root instruction, runtime udev reload/trigger, hardware communication, or generated-file change is introduced.
- Static checks and diff inspection are observed. Live ACL behavior is explicitly reported as maintainer verification, not automation evidence.

## Current progress and next step

TDD RED → GREEN → REFACTOR is complete. The live E2E acceptance passed, baseline recovery was verified, static checks passed, issue #77 was corrected, and work-unit commit `a516ba24f1635ac54b0ada4957fab58616d1badd` was independently verified. Source and tracking evidence commits are complete.
