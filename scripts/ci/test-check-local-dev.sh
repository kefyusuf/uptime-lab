#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
CHECKER="$SCRIPT_DIR/check-local-dev.sh"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

PASS=0
FAIL=0

pass() { printf 'PASS: %s\n' "$1"; PASS=$((PASS + 1)); }
fail() { printf 'FAIL: %s\n' "$1" >&2; FAIL=$((FAIL + 1)); }
expect_success() { local n="$1"; shift; if "$@" >/dev/null 2>&1; then pass "$n"; else fail "$n"; fi; }
expect_failure() { local n="$1"; shift; if "$@" >/dev/null 2>&1; then fail "$n"; else pass "$n"; fi; }

replace_literal_once() {
  local file="$1"
  local old="$2"
  local new="$3"
  local content
  content="$(cat "$file")"
  [[ "$content" == *"$old"* ]] || {
    printf 'fixture text not found in %s: %s\n' "$file" "$old" >&2
    return 1
  }
  content="${content/"$old"/"$new"}"
  printf '%s' "$content" > "$file"
}

insert_after_line() {
  local file="$1"
  local exact="$2"
  local insertion="$3"
  local tmp="$file.tmp"
  local found=0
  : > "$tmp"
  while IFS= read -r line || [[ -n "$line" ]]; do
    printf '%s\n' "$line" >> "$tmp"
    if [[ "$found" -eq 0 && "$line" == "$exact" ]]; then
      printf '%s\n' "$insertion" >> "$tmp"
      found=1
    fi
  done < "$file"
  [[ "$found" -eq 1 ]] || {
    rm -f "$tmp"
    printf 'fixture line not found in %s: %s\n' "$file" "$exact" >&2
    return 1
  }
  mv "$tmp" "$file"
}

remove_service_block() {
  local file="$1"
  local service="$2"
  local tmp="$file.tmp"

  awk -v target="  $service:" '
    $0 == target {
      in_target=1
      removed=1
      next
    }
    in_target && ($0 ~ /^  [A-Za-z0-9_.-]+:[[:space:]]*$/ || $0 ~ /^[^ ]/) {
      in_target=0
    }
    !in_target { print }
    END {
      if (removed != 1) exit 42
    }
  ' "$file" > "$tmp"

  mv "$tmp" "$file"
}

mutate_service_line() {
  local file="$1"
  local service="$2"
  local old_line="$3"
  local new_line="$4"
  local tmp="$file.tmp"

  awk -v target="  $service:" -v old="$old_line" -v new="$new_line" '
    $0 == target {
      in_target=1
      print
      next
    }
    in_target && ($0 ~ /^  [A-Za-z0-9_.-]+:[[:space:]]*$/ || $0 ~ /^[^ ]/) {
      in_target=0
    }
    in_target && $0 == old {
      if (new != "") print new
      changed++
      next
    }
    { print }
    END {
      if (changed != 1) exit 42
    }
  ' "$file" > "$tmp"

  mv "$tmp" "$file"
}

