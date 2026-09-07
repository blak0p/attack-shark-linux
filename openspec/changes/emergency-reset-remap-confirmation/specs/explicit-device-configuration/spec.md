# Explicit Device Configuration Specification

## Purpose

Define a safe configuration boundary in which editing is local and every physical X6 write is an explicit, selected-device, acknowledgement-gated action.

## Requirements

### Requirement: Stage locally and require explicit confirmation

The system MUST keep DPI, polling, and remap edits in pending state until the user explicitly confirms the corresponding apply action. Staging, debounce expiry, scheduler advancement, reconnect, and automatic retry MUST NOT write to hardware.

#### Scenario: Staging has no hardware side effect

- GIVEN a selected device and a staged DPI or polling change
- WHEN the scheduler advances past every pending deadline
- THEN no device command is sent and applied and persisted state is unchanged

#### Scenario: Confirmation authorizes one configuration write

- GIVEN a valid complete remap draft for the selected device
- WHEN the user explicitly confirms Apply remap
- THEN the system sends the remap command only to that selected binding

### Requirement: Validate and target the selected binding

The system MUST reject an invalid configuration or missing, stale, or unvalidated device binding before any physical write. A write MUST use the binding selected by the user, not an implicitly discovered device.

#### Scenario: Invalid draft is rejected safely

- GIVEN a remap draft with an unsupported action or incomplete required buttons
- WHEN the user confirms Apply remap
- THEN validation fails, no command is sent, and pending, applied, and persisted state are preserved

#### Scenario: Selection is required

- GIVEN no current validated device selection
- WHEN the user confirms any configuration apply
- THEN the operation returns a selection error and performs no write

### Requirement: Gate state and persistence on acknowledgement

The system MUST advance applied state only after the selected command receives its valid ACK. It MUST persist only acknowledged state; on command or persistence failure it MUST preserve the last acknowledged recovery state and expose whether retry is available.

#### Scenario: Failed ACK does not advance state

- GIVEN a valid confirmed configuration and a selected binding
- WHEN the command fails or its ACK times out
- THEN applied and persisted state remain at the previous acknowledged values and pending state remains recoverable

#### Scenario: Acknowledged configuration is persisted

- GIVEN a valid confirmed configuration whose command receives its ACK
- WHEN state reconciliation completes
- THEN applied state advances and the acknowledged configuration is persisted for that device

#### Scenario: Persistence failure remains recoverable

- GIVEN the device ACK succeeded but local persistence failed
- WHEN the operation reports failure
- THEN acknowledged applied state is not replaced by unacknowledged data and an explicit retry can attempt persistence without another automatic device write

### Requirement: Cancellation and restart are write-safe

The system MUST cancel pending configuration work when the user cancels, resets, disconnects, or restarts the service. Late callbacks from cancelled work MUST NOT write or overwrite newer state; a later explicit confirmation MAY start a fresh operation from the current selected binding.

#### Scenario: Cancelled work cannot write later

- GIVEN a staged configuration with a pending scheduled callback
- WHEN the user cancels it and then the callback executes
- THEN no command is sent and no applied or persisted state changes

#### Scenario: Restart preserves recovery state

- GIVEN a prior acknowledged configuration and no successful replacement
- WHEN the service restarts and the device is selected again
- THEN the last acknowledged state is restored and no write occurs until a new explicit confirmation
