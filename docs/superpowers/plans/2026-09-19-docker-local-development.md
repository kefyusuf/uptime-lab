# Docker-first Local Development Environment Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement a reproducible, infrastructure-only Docker Compose local-development substrate with deterministic placeholder lifecycle, PostgreSQL persistence, path-aware CI smoke evidence, and canonical contributor documentation.

**Architecture:** A single root `compose.yaml` runs four services on Compose-private networking: real PostgreSQL plus generic process-level placeholders for Web, API, and Checker. Repository-owned Bash checks pin the current Docker-foundation invariants; an isolated smoke script proves configuration, build, health-based startup, PostgreSQL persistence/reset, and cleanup. CI uses repository-owned Git diff detection so Docker smoke runs only for relevant changes while the stable required check remains `CI / gate`.

**Tech Stack:** Docker Compose >= 2.22.0, Docker Official Images, PostgreSQL 18.6, Alpine 3.24.2, Bash, Git, GitHub Actions, Markdown.

**Spec:** `docs/superpowers/specs/2026-09-19-docker-local-development-design.md`

## Global Constraints

- Infrastructure-only phase: no Go, Rust, React/TypeScript, OpenAPI, migration, or product implementation.
- Canonical services are exactly `web`, `api`, `checker`, and `db`.
- Hard startup direction is `db -> api -> checker`; `web` starts independently.
- Host requirements: Git + Docker Engine/Desktop + Docker Compose >= 2.22.0; no host Go/Rust/Node/Make/just requirement.
- One root `compose.yaml`; no Compose override files or profiles.
- No host ports in this phase.
- No fixed Compose project `name:`, `container_name`, globally named network, or globally named volume.
- PostgreSQL uses a project-scoped named volume mounted at `/var/lib/postgresql` for PostgreSQL 18+ semantics.
- Local defaults: `POSTGRES_DB=uptime_lab`, `POSTGRES_USER=uptime_lab`, `POSTGRES_PASSWORD=uptime_lab_local`; `.env` is optional and gitignored.
- Placeholder services use one generic image contract, are non-root, `read_only: true`, tmpfs-ready, `init: true`, signal-aware, and have no restart policy.
- Placeholder services expose no HTTP listener, fake health endpoint, or runtime-specific behavior.
- PostgreSQL defines an explicit Compose healthcheck backed by `pg_isready`; each placeholder defines an explicit Compose healthcheck that evaluates `/run/uptime-lab/ready`.
- Placeholder readiness combines Docker container-running lifecycle with the readiness marker; container-running state alone is insufficient.
- Readiness is health-based; no arbitrary startup sleeps or central wait script.
- Parallel worktrees are isolated only when they resolve to distinct Compose project names; colliding worktree basenames require distinct `COMPOSE_PROJECT_NAME` (or `-p`) values used consistently.
- PostgreSQL initialization overrides apply only to an empty data directory; changing `POSTGRES_DB`, `POSTGRES_USER`, or `POSTGRES_PASSWORD` after initialization requires an intentional destructive local reset before those initialization values can take effect.
- Compose Watch is a future preferred dev loop; no `develop.watch` rules in this phase.
- Floating `latest` tags are forbidden.
- Concrete plan-time image tags: `postgres:18.6-alpine3.24` and `alpine:3.24.2`.
- Local development and CI smoke use the same Compose graph.
- CI keeps `contents: read`, pinned checkout, and stable required check `CI / gate`.
- Documentation is English.
- After every task/decision, perform explicit self-review for scope alignment, spec/ADR invariants, dependency direction, unnecessary complexity, greenfield/brownfield safety, and verification evidence.

## Review Focus

1. Relevant file deletion must still produce `local_dev=true`; Task 1 tests deletion.
2. Zero/unavailable base SHA must conservatively produce `true`; Task 1 tests both.
3. Invalid `SERVICE_NAME` must fail before readiness; Task 3 tests it directly.
4. PostgreSQL 18+ must mount the named volume at `/var/lib/postgresql`, not `/var/lib/postgresql/data`; Task 2 rejects the old path and Task 4 proves persistence.
5. Mid-smoke failure must still execute destructive isolated cleanup; Task 4 proves it with a fake Docker executable.
6. Missing or incorrect placeholder healthchecks must fail static validation; Task 2 tests both absent healthcheck keys and healthchecks that do not evaluate the readiness marker.
7. Missing or incorrect PostgreSQL `pg_isready` healthchecks must fail static validation; Task 2 tests this explicitly.
8. Database inspection documentation must use the configured `POSTGRES_USER` / `POSTGRES_DB` values, and PostgreSQL first-initialization/reset semantics must be explicit; Task 5 owns both.
9. Parallel worktree documentation must promise isolation only for distinct resolved Compose project names; Task 5 owns this boundary.

## Stacked Review Model

```text
main
  └── docs/docker-local-development-plan
        └── devops/docker-local-development
```

- Design PR #9 is merged into `main`; the reviewed design is now the plan's direct authority.
- The plan PR targets `main` and must contain only the plan delta relative to the reviewed design.
- The implementation PR remains stacked on `docs/docker-local-development-plan` until this plan lands.
- The implementation branch exists because this plan was explicitly approved before execution; landing remains ordered and review-gated.
- Landing order is design (complete) -> plan -> implementation, with fresh CI and review evidence before each squash merge.

---

## Target File Map

```text
compose.yaml
.env.example
deploy/docker/placeholder/Dockerfile
deploy/docker/placeholder/entrypoint.sh
scripts/ci/detect-local-dev-changes.sh
scripts/ci/test-detect-local-dev-changes.sh
scripts/ci/check-local-dev.sh
scripts/ci/test-check-local-dev.sh
scripts/ci/smoke-local-dev.sh
scripts/ci/test-smoke-local-dev.sh
docs/devops/local-development.md
.github/workflows/ci.yml
README.md
docs/README.md
```

No file under `apps/**`, `contracts/**`, or `migrations/**` is created or modified.

---

### Task 1: Add repository-owned local-dev change detection with TDD

