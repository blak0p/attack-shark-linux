#!/bin/sh
# Release template, NOT installable until its public-key pin is rendered.
# VERSION is an exact release identity, not an upgrade comparison. Input: stdin.
set -eu
PATH=/usr/sbin:/usr/bin:/sbin:/bin
LC_ALL=C
export PATH LC_ALL
unset OPENSSL_CONF OPENSSL_MODULES OPENSSL_ENGINES ENV BASH_ENV LD_PRELOAD LD_LIBRARY_PATH
OPENSSL_CONF=/dev/null
export OPENSSL_CONF
umask 077
HELPER='/usr/local/libexec/attack-shark-linux-udev-helper'
TARGET='/etc/udev/rules.d/60-attack-shark-x6-hidraw.rules'
LOCK='/run/attack-shark-linux/helper.lock'
WORK='/run/attack-shark-linux'
USB_DEVICES='/sys/bus/usb/devices'
SYS_DEVICES='/sys/devices'
TRUSTED_PARENTS='/ /etc /etc/udev /etc/udev/rules.d /run /usr /usr/local /usr/local/libexec'
fail() { printf 'error: %s\n' "$1" >&2; exit 1; }
usage() { printf 'usage: attack-shark-linux-udev-helper VERSION < signed-policy\n'; }
if [ "$#" = 1 ] && [ "$1" = --help ]; then usage; exit 0; fi
if [ "$#" != 1 ]; then usage >&2; exit 2; fi
VERSION=$1
[ "$(/usr/bin/id -u)" = 0 ] || fail 'root required'
# Every ancestor is fixed and root-owned; independent root editors must not race
# this transaction. The fixed lock serializes all helper invocations.
safe() {
    [ ! -L "$1" ] || return 1
    meta=$(/usr/bin/stat -c '%u:%a' -- "$1") || return 1
    [ "${meta%%:*}" = 0 ] || return 1
    mode=${meta#*:}
    [ "$((0$mode & 022))" = 0 ]
}
for parent in $TRUSTED_PARENTS; do
    [ -d "$parent" ] && safe "$parent" || fail 'unsafe parent'
done
[ "$0" = "$HELPER" ] && [ -f "$HELPER" ] && safe "$HELPER" || fail 'unsafe helper'
# Recreate only fixed private runtime metadata after reboot. Concurrent helpers
# may win creation; validate actual resulting objects, never repair unsafe ones.
if [ ! -e "$WORK" ] && [ ! -L "$WORK" ]; then
    /usr/bin/mkdir -m 0700 -- "$WORK" 2>/dev/null || :
fi
[ -d "$WORK" ] && safe "$WORK" || fail 'unsafe lock'
[ "$(/usr/bin/stat -c '%a' -- "$WORK")" = 700 ] || fail 'unsafe lock'
if [ ! -e "$LOCK" ] && [ ! -L "$LOCK" ]; then
    (set -C; : > "$LOCK") 2>/dev/null || :
fi
[ -f "$LOCK" ] && safe "$LOCK" || fail 'unsafe lock'
[ "$(/usr/bin/stat -c '%a' -- "$LOCK")" = 600 ] || fail 'unsafe lock'
exec 9< "$LOCK"
/usr/bin/flock -w 5 9 || fail 'lock unavailable'
tmp=$(/usr/bin/mktemp -d "$WORK/transaction.XXXXXXXXXX") || fail 'temporary storage unavailable'
trap '/usr/bin/rm -rf -- "$tmp"' EXIT
trap 'exit 1' HUP INT TERM
# Never open a caller-selected file; cap both input bytes and time before parse.
/usr/bin/timeout 5 /usr/bin/dd bs=98305 count=1 iflag=fullblock status=none > "$tmp/wire" 2>/dev/null || fail 'invalid policy'
[ "$(/usr/bin/wc -c < "$tmp/wire")" -le 98304 ] || fail 'invalid policy'
[ "$(/usr/bin/tail -c 1 "$tmp/wire" | /usr/bin/od -An -tu1 | /usr/bin/tr -d ' ')" = 10 ] || fail 'invalid policy'
# Compare argv literally: awk -v would interpret backslash escapes in VERSION.
[ "$(/usr/bin/sed -n '2p' "$tmp/wire")" = "version=$VERSION" ] || fail 'invalid policy'
# Fixed line ordering and the same bounded semantic-version grammar as Go.
/usr/bin/awk '
function identifiers(text, prerelease, parts, n, i) {
    if (text=="") return 0
    n=split(text, parts, /\./)
    for (i=1; i<=n; i++) {
        if (parts[i]!~/^[0-9A-Za-z-]+$/) return 0
        if (prerelease && parts[i]~/^[0-9]+$/ && length(parts[i])>1 && substr(parts[i],1,1)=="0") return 0
    }
    return 1
}
function semver(version, build, count, core, dash, numbers, i, size) {
    if (length(version)<1 || length(version)>80) return 0
    sub(/^v/, "", version)
    count=split(version, build, /\+/)
    if (count>2 || (count==2 && !identifiers(build[2],0))) return 0
    core=build[1]
    dash=index(core,"-")
    if (dash) {
        if (!identifiers(substr(core,dash+1),1)) return 0
        core=substr(core,1,dash-1)
    }
    if (split(core,numbers,/\./)!=3) return 0
    for (i=1; i<=3; i++) {
        size=length(numbers[i])
        if (numbers[i]!~/^[0-9]+$/ || (size>1 && substr(numbers[i],1,1)=="0")) return 0
        # Prefix both operands to force lexical uint64-bound comparison.
        if (size>20 || (size==20 && ("x" numbers[i])>"x18446744073709551615")) return 0
    }
    return 1
}
NR==1 { if ($0!="attack-shark-udev-policy-v1") exit 1 }
NR==2 { if (substr($0,1,8)!="version=" || !semver(substr($0,9))) exit 1 }
NR==3 { if (length($0)!=71 || $0!~/^sha256=[0-9a-f]+$/) exit 1 }
NR==4 {
    if ($0!~/^previous=/) exit 1
    n=split(substr($0,10), history, ",")
    if (n<1 || n>16) exit 1
    for (i=1; i<=n; i++) {
        if (length(history[i])!=64 || history[i]!~/^[0-9a-f]+$/ || (i>1 && ("x" history[i-1])>=("x" history[i]))) exit 1
    }
}
NR==5 { if ($0!~/^payload=[A-Za-z0-9+\/]+={0,2}$/) exit 1 }
NR==6 { if (length($0)!=98 || $0!~/^signature=[A-Za-z0-9+\/]+==$/) exit 1 }
NR>6 { exit 1 }
END { if (NR!=6) exit 1 }
' "$tmp/wire" || fail 'invalid policy'
# Reconstruct and compare to reject missing final newline and noncanonical data.
/usr/bin/head -n 5 "$tmp/wire" > "$tmp/signed"
/usr/bin/tail -n 1 "$tmp/wire" > "$tmp/signature-line"
/usr/bin/cat "$tmp/signed" "$tmp/signature-line" > "$tmp/canonical"
/usr/bin/cmp -s "$tmp/wire" "$tmp/canonical" || fail 'invalid policy'
/usr/bin/cut -c 11- "$tmp/signature-line" | /usr/bin/openssl base64 -d -A > "$tmp/signature" 2>/dev/null || fail 'invalid policy'
[ "$(/usr/bin/wc -c < "$tmp/signature")" = 64 ] || fail 'invalid policy'
encoded_signature=$(/usr/bin/openssl base64 -A -in "$tmp/signature")
[ "signature=$encoded_signature" = "$(/usr/bin/cat "$tmp/signature-line")" ] || fail 'invalid policy'
/usr/bin/cat > "$tmp/key" <<'PUBLIC_KEY'
@RELEASE_PUBLIC_KEY_PEM@
PUBLIC_KEY
/usr/bin/openssl pkeyutl -verify -pubin -inkey "$tmp/key" -rawin -in "$tmp/signed" -sigfile "$tmp/signature" >/dev/null 2>&1 || fail 'invalid policy'
/usr/bin/sed -n '5s/^payload=//p' "$tmp/wire" > "$tmp/encoded"
/usr/bin/openssl base64 -d -A -in "$tmp/encoded" -out "$tmp/policy" 2>/dev/null || fail 'invalid policy'
[ "$(/usr/bin/wc -c < "$tmp/policy")" -le 65536 ] || fail 'invalid policy'
/usr/bin/openssl base64 -A -in "$tmp/policy" > "$tmp/reencoded"
printf '\n' >> "$tmp/reencoded"
/usr/bin/cmp -s "$tmp/encoded" "$tmp/reencoded" || fail 'invalid policy'
digest=$(/usr/bin/sha256sum "$tmp/policy"); digest=${digest%% *}
[ "sha256=$digest" = "$(/usr/bin/sed -n '3p' "$tmp/wire")" ] || fail 'invalid policy'
# Only the two X6 hidraw/uaccess rules; comments cannot become directives.
/usr/bin/awk '
/[[:cntrl:]]/ { exit 1 }
/^#/ || /^$/ { next }
$0!~/^SUBSYSTEM=="hidraw", ATTRS\{idVendor\}=="1d57", ATTRS\{idProduct\}=="fa6[01]", TAG\+="uaccess", MODE="0660"$/ { exit 1 }
{ rules++ }
END { if (!rules) exit 1 }
' "$tmp/policy" || fail 'invalid policy'
# Require newline termination independently of awk line normalization.
[ "$(/usr/bin/tail -c 1 "$tmp/policy" | /usr/bin/od -An -tu1 | /usr/bin/tr -d ' ')" = 10 ] || fail 'invalid policy'
# History is authority only after verification of the complete signed envelope.
previous=$(/usr/bin/sed -n '4s/^previous=//p' "$tmp/wire")
classify() {
    if [ -L "$TARGET" ]; then fail 'unsafe target'; fi
    if [ ! -e "$TARGET" ]; then state=missing; return; fi
    [ -f "$TARGET" ] && safe "$TARGET" || fail 'unsafe target'
    [ "$(/usr/bin/stat -c '%s' "$TARGET")" -le 65536 ] || fail 'custom policy'
    if /usr/bin/cmp -s "$TARGET" "$tmp/policy"; then state=current; return; fi
    old=$(/usr/bin/sha256sum "$TARGET"); old=${old%% *}
    case ",$previous," in *",$old,"*) ;; *) fail 'custom policy' ;; esac
    state=old
}
classify
if [ "$state" = current ]; then printf 'current\n'; exit 0; fi
# Stage on the destination filesystem, under its trusted non-writable parent.
staged=$(/usr/bin/mktemp "${TARGET%/*}/.attack-shark.XXXXXXXXXX") || fail 'staging failed'
trap '/usr/bin/rm -f -- "$staged"; /usr/bin/rm -rf -- "$tmp"' EXIT
/usr/bin/cat "$tmp/policy" > "$staged" || fail 'staging failed'
/usr/bin/chmod 0644 "$staged" || fail 'staging failed'
# Recheck immediately before rename while holding the helper lock.
classify
if [ "$state" = current ]; then printf 'current\n'; exit 0; fi
/usr/bin/mv -fT -- "$staged" "$TARGET" || fail 'installation failed'
printf 'installed\n'
if ! /usr/bin/timeout 10 /usr/bin/udevadm control --reload-rules >/dev/null 2>&1; then
    printf 'error: installed-but-reload-failed\n' >&2; exit 3
fi
# hidraw's idVendor/idProduct live on USB ancestors, not on the hidraw node.
# Resolve only matching kernel USB devices, then constrain events by that parent.
trigger_failed() { printf 'error: installed-but-trigger-failed\n' >&2; exit 4; }
for usb in "$USB_DEVICES"/*; do
    [ -f "$usb/idVendor" ] && [ -f "$usb/idProduct" ] || continue
    IFS= read -r vendor < "$usb/idVendor" || continue
    IFS= read -r product < "$usb/idProduct" || continue
    [ "$vendor" = 1d57 ] || continue
    case "$product" in fa60|fa61) ;; *) continue ;; esac
    physical=$(/usr/bin/readlink -f -- "$usb") || trigger_failed
    case "$physical" in "$SYS_DEVICES"/*) ;; *) trigger_failed ;; esac
    for subsystem in usb hidraw; do
        /usr/bin/timeout 10 /usr/bin/udevadm trigger --action=change --subsystem-match="$subsystem" --parent-match="$physical" >/dev/null 2>&1 || trigger_failed
    done
done
