#!/bin/sh
set -eu

APPIMAGE="${1:-}"

if [ -z "$APPIMAGE" ]; then
    echo "Usage: $0 <path-to-appimage>" >&2
    exit 1
fi

if [ ! -f "$APPIMAGE" ]; then
    echo "Error: AppImage not found at '$APPIMAGE'" >&2
    exit 1
fi

echo "==> [Smoke Test] 1. Validating extraction and CLI execution..."
set +e
cli_output=$(APPIMAGE_EXTRACT_AND_RUN=1 "$APPIMAGE" --help 2>&1)
cli_code=$?
set -e

# Exit code 2 (exitUsage) or 0 is expected when running --help
if [ "$cli_code" -ne 0 ] && [ "$cli_code" -ne 2 ]; then
    echo "Error: Basic CLI execution failed with unexpected exit code $cli_code" >&2
    echo "$cli_output" >&2
    exit "$cli_code"
fi

echo "==> [Smoke Test] 2. Validating headless GUI startup under xvfb..."
set +e
gui_output=$(APPIMAGE_EXTRACT_AND_RUN=1 xvfb-run -a timeout 5s "$APPIMAGE" 2>&1)
gui_code=$?
set -e

echo "$gui_output"

if echo "$gui_output" | grep -q "WebKitNetworkProcess.*No such file"; then
    echo "Error: WebKitNetworkProcess spawn failure detected in AppImage startup!" >&2
    exit 1
fi

if echo "$gui_output" | grep -q "WebKitWebProcess.*No such file"; then
    echo "Error: WebKitWebProcess spawn failure detected in AppImage startup!" >&2
    exit 1
fi

if [ "$gui_code" -ne 124 ]; then
    echo "Error: AppImage startup failed with exit code $gui_code (expected 124 timeout)" >&2
    exit "$gui_code"
fi

echo "==> [Smoke Test] AppImage successfully started and ran without WebKit crash."
