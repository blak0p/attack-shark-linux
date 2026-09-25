#!/bin/sh
if [ -z "$WEBKIT_EXEC_PATH" ] && [ -d "$APPDIR/usr/lib/x86_64-linux-gnu/webkitgtk-6.0" ]; then
    export WEBKIT_EXEC_PATH="$APPDIR/usr/lib/x86_64-linux-gnu/webkitgtk-6.0"
fi
