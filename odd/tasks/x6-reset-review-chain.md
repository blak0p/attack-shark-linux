# Captured X6 reset review chain

Issue: #149. Integration is draft; merge remains the maintainer's decision.

1. Captured protocol defaults and exact report vectors.
2. Four-lane backend reset, failure handling, reconciliation and serialization.
3. Six-snapshot UI refresh and selection-scoped reset ownership.

Review children against their immediate parent. Integrate the children back
through the chain before merging the tracker into main.

Existing ordinary stage-4 / 8-ms defaults remain unchanged.
Physical ACK-backed reset and the user's observed change were verified;
EEPROM erasure, onboard macro deletion and power-cycle persistence are not.
