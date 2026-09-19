#!/bin/bash
set -e

if ! command -v goreleaser >/dev/null 2>&1; then
  echo "goreleaser not found. Install it: https://goreleaser.com/install/"
  exit 1
fi

goreleaser release --snapshot --clean

bash "$(dirname "$0")/scripts/flatten-goreleaser-output.sh" build

echo
echo "Binaries are in the ./build directory."