make_fixture() {
  rm -rf "$TMP/repo"
  mkdir -p "$TMP/repo/deploy/docker/placeholder" "$TMP/repo/apps/api" "$TMP/repo/apps/checker" "$TMP/repo/apps/web" "$TMP/repo/contracts/openapi"
  cp "$SCRIPT_DIR/../../apps/web/Dockerfile" "$TMP/repo/apps/web/Dockerfile"
  printf '{}\n' > "$TMP/repo/apps/web/package-lock.json"
  printf 'module github.com/kefyusuf/uptime-lab/apps/api\n\ngo 1.27.1\n' > "$TMP/repo/apps/api/go.mod"
  : > "$TMP/repo/apps/api/go.sum"
  printf '[workspace]\n' > "$TMP/repo/apps/checker/Cargo.toml"
  : > "$TMP/repo/apps/checker/Cargo.lock"
  printf 'openapi: 3.1.2\n' > "$TMP/repo/contracts/openapi/public.yaml"

  cat > "$TMP/repo/compose.yaml" <<'YAML'
services:
  web:
    build:
      context: .
      dockerfile: apps/web/Dockerfile
    environment:
      UPTIME_LAB_WEB_PORT: ${UPTIME_LAB_WEB_PORT:-4173}
    init: true
    read_only: true
    tmpfs:
      - /run/uptime-lab:uid=10001,gid=10001,mode=0700
    healthcheck:
      test: ["CMD", "node", "-e", "fetch('http://127.0.0.1:8080/healthz').then(r=>process.exit(r.ok?0:1)).catch(()=>process.exit(1))"]
      interval: 2s
      timeout: 1s
      retries: 10
      start_period: 2s

  db:
    image: postgres:18.6-alpine3.24
    environment:
      POSTGRES_DB: ${POSTGRES_DB:-uptime_lab}
      POSTGRES_USER: ${POSTGRES_USER:-uptime_lab}
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD:-uptime_lab_local}
    volumes:
      - postgres-data:/var/lib/postgresql
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U \"$${POSTGRES_USER}\" -d \"$${POSTGRES_DB}\""]
      interval: 2s
      timeout: 2s
      retries: 15
      start_period: 3s

  api:
    build:
      context: .
      dockerfile: apps/api/Dockerfile
    environment:
      UPTIME_LAB_HTTP_ADDR: ":8080"
      UPTIME_LAB_LOG_LEVEL: info
      PGHOST: db
      PGPORT: "5432"
      PGDATABASE: ${POSTGRES_DB:-uptime_lab}
      PGUSER: ${POSTGRES_USER:-uptime_lab}
      PGPASSWORD: ${POSTGRES_PASSWORD:-uptime_lab_local}
      PGSSLMODE: disable
    init: true
    read_only: true
    healthcheck:
      test: ["CMD", "wget", "-q", "-O", "/dev/null", "http://127.0.0.1:8080/readyz"]
      interval: 2s
      timeout: 2s
      retries: 15
      start_period: 2s
    depends_on:
      db:
        condition: service_healthy

  checker:
    build:
      context: .
      dockerfile: apps/checker/Dockerfile
    environment:
      UPTIME_LAB_CONTROL_PLANE_URL: http://api:8080
    init: true
    read_only: true
    tmpfs:
      - /run/uptime-lab:uid=10001,gid=10001,mode=0700
    healthcheck:
      test: ["CMD-SHELL", "test -f /run/uptime-lab/ready"]
      interval: 2s
      timeout: 1s
      retries: 10
      start_period: 2s
    depends_on:
      api:
        condition: service_healthy

volumes:
  postgres-data:
YAML

  cat > "$TMP/repo/.env.example" <<'ENV'
POSTGRES_DB=uptime_lab
POSTGRES_USER=uptime_lab
POSTGRES_PASSWORD=uptime_lab_local
ENV

  cat > "$TMP/repo/deploy/docker/placeholder/Dockerfile" <<'DOCKER'
FROM alpine:3.24.2
RUN addgroup -S -g 10001 uptime && adduser -S -D -H -u 10001 -G uptime uptime
COPY --chown=10001:10001 entrypoint.sh /usr/local/bin/placeholder-entrypoint
USER 10001:10001
ENTRYPOINT ["/usr/local/bin/placeholder-entrypoint"]
DOCKER

  cat > "$TMP/repo/deploy/docker/placeholder/entrypoint.sh" <<'SH'
#!/bin/sh
set -eu
: > /run/uptime-lab/ready
exec tail -f /dev/null
SH

  cat > "$TMP/repo/apps/api/Dockerfile" <<'DOCKER'
FROM golang:1.27.1-alpine3.24 AS builder
WORKDIR /src/apps/api
COPY apps/api/go.mod apps/api/go.sum ./
RUN go mod download
COPY apps/api/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -buildvcs=false -o /out/uptime-lab-api ./cmd/api \
    && CGO_ENABLED=0 GOOS=linux go build -trimpath -buildvcs=false -o /out/uptime-lab-migrate ./cmd/migrate

FROM alpine:3.24.2
RUN apk add --no-cache ca-certificates \
    && addgroup -S -g 10001 uptime \
    && adduser -S -D -H -u 10001 -G uptime uptime
COPY --from=builder /out/uptime-lab-api /usr/local/bin/uptime-lab-api
COPY --from=builder /out/uptime-lab-migrate /usr/local/bin/uptime-lab-migrate
RUN chmod 0555 /usr/local/bin/uptime-lab-api /usr/local/bin/uptime-lab-migrate
USER 10001:10001
ENTRYPOINT ["/usr/local/bin/uptime-lab-api"]
DOCKER

  cat > "$TMP/repo/apps/checker/Dockerfile" <<'DOCKER'
