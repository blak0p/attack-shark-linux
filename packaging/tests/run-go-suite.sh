#!/bin/sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd -P)
image=localhost/attack-shark-go-tests:1.25-ubuntu24.04

if ! command -v podman >/dev/null 2>&1; then
    echo 'Rootless Podman is required; no host packages will be installed.' >&2
    exit 1
fi
if [ "$(id -u)" -eq 0 ]; then
    echo 'Run this script as a non-root user (rootless Podman required).' >&2
    exit 1
fi
if [ ! -d "$repo_root/cmd/x6configurator/frontend/dist" ]; then
    echo 'Missing cmd/x6configurator/frontend/dist: build assets first with cd frontend && npm run build.' >&2
    exit 1
fi

# Building the image may access the network; execution below never does.
podman build --tag "$image" --file "$repo_root/packaging/tests/Containerfile" "$repo_root"

podman run --rm --network none --read-only \
    --mount "type=bind,src=$repo_root,dst=/workspace,ro=true" \
    --tmpfs /tmp:rw,nosuid,nodev,size=2g \
    --tmpfs /opt/go-build:rw,nosuid,nodev,size=2g \
    --env GOCACHE=/opt/go-build \
    --env GOTMPDIR=/tmp \
    --env GOPROXY=off \
    --env GOSUMDB=off \
    --workdir /workspace \
    "$image" sh -ec '
        version=$(go env GOVERSION)
        case "$version" in
            go1.25.*) ;;
            *) echo "Expected Go 1.25, found $version" >&2; exit 1 ;;
        esac
        for package in gtk4 webkitgtk-6.0 libsoup-3.0; do
            pkg-config --exists "$package" || {
                echo "Missing native pkg-config dependency: $package" >&2
                exit 1
            }
        done
        echo "Using $version with GTK4, WebKitGTK6 and libsoup3; running full Go suite offline."
        go test ./...
    '
