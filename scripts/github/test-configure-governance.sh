#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT
mkdir -p "$TMP_DIR/bin"

cat > "$TMP_DIR/bin/gh" <<'FAKE_GH'
#!/usr/bin/env bash
set -euo pipefail

: "${GH_LOG:?GH_LOG is required}"
: "${FAKE_EXISTING_RULESET:=0}"

printf 'CALL:' >> "$GH_LOG"
printf ' %q' "$@" >> "$GH_LOG"
printf '\n' >> "$GH_LOG"

if [[ "${1:-}" == "auth" && "${2:-}" == "status" ]]; then
  exit 0
fi

if [[ "${1:-}" != "api" ]]; then
  printf 'unexpected gh command\n' >&2
  exit 2
fi

method="GET"
endpoint=""
jq_expr=""
read_stdin=0
prev=""
for arg in "$@"; do
  if [[ "$prev" == "--method" ]]; then method="$arg"; fi
  if [[ "$prev" == "--jq" ]]; then jq_expr="$arg"; fi
  if [[ "$prev" == "--input" && "$arg" == "-" ]]; then read_stdin=1; fi
  if [[ "$arg" == repos/* ]]; then endpoint="$arg"; fi
  prev="$arg"
done

if [[ "$read_stdin" -eq 1 ]]; then
  payload="$(cat)"
  printf 'PAYLOAD:%s\n' "$payload" >> "$GH_LOG"
fi

if [[ "$endpoint" == "repos/kefyusuf/uptime-lab" && "$jq_expr" == ".permissions.admin" ]]; then
  printf 'true\n'
  exit 0
fi

if [[ "$endpoint" == "repos/kefyusuf/uptime-lab" && "$method" == "PATCH" ]]; then
  exit 0
fi

if [[ "$endpoint" == "repos/kefyusuf/uptime-lab/rulesets" && "$method" == "GET" ]]; then
  if [[ "$FAKE_EXISTING_RULESET" == "1" ]]; then printf '123\n'; fi
  exit 0
fi

if [[ "$endpoint" == "repos/kefyusuf/uptime-lab/rulesets" && "$method" == "POST" ]]; then
  printf '123\n'
  exit 0
fi

if [[ "$endpoint" == "repos/kefyusuf/uptime-lab/rulesets/123" && "$method" == "PUT" ]]; then
  printf '123\n'
  exit 0
fi

if [[ "$endpoint" == "repos/kefyusuf/uptime-lab" && "$jq_expr" == *"allow_squash_merge"* ]]; then
  printf 'true\tfalse\tfalse\ttrue\n'
  exit 0
fi

if [[ "$endpoint" == "repos/kefyusuf/uptime-lab/rulesets/123" ]]; then
  case "$jq_expr" in
    *".name,.target,.enforcement"*) printf 'main-protection\tbranch\tactive\n' ;;
    *".conditions.ref_name.include"*) printf 'refs/heads/main\n' ;;
    *".rules[].type"*) printf 'deletion,non_fast_forward,pull_request,required_linear_history,required_status_checks\n' ;;
    *"required_approving_review_count"*) printf '0\ttrue\tsquash\n' ;;
    *"strict_required_status_checks_policy"*) printf 'true\tCI / gate\n' ;;
    *) printf 'unexpected jq expression: %s\n' "$jq_expr" >&2; exit 2 ;;
  esac
  exit 0
fi

printf 'unexpected endpoint/method: %s %s (%s)\n' "$method" "$endpoint" "$jq_expr" >&2
exit 2
FAKE_GH
chmod +x "$TMP_DIR/bin/gh"

run_case() {
  local existing="$1"
  local expected_method="$2"
  : > "$TMP_DIR/gh.log"
  GH_LOG="$TMP_DIR/gh.log" FAKE_EXISTING_RULESET="$existing" PATH="$TMP_DIR/bin:$PATH" \
    "$SCRIPT_DIR/configure-governance.sh" >/dev/null

  grep -E -- 'CALL:.*--method PATCH.*repos/kefyusuf/uptime-lab' "$TMP_DIR/gh.log" >/dev/null
  grep -F -- '-F allow_squash_merge=true' "$TMP_DIR/gh.log" >/dev/null
  grep -F -- '-F allow_merge_commit=false' "$TMP_DIR/gh.log" >/dev/null
  grep -F -- '-F allow_rebase_merge=false' "$TMP_DIR/gh.log" >/dev/null
  grep -F -- '-F delete_branch_on_merge=true' "$TMP_DIR/gh.log" >/dev/null
  grep -E -- "CALL:.*--method $expected_method.*repos/kefyusuf/uptime-lab/rulesets" "$TMP_DIR/gh.log" >/dev/null
  grep -F -- '"name": "main-protection"' "$TMP_DIR/gh.log" >/dev/null
  grep -F -- '"include": ["refs/heads/main"]' "$TMP_DIR/gh.log" >/dev/null
  grep -F -- '"allowed_merge_methods": ["squash"]' "$TMP_DIR/gh.log" >/dev/null
  grep -F -- '"required_approving_review_count": 0' "$TMP_DIR/gh.log" >/dev/null
  grep -F -- '"required_review_thread_resolution": true' "$TMP_DIR/gh.log" >/dev/null
  grep -F -- '"context": "CI / gate"' "$TMP_DIR/gh.log" >/dev/null
  grep -F -- '"strict_required_status_checks_policy": true' "$TMP_DIR/gh.log" >/dev/null
}

run_case 0 POST
run_case 1 PUT
printf 'GitHub governance bootstrap tests: 2 passed, 0 failed\n'
