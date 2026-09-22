#!/usr/bin/env bash
# Migrate one host's Agent to Komari Probe. Server hosts use the Server repo script.
set -u -o pipefail

REPO="komari-probe/komari-probe-agent"; TAG="latest"; SERVICE="komari-agent"; BINARY=""; CONFIG=""
BACKUP_ROOT="/var/backups/komari-agent-migration"; BINARY_URL=""; CHECKSUM_URL=""; CLEANUP_ID=""; DRY_RUN=0
TMP_DIR=""; BACKUP_DIR=""

usage() { cat <<'EOF'
Usage: sudo bash migrate-agent-host.sh [options]

Backs up this host's old Agent binary and local connection configuration,
installs a SHA-256-verified Komari Probe Agent, and rolls the binary/config back
if the replacement cannot start. It does not touch any Server.

Options:
  --service NAME           Default: komari-agent
  --binary PATH            Old Agent executable; auto-detected by default
  --config PATH            Additional Agent config file to preserve
  --backup-root PATH       Default: /var/backups/komari-agent-migration
  --tag TAG                Default: latest stable release
  --url URL                Override binary URL (controlled testing)
  --checksum-url URL       Override checksums.txt URL
  --dry-run                Print the plan without changing this host
  --cleanup-backup ID      Explicitly delete one completed backup
  -h, --help               Show this help
EOF
}
log() { printf '[komari-agent-migrate] %s\n' "$*"; }
die() { printf '[komari-agent-migrate] ERROR: %s\n' "$*" >&2; exit 1; }
need() { command -v "$1" >/dev/null 2>&1 || die "Missing required command: $1"; }
arch() { case "$(uname -m)" in x86_64|amd64) echo amd64;; aarch64|arm64) echo arm64;; i386|i686) echo 386;; riscv64) echo riscv64;; loongarch64|loong64) echo loong64;; *) die "Unsupported CPU architecture: $(uname -m)";; esac; }
sha256() { if command -v sha256sum >/dev/null 2>&1; then sha256sum "$1" | awk '{print $1}'; else shasum -a 256 "$1" | awk '{print $1}'; fi; }
release_base() { [ "$TAG" = latest ] && printf 'https://github.com/%s/releases/latest/download' "$REPO" || printf 'https://github.com/%s/releases/download/%s' "$REPO" "$TAG"; }
detect_binary() { local p; for p in /opt/komari/agent /opt/komari/komari-agent /usr/local/bin/komari-agent; do [ -x "$p" ] && { printf '%s' "$p"; return; }; done; return 1; }
backup_unit() { local fragment; mkdir -p "$BACKUP_DIR/systemd" || return 1; systemctl cat "$SERVICE" > "$BACKUP_DIR/systemd/$SERVICE.service.rendered" 2>/dev/null || true; fragment=$(systemctl show -p FragmentPath --value "$SERVICE" 2>/dev/null || true); [ -n "$fragment" ] && [ -f "$fragment" ] && cp -a "$fragment" "$BACKUP_DIR/systemd/$(basename "$fragment")"; return 0; }
backup() { local f; BACKUP_DIR="$BACKUP_ROOT/$BACKUP_ID"; mkdir -p "$BACKUP_DIR" || return 1; cp -a "$BINARY" "$BACKUP_DIR/$(basename "$BINARY").old" || return 1; backup_unit || return 1; for f in "$CONFIG" "$(dirname "$BINARY")/auto-discovery.json" "$(dirname "$BINARY")/config.json"; do if [ -n "$f" ] && [ -f "$f" ]; then cp -a "$f" "$BACKUP_DIR/$(basename "$f")" || return 1; fi; done; }
restore() { local old="$BACKUP_DIR/$(basename "$BINARY").old" f; [ -f "$old" ] || return 0; log "Rolling back Agent binary and configuration..."; systemctl stop "$SERVICE" >/dev/null 2>&1 || true; install -m 0755 "$old" "$BINARY" || true; for f in "$CONFIG" "$(dirname "$BINARY")/auto-discovery.json" "$(dirname "$BINARY")/config.json"; do [ -n "$f" ] && [ -f "$BACKUP_DIR/$(basename "$f")" ] && cp -a "$BACKUP_DIR/$(basename "$f")" "$f"; done; systemctl daemon-reload; systemctl start "$SERVICE" || log "Rollback could not restart $SERVICE; inspect systemctl status."; }
abort() { log "Migration failed: $1"; restore; exit 1; }
cleanup() { local target="$BACKUP_ROOT/$CLEANUP_ID" root target_real; [[ "$CLEANUP_ID" =~ ^[0-9]{8}T[0-9]{6}Z$ ]] || die "Backup ID must look like 20260922T120000Z."; [ -d "$target" ] || die "Backup does not exist: $target"; root=$(realpath -m "$BACKUP_ROOT"); target_real=$(realpath -m "$target"); [[ "$target_real" == "$root"/* ]] || die "Refusing to delete outside backup root."; rm -rf -- "$target_real"; log "Deleted explicitly selected backup: $target_real"; }
download() { local manifest="$TMP_DIR/checksums.txt" expected actual; curl -fsSL --connect-timeout 15 --retry 3 -o "$manifest" "$CHECKSUM_URL" || return 1; expected=$(awk -v n="$ASSET" '$2 == n || $2 == "*" n { print $1 }' "$manifest"); [ "$(printf '%s\n' "$expected" | sed '/^$/d' | wc -l | tr -d ' ')" = 1 ] || return 1; curl -fsSL --connect-timeout 15 --retry 3 -o "$TMP_DIR/$ASSET" "$BINARY_URL" || return 1; actual=$(sha256 "$TMP_DIR/$ASSET"); [ "$actual" = "$expected" ] || return 1; chmod 0755 "$TMP_DIR/$ASSET"; }

while [ "$#" -gt 0 ]; do case "$1" in --service) SERVICE=$2; shift;; --binary) BINARY=$2; shift;; --config) CONFIG=$2; shift;; --backup-root) BACKUP_ROOT=$2; shift;; --tag) TAG=$2; shift;; --url) BINARY_URL=$2; shift;; --checksum-url) CHECKSUM_URL=$2; shift;; --dry-run) DRY_RUN=1;; --cleanup-backup) CLEANUP_ID=$2; shift;; -h|--help) usage; exit 0;; *) die "Unknown option: $1";; esac; shift; done
[ "${EUID:-$(id -u)}" -eq 0 ] || die "Run as root (sudo)."; need curl; need systemctl; need realpath; need awk; command -v sha256sum >/dev/null 2>&1 || need shasum
[ -n "$CLEANUP_ID" ] && { cleanup; exit 0; }
BINARY=${BINARY:-$(detect_binary || true)}; [ -n "$BINARY" ] || die "Could not detect old Agent binary; pass --binary PATH."; [ -x "$BINARY" ] || die "Old Agent binary not found: $BINARY"; systemctl status "$SERVICE" >/dev/null 2>&1 || die "Agent service not found: $SERVICE"
ASSET="komari-agent-linux-$(arch)"; BASE=$(release_base); BINARY_URL=${BINARY_URL:-"$BASE/$ASSET"}; CHECKSUM_URL=${CHECKSUM_URL:-"$BASE/checksums.txt"}; BACKUP_ID=$(date -u +%Y%m%dT%H%M%SZ)
log "Plan: Agent $SERVICE ($BINARY); backup=$BACKUP_ROOT/$BACKUP_ID; asset=$ASSET"; [ "$DRY_RUN" -eq 0 ] || { log "Dry run finished; no changes were made."; exit 0; }
TMP_DIR=$(mktemp -d); trap 'rm -rf "$TMP_DIR"' EXIT
backup || die "Backup failed; old Agent remains untouched."; download || die "Download or checksum verification failed; old Agent remains untouched."
systemctl stop "$SERVICE" || abort "could not stop old Agent"; install -m 0755 "$TMP_DIR/$ASSET" "$BINARY" || abort "could not install new binary"; systemctl start "$SERVICE" || abort "new Agent did not start"; sleep 2; systemctl is-active --quiet "$SERVICE" || abort "new Agent is not active"
log "Agent service migration verified. Existing Server URL/token/configuration were preserved. Backup retained at $BACKUP_DIR"
