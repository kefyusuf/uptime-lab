#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)"
DOCKER_BIN="${DOCKER_BIN:-docker}"
COMPOSE_FILE="${COMPOSE_FILE:-$ROOT/compose.yaml}"
export COMPOSE_PROJECT_NAME="${COMPOSE_PROJECT_NAME:-uptime-lab-smoke-$$}"

compose() { "$DOCKER_BIN" compose -f "$COMPOSE_FILE" "$@"; }
cleanup() { compose down -v --remove-orphans >/dev/null 2>&1 || true; }

trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

VERSION="$(compose version --short)"
VERSION="${VERSION#v}"
IFS=. read -r MAJOR MINOR PATCH <<EOF
$VERSION
EOF
MAJOR="${MAJOR:-0}"
MINOR="${MINOR:-0}"
if (( MAJOR < 2 || (MAJOR == 2 && MINOR < 22) )); then
  printf 'Docker Compose >= 2.22.0 is required; found %s\n' "$VERSION" >&2
  exit 1
fi

assert_service_healthy() {
  local service="$1"
  local container_id
  local health

  container_id="$(compose ps -q "$service")"
  [[ -n "$container_id" ]] || {
    printf 'Service %s has no running container\n' "$service" >&2
    return 1
  }

  health="$("$DOCKER_BIN" inspect --format '{{.State.Health.Status}}' "$container_id")"
  [[ "$health" == "healthy" ]] || {
    printf 'Service %s health is %s, want healthy\n' "$service" "$health" >&2
    return 1
  }

  printf 'Service %s health=%s\n' "$service" "$health"
}

assert_api_operational_health() {
  local livez
  local readyz

  assert_service_healthy api

  livez="$(compose exec -T api wget -q -O - http://127.0.0.1:8080/livez)"
  [[ "$livez" == "ok" ]] || {
    printf 'API /livez returned %q, want ok\n' "$livez" >&2
    return 1
  }

  readyz="$(compose exec -T api wget -q -O - http://127.0.0.1:8080/readyz)"
  [[ "$readyz" == "ok" ]] || {
    printf 'API /readyz returned %q, want ok\n' "$readyz" >&2
    return 1
  }

  printf 'API probes: livez=%s readyz=%s\n' "$livez" "$readyz"
  assert_service_healthy checker
}

cleanup
compose config --quiet
"$ROOT/scripts/ci/check-local-dev.sh" "$ROOT"
compose build api
compose build web checker
compose up -d --wait --wait-timeout 60
compose ps
assert_api_operational_health

PROBE_TABLE="public.__uptime_lab_local_dev_probe"

compose exec -T db sh -lc \
  'psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" -c "$1"' \
  sh "CREATE TABLE $PROBE_TABLE (id integer PRIMARY KEY); INSERT INTO $PROBE_TABLE (id) VALUES (1);"

compose down
compose up -d --wait --wait-timeout 60
PERSISTED="$(compose exec -T db sh -lc \
  'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Atc "$1"' \
  sh "SELECT to_regclass('$PROBE_TABLE') IS NOT NULL;")"
test "$PERSISTED" = "t"

compose down -v --remove-orphans
compose up -d --wait --wait-timeout 60
RESET="$(compose exec -T db sh -lc \
  'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Atc "$1"' \
  sh "SELECT to_regclass('$PROBE_TABLE') IS NULL;")"
test "$RESET" = "t"

cleanup
trap - EXIT INT TERM
