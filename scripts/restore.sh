#!/usr/bin/env bash
# Restore a Kleos pg_dump.gz into a target Postgres container.
#
# Usage: ./restore.sh /path/to/kleos-YYYY-MM-DD.sql.gz [target-container]
#
# By default restores into the live `kleos-postgres-1` container, dropping and
# recreating the public schema first. For DR drills, pass a freshly started
# throwaway container as the second arg.
set -euo pipefail

DUMP=${1:-}
TARGET=${2:-kleos-postgres-1}
if [[ -z "$DUMP" || ! -f "$DUMP" ]]; then
  echo "usage: $0 <dump.sql.gz> [target-container]" >&2
  exit 1
fi

PGUSER=${PGUSER:-kleos}
PGDATABASE=${PGDATABASE:-kleos}

echo "Restoring $DUMP into container $TARGET (db=$PGDATABASE)…"

# Drop+recreate public schema so the restore is deterministic.
docker exec -i -e PGPASSWORD="${PGPASSWORD:-kleos}" "$TARGET" \
  psql -U "$PGUSER" -d "$PGDATABASE" -c \
  "DROP SCHEMA IF EXISTS public CASCADE; CREATE SCHEMA public; GRANT ALL ON SCHEMA public TO public;"

gunzip -c "$DUMP" | docker exec -i -e PGPASSWORD="${PGPASSWORD:-kleos}" "$TARGET" \
  psql -U "$PGUSER" -d "$PGDATABASE"

echo "Restore complete. Sanity check:"
docker exec -e PGPASSWORD="${PGPASSWORD:-kleos}" "$TARGET" \
  psql -U "$PGUSER" -d "$PGDATABASE" -c "\\dt" | head -40
