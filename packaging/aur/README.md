# AUR packages

| Directory | AUR package | Source |
| --- | --- | --- |
| `attack-shark-linux/` | `attack-shark-linux` | Latest stable tag tarball |
| `attack-shark-linux-git/` | `attack-shark-linux-git` | `main` branch |

Both build with `-X .../internal/update.PackageManager=pacman`, which disables
the in-app updater. Pacman owns the files, so the app must never replace itself.

## Bump the release package

After publishing a stable tag `vX.Y.Z` (never an `-rc.N` tag):

```sh
cd packaging/aur/attack-shark-linux
sed -i 's/^pkgver=.*/pkgver=X.Y.Z/; s/^pkgrel=.*/pkgrel=1/' PKGBUILD
updpkgsums
makepkg -f                      # local build check
makepkg --printsrcinfo > .SRCINFO
```

## Publish to the AUR

Each package is its own AUR Git repository. Copy `PKGBUILD` and `.SRCINFO`
into it, commit and push:

```sh
git clone ssh://aur@aur.archlinux.org/attack-shark-linux.git
cp packaging/aur/attack-shark-linux/{PKGBUILD,.SRCINFO} attack-shark-linux/
```

The `-git` package only needs a push when its `PKGBUILD` changes; its
`pkgver()` follows `main` on every build.
