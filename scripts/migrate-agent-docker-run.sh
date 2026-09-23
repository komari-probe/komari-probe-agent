#!/usr/bin/env bash
# Migrate a single docker run Agent container without Compose or Coolify.
set -u -o pipefail
CONTAINER=""; TARGET_IMAGE=""; CONFIG_PATH="/app/auto-discovery.json"; BACKUP_ROOT="/var/backups/sonar-agent-docker-run"; BACKUP_DIR=""; OLD_NAME=""; CONFIG_CAPTURED=0; CONFIG_MOUNTED=0; CONFIG_BACKUP=""; CONFIG_HOST_PATH=""; DRY_RUN=0
die(){ echo "[sonar-agent-docker-run] ERROR: $*" >&2; exit 1; }; log(){ echo "[sonar-agent-docker-run] $*"; }
usage(){ cat <<'EOF'
Usage: sudo bash migrate-agent-docker-run.sh --container NAME --target-image IMAGE [options]

Records the old docker run container configuration, preserves the Agent
auto-discovery credential, recreates it with the target image, and rolls back
the old container if the new one cannot run. No Compose or Coolify is needed.

Options:
  --config-path PATH       Credential file in the container (default /app/auto-discovery.json)
  --config-host-path PATH  Persistent host path for a container-local credential
  --backup-root PATH       Default /var/backups/sonar-agent-docker-run
  --dry-run                Validate and print the plan without pulling or changing anything
EOF
}
while [ $# -gt 0 ]; do case "$1" in --container) CONTAINER=$2;shift;;--target-image) TARGET_IMAGE=$2;shift;;--config-path) CONFIG_PATH=$2;shift;;--config-host-path) CONFIG_HOST_PATH=$2;shift;;--backup-root) BACKUP_ROOT=$2;shift;;--dry-run) DRY_RUN=1;;-h|--help) usage;exit;;*) die "Unknown option: $1";;esac;shift;done
[ "${EUID:-$(id -u)}" -eq 0 ] || die "Run as root."; command -v docker >/dev/null || die "docker is required"; command -v python3 >/dev/null || die "python3 is required"; command -v curl >/dev/null || die "curl is required"
[ -n "$CONTAINER" ] && [ -n "$TARGET_IMAGE" ] || { usage; exit 1; }; docker inspect "$CONTAINER" >/dev/null 2>&1 || die "Container not found: $CONTAINER"
CID=$(docker inspect -f '{{.Id}}' "$CONTAINER"); INSPECT_FILE=/tmp/sonar-agent-inspect.$$; docker inspect "$CONTAINER" > "$INSPECT_FILE"; trap 'rm -f "$INSPECT_FILE"' EXIT
if python3 - "$CONFIG_PATH" "$INSPECT_FILE" <<'PY'
import json, sys
path = sys.argv[1]
for mount in json.load(open(sys.argv[2]))[0].get("Mounts", []):
    target = mount.get("Destination", "")
    if path == target or path.startswith(target + "/"):
        raise SystemExit(0)
raise SystemExit(1)
PY
then CONFIG_MOUNTED=1; fi
CONFIG_HOST_PATH=${CONFIG_HOST_PATH:-"$BACKUP_ROOT/persistent/$CONTAINER/$(basename "$CONFIG_PATH")"}
if [ "$DRY_RUN" -eq 1 ]; then
    log "Plan: container=$CONTAINER; old-image=$(docker inspect --format '{{.Config.Image}}' "$CONTAINER"); target=$TARGET_IMAGE; config=$CONFIG_PATH; config-mounted=$CONFIG_MOUNTED; credential-host-path=$CONFIG_HOST_PATH; backup-root=$BACKUP_ROOT"
    log "Dry run finished; no changes were made and the target image was not pulled."
    exit 0
fi
ID=$(date -u +%Y%m%dT%H%M%SZ); BACKUP_DIR="$BACKUP_ROOT/$ID"; mkdir -p "$BACKUP_DIR"; cp "$INSPECT_FILE" "$BACKUP_DIR/container.inspect.json"; CONFIG_BACKUP="$BACKUP_DIR/$(basename "$CONFIG_PATH")"
docker cp "$CID:$CONFIG_PATH" "$CONFIG_BACKUP" >/dev/null 2>&1 && [ -f "$CONFIG_BACKUP" ] && CONFIG_CAPTURED=1 || true
docker pull "$TARGET_IMAGE" || die "Target image pull failed; old container is unchanged."
OLD_NAME="${CONTAINER}.pre-migration-${ID}"; docker stop "$CONTAINER"; docker rename "$CONTAINER" "$OLD_NAME"
payload(){ python3 - "$TARGET_IMAGE" "$CONFIG_HOST_PATH:$CONFIG_PATH:ro" "$CONFIG_CAPTURED" "$CONFIG_MOUNTED" "$INSPECT_FILE" <<'PY'
import json, sys
image, bind, captured, mounted, inspect = sys.argv[1:]
x = json.load(open(inspect))[0]
c, host = x["Config"], x.get("HostConfig", {})
if captured == "1" and mounted == "0":
    host["Binds"] = (host.get("Binds") or []) + [bind]
allowed = {"Aliases", "Links", "IPAMConfig", "MacAddress", "DriverOpts"}
endpoints = {name: {k:v for k,v in value.items() if k in allowed} for name,value in x.get("NetworkSettings", {}).get("Networks", {}).items()}
print(json.dumps({"Image":image, "Hostname":c.get("Hostname"), "User":c.get("User"), "Env":c.get("Env"), "Cmd":c.get("Cmd"), "Entrypoint":c.get("Entrypoint"), "WorkingDir":c.get("WorkingDir"), "Labels":c.get("Labels"), "ExposedPorts":c.get("ExposedPorts"), "HostConfig":host, "NetworkingConfig":{"EndpointsConfig":endpoints}}))
PY
}
rollback(){ log "Rolling back..."; docker rm -f "$CONTAINER" >/dev/null 2>&1 || true; docker rename "$OLD_NAME" "$CONTAINER" >/dev/null 2>&1 || true; docker start "$CONTAINER" >/dev/null || true; }
if [ "$CONFIG_CAPTURED" -eq 1 ] && [ "$CONFIG_MOUNTED" -eq 0 ]; then mkdir -p "$(dirname "$CONFIG_HOST_PATH")" || { rollback; die "Cannot create persistent credential directory"; }; cp -a "$CONFIG_BACKUP" "$CONFIG_HOST_PATH" || { rollback; die "Cannot preserve credential file"; }; fi
curl --unix-socket /var/run/docker.sock -fsS -H 'Content-Type: application/json' -X POST "http://localhost/containers/create?name=$CONTAINER" --data-binary @<(payload) >/dev/null || { rollback; die "Could not create target container"; }
docker start "$CONTAINER" >/dev/null || { rollback; die "Could not start target container"; }; sleep 3; docker inspect -f '{{.State.Running}}' "$CONTAINER" | grep -qx true || { rollback; die "Target Agent is not running"; }
log "Migration verified. Old container retained as $OLD_NAME; backup retained at $BACKUP_DIR"
