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

json_check_id() {
  printf '%s\n' "$1" | sed -n 's/.*"checkId":"\([^"]*\)".*/\1/p'
}

json_result_kind() {
  printf '%s\n' "$1" | sed -n 's/.*"resultKind":"\([^"]*\)".*/\1/p'
}

json_duration_ms() {
  printf '%s\n' "$1" | sed -n 's/.*"durationMs":\([0-9][0-9]*\).*/\1/p'
}

json_completed_at() {
  printf '%s\n' "$1" | sed -n 's/.*"completedAt":"\([^"]*\)".*/\1/p'
}

json_keys() {
  printf '%s\n' "$1" |
    grep -oE '"[^"]+":' |
    sed 's/^"//; s/":$//' |
    sort |
    paste -sd, -
}

# Parse only the closed string/object shape emitted by the availability route.
# Reject malformed JSON and duplicate keys rather than trusting substring matches.
json_availability_fields() {
  printf '%s\n' "$1" | awk '
    function ws() { while (substr(text,pos,1) ~ /[ \t\r\n]/ && pos<=length(text)) pos++ }
    function quoted(    start,value) {
      ws(); if(substr(text,pos,1)!="\"") exit 1
      start=++pos
      while(pos<=length(text) && substr(text,pos,1)!="\"") {
        if(substr(text,pos,1)=="\\" || substr(text,pos,1) ~ /[[:cntrl:]]/) exit 1
        pos++
      }
      if(pos>length(text)) exit 1
      value=substr(text,start,pos-start);pos++;return value
    }
    function object(prefix,    key,full,value,seen,count) {
      ws();if(substr(text,pos++,1)!="{") exit 1
      ws();if(substr(text,pos,1)=="}"){pos++;return}
      while(1) {
        key=quoted();full=prefix key
        if(full in names) exit 1
        names[full]=1
        ws();if(substr(text,pos++,1)!=":") exit 1
        ws()
        if(substr(text,pos,1)=="{") {
          if(full!="evidence") exit 1
          object(full ".")
        } else values[full]=quoted()
        ws();value=substr(text,pos++,1)
        if(value=="}") return
        if(value!=",") exit 1
      }
    }
    {text=text $0 "\n"}
    END {
      pos=1;object("");ws();if(pos<=length(text)) exit 1
      for(key in names) print "key\t" key
      for(key in values) print key "\t" values[key]
    }'
}
json_availability_value() { json_availability_fields "$1" | awk -F '\t' -v key="$2" '$1==key {print $2}'; }
json_availability_status() { json_availability_value "$1" status; }
json_availability_reason() { json_availability_value "$1" reason; }
json_availability_evidence_check_id() { json_availability_value "$1" evidence.checkId; }
json_availability_completed_at() { json_availability_value "$1" evidence.completedAt; }
json_availability_evaluated_at() { json_availability_value "$1" evaluatedAt; }
json_availability_keys() { json_availability_fields "$1" | awk -F '\t' '$1=="key" && $2 !~ /\./ {print $2}' | sort | paste -sd, -; }
json_availability_evidence_keys() { json_availability_fields "$1" | awk -F '\t' '$1=="key" && $2 ~ /^evidence\./ {sub(/^evidence\./,"",$2);print $2}' | sort | paste -sd, -; }

MONITOR_ID=""
MONITOR_TARGET="http://web/"
MONITOR_CREATED_AT=""
CHECK_RUN_ID=""
CHECK_RUN_DURATION_MS=""

