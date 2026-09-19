#!/usr/bin/env bash
set -euo pipefail

ROOT="${1:-.}"
COMPOSE_FILE="$ROOT/compose.yaml"
DOCKERFILE="$ROOT/deploy/docker/placeholder/Dockerfile"
ENTRYPOINT="$ROOT/deploy/docker/placeholder/entrypoint.sh"

fail() {
  printf 'Local-dev invariant failed: %s\n' "$1" >&2
  exit 1
}

[[ -f "$COMPOSE_FILE" ]] || fail "compose.yaml is missing"
[[ -f "$DOCKERFILE" ]] || fail "placeholder Dockerfile is missing"
[[ -f "$ENTRYPOINT" ]] || fail "placeholder entrypoint is missing"

extract_service_block() {
  local service="$1"
  awk -v service="$service" '
    $0 == "services:" { in_services=1; next }
    in_services && /^[^ ]/ { exit }
    in_services && $0 == "  " service ":" { in_target=1; next }
    in_target && /^  [A-Za-z0-9_.-]+:[[:space:]]*$/ { exit }
    in_target { print }
  ' "$COMPOSE_FILE"
}

SERVICES="$({
  awk '
    $0 == "services:" { in_services=1; next }
    in_services && /^[^ ]/ { exit }
    in_services && /^  [A-Za-z0-9_.-]+:[[:space:]]*$/ {
      service=$0
      sub(/^  /, "", service)
      sub(/:[[:space:]]*$/, "", service)
      print service
    }
  ' "$COMPOSE_FILE"
} | sort)"

[[ "$SERVICES" == $'api\nchecker\ndb\nweb' ]] || fail "service set must be exactly api, checker, db, web"

if grep -Eq '^name:[[:space:]]' "$COMPOSE_FILE"; then
  fail "fixed root Compose project name is forbidden"
fi
if grep -Eq '^[[:space:]]+container_name:' "$COMPOSE_FILE"; then
  fail "container_name is forbidden"
fi
if grep -Eq '^[[:space:]]+ports:' "$COMPOSE_FILE"; then
  fail "host ports are forbidden"
fi
if grep -Eq '^[[:space:]]+restart:' "$COMPOSE_FILE"; then
  fail "restart policy is forbidden"
fi
if grep -Eq '^[[:space:]]+profiles:' "$COMPOSE_FILE"; then
  fail "Compose profiles are forbidden"
fi
if grep -Eq '^[[:space:]]+network_mode:[[:space:]]*host([[:space:]]|$)' "$COMPOSE_FILE"; then
  fail "host network mode is forbidden"
fi
if grep -Eq '^[[:space:]]+external:[[:space:]]*true([[:space:]]|$)' "$COMPOSE_FILE"; then
  fail "external Compose resources are forbidden"
fi
if grep -Eq '^[[:space:]]+image:[[:space:]]*[^[:space:]#]+:latest([[:space:]]|$)' "$COMPOSE_FILE"; then
  fail "floating latest image is forbidden"
fi
if grep -Eq '^    name:[[:space:]]' "$COMPOSE_FILE"; then
  fail "globally named Compose resources are forbidden"
fi

require_placeholder_service() {
  local service="$1"
  local block
  block="$(extract_service_block "$service")"
  [[ -n "$block" ]] || fail "$service service is missing"

  grep -Fqx '    build: ./deploy/docker/placeholder' <<<"$block" || fail "$service must use placeholder build"
  grep -Fqx "      SERVICE_NAME: $service" <<<"$block" || fail "$service SERVICE_NAME is invalid"
  grep -Fqx '    init: true' <<<"$block" || fail "$service must enable init"
  grep -Fqx '    read_only: true' <<<"$block" || fail "$service must be read-only"
  grep -Fqx '      - /run/uptime-lab:uid=10001,gid=10001,mode=0700' <<<"$block" || fail "$service tmpfs contract is missing"
  grep -Fq '/run/uptime-lab/ready' <<<"$block" || fail "$service readiness marker is missing"
}

for service in web api checker; do
  require_placeholder_service "$service"
done

WEB_BLOCK="$(extract_service_block web)"
API_BLOCK="$(extract_service_block api)"
CHECKER_BLOCK="$(extract_service_block checker)"
DB_BLOCK="$(extract_service_block db)"

if grep -Fq '    depends_on:' <<<"$WEB_BLOCK"; then
  fail "web must start independently"
fi

grep -Fqx '    depends_on:' <<<"$API_BLOCK" || fail "api dependency is missing"
grep -Fqx '      db:' <<<"$API_BLOCK" || fail "api must depend on db"
grep -Fqx '        condition: service_healthy' <<<"$API_BLOCK" || fail "api must wait for healthy db"

grep -Fqx '    depends_on:' <<<"$CHECKER_BLOCK" || fail "checker dependency is missing"
grep -Fqx '      api:' <<<"$CHECKER_BLOCK" || fail "checker must depend on api"
grep -Fqx '        condition: service_healthy' <<<"$CHECKER_BLOCK" || fail "checker must wait for healthy api"

grep -Eq '^    image:[[:space:]]+postgres:[^[:space:]#]+$' <<<"$DB_BLOCK" || fail "db must use explicitly tagged postgres image"
grep -Fqx '      - postgres-data:/var/lib/postgresql' <<<"$DB_BLOCK" || fail "db must mount postgres-data at /var/lib/postgresql"
grep -Fq 'pg_isready' <<<"$DB_BLOCK" || fail "db healthcheck must use pg_isready"

for forbidden in apps contracts migrations package.json go.mod Cargo.toml; do
  [[ ! -e "$ROOT/$forbidden" ]] || fail "phase-forbidden path exists: $forbidden"
done

FROM_LINE="$(awk '/^FROM[[:space:]]+/ { print; exit }' "$DOCKERFILE")"
[[ "$FROM_LINE" =~ ^FROM[[:space:]][^[:space:]]+:[^[:space:]]+$ ]] || fail "placeholder base image must use an explicit tag"
[[ "$FROM_LINE" != *":latest" ]] || fail "placeholder base image must not use latest"
grep -Fqx 'USER 10001:10001' "$DOCKERFILE" || fail "placeholder must run as USER 10001:10001"
