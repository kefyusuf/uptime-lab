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

cat > "$FAKE_DOCKER" <<'FAKE'
#!/usr/bin/env bash
set -euo pipefail

printf '%s\n' "$*" >> "$DOCKER_LOG"

joined="$*"
if [[ -n "${FAIL_ON_PATTERN:-}" && "$joined" == *"$FAIL_ON_PATTERN"* ]]; then
  exit 42
fi

if [[ "$joined" == *"version --short"* ]]; then
  printf '%s\n' "${FAKE_COMPOSE_VERSION:-2.22.0}"
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

if [[ "$joined" == *"exec -T api wget -q -O - http://127.0.0.1:8080/livez"* ]] || \
   [[ "$joined" == *"exec -T api wget -q -O - http://127.0.0.1:8080/readyz"* ]]; then
  printf 'ok\n'
  exit 0
fi

if [[ "$joined" == *"psql"* && "$joined" == *"-Atc"* ]]; then
  printf 't\n'
  exit 0
fi

exit 0
FAKE

chmod +x "$FAKE_DOCKER"

case_success_path() {
  : > "$DOCKER_LOG"

  DOCKER_LOG="$DOCKER_LOG" \
    DOCKER_BIN="$FAKE_DOCKER" \
    "$SMOKE" >/dev/null 2>&1 || return 1

  grep -Fq 'version --short' "$DOCKER_LOG" || return 1
  grep -Fq 'config --quiet' "$DOCKER_LOG" || return 1
  grep -Fq 'build api' "$DOCKER_LOG" || return 1
  grep -Fq 'build web checker' "$DOCKER_LOG" || return 1
  grep -Fq 'up -d --wait --wait-timeout 60' "$DOCKER_LOG" || return 1
  grep -Fq 'ps' "$DOCKER_LOG" || return 1
  grep -Fq 'ps -q api' "$DOCKER_LOG" || return 1
  grep -Fq 'inspect --format {{.State.Health.Status}} api-container' "$DOCKER_LOG" || return 1
  grep -Fq 'exec -T api wget -q -O - http://127.0.0.1:8080/livez' "$DOCKER_LOG" || return 1
  grep -Fq 'exec -T api wget -q -O - http://127.0.0.1:8080/readyz' "$DOCKER_LOG" || return 1
  grep -Fq 'ps -q checker' "$DOCKER_LOG" || return 1
  grep -Fq 'inspect --format {{.State.Health.Status}} checker-container' "$DOCKER_LOG" || return 1
  grep -Fq 'psql' "$DOCKER_LOG" || return 1
  grep -Fq 'POSTGRES_USER' "$DOCKER_LOG" || return 1
  grep -Fq 'POSTGRES_DB' "$DOCKER_LOG" || return 1
  grep -Fq 'down -v --remove-orphans' "$DOCKER_LOG" || return 1
}

case_old_compose() {
  : > "$DOCKER_LOG"

  if DOCKER_LOG="$DOCKER_LOG" \
    DOCKER_BIN="$FAKE_DOCKER" \
    FAKE_COMPOSE_VERSION=2.21.0 \
    "$SMOKE" >/dev/null 2>&1; then
    return 1
  fi

  grep -Fq 'version --short' "$DOCKER_LOG" || return 1
}

case_failure_cleanup() {
  : > "$DOCKER_LOG"

  if DOCKER_LOG="$DOCKER_LOG" \
    DOCKER_BIN="$FAKE_DOCKER" \
    FAIL_ON_PATTERN=build \
    "$SMOKE" >/dev/null 2>&1; then
    return 1
  fi

  grep -Fq 'build' "$DOCKER_LOG" || return 1
  grep -Fq 'down -v --remove-orphans' "$DOCKER_LOG" || return 1
}

if case_success_path; then
  pass "smoke success path"
else
  fail "smoke success path"
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

printf '\nLocal-dev smoke tests: %d passed, %d failed\n' "$PASS" "$FAIL"
test "$FAIL" -eq 0
