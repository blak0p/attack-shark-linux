#!/bin/sh
# This file is published only as a GitHub release asset. The workflow replaces
# this placeholder with the release signing public key before uploading it.
PUBLIC_KEY="__ATTACK_SHARK_RELEASE_PUBLIC_KEY__"
API_URL="https://api.github.com/repos/blak0p/attack-shark-linux/releases"
APPIMAGE_NAME="attack-shark-linux-x86_64.AppImage"
MANIFEST_NAME="update-manifest.json"
UDEV_RULE_NAME="60-attack-shark-x6-hidraw.rules"
DOWNLOAD_ROOT="https://github.com/blak0p/attack-shark-linux/releases/download"
LC_ALL=C
export LC_ALL

help_text() {
	printf '%s\n' 'Usage: install.sh [--beta] [--install-udev]' \
		'       install.sh [--beta] --setup-usb-permissions' \
		'       install.sh --help' \
		'--setup-usb-permissions: one-time administrator-trusted helper/policy setup;' \
		'  authorization required, no daemon or AppImage installation.' \
		'  Requires /usr/bin/python3 only for setup. Trust this versioned installer' \
		'  and its embedded release public key; an AppImage is not a root trust anchor.'
}
usage() { help_text >&2; exit 2; }

# Fixed installer-owned program, never loaded from a downloaded script. The
# release workflow renders BOTH pins in this trusted installer. -I -S excludes
# caller Python paths, environment configuration and site startup code.
usb_bootstrap_program() {
	cat <<'USB_SETUP_PYTHON'
import os
import sys
ENV = {'PATH': '/usr/sbin:/usr/bin:/sbin:/bin', 'LC_ALL': 'C', 'OPENSSL_CONF': '/dev/null'}
os.environ.clear()
os.environ.update(ENV)
import base64
import binascii
import hashlib
import re
import signal
import stat
import subprocess
import tempfile

PIN = '__ATTACK_SHARK_RELEASE_PUBLIC_KEY__'
ROOT_UID = 0
RUNTIME = '/run'
ROOT_PARENTS = ('/', '/run')
INSTALL_PARENTS = ('/', '/usr', '/usr/local', '/usr/local/libexec')
HELPER = '/usr/local/libexec/attack-shark-linux-udev-helper'
ASSETS = (('attack-shark-linux-udev-helper', 65536),
          ('udev-helper-manifest.txt', 98304), ('udev-policy.txt', 98304))

class Invalid(Exception):
    pass

def reject(message='invalid authenticated input'):
    raise Invalid(message)

def deadline(signum, frame):
    reject()

signal.signal(signal.SIGALRM, deadline)

def read_regular(directory, name, limit, trusted=False):
    # One descriptor: no pathname type-check/open race, FIFO blocking, or final
    # symlink traversal. A directory fd anchors all three source basenames.
    signal.alarm(5)
    try:
        fd = os.open(name, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK | os.O_CLOEXEC, dir_fd=directory)
        try:
            meta = os.fstat(fd)
            if not stat.S_ISREG(meta.st_mode) or meta.st_size > limit:
                reject()
            if trusted and (meta.st_uid != ROOT_UID or meta.st_mode & 0o077):
                reject()
            data = bytearray()
            while len(data) <= limit:
                chunk = os.read(fd, min(16384, limit + 1 - len(data)))
                if not chunk:
                    break
                data.extend(chunk)
            if not data or len(data) > limit:
                reject()
            return bytes(data)
        finally:
            os.close(fd)
    finally:
        signal.alarm(0)

def read_assets(path, trusted=False):
    directory = os.open(path, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW | os.O_CLOEXEC)
    try:
        return {name: read_regular(directory, name, limit, trusted) for name, limit in ASSETS}
    finally:
        os.close(directory)

def decode(encoded, size=None):
    try:
        data = base64.b64decode(encoded, validate=True)
    except (ValueError, binascii.Error):
        reject()
    if base64.b64encode(data) != encoded or (size is not None and len(data) != size):
        reject()
    return data

def put(path, data):
    with open(path, 'xb') as output:
        output.write(data)

# Temporary paths below are in an initializer-owned private directory. Neither
# destination names nor crypto commands/keys are accepted from downloaded data.
def envelope(wire, domain, version, work):
    count = 6 if domain == b'attack-shark-udev-policy-v1' else 4
    lines = wire.split(b'\n')
    if len(wire) > 98304 or len(lines) != count + 1 or lines[-1] != b'' or lines[0] != domain:
        reject()
    if lines[1] != b'version=' + version.encode('ascii') or not re.fullmatch(b'sha256=[0-9a-f]{64}', lines[2]):
        reject()
    if not lines[-2].startswith(b'signature='):
        reject()
    signature = decode(lines[-2][10:], 64)
    signed = b'\n'.join(lines[:-2]) + b'\n'
    # Each verification has its own private child directory and bounded inputs.
    with tempfile.TemporaryDirectory(dir=work) as crypto:
        put(crypto + '/key', bytes.fromhex('302a300506032b6570032100') + decode(PIN.encode('ascii'), 32))
        put(crypto + '/signed', signed)
        put(crypto + '/signature', signature)
        result = subprocess.run(['/usr/bin/openssl', 'pkeyutl', '-verify', '-pubin', '-rawin',
                                 '-keyform', 'DER', '-inkey', crypto + '/key', '-in', crypto + '/signed',
                                 '-sigfile', crypto + '/signature'], env=ENV, close_fds=True,
                                stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, timeout=5)
        if result.returncode != 0:
            reject()
    if count == 4:
        return lines[2][7:]
    if not lines[3].startswith(b'previous=') or not lines[4].startswith(b'payload='):
        reject()
    history = lines[3][9:].split(b',')
    if not 1 <= len(history) <= 16 or history != sorted(set(history)):
        reject()
    if any(not re.fullmatch(b'[0-9a-f]{64}', item) for item in history):
        reject()
    policy = decode(lines[4][8:])
    if not policy or len(policy) > 65536 or not policy.endswith(b'\n'):
        reject()
    if hashlib.sha256(policy).hexdigest().encode('ascii') != lines[2][7:]:
        reject()
    rules = 0
    for line in policy[:-1].decode('utf-8').split('\n'):
        if any(ord(c) < 32 or ord(c) == 127 for c in line):
            reject()
        if not line or line.startswith('#'):
            continue
        if not re.fullmatch(r'SUBSYSTEM=="hidraw", ATTRS\{idVendor\}=="1d57", ATTRS\{idProduct\}=="fa6[01]", TAG\+="uaccess", MODE="0660"', line):
            reject()
        rules += 1
    if not rules:
        reject()
    return policy

def verify(data, version, work):
    # Setup accepts only the installer's stable/RC channel identities, exactly
    # tag-without-v, and the artifact API's version length/uint64 core bounds.
    if len(version) > 80 or not re.fullmatch(r'(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-rc\.(0|[1-9][0-9]*))?', version):
        reject()
    if any(len(n) > 20 or int(n) > 18446744073709551615 for n in version.split('-')[0].split('.')):
        reject()
    digest = envelope(data['udev-helper-manifest.txt'], b'attack-shark-udev-helper-v1', version, work)
    if hashlib.sha256(data['attack-shark-linux-udev-helper']).hexdigest().encode('ascii') != digest:
        reject()
    envelope(data['udev-policy.txt'], b'attack-shark-udev-policy-v1', version, work)

def safe(path, directory=False):
    meta = os.lstat(path)
    expected = stat.S_ISDIR if directory else stat.S_ISREG
    return expected(meta.st_mode) and meta.st_uid == ROOT_UID and not meta.st_mode & 0o022

def parents(paths, create=False):
    for path in paths:
        if create and not os.path.lexists(path):
            try:
                os.mkdir(path, 0o755)
            except FileExistsError:
                pass
        if not safe(path, True):
            reject('unsafe parent')

def safe_helper():
    if os.path.lexists(HELPER) and not safe(HELPER):
        reject('unsafe helper')

installed = False
action = sys.argv[1] if len(sys.argv) == 4 else ''
try:
    if action not in ('check', 'install'):
        reject()
    source, version = sys.argv[2:]
    os.umask(0o077)
    if action == 'check':
        # Caller-private scratch only. Nothing here becomes privileged code.
        with tempfile.TemporaryDirectory(dir=source) as work:
            verify(read_assets(source), version, work)
    else:
        if os.geteuid() != ROOT_UID:
            reject('root required')
        parents(ROOT_PARENTS)
        # No final-system changes before the independently authenticated copy.
        with tempfile.TemporaryDirectory(prefix='.attack-shark-bootstrap-', dir=RUNTIME) as work:
            for name, data in read_assets(source).items():
                put(work + '/' + name, data)
            copied = read_assets(work, trusted=True)
            verify(copied, version, work)
            parents(INSTALL_PARENTS, create=True)
            safe_helper()
            # Same-filesystem private staging, then atomic canonical rename.
            with tempfile.TemporaryDirectory(prefix='.attack-shark-bootstrap-', dir=INSTALL_PARENTS[-1]) as final:
                candidate = final + '/helper'
                put(candidate, copied['attack-shark-linux-udev-helper'])
                os.chmod(candidate, 0o755)
                parents(INSTALL_PARENTS)
                safe_helper()
                os.replace(candidate, HELPER)
                installed = True
            print('helper-installed', flush=True)
            # Only the verified root-owned canonical path is ever executed.
            result = subprocess.run([HELPER, version], input=copied['udev-policy.txt'],
                                    env=ENV, close_fds=True, timeout=60)
            if result.returncode != 0:
                reject('helper installed; policy application failed (see helper output)')
            print('USB permissions setup complete.', flush=True)
except (Invalid, OSError, ValueError, subprocess.SubprocessError) as error:
    message = str(error) if isinstance(error, Invalid) else 'invalid authenticated input'
    if action == 'install' and not installed:
        message += '; helper unchanged; policy not applied'
    elif installed and not message.startswith('helper installed;'):
        message = 'helper installed; policy outcome uncertain (no automatic retry)'
    print('usb-setup: ' + message, file=sys.stderr)
    sys.exit(1)
USB_SETUP_PYTHON
}

