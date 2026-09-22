#!/usr/bin/env bash
# Docker Compose Agent migration. No Coolify dependency.
set -u -o pipefail

COMPOSE_FILE=""; SERVICE=""; TARGET_IMAGE=""; PROJECT=""; CONFIG_PATH="/app/auto-discovery.json"; CONFIG_HOST_PATH=""
BACKUP_ROOT="/var/backups/komari-agent-docker-migration"; OVERRIDE_FILE=""; CLEANUP_ID=""; DRY_RUN=0
BACKUP_DIR=""; OVERRIDE_CREATED=0; CONFIG_CAPTURED=0; CONFIG_WAS_MOUNTED=0; CONFIG_BACKUP=""

usage() { cat <<'EOF'
Usage: sudo bash migrate-agent-docker.sh --compose-file FILE --service NAME --target-image IMAGE [options]

Migrates one Docker Compose Agent service without requiring Coolify. It records
the old image/configuration, preserves an existing auto-discovery credential
file, recreates only that Agent service, and rolls back if it cannot run.

Required:
  --compose-file FILE       Existing Docker Compose file
  --service NAME            Agent service in that Compose project
  --target-image IMAGE      New Komari Probe Agent image (prefer a digest)

Options:
  --project NAME            Compose project name, if not inferred
  --config-path PATH        Credential file in container; default: /app/auto-discovery.json
  --config-host-path PATH   Where a previously container-local credential becomes persistent
  --backup-root PATH        Default: /var/backups/komari-agent-docker-migration
  --override-file FILE      Persistent image/config override beside Compose file by default
  --dry-run                 Validate and print the plan only
  --cleanup-backup ID       Explicitly delete one completed backup
EOF
}
log() { printf '[komari-agent-docker] %s\n' "$*"; }
die() { printf '[komari-agent-docker] ERROR: %s\n' "$*" >&2; exit 1; }
need() { command -v "$1" >/dev/null 2>&1 || die "Missing required command: $1"; }
compose() { if [ -n "$PROJECT" ]; then docker compose -p "$PROJECT" -f "$COMPOSE_FILE" "$@"; else docker compose -f "$COMPOSE_FILE" "$@"; fi; }
compose_target() { if [ -n "$PROJECT" ]; then docker compose -p "$PROJECT" -f "$COMPOSE_FILE" -f "$OVERRIDE_FILE" "$@"; else docker compose -f "$COMPOSE_FILE" -f "$OVERRIDE_FILE" "$@"; fi; }
cleanup_backup() { local target="$BACKUP_ROOT/$CLEANUP_ID" root resolved; [[ "$CLEANUP_ID" =~ ^[0-9]{8}T[0-9]{6}Z$ ]] || die "Backup ID must look like 20260922T120000Z."; [ -d "$target" ] || die "Backup does not exist: $target"; root=$(realpath -m "$BACKUP_ROOT"); resolved=$(realpath -m "$target"); [[ "$resolved" == "$root"/* ]] || die "Refusing to delete outside backup root."; rm -rf -- "$resolved"; log "Deleted explicitly selected backup: $resolved"; }
config_is_mounted() { local cid=$1 target=$2 type name source destination; while IFS='|' read -r type name source destination; do [[ "$target" == "$destination" || "$target" == "$destination"/* ]] && return 0; done < <(docker inspect --format '{{range .Mounts}}{{printf "%s|%s|%s|%s\n" .Type .Name .Source .Destination}}{{end}}' "$cid"); return 1; }
capture_config() { local cid=$1 capture_dir="$BACKUP_DIR/config-capture"; mkdir -p "$capture_dir" || return 1; CONFIG_BACKUP="$capture_dir/$(basename "$CONFIG_PATH")"; docker cp "$cid:$CONFIG_PATH" "$capture_dir/" >/dev/null 2>&1 && [ -f "$CONFIG_BACKUP" ] && CONFIG_CAPTURED=1 || true; }
restore_config_into() { local cid=$1; [ "$CONFIG_CAPTURED" -eq 1 ] || return 0; docker cp "$CONFIG_BACKUP" "$cid:$CONFIG_PATH" >/dev/null 2>&1 || log "Could not restore credential file into rollback container."; }
rollback() { log "Rolling back Agent image and configuration..."; compose_target stop "$SERVICE" >/dev/null 2>&1 || true; [ "$OVERRIDE_CREATED" -eq 1 ] && rm -f -- "$OVERRIDE_FILE"; compose up -d --no-build "$SERVICE" || { log "Could not recreate old Agent service."; return; }; local cid; cid=$(compose ps -q "$SERVICE"); [ -n "$cid" ] && restore_config_into "$cid"; compose restart "$SERVICE" >/dev/null 2>&1 || true; }
abort() { log "Migration failed: $1"; rollback; exit 1; }

while [ "$#" -gt 0 ]; do case "$1" in --compose-file) COMPOSE_FILE=$2; shift;; --service) SERVICE=$2; shift;; --target-image) TARGET_IMAGE=$2; shift;; --project) PROJECT=$2; shift;; --config-path) CONFIG_PATH=$2; shift;; --config-host-path) CONFIG_HOST_PATH=$2; shift;; --backup-root) BACKUP_ROOT=$2; shift;; --override-file) OVERRIDE_FILE=$2; shift;; --dry-run) DRY_RUN=1;; --cleanup-backup) CLEANUP_ID=$2; shift;; -h|--help) usage; exit 0;; *) die "Unknown option: $1";; esac; shift; done
[ "${EUID:-$(id -u)}" -eq 0 ] || die "Run as root (sudo)."; need docker; need realpath
[ -n "$CLEANUP_ID" ] && { cleanup_backup; exit 0; }
[ -n "$COMPOSE_FILE" ] && [ -f "$COMPOSE_FILE" ] || die "--compose-file must name an existing file."; [ -n "$SERVICE" ] || die "--service is required."; [ -n "$TARGET_IMAGE" ] || die "--target-image is required."
compose config --services | grep -Fx "$SERVICE" >/dev/null || die "Service not found in Compose file: $SERVICE"; CID=$(compose ps -q "$SERVICE"); [ -n "$CID" ] || die "Service is not running: $SERVICE"
BACKUP_ID=$(date -u +%Y%m%dT%H%M%SZ); BACKUP_DIR="$BACKUP_ROOT/$BACKUP_ID"; OVERRIDE_FILE=${OVERRIDE_FILE:-"$(dirname "$COMPOSE_FILE")/.komari-probe-${SERVICE}.override.yml"}; CONFIG_HOST_PATH=${CONFIG_HOST_PATH:-"$(dirname "$COMPOSE_FILE")/.komari-probe-agent-data/$SERVICE/$(basename "$CONFIG_PATH")"}
[ ! -e "$OVERRIDE_FILE" ] || die "Override already exists: $OVERRIDE_FILE. Use that Compose configuration or choose --override-file."
config_is_mounted "$CID" "$CONFIG_PATH" && CONFIG_WAS_MOUNTED=1 || true
log "Plan: service=$SERVICE; old-image=$(docker inspect --format '{{.Config.Image}}' "$CID"); target=$TARGET_IMAGE; config=$CONFIG_PATH; backup=$BACKUP_DIR"
[ "$DRY_RUN" -eq 0 ] || { log "Dry run finished; no changes were made."; exit 0; }
mkdir -p "$BACKUP_DIR" || die "Cannot create backup directory."; cp -a "$COMPOSE_FILE" "$BACKUP_DIR/compose.before.yml" || die "Cannot back up Compose file."; docker inspect "$CID" > "$BACKUP_DIR/container.inspect.json" || die "Cannot record container configuration."; capture_config "$CID" || die "Cannot prepare credential backup directory."; printf 'captured=%s\nmounted=%s\npath=%s\n' "$CONFIG_CAPTURED" "$CONFIG_WAS_MOUNTED" "$CONFIG_PATH" > "$BACKUP_DIR/config-state.txt"
docker pull "$TARGET_IMAGE" || die "Could not pull target image; old Agent remains running."
if [ "$CONFIG_CAPTURED" -eq 1 ]; then mkdir -p "$(dirname "$CONFIG_HOST_PATH")" || die "Cannot create persistent credential directory."; cp -a "$CONFIG_BACKUP" "$CONFIG_HOST_PATH" || die "Cannot preserve existing credential file."; printf 'services:\n  %s:\n    image: %s\n    volumes:\n      - %s:%s\n' "$SERVICE" "$TARGET_IMAGE" "$CONFIG_HOST_PATH" "$CONFIG_PATH" > "$OVERRIDE_FILE"; else printf 'services:\n  %s:\n    image: %s\n' "$SERVICE" "$TARGET_IMAGE" > "$OVERRIDE_FILE"; fi; OVERRIDE_CREATED=1
compose_target up -d --no-build "$SERVICE" || abort "Compose could not recreate the target Agent service"; sleep 3; CID=$(compose_target ps -q "$SERVICE"); [ -n "$CID" ] || abort "target container was not created"; [ "$(docker inspect --format '{{.State.Running}}' "$CID")" = true ] || abort "target Agent container is not running"
log "Agent migration verified. Keep $OVERRIDE_FILE for future Compose commands. Backup retained at $BACKUP_DIR"
