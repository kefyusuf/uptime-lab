#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
SMOKE="$SCRIPT_DIR/smoke-local-dev.sh"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

PASS=0
FAIL=0

pass() {
  printf 'PASS: %s\n' "$1"
  PASS=$((PASS + 1))
}

fail() {
  printf 'FAIL: %s\n' "$1" >&2
  FAIL=$((FAIL + 1))
}

FAKE_DOCKER="$TMP/docker"
DOCKER_LOG="$TMP/docker.log"
FAKE_STATE_DIR="$TMP/state"
mkdir -p "$FAKE_STATE_DIR"

cat > "$FAKE_DOCKER" <<'FAKE'
#!/usr/bin/env bash
set -euo pipefail

printf '%s\n' "$*" >> "$DOCKER_LOG"

joined="$*"
state_dir="${FAKE_STATE_DIR:?FAKE_STATE_DIR is required}"
mkdir -p "$state_dir"
migrated="$state_dir/migrated"
product="$state_dir/product"
probe="$state_dir/probe"

if [[ -n "${FAIL_ON_PATTERN:-}" && "$joined" == *"$FAIL_ON_PATTERN"* ]]; then
  exit 42
fi

if [[ "$joined" == *"version --short"* ]]; then
  printf '%s\n' "${FAKE_COMPOSE_VERSION:-2.22.0}"
  exit 0
fi

if [[ "$joined" == *"down -v --remove-orphans"* ]]; then
  rm -f "$migrated" "$product" "$probe"
  exit 0
fi

if [[ "$joined" == *"exec -T api /usr/local/bin/uptime-lab-migrate up"* ]]; then
  : > "$migrated"
  exit 0
fi

if [[ "$joined" == *"up -d --wait --wait-timeout 60"* ]]; then
  [[ -f "$migrated" ]] || exit 43
  exit 0
fi

if [[ "$joined" == *"exec -T api wget"* && "$joined" == *"/livez"* ]]; then
  printf 'ok\n'
  exit 0
fi

if [[ "$joined" == *"exec -T api wget"* && "$joined" == *"/readyz"* ]]; then
  if [[ -f "$migrated" ]]; then
    printf 'ok\n'
    exit 0
  fi
  exit 8
fi

if [[ "$joined" == *"exec -T api wget"* && "$joined" == *"--post-data="* && "$joined" == *"/monitors"* ]]; then
  [[ -f "$migrated" ]] || exit 44
  : > "$product"
  printf '{"id":"018f22d3-1d6a-7cc0-a37b-46fc3fafdcb2","targetUrl":"https://example.com/local-smoke","createdAt":"2026-09-24T22:00:00.123456Z"}\n'
  exit 0
fi

if [[ "$joined" == *"exec -T api wget"* && "$joined" == *"/monitors/018f22d3-1d6a-7cc0-a37b-46fc3fafdcb2"* ]]; then
  if [[ -f "$product" ]]; then
    printf '{"id":"018f22d3-1d6a-7cc0-a37b-46fc3fafdcb2","targetUrl":"https://example.com/local-smoke","createdAt":"2026-09-24T22:00:00.123456Z"}\n'
    exit 0
  fi
  exit 8
fi

if [[ "$joined" == *"CREATE TABLE public.__uptime_lab_local_dev_probe"* ]]; then
  : > "$probe"
  exit 0
fi

if [[ "$joined" == *"psql"* && "$joined" == *"-Atc"* && "$joined" == *"goose_db_version"* && "$joined" == *"IS NULL"* ]]; then
  if [[ -f "$migrated" ]]; then printf 'f\n'; else printf 't\n'; fi
  exit 0
fi

if [[ "$joined" == *"psql"* && "$joined" == *"-Atc"* && "$joined" == *"__uptime_lab_local_dev_probe"* && "$joined" == *"IS NOT NULL"* ]]; then
  if [[ -f "$probe" ]]; then printf 't\n'; else printf 'f\n'; fi
  exit 0
fi

if [[ "$joined" == *"psql"* && "$joined" == *"-Atc"* && "$joined" == *"__uptime_lab_local_dev_probe"* && "$joined" == *"IS NULL"* ]]; then
  if [[ -f "$probe" ]]; then printf 'f\n'; else printf 't\n'; fi
  exit 0
fi

if [[ "$joined" == *"ps -q api"* ]]; then
  printf 'api-container\n'
  exit 0
fi

if [[ "$joined" == *"ps -q checker"* ]]; then
  printf 'checker-container\n'
  exit 0
fi

if [[ "$joined" == *"inspect --format {{.State.Health.Status}} api-container"* ]] || \
   [[ "$joined" == *"inspect --format {{.State.Health.Status}} checker-container"* ]]; then
  printf 'healthy\n'
  exit 0
fi

exit 0
FAKE

chmod +x "$FAKE_DOCKER"

reset_state() {
  rm -rf "$FAKE_STATE_DIR"
  mkdir -p "$FAKE_STATE_DIR"
  : > "$DOCKER_LOG"
}

