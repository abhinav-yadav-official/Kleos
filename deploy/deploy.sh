#!/usr/bin/env bash
set -euo pipefail

APP_DIR=/opt/kleos
TAG=${TAG:?TAG is required}

cd "$APP_DIR"

mkdir -p "$APP_DIR/data/resumes" "$APP_DIR/logs"
docker run --rm \
  -v "$APP_DIR/data:/data" \
  -v "$APP_DIR/logs:/logs" \
  alpine:3.20 sh -c 'chown -R 10001:10001 /data /logs'

docker build \
  -t "kleos/api:${TAG}" \
  -f "$APP_DIR/deploy/Dockerfile.runtime" \
  --build-arg BINARY=api \
  "$APP_DIR/bin.new"

docker compose -f "$APP_DIR/deploy/docker-compose.yml" --env-file "$APP_DIR/.env" up -d --wait postgres redis

docker run --rm \
  --env-file "$APP_DIR/.env" \
  --network kleos_default \
  -v "$APP_DIR/migrations:/migrations:ro" \
  -v "$APP_DIR/bin.new:/bin:ro" \
  --entrypoint /bin/migrate \
  alpine:3.20 up

TAG="$TAG" docker compose -f "$APP_DIR/deploy/docker-compose.yml" --env-file "$APP_DIR/.env" up -d api

rm -rf "$APP_DIR/bin"
mv "$APP_DIR/bin.new" "$APP_DIR/bin"
mkdir -p "$APP_DIR/bin.new"

# Make ops scripts executable.
if [[ -d "$APP_DIR/scripts" ]]; then
  chmod +x "$APP_DIR/scripts"/*.sh 2>/dev/null || true
fi

# Logrotate config + daily backup cron are installed once during VPS setup
# (see plan §16). deploy.sh stays idempotent without root privileges.

docker image prune -f
echo "deploy ok: TAG=${TAG}"
