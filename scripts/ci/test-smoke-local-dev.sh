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
check_run_id="018f22d3-1d6a-7cc0-a37b-46fc3fafdcb3"
completed_at="2026-09-24T22:00:07.123456Z"

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
  printf '{"id":"018f22d3-1d6a-7cc0-a37b-46fc3fafdcb2","targetUrl":"http://web/","createdAt":"2026-09-24T22:00:00.123456Z"}\n'
  exit 0
fi

if [[ "$joined" == *"exec -T api wget"* && "$joined" == *"/monitors/018f22d3-1d6a-7cc0-a37b-46fc3fafdcb2/latest-result"* ]]; then
  [[ -f "$product" ]] || exit 49
  case "${FAKE_LATEST_RESULT_MODE:-valid}" in
    valid)
      printf '{"checkId":"%s","resultKind":"policy_rejected","durationMs":7,"completedAt":"%s"}\n'         "$check_run_id" "$completed_at"
      ;;
    wrong-check-id)
      printf '{"checkId":"018f22d3-1d6a-7cc0-a37b-46fc3fafdcb4","resultKind":"policy_rejected","durationMs":7,"completedAt":"%s"}\n'         "$completed_at"
      ;;
    exposes-http-status)
      printf '{"checkId":"%s","resultKind":"policy_rejected","httpStatus":403,"durationMs":7,"completedAt":"%s"}\n'         "$check_run_id" "$completed_at"
      ;;
    invalid-completed-at)
      printf '{"checkId":"%s","resultKind":"policy_rejected","durationMs":7,"completedAt":"not-a-timestamp"}\n'         "$check_run_id"
      ;;
    *)
      exit 50
      ;;
  esac
  exit 0
fi

if [[ "$joined" == *"exec -T api wget"* && "$joined" == *"/availability"* ]]; then
  [[ -f "$product" ]] || exit 51
  status=unknown
  reason=policy_rejected
  evidence="\"evidence\":{\"completedAt\":\"$completed_at\",\"checkId\":\"$check_run_id\"}"
  evaluated_at="2026-09-24T22:00:08.123456Z"
  case "${FAKE_AVAILABILITY_MODE:-valid}" in
    wrong-check-id) evidence="\"evidence\":{\"completedAt\":\"$completed_at\",\"checkId\":\"wrong\"}" ;;
    stale-reason) reason=stale_result ;;
    unavailable-status) status=unavailable ;;
    missing-evidence) evidence='"extra":"missing"' ;;
    invalid-timestamp) evaluated_at=invalid ;;
    extra-evidence-key) evidence="\"evidence\":{\"completedAt\":\"$completed_at\",\"checkId\":\"$check_run_id\",\"extra\":\"bad\"}" ;;
    expired-age|future-age|wrong-completion) ;;
  esac
  printf '{ %s, "reason" : "%s", "evaluatedAt" : "%s", "status" : "%s" }\n' "$evidence" "$reason" "$evaluated_at" "$status"
  exit 0
fi

if [[ "$joined" == *"availability_age_valid"* ]]; then
  case "${FAKE_AVAILABILITY_MODE:-valid}" in
    expired-age|future-age|wrong-completion) printf 'f\n' ;;
    *) printf 't\n' ;;
  esac
  exit 0
fi

if [[ "$joined" == *"exec -T api wget"* && "$joined" == *"/monitors/018f22d3-1d6a-7cc0-a37b-46fc3fafdcb2"* ]]; then
  if [[ -f "$product" ]]; then
    printf '{"id":"018f22d3-1d6a-7cc0-a37b-46fc3fafdcb2","targetUrl":"http://web/","createdAt":"2026-09-24T22:00:00.123456Z"}\n'
    exit 0
  fi
  exit 8
fi

if [[ "$joined" == *"CREATE TABLE public.__uptime_lab_local_dev_probe"* ]]; then
  : > "$probe"
  exit 0
fi

if [[ "$joined" == *"FROM monitoring.check_runs"* && "$joined" == *"result_kind"* && "$joined" == *"duration_ms"* && "$joined" == *"completed_at IS NOT NULL"* ]]; then
  [[ -f "$product" ]] || exit 45
  printf '%s|policy_rejected||7|t\n' "$check_run_id"
  exit 0