first_line() {
  local pattern="$1"
  grep -n -F -- "$pattern" "$DOCKER_LOG" | head -n1 | cut -d: -f1
}

case_success_path() {
  reset_state

  DOCKER_LOG="$DOCKER_LOG" \
    FAKE_STATE_DIR="$FAKE_STATE_DIR" \
    DOCKER_BIN="$FAKE_DOCKER" \
    "$SMOKE" >/dev/null 2>&1 || return 1

  grep -Fq 'version --short' "$DOCKER_LOG" || return 1
  grep -Fq 'config --quiet' "$DOCKER_LOG" || return 1
  grep -Fq 'build api' "$DOCKER_LOG" || return 1
  grep -Fq 'build web checker' "$DOCKER_LOG" || return 1

  [[ "$(grep -Fc 'up -d db api' "$DOCKER_LOG")" -eq 2 ]] || return 1
  [[ "$(grep -Fc 'exec -T api /usr/local/bin/uptime-lab-migrate up' "$DOCKER_LOG")" -eq 2 ]] || return 1
  [[ "$(grep -Fc 'up -d --wait --wait-timeout 60' "$DOCKER_LOG")" -eq 3 ]] || return 1

  grep -Fq 'exec -T api wget -q -O - http://127.0.0.1:8080/livez' "$DOCKER_LOG" || return 1
  grep -Fq 'exec -T api wget -q -O - http://127.0.0.1:8080/readyz' "$DOCKER_LOG" || return 1
  grep -Fq "goose_db_version" "$DOCKER_LOG" || return 1
  grep -Fq -- '--post-data={"targetUrl":"https://example.com/local-smoke"}' "$DOCKER_LOG" || return 1
  grep -Fq '/monitors/018f22d3-1d6a-7cc0-a37b-46fc3fafdcb2' "$DOCKER_LOG" || return 1
  grep -Fq 'CREATE TABLE public.__uptime_lab_local_dev_probe' "$DOCKER_LOG" || return 1
  grep -Fq 'down -v --remove-orphans' "$DOCKER_LOG" || return 1

  local start_line migrate_line wait_line post_line
  start_line="$(first_line 'up -d db api')"
  migrate_line="$(first_line 'exec -T api /usr/local/bin/uptime-lab-migrate up')"
  wait_line="$(first_line 'up -d --wait --wait-timeout 60')"
  post_line="$(first_line '--post-data={"targetUrl":"https://example.com/local-smoke"}')"

  (( start_line < migrate_line )) || return 1
  (( migrate_line < wait_line )) || return 1
  (( wait_line < post_line )) || return 1
}

case_old_compose() {
  reset_state

  if DOCKER_LOG="$DOCKER_LOG" \
    FAKE_STATE_DIR="$FAKE_STATE_DIR" \
    DOCKER_BIN="$FAKE_DOCKER" \
    FAKE_COMPOSE_VERSION=2.21.0 \
    "$SMOKE" >/dev/null 2>&1; then
    return 1
  fi

  grep -Fq 'version --short' "$DOCKER_LOG" || return 1
}

case_failure_cleanup() {
  reset_state

  if DOCKER_LOG="$DOCKER_LOG" \
    FAKE_STATE_DIR="$FAKE_STATE_DIR" \
    DOCKER_BIN="$FAKE_DOCKER" \
    FAIL_ON_PATTERN=build \
    "$SMOKE" >/dev/null 2>&1; then
    return 1
  fi

  grep -Fq 'build' "$DOCKER_LOG" || return 1
  grep -Fq 'down -v --remove-orphans' "$DOCKER_LOG" || return 1
}

case_migration_failure_cleanup() {
  reset_state

  if DOCKER_LOG="$DOCKER_LOG" \
    FAKE_STATE_DIR="$FAKE_STATE_DIR" \
    DOCKER_BIN="$FAKE_DOCKER" \
    FAIL_ON_PATTERN='uptime-lab-migrate up' \
    "$SMOKE" >/dev/null 2>&1; then
    return 1
  fi

  grep -Fq 'exec -T api /usr/local/bin/uptime-lab-migrate up' "$DOCKER_LOG" || return 1
  grep -Fq 'down -v --remove-orphans' "$DOCKER_LOG" || return 1
}

if case_success_path; then
  pass "schema-aware smoke success path"
else
  fail "schema-aware smoke success path"
fi

if case_old_compose; then
  pass "reject Compose 2.21"
else
  fail "reject Compose 2.21"
fi

if case_failure_cleanup; then
  pass "build failure triggers cleanup"
else
  fail "build failure triggers cleanup"
fi

if case_migration_failure_cleanup; then
  pass "migration failure triggers cleanup"
else
  fail "migration failure triggers cleanup"
fi

printf '\nLocal-dev smoke tests: %d passed, %d failed\n' "$PASS" "$FAIL"
test "$PASS" -eq 4
test "$FAIL" -eq 0
