#!/usr/bin/env bash
set -euo pipefail

ROOT="${1:-.}"
COMPOSE_FILE="$ROOT/compose.yaml"
PLACEHOLDER_DOCKERFILE="$ROOT/deploy/docker/placeholder/Dockerfile"
PLACEHOLDER_ENTRYPOINT="$ROOT/deploy/docker/placeholder/entrypoint.sh"
API_DOCKERFILE="$ROOT/apps/api/Dockerfile"

fail() {
  printf 'Local-dev invariant failed: %s\n' "$1" >&2
  exit 1
}

[[ -f "$COMPOSE_FILE" ]] || fail "compose.yaml is missing"
[[ -f "$PLACEHOLDER_DOCKERFILE" ]] || fail "placeholder Dockerfile is missing"
[[ -f "$PLACEHOLDER_ENTRYPOINT" ]] || fail "placeholder entrypoint is missing"
[[ -f "$API_DOCKERFILE" ]] || fail "API Dockerfile is missing"

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
  grep -Fqx '    healthcheck:' <<<"$block" || fail "$service healthcheck is missing"
  grep -Fqx '      test: ["CMD-SHELL", "test -f /run/uptime-lab/ready"]' <<<"$block" || fail "$service healthcheck must evaluate readiness marker"
}

for service in web checker; do
  require_placeholder_service "$service"
done

WEB_BLOCK="$(extract_service_block web)"
API_BLOCK="$(extract_service_block api)"
CHECKER_BLOCK="$(extract_service_block checker)"
DB_BLOCK="$(extract_service_block db)"

if grep -Fq '    depends_on:' <<<"$WEB_BLOCK"; then
  fail "web must start independently"
fi

grep -Fqx '    build:' <<<"$API_BLOCK" || fail "api must use explicit build mapping"
grep -Fqx '      context: .' <<<"$API_BLOCK" || fail "api build context must be repository root"
grep -Fqx '      dockerfile: apps/api/Dockerfile' <<<"$API_BLOCK" || fail "api must use apps/api/Dockerfile"
grep -Fqx '      UPTIME_LAB_HTTP_ADDR: ":8080"' <<<"$API_BLOCK" || fail "api HTTP address is missing"
grep -Fqx '      UPTIME_LAB_LOG_LEVEL: info' <<<"$API_BLOCK" || fail "api log level is missing"
grep -Fqx '      PGHOST: db' <<<"$API_BLOCK" || fail "api PGHOST must be db"
grep -Fqx '      PGPORT: "5432"' <<<"$API_BLOCK" || fail "api PGPORT must be 5432"
grep -Fqx '      PGDATABASE: ${POSTGRES_DB:-uptime_lab}' <<<"$API_BLOCK" || fail "api PGDATABASE mapping is missing"
grep -Fqx '      PGUSER: ${POSTGRES_USER:-uptime_lab}' <<<"$API_BLOCK" || fail "api PGUSER mapping is missing"
grep -Fqx '      PGPASSWORD: ${POSTGRES_PASSWORD:-uptime_lab_local}' <<<"$API_BLOCK" || fail "api PGPASSWORD mapping is missing"
grep -Fqx '      PGSSLMODE: disable' <<<"$API_BLOCK" || fail "api PGSSLMODE must be disable"
grep -Fqx '    init: true' <<<"$API_BLOCK" || fail "api must enable init"
grep -Fqx '    read_only: true' <<<"$API_BLOCK" || fail "api must be read-only"
grep -Fqx '    healthcheck:' <<<"$API_BLOCK" || fail "api healthcheck is missing"
grep -Fqx '      test: ["CMD", "wget", "-q", "-O", "/dev/null", "http://127.0.0.1:8080/readyz"]' <<<"$API_BLOCK" || fail "api healthcheck must call real /readyz"
grep -Fqx '    depends_on:' <<<"$API_BLOCK" || fail "api dependency is missing"
grep -Fqx '      db:' <<<"$API_BLOCK" || fail "api must depend on db"
grep -Fqx '        condition: service_healthy' <<<"$API_BLOCK" || fail "api must wait for healthy db"

grep -Fqx '    depends_on:' <<<"$CHECKER_BLOCK" || fail "checker dependency is missing"
grep -Fqx '      api:' <<<"$CHECKER_BLOCK" || fail "checker must depend on api"
grep -Fqx '        condition: service_healthy' <<<"$CHECKER_BLOCK" || fail "checker must wait for healthy api"

