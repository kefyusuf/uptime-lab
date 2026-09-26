#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
DETECT="$SCRIPT_DIR/detect-checker-changes.sh"
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

HEAD="$(commit_file "apps/checker/crates/checker/src/main.rs" 'fn main() {}\n')"
expect_value "Checker source change triggers checker" "true" bash -c "cd '$TMP' && '$DETECT' '$BASE' '$HEAD'"
reset_base

HEAD="$(commit_file "apps/checker/Cargo.lock" 'version = 4\n')"
expect_value "Checker lockfile change triggers checker" "true" bash -c "cd '$TMP' && '$DETECT' '$BASE' '$HEAD'"
reset_base

HEAD="$(commit_file "scripts/ci/check-rust-architecture.py" '# checker fixture\n')"
expect_value "Rust architecture checker change triggers checker" "true" bash -c "cd '$TMP' && '$DETECT' '$BASE' '$HEAD'"
reset_base

HEAD="$(commit_file "scripts/ci/test-check-rust-architecture.sh" '# architecture test fixture\n')"
expect_value "Rust architecture test change triggers checker" "true" bash -c "cd '$TMP' && '$DETECT' '$BASE' '$HEAD'"
reset_base

HEAD="$(commit_file "scripts/ci/detect-checker-changes.sh" '# detector fixture\n')"
expect_value "detector self-change triggers checker" "true" bash -c "cd '$TMP' && '$DETECT' '$BASE' '$HEAD'"
reset_base

HEAD="$(commit_file "scripts/ci/test-detect-checker-changes.sh" '# detector test fixture\n')"
expect_value "detector test change triggers checker" "true" bash -c "cd '$TMP' && '$DETECT' '$BASE' '$HEAD'"
reset_base

HEAD="$(commit_file "scripts/ci/fixtures/internal-contract/check-work.json" '{}\n')"
expect_value "cross-runtime internal fixture triggers checker" "true" bash -c "cd '$TMP' && '$DETECT' '$BASE' '$HEAD'"
reset_base

HEAD="$(commit_file ".github/workflows/ci.yml" 'name: CI\n')"
expect_value "workflow change triggers checker" "true" bash -c "cd '$TMP' && '$DETECT' '$BASE' '$HEAD'"
reset_base

HEAD="$(commit_file "apps/api/internal/example.go" 'package example\n')"
expect_value "Go API source alone skips checker" "false" bash -c "cd '$TMP' && '$DETECT' '$BASE' '$HEAD'"
reset_base

HEAD="$(commit_file "docs/architecture/unrelated.md" '# Unrelated\n')"
expect_value "unrelated docs skip checker" "false" bash -c "cd '$TMP' && '$DETECT' '$BASE' '$HEAD'"
reset_base

mkdir -p "$TMP/apps/checker"
printf '[workspace]\n' > "$TMP/apps/checker/Cargo.toml"
git -C "$TMP" add apps/checker/Cargo.toml
git -C "$TMP" commit -q -m "test: add checker deletion fixture"
DELETE_BASE="$(git -C "$TMP" rev-parse HEAD)"
rm "$TMP/apps/checker/Cargo.toml"
git -C "$TMP" add -u
git -C "$TMP" commit -q -m "test: delete checker fixture"
DELETE_HEAD="$(git -C "$TMP" rev-parse HEAD)"
expect_value "Checker deletion triggers checker" "true" bash -c "cd '$TMP' && '$DETECT' '$DELETE_BASE' '$DELETE_HEAD'"

ZERO_SHA="0000000000000000000000000000000000000000"
expect_value "zero base is conservative" "true" bash -c "cd '$TMP' && '$DETECT' '$ZERO_SHA' '$DELETE_HEAD'"
expect_value "unavailable base is conservative" "true" bash -c "cd '$TMP' && '$DETECT' '1111111111111111111111111111111111111111' '$DELETE_HEAD'"
expect_failure "unavailable head fails closed" bash -c "cd '$TMP' && '$DETECT' '$DELETE_BASE' '2222222222222222222222222222222222222222'"

printf '\nChecker change detection tests: %d passed, %d failed\n' "$PASS" "$FAIL"
test "$PASS" -eq 13
test "$FAIL" -eq 0
