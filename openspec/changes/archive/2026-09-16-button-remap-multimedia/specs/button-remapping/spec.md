# Delta Specification: Multimedia Button Remapping

## MODIFIED Requirements

### Requirement: Remap action catalog remains closed and ordered

The system MUST retain every existing Basic remap action in its existing order and MUST add exactly the following Multimedia actions in this product order:

1. Media Player — `media_player` — `0x15`
2. Play/Pause — `play_pause` — `0x18`
3. Stop — `stop` — `0x19`
4. Previous Track — `previous_track` — `0x16`
5. Next Track — `next_track` — `0x17`
6. Volume Up — `volume_up` — `0x1b`
7. Volume Down — `volume_down` — `0x1c`
8. Mute — `mute` — `0x1a`

The catalog order MUST NOT be derived from numeric wire-ID order. Browser/system actions, keyboard shortcuts, macros, and unknown values MUST remain unavailable and fail closed.

#### Scenario: Eligible action encodes its exact ID

- **GIVEN** a valid Multimedia action is selected for an eligible button
- **WHEN** the complete remap is encoded
- **THEN** the selected group contains that action's exact wire ID
- **AND** no product-order sorting changes the action

#### Scenario: Excluded values fail closed

- **GIVEN** a configuration includes shortcut `0x11`, browser/system `0x1d` through `0x26`, a macro, or an unknown action
- **WHEN** it is validated
- **THEN** validation rejects it before report encoding or device I/O

### Requirement: Selectors visibly separate Basic and Multimedia actions

Every one of the seven remap selectors MUST visibly expose a Basic group and a Multimedia group. The groups MUST preserve the catalog order. Existing labels, staging, apply, discard, keyboard behavior, and readiness behavior MUST remain understandable and unchanged except for the added grouping and disabled choices.

#### Scenario: All selectors show both groups

- **GIVEN** the remap panel is rendered
- **WHEN** a physical-button selector is opened
- **THEN** it exposes Basic and Multimedia groups
- **AND** Multimedia contains exactly the eight ordered actions
- **AND** no excluded category is rendered

### Requirement: Multimedia assignments are restricted by physical button

Buttons 2–7 MUST accept every Multimedia action. Button 1 MUST retain its existing assignment and MUST NOT accept a Multimedia action. The Button 1 selector MUST keep its Multimedia group visible but every Multimedia entry MUST be disabled. Authoritative backend validation MUST reject a bypassed Button 1 Multimedia assignment before report encoding, pending/applied-state advance, persistence, transport, or device I/O.

#### Scenario: Eligible buttons accept Multimedia actions

- **GIVEN** any Button 2–7 and any of the eight Multimedia actions
- **WHEN** the action is staged and explicitly applied
- **THEN** the action is accepted and uses its exact wire ID

#### Scenario: Button 1 exposes but disables Multimedia

- **GIVEN** the Button 1 selector is rendered
- **WHEN** its options are inspected
- **THEN** its Multimedia group is visible
- **AND** all eight entries are visibly disabled with accessible disabled semantics
- **AND** pointer and keyboard interaction cannot stage one

#### Scenario: Backend blocks a Button 1 bypass

- **GIVEN** a direct request assigns a Multimedia action to Button 1
- **WHEN** validation runs
- **THEN** the configuration is rejected before encoding and I/O
- **AND** pending/applied configuration, revision, and persistence remain unchanged

### Requirement: Existing remap report and acknowledgement contracts remain unchanged

`RemapConfig` MUST continue to contain exactly seven ordered physical buttons. Application-to-wire order MUST remain `[1, 2, 3, 7, 8, 5, 6]`. Reports MUST remain 59 bytes with header `08 3b 01`, eighteen three-byte groups, `00 00` simple-action parameters, and a big-endian additive checksum over bytes `[3:57]` in bytes `[57:59]`. Only `03 10 50 00 08` MUST be accepted as the acknowledgement.

Adding Multimedia MAY change only the selected action byte and resulting checksum. It MUST NOT change report shape, hidden groups, parameters, mapping, checksum range/order, or ACK contract.

#### Scenario: Multimedia preserves report shape

- **GIVEN** an eligible Multimedia assignment
- **WHEN** its report is encoded
- **THEN** its length, header, group count, hidden groups, and parameter bytes remain unchanged
- **AND** its non-linear target offset and checksum are correct

#### Scenario: Basic remaps retain behavior

- **GIVEN** an existing Basic configuration
- **WHEN** it is encoded after this change
- **THEN** its report and ACK behavior are byte-for-byte unchanged

### Requirement: Explicit apply and recovery lifecycle remain unchanged

Staging a valid Multimedia action MUST cause no device write. Only explicit `ApplyRemap` MAY authorize the existing bounded write. Applied state MUST advance only after the exact ACK. Existing persistence retry MUST not repeat hardware I/O. Discard MUST abandon the local draft without I/O. Factory reset MUST continue to restore the existing Basic default remap.

#### Scenario: Staging is side-effect free

- **GIVEN** a user stages a valid Multimedia action on Buttons 2–7
- **WHEN** Apply is not invoked
- **THEN** no report is sent and no applied or persisted state changes

#### Scenario: Apply remains ACK-gated

- **GIVEN** a valid staged Multimedia remap
- **WHEN** Apply is explicitly invoked
- **THEN** applied state advances only after the exact ACK
- **AND** a persistence retry does not send another report

#### Scenario: Reset remains Basic

- **GIVEN** factory reset is invoked
- **WHEN** it completes
- **THEN** it restores the existing Basic default remap
- **AND** it introduces no Multimedia mapping
