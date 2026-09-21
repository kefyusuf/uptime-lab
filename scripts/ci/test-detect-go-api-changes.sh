#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
DETECT="$SCRIPT_DIR/detect-go-api-changes.sh"
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

HEAD="$(commit_file "apps/api/internal/example.go" 'package example\n')"
expect_value "Go source change triggers go-api" "true" bash -c "cd '$TMP' && '$DETECT' '$BASE' '$HEAD'"
reset_base

HEAD="$(commit_file "apps/api/go.mod" 'module example.invalid/api\n')"
expect_value "go.mod change triggers go-api" "true" bash -c "cd '$TMP' && '$DETECT' '$BASE' '$HEAD'"
reset_base

HEAD="$(commit_file "apps/api/go.sum" 'example.invalid/module v1.0.0 h1:test\n')"
expect_value "go.sum change triggers go-api" "true" bash -c "cd '$TMP' && '$DETECT' '$BASE' '$HEAD'"
reset_base

HEAD="$(commit_file "apps/api/Dockerfile" 'FROM scratch\n')"
expect_value "API Dockerfile change triggers go-api" "true" bash -c "cd '$TMP' && '$DETECT' '$BASE' '$HEAD'"
reset_base

HEAD="$(commit_file "scripts/ci/check-go-architecture.sh" '#!/usr/bin/env bash\n')"
expect_value "Go architecture checker change triggers go-api" "true" bash -c "cd '$TMP' && '$DETECT' '$BASE' '$HEAD'"
reset_base

HEAD="$(commit_file "scripts/ci/run-go-postgres-tests.sh" '#!/usr/bin/env bash\n')"
expect_value "PostgreSQL test runner change triggers go-api" "true" bash -c "cd '$TMP' && '$DETECT' '$BASE' '$HEAD'"
reset_base

HEAD="$(commit_file "scripts/ci/detect-go-api-changes.sh" '# detector fixture\n')"
expect_value "detector self-change triggers go-api" "true" bash -c "cd '$TMP' && '$DETECT' '$BASE' '$HEAD'"
reset_base

HEAD="$(commit_file ".github/workflows/ci.yml" 'name: CI\n')"
expect_value "workflow change triggers go-api" "true" bash -c "cd '$TMP' && '$DETECT' '$BASE' '$HEAD'"
reset_base

HEAD="$(commit_file "docs/architecture/unrelated.md" '# Unrelated\n')"
expect_value "unrelated docs skip go-api" "false" bash -c "cd '$TMP' && '$DETECT' '$BASE' '$HEAD'"
reset_base

mkdir -p "$TMP/apps/api/internal"
printf 'package example\n' > "$TMP/apps/api/internal/deleted.go"
git -C "$TMP" add apps/api/internal/deleted.go
git -C "$TMP" commit -q -m "test: add deletion fixture"
DELETE_BASE="$(git -C "$TMP" rev-parse HEAD)"
rm "$TMP/apps/api/internal/deleted.go"
git -C "$TMP" add -u
git -C "$TMP" commit -q -m "test: delete Go source"
DELETE_HEAD="$(git -C "$TMP" rev-parse HEAD)"
expect_value "relevant deletion triggers go-api" "true" bash -c "cd '$TMP' && '$DETECT' '$DELETE_BASE' '$DELETE_HEAD'"

ZERO_SHA="0000000000000000000000000000000000000000"
expect_value "zero base is conservative" "true" bash -c "cd '$TMP' && '$DETECT' '$ZERO_SHA' '$DELETE_HEAD'"
expect_value "unavailable base is conservative" "true" bash -c "cd '$TMP' && '$DETECT' '1111111111111111111111111111111111111111' '$DELETE_HEAD'"
expect_failure "unavailable head fails closed" bash -c "cd '$TMP' && '$DETECT' '$DELETE_BASE' '2222222222222222222222222222222222222222'"

printf '\nGo API change detection tests: %d passed, %d failed\n' "$PASS" "$FAIL"
test "$FAIL" -eq 0
