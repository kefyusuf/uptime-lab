#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
DETECT="$SCRIPT_DIR/detect-local-dev-changes.sh"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

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

expect_value() {
  local name="$1"
  local expected="$2"
  shift 2
  local actual

  if actual="$("$@" 2>/dev/null)" && [[ "$actual" == "$expected" ]]; then
    pass "$name"
  else
    fail "$name"
  fi
}

git -C "$TMP" init -q
git -C "$TMP" config user.name "uptime-lab test"
git -C "$TMP" config user.email "test@example.invalid"

mkdir -p "$TMP/docs/architecture" "$TMP/deploy/docker/placeholder" "$TMP/.github/workflows"
printf 'base\n' > "$TMP/README.md"
git -C "$TMP" add .
git -C "$TMP" commit -q -m "chore: establish fixture"
BASE="$(git -C "$TMP" rev-parse HEAD)"

# Case 1: compose.yaml modification
printf 'services: {}\n' > "$TMP/compose.yaml"
git -C "$TMP" add compose.yaml
git -C "$TMP" commit -q -m "build(devops): add compose fixture"
HEAD="$(git -C "$TMP" rev-parse HEAD)"
expect_value "compose modification triggers local-dev" "true" \
  bash -c "cd '$TMP' && '$DETECT' '$BASE' '$HEAD'"
git -C "$TMP" reset --hard -q "$BASE"
git -C "$TMP" clean -fdq

# Case 2: relevant deletion
mkdir -p "$TMP/deploy/docker/placeholder"
printf 'FROM scratch\n' > "$TMP/deploy/docker/placeholder/Dockerfile"
git -C "$TMP" add deploy/docker/placeholder/Dockerfile
git -C "$TMP" commit -q -m "build(devops): add placeholder fixture"
DELETE_BASE="$(git -C "$TMP" rev-parse HEAD)"
rm "$TMP/deploy/docker/placeholder/Dockerfile"
git -C "$TMP" add -u
git -C "$TMP" commit -q -m "build(devops): remove placeholder fixture"
DELETE_HEAD="$(git -C "$TMP" rev-parse HEAD)"
expect_value "relevant deletion triggers local-dev" "true" \
  bash -c "cd '$TMP' && '$DETECT' '$DELETE_BASE' '$DELETE_HEAD'"
git -C "$TMP" reset --hard -q "$BASE"
git -C "$TMP" clean -fdq

# Case 3: workflow modification
mkdir -p "$TMP/.github/workflows"
printf 'name: CI\n' > "$TMP/.github/workflows/ci.yml"
git -C "$TMP" add .github/workflows/ci.yml
git -C "$TMP" commit -q -m "ci: change workflow fixture"
HEAD="$(git -C "$TMP" rev-parse HEAD)"
expect_value "workflow change triggers local-dev" "true" \
  bash -c "cd '$TMP' && '$DETECT' '$BASE' '$HEAD'"
git -C "$TMP" reset --hard -q "$BASE"
git -C "$TMP" clean -fdq

# Case 4: unrelated architecture documentation
mkdir -p "$TMP/docs/architecture"
printf '# Unrelated\n' > "$TMP/docs/architecture/unrelated.md"
git -C "$TMP" add docs/architecture/unrelated.md
git -C "$TMP" commit -q -m "docs: add unrelated fixture"
HEAD="$(git -C "$TMP" rev-parse HEAD)"
expect_value "unrelated docs skip local-dev" "false" \
  bash -c "cd '$TMP' && '$DETECT' '$BASE' '$HEAD'"

# Cases 5-6 reuse the unrelated HEAD because only base-resolution behavior changes.
ZERO_SHA="0000000000000000000000000000000000000000"
expect_value "zero base is conservative" "true" \
  bash -c "cd '$TMP' && '$DETECT' '$ZERO_SHA' '$HEAD'"
expect_value "unavailable base is conservative" "true" \
  bash -c "cd '$TMP' && '$DETECT' '1111111111111111111111111111111111111111' '$HEAD'"

printf '\nLocal-dev change detection tests: %d passed, %d failed\n' "$PASS" "$FAIL"
test "$FAIL" -eq 0
