#!/bin/bash
# goreleaser's "binary" archive format doesn't move the built binary into a
# flat file - it only records a friendly name in build/artifacts.json,
# leaving the file under build/<id>_<goos>_<goarch>[_<variant>]/. This copies
# each one out to the flat komari-agent-<os>-<arch>[.exe] layout the rest of
# the project (Dockerfile, install.sh/ps1, CI) expects, then removes the
# per-target directories and goreleaser's own metadata files.
set -e

BUILD_DIR="${1:-build}"

while IFS=' ' read -r name path; do
  cp "$path" "$BUILD_DIR/$name"
done < <(grep -oE '"name":"komari-agent-[^"]+","path":"[^"]+"' "$BUILD_DIR/artifacts.json" | sed -E 's/"name":"([^"]+)","path":"([^"]+)"/\1 \2/')

find "$BUILD_DIR" -mindepth 1 -maxdepth 1 -type d -name "komari-agent_*" -exec rm -rf {} +
rm -f "$BUILD_DIR/artifacts.json" "$BUILD_DIR/config.yaml" "$BUILD_DIR/metadata.json" "$BUILD_DIR/checksums.txt"

(
  cd "$BUILD_DIR"
  sha256sum komari-agent-* > checksums.txt
)
