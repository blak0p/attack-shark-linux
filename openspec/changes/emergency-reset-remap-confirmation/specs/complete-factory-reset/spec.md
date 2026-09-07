# Complete Factory Reset Specification

## Purpose

Define one confirmed, failure-safe factory reset that restores all three non-lighting X6 configuration lanes through the selected device binding.

## Requirements

### Requirement: Reset requires explicit confirmation

`ResetToFactory` MUST perform no physical write when the user has not explicitly confirmed the reset, cancels it, or the service cannot establish exactly one eligible validated device. It MUST NOT use automatic staging or retry as confirmation.

#### Scenario: Unconfirmed reset is inert

- GIVEN a selected device and existing pending configuration
- WHEN reset is displayed, staged, or cancelled without explicit confirmation
- THEN no reset command is sent and persisted recovery state is unchanged

#### Scenario: Reset requires one eligible device

- GIVEN zero or multiple eligible devices, or no validated selection
- WHEN the user explicitly confirms factory reset
- THEN the reset fails before any lane write and persisted state is preserved

### Requirement: Execute three ordered, ACK-gated reset lanes

After confirmation, the system MUST quiesce pending configuration writes and apply exactly these lanes to the selected binding in order: documented factory DPI, 1000 Hz polling, and default remap. Each lane MUST receive its valid ACK before the next lane starts. Lighting MUST NOT be changed.

#### Scenario: Successful reset restores all lanes

- GIVEN one eligible selected device and a confirmed reset
- WHEN the reset runs to completion
- THEN DPI is acknowledged first, polling second, remap third, and no lighting command is sent

#### Scenario: A failed lane stops the sequence

- GIVEN the reset has acknowledged every lane before a failing lane
- WHEN that lane fails or its ACK times out
- THEN no later lane is attempted and the result identifies the failed lane

### Requirement: Preserve recovery state on every failure

The system MUST purge persisted X6 state only after all three lane ACKs succeed and cleanup succeeds. Any lane failure, binding failure, cancellation, or cleanup failure MUST leave persisted recovery state intact and MUST expose a retryable failure without claiming reset success.

#### Scenario: Failed ACK preserves state

- GIVEN persisted pre-reset state exists
- WHEN any reset lane fails before all three ACKs
- THEN persisted state is not purged and the partial lane result is reported as failure

#### Scenario: Cleanup failure preserves state

- GIVEN all three lane commands were acknowledged
- WHEN purging persisted state fails
- THEN reset reports cleanup failure and retains persisted recovery state

#### Scenario: Successful reset purges after acknowledgements

- GIVEN all three lanes have returned valid ACKs
- WHEN persistence cleanup succeeds
- THEN persisted X6 state is purged and the reset result reports success

### Requirement: Cancellation and restart are safe and repeatable

The system MUST invalidate scheduled writes and late callbacks before reset lanes begin. Cancellation or disconnect MUST stop further lanes without purging state; after service restart or reconnection, a new explicit confirmation MUST be able to start a fresh reset using the current validated binding.

#### Scenario: Late callback is harmless

- GIVEN pending DPI or polling work exists when reset is confirmed
- WHEN reset quiesces it and the old callback later fires
- THEN the callback sends no command and cannot overwrite reset state

#### Scenario: A cancelled reset can restart

- GIVEN a reset is cancelled or interrupted before cleanup
- WHEN the device is reselected after service restart and reset is explicitly confirmed again
- THEN a new ordered three-lane attempt begins and no prior attempt purges state
