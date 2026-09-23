#!/bin/bash
# Verify the flattened release assets consumed by installers and Docker builds.
set -euo pipefail

BUILD_DIR="${1:-build}"
CHECKSUM_FILE="${BUILD_DIR}/checksums.txt"

expected_assets=(
  sonar-agent-darwin-amd64
  sonar-agent-darwin-arm64
  sonar-agent-freebsd-386
  sonar-agent-freebsd-amd64
  sonar-agent-freebsd-arm
  sonar-agent-freebsd-arm64
  sonar-agent-linux-386
  sonar-agent-linux-amd64
  sonar-agent-linux-arm
  sonar-agent-linux-arm64
  sonar-agent-linux-loong64
  sonar-agent-windows-386.exe
  sonar-agent-windows-amd64.exe
  sonar-agent-windows-arm64.exe
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
actual_list=$(find "$BUILD_DIR" -maxdepth 1 -type f -name 'sonar-agent-*' -printf '%f\n' | sort)
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
