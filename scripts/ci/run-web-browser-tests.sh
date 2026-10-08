#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)"
DOCKER_BIN="${DOCKER_BIN:-docker}"
NPM_BIN="${NPM_BIN:-npm}"
export UPTIME_LAB_WEB_PORT="${UPTIME_LAB_WEB_PORT:-4173}"
export COMPOSE_PROJECT_NAME="uptime-lab-web-$$-$RANDOM"
mkdir -p "$ROOT/.cache"
WORK="$(mktemp -d "$ROOT/.cache/web-browser.XXXXXX")"
export WEB_BROWSER_ARTIFACT="$WORK/browser.json"
if command -v cygpath >/dev/null;then WEB_BROWSER_ARTIFACT="$(cygpath -m "$WEB_BROWSER_ARTIFACT")";export WEB_BROWSER_ARTIFACT;fi
compose() { "$DOCKER_BIN" compose -f "$ROOT/compose.yaml" -f "$ROOT/compose.web-local.yaml" "$@"; }
cleanup() { compose down -v --remove-orphans --rmi local >/dev/null 2>&1 || true;rm -rf "$WORK"; }
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
node --input-type=module -e 'import net from "node:net";const port=process.env.UPTIME_LAB_WEB_PORT;if(!/^[1-9][0-9]*$/.test(port)||+port<1024||+port>65535)throw new Error("Invalid local browser port");const server=net.createServer();server.once("error",()=>{console.error("Local Web port is occupied");process.exitCode=1;});server.listen(+port,"127.0.0.1",()=>server.close());'
"$DOCKER_BIN" compose -f "$ROOT/compose.yaml" config --format json > "$WORK/canonical.json"
compose config --format json > "$WORK/local.json"
node "$ROOT/scripts/ci/check-web-compose.mjs" "$WORK/canonical.json" "$WORK/local.json" "$UPTIME_LAB_WEB_PORT"
compose build api web checker
compose up -d db api web
live=false
for ((attempt=1;attempt<=30;attempt++));do
  if [[ "$(compose exec -T api wget -q -O - http://127.0.0.1:8080/livez 2>/dev/null)" == ok ]];then live=true;break;fi;sleep 1
done
[[ "$live" == true ]] || { printf 'API did not become live\n' >&2;exit 1; }
if compose exec -T api wget -q -O - http://127.0.0.1:8080/readyz >/dev/null 2>&1;then printf 'API ready before explicit migration\n' >&2;exit 1;fi
compose exec -T api /usr/local/bin/uptime-lab-migrate up
compose up -d --wait --wait-timeout 60 db api web
compose exec -T web node --input-type=module -e 'import fs from "node:fs";if(process.getuid()!==10001||process.getgid()!==10001)throw new Error("Web must be non-root");for(const path of ["/app/node_modules","/app/src","/app/server","/app/e2e"])if(fs.existsSync(path))throw new Error("Development source in runtime");try{fs.writeFileSync("/app/write-probe","x");throw new Error("Runtime is writable");}catch(error){if(error.code!=="EROFS"&&error.code!=="EACCES")throw error;}const r=await fetch("http://127.0.0.1:8080/healthz");if(!r.ok)throw new Error("Missing healthy build assets");'
browser() { (cd "$ROOT/apps/web";WEB_BROWSER_PHASE="$1" "$NPM_BIN" run test:e2e); }
browser register
compose up -d --wait --wait-timeout 60 checker
browser result
scheduling() { node "$ROOT/scripts/ci/web-scheduling-evidence.mjs" "$1" "$WEB_BROWSER_ARTIFACT" "${@:2}"; }
sql() { compose exec -T db sh -lc 'psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Atc "$1"' sh "$1"; }
browser pause
SNAPSHOT_SQL="$(scheduling sql-snapshot)"
settled=false
for ((attempt=1;attempt<=20;attempt++));do
  sql "$SNAPSHOT_SQL" > "$WORK/snapshot.json"
  if node --input-type=module -e 'import fs from "node:fs";const s=JSON.parse(fs.readFileSync(process.argv[1],"utf8"));process.exit(s.pending===0?0:1);' "$WORK/snapshot.json";then settled=true;break;fi;sleep 1