FROM rust:1.98.1-alpine3.24 AS builder
WORKDIR /src/apps/checker
COPY apps/checker/ ./
RUN cargo build --release --locked -p checker

FROM alpine:3.24.2
RUN apk add --no-cache ca-certificates \
    && addgroup -S -g 10001 uptime \
    && adduser -S -D -H -u 10001 -G uptime uptime
COPY --from=builder /src/apps/checker/target/release/checker /usr/local/bin/uptime-lab-checker
RUN chmod 0555 /usr/local/bin/uptime-lab-checker
USER 10001:10001
ENTRYPOINT ["/usr/local/bin/uptime-lab-checker"]
DOCKER
}

make_fixture
expect_success "canonical real API + Checker fixture passes" "$CHECKER" "$TMP/repo"

make_fixture
remove_service_block "$TMP/repo/compose.yaml" checker
expect_failure "missing checker service fails" "$CHECKER" "$TMP/repo"

make_fixture
insert_after_line "$TMP/repo/compose.yaml" "  web:" $'    ports:\n      - "3000:3000"'
expect_failure "host ports mapping fails" "$CHECKER" "$TMP/repo"

make_fixture
insert_after_line "$TMP/repo/compose.yaml" "  web:" "    container_name: uptime-lab-web"
expect_failure "container_name fails" "$CHECKER" "$TMP/repo"

make_fixture
{ printf 'name: uptime-lab\n'; cat "$TMP/repo/compose.yaml"; } > "$TMP/repo/compose.yaml.tmp"
mv "$TMP/repo/compose.yaml.tmp" "$TMP/repo/compose.yaml"
expect_failure "fixed root project name fails" "$CHECKER" "$TMP/repo"

make_fixture
insert_after_line "$TMP/repo/compose.yaml" "  postgres-data:" "    name: uptime-lab-postgres-data"
expect_failure "globally named volume fails" "$CHECKER" "$TMP/repo"

make_fixture
insert_after_line "$TMP/repo/compose.yaml" "  web:" "    network_mode: host"
expect_failure "host network mode fails" "$CHECKER" "$TMP/repo"

make_fixture
replace_literal_once "$TMP/repo/compose.yaml" "postgres-data:/var/lib/postgresql" "postgres-data:/var/lib/postgresql/data"
expect_failure "PostgreSQL old data mount fails" "$CHECKER" "$TMP/repo"

make_fixture
mutate_service_line "$TMP/repo/compose.yaml" api "        condition: service_healthy" "        condition: service_started"
expect_failure "API missing db service_healthy fails" "$CHECKER" "$TMP/repo"

make_fixture
mutate_service_line "$TMP/repo/compose.yaml" checker "        condition: service_healthy" "        condition: service_started"
expect_failure "Checker missing api service_healthy fails" "$CHECKER" "$TMP/repo"

make_fixture
insert_after_line "$TMP/repo/compose.yaml" "  web:" $'    depends_on:\n      api:\n        condition: service_healthy'
expect_failure "Web hard dependency fails" "$CHECKER" "$TMP/repo"

make_fixture
replace_literal_once "$TMP/repo/compose.yaml" "image: postgres:18.6-alpine3.24" "image: postgres:latest"
expect_failure "floating latest image fails" "$CHECKER" "$TMP/repo"

make_fixture
mutate_service_line "$TMP/repo/compose.yaml" web "    read_only: true" ""
expect_failure "placeholder missing hardening fails" "$CHECKER" "$TMP/repo"

make_fixture
insert_after_line "$TMP/repo/compose.yaml" "  web:" "    restart: unless-stopped"
expect_failure "placeholder restart policy fails" "$CHECKER" "$TMP/repo"

make_fixture
insert_after_line "$TMP/repo/compose.yaml" "  web:" "    profiles: [dev]"
expect_failure "Compose profiles fail" "$CHECKER" "$TMP/repo"

make_fixture
rm "$TMP/repo/apps/web/package-lock.json"
expect_failure "Web missing frozen lock fails" "$CHECKER" "$TMP/repo"

make_fixture
mutate_service_line "$TMP/repo/compose.yaml" web "    healthcheck:" "    x-healthcheck:"
expect_failure "placeholder missing explicit healthcheck fails" "$CHECKER" "$TMP/repo"

