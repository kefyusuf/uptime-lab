#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
DETECT="$SCRIPT_DIR/detect-internal-contract-changes.sh"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

PASS=0
FAIL=0

pass() { printf 'PASS: %s\n' "$1"; PASS=$((PASS + 1)); }
fail() { printf 'FAIL: %s\n' "$1" >&2; FAIL=$((FAIL + 1)); }

expect_value() {
  local name="$1"
  local expected="$2"
  shift 2
  local actual
  if actual="$("$@" 2>/dev/null)" && [[ "$actual" == "$expected" ]]; then pass "$name"; else fail "$name"; fi
}

expect_failure() {
  local name="$1"
  shift
  if "$@" >/dev/null 2>&1; then fail "$name"; else pass "$name"; fi
}

reset_base() {
  git -C "$TMP" reset --hard -q "$BASE"
  git -C "$TMP" clean -fdq
}

commit_file() {
  local path="$1"
  local content="$2"
  mkdir -p "$TMP/$(dirname "$path")"
  printf '%b' "$content" > "$TMP/$path"
  git -C "$TMP" add "$path"
  git -C "$TMP" commit -q -m "test: change $path"
  git -C "$TMP" rev-parse HEAD
}

git -C "$TMP" init -q
git -C "$TMP" config user.name "uptime-lab test"
git -C "$TMP" config user.email "test@example.invalid"
printf 'base\n' > "$TMP/README.md"
git -C "$TMP" add README.md
git -C "$TMP" commit -q -m "test: establish fixture"
BASE="$(git -C "$TMP" rev-parse HEAD)"

HEAD="$(commit_file "contracts/openapi/internal.yaml" 'openapi: 3.1.2\n')"
expect_value "internal contract change triggers internal-contract" "true" bash -c "cd '$TMP' && '$DETECT' '$BASE' '$HEAD'"
reset_base

HEAD="$(commit_file "contracts/openapi/public.yaml" 'openapi: 3.1.2\n')"
expect_value "public contract alone skips internal-contract" "false" bash -c "cd '$TMP' && '$DETECT' '$BASE' '$HEAD'"
reset_base

HEAD="$(commit_file "scripts/ci/detect-internal-contract-changes.sh" '# detector fixture\n')"
expect_value "detector self-change triggers internal-contract" "true" bash -c "cd '$TMP' && '$DETECT' '$BASE' '$HEAD'"
reset_base

HEAD="$(commit_file "scripts/ci/test-detect-internal-contract-changes.sh" '# detector test fixture\n')"
expect_value "detector test change triggers internal-contract" "true" bash -c "cd '$TMP' && '$DETECT' '$BASE' '$HEAD'"
reset_base

HEAD="$(commit_file "scripts/ci/check-internal-contract.mjs" '// checker fixture\n')"
expect_value "semantic checker change triggers internal-contract" "true" bash -c "cd '$TMP' && '$DETECT' '$BASE' '$HEAD'"
reset_base

HEAD="$(commit_file "scripts/ci/test-check-internal-contract.mjs" '// checker test fixture\n')"
expect_value "semantic checker test change triggers internal-contract" "true" bash -c "cd '$TMP' && '$DETECT' '$BASE' '$HEAD'"
reset_base

HEAD="$(commit_file "scripts/ci/fixtures/internal-contract/example.json" '{}\n')"
expect_value "internal fixture change triggers internal-contract" "true" bash -c "cd '$TMP' && '$DETECT' '$BASE' '$HEAD'"
reset_base

HEAD="$(commit_file ".github/workflows/ci.yml" 'name: CI\n')"
expect_value "workflow change triggers internal-contract" "true" bash -c "cd '$TMP' && '$DETECT' '$BASE' '$HEAD'"
reset_base

HEAD="$(commit_file "docs/architecture/unrelated.md" '# Unrelated\n')"
expect_value "unrelated docs skip internal-contract" "false" bash -c "cd '$TMP' && '$DETECT' '$BASE' '$HEAD'"
reset_base

HEAD="$(commit_file "apps/api/internal/example.go" 'package example\n')"
expect_value "Go source alone skips internal-contract" "false" bash -c "cd '$TMP' && '$DETECT' '$BASE' '$HEAD'"
reset_base

HEAD="$(commit_file "compose.yaml" 'services: {}\n')"
expect_value "Compose-only change skips internal-contract" "false" bash -c "cd '$TMP' && '$DETECT' '$BASE' '$HEAD'"
reset_base

mkdir -p "$TMP/contracts/openapi"
printf 'openapi: 3.1.2\n' > "$TMP/contracts/openapi/internal.yaml"
git -C "$TMP" add contracts/openapi/internal.yaml
git -C "$TMP" commit -q -m "test: add deletion fixture"
DELETE_BASE="$(git -C "$TMP" rev-parse HEAD)"
rm "$TMP/contracts/openapi/internal.yaml"
git -C "$TMP" add -u
git -C "$TMP" commit -q -m "test: delete contract fixture"
DELETE_HEAD="$(git -C "$TMP" rev-parse HEAD)"
expect_value "internal contract deletion triggers internal-contract" "true" bash -c "cd '$TMP' && '$DETECT' '$DELETE_BASE' '$DELETE_HEAD'"

ZERO_SHA="0000000000000000000000000000000000000000"
expect_value "zero base is conservative" "true" bash -c "cd '$TMP' && '$DETECT' '$ZERO_SHA' '$DELETE_HEAD'"
expect_value "unavailable base is conservative" "true" bash -c "cd '$TMP' && '$DETECT' '1111111111111111111111111111111111111111' '$DELETE_HEAD'"
expect_failure "unavailable head fails closed" bash -c "cd '$TMP' && '$DETECT' '$DELETE_BASE' '2222222222222222222222222222222222222222'"

printf '\nInternal contract change detection tests: %d passed, %d failed\n' "$PASS" "$FAIL"
test "$PASS" -eq 15
test "$FAIL" -eq 0
