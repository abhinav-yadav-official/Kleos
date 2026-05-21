#!/usr/bin/env bash
set -euo pipefail

APP_DIR=/opt/kleos
TAG=${TAG:?TAG is required}

cd "$APP_DIR"

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

docker image prune -f
echo "deploy ok: TAG=${TAG}"
