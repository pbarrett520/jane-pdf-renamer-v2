#!/usr/bin/env bash
# Cross-compile Jane PDF Renamer for all distribution targets.
# Run from anywhere; outputs land in go-app/dist/.
set -euo pipefail
cd "$(dirname "$0")"

mkdir -p dist

echo "Building Windows (x64)..."
GOOS=windows GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o "dist/JanePDFRenamer.exe" .

echo "Building macOS (Apple Silicon)..."
GOOS=darwin GOARCH=arm64 go build -trimpath -ldflags="-s -w" -o "dist/JanePDFRenamer-mac-applesilicon" .

echo "Building macOS (Intel)..."
GOOS=darwin GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o "dist/JanePDFRenamer-mac-intel" .

echo "Building Linux (x64)..."
GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o "dist/JanePDFRenamer-linux" .

echo
ls -lh dist/
