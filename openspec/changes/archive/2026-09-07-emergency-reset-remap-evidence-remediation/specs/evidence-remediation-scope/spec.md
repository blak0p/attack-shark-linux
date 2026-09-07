# Evidence Remediation Scope Specification

## Purpose

Record the capability-neutral scope of this change. The predecessor reset and explicit-configuration requirements remain authoritative; this change only corrects their bounded compliance gap and supplies executable evidence.

## Requirements

### Requirement: Preserve the existing product capability scope

This change MUST NOT add, modify, remove, or rename product capability requirements. Implementation and verification work SHALL remain limited to the proposal's polling-selection contract correction, named runtime assertions, minimum generated contract propagation, and canonical evidence updates.

#### Scenario: Predecessor requirements remain authoritative

- GIVEN the predecessor reset and explicit-configuration requirements
- WHEN this remediation is implemented and verified
- THEN those requirements remain unchanged and no replacement product capability is introduced

#### Scenario: Remediation remains bounded

- GIVEN the proposal's named polling defect and evidence gaps
- WHEN implementation work is planned
- THEN protocol changes, hardware access, lighting reset, automatic retry, UI redesign, unrelated refactoring, and predecessor-artifact edits remain out of scope