fi

if [[ "$joined" == *"FROM monitoring.check_runs"* && "$joined" == *"count(*)"* && "$joined" == *"completed_at IS NULL"* ]]; then
  [[ -f "$product" ]] || exit 46
  printf '0\n'
  exit 0
fi

if [[ "$joined" == *"FROM monitoring.check_runs"* && "$joined" == *"count(*)"* && "$joined" == *"completed_at IS NOT NULL"* ]]; then
  [[ -f "$product" ]] || exit 47
  printf '1\n'
  exit 0
fi

if [[ "$joined" == *"logs --no-color checker"* ]]; then
  [[ -f "$product" ]] || exit 48
  printf 'checker-1 | event=check_claimed check_id=%s monitor_id=018f22d3-1d6a-7cc0-a37b-46fc3fafdcb2\n' "$check_run_id"
  printf 'checker-1 | event=probe_completed check_id=%s monitor_id=018f22d3-1d6a-7cc0-a37b-46fc3fafdcb2 result_kind=policy_rejected duration_ms=7\n' "$check_run_id"
  printf 'checker-1 | event=result_delivered check_id=%s monitor_id=018f22d3-1d6a-7cc0-a37b-46fc3fafdcb2 result_kind=policy_rejected duration_ms=7\n' "$check_run_id"
  exit 0
fi

if [[ "$joined" == *"psql"* && "$joined" == *"-Atc"* && "$joined" == *"::timestamptz IS NOT NULL"* ]]; then
  if [[ "$joined" == *"not-a-timestamp"* ]]; then
    exit 51
  fi
  printf 't\n'
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
  grep -Fq -- '--post-data={"targetUrl":"http://web/"}' "$DOCKER_LOG" || return 1
  grep -Fq '/monitors/018f22d3-1d6a-7cc0-a37b-46fc3fafdcb2' "$DOCKER_LOG" || return 1
  grep -Fq '/monitors/018f22d3-1d6a-7cc0-a37b-46fc3fafdcb2/latest-result' "$DOCKER_LOG" || return 1
  grep -Fq '/monitors/018f22d3-1d6a-7cc0-a37b-46fc3fafdcb2/availability' "$DOCKER_LOG" || return 1
  grep -Fq "::timestamptz IS NOT NULL" "$DOCKER_LOG" || return 1
  grep -Fq 'CREATE TABLE public.__uptime_lab_local_dev_probe' "$DOCKER_LOG" || return 1
  grep -Fq 'FROM monitoring.check_runs' "$DOCKER_LOG" || return 1
  grep -Fq 'completed_at IS NULL' "$DOCKER_LOG" || return 1
  grep -Fq 'completed_at IS NOT NULL' "$DOCKER_LOG" || return 1
  grep -Fq 'logs --no-color checker' "$DOCKER_LOG" || return 1
  grep -Fq 'down -v --remove-orphans' "$DOCKER_LOG" || return 1

  local start_line migrate_line wait_line post_line execution_line latest_line logs_line
  start_line="$(first_line 'up -d db api')"
  migrate_line="$(first_line 'exec -T api /usr/local/bin/uptime-lab-migrate up')"
  wait_line="$(first_line 'up -d --wait --wait-timeout 60')"
  post_line="$(first_line '--post-data={"targetUrl":"http://web/"}')"
  execution_line="$(first_line 'FROM monitoring.check_runs')"
  latest_line="$(first_line '/monitors/018f22d3-1d6a-7cc0-a37b-46fc3fafdcb2/latest-result')"
  logs_line="$(first_line 'logs --no-color checker')"

  (( start_line < migrate_line )) || return 1
  (( migrate_line < wait_line )) || return 1
  (( wait_line < post_line )) || return 1
  (( post_line < execution_line )) || return 1
  (( execution_line < latest_line )) || return 1
  (( latest_line < logs_line )) || return 1
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

case_latest_result_wrong_check_id() {
  reset_state

  if DOCKER_LOG="$DOCKER_LOG"     FAKE_STATE_DIR="$FAKE_STATE_DIR"     DOCKER_BIN="$FAKE_DOCKER"     FAKE_LATEST_RESULT_MODE=wrong-check-id     "$SMOKE" >/dev/null 2>&1; then
    return 1
  fi

  grep -Fq '/latest-result' "$DOCKER_LOG" || return 1
}

