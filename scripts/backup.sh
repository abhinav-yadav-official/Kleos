#!/usr/bin/env bash
# Daily Postgres dump for Kleos. Runs on the VPS via root cron at 04:15 UTC.
# Output: /opt/kleos/backups/kleos-YYYY-MM-DD.sql.gz. Retains 14 days.
set -euo pipefail

APP_DIR=${APP_DIR:-/opt/kleos}
BACKUP_DIR="$APP_DIR/backups"
RETENTION_DAYS=${RETENTION_DAYS:-14}
COMPOSE_FILE="$APP_DIR/deploy/docker-compose.yml"

mkdir -p "$BACKUP_DIR"

STAMP=$(date -u +%Y-%m-%d_%H%M%S)
OUT="$BACKUP_DIR/kleos-${STAMP}.sql.gz"

# Resolve postgres container via compose so the script works if names change.
PG_CONTAINER=$(docker compose -f "$COMPOSE_FILE" ps -q postgres)
if [[ -z "$PG_CONTAINER" ]]; then
  echo "postgres container not running" >&2
  exit 1
fi

docker exec -e PGPASSWORD="${PGPASSWORD:-kleos}" "$PG_CONTAINER" \
  pg_dump -U "${PGUSER:-kleos}" -d "${PGDATABASE:-kleos}" --no-owner --no-privileges \
  | gzip -9 > "$OUT.tmp"
mv "$OUT.tmp" "$OUT"

# Retention: delete dumps older than RETENTION_DAYS days
find "$BACKUP_DIR" -maxdepth 1 -name 'kleos-*.sql.gz' -type f -mtime "+$RETENTION_DAYS" -delete

ls -la "$OUT"
