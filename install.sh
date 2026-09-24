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

usage() {
	printf '%s\n' 'Usage: install.sh [--beta] [--install-udev]' >&2
	exit 2
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
	rm -f "$workdir/manifest.json" "$workdir/appimage" "$workdir/payload" "$workdir/signature" "$workdir/public-key.der" "$workdir/udev.rules"
	rmdir "$workdir" 2>/dev/null || :
}

channel=stable
if [ "${1:-}" = "--beta" ]; then
	channel=RC
	shift
fi
install_udev=false
if [ "$#" -eq 1 ]; then
	[ "$1" = "--install-udev" ] || usage
	install_udev=true
	shift
fi
[ "$#" -eq 0 ] || usage

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
			[.[] | select(.prerelease == ($prerelease == "true") and .draft == false and .tag_name == $tag) |
			 select((.assets | length) == 5) |
			 select(all(.assets[]; .name == $appimage or .name == ($appimage + ".sha256") or .name == $manifest or .name == $rule or .name == "install.sh")) |
			 select((.assets | map(.name) | unique | length) == 5) |
			 select(all(.assets[]; .browser_download_url == ($root + "/" + .name)))
			] | length == 1' 2>/dev/null) || continue
		[ "$valid_release" = true ] || continue
		manifest_is_valid "$tag" || continue
		if [ -z "$selected_tag" ] || version_is_newer "$tag" "$selected_tag"; then
			selected_tag=$tag
			appimage_url=$candidate_appimage_url
			udev_url="$DOWNLOAD_ROOT/$tag/$UDEV_RULE_NAME"
			digest=$candidate_digest
		fi
	done
	[ "$releases_page" -lt "$MAX_RELEASE_PAGES" ] || fail 'release discovery did not terminate within 1000 pages'
	releases_page=$((releases_page + 1))
done
[ -n "$selected_tag" ] || fail "No valid signed GitHub $channel release is available."

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

desktop_dir="$account_home/.local/share/applications"
mkdir -p "$desktop_dir" || fail 'create user-local applications directory'
desktop_temporary=$(mktemp "$desktop_dir/.attack-shark-x6.desktop.XXXXXX") || fail 'create atomic desktop entry'
cat > "$desktop_temporary" <<EOF
[Desktop Entry]
Type=Application
Name=Attack Shark X6 Configurator
Exec=$install_path
Terminal=false
Categories=Settings;Utility;
EOF
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