**Files:**
- Create: `scripts/ci/detect-local-dev-changes.sh`
- Create: `scripts/ci/test-detect-local-dev-changes.sh`

**Interfaces:**
- Consumes: `<base-sha> <head-sha>`.
- Produces: exactly `true` or `false` on stdout.
- CI consumes the output as `changes.outputs.local_dev`.

- [ ] **Step 1: Write the failing test harness**

Create `scripts/ci/test-detect-local-dev-changes.sh` with a temporary Git repository and the repository's PASS/FAIL helper style. Pin exactly these cases:

```text
1. compose.yaml modification -> true
2. deploy/docker file deletion -> true
3. .github/workflows/ci.yml modification -> true
4. unrelated architecture documentation -> false
5. all-zero base SHA -> true
6. unavailable base SHA -> true
```

Use these exact per-case mutations after the fixture base commit:

```bash
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
printf 'name: CI\n' > "$TMP/.github/workflows/ci.yml"
git -C "$TMP" add .github/workflows/ci.yml
git -C "$TMP" commit -q -m "ci: change workflow fixture"
HEAD="$(git -C "$TMP" rev-parse HEAD)"
expect_value "workflow change triggers local-dev" "true" \
  bash -c "cd '$TMP' && '$DETECT' '$BASE' '$HEAD'"
git -C "$TMP" reset --hard -q "$BASE"
git -C "$TMP" clean -fdq

# Case 4: unrelated architecture documentation
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
```

Core harness mechanics:

```bash
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
git -C "$TMP" init -q
git -C "$TMP" config user.name "uptime-lab test"
git -C "$TMP" config user.email "test@example.invalid"

mkdir -p "$TMP/docs/architecture" "$TMP/deploy/docker/placeholder" "$TMP/.github/workflows"
printf 'base\n' > "$TMP/README.md"
git -C "$TMP" add .
git -C "$TMP" commit -q -m "chore: establish fixture"
BASE="$(git -C "$TMP" rev-parse HEAD)"

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
```

For the deletion case, first commit `deploy/docker/placeholder/Dockerfile`, capture `DELETE_BASE`, delete it in a second commit, then assert `true` over `DELETE_BASE..DELETE_HEAD`.

Expected GREEN summary:

```text
Local-dev change detection tests: 6 passed, 0 failed
```

- [ ] **Step 2: Verify RED**

```bash
chmod +x scripts/ci/test-detect-local-dev-changes.sh
./scripts/ci/test-detect-local-dev-changes.sh
```

Expected: non-zero because the detector does not exist.

- [ ] **Step 3: Implement the detector**

```bash
#!/usr/bin/env bash
set -euo pipefail

if [[ "$#" -ne 2 ]]; then
  printf 'Usage: %s <base-sha> <head-sha>\n' "$0" >&2
  exit 2
fi

BASE_SHA="$1"
HEAD_SHA="$2"
ZERO_SHA="0000000000000000000000000000000000000000"

if [[ "$BASE_SHA" == "$ZERO_SHA" ]]; then
  printf 'true\n'
  exit 0
fi

if ! git cat-file -e "$HEAD_SHA^{commit}" 2>/dev/null; then
  printf 'Head commit is unavailable: %s\n' "$HEAD_SHA" >&2
  exit 1
fi

if ! git cat-file -e "$BASE_SHA^{commit}" 2>/dev/null; then
  printf 'true\n'
  exit 0
fi

LOCAL_DEV=false
while IFS= read -r path; do
  case "$path" in
    compose.yaml|.env.example|deploy/docker/*|\
    scripts/ci/detect-local-dev-changes.sh|scripts/ci/test-detect-local-dev-changes.sh|\
    scripts/ci/check-local-dev.sh|scripts/ci/test-check-local-dev.sh|\
    scripts/ci/smoke-local-dev.sh|scripts/ci/test-smoke-local-dev.sh|\
    docs/devops/local-development.md|.github/workflows/ci.yml)
      LOCAL_DEV=true
      break
      ;;
  esac
done < <(git diff --name-only "$BASE_SHA" "$HEAD_SHA" --)

printf '%s\n' "$LOCAL_DEV"
```

- [ ] **Step 4: Verify GREEN**

```bash
chmod +x scripts/ci/detect-local-dev-changes.sh
./scripts/ci/test-detect-local-dev-changes.sh
bash -n scripts/ci/detect-local-dev-changes.sh scripts/ci/test-detect-local-dev-changes.sh
```

Expected: `Local-dev change detection tests: 6 passed, 0 failed`.

- [ ] **Step 5: Self-review Task 1**

Check deletion handling, conservative unknown-base behavior, no third-party path filter, no runtime path guesses, and fresh test evidence.

- [ ] **Step 6: Commit**

```bash
git add scripts/ci/detect-local-dev-changes.sh scripts/ci/test-detect-local-dev-changes.sh
git commit -m "test(devops): add local-dev change detection"
```

---

### Task 2: Add static Docker-foundation fitness checks with TDD

**Files:**
- Create: `scripts/ci/check-local-dev.sh`
- Create: `scripts/ci/test-check-local-dev.sh`

**Interfaces:**
- Consumes: repository root, default `.`.
- Produces: exit `0` only while the repository obeys the current Docker-foundation phase contract.
- Later runtime foundations deliberately update this checker when placeholders, Watch rules, or host-port policy evolve.

- [ ] **Step 1: Create a canonical fixture**

The fixture contains `compose.yaml`, `.env.example`, `deploy/docker/placeholder/Dockerfile`, and `entrypoint.sh`.

Canonical Compose fixture:

