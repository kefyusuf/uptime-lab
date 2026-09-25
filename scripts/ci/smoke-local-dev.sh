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

wait_for_api_livez() {
  local attempt
  local livez=""

  for ((attempt = 1; attempt <= 20; attempt++)); do
    if livez="$(compose exec -T api wget -q -O - http://127.0.0.1:8080/livez 2>/dev/null)" && [[ "$livez" == "ok" ]]; then
      printf 'API /livez is available before schema readiness\n'
      return 0
    fi
    sleep 1
  done

  printf 'API /livez did not become available\n' >&2
  return 1
}

assert_api_unready_before_migration() {
  local readyz=""

  if readyz="$(compose exec -T api wget -q -O - http://127.0.0.1:8080/readyz 2>/dev/null)"; then
    printf 'API /readyz unexpectedly succeeded before migration: %q\n' "$readyz" >&2
    return 1
  fi

  printf 'API /readyz is correctly unavailable before migration\n'
}

assert_migration_metadata_absent() {
  local absent
  absent="$(compose exec -T db sh -lc \
    'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Atc "$1"' \
    sh "SELECT to_regclass('public.goose_db_version') IS NULL;")"
  [[ "$absent" == "t" ]] || {
    printf 'Goose metadata exists before explicit migration\n' >&2
    return 1
  }
  printf 'Migration metadata is absent before explicit migration\n'
}

apply_migrations() {
  compose exec -T api /usr/local/bin/uptime-lab-migrate up
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

json_id() {
  printf '%s\n' "$1" | sed -n 's/.*"id":"\([^"]*\)".*/\1/p'
}

json_target_url() {
  printf '%s\n' "$1" | sed -n 's/.*"targetUrl":"\([^"]*\)".*/\1/p'
}

json_created_at() {
  printf '%s\n' "$1" | sed -n 's/.*"createdAt":"\([^"]*\)".*/\1/p'
}

MONITOR_ID=""
MONITOR_TARGET="https://example.com/local-smoke"
MONITOR_CREATED_AT=""

create_monitor() {
  local response
  local target
  local created_at

  response="$(compose exec -T api wget -q -O - \
    --header='Content-Type: application/json' \
    --post-data='{"targetUrl":"https://example.com/local-smoke"}' \
    http://127.0.0.1:8080/monitors)"

  MONITOR_ID="$(json_id "$response")"
  target="$(json_target_url "$response")"
  MONITOR_CREATED_AT="$(json_created_at "$response")"

  [[ -n "$MONITOR_ID" ]] || {
    printf 'POST /monitors response has no id: %s\n' "$response" >&2
    return 1
  }
  [[ "$target" == "$MONITOR_TARGET" ]] || {
    printf 'POST /monitors targetUrl=%q, want %q\n' "$target" "$MONITOR_TARGET" >&2
    return 1
  }
  [[ -n "$MONITOR_CREATED_AT" ]] || {
    printf 'POST /monitors response has no createdAt: %s\n' "$response" >&2
    return 1
  }

  printf 'Created monitor id=%s\n' "$MONITOR_ID"
}

assert_monitor_get() {
  local response
  local id
  local target
  local created_at

  response="$(compose exec -T api wget -q -O - "http://127.0.0.1:8080/monitors/$MONITOR_ID")"
  id="$(json_id "$response")"
  target="$(json_target_url "$response")"
  created_at="$(json_created_at "$response")"

  [[ "$id" == "$MONITOR_ID" ]] || {
    printf 'GET monitor id=%q, want %q\n' "$id" "$MONITOR_ID" >&2
    return 1
  }
  [[ "$target" == "$MONITOR_TARGET" ]] || {
    printf 'GET monitor targetUrl=%q, want %q\n' "$target" "$MONITOR_TARGET" >&2
    return 1
  }
  [[ "$created_at" == "$MONITOR_CREATED_AT" ]] || {
    printf 'GET monitor createdAt=%q, want exact POST value %q\n' "$created_at" "$MONITOR_CREATED_AT" >&2
    return 1
  }

  printf 'Retrieved monitor id=%s with exact POST/GET timestamp\n' "$MONITOR_ID"
}

assert_monitor_absent() {
  if compose exec -T api wget -q -O - "http://127.0.0.1:8080/monitors/$MONITOR_ID" >/dev/null 2>&1; then
    printf 'Monitor %s unexpectedly survived destructive reset\n' "$MONITOR_ID" >&2
    return 1
  fi
  printf 'Monitor %s is absent after destructive reset\n' "$MONITOR_ID"
}

PROBE_TABLE="public.__uptime_lab_local_dev_probe"

cleanup
compose config --quiet
"$ROOT/scripts/ci/check-local-dev.sh" "$ROOT"
compose build api
compose build web checker

# Fresh database: start only DB + API. API is live but intentionally not ready.
compose up -d db api
wait_for_api_livez
assert_api_unready_before_migration
assert_migration_metadata_absent

# Schema changes are explicit and separate from API startup.
apply_migrations
compose up -d --wait --wait-timeout 60
compose ps
assert_api_operational_health

# Prove the real public product transport inside the container network.
create_monitor
assert_monitor_get

compose exec -T db sh -lc \
  'psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" -c "$1"' \
  sh "CREATE TABLE $PROBE_TABLE (id integer PRIMARY KEY); INSERT INTO $PROBE_TABLE (id) VALUES (1);"

# Normal down/up preserves schema, probe state, and product state without rerunning migration.
compose down
compose up -d --wait --wait-timeout 60
assert_api_operational_health

PERSISTED="$(compose exec -T db sh -lc \
  'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Atc "$1"' \
  sh "SELECT to_regclass('$PROBE_TABLE') IS NOT NULL;")"
test "$PERSISTED" = "t"
assert_monitor_get

# Destructive reset returns to live-but-unready until migration is explicitly applied again.
compose down -v --remove-orphans
compose up -d db api
wait_for_api_livez
assert_api_unready_before_migration
assert_migration_metadata_absent

RESET="$(compose exec -T db sh -lc \
  'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Atc "$1"' \
  sh "SELECT to_regclass('$PROBE_TABLE') IS NULL;")"
test "$RESET" = "t"

apply_migrations
compose up -d --wait --wait-timeout 60
assert_api_operational_health
assert_monitor_absent

cleanup
trap - EXIT INT TERM