grep -Eq '^    image:[[:space:]]+postgres:[^[:space:]#]+$' <<<"$DB_BLOCK" || fail "db must use explicitly tagged postgres image"
grep -Fqx '      - postgres-data:/var/lib/postgresql' <<<"$DB_BLOCK" || fail "db must mount postgres-data at /var/lib/postgresql"
grep -Fqx '    healthcheck:' <<<"$DB_BLOCK" || fail "db healthcheck is missing"
grep -Fq 'pg_isready' <<<"$DB_BLOCK" || fail "db healthcheck must use pg_isready"

for forbidden in migrations package.json go.mod go.work Cargo.toml; do
  [[ ! -e "$ROOT/$forbidden" ]] || fail "phase-forbidden path exists: $forbidden"
done

if [[ -e "$ROOT/apps" && ! -d "$ROOT/apps" ]]; then
  fail "apps must be a directory when present"
fi

if [[ -d "$ROOT/apps" ]]; then
  while IFS= read -r app_path; do
    app_name="$(basename "$app_path")"
    [[ "$app_name" == "api" ]] || fail "phase-forbidden app path exists: apps/$app_name"
  done < <(find "$ROOT/apps" -mindepth 1 -maxdepth 1 -print)

  if [[ -e "$ROOT/apps/api" && ! -d "$ROOT/apps/api" ]]; then
    fail "apps/api must be a directory"
  fi
fi

PLACEHOLDER_FROM="$(awk '/^FROM[[:space:]]+/ { print; exit }' "$PLACEHOLDER_DOCKERFILE")"
[[ "$PLACEHOLDER_FROM" =~ ^FROM[[:space:]][^[:space:]]+:[^[:space:]]+$ ]] || fail "placeholder base image must use an explicit tag"
[[ "$PLACEHOLDER_FROM" != *":latest" ]] || fail "placeholder base image must not use latest"
grep -Fqx 'USER 10001:10001' "$PLACEHOLDER_DOCKERFILE" || fail "placeholder must run as USER 10001:10001"

mapfile -t API_FROM_LINES < <(grep -E '^FROM[[:space:]]+' "$API_DOCKERFILE")
[[ "${#API_FROM_LINES[@]}" -eq 2 ]] || fail "API Dockerfile must have exactly builder and runtime stages"
[[ "${API_FROM_LINES[0]}" == 'FROM golang:1.27.1-alpine3.24 AS builder' ]] || fail "API builder image must use exact reviewed Go pin"
[[ "${API_FROM_LINES[1]}" == 'FROM alpine:3.24.2' ]] || fail "API runtime image must use exact Alpine pin"

grep -Fqx 'WORKDIR /src/apps/api' "$API_DOCKERFILE" || fail "API builder workdir is invalid"
grep -Fqx 'COPY apps/api/go.mod apps/api/go.sum ./' "$API_DOCKERFILE" || fail "API Dockerfile must copy module manifests before source"
grep -Fqx 'RUN go mod download' "$API_DOCKERFILE" || fail "API Dockerfile must download modules before source copy"
grep -Fqx 'COPY apps/api/ ./' "$API_DOCKERFILE" || fail "API Dockerfile source copy is missing"
grep -Fq 'CGO_ENABLED=0' "$API_DOCKERFILE" || fail "API build must disable CGO"
grep -Fq -- '-trimpath' "$API_DOCKERFILE" || fail "API build must use -trimpath"
grep -Fq -- '-buildvcs=false' "$API_DOCKERFILE" || fail "API build must disable accidental VCS stamping"
grep -Fq '/out/uptime-lab-api ./cmd/api' "$API_DOCKERFILE" || fail "API binary build is missing"
grep -Fq '/out/uptime-lab-migrate ./cmd/migrate' "$API_DOCKERFILE" || fail "migration binary build is missing"
grep -Fq 'apk add --no-cache ca-certificates' "$API_DOCKERFILE" || fail "API runtime must install CA certificates"
grep -Fqx 'COPY --from=builder /out/uptime-lab-api /usr/local/bin/uptime-lab-api' "$API_DOCKERFILE" || fail "API runtime binary copy is missing"
grep -Fqx 'COPY --from=builder /out/uptime-lab-migrate /usr/local/bin/uptime-lab-migrate' "$API_DOCKERFILE" || fail "migration runtime binary copy is missing"
grep -Fqx 'USER 10001:10001' "$API_DOCKERFILE" || fail "API runtime must run as USER 10001:10001"
grep -Fqx 'ENTRYPOINT ["/usr/local/bin/uptime-lab-api"]' "$API_DOCKERFILE" || fail "API runtime entrypoint is invalid"