case_latest_result_exposes_http_status() {
  reset_state

  if DOCKER_LOG="$DOCKER_LOG"     FAKE_STATE_DIR="$FAKE_STATE_DIR"     DOCKER_BIN="$FAKE_DOCKER"     FAKE_LATEST_RESULT_MODE=exposes-http-status     "$SMOKE" >/dev/null 2>&1; then
    return 1
  fi

  grep -Fq '/latest-result' "$DOCKER_LOG" || return 1
}

case_latest_result_invalid_completed_at() {
  reset_state

  if DOCKER_LOG="$DOCKER_LOG"     FAKE_STATE_DIR="$FAKE_STATE_DIR"     DOCKER_BIN="$FAKE_DOCKER"     FAKE_LATEST_RESULT_MODE=invalid-completed-at     "$SMOKE" >/dev/null 2>&1; then
    return 1
  fi

  grep -Fq '/latest-result' "$DOCKER_LOG" || return 1
}

case_availability_json_helpers() {
  source <(sed -n '/^json_availability_fields()/,/^MONITOR_ID=/p' "$SMOKE" | sed '$d')
  local payload='{ "evidence": {"completedAt":"2026-10-02T12:00:00.123456Z", "checkId":"id"}, "reason":"policy_rejected", "status":"unknown", "evaluatedAt":"2026-10-02T12:00:01Z" }'
  [[ "$(json_availability_keys "$payload")" == 'evaluatedAt,evidence,reason,status' ]] || return 1
  [[ "$(json_availability_evidence_keys "$payload")" == 'checkId,completedAt' ]] || return 1
  [[ "$(json_availability_completed_at "$payload")" == '2026-10-02T12:00:00.123456Z' ]] || return 1
  [[ "$(json_availability_status "$payload")" == unknown ]] || return 1
  local malformed
  for malformed in '{"status":"unknown","status":"unknown"}' '{"evidence":[]}' '{"status":"unknown",}' '{"status":"unknown"} trailing'; do
    if json_availability_fields "$malformed" >/dev/null; then return 1; fi
  done
}

case_availability_rejects() {
  reset_state
  if DOCKER_LOG="$DOCKER_LOG" FAKE_STATE_DIR="$FAKE_STATE_DIR" DOCKER_BIN="$FAKE_DOCKER" FAKE_AVAILABILITY_MODE="$1" "$SMOKE" >/dev/null 2>&1; then return 1; fi
  grep -Fq '/availability' "$DOCKER_LOG" || return 1
}
case_availability_wrong_check_id() { case_availability_rejects wrong-check-id; }
case_availability_stale_reason() { case_availability_rejects stale-reason; }
case_availability_unavailable_status() { case_availability_rejects unavailable-status; }
case_availability_missing_evidence() { case_availability_rejects missing-evidence; }
case_availability_invalid_timestamp() { case_availability_rejects invalid-timestamp; }

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

if case_latest_result_wrong_check_id; then
  pass "latest-result rejects wrong check id"
else
  fail "latest-result rejects wrong check id"
fi

if case_latest_result_exposes_http_status; then
  pass "latest-result rejects forbidden httpStatus"
else
  fail "latest-result rejects forbidden httpStatus"
fi

if case_latest_result_invalid_completed_at; then
  pass "latest-result rejects invalid completedAt"
else
  fail "latest-result rejects invalid completedAt"
fi

if case_migration_failure_cleanup; then
  pass "migration failure triggers cleanup"
else
  fail "migration failure triggers cleanup"
fi

for case_name in case_availability_json_helpers case_availability_wrong_check_id case_availability_stale_reason case_availability_unavailable_status case_availability_missing_evidence case_availability_invalid_timestamp; do
  if "$case_name"; then pass "$case_name"; else fail "$case_name"; fi
done
for mode in extra-evidence-key expired-age future-age wrong-completion; do
  if case_availability_rejects "$mode"; then pass "availability rejects $mode"; else fail "availability rejects $mode"; fi
done
printf '\nLocal-dev smoke tests: %d passed, %d failed\n' "$PASS" "$FAIL"
test "$PASS" -eq 17
test "$FAIL" -eq 0