make_fixture
mutate_service_line "$TMP/repo/compose.yaml" web '      test: ["CMD", "node", "-e", "fetch('\''http://127.0.0.1:8080/healthz'\'').then(r=>process.exit(r.ok?0:1)).catch(()=>process.exit(1))"]' '      test: ["CMD-SHELL", "test -f /run/uptime-lab/ready"]'
expect_failure "Web marker-only health fails" "$CHECKER" "$TMP/repo"

make_fixture
replace_literal_once "$TMP/repo/compose.yaml" '      test: ["CMD-SHELL", "pg_isready -U \"$${POSTGRES_USER}\" -d \"$${POSTGRES_DB}\""]' '      test: ["CMD-SHELL", "test -f /tmp/db-ready"]'
expect_failure "PostgreSQL healthcheck without pg_isready fails" "$CHECKER" "$TMP/repo"

make_fixture
replace_literal_once "$TMP/repo/compose.yaml" $'    build:\n      context: .\n      dockerfile: apps/api/Dockerfile' '    build: ./deploy/docker/placeholder'
expect_failure "API placeholder build fails" "$CHECKER" "$TMP/repo"

make_fixture
replace_literal_once "$TMP/repo/compose.yaml" "      dockerfile: apps/api/Dockerfile" "      dockerfile: deploy/docker/placeholder/Dockerfile"
expect_failure "API wrong Dockerfile path fails" "$CHECKER" "$TMP/repo"

make_fixture
mutate_service_line "$TMP/repo/compose.yaml" api "    init: true" ""
expect_failure "API missing init fails" "$CHECKER" "$TMP/repo"

make_fixture
mutate_service_line "$TMP/repo/compose.yaml" api "    read_only: true" ""
expect_failure "API missing read_only fails" "$CHECKER" "$TMP/repo"

make_fixture
mutate_service_line "$TMP/repo/compose.yaml" api "      PGHOST: db" ""
expect_failure "API missing PGHOST fails" "$CHECKER" "$TMP/repo"

make_fixture
mutate_service_line "$TMP/repo/compose.yaml" api '      PGDATABASE: ${POSTGRES_DB:-uptime_lab}' ""
expect_failure "API missing PGDATABASE fails" "$CHECKER" "$TMP/repo"

make_fixture
mutate_service_line "$TMP/repo/compose.yaml" api '      PGUSER: ${POSTGRES_USER:-uptime_lab}' ""
expect_failure "API missing PGUSER fails" "$CHECKER" "$TMP/repo"

make_fixture
mutate_service_line "$TMP/repo/compose.yaml" api '      UPTIME_LAB_HTTP_ADDR: ":8080"' ""
expect_failure "API missing HTTP address fails" "$CHECKER" "$TMP/repo"

make_fixture
replace_literal_once "$TMP/repo/compose.yaml" "http://127.0.0.1:8080/readyz" "http://127.0.0.1:8080/livez"
expect_failure "API healthcheck without readyz fails" "$CHECKER" "$TMP/repo"

make_fixture
mutate_service_line "$TMP/repo/compose.yaml" api '      test: ["CMD", "wget", "-q", "-O", "/dev/null", "http://127.0.0.1:8080/readyz"]' '      test: ["CMD-SHELL", "test -f /run/uptime-lab/ready"]'
expect_failure "API placeholder readiness marker fails" "$CHECKER" "$TMP/repo"

make_fixture
replace_literal_once "$TMP/repo/apps/api/Dockerfile" "FROM golang:1.27.1-alpine3.24 AS builder" "FROM golang:latest AS builder"
expect_failure "API floating builder image fails" "$CHECKER" "$TMP/repo"

make_fixture
replace_literal_once "$TMP/repo/apps/api/Dockerfile" "FROM alpine:3.24.2" "FROM alpine:latest"
expect_failure "API floating runtime image fails" "$CHECKER" "$TMP/repo"

make_fixture
replace_literal_once "$TMP/repo/apps/api/Dockerfile" "USER 10001:10001" "# USER intentionally omitted"
expect_failure "API Dockerfile missing non-root USER fails" "$CHECKER" "$TMP/repo"

make_fixture
expect_success "Checker Rust source presence is valid with real Checker Compose wiring" "$CHECKER" "$TMP/repo"

