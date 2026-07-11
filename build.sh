#!/usr/bin/env bash
# Cross-compile Jane PDF Renamer for all distribution targets.
# Run from anywhere; outputs land in dist/.
set -euo pipefail
cd "$(dirname "$0")"

VERSION="${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo dev)}"
LDFLAGS="-s -w -X main.version=${VERSION}"

mkdir -p dist

echo "Building ${VERSION} for Windows (x64)..."
GOOS=windows GOARCH=amd64 go build -trimpath -ldflags="${LDFLAGS}" -o "dist/JanePDFRenamer.exe" .

echo "Building ${VERSION} for macOS (Apple Silicon)..."
GOOS=darwin GOARCH=arm64 go build -trimpath -ldflags="${LDFLAGS}" -o "dist/JanePDFRenamer-mac-applesilicon" .

echo "Building ${VERSION} for macOS (Intel)..."
GOOS=darwin GOARCH=amd64 go build -trimpath -ldflags="${LDFLAGS}" -o "dist/JanePDFRenamer-mac-intel" .

echo "Building ${VERSION} for Linux (x64)..."
GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="${LDFLAGS}" -o "dist/JanePDFRenamer-linux" .

echo
ls -lh dist/