```yaml
services:
  web:
    build: ./deploy/docker/placeholder
    environment:
      SERVICE_NAME: web
    init: true
    read_only: true
    tmpfs:
      - /run/uptime-lab:uid=10001,gid=10001,mode=0700
    healthcheck:
      test: ["CMD-SHELL", "test -f /run/uptime-lab/ready"]
      interval: 2s
      timeout: 1s
      retries: 10
      start_period: 2s

  db:
    image: postgres:18.6-alpine3.24
    environment:
      POSTGRES_DB: ${POSTGRES_DB:-uptime_lab}
      POSTGRES_USER: ${POSTGRES_USER:-uptime_lab}
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD:-uptime_lab_local}
    volumes:
      - postgres-data:/var/lib/postgresql
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U \"$${POSTGRES_USER}\" -d \"$${POSTGRES_DB}\""]
      interval: 2s
      timeout: 2s
      retries: 15
      start_period: 3s

  api:
    build: ./deploy/docker/placeholder
    environment:
      SERVICE_NAME: api
    init: true
    read_only: true
    tmpfs:
      - /run/uptime-lab:uid=10001,gid=10001,mode=0700
    healthcheck:
      test: ["CMD-SHELL", "test -f /run/uptime-lab/ready"]
      interval: 2s
      timeout: 1s
      retries: 10
      start_period: 2s
    depends_on:
      db:
        condition: service_healthy

  checker:
    build: ./deploy/docker/placeholder
    environment:
      SERVICE_NAME: checker
    init: true
    read_only: true
    tmpfs:
      - /run/uptime-lab:uid=10001,gid=10001,mode=0700
    healthcheck:
      test: ["CMD-SHELL", "test -f /run/uptime-lab/ready"]
      interval: 2s
      timeout: 1s
      retries: 10
      start_period: 2s
    depends_on:
      api:
        condition: service_healthy

volumes:
  postgres-data:
```

- [ ] **Step 2: Pin exactly 19 cases**

```text
1. canonical fixture passes
2. missing checker service fails
3. host ports mapping fails
4. container_name fails
5. fixed root project name fails
6. globally named volume fails
7. host network mode fails
8. PostgreSQL old /var/lib/postgresql/data mount fails
9. API missing db service_healthy fails
10. Checker missing api service_healthy fails
11. Web hard dependency fails
12. floating latest image fails
13. placeholder missing init/read_only/tmpfs fails
14. placeholder restart policy fails
15. Compose profiles fail
16. forbidden runtime scaffold path fails
17. placeholder missing explicit healthcheck fails
18. placeholder healthcheck that does not evaluate /run/uptime-lab/ready fails
19. PostgreSQL healthcheck without pg_isready fails
```

Add these portable fixture-mutation helpers to the test script:

```bash
replace_literal_once() {
  local file="$1"
  local old="$2"
  local new="$3"
  local content
  content="$(cat "$file")"
  [[ "$content" == *"$old"* ]] || {
    printf 'fixture text not found in %s: %s\n' "$file" "$old" >&2
    return 1
  }
  content="${content/"$old"/"$new"}"
  printf '%s' "$content" > "$file"
}

insert_after_line() {
  local file="$1"
  local exact="$2"
  local insertion="$3"
  local tmp="$file.tmp"
  local found=0
  : > "$tmp"
  while IFS= read -r line || [[ -n "$line" ]]; do
    printf '%s\n' "$line" >> "$tmp"
    if [[ "$found" -eq 0 && "$line" == "$exact" ]]; then
      printf '%s\n' "$insertion" >> "$tmp"
      found=1
    fi
  done < "$file"
  [[ "$found" -eq 1 ]] || {
    rm -f "$tmp"
    printf 'fixture line not found in %s: %s\n' "$file" "$exact" >&2
    return 1
  }
  mv "$tmp" "$file"
}
```

Use the following exact single-concern mutations:

```text
2  remove the checker block from "  checker:" through the line before "volumes:"
3  insert under "  web:": "    ports:" + "      - \"3000:3000\""
4  insert under "  web:": "    container_name: uptime-lab-web"
5  prepend "name: uptime-lab"
6  insert under "  postgres-data:": "    name: uptime-lab-postgres-data"
7  insert under "  web:": "    network_mode: host"
8  replace "postgres-data:/var/lib/postgresql" with "postgres-data:/var/lib/postgresql/data"
9  inside the api block replace its "condition: service_healthy" with "condition: service_started"
10 inside the checker block replace its "condition: service_healthy" with "condition: service_started"
11 insert under "  web:": "    depends_on:" + "      api:" + "        condition: service_healthy"
12 replace "image: postgres:18.6-alpine3.24" with "image: postgres:latest"
13 remove "    read_only: true" from the web block
14 insert under "  web:": "    restart: unless-stopped"
15 insert under "  web:": "    profiles: [dev]"
16 create "apps/api/go.mod"
17 inside the web block replace "    healthcheck:" with "    x-healthcheck:"
18 inside the web block replace its readiness test with "      test: [\"CMD-SHELL\", \"test -f /run/uptime-lab/not-ready\"]"
19 inside the db block replace its pg_isready test with "      test: [\"CMD-SHELL\", \"test -f /tmp/db-ready\"]"
```

Add these two service-scoped helpers so mutations cannot accidentally hit another service:

```bash
remove_service_block() {
  local file="$1"
  local service="$2"
  local tmp="$file.tmp"

  awk -v target="  $service:" '
    $0 == target {
      in_target=1
      removed=1
      next
    }
    in_target && ($0 ~ /^  [A-Za-z0-9_.-]+:[[:space:]]*$/ || $0 ~ /^[^ ]/) {
      in_target=0
    }
    !in_target { print }
    END {
      if (removed != 1) exit 42
    }
  ' "$file" > "$tmp"

  mv "$tmp" "$file"
}

mutate_service_line() {
  local file="$1"
  local service="$2"
  local old_line="$3"
  local new_line="$4"
  local tmp="$file.tmp"

  awk -v target="  $service:" -v old="$old_line" -v new="$new_line" '
    $0 == target {
      in_target=1
      print
      next
    }
    in_target && ($0 ~ /^  [A-Za-z0-9_.-]+:[[:space:]]*$/ || $0 ~ /^[^ ]/) {
      in_target=0
    }
    in_target && $0 == old {
      if (new != "") print new
      changed++
      next
    }
    { print }
    END {
      if (changed != 1) exit 42
    }
  ' "$file" > "$tmp"

  mv "$tmp" "$file"
}
```

Use these exact mutations after `make_fixture`:

