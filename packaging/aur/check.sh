#!/usr/bin/env bash
# Builds, lints, installs and removes both AUR packages in a clean Arch Linux
# container. Run as root inside archlinux:latest with the repository at $SRC:
#
#   podman run --rm -v "$PWD:/src:ro" docker.io/library/archlinux:latest \
#     bash /src/packaging/aur/check.sh
#
# The -git package is built from the commit checked out at $SRC (GIT_REF,
# default HEAD) instead of GitHub main, so a pull request tests its own code.
set -euo pipefail

SRC=${SRC:-/src}
GIT_REF=${GIT_REF:-HEAD}
BUILD=/build
PACKAGES=(attack-shark-linux attack-shark-linux-git)
FILES=(
  usr/bin/attack-shark-linux
  usr/lib/udev/rules.d/60-attack-shark-x6-hidraw.rules
  usr/share/applications/attack-shark-x6.desktop
  usr/share/icons/hicolor/scalable/apps/attack-shark-x6.svg
)

step() { printf '\n==> %s\n' "$*"; }
fail() { printf 'FAIL: %s\n' "$*" >&2; exit 1; }

step "Install build tooling"
pacman -Syu --noconfirm --needed base-devel git namcap desktop-file-utils >/dev/null
useradd -m builder
echo 'builder ALL=(ALL) NOPASSWD: ALL' >/etc/sudoers.d/builder
git config --system --add safe.directory '*'
commit=$(git -C "$SRC" rev-parse "$GIT_REF")
echo "go in repos: $(pacman -Si go | awk '/^Version/ {print $3}')"
echo "-git source commit: $commit"

for pkg in "${PACKAGES[@]}"; do
  step "$pkg: lint PKGBUILD"
  mkdir -p "$BUILD/$pkg"
  cp "$SRC/packaging/aur/$pkg/PKGBUILD" "$SRC/packaging/aur/$pkg/.SRCINFO" "$BUILD/$pkg/"
  if [[ $pkg == *-git ]]; then
    sed -i "s|^source=.*|source=(\"\$_pkgname::git+file://$SRC#commit=$commit\")|" "$BUILD/$pkg/PKGBUILD"
  fi
  chown -R builder: "$BUILD"
  namcap "$BUILD/$pkg/PKGBUILD" >"$BUILD/$pkg/namcap-pkgbuild.txt" || fail "$pkg: namcap could not read the PKGBUILD"
  cat "$BUILD/$pkg/namcap-pkgbuild.txt"
  if grep -E ' E: ' "$BUILD/$pkg/namcap-pkgbuild.txt"; then
    fail "$pkg: namcap reported PKGBUILD errors"
  fi
  if [[ $pkg != *-git ]]; then
    diff <(cd "$BUILD/$pkg" && sudo -u builder makepkg --printsrcinfo) "$BUILD/$pkg/.SRCINFO" ||
      fail "$pkg: .SRCINFO is stale; run makepkg --printsrcinfo > .SRCINFO"
  fi

  step "$pkg: build with dependency resolution"
  start=$SECONDS
  (cd "$BUILD/$pkg" && sudo -u builder makepkg -s --noconfirm) || fail "$pkg: makepkg failed"
  echo "build time: $((SECONDS - start))s"
  # makepkg may also emit a -debug package; check only the main one.
  archive=$(cd "$BUILD/$pkg" && sudo -u builder makepkg --packagelist | grep -v -- '-debug-')
  [[ -f $archive ]] || fail "$pkg: package archive not found: $archive"

  step "$pkg: lint package"
  namcap "$archive" >"$BUILD/$pkg/namcap-package.txt" || fail "$pkg: namcap could not read the package"
  cat "$BUILD/$pkg/namcap-package.txt"
  if grep -E ' E: ' "$BUILD/$pkg/namcap-package.txt"; then
    fail "$pkg: namcap reported errors"
  fi

  step "$pkg: package contents"
  for file in "${FILES[@]}" "usr/share/licenses/$pkg/LICENSE"; do
    bsdtar -tf "$archive" | grep -qx "$file" || fail "$pkg: missing $file"
  done

  step "$pkg: install, runtime libraries and removal"
  pacman -U --noconfirm "$archive" >/dev/null
  for file in "${FILES[@]}"; do [[ -e /$file ]] || fail "$pkg: /$file not installed"; done
  if ldd /usr/bin/attack-shark-linux | grep 'not found'; then
    fail "$pkg: unresolved runtime libraries"
  fi
  # The build contract is measured on the installed binary itself: --version
  # reports the package metadata that also switches the in-app updater off.
  pkgver=$(bsdtar -xOf "$archive" .PKGINFO | awk -F' = ' '$1 == "pkgver" {print $2}')
  if [[ $pkg == *-git ]]; then
    want="attack-shark-linux main ${commit:0:7} (AUR, pacman)"
  else
    want="attack-shark-linux ${pkgver%-*} (AUR, pacman)"
  fi
  if got=$(env -u DISPLAY -u WAYLAND_DISPLAY /usr/bin/attack-shark-linux --version 2>&1); then
    [[ $got == "$want" ]] || fail "$pkg: --version printed '$got', want '$want'"
    echo "--version: $got"
  elif [[ $pkg != *-git ]] && ! grep -rq 'func BuildLabel' "$BUILD/$pkg/src"; then
    # Stable tags older than --version cannot report it; they still get no
    # release key, so their updater stays off. Report it, never pass silently.
    echo "SKIP: $pkg $pkgver predates --version; build contract not measurable"
  else
    fail "$pkg: --version failed: $got"
  fi
  desktop-file-validate /usr/share/applications/attack-shark-x6.desktop || fail "$pkg: invalid desktop entry"
  installed=$(pacman -Qq | grep -x "$pkg")
  pacman -R --noconfirm "$installed" >/dev/null
  for file in "${FILES[@]}"; do [[ ! -e /$file ]] || fail "$pkg: /$file left after removal"; done
  echo "$pkg: OK"
done

step "All AUR package checks passed"
