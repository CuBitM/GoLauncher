#!/bin/bash
# GoLauncher Build Script
# Requires: Go 1.21+, gcc (for Fyne CGO)

set -e

echo "=== GoLauncher Build ==="

# Install deps
echo "[1/3] Downloading dependencies..."
go mod tidy

echo "[2/3] Building..."

case "$1" in
  windows)
    GOOS=windows GOARCH=amd64 CGO_ENABLED=1 \
      go build -ldflags="-s -w -H windowsgui" \
      -o dist/GoLauncher.exe ./cmd/
    echo "Built: dist/GoLauncher.exe"
    ;;
  linux)
    GOOS=linux GOARCH=amd64 CGO_ENABLED=1 \
      go build -ldflags="-s -w" \
      -o dist/GoLauncher-linux ./cmd/
    echo "Built: dist/GoLauncher-linux"
    ;;
  darwin)
    GOOS=darwin GOARCH=amd64 CGO_ENABLED=1 \
      go build -ldflags="-s -w" \
      -o dist/GoLauncher-macos ./cmd/
    echo "Built: dist/GoLauncher-macos"
    ;;
  *)
    # Current platform
    mkdir -p dist
    CGO_ENABLED=1 go build -ldflags="-s -w" -o dist/GoLauncher ./cmd/
    echo "Built: dist/GoLauncher (current platform)"
    ;;
esac

echo "[3/3] Done!"
