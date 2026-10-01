# Refactor integration + release v1.3.0

## Objective and rationale
Integrate the completed desktop ownership refactor (2dc5b2e, 18 files, +1965/-938) onto current main (now includes jsdom 30.1.1 merge), plus upload-artifact v7 (#129), in one clean PR with approved size:exception. Then push to origin, pre-release and release v1.3.0, closing #130 and #129.

## Authorized scope and exclusions
- Source canonica: 2dc5b2e limpio, sin re-extraer las 9 unidades.
- Destino: origin, version v1.3.0 (pre-release + release). PR unico con size:exception aprobado.
- Issues: #130 (refactor), #129 (upload-artifact v7). #128 ya cerrado por merge dependabot (jsdom) — no tocar.
- Exclusiones: no re-hacer extraccion, no split en chained PRs, no tags extra, no cambio de API publica desktop, retener catalogo Browser 30 acciones y toolchain Wails beta.26.
- Rama de trabajo: crear `feat/refactor-integration-v1-3-0` desde origin/main actualizado. No trabajar sobre feat/browser-button-remapping.

## Verification configuration and delivery
- TDD: integracion de codigo ya verificado — no se exige RED previo; se corre verificacion completa observada (go test, frontend test/build, vet, diff-check).
- Comandos base: `go test ./...`, `cd frontend && npm ci && npm test`, `cd frontend && npm ci && npm run build && cd .. && go vet ./...`, `git diff --check`.
- Strategy: single-pr + exception-ok (ya aprobado). Push, pre-release y release a origin autorizados en v1.3.0.

## Tasks
- [ ] RI-1 — Sincronizar base: fetch origin/main (trae merge jsdom), crear rama trabajo, constatar diff main..2dc5b2e y estado #129 (upload-artifact v7 pendiente). Route: delegated mapper (multi-file). Checks: base HEAD + diffstat observados.
- [ ] RI-2 — Integrar refactor 2dc5b2e limpio + upload-artifact v7 sobre main, reteniendo Browser y beta.26. Route: delegated writer (18 files). Checks: comandos verificacion observados, diff-check limpio.
- [ ] RI-3 — Push rama a origin + PR unico con size:exception, CI/review ordinaria, merge. Route: delegated direct (remote autorizado). Checks: PR mergeado, CI verde.
- [ ] RI-4 — Pre-release v1.3.0-rc + release v1.3.0, cierre #130 y #129 con evidencia. Route: delegated direct. Checks: releases publicados, issues cerrados.

## Progress and evidence
- 2026-10-01: Fuente 2dc5b2e confirmada por usuario; destino origin + v1.3.0 confirmado; #128 cerrado por merge dependabot, quedan #130 y #129.
- 2026-10-01 (RI-1 inline): fetch origin listo. Nueva base origin/main=e7611cc ya trae #120 (jsdom), #122 (upload-artifact v7 confirmado en workflow), #116 y #126 (beta.26). Rama `feat/refactor-integration-v1-3-0` creada desde origin/main. Diff real origin/main..2dc5b2e = 47 archivos (+2057/-1373): la base avanzo mucho (Browser, beta26, packaging), asi que RI-2 integra con conflictos reales en remap/Browser. #129 queda solo para cierre como completado, sin codigo extra.

## Next step
RI-2 writer unico (dispara writer-trigger: 47 archivos): integrar 2dc5b2e sobre e7611cc reteniendo Browser 30 acciones + beta.26 + artifact v7.