manual_udev_fallback() {
	rule_url=$1
	printf '%s\n' 'Install the canonical udev rule manually, then reload and trigger rules before reconnecting the dongle:' >&2
	printf '%s\n' "curl --fail --location --silent --show-error '$rule_url' | sudo install -Dm0644 /dev/stdin /etc/udev/rules.d/60-attack-shark-x6-hidraw.rules" >&2
	printf '%s\n' 'sudo udevadm control --reload-rules' >&2
	printf '%s\n' 'sudo udevadm trigger' >&2
}

fail() {
	printf 'install.sh: %s\n' "$*" >&2
	exit 1
}

require_command() {
	command -v "$1" >/dev/null 2>&1 || fail "required command is unavailable: $1"
}

is_stable_tag() {
	printf '%s\n' "$1" | grep -Eq '^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$'
}

is_rc_tag() {
	printf '%s\n' "$1" | grep -Eq '^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)-rc\.(0|[1-9][0-9]*)$'
}

cleanup() {
	rm -rf "$workdir/squashfs-root"
	rm -f "$workdir/manifest.json" "$workdir/appimage" "$workdir/payload" "$workdir/signature" "$workdir/public-key.der" "$workdir/udev.rules" \
		"$workdir/attack-shark-linux-udev-helper" "$workdir/udev-helper-manifest.txt" "$workdir/udev-policy.txt"
	rmdir "$workdir" 2>/dev/null || :
}

