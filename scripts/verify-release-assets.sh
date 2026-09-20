#!/bin/bash
# Verify the flattened release assets consumed by installers and Docker builds.
set -euo pipefail

BUILD_DIR="${1:-build}"
CHECKSUM_FILE="${BUILD_DIR}/checksums.txt"

expected_assets=(
  komari-agent-darwin-amd64
  komari-agent-darwin-arm64
  komari-agent-freebsd-386
  komari-agent-freebsd-amd64
  komari-agent-freebsd-arm
  komari-agent-freebsd-arm64
  komari-agent-linux-386
  komari-agent-linux-amd64
  komari-agent-linux-arm
  komari-agent-linux-arm64
  komari-agent-linux-loong64
  komari-agent-windows-386.exe
  komari-agent-windows-amd64.exe
  komari-agent-windows-arm64.exe
)

if [ ! -d "$BUILD_DIR" ]; then
  echo "Release asset directory does not exist: $BUILD_DIR" >&2
  exit 1
fi
if [ ! -f "$CHECKSUM_FILE" ]; then
  echo "Release checksum manifest does not exist: $CHECKSUM_FILE" >&2
  exit 1
fi

expected_list=$(printf '%s\n' "${expected_assets[@]}" | sort)
actual_list=$(find "$BUILD_DIR" -maxdepth 1 -type f -name 'komari-agent-*' -printf '%f\n' | sort)
if [ "$actual_list" != "$expected_list" ]; then
  echo "Release binaries do not match the supported target matrix:" >&2
  diff -u <(printf '%s\n' "$expected_list") <(printf '%s\n' "$actual_list") || true
  exit 1
fi

manifest_list=$(awk '{print $2}' "$CHECKSUM_FILE" | sed 's/^\*//' | sort)
if [ "$manifest_list" != "$expected_list" ]; then
  echo "Checksum manifest does not match the supported target matrix:" >&2
  diff -u <(printf '%s\n' "$expected_list") <(printf '%s\n' "$manifest_list") || true
  exit 1
fi

(
  cd "$BUILD_DIR"
  sha256sum --check --strict checksums.txt
)