```bash
# 2
remove_service_block "$TMP/repo/compose.yaml" checker

# 3
insert_after_line "$TMP/repo/compose.yaml" "  web:" $'    ports:\n      - "3000:3000"'

# 4
insert_after_line "$TMP/repo/compose.yaml" "  web:" "    container_name: uptime-lab-web"

# 5
{ printf 'name: uptime-lab\n'; cat "$TMP/repo/compose.yaml"; } > "$TMP/repo/compose.yaml.tmp"
mv "$TMP/repo/compose.yaml.tmp" "$TMP/repo/compose.yaml"

# 6
insert_after_line "$TMP/repo/compose.yaml" "  postgres-data:" "    name: uptime-lab-postgres-data"

# 7
insert_after_line "$TMP/repo/compose.yaml" "  web:" "    network_mode: host"

# 8
replace_literal_once "$TMP/repo/compose.yaml"   "postgres-data:/var/lib/postgresql"   "postgres-data:/var/lib/postgresql/data"

# 9
mutate_service_line "$TMP/repo/compose.yaml" api   "        condition: service_healthy"   "        condition: service_started"

# 10
mutate_service_line "$TMP/repo/compose.yaml" checker   "        condition: service_healthy"   "        condition: service_started"

# 11
insert_after_line "$TMP/repo/compose.yaml" "  web:"   $'    depends_on:\n      api:\n        condition: service_healthy'

# 12
replace_literal_once "$TMP/repo/compose.yaml"   "image: postgres:18.6-alpine3.24"   "image: postgres:latest"

# 13
mutate_service_line "$TMP/repo/compose.yaml" web "    read_only: true" ""

# 14
insert_after_line "$TMP/repo/compose.yaml" "  web:" "    restart: unless-stopped"

# 15
insert_after_line "$TMP/repo/compose.yaml" "  web:" "    profiles: [dev]"

# 16
mkdir -p "$TMP/repo/apps/api"
printf 'module example.invalid/forbidden\n' > "$TMP/repo/apps/api/go.mod"

# 17
mutate_service_line "$TMP/repo/compose.yaml" web \
  "    healthcheck:" \
  "    x-healthcheck:"

# 18
mutate_service_line "$TMP/repo/compose.yaml" web \
  '      test: ["CMD-SHELL", "test -f /run/uptime-lab/ready"]' \
  '      test: ["CMD-SHELL", "test -f /run/uptime-lab/not-ready"]'

# 19
mutate_service_line "$TMP/repo/compose.yaml" db \
  '      test: ["CMD-SHELL", "pg_isready -U \"${POSTGRES_USER}\" -d \"${POSTGRES_DB}\""]' \
  '      test: ["CMD-SHELL", "test -f /tmp/db-ready"]'
```

Every case calls `make_fixture` first and then exactly one mutation. Case 1 uses the untouched fixture.

The runtime-scope case creates `apps/api/go.mod`.

- [ ] **Step 3: Verify RED**

```bash
chmod +x scripts/ci/test-check-local-dev.sh
./scripts/ci/test-check-local-dev.sh
```

Expected: non-zero because the checker is absent.

- [ ] **Step 4: Implement `check-local-dev.sh`**

Use dependency-free Bash/Awk against the intentionally simple canonical file. Do not add a YAML package ecosystem.

Required structure:

```bash
#!/usr/bin/env bash
set -euo pipefail

ROOT="${1:-.}"
COMPOSE_FILE="$ROOT/compose.yaml"

fail() {
  printf 'Local-dev invariant failed: %s\n' "$1" >&2
  exit 1
}

[[ -f "$COMPOSE_FILE" ]] || fail "compose.yaml is missing"

extract_service_block() {
  local service="$1"
  awk -v service="$service" '
    $0 == "services:" { in_services=1; next }
    in_services && /^[^ ]/ { exit }
    in_services && $0 == "  " service ":" { in_target=1; next }
    in_target && /^  [A-Za-z0-9_.-]+:[[:space:]]*$/ { exit }
    in_target { print }
  ' "$COMPOSE_FILE"
}
```

Collect service keys under `services:` and require sorted set `api checker db web`.

Reject globally:

```text
root name:
container_name:
ports:
restart:
profiles:
network_mode: host
external: true
image ending :latest
globally named Compose resource
```

For Web/API/Checker require:

```text
build: ./deploy/docker/placeholder
SERVICE_NAME matching service
init: true
read_only: true
tmpfs /run/uptime-lab:uid=10001,gid=10001,mode=0700
explicit healthcheck: key
healthcheck test evaluates /run/uptime-lab/ready
```

For DB require:

```text
postgres image with explicit non-latest tag
postgres-data:/var/lib/postgresql
explicit healthcheck: key
healthcheck test uses pg_isready
```

Require API -> DB `service_healthy`, Checker -> API `service_healthy`, and no Web `depends_on`.

Reject phase-forbidden paths if present:

```text
apps
contracts
migrations
package.json
go.mod
Cargo.toml
```

Require placeholder Dockerfile + entrypoint; Dockerfile must use an explicit non-latest base tag and `USER 10001:10001`.

- [ ] **Step 5: Verify GREEN**

```bash
chmod +x scripts/ci/check-local-dev.sh scripts/ci/test-check-local-dev.sh
./scripts/ci/test-check-local-dev.sh
bash -n scripts/ci/check-local-dev.sh scripts/ci/test-check-local-dev.sh
```

Expected: `Local development tests: 19 passed, 0 failed`.

- [ ] **Step 6: Self-review Task 2**

Check phase specificity, PostgreSQL 18+ path, explicit DB/placeholder healthchecks, readiness-marker evaluation, absence of fake ports/contracts, deterministic fixture isolation, no parser/toolchain creep, and fresh evidence.

- [ ] **Step 7: Commit**

```bash
git add scripts/ci/check-local-dev.sh scripts/ci/test-check-local-dev.sh
git commit -m "test(devops): add local-dev topology checks"
```

---

### Task 3: Implement the generic placeholder image and canonical Compose topology

**Files:**
- Create: `deploy/docker/placeholder/Dockerfile`
- Create: `deploy/docker/placeholder/entrypoint.sh`
- Create: `compose.yaml`
- Create: `.env.example`