if [ "$#" -eq 1 ] && [ "$1" = --help ]; then help_text; exit 0; fi
channel=stable
if [ "${1:-}" = "--beta" ]; then
	channel=RC
	shift
fi
install_udev=false
setup_usb=false
if [ "$#" -eq 1 ]; then
	if [ "$1" = "--install-udev" ]; then
		install_udev=true
	elif [ "$1" = "--setup-usb-permissions" ]; then
		setup_usb=true
	else
		usage
	fi
	shift
fi
[ "$#" -eq 0 ] || usage
if [ "$setup_usb" = true ]; then
	[ -x /usr/bin/python3 ] || fail 'USB setup requires /usr/bin/python3. Install Python 3 with your distro package manager, then rerun --setup-usb-permissions. No packages were installed.'
fi

account_home=$(getent passwd "$(id -u)" | awk -F: 'NR == 1 { print $6 }')
case "$account_home" in
	/*) ;;
	*) fail 'current OS account home is empty, unresolvable, or not absolute' ;;
esac
# Split the sentinel so workflow-wide key substitution cannot rewrite this guard.
[ "$PUBLIC_KEY" != '__ATTACK_SHARK_RELEASE_'"PUBLIC_KEY__" ] || fail 'installer public key was not injected by the release workflow'

for command in awk curl getent id jq openssl base64 sha256sum grep mktemp; do
	require_command "$command"
done

workdir=$(mktemp -d "${TMPDIR:-/tmp}/attack-shark-x6-install.XXXXXX") || fail 'create temporary directory'
trap cleanup 0 1 2 15
decimal_is_longer() {
	length_left_number=$1
	length_right_number=$2
	while [ -n "$length_left_number" ] && [ -n "$length_right_number" ]; do
		length_left_number=${length_left_number#?}
		length_right_number=${length_right_number#?}
	done
	[ -n "$length_left_number" ] && [ -z "$length_right_number" ]
}

# Both inputs have the same decimal length. Compare digit by digit so values
# never enter shell arithmetic and remain valid at arbitrary precision.
decimal_is_greater_at_equal_length() {
	left_number=$1
	right_number=$2
	while [ -n "$left_number" ]; do
		left_digit=${left_number%"${left_number#?}"}
		right_digit=${right_number%"${right_number#?}"}
		if [ "$left_digit" != "$right_digit" ]; then
			case "$left_digit:$right_digit" in
				1:0|2:0|2:1|3:0|3:1|3:2|4:0|4:1|4:2|4:3|5:0|5:1|5:2|5:3|5:4|6:0|6:1|6:2|6:3|6:4|6:5|7:0|7:1|7:2|7:3|7:4|7:5|7:6|8:0|8:1|8:2|8:3|8:4|8:5|8:6|8:7|9:0|9:1|9:2|9:3|9:4|9:5|9:6|9:7|9:8) return 0 ;;
				*) return 1 ;;
			esac
		fi
		left_number=${left_number#?}
		right_number=${right_number#?}
	done
	return 1
}

version_is_newer() {
	left=${1#v}; right=${2#v}
	left_base=${left%-rc.*}; right_base=${right%-rc.*}
	left_rc=${left##*.}; right_rc=${right##*.}
	left_major=${left_base%%.*}; left_rest=${left_base#*.}
	left_minor=${left_rest%%.*}; left_patch=${left_rest#*.}
	right_major=${right_base%%.*}; right_rest=${right_base#*.}
	right_minor=${right_rest%%.*}; right_patch=${right_rest#*.}
	for pair in "$left_major:$right_major" "$left_minor:$right_minor" "$left_patch:$right_patch"; do
		left_number=${pair%%:*}; right_number=${pair#*:}
		if decimal_is_longer "$left_number" "$right_number"; then
			return 0
		fi
		if decimal_is_longer "$right_number" "$left_number"; then
			return 1
		fi
		if [ "$left_number" != "$right_number" ]; then
			decimal_is_greater_at_equal_length "$left_number" "$right_number"
			return
		fi
	done
	if [ "$channel" = RC ]; then
		if decimal_is_longer "$left_rc" "$right_rc"; then return 0; fi
		if decimal_is_longer "$right_rc" "$left_rc"; then return 1; fi
		decimal_is_greater_at_equal_length "$left_rc" "$right_rc"
		return
	fi
	return 1
}

manifest_is_valid() {
	candidate_tag=$1
	candidate_url="$DOWNLOAD_ROOT/$candidate_tag"
	candidate_manifest_url="$candidate_url/$MANIFEST_NAME"
	candidate_appimage_url="$candidate_url/$APPIMAGE_NAME"
	curl --fail --location --silent --show-error "$candidate_manifest_url" -o "$workdir/manifest.json" >/dev/null 2>&1 || return 1
	candidate_version=$(jq -er '.version | strings' "$workdir/manifest.json" 2>/dev/null) || return 1
	candidate_manifest_url_value=$(jq -er '.url | strings' "$workdir/manifest.json" 2>/dev/null) || return 1
	candidate_digest=$(jq -er '.sha256 | strings' "$workdir/manifest.json" 2>/dev/null) || return 1
	candidate_signature=$(jq -er '.signature | strings' "$workdir/manifest.json" 2>/dev/null) || return 1
	[ "$candidate_version" = "${candidate_tag#v}" ] || return 1
	[ "$candidate_manifest_url_value" = "$candidate_appimage_url" ] || return 1
	printf '%s\n' "$candidate_digest" | grep -Eq '^[0-9a-f]{64}$' || return 1
	printf 'version=%s\nurl=%s\nsha256=%s\n' "$candidate_version" "$candidate_manifest_url_value" "$candidate_digest" > "$workdir/payload"
	printf '%s' "$candidate_signature" | base64 -d > "$workdir/signature" 2>/dev/null || return 1
	openssl pkeyutl -verify -pubin -rawin -keyform DER -inkey "$workdir/public-key.der" -in "$workdir/payload" -sigfile "$workdir/signature" >/dev/null 2>&1 || return 1
	return 0
}

printf '\060\052\060\005\006\003\053\145\160\003\041\000' > "$workdir/public-key.der"
printf '%s' "$PUBLIC_KEY" | base64 -d >> "$workdir/public-key.der" 2>/dev/null || fail 'embedded public key is invalid'
selected_tag=
# GitHub returns at most 100 releases per page. A 1,000-page ceiling permits
# discovery through 100,000 releases while bounding arithmetic and bad feeds.
MAX_RELEASE_PAGES=1000
releases_page=1
seen_page_fingerprints=
while [ "$releases_page" -le "$MAX_RELEASE_PAGES" ]; do
	releases=$(curl --fail --location --silent --show-error "$API_URL?per_page=100&page=$releases_page") || fail 'query GitHub releases'
	release_count=$(printf '%s' "$releases" | jq -er 'if type == "array" then length else error("release response is not an array") end' 2>/dev/null) || fail 'GitHub release response is invalid'
	[ "$release_count" -eq 0 ] && break
	page_fingerprint=$(printf '%s' "$releases" | sha256sum) || fail 'fingerprint GitHub release response'
	if printf '%s\n' "$seen_page_fingerprints" | grep -Fqx "$page_fingerprint"; then
		fail 'release discovery did not terminate: repeated nonempty GitHub release page'
	fi
	seen_page_fingerprints="${seen_page_fingerprints}${page_fingerprint}
"
	prerelease=false
	if [ "$channel" = RC ]; then prerelease=true; fi
	for tag in $(printf '%s' "$releases" | jq -r --arg prerelease "$prerelease" '.[] | select(.prerelease == ($prerelease == "true") and .draft == false) | .tag_name' 2>/dev/null); do
		if [ "$channel" = RC ]; then is_rc_tag "$tag" || continue; else is_stable_tag "$tag" || continue; fi
		download_url="$DOWNLOAD_ROOT/$tag"
		valid_release=$(printf '%s' "$releases" | jq -e --arg tag "$tag" --arg root "$download_url" \
			--arg appimage "$APPIMAGE_NAME" --arg manifest "$MANIFEST_NAME" --arg rule "$UDEV_RULE_NAME" --arg prerelease "$prerelease" '
			([$appimage, ($appimage + ".sha256"), $manifest, $rule, "install.sh"] | sort) as $legacy |
			(($legacy + ["attack-shark-linux-udev-helper", "udev-helper-manifest.txt", "udev-policy.txt"]) | sort) as $complete |
			[.[] | select(.prerelease == ($prerelease == "true") and .draft == false and .tag_name == $tag) |
			 (.assets | map(.name) | sort) as $names |
			 select($names == $legacy or $names == $complete) |
			 select(all(.assets[]; .browser_download_url == ($root + "/" + .name)))
			] | length == 1' 2>/dev/null) || continue
		[ "$valid_release" = true ] || continue
		manifest_is_valid "$tag" || continue
		if [ -z "$selected_tag" ] || version_is_newer "$tag" "$selected_tag"; then
			selected_tag=$tag
			if [ "$setup_usb" = true ]; then
				selected_has_usb=$(printf '%s' "$releases" | jq -r --arg tag "$tag" '.[] | select(.tag_name == $tag) | (.assets | length) == 8')
			fi
			appimage_url=$candidate_appimage_url
			udev_url="$DOWNLOAD_ROOT/$tag/$UDEV_RULE_NAME"
			digest=$candidate_digest
		fi
	done
	[ "$releases_page" -lt "$MAX_RELEASE_PAGES" ] || fail 'release discovery did not terminate within 1000 pages'
	releases_page=$((releases_page + 1))
done
[ -n "$selected_tag" ] || fail "No valid signed GitHub $channel release is available."

if [ "$setup_usb" = true ]; then
	[ "$selected_has_usb" = true ] || fail 'Selected release has no authenticated USB setup assets; use a release that publishes them. No AppImage changes made.'
	for asset in attack-shark-linux-udev-helper udev-helper-manifest.txt udev-policy.txt; do
		curl --fail --location --silent --show-error --max-time 20 --max-filesize 98304 "$DOWNLOAD_ROOT/$selected_tag/$asset" -o "$workdir/$asset" || fail 'download authenticated USB setup assets'
	done
	usb_bootstrap_program | /usr/bin/python3 -I -S - check "$workdir" "${selected_tag#v}" || fail 'USB setup validation failed before authorization.'
	printf '%s\n' 'USB setup requires administrator authorization; trust this installer and its embedded release key.' >&2
	usb_bootstrap_program | /usr/bin/sudo -- /usr/bin/python3 -I -S - install "$workdir" "${selected_tag#v}" || fail 'USB setup transaction failed; inspect status above (no automatic retry).'
	exit 0
fi

curl --fail --location --silent --show-error "$appimage_url" -o "$workdir/appimage" || fail 'download AppImage'
printf '%s  %s\n' "$digest" "$workdir/appimage" | sha256sum -c - >/dev/null 2>&1 || fail 'downloaded AppImage SHA-256 does not match signed manifest'

install_dir="$account_home/.local/share/attack-shark-x6"
install_path="$install_dir/$APPIMAGE_NAME"
mkdir -p "$install_dir" || fail 'create user-local install directory'
temporary_install=$(mktemp "$install_dir/.attack-shark-linux-x86_64.AppImage.XXXXXX") || fail 'create atomic install target'
chmod 0755 "$workdir/appimage" || fail 'mark downloaded AppImage executable'
cat "$workdir/appimage" > "$temporary_install" || fail 'write AppImage'
chmod 0755 "$temporary_install" || fail 'preserve AppImage executable permission'
mv -f "$temporary_install" "$install_path" || fail 'atomically install AppImage'

(cd "$workdir" && "$install_path" --appimage-extract .DirIcon) >/dev/null 2>&1 || :
if [ ! -s "$workdir/squashfs-root/.DirIcon" ]; then
	(cd "$workdir" && "$install_path" --appimage-extract attack-shark-x6.svg) >/dev/null 2>&1 || :
fi
if [ ! -s "$workdir/squashfs-root/.DirIcon" ] && [ ! -s "$workdir/squashfs-root/attack-shark-x6.svg" ]; then
	(cd "$workdir" && "$install_path" --appimage-extract packaging/icons/attack-shark-x6.svg) >/dev/null 2>&1 || :
fi

icon_source=
for candidate in \
	"$workdir/squashfs-root/.DirIcon" \
	"$workdir/squashfs-root/attack-shark-x6.svg" \
	"$workdir/squashfs-root/packaging/icons/attack-shark-x6.svg"; do
	if [ -s "$candidate" ]; then
		icon_source=$candidate
		break
	fi
done
[ -n "$icon_source" ] || fail 'extract application icon from AppImage'

icon_theme_dir="$account_home/.local/share/icons/hicolor/scalable/apps"
mkdir -p "$icon_theme_dir" || fail 'create user-local icon theme directory'

icon_temporary=$(mktemp "$install_dir/.attack-shark-x6.svg.XXXXXX") || fail 'create atomic icon target'
cat "$icon_source" > "$icon_temporary" || fail 'write application icon'
chmod 0644 "$icon_temporary" || fail 'preserve application icon permissions'
mv -f "$icon_temporary" "$install_dir/attack-shark-x6.svg" || fail 'atomically install application icon'

theme_icon_temporary=$(mktemp "$icon_theme_dir/.attack-shark-x6.svg.XXXXXX") || fail 'create atomic theme icon target'
cat "$icon_source" > "$theme_icon_temporary" || fail 'write theme icon'
chmod 0644 "$theme_icon_temporary" || fail 'preserve theme icon permissions'
mv -f "$theme_icon_temporary" "$icon_theme_dir/attack-shark-x6.svg" || fail 'atomically install theme icon'

rm -rf "$workdir/squashfs-root"

desktop_dir="$account_home/.local/share/applications"
mkdir -p "$desktop_dir" || fail 'create user-local applications directory'
desktop_temporary=$(mktemp "$desktop_dir/.attack-shark-x6.desktop.XXXXXX") || fail 'create atomic desktop entry'
cat > "$desktop_temporary" <<EOF
[Desktop Entry]
Type=Application
Name=Attack Shark X6 Configurator
Exec=$install_path
Icon=attack-shark-x6
Terminal=false
Categories=Settings;Utility;
EOF
chmod 0644 "$desktop_temporary" || fail 'preserve desktop entry permissions'
mv -f "$desktop_temporary" "$desktop_dir/attack-shark-x6.desktop" || fail 'atomically install desktop entry'
printf 'Installed %s from signed %s %s\n' "$install_path" "$channel" "$selected_tag"

if [ "$install_udev" = true ]; then
	if [ ! -t 0 ]; then
		printf '%s\n' 'Skipping udev installation because input is noninteractive.' >&2
		manual_udev_fallback "$udev_url"
		exit 0
	fi
	printf 'Install the Attack Shark X6 udev rule with sudo? [y/N] ' >&2
	IFS= read -r confirmation || confirmation=
	case "$confirmation" in
		y|Y|yes|YES)
			if curl --fail --location --silent --show-error "$udev_url" -o "$workdir/udev.rules" && \
				sudo install -Dm0644 "$workdir/udev.rules" "/etc/udev/rules.d/$UDEV_RULE_NAME" && \
				sudo udevadm control --reload-rules && sudo udevadm trigger; then
				printf '%s\n' 'Installed and reloaded the udev rule.'
			else
				printf '%s\n' 'Unable to install or reload the udev rule.' >&2
				manual_udev_fallback "$udev_url"
			fi
			;;
		*)
			printf '%s\n' 'Udev installation was not confirmed.' >&2
			manual_udev_fallback "$udev_url"
			;;
	esac
fi
