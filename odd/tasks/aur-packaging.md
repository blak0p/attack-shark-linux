# AUR packaging

Two AUR packages (stable tag and `-git` from main) with the in-app updater disabled for
package-manager builds. Branch: `feat/aur-packaging` (from `main`).

## Specs

- S1: "dos caminos para aur uno que compile main y otro la ultima release"
- S2: "el actualizador in app habra que quitarlo si esta compilado por aur ya que esta gestionado por el gestor de paquetes"
- S3: "primero verifica bien porfa" / "hay que verificar bien esto es algo que hay que medir bien"
- S4: "ahora hay qeu hacer cosas de ui obvio que es una [Image#1] eso" (Image#1: Estado panel row "Versión de la aplicación 1.8.0")
- S5: "antes de publicar todo esto hay qeu hacer muchos test en pr todo para que sea fiable todo" / "es que claro hay que asegurarnos que el main sea realmente estable sabes"

## Tasks

- [x] T1 (S1, S2) PKGBUILDs + `update.PackageManager` ldflag kill switch + docs; inline; commit: d45f758
- [x] T2 (S3) Clean Arch container verification via `packaging/aur/check.sh` (commits 5416fc8, 5579b93): dependency resolution, namcap, install, runtime libraries, `--version` contract, removal; inline (assess plan: writer self-verification, no separate verifier)
- [x] T3 (S4) Estado panel: '1.8.0 · AUR' / 'main · <commit> · AUR' + Updates row 'Gestionadas por pacman'; `--version` CLI; inline; commit: 01f5072
- [x] T4 (S5) CI job `aur` in .github/workflows/test.yml runs `packaging/aur/check.sh` on every PR to main and push to main, next to the existing go/vitest/playwright jobs; committed once T2 passes locally; proven only by its first GitHub run; inline
- [x] T5 Commit work units on `feat/aur-packaging` (d45f758, 01f5072, 5416fc8, 5579b93, CI commit)

## Log

- L1: "vale te explico lo que quiero es facil dos caminos para aur uno que compile main y otro la ultima release tambien el actualizador in app habra que quitarlo si esta compilado por aur ya que esta gestionado por el gestor de paquetes"
- L2: "y tambien ahora hay que hacer cosas de ui obvio que es una [Image#1] eso y tambien hay que verificar bien esto es algo que hay que medir bien y tambien ahora despues antes de publicar todo esto hay qeu hacer muchos test en pr todo para que sea fiable todo sabes y primero verifica bien porfa"
- L3: T1 evidence: both `makepkg -df` builds succeeded locally (1.8.0-1, 1.8.0.r1.g2bf7932-1); `go test ./...` green; `PackageManager.str` symbol present in ldflag build. Local Go is not pacman-managed, so dependency resolution was not exercised.
- L4: "es que claro hay que asegurarnos que el main sea realmente estable sabes"
- L5: "y si es main ponemos main  y el commit y hash no?" -> S4 decision: Versión + fila de actualizaciones; -git shows branch and commit instead of the version.
- L6: Container run 1-2 found check.sh bugs (debug package picked, namcap errors not failing, stripped binary has no symbols, -trimpath hides -ldflags); contract now measured via installed `--version`.
- L7: Slice boundary: 628 authored lines over 3 commits (d45f758, 01f5072, 5416fc8) — deliver as one PR split by commit for review; CI commit follows.
- L8: "por cierto no se de aur quiero que me enseñes ... no se como se gestiona me puedes explicar" — explanation request, answered in chat.
- L9: "y por cierto tambien mas tarde antes de publicar release con esto y tal va a ver que hacer que las pr tengan mas test y tal ya lo vermos guardalo epro eso para mas adelante porque hay qeu hacer que sea seguro" — deferred, out of this feature; tracked in Engram `odd/aur-packaging/deferred-pr-hardening`.
- L10: Container run at 5416fc8: stable package OK (35s, SKIP --version: v1.8.0 predates it); -git failed namcap E: license must live in /usr/share/licenses/attack-shark-linux-git/.
- L11: Container run at 5579b93: "All AUR package checks passed"; stable 33s (SKIP --version, v1.8.0 predates it), -git 17s, `--version: attack-shark-linux main 5579b93 (AUR, pacman)`.
- L12: Native review review-a3f038ab0d8e685f (risk, reliability) approved and acknowledged. Non-blocking follow-ups: R1-mutable-refs (pin archlinux image/actions by digest), R1-persist-credentials (checkout persist-credentials: false), R3-aur-ci-known-red, R3-aur-ci-root-gitowner (builder user reading a root-owned checkout) — proven only by the first GitHub run.
