#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)"
TMP="$(mktemp -d)";trap 'rm -rf "$TMP"' EXIT
awk '/^  gate:/ {gate=1} gate && /^        run: \|/ {body=1;next} body {sub(/^          /, "");print}' "$ROOT/.github/workflows/ci.yml" > "$TMP/gate.sh"
grep -q WEB_RESULT "$TMP/gate.sh"
export POLICY_RESULT=success REPOSITORY_RESULT=success CHANGES_RESULT=success LOCAL_DEV_RESULT=success GO_API_RESULT=success PUBLIC_CONTRACT_RESULT=success INTERNAL_CONTRACT_RESULT=success CHECKER_RESULT=success
for result in success failure cancelled skipped '';do
  status=0;WEB_REQUIRED=true WEB_RESULT="$result" bash "$TMP/gate.sh" >/dev/null 2>&1 || status=$?
  if [[ "$result" == success ]];then [[ "$status" == 0 ]];else [[ "$status" != 0 ]];fi
done
WEB_REQUIRED=false WEB_RESULT=skipped bash "$TMP/gate.sh"
if WEB_REQUIRED=false WEB_RESULT=cancelled bash "$TMP/gate.sh";then exit 1;fi
printf 'Web gate: 7 cases passed\n'