make_fixture
replace_literal_once "$TMP/repo/compose.yaml" "      dockerfile: apps/checker/Dockerfile" "      dockerfile: deploy/docker/placeholder/Dockerfile"
expect_failure "Checker wrong Dockerfile path fails" "$CHECKER" "$TMP/repo"

make_fixture
mutate_service_line "$TMP/repo/compose.yaml" checker "      UPTIME_LAB_CONTROL_PLANE_URL: http://api:8080" ""
expect_failure "Checker missing control-plane URL fails" "$CHECKER" "$TMP/repo"

make_fixture
mutate_service_line "$TMP/repo/compose.yaml" checker "    init: true" ""
expect_failure "Checker missing init fails" "$CHECKER" "$TMP/repo"

make_fixture
mutate_service_line "$TMP/repo/compose.yaml" checker "    read_only: true" ""
expect_failure "Checker missing read_only fails" "$CHECKER" "$TMP/repo"

make_fixture
mutate_service_line "$TMP/repo/compose.yaml" checker '      test: ["CMD-SHELL", "test -f /run/uptime-lab/ready"]' '      test: ["CMD-SHELL", "test -f /run/uptime-lab/not-ready"]'
expect_failure "Checker healthcheck without readiness marker fails" "$CHECKER" "$TMP/repo"

make_fixture
rm "$TMP/repo/apps/checker/Cargo.lock"
expect_failure "Checker missing Cargo.lock fails" "$CHECKER" "$TMP/repo"

make_fixture
replace_literal_once "$TMP/repo/apps/checker/Dockerfile" "FROM rust:1.98.1-alpine3.24 AS builder" "FROM rust:latest AS builder"
expect_failure "Checker floating builder image fails" "$CHECKER" "$TMP/repo"

make_fixture
replace_literal_once "$TMP/repo/apps/checker/Dockerfile" "FROM alpine:3.24.2" "FROM alpine:latest"
expect_failure "Checker floating runtime image fails" "$CHECKER" "$TMP/repo"

make_fixture
replace_literal_once "$TMP/repo/apps/checker/Dockerfile" "RUN cargo build --release --locked -p checker" "RUN cargo build --release -p checker"
expect_failure "Checker build without locked dependency graph fails" "$CHECKER" "$TMP/repo"

make_fixture
replace_literal_once "$TMP/repo/apps/checker/Dockerfile" "apk add --no-cache ca-certificates" "apk add --no-cache busybox"
expect_failure "Checker runtime missing CA certificates fails" "$CHECKER" "$TMP/repo"

make_fixture
replace_literal_once "$TMP/repo/apps/checker/Dockerfile" "USER 10001:10001" "# USER intentionally omitted"
expect_failure "Checker Dockerfile missing non-root USER fails" "$CHECKER" "$TMP/repo"

make_fixture
replace_literal_once "$TMP/repo/apps/checker/Dockerfile" 'ENTRYPOINT ["/usr/local/bin/uptime-lab-checker"]' 'ENTRYPOINT ["/bin/sh"]'
expect_failure "Checker Dockerfile wrong entrypoint fails" "$CHECKER" "$TMP/repo"

make_fixture
printf 'module example.invalid/root\n' > "$TMP/repo/go.mod"
expect_failure "root go.mod fails" "$CHECKER" "$TMP/repo"

make_fixture
replace_literal_once "$TMP/repo/apps/api/Dockerfile" "COPY --from=builder /out/uptime-lab-migrate /usr/local/bin/uptime-lab-migrate" "# migration binary copy intentionally omitted"
expect_failure "API Dockerfile missing migrate binary fails" "$CHECKER" "$TMP/repo"

make_fixture
replace_literal_once "$TMP/repo/apps/api/Dockerfile" 'ENTRYPOINT ["/usr/local/bin/uptime-lab-api"]' 'ENTRYPOINT ["/usr/local/bin/uptime-lab-migrate"]'
expect_failure "API Dockerfile wrong default entrypoint fails" "$CHECKER" "$TMP/repo"

make_fixture
replace_literal_once "$TMP/repo/apps/api/Dockerfile" "apk add --no-cache ca-certificates" "apk add --no-cache busybox"
expect_failure "API runtime missing CA certificates fails" "$CHECKER" "$TMP/repo"

printf '\nLocal development tests: %d passed, %d failed\n' "$PASS" "$FAIL"
test "$PASS" -eq 49
test "$FAIL" -eq 0