done
[[ "$settled" == true ]] || { printf 'Earlier pending work did not settle\n' >&2;exit 1; }
scheduling record-snapshot before "$WORK/snapshot.json"
# Pause only this isolated fixture's other Monitors; preserve captured terminal timestamps.
sql "$(scheduling sql-prepare)" >/dev/null
due=false
for ((attempt=1;attempt<=65;attempt++));do
  sql "$SNAPSHOT_SQL" > "$WORK/snapshot.json"
  scheduling record-snapshot during "$WORK/snapshot.json"
  if node --input-type=module -e 'import fs from "node:fs";const s=JSON.parse(fs.readFileSync(process.argv[1],"utf8"));process.exit(s.due===true?0:1);' "$WORK/snapshot.json";then due=true;break;fi;sleep 1
done
[[ "$due" == true ]] || { printf 'Paused fixture did not reach existing cadence boundary\n' >&2;exit 1; }
for service in api checker;do
  container="$(compose ps -q "$service")"
  scheduling restart "$service" before "$("$DOCKER_BIN" inspect --format '{{.State.StartedAt}}' "$container")"
  compose restart "$service"
  compose up -d --wait --wait-timeout 60 "$service"
  scheduling restart "$service" after "$("$DOCKER_BIN" inspect --format '{{.State.StartedAt}}' "$(compose ps -q "$service")")"
done
browser paused
opportunities=false
for ((attempt=1;attempt<=20;attempt++));do
  count="$(compose logs --no-color --since "$(node --input-type=module -e 'import fs from "node:fs";console.log(JSON.parse(fs.readFileSync(process.argv[1],"utf8")).scheduling.restarts.checker.afterStartedAt);' "$WEB_BROWSER_ARTIFACT")" checker 2>&1 | grep -c 'event=claim_no_work' || true)"
  if (( count >= 3 ));then opportunities=true;break;fi;sleep 1
done
[[ "$opportunities" == true ]] || { printf 'Missing real paused claim opportunities\n' >&2;exit 1; }
sql "$SNAPSHOT_SQL" > "$WORK/snapshot.json"
scheduling record-snapshot during "$WORK/snapshot.json"
browser resume
resumed=false
for ((attempt=1;attempt<=20;attempt++));do
  sql "$SNAPSHOT_SQL" > "$WORK/snapshot.json"
  if scheduling record-resume "$WORK/snapshot.json" 2>/dev/null;then resumed=true;break;fi;sleep 1
done
[[ "$resumed" == true ]] || { printf 'Missing real resumed claim\n' >&2;exit 1; }
SQL="$(scheduling sql-rows)"
compose exec -T db sh -lc 'psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Atc "$1"' sh "$SQL" > "$WORK/rows.json"
MONITOR_SQL="$(node --input-type=module -e 'import fs from "node:fs";const c=JSON.parse(fs.readFileSync(process.argv[1],"utf8")),ids=c.inventory?.ids;if(!Array.isArray(ids)||ids.length!==21||new Set(ids).size!==21||!ids.every(id=>typeof id==="string"&&/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/.test(id)))throw new Error("Invalid captured inventory UUIDs");const q=String.fromCharCode(39);console.log(`SELECT coalesce(json_agg(row_to_json(m)), ${q}[]${q}::json) FROM (SELECT id,target_url,paused FROM monitoring.monitors WHERE id IN (${ids.map(id=>q+id+q).join(",")})) m;`);' "$WEB_BROWSER_ARTIFACT")"
compose exec -T db sh -lc 'psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Atc "$1"' sh "$MONITOR_SQL" > "$WORK/monitors.json"
node "$ROOT/scripts/ci/verify-web-evidence.mjs" "$WEB_BROWSER_ARTIFACT" "$WORK/rows.json" "$WORK/monitors.json"
printf 'Real browser/Go/Checker/PostgreSQL journey passed on loopback port %s\n' "$UPTIME_LAB_WEB_PORT"
