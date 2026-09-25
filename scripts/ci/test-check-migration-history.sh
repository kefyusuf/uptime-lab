#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
CHECK="$SCRIPT_DIR/check-migration-history.sh"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

PASS=0
FAIL=0

pass() { printf 'PASS: %s\n' "$1"; PASS=$((PASS + 1)); }
fail() { printf 'FAIL: %s\n' "$1" >&2; FAIL=$((FAIL + 1)); }

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

reset_base() {
  git -C "$TMP" reset --hard -q "$BASE"
  git -C "$TMP" clean -fdq
}

commit_path() {
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
mkdir -p "$TMP/apps/api/migrations"
printf '%s\n' '-- +goose Up' 'CREATE TABLE example (id integer);' > "$TMP/apps/api/migrations/00001_initial.sql"
printf 'package migrations\n' > "$TMP/apps/api/migrations/provider.go"
printf '# fixture\n' > "$TMP/README.md"
git -C "$TMP" add .
git -C "$TMP" commit -q -m "test: establish fixture"
BASE="$(git -C "$TMP" rev-parse HEAD)"

HEAD="$(commit_path "README.md" '# changed\n')"
expect_success "unrelated change is allowed" bash -c "cd '$TMP' && '$CHECK' '$BASE' '$HEAD'"
reset_base

HEAD="$(commit_path "apps/api/migrations/00002_added.sql" '-- +goose Up\nCREATE TABLE added (id integer);\n')"
expect_success "new SQL migration is allowed" bash -c "cd '$TMP' && '$CHECK' '$BASE' '$HEAD'"
reset_base

HEAD="$(commit_path "apps/api/migrations/provider.go" 'package migrations\n// changed helper\n')"
expect_success "migration helper Go change is allowed" bash -c "cd '$TMP' && '$CHECK' '$BASE' '$HEAD'"
reset_base

HEAD="$(commit_path "docs/migrations.md" '# migration docs\n')"
expect_success "documentation change is allowed" bash -c "cd '$TMP' && '$CHECK' '$BASE' '$HEAD'"
reset_base

HEAD="$(commit_path "apps/api/migrations/00001_initial.sql" '-- +goose Up\nCREATE TABLE changed (id integer);\n')"
expect_failure "modifying landed SQL migration is forbidden" bash -c "cd '$TMP' && '$CHECK' '$BASE' '$HEAD'"
reset_base

rm "$TMP/apps/api/migrations/00001_initial.sql"
git -C "$TMP" add -u
git -C "$TMP" commit -q -m "test: delete migration"
HEAD="$(git -C "$TMP" rev-parse HEAD)"
expect_failure "deleting landed SQL migration is forbidden" bash -c "cd '$TMP' && '$CHECK' '$BASE' '$HEAD'"
reset_base

git -C "$TMP" mv apps/api/migrations/00001_initial.sql apps/api/migrations/00001_renamed.sql
git -C "$TMP" commit -q -m "test: rename migration"
HEAD="$(git -C "$TMP" rev-parse HEAD)"
expect_failure "renaming landed SQL migration is forbidden" bash -c "cd '$TMP' && '$CHECK' '$BASE' '$HEAD'"
reset_base

rm "$TMP/apps/api/migrations/00001_initial.sql"
printf '%s\n' '-- +goose Up' 'CREATE TABLE replacement (id integer);' > "$TMP/apps/api/migrations/00002_replacement.sql"
git -C "$TMP" add -A
git -C "$TMP" commit -q -m "test: replace migration"
HEAD="$(git -C "$TMP" rev-parse HEAD)"
expect_failure "replacing landed SQL migration with another path is forbidden" bash -c "cd '$TMP' && '$CHECK' '$BASE' '$HEAD'"

ZERO_SHA="0000000000000000000000000000000000000000"
expect_success "zero base has no immutable prior set" bash -c "cd '$TMP' && '$CHECK' '$ZERO_SHA' '$HEAD'"
expect_failure "missing non-zero base fails closed" bash -c "cd '$TMP' && '$CHECK' '1111111111111111111111111111111111111111' '$HEAD'"
expect_failure "missing head fails closed" bash -c "cd '$TMP' && '$CHECK' '$BASE' '2222222222222222222222222222222222222222'"

printf '\nMigration history tests: %d passed, %d failed\n' "$PASS" "$FAIL"
test "$FAIL" -eq 0
