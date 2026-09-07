schema: gentle-ai.sdd-research/v1
revision: 1
status: blocked
change: emergency-reset-remap-confirmation
selected_request:
  questions:
    - "Source-backed evidence for explicit user confirmation before HID configuration writes."
    - "Source-backed evidence for Wails service/runtime test boundaries that prove reset cancellation and restart behavior."
  requested_source_classes:
    - documentation
    - open-web
admission:
  capability: gentle-ai.sdd-research-capability/v1
  outcome: denied
  declared_grants: []
  observed_exact_grants: []
  reason: "No runtime capability declaration was supplied; documentation and open-web cannot be admitted."
sources: []
validated_claims: []
contradictions: []
uncertainty:
  - "No requested source class was admitted, so no external source collection or claim validation occurred."
freshness: "Not assessed because no sources were admitted."
non_authoritative_product_choices: []
recovery:
  retained_selected_intent: true
  required_capability: "gentle-ai.sdd-research-capability/v1 with exact documentation and open-web grants."
  retry_behavior: "Re-enter the selected research request after capability admission; do not treat this blocked artifact as proposal-ready evidence."
