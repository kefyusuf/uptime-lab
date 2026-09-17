#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
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

expect_success() {
  local name="$1"
  shift
  if "$@" >/dev/null 2>&1; then pass "$name"; else fail "$name"; fi
}

expect_failure() {
  local name="$1"
  shift
  if "$@" >/dev/null 2>&1; then fail "$name"; else pass "$name"; fi
}

expect_success "valid scoped title" "$SCRIPT_DIR/check-pr-title.sh" "feat(api): add monitor registration"
expect_success "valid unscoped title" "$SCRIPT_DIR/check-pr-title.sh" "docs: update readme"
expect_success "valid breaking title" "$SCRIPT_DIR/check-pr-title.sh" "fix(checker)!: change timeout contract"
expect_failure "reject unknown type" "$SCRIPT_DIR/check-pr-title.sh" "feature(api): add monitor registration"
expect_failure "reject unknown scope" "$SCRIPT_DIR/check-pr-title.sh" "feat(database): add monitor registration"
expect_failure "reject missing colon" "$SCRIPT_DIR/check-pr-title.sh" "feat(api) add monitor registration"
expect_failure "reject uppercase description start" "$SCRIPT_DIR/check-pr-title.sh" "feat(api): Add monitor registration"

TMP_ROOT="$(mktemp -d)"
TMP_GIT="$(mktemp -d)"
trap 'rm -rf "$TMP_ROOT" "$TMP_GIT"' EXIT

git -C "$TMP_ROOT" init -q
git -C "$TMP_ROOT" config user.name "uptime-lab test"
git -C "$TMP_ROOT" config user.email "test@example.invalid"
mkdir -p "$TMP_ROOT/docs/superpowers/specs"
touch "$TMP_ROOT/docs/superpowers/specs/2026-09-17-uptime-lab-foundation-design.md"
git -C "$TMP_ROOT" add docs/superpowers/specs/2026-09-17-uptime-lab-foundation-design.md
git -C "$TMP_ROOT" commit -q -m "docs(architecture): add foundation spec"
expect_success "valid repository shape" "$SCRIPT_DIR/check-repository-shape.sh" "$TMP_ROOT"
mkdir "$TMP_ROOT/shared"
expect_failure "reject forbidden root shared directory" "$SCRIPT_DIR/check-repository-shape.sh" "$TMP_ROOT"
rmdir "$TMP_ROOT/shared"
touch "$TMP_ROOT/.env"
git -C "$TMP_ROOT" add -f .env
expect_failure "reject tracked root .env" "$SCRIPT_DIR/check-repository-shape.sh" "$TMP_ROOT"
git -C "$TMP_ROOT" reset -q HEAD .env
expect_success "allow untracked local .env" "$SCRIPT_DIR/check-repository-shape.sh" "$TMP_ROOT"

git -C "$TMP_GIT" init -q
git -C "$TMP_GIT" config user.name "uptime-lab test"
git -C "$TMP_GIT" config user.email "test@example.invalid"
printf 'base\n' > "$TMP_GIT/file.txt"
git -C "$TMP_GIT" add file.txt
git -C "$TMP_GIT" commit -q -m "chore: establish base"
BASE_SHA="$(git -C "$TMP_GIT" rev-parse HEAD)"
printf 'valid\n' >> "$TMP_GIT/file.txt"
git -C "$TMP_GIT" add file.txt
git -C "$TMP_GIT" commit -q -m "feat(api): add valid change"
VALID_HEAD="$(git -C "$TMP_GIT" rev-parse HEAD)"
expect_success "accept valid commit range" bash -c "cd '$TMP_GIT' && '$SCRIPT_DIR/check-commit-range.sh' '$BASE_SHA' '$VALID_HEAD'"
printf 'invalid\n' >> "$TMP_GIT/file.txt"
git -C "$TMP_GIT" add file.txt
git -C "$TMP_GIT" commit -q -m "Added invalid change"
INVALID_HEAD="$(git -C "$TMP_GIT" rev-parse HEAD)"
expect_failure "reject invalid commit range" bash -c "cd '$TMP_GIT' && '$SCRIPT_DIR/check-commit-range.sh' '$VALID_HEAD' '$INVALID_HEAD'"

printf '\nGovernance tests: %d passed, %d failed\n' "$PASS" "$FAIL"
test "$FAIL" -eq 0