**Interfaces:**
- Consumes: Task 2 static contract.
- Produces: a four-service topology without real application source.

- [ ] **Step 1: Verify repository RED**

```bash
./scripts/ci/check-local-dev.sh .
```

Expected: FAIL because `compose.yaml` is absent.

- [ ] **Step 2: Create placeholder Dockerfile**

```dockerfile
FROM alpine:3.24.2

RUN addgroup -S -g 10001 uptime \
    && adduser -S -D -H -u 10001 -G uptime uptime \
    && mkdir -p /run/uptime-lab \
    && chown 10001:10001 /run/uptime-lab

COPY --chown=10001:10001 entrypoint.sh /usr/local/bin/placeholder-entrypoint
RUN chmod 0555 /usr/local/bin/placeholder-entrypoint

USER 10001:10001
ENTRYPOINT ["/usr/local/bin/placeholder-entrypoint"]
```

- [ ] **Step 3: Create placeholder entrypoint**

```sh
#!/bin/sh
set -eu

case "${SERVICE_NAME:-}" in
  web|api|checker) ;;
  *)
    printf 'Invalid SERVICE_NAME: %s\n' "${SERVICE_NAME:-<unset>}" >&2
    exit 64
    ;;
esac

READY_FILE="/run/uptime-lab/ready"
child_pid=""
stopping=0

log() { printf 'service=%s event=%s\n' "$SERVICE_NAME" "$1"; }

shutdown() {
  if [ "$stopping" -eq 1 ]; then return; fi
  stopping=1
  rm -f "$READY_FILE"
  log stopping
  if [ -n "$child_pid" ]; then kill "$child_pid" 2>/dev/null || true; fi
}

trap 'exit 130' INT
trap 'exit 143' TERM
trap shutdown EXIT

log starting
: > "$READY_FILE"
log ready

tail -f /dev/null &
child_pid=$!
if wait "$child_pid"; then status=0; else status=$?; fi
exit "$status"
```

- [ ] **Step 4: Test invalid identity**

```bash
chmod +x deploy/docker/placeholder/entrypoint.sh
set +e
SERVICE_NAME=invalid sh deploy/docker/placeholder/entrypoint.sh
status=$?
set -e
test "$status" -eq 64
```

- [ ] **Step 5: Create `compose.yaml`**

Use the exact valid Compose fixture from Task 2 as the repository file. PostgreSQL must use:

```yaml
image: postgres:18.6-alpine3.24
volumes:
  - postgres-data:/var/lib/postgresql
```

No ports, profiles, restart policies, fixed names, external networks, or Watch rules.

- [ ] **Step 6: Create `.env.example`**

```dotenv
# Optional local overrides.
# Canonical startup works without copying this file.

POSTGRES_DB=uptime_lab
POSTGRES_USER=uptime_lab
POSTGRES_PASSWORD=uptime_lab_local
```

Do not create/track `.env`.

- [ ] **Step 7: Verify static GREEN**

```bash
./scripts/ci/test-check-local-dev.sh
./scripts/ci/check-local-dev.sh .
bash -n deploy/docker/placeholder/entrypoint.sh
git diff --check
```

- [ ] **Step 8: Validate Compose when available**

```bash
docker compose version
docker compose config --quiet
docker compose config --services
docker compose config --volumes
```

Expected service set: `web`, `db`, `api`, `checker`; volume `postgres-data`.

If the execution host has no Docker/Compose access, record local runtime evidence as unavailable and require Task 6/7 GitHub Actions for authoritative Docker proof. Do not add another parser.

- [ ] **Step 9: Self-review Task 3**

Check no listener, runtime source, ports, fixed names, or Watch mapping; correct PG18 path; explicit tags; fresh static evidence.

- [ ] **Step 10: Commit**

```bash
git add compose.yaml .env.example deploy/docker/placeholder
git commit -m "build(devops): add Docker local topology"
```

---

### Task 4: Add isolated real Compose smoke verification

**Files:**
- Create: `scripts/ci/smoke-local-dev.sh`
- Create: `scripts/ci/test-smoke-local-dev.sh`

**Interfaces:**
- Consumes: Task 3 topology.
- Produces: isolated evidence for Compose version, config, build, health startup, persistence, reset, and cleanup.
- Test seam: `DOCKER_BIN`; default `docker`.

- [ ] **Step 1: Write failing fake-Docker harness**

The harness creates a fake `docker` executable:

```bash
cat > "$FAKE_DOCKER" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >> "$DOCKER_LOG"

joined="$*"
if [[ -n "${FAIL_ON_PATTERN:-}" && "$joined" == *"$FAIL_ON_PATTERN"* ]]; then
  exit 42
fi

if [[ "$joined" == *"version --short"* ]]; then
  printf '%s\n' "${FAKE_COMPOSE_VERSION:-2.22.0}"
  exit 0
fi

if [[ "$joined" == *"psql"* && "$joined" == *"-Atc"* ]]; then
  printf 't\n'
  exit 0
fi

exit 0
EOF
```

Required cases:

```text
1. success path logs config/build/up/ps/persistence/reset/cleanup
2. Compose 2.21.0 fails
3. FAIL_ON_PATTERN=build fails but log still contains down -v --remove-orphans
```

Use these exact assertions:

```bash
# Case 1: success path
: > "$DOCKER_LOG"
expect_success "smoke success path" env \
  DOCKER_LOG="$DOCKER_LOG" \
  DOCKER_BIN="$FAKE_DOCKER" \
  ./scripts/ci/smoke-local-dev.sh
grep -Fq 'version --short' "$DOCKER_LOG"
grep -Fq 'config --quiet' "$DOCKER_LOG"
grep -Fq 'build' "$DOCKER_LOG"
grep -Fq 'up -d --wait --wait-timeout 60' "$DOCKER_LOG"
grep -Fq 'ps' "$DOCKER_LOG"
grep -Fq 'down -v --remove-orphans' "$DOCKER_LOG"

# Case 2: unsupported Compose
: > "$DOCKER_LOG"
expect_failure "reject Compose 2.21" env \
  DOCKER_LOG="$DOCKER_LOG" \
  DOCKER_BIN="$FAKE_DOCKER" \
  FAKE_COMPOSE_VERSION=2.21.0 \
  ./scripts/ci/smoke-local-dev.sh

# Case 3: failure still cleans up
: > "$DOCKER_LOG"
expect_failure "build failure triggers cleanup" env \
  DOCKER_LOG="$DOCKER_LOG" \
  DOCKER_BIN="$FAKE_DOCKER" \
  FAIL_ON_PATTERN=build \
  ./scripts/ci/smoke-local-dev.sh
grep -Fq 'down -v --remove-orphans' "$DOCKER_LOG"
```

