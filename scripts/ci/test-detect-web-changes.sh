#!/usr/bin/env bash
set -euo pipefail
SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
TMP="$(mktemp -d)";trap 'rm -rf "$TMP"' EXIT
git -C "$TMP" init -q;git -C "$TMP" config user.name 'uptime-lab test';git -C "$TMP" config user.email test@example.invalid
printf 'base\n' > "$TMP/README.md";git -C "$TMP" add .;git -C "$TMP" commit -qm 'chore: fixture';BASE="$(git -C "$TMP" rev-parse HEAD)"
DETECT="$SCRIPT_DIR/detect-web-changes.sh"
expect_value(){ local expected="$1" base="$2" head="$3" actual;actual="$(cd "$TMP";bash "$DETECT" "$base" "$head")";[[ "$actual" == "$expected" ]] || { printf 'Detection mismatch: %s != %s\n' "$actual" "$expected";exit 1;}; }
for path in apps/web/package-lock.json apps/api/main.go apps/checker/main.rs contracts/openapi/public.yaml contracts/fixtures/public/example.json compose.yaml compose.web-local.yaml .env.example scripts/ci/check-web-compose.mjs scripts/ci/run-web-browser-tests.sh .github/workflows/ci.yml;do
  mkdir -p "$TMP/$(dirname "$path")";printf 'fixture\n' > "$TMP/$path";git -C "$TMP" add .;git -C "$TMP" commit -qm 'chore: fixture';HEAD="$(git -C "$TMP" rev-parse HEAD)";expect_value true "$BASE" "$HEAD"
  git -C "$TMP" rm -q "$path";git -C "$TMP" commit -qm 'chore: remove fixture';expect_value true "$HEAD" "$(git -C "$TMP" rev-parse HEAD)"
  git -C "$TMP" reset --hard -q "$BASE";git -C "$TMP" clean -fdq
done
mkdir -p "$TMP/apps/web";printf 'x' > "$TMP/apps/web/old.ts";git -C "$TMP" add .;git -C "$TMP" commit -qm 'chore: rename base';FROM="$(git -C "$TMP" rev-parse HEAD)";git -C "$TMP" mv apps/web/old.ts unrelated.ts;git -C "$TMP" commit -qm 'chore: rename';expect_value true "$FROM" "$(git -C "$TMP" rev-parse HEAD)"
git -C "$TMP" reset --hard -q "$BASE";mkdir -p "$TMP/docs/architecture";printf 'doc' > "$TMP/docs/architecture/unrelated.md";git -C "$TMP" add .;git -C "$TMP" commit -qm 'docs: fixture';HEAD="$(git -C "$TMP" rev-parse HEAD)";expect_value false "$BASE" "$HEAD"
expect_value true 0000000000000000000000000000000000000000 "$HEAD";expect_value true 1111111111111111111111111111111111111111 "$HEAD"
if (cd "$TMP";bash "$DETECT" "$BASE" 1111111111111111111111111111111111111111 >/dev/null 2>&1);then exit 1;fi
printf 'Web change detector: 27 passed\n'
