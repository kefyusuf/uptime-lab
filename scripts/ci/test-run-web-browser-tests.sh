#!/usr/bin/env bash
set -euo pipefail
SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
TMP="$(mktemp -d)"
LISTENER_PID=''
trap 'if [[ -n "$LISTENER_PID" ]];then kill "$LISTENER_PID" 2>/dev/null || true;fi;rm -rf "$TMP"' EXIT
export DOCKER_LOG="$TMP/docker.log"
export BROWSER_LOG="$TMP/browser.log"
cat > "$TMP/docker" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >> "$DOCKER_LOG"
if [[ "$*" == *'config --format json'* ]]; then
  ports='';[[ "$*" != *'compose.web-local.yaml'* ]] || ports=',"ports":[{"host_ip":"127.0.0.1","published":"4173","target":8080,"protocol":"tcp"}]'
  printf '{"services":{"api":{},"db":{},"checker":{},"web":{"build":{"dockerfile":"apps/web/Dockerfile"},"init":true,"read_only":true,"environment":{"UPTIME_LAB_WEB_PORT":"4173"}%s}}}\n' "$ports"
elif [[ "$*" == *'up -d db api web'* && "$SCENARIO" == startup ]];then exit 143
elif [[ "$*" == *'/readyz'* && "$*" == *'exec -T api'* ]];then
  if [[ ! -f "$DOCKER_LOG.migrated" ]];then exit 1;fi;printf 'ok'
elif [[ "$*" == *'/livez'* ]];then printf 'ok'
elif [[ "$*" == *'uptime-lab-migrate up'* ]];then
  [[ "$SCENARIO" != migration ]] || exit 1;touch "$DOCKER_LOG.migrated"
elif [[ "$*" == *'SELECT coalesce'* ]];then
  printf '[{"id":"bb29443e-c597-4e7b-9202-a4762e0e04c0","monitor_id":"aa29443e-c597-4e7b-9202-a4762e0e04c0","result_kind":"policy_rejected","completed_at":"2026-10-02T12:00:00+00:00","http_status":null,"duration_ms":0},{"id":"cc29443e-c597-4e7b-9202-a4762e0e04c0","monitor_id":"aa29443e-c597-4e7b-9202-a4762e0e04c0","result_kind":"policy_rejected","completed_at":"2026-10-02T12:00:01+00:00","http_status":null,"duration_ms":0}]'
fi
SH
cat > "$TMP/npm" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$WEB_BROWSER_PHASE" >> "$BROWSER_LOG"
[[ "$SCENARIO" != browser ]] || exit 1
printf '{"id":"aa29443e-c597-4e7b-9202-a4762e0e04c0","target":"http://web/","raw":{"checkId":"bb29443e-c597-4e7b-9202-a4762e0e04c0","completedAt":"2026-10-02T12:00:00Z","durationMs":0},"availability":{"checkId":"cc29443e-c597-4e7b-9202-a4762e0e04c0","completedAt":"2026-10-02T12:00:01Z"}}' > "$WEB_BROWSER_ARTIFACT"
SH
chmod +x "$TMP/docker" "$TMP/npm"
for scenario in success migration browser startup;do
  export SCENARIO="$scenario";: > "$DOCKER_LOG";: > "$BROWSER_LOG";rm -f "$DOCKER_LOG.migrated"
  status=0;DOCKER_BIN="$TMP/docker" NPM_BIN="$TMP/npm" bash "$SCRIPT_DIR/run-web-browser-tests.sh" > "$TMP/output" 2>&1 || status=$?
  if [[ "$scenario" == success ]];then [[ "$status" == 0 ]] || { cat "$TMP/output";exit 1;};grep -Fxq register "$BROWSER_LOG";grep -Fxq result "$BROWSER_LOG";grep -Fq 'up -d --wait --wait-timeout 60 checker' "$DOCKER_LOG"
  else [[ "$status" != 0 ]] || { printf 'Unexpected success: %s\n' "$scenario";exit 1;};fi
  grep -Fq 'down -v --remove-orphans' "$DOCKER_LOG" || { printf 'Missing cleanup: %s\n' "$scenario";exit 1;}
  printf 'PASS: %s orchestration and cleanup\n' "$scenario"
done
node --input-type=module -e 'import net from "node:net";net.createServer().listen(48173,"127.0.0.1",()=>console.log("ready"));' > "$TMP/listener" 2>&1 &
LISTENER_PID=$!
for ((i=0;i<30;i++));do grep -q ready "$TMP/listener" && break;sleep 0.1;done
grep -q ready "$TMP/listener"
: > "$DOCKER_LOG"
if UPTIME_LAB_WEB_PORT=48173 DOCKER_BIN="$TMP/docker" NPM_BIN="$TMP/npm" bash "$SCRIPT_DIR/run-web-browser-tests.sh" > "$TMP/collision" 2>&1;then printf 'Occupied port unexpectedly accepted\n' >&2;exit 1;fi
grep -q 'port is occupied' "$TMP/collision"
! grep -q 'build api web checker' "$DOCKER_LOG"
grep -q 'down -v --remove-orphans' "$DOCKER_LOG"
kill "$LISTENER_PID";wait "$LISTENER_PID" 2>/dev/null || true;LISTENER_PID=''
printf 'PASS: occupied loopback port fails before build\n'
printf 'Browser runner harness: 5 passed (fake orchestration only)\n'