Each `grep` failure counts as a harness failure rather than aborting before the final summary; wrap assertions in the same PASS/FAIL helper pattern as the other repository tests.

Expected GREEN: `Local-dev smoke tests: 3 passed, 0 failed`.

- [ ] **Step 2: Verify RED**

```bash
chmod +x scripts/ci/test-smoke-local-dev.sh
./scripts/ci/test-smoke-local-dev.sh
```

Expected non-zero because smoke script is absent.

- [ ] **Step 3: Implement `smoke-local-dev.sh`**

```bash
#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)"
DOCKER_BIN="${DOCKER_BIN:-docker}"
COMPOSE_FILE="${COMPOSE_FILE:-$ROOT/compose.yaml}"
export COMPOSE_PROJECT_NAME="${COMPOSE_PROJECT_NAME:-uptime-lab-smoke-$$}"

compose() { "$DOCKER_BIN" compose -f "$COMPOSE_FILE" "$@"; }
cleanup() { compose down -v --remove-orphans >/dev/null 2>&1 || true; }

trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

VERSION="$(compose version --short)"
VERSION="${VERSION#v}"
IFS=. read -r MAJOR MINOR PATCH <<EOF
$VERSION
EOF
MAJOR="${MAJOR:-0}"
MINOR="${MINOR:-0}"
if (( MAJOR < 2 || (MAJOR == 2 && MINOR < 22) )); then
  printf 'Docker Compose >= 2.22.0 is required; found %s\n' "$VERSION" >&2
  exit 1
fi

cleanup
compose config --quiet
"$ROOT/scripts/ci/check-local-dev.sh" "$ROOT"
compose build
compose up -d --wait --wait-timeout 60
compose ps

DB_USER="${POSTGRES_USER:-uptime_lab}"
DB_NAME="${POSTGRES_DB:-uptime_lab}"
PROBE_TABLE="public.__uptime_lab_local_dev_probe"

compose exec -T db psql -v ON_ERROR_STOP=1 -U "$DB_USER" -d "$DB_NAME" \
  -c "CREATE TABLE $PROBE_TABLE (id integer PRIMARY KEY); INSERT INTO $PROBE_TABLE (id) VALUES (1);"

compose down
compose up -d --wait --wait-timeout 60
PERSISTED="$(compose exec -T db psql -U "$DB_USER" -d "$DB_NAME" -Atc \
  "SELECT to_regclass('$PROBE_TABLE') IS NOT NULL;")"
test "$PERSISTED" = "t"

compose down -v --remove-orphans
compose up -d --wait --wait-timeout 60
RESET="$(compose exec -T db psql -U "$DB_USER" -d "$DB_NAME" -Atc \
  "SELECT to_regclass('$PROBE_TABLE') IS NULL;")"
test "$RESET" = "t"

cleanup
trap - EXIT INT TERM
```

- [ ] **Step 4: Verify fake-Docker GREEN**

```bash
chmod +x scripts/ci/smoke-local-dev.sh scripts/ci/test-smoke-local-dev.sh
./scripts/ci/test-smoke-local-dev.sh
bash -n scripts/ci/smoke-local-dev.sh scripts/ci/test-smoke-local-dev.sh
```

Expected: `Local-dev smoke tests: 3 passed, 0 failed`.

- [ ] **Step 5: Run real smoke if Docker is available**

```bash
./scripts/ci/smoke-local-dev.sh
```

Expected exit `0` and no remaining smoke project.

If Docker is unavailable locally, PR CI is mandatory real-runtime evidence.

- [ ] **Step 6: Self-review Task 4**

Check project isolation, EXIT cleanup, no sleeps, persistence/reset semantics, version floor, no host-port assumption, fresh tests.

- [ ] **Step 7: Commit**

```bash
git add scripts/ci/smoke-local-dev.sh scripts/ci/test-smoke-local-dev.sh
git commit -m "test(devops): add local-dev smoke verification"
```

---

### Task 5: Document canonical local development

**Files:**
- Create: `docs/devops/local-development.md`
- Modify: `README.md`
- Modify: `docs/README.md`

**Interfaces:** Produces one canonical operational truth; root/index docs only link.

- [ ] **Step 1: Create `docs/devops/local-development.md`**

Required headings:

```text
Status
Prerequisites
Canonical Topology
Start the Stack
Inspect Service State
Follow Logs
Health and Readiness
PostgreSQL Access
Local Persistence
Destructive Reset
Optional Environment Overrides
Worktree and Project Isolation
Compose Watch
CI Verification
Troubleshooting
Current Limitations
Related Architecture
```

Required commands:

```bash
docker compose version
docker compose up -d --build --wait
docker compose ps
docker compose logs -f
docker compose exec db sh -lc 'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB"'
docker compose down
docker compose down -v
```

Required limitations:

```text
Web is a placeholder, not React.
API is a placeholder, not Go.
Checker is a placeholder, not Rust.
No product HTTP API exists.
No host application port is exposed.
No monitoring migrations/tables exist.
No probe execution occurs.
No real develop.watch mappings exist.
```

Document Compose >=2.22.0, optional `.env`, development-only password, and destructive reset.

Also document these reviewed-design requirements explicitly:

