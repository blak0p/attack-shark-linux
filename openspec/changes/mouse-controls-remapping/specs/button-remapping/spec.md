# Delta Specification: Mouse Controls Button Remapping

## MODIFIED Requirements

### Requirement: Remap action catalog remains closed and ordered

The system MUST retain every existing Basic and Multimedia action in its current catalog and order and MUST add exactly these five Mouse Controls actions in this order:

1. Scroll Up — `scroll_up` — `0x09`
2. Scroll Down — `scroll_down` — `0x0a`
3. DPI Cycle — `dpi_cycle` — `0x0d`
4. DPI+ — `dpi_plus` — `0x0e`
5. DPI− — `dpi_minus` — `0x0f`

The catalog MUST NOT derive display order from numeric wire IDs. Browser/system actions, shortcuts, Easy Aim, macros, and unknown values MUST remain excluded and fail closed.

#### Scenario: Mouse Controls catalog is exact

- **GIVEN** the remap catalog is requested
- **WHEN** its categories and entries are rendered
- **THEN** it contains one `Mouse Controls` category
- **AND** that category contains exactly the five entries above in the stated order
- **AND** each entry retains its stated wire ID

#### Scenario: Excluded values fail closed

- **GIVEN** a configuration contains a Browser/system action, shortcut, Easy Aim, macro, or unknown value
- **WHEN** authoritative validation runs
- **THEN** validation rejects the configuration before report encoding or device I/O

### Requirement: Selectors expose Mouse Controls without changing existing categories

Every one of the seven physical-button selectors MUST retain the existing Basic and Multimedia catalog/order and MUST visibly expose the new Mouse Controls category in the specified order. No selector may expose an eighth button or internal wire group 4 as a physical slot.

#### Scenario: All seven selectors show the new category

- **GIVEN** the remap panel is ready
- **WHEN** each physical-button selector is opened
- **THEN** it exposes Basic, Multimedia, and Mouse Controls
- **AND** Mouse Controls contains exactly the five ordered actions
- **AND** the existing categories remain unchanged

### Requirement: Mouse Controls assignments are restricted by physical button

Buttons 2–7 MUST accept all five Mouse Controls actions. Button 1 MUST retain its current assignment and MUST display all five Mouse Controls entries as visibly and accessibly disabled. Backend validation MUST reject a Button 1 Mouse Controls assignment before state mutation, encoding, persistence, transport, or I/O.

#### Scenario: Buttons 2–7 accept every Mouse Controls action

- **GIVEN** any physical Button 2–7 and any Mouse Controls action
- **WHEN** the action is staged and explicitly applied
- **THEN** staging succeeds without a device write
- **AND** apply uses the exact action ID for the selected action

#### Scenario: Button 1 entries are disabled

- **GIVEN** the Button 1 selector is rendered
- **WHEN** its Mouse Controls category is inspected
- **THEN** all five entries are visible and have disabled semantics
- **AND** pointer, Enter, and Space interaction cannot stage an entry

#### Scenario: Backend rejects a Button 1 bypass

- **GIVEN** a direct request assigns any Mouse Controls action to Button 1
- **WHEN** authoritative validation runs
- **THEN** it rejects the request before state mutation, report encoding, persistence, transport, or I/O
- **AND** pending, applied, revision, and persistence state remain unchanged

### Requirement: Existing report and acknowledgement contracts remain unchanged

`RemapConfig` MUST continue to contain exactly seven ordered physical buttons. The physical-to-wire mapping MUST remain `[1, 2, 3, 7, 8, 5, 6]`; wire group 4 remains an internal group and MUST NOT become an exposed button. Reports MUST remain 59 bytes with header `08 3b 01`, eighteen three-byte groups, unchanged parameters, and the big-endian additive checksum over bytes `[3:57]` stored at `[57:59]`. Only `03 10 50 00 08` MUST be accepted as the remap ACK.

#### Scenario: A Mouse Controls assignment preserves report shape

- **GIVEN** an eligible Button 2–7 has one Mouse Controls action
- **WHEN** the complete report is encoded
- **THEN** its length, header, group count, hidden groups, parameter bytes, checksum range, and ACK contract are unchanged
- **AND** only the selected action byte and resulting checksum differ as applicable

#### Scenario: Existing behavior is preserved

- **GIVEN** an existing Basic or Multimedia remap configuration
- **WHEN** it is encoded and applied
- **THEN** its report, ACK, binding, and lifecycle behavior remain unchanged

### Requirement: Explicit apply, persistence, discard, reset, and preserved markers remain unchanged

Staging MUST perform no device write. Only explicit `ApplyRemap` MAY authorize the existing bounded write, and applied state MUST advance only after the exact ACK. Persistence retry MUST NOT repeat hardware I/O. Discard MUST abandon the local draft without I/O. Factory reset MUST restore the existing Basic default remap. Buttons 6/7 preserved-default markers MUST clear only when a user stages an explicit replacement.

#### Scenario: Staging is side-effect free

- **GIVEN** a valid Mouse Controls action is staged on Button 2–7
- **WHEN** the user does not invoke Apply
- **THEN** no report, transport, persistence, or device I/O occurs

#### Scenario: Apply is ACK-gated and retry-safe

- **GIVEN** a valid staged Mouse Controls remap
- **WHEN** Apply is invoked
- **THEN** one existing report lifecycle is used
- **AND** applied state advances only after `03 10 50 00 08`
- **AND** persistence retry does not issue a second device write

#### Scenario: Reset and DPI markers retain established behavior

- **GIVEN** factory reset is invoked, or a user stages an explicit replacement on Button 6 or 7
- **WHEN** the operation completes
- **THEN** reset restores the existing Basic defaults
- **AND** only the explicitly replaced button's preserved-default marker is cleared
- **AND** no new button, wire group, or Mouse Controls reset default is introduced
