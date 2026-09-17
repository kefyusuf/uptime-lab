#!/usr/bin/env bash
set -euo pipefail

REPO="${REPO:-kefyusuf/uptime-lab}"
RULESET_NAME="${RULESET_NAME:-main-protection}"
API_VERSION="${GITHUB_API_VERSION:-2026-03-10}"
REQUIRED_CHECK="${REQUIRED_CHECK:-CI / gate}"

require_command() {
  command -v "$1" >/dev/null 2>&1 || {
    printf 'Required command not found: %s\n' "$1" >&2
    exit 2
  }
}

api() {
  gh api -H "Accept: application/vnd.github+json" \
    -H "X-GitHub-Api-Version: ${API_VERSION}" "$@"
}

require_command gh
gh auth status >/dev/null

if [[ "$(api "repos/${REPO}" --jq '.permissions.admin')" != "true" ]]; then
  printf 'The authenticated GitHub account must have admin access to %s.\n' "$REPO" >&2
  exit 1
fi

printf 'Configuring merge policy for %s...\n' "$REPO"
api --method PATCH "repos/${REPO}" \
  -F allow_squash_merge=true \
  -F allow_merge_commit=false \
  -F allow_rebase_merge=false \
  -F delete_branch_on_merge=true \
  --silent

ruleset_payload="$(cat <<JSON
{
  "name": "${RULESET_NAME}",
  "target": "branch",
  "enforcement": "active",
  "bypass_actors": [],
  "conditions": {
    "ref_name": {
      "include": ["refs/heads/main"],
      "exclude": []
    }
  },
  "rules": [
    {"type": "deletion"},
    {"type": "non_fast_forward"},
    {"type": "required_linear_history"},
    {
      "type": "pull_request",
      "parameters": {
        "allowed_merge_methods": ["squash"],
        "dismiss_stale_reviews_on_push": false,
        "require_code_owner_review": false,
        "require_last_push_approval": false,
        "required_approving_review_count": 0,
        "required_review_thread_resolution": true
      }
    },
    {
      "type": "required_status_checks",
      "parameters": {
        "do_not_enforce_on_create": false,
        "required_status_checks": [
          {"context": "${REQUIRED_CHECK}"}
        ],
        "strict_required_status_checks_policy": true
      }
    }
  ]
}
JSON
)"

mapfile -t matching_rulesets < <(api "repos/${REPO}/rulesets" --paginate \
  --jq ".[] | select(.name == \"${RULESET_NAME}\") | .id")

if [[ "${#matching_rulesets[@]}" -gt 1 ]]; then
  printf 'Multiple rulesets named %s exist; refusing to guess which one to update.\n' "$RULESET_NAME" >&2
  exit 1
fi

ruleset_id="${matching_rulesets[0]:-}"

if [[ -n "$ruleset_id" ]]; then
  printf 'Updating ruleset %s (%s)...\n' "$RULESET_NAME" "$ruleset_id"
  ruleset_id="$(printf '%s' "$ruleset_payload" | api --method PUT \
    "repos/${REPO}/rulesets/${ruleset_id}" --input - --jq '.id')"
else
  printf 'Creating ruleset %s...\n' "$RULESET_NAME"
  ruleset_id="$(printf '%s' "$ruleset_payload" | api --method POST \
    "repos/${REPO}/rulesets" --input - --jq '.id')"
fi

printf 'Verifying repository settings...\n'
repo_settings="$(api "repos/${REPO}" \
  --jq '[.allow_squash_merge,.allow_merge_commit,.allow_rebase_merge,.delete_branch_on_merge] | @tsv')"
if [[ "$repo_settings" != $'true\tfalse\tfalse\ttrue' ]]; then
  printf 'Unexpected repository merge settings: %s\n' "$repo_settings" >&2
  exit 1
fi

ruleset_summary="$(api "repos/${REPO}/rulesets/${ruleset_id}" \
  --jq '[.name,.target,.enforcement] | @tsv')"
expected_summary="${RULESET_NAME}"$'\tbranch\tactive'
if [[ "$ruleset_summary" != "$expected_summary" ]]; then
  printf 'Unexpected ruleset summary: %s\n' "$ruleset_summary" >&2
  exit 1
fi

included_refs="$(api "repos/${REPO}/rulesets/${ruleset_id}" \
  --jq '[.conditions.ref_name.include[]] | join(",")')"
if [[ "$included_refs" != 'refs/heads/main' ]]; then
  printf 'Ruleset does not target only main: %s\n' "$included_refs" >&2
  exit 1
fi

rule_types="$(api "repos/${REPO}/rulesets/${ruleset_id}" \
  --jq '[.rules[].type] | sort | join(",")')"
if [[ "$rule_types" != 'deletion,non_fast_forward,pull_request,required_linear_history,required_status_checks' ]]; then
  printf 'Unexpected ruleset rule types: %s\n' "$rule_types" >&2
  exit 1
fi

pull_request_rule="$(api "repos/${REPO}/rulesets/${ruleset_id}" \
  --jq '.rules[] | select(.type=="pull_request") | [.parameters.required_approving_review_count,.parameters.required_review_thread_resolution,(.parameters.allowed_merge_methods|join(","))] | @tsv')"
if [[ "$pull_request_rule" != $'0\ttrue\tsquash' ]]; then
  printf 'Unexpected pull-request rule: %s\n' "$pull_request_rule" >&2
  exit 1
fi

status_rule="$(api "repos/${REPO}/rulesets/${ruleset_id}" \
  --jq '.rules[] | select(.type=="required_status_checks") | [.parameters.strict_required_status_checks_policy,(.parameters.required_status_checks|map(.context)|join(","))] | @tsv')"
expected_status_rule=$'true\t'"${REQUIRED_CHECK}"
if [[ "$status_rule" != "$expected_status_rule" ]]; then
  printf 'Unexpected status-check rule: %s\n' "$status_rule" >&2
  exit 1
fi

printf 'GitHub governance configured and verified for %s.\n' "$REPO"