- database inspection uses the configured `POSTGRES_USER` and `POSTGRES_DB` values resolved inside the `db` service;
- PostgreSQL initialization variables apply only when the data directory is empty, so changing `POSTGRES_DB`, `POSTGRES_USER`, or `POSTGRES_PASSWORD` after initialization requires an intentional `docker compose down -v` reset before those initialization values can take effect;
- parallel worktrees are supported only when each invocation resolves to a distinct Compose project name; colliding worktree basenames require a distinct `COMPOSE_PROJECT_NAME` (or `-p`) used consistently across startup, inspection, logs, shutdown, and reset.

- [ ] **Step 2: Update root README**

Replace the future-tense Docker bullet with:

```markdown
- **Docker Compose** is the canonical local-development substrate. The current Docker foundation runs PostgreSQL plus process-level placeholders; Go, Rust, and React runtimes remain unimplemented.
```

Add a short `## Local Development` section linking only to `docs/devops/local-development.md`.

- [ ] **Step 3: Update docs index**

Add a `## Local Development` section linking to `devops/local-development.md`.

- [ ] **Step 4: Verify docs**

```bash
grep -Fq 'Docker Compose >= 2.22.0' docs/devops/local-development.md
grep -Fq 'docker compose up -d --build --wait' docs/devops/local-development.md
grep -Fq 'docker compose down -v' docs/devops/local-development.md
grep -Fq 'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB"' docs/devops/local-development.md
grep -Fq 'only when the database data directory is empty' docs/devops/local-development.md
grep -Fq 'distinct Compose project name' docs/devops/local-development.md
grep -Fq 'Web is a placeholder' docs/devops/local-development.md
grep -Fq 'devops/local-development.md' README.md
grep -Fq 'devops/local-development.md' docs/README.md
./scripts/ci/check-architecture-docs.sh .
git diff --check
```

- [ ] **Step 5: Self-review Task 5**

Check DRY ownership, configured database inspection, PostgreSQL first-initialization/reset clarity, distinct-project-name worktree isolation, no production claims, explicit placeholder limitations, and fresh evidence.

- [ ] **Step 6: Commit**

```bash
git add docs/devops/local-development.md README.md docs/README.md
git commit -m "docs(devops): document local development workflow"
```

---

### Task 6: Integrate path-aware Docker smoke into CI

**Files:**
- Modify: `.github/workflows/ci.yml`

**Interfaces:**
- Consumes Task 1 detector, Task 2 checker, Task 4 smoke.
- Produces `changes.outputs.local_dev`, conditional `local-dev`, and extended stable `CI / gate`.

- [ ] **Step 1: Add `changes` job**

```yaml
  changes:
    name: changes
    runs-on: ubuntu-24.04
    timeout-minutes: 5
    outputs:
      local_dev: ${{ steps.detect.outputs.local_dev }}
    steps:
      - name: Check out repository
        uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
        with:
          fetch-depth: 0
          persist-credentials: false

      - name: Test local-dev change detection
        run: ./scripts/ci/test-detect-local-dev-changes.sh

      - name: Detect local-dev changes
        id: detect
        env:
          BASE_SHA: ${{ github.event.pull_request.base.sha || github.event.before }}
          HEAD_SHA: ${{ github.event.pull_request.head.sha || github.sha }}
        run: |
          local_dev="$(./scripts/ci/detect-local-dev-changes.sh "$BASE_SHA" "$HEAD_SHA")"
          printf 'local_dev=%s\n' "$local_dev" >> "$GITHUB_OUTPUT"
```

- [ ] **Step 2: Add conditional `local-dev` job**

```yaml
  local-dev:
    name: local-dev
    needs:
      - changes
    if: ${{ needs.changes.outputs.local_dev == 'true' }}
    runs-on: ubuntu-24.04
    timeout-minutes: 10
    env:
      COMPOSE_PROJECT_NAME: uptime-lab-ci-${{ github.run_id }}
    steps:
      - name: Check out repository
        uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
        with:
          fetch-depth: 0
          persist-credentials: false

      - name: Test local-dev topology checks
        run: ./scripts/ci/test-check-local-dev.sh

      - name: Test local-dev smoke harness
        run: ./scripts/ci/test-smoke-local-dev.sh

      - name: Run Docker local-dev smoke
        run: ./scripts/ci/smoke-local-dev.sh

      - name: Cleanup Docker local-dev smoke
        if: ${{ always() }}
        run: docker compose down -v --remove-orphans || true
```

The job-level project name makes workflow cleanup and smoke cleanup target the same isolated namespace.

- [ ] **Step 3: Extend `CI / gate`**

Preserve the existing `if: ${{ always() }}` on the aggregate gate. This is required so the gate still executes when `local-dev` is legitimately skipped.

Add `changes` and `local-dev` to `needs`.

Require:

```bash
test "$POLICY_RESULT" = "success"
test "$REPOSITORY_RESULT" = "success"
test "$CHANGES_RESULT" = "success"
case "$LOCAL_DEV_RESULT" in
  success|skipped) ;;
  *) exit 1 ;;
esac
```

Keep job name exactly `CI / gate`.

- [ ] **Step 4: Verify workflow security**

```bash
grep -Fq 'contents: read' .github/workflows/ci.yml
grep -Fq 'persist-credentials: false' .github/workflows/ci.yml
! grep -Fq 'pull_request_target' .github/workflows/ci.yml
grep -Fq 'name: CI / gate' .github/workflows/ci.yml
grep -Fq 'name: local-dev' .github/workflows/ci.yml
grep -Fq 'name: changes' .github/workflows/ci.yml
git diff --check
```

- [ ] **Step 5: Run locally available checks**

```bash
./scripts/ci/test-governance.sh
./scripts/github/test-configure-governance.sh
./scripts/ci/test-architecture-docs.sh
./scripts/ci/check-architecture-docs.sh .
./scripts/ci/test-detect-local-dev-changes.sh
./scripts/ci/test-check-local-dev.sh
./scripts/ci/test-smoke-local-dev.sh
./scripts/ci/check-repository-shape.sh .
git diff --check
```

If Docker is available, also run `./scripts/ci/smoke-local-dev.sh`.

- [ ] **Step 6: Self-review Task 6**

