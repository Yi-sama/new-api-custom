#!/usr/bin/env bash
set -Eeuo pipefail

umask 077
PG_CONTAINER="sub2api-postgres"
PG_DATABASE="newapi"
PG_USER="sub2api"
BACKUP_DIR="/opt/new-api/backups"
LOCK_PATH="/run/lock/new-api-db-backup.lock"
RETENTION_DAYS=7

install -d -m 700 "$BACKUP_DIR" "$(dirname "$LOCK_PATH")"
exec 9>"$LOCK_PATH"
if ! flock -n 9; then
  echo "backup already running"
  exit 0
fi

stamp="$(date -u +%Y%m%d-%H%M%S)"
base="newapi-pg-${stamp}"
work_dir="$(mktemp -d "${BACKUP_DIR}/.work.XXXXXX")"
trap 'rm -rf -- "$work_dir"' EXIT

dump_path="$work_dir/${base}.dump"
docker exec "$PG_CONTAINER" pg_dump -U "$PG_USER" -d "$PG_DATABASE" \
  --format=custom --lock-wait-timeout=30s > "$dump_path"
# Read every archived data block, not just the table of contents.
docker exec -i "$PG_CONTAINER" pg_restore --exit-on-error --file=/dev/null < "$dump_path"
(cd "$work_dir" && sha256sum "${base}.dump") > "$work_dir/${base}.dump.sha256"
printf '{"engine":"postgresql","database":"%s","format":"pg_custom","created_at":"%s","archive_check":"ok","source_container":"%s"}\n' \
  "$PG_DATABASE" "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$PG_CONTAINER" > "$work_dir/${base}.json"

mv -- "$work_dir/${base}.dump.sha256" "$BACKUP_DIR/${base}.dump.sha256"
mv -- "$work_dir/${base}.json" "$BACKUP_DIR/${base}.json"
# Publish the dump last, so pullers never see an incomplete bundle.
mv -- "$dump_path" "$BACKUP_DIR/${base}.dump"
while IFS= read -r -d '' expired; do
  rm -f -- "$expired" "$expired.sha256" "${expired%.dump}.json"
done < <(find "$BACKUP_DIR" -maxdepth 1 -type f -name 'newapi-pg-*.dump' -mtime "+$RETENTION_DAYS" -print0)
echo "created $BACKUP_DIR/${base}.dump"