create_monitor() {
  local response
  local target
  local created_at

  response="$(compose exec -T api wget -q -O - \
    --header='Content-Type: application/json' \
    --post-data='{"targetUrl":"http://web/"}' \
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

query_check_runs() {
  local sql="$1"
  compose exec -T db sh -lc \
    'psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Atc "$1"' \
    sh "$sql"
}

wait_for_policy_rejected_check_run() {
  local attempt
  local row=""
  local kind
  local http_status
  local terminal
  local pending_count
  local terminal_count

  for ((attempt = 1; attempt <= 30; attempt++)); do
    row="$(query_check_runs "
      SELECT
        id::text || '|' ||
        result_kind || '|' ||
        COALESCE(http_status::text, '') || '|' ||
        COALESCE(duration_ms::text, '') || '|' ||
        CASE WHEN completed_at IS NOT NULL THEN 't' ELSE 'f' END
      FROM monitoring.check_runs
      WHERE monitor_id = '$MONITOR_ID'::uuid
        AND completed_at IS NOT NULL
      ORDER BY completed_at ASC;
    ")"

    if [[ -n "$row" ]]; then
      break
    fi
    sleep 1
  done

  [[ -n "$row" ]] || {
    printf 'No terminal CheckRun appeared for Monitor %s\n' "$MONITOR_ID" >&2
    return 1
  }

  [[ "$row" != *$'\n'* ]] || {
    printf 'Expected exactly one terminal CheckRun row, got multiple rows: %s\n' "$row" >&2
    return 1
  }

  IFS='|' read -r CHECK_RUN_ID kind http_status CHECK_RUN_DURATION_MS terminal <<<"$row"

  [[ "$CHECK_RUN_ID" =~ ^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$ ]] || {
    printf 'Terminal CheckRun id is not a canonical UUID: %q\n' "$CHECK_RUN_ID" >&2
    return 1
  }
  [[ "$kind" == "policy_rejected" ]] || {
    printf 'Terminal CheckRun result_kind=%q, want policy_rejected\n' "$kind" >&2
    return 1
  }
  [[ -z "$http_status" ]] || {
    printf 'policy_rejected CheckRun has unexpected http_status=%q\n' "$http_status" >&2
    return 1
  }
  [[ "$CHECK_RUN_DURATION_MS" =~ ^[0-9]+$ ]] || {
    printf 'policy_rejected CheckRun duration_ms=%q is not an integer\n' "$CHECK_RUN_DURATION_MS" >&2
    return 1
  }
  (( CHECK_RUN_DURATION_MS <= 20000 )) || {
    printf 'policy_rejected CheckRun duration_ms=%s exceeds contract maximum\n' "$CHECK_RUN_DURATION_MS" >&2
    return 1
  }
  [[ "$terminal" == "t" ]] || {
    printf 'CheckRun %s is not terminal\n' "$CHECK_RUN_ID" >&2
    return 1
  }

  pending_count="$(query_check_runs "
    SELECT count(*)
    FROM monitoring.check_runs
    WHERE monitor_id = '$MONITOR_ID'::uuid
      AND completed_at IS NULL;
  ")"
  terminal_count="$(query_check_runs "
    SELECT count(*)
    FROM monitoring.check_runs
    WHERE monitor_id = '$MONITOR_ID'::uuid
      AND completed_at IS NOT NULL;
  ")"

  [[ "$pending_count" == "0" ]] || {
    printf 'Monitor %s has %s pending CheckRun rows, want 0\n' "$MONITOR_ID" "$pending_count" >&2
    return 1
  }
  [[ "$terminal_count" == "1" ]] || {
    printf 'Monitor %s has %s terminal CheckRun rows, want 1\n' "$MONITOR_ID" "$terminal_count" >&2
    return 1
  }

  printf 'PostgreSQL execution evidence: monitor_id=%s check_id=%s result_kind=policy_rejected duration_ms=%s\n' \
    "$MONITOR_ID" "$CHECK_RUN_ID" "$CHECK_RUN_DURATION_MS"
}

assert_latest_result_public_read() {
  local response
  local keys
  local check_id
  local kind
  local duration_ms
  local completed_at
  local parseable

  response="$(compose exec -T api wget -q -O -     "http://127.0.0.1:8080/monitors/$MONITOR_ID/latest-result")"

  keys="$(json_keys "$response")"
  check_id="$(json_check_id "$response")"
  kind="$(json_result_kind "$response")"
  duration_ms="$(json_duration_ms "$response")"
  completed_at="$(json_completed_at "$response")"

  [[ "$keys" == "checkId,completedAt,durationMs,resultKind" ]] || {
    printf 'GET latest-result keys=%q, want exact checkId,completedAt,durationMs,resultKind; response=%s\n'       "$keys" "$response" >&2
    return 1
  }
  [[ "$check_id" == "$CHECK_RUN_ID" ]] || {
    printf 'GET latest-result checkId=%q, want persisted %q\n' "$check_id" "$CHECK_RUN_ID" >&2
    return 1
  }
  [[ "$kind" == "policy_rejected" ]] || {
    printf 'GET latest-result resultKind=%q, want policy_rejected\n' "$kind" >&2
    return 1
  }
  [[ "$duration_ms" == "$CHECK_RUN_DURATION_MS" ]] || {
    printf 'GET latest-result durationMs=%q, want persisted %q\n'       "$duration_ms" "$CHECK_RUN_DURATION_MS" >&2
    return 1
  }
  [[ "$response" != *'"httpStatus":'* ]] || {
    printf 'GET latest-result unexpectedly exposes httpStatus for policy_rejected: %s\n'       "$response" >&2
    return 1
  }
  [[ "$completed_at" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(\.[0-9]+)?Z$ ]] || {
    printf 'GET latest-result completedAt=%q is not canonical UTC RFC3339\n'       "$completed_at" >&2
    return 1
  }

  parseable="$(query_check_runs "SELECT '$completed_at'::timestamptz IS NOT NULL;")"
  [[ "$parseable" == "t" ]] || {
    printf 'GET latest-result completedAt=%q is not parseable by PostgreSQL\n'       "$completed_at" >&2
    return 1
  }

  printf 'Public latest-result evidence: monitor_id=%s check_id=%s result_kind=%s duration_ms=%s completed_at=%s\n'     "$MONITOR_ID" "$check_id" "$kind" "$duration_ms" "$completed_at"
}

assert_availability_public_read() {
  local response status reason check_id completed_at evaluated_at valid
  response="$(compose exec -T api wget -q -O - "http://127.0.0.1:8080/monitors/$MONITOR_ID/availability")"
  [[ "$(json_availability_keys "$response")" == "evaluatedAt,evidence,reason,status" && "$(json_availability_evidence_keys "$response")" == "checkId,completedAt" ]] || {
    printf 'GET availability must have exact assessment/evidence keys: %s\n' "$response" >&2; return 1;
  }
  status="$(json_availability_status "$response")"
  reason="$(json_availability_reason "$response")"
  check_id="$(json_availability_evidence_check_id "$response")"
  completed_at="$(json_availability_completed_at "$response")"
  evaluated_at="$(json_availability_evaluated_at "$response")"
  [[ "$status" == unknown && "$reason" == policy_rejected && "$check_id" == "$CHECK_RUN_ID" ]] || {
    printf 'GET availability must identify fresh policy_rejected evidence: %s\n' "$response" >&2; return 1;
  }
  local timestamp
  for timestamp in "$completed_at" "$evaluated_at"; do
    [[ "$timestamp" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(\.[0-9]+)?Z$ ]] || {
      printf 'GET availability invalid UTC timestamp: %q\n' "$timestamp" >&2; return 1;
    }
  done
  valid="$(query_check_runs "SELECT (completed_at = '$completed_at'::timestamptz AND '$evaluated_at'::timestamptz - completed_at BETWEEN interval '0 seconds' AND interval '120 seconds') AS availability_age_valid FROM monitoring.check_runs WHERE id = '$CHECK_RUN_ID'::uuid;")"
  [[ "$valid" == t ]] || { printf 'GET availability completion or freshness does not match durable evidence\n' >&2; return 1; }
  printf 'Public availability evidence: status=%s reason=%s check_id=%s completed_at=%s evaluated_at=%s\n' "$status" "$reason" "$check_id" "$completed_at" "$evaluated_at"
}

assert_checker_execution_events() {
  local logs
  local claimed_line
  local probed_line
  local delivered_line

  logs="$(compose logs --no-color checker)"

  claimed_line="$(printf '%s\n' "$logs" | grep -n -F -m1 \
    "event=check_claimed check_id=$CHECK_RUN_ID monitor_id=$MONITOR_ID" | cut -d: -f1)"
  probed_line="$(printf '%s\n' "$logs" | grep -n -F -m1 \
    "event=probe_completed check_id=$CHECK_RUN_ID monitor_id=$MONITOR_ID result_kind=policy_rejected duration_ms=$CHECK_RUN_DURATION_MS" | cut -d: -f1)"
  delivered_line="$(printf '%s\n' "$logs" | grep -n -F -m1 \
    "event=result_delivered check_id=$CHECK_RUN_ID monitor_id=$MONITOR_ID result_kind=policy_rejected duration_ms=$CHECK_RUN_DURATION_MS" | cut -d: -f1)"

  [[ -n "$claimed_line" && -n "$probed_line" && -n "$delivered_line" ]] || {
    printf 'Checker logs do not contain the complete cross-runtime execution evidence for CheckRun %s\n' "$CHECK_RUN_ID" >&2
    return 1
  }

  (( claimed_line < probed_line && probed_line < delivered_line )) || {
    printf 'Checker execution events are out of order for CheckRun %s\n' "$CHECK_RUN_ID" >&2
    return 1
  }

  printf 'Checker execution evidence: check_claimed -> probe_completed(policy_rejected) -> result_delivered\n'
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

# Prove the real cross-runtime execution path:
# Go claim -> Rust Checker -> production private-address policy -> Go result -> PostgreSQL terminal CheckRun.
wait_for_policy_rejected_check_run
assert_latest_result_public_read
assert_availability_public_read
assert_checker_execution_events

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
