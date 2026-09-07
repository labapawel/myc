#!/bin/bash
set -e

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$DIR"

echo "=== Kompilacja myc (2 wersje: CLI i Okienkowa dla Windows) ==="

# Linux 64-bit
echo "--> Budowanie myc (Linux CLI)..."
GOOS=linux GOARCH=amd64 go build -o myc ./cmd/myc

echo "--> Budowanie myc-gui (Linux Desktop)..."
GOOS=linux GOARCH=amd64 go build -o myc-gui ./cmd/myc-gui

# Windows 64-bit
echo "--> Budowanie myc.exe (Windows CLI)..."
GOOS=windows GOARCH=amd64 go build -o myc.exe ./cmd/myc

echo "--> Budowanie myc-gui.exe (Windows Okienkowa -H=windowsgui)..."
GOOS=windows GOARCH=amd64 go build -ldflags="-H=windowsgui" -o myc-gui.exe ./cmd/myc-gui

# Windows 32-bit
echo "--> Budowanie myc-386.exe (Windows 32-bit CLI)..."
GOOS=windows GOARCH=386 go build -o myc-386.exe ./cmd/myc

echo "--> Budowanie myc-gui-386.exe (Windows 32-bit Okienkowa)..."
GOOS=windows GOARCH=386 go build -ldflags="-H=windowsgui" -o myc-gui-386.exe ./cmd/myc-gui

echo "=== Zakończono pomyślnie! Utworzone pliki: ==="
ls -lh myc myc.exe myc-gui myc-gui.exe myc-386.exe myc-gui-386.exe