Check unrelated-change skip, detector failure semantics, local-dev failure propagation, unchanged gate name, read-only permissions, no third-party path filter, and fresh checks.

- [ ] **Step 7: Commit**

```bash
git add .github/workflows/ci.yml
git commit -m "ci(devops): add path-aware local-dev smoke"
```

---

### Task 7: Final verification, stacked PR, and landing gate

**Files:** No new implementation files.

**Interfaces:** Produces a reviewable implementation PR and ordered landing path.

- [ ] **Step 1: Create implementation branch only after plan approval**

```bash
git switch docs/docker-local-development-plan
git switch -c devops/docker-local-development
```

- [ ] **Step 2: Run final static verification**

```bash
./scripts/ci/test-governance.sh
./scripts/github/test-configure-governance.sh
./scripts/ci/test-architecture-docs.sh
./scripts/ci/check-architecture-docs.sh .
./scripts/ci/test-detect-local-dev-changes.sh
./scripts/ci/test-check-local-dev.sh
./scripts/ci/check-local-dev.sh .
./scripts/ci/test-smoke-local-dev.sh
./scripts/ci/check-repository-shape.sh .
./scripts/ci/check-pr-title.sh "build(devops): establish Docker local development environment"
bash -n scripts/ci/detect-local-dev-changes.sh \
  scripts/ci/test-detect-local-dev-changes.sh \
  scripts/ci/check-local-dev.sh \
  scripts/ci/test-check-local-dev.sh \
  scripts/ci/smoke-local-dev.sh \
  scripts/ci/test-smoke-local-dev.sh \
  deploy/docker/placeholder/entrypoint.sh
git diff --check
```

- [ ] **Step 3: Prove scope**

```bash
git diff --name-only docs/docker-local-development-plan...HEAD
```

Allowed only:

```text
compose.yaml
.env.example
deploy/docker/placeholder/**
scripts/ci/**
docs/devops/local-development.md
.github/workflows/ci.yml
README.md
docs/README.md
```

Forbidden:

```text
apps/
contracts/
migrations/
package.json
go.mod
Cargo.toml
```

- [ ] **Step 4: Run real Docker smoke if available**

```bash
./scripts/ci/smoke-local-dev.sh
```

If Docker is unavailable locally, do not claim local runtime verification; PR CI must provide it.

- [ ] **Step 5: Open stacked implementation PR**

Base: `docs/docker-local-development-plan`

Title:

```text
build(devops): establish Docker local development environment
```

PR body states:
- approved Docker-first substrate only;
- placeholders remain non-functional runtimes;
- no API contract or business schema;
- local PostgreSQL persistence only;
- private networking/no host ports/no real secrets;
- detector/topology/smoke/real Compose/CI-gate evidence.

- [ ] **Step 6: Require fresh PR CI**

Implementation head must show:

```text
policy       success
repository   success
changes      success
local-dev    success
CI / gate    success
```

`local-dev` must be success, not skipped. Inspect topology tests, fake-smoke tests, real smoke, and cleanup.

- [ ] **Step 7: Whole-branch self-review**

Verify DL-001..DL-018, PostgreSQL 18+ path, no ports/fixed names/fake endpoints, placeholder lifecycle/security, persistence/reset, path-aware CI, docs limitations, and zero runtime leakage.

- [ ] **Step 8: Land stack only after review approval**

```text
A. Mark design PR #9 ready; require fresh CI; squash-merge to main.
B. Retarget plan PR to main; require fresh CI; squash-merge.
C. Retarget implementation PR to main; require fresh CI including local-dev; squash-merge.
D. Verify final main push has CI / gate = success.
```

Implementation squash subject:

```text
build(devops): establish Docker local development environment
```

- [ ] **Step 9: Stop at next gate**

Do not start Go code. Next separate gate: `Go Monitoring Foundation — scope/design`.

---

## Implementer Self-Review

- [ ] Design, plan, implementation remain independently reviewable.
- [ ] Only approved infrastructure/docs/CI paths change.
- [ ] No `apps/**`, `contracts/**`, migrations, `package.json`, `go.mod`, or `Cargo.toml`.
- [ ] Services exactly Web/API/Checker/DB.
- [ ] API waits for DB; Checker waits for API; Web independent.
- [ ] PostgreSQL explicit tag and `/var/lib/postgresql` mount.
- [ ] PostgreSQL and placeholder Compose healthchecks are explicit and statically regression-tested.
- [ ] Placeholder healthchecks evaluate `/run/uptime-lab/ready`; container-running state alone is not treated as readiness.
- [ ] Normal `down` persistence and `down -v` reset proven.
- [ ] PostgreSQL first-initialization semantics and configured database inspection are documented.
- [ ] Parallel worktree isolation is documented as requiring distinct resolved Compose project names.
- [ ] No ports, fixed names, profiles, or restart policies.
- [ ] Placeholders non-root/read-only/tmpfs/init and listener-free.
- [ ] Invalid `SERVICE_NAME` fails before readiness.
- [ ] Compose >=2.22.0 enforced and documented.
- [ ] `.env` optional/untracked; no real secret.
- [ ] No fake Watch mapping.
- [ ] Change detection handles deletion and unknown base conservatively.
- [ ] Smoke cleanup works after success and failure.
- [ ] `changes` + `local-dev` integrate without renaming `CI / gate`.
- [ ] Workflow permission remains `contents: read`.
- [ ] Canonical docs state current limitations.
- [ ] Every task received explicit self-review.

## Exit Criteria

This plan is complete only when:

1. Docker-local substrate exists with no runtime implementation leakage.
2. Change-detection tests report `6 passed, 0 failed`.
3. Topology tests report `19 passed, 0 failed`.
4. Smoke-harness tests report `3 passed, 0 failed`.
5. Real Compose smoke proves config, build, health, persistence, reset, and cleanup.
6. Implementation PR runs path-aware `local-dev`.
7. `policy`, `repository`, `changes`, `local-dev`, and `CI / gate` are green.
8. Design -> plan -> implementation land in order under active governance.
9. Final `main` push has `CI / gate = success`.
10. No Go/Rust/React/OpenAPI/product implementation begins.
