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
- [ ] T2 (S3) Clean Arch container verification via `packaging/aur/check.sh`: dependency resolution, namcap, install, runtime deps, updater off; inline + independent verify
- [ ] T3 (S4) UI shows the package-manager install in the Estado panel; inline; awaiting UI decision
- [ ] T4 (S5) PR CI job running `packaging/aur/check.sh` on every PR to main (existing test.yml already runs go build/vet/test, vitest, playwright); inline
- [ ] T5 Commit work units on `feat/aur-packaging`

## Log

- L1: "vale te explico lo que quiero es facil dos caminos para aur uno que compile main y otro la ultima release tambien el actualizador in app habra que quitarlo si esta compilado por aur ya que esta gestionado por el gestor de paquetes"
- L2: "y tambien ahora hay que hacer cosas de ui obvio que es una [Image#1] eso y tambien hay que verificar bien esto es algo que hay que medir bien y tambien ahora despues antes de publicar todo esto hay qeu hacer muchos test en pr todo para que sea fiable todo sabes y primero verifica bien porfa"
- L3: T1 evidence: both `makepkg -df` builds succeeded locally (1.8.0-1, 1.8.0.r1.g2bf7932-1); `go test ./...` green; `PackageManager.str` symbol present in ldflag build. Local Go is not pacman-managed, so dependency resolution was not exercised.
- L4: "es que claro hay que asegurarnos que el main sea realmente estable sabes"
