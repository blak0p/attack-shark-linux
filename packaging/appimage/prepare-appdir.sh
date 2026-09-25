#!/bin/sh
set -eu

APPDIR_TARGET="${1:-build/appimage/attack-shark-linux-x86_64.AppDir}"

if [ ! -d "$APPDIR_TARGET" ]; then
    echo "Error: AppDir directory not found at $APPDIR_TARGET" >&2
    exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

# Ensure bundled WebKitGTK helper processes have executable permissions
for helper in WebKitNetworkProcess WebKitWebProcess; do
    helper_path="$APPDIR_TARGET/usr/lib/x86_64-linux-gnu/webkitgtk-6.0/$helper"
    if [ -f "$helper_path" ]; then
        chmod 0755 "$helper_path"
    fi
done

# Compile WebKitGTK relocation shim to redirect helper process execution to AppDir
if [ -f "$SCRIPT_DIR/webkit-relocate.c" ]; then
    mkdir -p "$APPDIR_TARGET/usr/lib"
    gcc -O2 -shared -fPIC -o "$APPDIR_TARGET/usr/lib/libwebkit-relocate.so" "$SCRIPT_DIR/webkit-relocate.c" -ldl
    chmod 0755 "$APPDIR_TARGET/usr/lib/libwebkit-relocate.so"
fi

# Install apprun-hooks directory and runtime hooks
mkdir -p "$APPDIR_TARGET/apprun-hooks"
if [ -f "$SCRIPT_DIR/apprun-hooks/01-webkit-exec-path.sh" ]; then
    cp -f "$SCRIPT_DIR/apprun-hooks/01-webkit-exec-path.sh" "$APPDIR_TARGET/apprun-hooks/01-webkit-exec-path.sh"
else
    cat << 'EOF' > "$APPDIR_TARGET/apprun-hooks/01-webkit-exec-path.sh"
#!/bin/sh
if [ -z "$WEBKIT_EXEC_PATH" ] && [ -d "$APPDIR/usr/lib/x86_64-linux-gnu/webkitgtk-6.0" ]; then
    export WEBKIT_EXEC_PATH="$APPDIR/usr/lib/x86_64-linux-gnu/webkitgtk-6.0"
fi
EOF
fi
chmod 0755 "$APPDIR_TARGET/apprun-hooks/01-webkit-exec-path.sh"

# If AppRun is an ELF binary, wrap it so apprun-hooks are executed before startup
if [ -f "$APPDIR_TARGET/AppRun" ] && head -c 4 "$APPDIR_TARGET/AppRun" | grep -q 'ELF'; then
    mv -f "$APPDIR_TARGET/AppRun" "$APPDIR_TARGET/AppRun.wrapped"
    chmod 0755 "$APPDIR_TARGET/AppRun.wrapped"
fi

if [ -f "$SCRIPT_DIR/AppRun.wrapper" ]; then
    cp -f "$SCRIPT_DIR/AppRun.wrapper" "$APPDIR_TARGET/AppRun"
else
    cat << 'EOF' > "$APPDIR_TARGET/AppRun"
#!/bin/sh
set -e

this_dir="$(readlink -f "$(dirname "$0")")"
export APPDIR="${APPDIR:-"$this_dir"}"

if [ -f "$this_dir/usr/lib/libwebkit-relocate.so" ]; then
    export LD_PRELOAD="$this_dir/usr/lib/libwebkit-relocate.so${LD_PRELOAD:+:$LD_PRELOAD}"
fi

if [ -d "$this_dir/apprun-hooks" ]; then
    for hook in "$this_dir"/apprun-hooks/*.sh; do
        if [ -r "$hook" ]; then
            . "$hook"
        fi
    done
fi

exec "$this_dir/AppRun.wrapped" "$@"
EOF
fi
chmod 0755 "$APPDIR_TARGET/AppRun"
