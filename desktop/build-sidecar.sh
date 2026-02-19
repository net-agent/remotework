#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
TARGET_DIR="$SCRIPT_DIR/src-tauri"

build_target() {
    local os=$1
    local arch=$2
    local target_triple=$3
    local ext=$4

    echo "Building for $target_triple..."
    CGO_ENABLED=0 GOOS=$os GOARCH=$arch go build -o "$TARGET_DIR/remotework-${target_triple}${ext}" "$ROOT_DIR/cmd"
    echo "  -> remotework-${target_triple}${ext}"
}

case "${1:-current}" in
    all)
        build_target darwin arm64 aarch64-apple-darwin ""
        build_target darwin amd64 x86_64-apple-darwin ""
        build_target windows amd64 x86_64-pc-windows-msvc ".exe"
        build_target linux amd64 x86_64-unknown-linux-gnu ""
        ;;
    current)
        OS=$(uname -s | tr '[:upper:]' '[:lower:]')
        ARCH=$(uname -m)
        case "$OS" in
            darwin)
                case "$ARCH" in
                    arm64) build_target darwin arm64 aarch64-apple-darwin "" ;;
                    x86_64) build_target darwin amd64 x86_64-apple-darwin "" ;;
                esac
                ;;
            linux)
                build_target linux amd64 x86_64-unknown-linux-gnu ""
                ;;
            *)
                echo "Unsupported OS: $OS"
                exit 1
                ;;
        esac
        ;;
    *)
        echo "Usage: $0 [current|all]"
        exit 1
        ;;
esac

echo "Done."
