#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)"
COMPOSE_FILE="$ROOT/compose.yaml"

export COMPOSE_PROJECT_NAME="uptime-lab-go-postgres-${GITHUB_RUN_ID:-local}-$$"
export POSTGRES_DB="uptime_lab_test"
export POSTGRES_USER="uptime_lab_test"
export POSTGRES_PASSWORD="uptime_lab_test_password"

cleanup() {
  docker compose -f "$COMPOSE_FILE" down -v --remove-orphans >/dev/null 2>&1 || true
}
trap cleanup EXIT

cleanup

docker compose -f "$COMPOSE_FILE" up -d db --wait

docker run --rm \
  --network "${COMPOSE_PROJECT_NAME}_default" \
  -e PGHOST=db \
  -e PGPORT=5432 \
  -e PGDATABASE="$POSTGRES_DB" \
  -e PGUSER="$POSTGRES_USER" \
  -e PGPASSWORD="$POSTGRES_PASSWORD" \
  -e PGSSLMODE=disable \
  -e GOMODCACHE=/tmp/go-mod-cache \
  -e GOCACHE=/tmp/go-build-cache \
  -v "$ROOT/apps/api:/workspace:ro" \
  -w /workspace \
  golang:1.27.1-alpine3.24 \
  sh -euc 'go test -tags=integration ./migrations'
