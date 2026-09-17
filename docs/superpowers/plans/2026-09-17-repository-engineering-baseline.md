# Repository Engineering Baseline Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Establish the first production-disciplined repository governance baseline for `uptime-lab`: contributor documentation, Git/PR conventions, mechanical policy checks, a stable GitHub Actions gate, dependency-update scaffolding, and protected `main` semantics without introducing application runtime code.

**Architecture:** This plan implements only **PR 1 — Repository engineering baseline** from the approved foundation design. Policy is expressed both as English contributor documentation and as dependency-free Bash checks invoked by a minimal GitHub Actions workflow. Go, Rust, React, PostgreSQL, Docker Compose, OpenAPI runtime contracts, and product features are intentionally excluded and receive separate plans.

**Tech Stack:** Git, GitHub pull requests, GitHub Actions, Bash, Markdown, YAML, Dependabot.

**Spec:** `docs/superpowers/specs/2026-09-17-uptime-lab-foundation-design.md`

## Global Constraints

- The repository is a monorepo.
- `main` is the only long-lived branch; there is no permanent `develop` branch.
- After the empty-repository bootstrap commit, all normal changes use short-lived branches and pull requests.
- Conventional Commits are required. Canonical form: `<type>(<scope>): <description>`; scope is optional and breaking changes may use `!` before `:`.
- Canonical types: `feat`, `fix`, `refactor`, `test`, `docs`, `ci`, `build`, `chore`, `perf`, `revert`.
- Canonical scopes: `web`, `api`, `checker`, `contracts`, `devops`, `architecture`, `docs`.
- Squash merge is canonical; merge commits and rebase merge are disabled for `main`.
- Repository-facing documentation, templates, ADRs, and contributor-facing text are English.
- GitHub Actions permissions default to read-only; this baseline introduces no secrets and no write permission.
- The only action used by the baseline CI is `actions/checkout`, pinned to immutable commit `3d3c42e5aac5ba805825da76410c181273ba90b1` (`v7.0.1`, resolved on 2026-09-17).
- The stable branch-protection check is `CI / gate`.
- No Go, Rust, React, PostgreSQL, Docker Compose, contract, or product implementation is introduced by this plan.
- Root business directories named `shared`, `common`, `utils`, or `helpers` are forbidden.
- A local untracked `.env` is allowed. A tracked root `.env` is forbidden.
- No open-source license is selected in this plan. The foundation spec does not lock a license; that legal decision remains explicit and separate.

---

## Plan Boundary

The foundation design spans multiple independently reviewable subsystems. Implementation remains split as follows:

1. **This plan:** repository engineering baseline.
2. Architecture documentation baseline.
3. Docker-first local development environment.
4. Go control-plane / monitoring-module foundation.
5. Rust checker foundation.
6. React/TypeScript frontend foundation.
7. First end-to-end monitoring vertical slice.

Completing this plan does not authorize step 2 automatically.

## Target Files

```text
uptime-lab/
├── .github/
│   ├── CODEOWNERS
│   ├── PULL_REQUEST_TEMPLATE.md
│   ├── ISSUE_TEMPLATE/
│   │   ├── bug_report.yml
│   │   ├── feature_request.yml
│   │   └── config.yml
│   ├── dependabot.yml
│   └── workflows/
│       └── ci.yml
├── docs/
│   ├── README.md
│   ├── devops/
│   │   └── repository-governance.md
│   └── superpowers/
│       ├── specs/2026-09-17-uptime-lab-foundation-design.md
│       └── plans/2026-09-17-repository-engineering-baseline.md
├── scripts/
│   └── ci/
│       ├── lib/conventional.sh
│       ├── check-pr-title.sh
│       ├── check-commit-range.sh
│       ├── check-repository-shape.sh
│       └── test-governance.sh
├── .editorconfig
├── .gitattributes
├── .gitignore
├── CONTRIBUTING.md
├── README.md
└── SECURITY.md
```

Do not create empty `apps/`, `contracts/`, or `deploy/` directories in this PR.

---

### Task 1: Establish deterministic root metadata and repository navigation

**Files:**
- Create: `.editorconfig`
- Create: `.gitattributes`
- Create: `.gitignore`
- Create: `README.md`
- Create: `docs/README.md`

**Interfaces:**
- Consumes: approved foundation spec.
- Produces: repository entry point, documentation entry point, text normalization policy, and safe cross-language ignore defaults.

- [ ] **Step 1: Create `.editorconfig`**

```ini
root = true

[*]
charset = utf-8
end_of_line = lf
insert_final_newline = true
trim_trailing_whitespace = true

[*.md]
trim_trailing_whitespace = false

[Makefile]
indent_style = tab
```

- [ ] **Step 2: Create `.gitattributes`**

```gitattributes
* text=auto eol=lf
*.bat text eol=crlf
*.cmd text eol=crlf
```

- [ ] **Step 3: Create `.gitignore`**

```gitignore
# Secrets and local environment
.env
.env.local
.env.*.local

# Operating systems
.DS_Store
Thumbs.db
Desktop.ini

# Editors and IDEs
.idea/
.vscode/*
!.vscode/extensions.json
!.vscode/settings.json

# Temporary files
*.swp
*.swo
*~
.tmp/
tmp/

# Local tooling
.coverage/
.cache/
```

Do not add ecosystem-specific outputs such as `node_modules/` or `target/` before the plan that introduces that ecosystem.

- [ ] **Step 4: Create root `README.md`**

Use these sections, in order:

```markdown
# uptime-lab
## Status
## Architecture
## Engineering Principles
## Repository Workflow
## Documentation
## Security
## License
```

Required content:

- describe the project as a production-disciplined uptime monitoring laboratory using React/TypeScript, Go, and Rust;
- state clearly that only the foundation is present and no production-ready monitoring application exists yet;
- summarize responsibility boundaries: React presentation, Go control plane/persistence ownership, Rust bounded probe execution, PostgreSQL owned by Go, Docker Compose as the future canonical local topology;
- link `docs/superpowers/specs/2026-09-17-uptime-lab-foundation-design.md` as canonical architecture;
- state `main` + short-lived PR branches + Conventional Commits + squash merge;
- link `CONTRIBUTING.md`, `docs/README.md`, and `SECURITY.md`;
- state that no open-source license has been selected and normal copyright rules apply until one is added.

Do not add CI/release badges before those surfaces exist on `main`.

- [ ] **Step 5: Create `docs/README.md`**

Use these sections:

```markdown
# Documentation
## Canonical Design
## Architecture
## Area Ownership
```

Document ownership for future `architecture/`, `frontend/`, `backend/`, `checker/`, `devops/`, `testing/`, `security/`, `operations/`, and `adr/` areas, but explicitly state that directories are created only when they first contain a real document.

- [ ] **Step 6: Verify text hygiene**

```bash
git diff --check
```

Expected: exit `0`, no output.

- [ ] **Step 7: Commit**

```bash
git add .editorconfig .gitattributes .gitignore README.md docs/README.md
git commit -m "chore: establish repository metadata baseline"
```

---

### Task 2: Define contributor, security, and repository governance policy

**Files:**
- Create: `CONTRIBUTING.md`
- Create: `SECURITY.md`
- Create: `docs/devops/repository-governance.md`

**Interfaces:**
- Consumes: Git strategy, PR standard, CI strategy, security architecture, and Definition of Done from the spec.
- Produces: human-readable governance and exact GitHub repository settings to activate after the baseline PR has merged.

- [ ] **Step 1: Create `CONTRIBUTING.md`**

It must explicitly define:

1. `main` as the only long-lived branch;
2. branch prefixes `feat/`, `fix/`, `refactor/`, `test/`, `docs/`, `ci/`, `chore/`;
3. issue numbers in branch names once an issue exists;
4. no normal direct push to `main`;
5. Conventional Commit subjects for every branch commit;
6. Conventional Commit PR titles because the PR title becomes the squash commit subject;
7. rebasing/updating from `main` rather than merge commits in topic branches;
8. English project-facing documentation;
9. explicit PR sections for architecture, contract, database, security, testing, docs, and breaking-change impact, using `None` when not applicable;
10. Definition of Done requiring relevant implementation, tests, contracts, migrations, architecture checks, security, observability, docs, and CI evidence.

Include these examples:

```text
feat/12-create-monitor
fix/31-checker-timeout
refactor/44-monitor-domain
test/52-probe-retry
docs/18-architecture
ci/23-path-aware-checks
chore/repository-bootstrap
```

```text
feat(api): add monitor registration use case
fix(checker): enforce probe timeout
test(api): cover disabled monitor scheduling
ci(checker): add clippy verification
docs(architecture): document module boundaries
```

- [ ] **Step 2: Create `SECURITY.md`**

State explicitly that:

- foundation builds are experimental and unauthenticated builds are not suitable for arbitrary internet exposure;
- suspected vulnerabilities should use GitHub private vulnerability reporting when available rather than a public issue;
- exploit details should not be publicly disclosed before a private reporting path exists;
- secrets and real credentials must never be committed;
- logs must avoid credentials, tokens, and sensitive URL components;
- user-provided probe targets make SSRF, DNS rebinding, redirect validation, private/reserved address blocking, cloud metadata access, bounded response size, bounded request duration, port policy, and bounded concurrency first-class threats;
- checker controls not yet implemented are described as future requirements, not current guarantees;
- security fixes should include regression evidence at the cheapest meaningful test layer.

- [ ] **Step 3: Create `docs/devops/repository-governance.md`**

Use these headings:

```markdown
# Repository Governance
## Purpose
## Branch Model
## Branch Naming
## Commit Policy
## Pull Request Policy
## Merge Policy
## Required CI Gate
## Main Branch Ruleset
## GitHub Actions Security
## Dependency Updates
## Bootstrap Exception
## Verification
## Related Architecture Decisions
```

The target `main` settings must be documented exactly:

```text
Target branch: main
Enforcement: active
Pull request required: yes
Required approvals: 0 while there is one active maintainer
Conversation resolution: required
Required status check: CI / gate
Require branch up to date before merge: yes, after CI / gate exists on main
Linear history: required
Force pushes: blocked
Branch deletion: blocked
Normal-development bypass actors: none
Merge commits: disabled
Rebase merge: disabled
Squash merge: enabled
Delete head branch after merge: enabled
```

Explain why required approvals are initially `0`: PR + CI remains mandatory without creating an impossible self-approval requirement. Strengthen review requirements when another maintainer exists.

- [ ] **Step 4: Verify and commit**

```bash
git diff --check
git add CONTRIBUTING.md SECURITY.md docs/devops/repository-governance.md
git commit -m "docs: define repository governance policies"
```

Expected: `git diff --check` exits `0` before commit.

---

### Task 3: Add structured GitHub collaboration templates and ownership

**Files:**
- Create: `.github/CODEOWNERS`
- Create: `.github/PULL_REQUEST_TEMPLATE.md`
- Create: `.github/ISSUE_TEMPLATE/bug_report.yml`
- Create: `.github/ISSUE_TEMPLATE/feature_request.yml`
- Create: `.github/ISSUE_TEMPLATE/config.yml`

**Interfaces:**
- Consumes: `CONTRIBUTING.md` and foundation PR standard.
- Produces: structured PR evidence, issue intake, and initial ownership routing.

- [ ] **Step 1: Create `.github/CODEOWNERS`**

```text
* @kefyusuf
```

Do not make code-owner approval mandatory while only one maintainer exists.

- [ ] **Step 2: Create `.github/PULL_REQUEST_TEMPLATE.md`**

```markdown
## Summary

## Why

## Scope

## Architecture impact

## Contract impact

## Database impact

## Security impact

## Testing evidence

## Documentation

## Breaking changes

## Checklist

- [ ] The PR title follows Conventional Commits.
- [ ] The change is scoped to one coherent reviewable outcome.
- [ ] Tests were added or updated where behavior changed.
- [ ] Architecture boundaries remain valid or the relevant decision/documentation was updated.
- [ ] Contract and database impacts are described explicitly, including `None` where not applicable.
- [ ] Security and observability impacts were considered.
- [ ] Contributor-facing documentation is in English.
- [ ] No secrets or real credentials are included.
```

- [ ] **Step 3: Create `.github/ISSUE_TEMPLATE/bug_report.yml`**

```yaml
name: Bug report
description: Report a reproducible defect in uptime-lab
title: "bug: "
labels: []
body:
  - type: markdown
    attributes:
      value: |
        Thanks for reporting a defect. Do not include secrets, credentials, or private target URLs.
  - type: textarea
    id: description
    attributes:
      label: Description
      description: What happened, and what did you expect instead?
    validations:
      required: true
  - type: textarea
    id: reproduction
    attributes:
      label: Reproduction
      description: Provide the smallest deterministic sequence that reproduces the problem.
    validations:
      required: true
  - type: textarea
    id: environment
    attributes:
      label: Environment
      description: Include OS, Docker/Compose versions, revision, and relevant runtime information.
    validations:
      required: true
  - type: textarea
    id: evidence
    attributes:
      label: Evidence
      description: Include sanitized logs, screenshots, failing test names, or other useful evidence.
    validations:
      required: false
  - type: checkboxes
    id: checks
    attributes:
      label: Checks
      options:
        - label: I removed secrets, credentials, and private target data.
          required: true
        - label: I searched existing issues for the same defect.
          required: true
```

- [ ] **Step 4: Create `.github/ISSUE_TEMPLATE/feature_request.yml`**

```yaml
name: Feature request
description: Propose a capability or engineering improvement
title: "feat: "
labels: []
body:
  - type: textarea
    id: problem
    attributes:
      label: Problem
      description: What concrete problem should this change solve?
    validations:
      required: true
  - type: textarea
    id: outcome
    attributes:
      label: Desired outcome
      description: Describe observable behavior rather than a preferred implementation when possible.
    validations:
      required: true
  - type: textarea
    id: boundaries
    attributes:
      label: Affected boundaries
      description: Note expected impact on web, API, checker, contracts, persistence, security, or operations.
    validations:
      required: true
  - type: textarea
    id: alternatives
    attributes:
      label: Alternatives considered
      description: Describe simpler alternatives or why existing behavior is insufficient.
    validations:
      required: false
```

- [ ] **Step 5: Create `.github/ISSUE_TEMPLATE/config.yml`**

```yaml
blank_issues_enabled: false
contact_links:
  - name: Security vulnerability
    url: https://github.com/kefyusuf/uptime-lab/security
    about: Review the security policy before disclosing vulnerability details.
```

- [ ] **Step 6: Verify and commit**

```bash
git diff --check
git status --short
git add .github/CODEOWNERS .github/PULL_REQUEST_TEMPLATE.md .github/ISSUE_TEMPLATE
git commit -m "chore: add collaboration templates and ownership"
```

Before staging, `git status --short` must show only the five intended `.github` files from this task.

---

### Task 4: Build dependency-free governance checks using TDD

**Files:**
- Create: `scripts/ci/lib/conventional.sh`
- Create: `scripts/ci/check-pr-title.sh`
- Create: `scripts/ci/check-commit-range.sh`
- Create: `scripts/ci/check-repository-shape.sh`
- Create: `scripts/ci/test-governance.sh`

**Interfaces:**
- Consumes: commit types/scopes and permanent repository invariants.
- Produces:
  - `is_conventional_subject <subject>`;
  - `check-pr-title.sh <title>`;
  - `check-commit-range.sh <base-sha> <head-sha>`;
  - `check-repository-shape.sh [root]`.

- [ ] **Step 1: Write the failing test harness first**

Create `scripts/ci/test-governance.sh`:

```bash
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
```

Make it executable:

```bash
chmod +x scripts/ci/test-governance.sh
```

- [ ] **Step 2: Run the harness and verify RED**

```bash
./scripts/ci/test-governance.sh
```

Expected: non-zero exit because the three checker scripts do not exist yet.

- [ ] **Step 3: Implement the shared Conventional Commit predicate**

Create `scripts/ci/lib/conventional.sh`:

```bash
#!/usr/bin/env bash

CONVENTIONAL_TYPES='feat|fix|refactor|test|docs|ci|build|chore|perf|revert'
CONVENTIONAL_SCOPES='web|api|checker|contracts|devops|architecture|docs'
CONVENTIONAL_SUBJECT_RE="^(${CONVENTIONAL_TYPES})(\\((${CONVENTIONAL_SCOPES})\\))?(!)?: [a-z0-9][^[:cntrl:]]*$"

is_conventional_subject() {
  local subject="${1:-}"
  [[ "$subject" =~ $CONVENTIONAL_SUBJECT_RE ]]
}

print_conventional_error() {
  local subject="${1:-}"
  printf 'Invalid Conventional Commit subject: %s\n' "$subject" >&2
  printf 'Expected: <type>(<scope>): <lowercase description>\n' >&2
  printf 'Types: feat, fix, refactor, test, docs, ci, build, chore, perf, revert\n' >&2
  printf 'Scopes: web, api, checker, contracts, devops, architecture, docs\n' >&2
}
```

- [ ] **Step 4: Implement PR-title validation**

Create `scripts/ci/check-pr-title.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib/conventional.sh
source "$SCRIPT_DIR/lib/conventional.sh"

if [[ "$#" -ne 1 ]]; then
  printf 'Usage: %s <pull-request-title>\n' "$0" >&2
  exit 2
fi

if ! is_conventional_subject "$1"; then
  print_conventional_error "$1"
  exit 1
fi
```

- [ ] **Step 5: Implement commit-range validation**

Create `scripts/ci/check-commit-range.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib/conventional.sh
source "$SCRIPT_DIR/lib/conventional.sh"

if [[ "$#" -ne 2 ]]; then
  printf 'Usage: %s <base-sha> <head-sha>\n' "$0" >&2
  exit 2
fi

BASE_SHA="$1"
HEAD_SHA="$2"

if [[ "$BASE_SHA" =~ ^0{40}$ ]]; then
  mapfile -t SUBJECTS < <(git log -1 --format=%s "$HEAD_SHA")
else
  mapfile -t SUBJECTS < <(git log --format=%s "$BASE_SHA..$HEAD_SHA")
fi

if [[ "${#SUBJECTS[@]}" -eq 0 ]]; then
  printf 'No commits found for range %s..%s\n' "$BASE_SHA" "$HEAD_SHA" >&2
  exit 1
fi

FAILED=0
for subject in "${SUBJECTS[@]}"; do
  if ! is_conventional_subject "$subject"; then
    print_conventional_error "$subject"
    FAILED=1
  fi
done

exit "$FAILED"
```

- [ ] **Step 6: Implement repository-shape validation**

Create `scripts/ci/check-repository-shape.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail

ROOT="${1:-.}"
REQUIRED_SPEC="docs/superpowers/specs/2026-09-17-uptime-lab-foundation-design.md"
FORBIDDEN_DIRS=(shared common utils helpers)

if [[ ! -f "$ROOT/$REQUIRED_SPEC" ]]; then
  printf 'Missing canonical foundation spec: %s\n' "$REQUIRED_SPEC" >&2
  exit 1
fi

for directory in "${FORBIDDEN_DIRS[@]}"; do
  if [[ -d "$ROOT/$directory" ]]; then
    printf 'Forbidden root business directory: %s/\n' "$directory" >&2
    exit 1
  fi
done

if git -C "$ROOT" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  if git -C "$ROOT" ls-files --error-unmatch .env >/dev/null 2>&1; then
    printf 'Tracked root .env is forbidden. Keep local secrets untracked and introduce .env.example when configuration exists.\n' >&2
    exit 1
  fi
fi
```

The checker intentionally tests only permanent invariants and does not use a top-level allow-list that would block future planned directories.

- [ ] **Step 7: Make checker scripts executable**

```bash
chmod +x \
  scripts/ci/check-pr-title.sh \
  scripts/ci/check-commit-range.sh \
  scripts/ci/check-repository-shape.sh \
  scripts/ci/test-governance.sh
```

- [ ] **Step 8: Re-run and verify GREEN**

```bash
./scripts/ci/test-governance.sh
```

Expected final line:

```text
Governance tests: 12 passed, 0 failed
```

- [ ] **Step 9: Verify against the real repository and commit**

```bash
./scripts/ci/check-repository-shape.sh .
./scripts/ci/check-pr-title.sh "chore: establish repository engineering baseline"
git diff --check
git add scripts/ci
git commit -m "test: add repository governance checks"
```

Expected: all checks exit `0` before commit.

---

### Task 5: Add the stable GitHub Actions baseline gate

**Files:**
- Create: `.github/workflows/ci.yml`

**Interfaces:**
- Consumes: Task 4 policy scripts.
- Produces: `CI / policy`, `CI / repository`, and stable aggregate `CI / gate`.

- [ ] **Step 1: Create `.github/workflows/ci.yml`**

```yaml
name: CI

on:
  pull_request:
  push:
    branches:
      - main

permissions:
  contents: read

concurrency:
  group: ci-${{ github.workflow }}-${{ github.event.pull_request.number || github.ref }}
  cancel-in-progress: true

jobs:
  policy:
    name: policy
    runs-on: ubuntu-24.04
    timeout-minutes: 5
    steps:
      - name: Check out repository
        uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
        with:
          fetch-depth: 0
          persist-credentials: false

      - name: Validate pull request title
        if: github.event_name == 'pull_request'
        env:
          PR_TITLE: ${{ github.event.pull_request.title }}
        run: ./scripts/ci/check-pr-title.sh "$PR_TITLE"

      - name: Validate commit subjects
        env:
          BASE_SHA: ${{ github.event.pull_request.base.sha || github.event.before }}
          HEAD_SHA: ${{ github.event.pull_request.head.sha || github.sha }}
        run: ./scripts/ci/check-commit-range.sh "$BASE_SHA" "$HEAD_SHA"

  repository:
    name: repository
    runs-on: ubuntu-24.04
    timeout-minutes: 5
    steps:
      - name: Check out repository
        uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
        with:
          fetch-depth: 0
          persist-credentials: false

      - name: Run governance tests
        run: ./scripts/ci/test-governance.sh

      - name: Validate repository invariants
        run: ./scripts/ci/check-repository-shape.sh .

      - name: Validate whitespace
        env:
          BASE_SHA: ${{ github.event.pull_request.base.sha || github.event.before }}
          HEAD_SHA: ${{ github.event.pull_request.head.sha || github.sha }}
        run: git diff --check "$BASE_SHA" "$HEAD_SHA"

  gate:
    name: gate
    if: ${{ always() }}
    needs:
      - policy
      - repository
    runs-on: ubuntu-24.04
    timeout-minutes: 2
    steps:
      - name: Require baseline jobs
        env:
          POLICY_RESULT: ${{ needs.policy.result }}
          REPOSITORY_RESULT: ${{ needs.repository.result }}
        run: |
          set -euo pipefail
          test "$POLICY_RESULT" = "success"
          test "$REPOSITORY_RESULT" = "success"
```

Security properties are deliberate: read-only permissions, no secret, no `pull_request_target`, checkout credentials removed, PR title passed via environment, immutable action SHA, superseded runs cancelled.

- [ ] **Step 2: Run local checks**

```bash
./scripts/ci/test-governance.sh
./scripts/ci/check-repository-shape.sh .
./scripts/ci/check-pr-title.sh "chore: establish repository engineering baseline"
git diff --check
```

Expected: all exit `0`.

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/ci.yml
git commit -m "ci: establish stable repository gate"
```

- [ ] **Step 4: After the implementation PR is opened, verify GitHub checks**

Required observed checks:

```text
CI / policy
CI / repository
CI / gate
```

The gate must fail when either upstream job fails.

---

### Task 6: Add conservative dependency automation for infrastructure that exists

**Files:**
- Create: `.github/dependabot.yml`

**Interfaces:**
- Consumes: GitHub Actions workflow.
- Produces: weekly GitHub Actions dependency-update PRs only.

- [ ] **Step 1: Create `.github/dependabot.yml`**

```yaml
version: 2
updates:
  - package-ecosystem: github-actions
    directory: /
    schedule:
      interval: weekly
      day: monday
      time: "04:00"
      timezone: Etc/UTC
    open-pull-requests-limit: 5
```

Do not add npm, Go modules, Cargo, or Docker ecosystems before their manifests exist.

- [ ] **Step 2: Verify and commit**

```bash
./scripts/ci/test-governance.sh
./scripts/ci/check-repository-shape.sh .
git diff --check
git add .github/dependabot.yml
git commit -m "chore: configure GitHub Actions dependency updates"
```

Expected: all checks exit `0` before commit.

---

### Task 7: Open and validate the repository baseline pull request

**Files:**
- No new source files.
- Review all files created by Tasks 1–6.

**Interfaces:**
- Consumes: completed repository baseline branch.
- Produces: one reviewable PR whose title becomes the squash commit subject.

- [ ] **Step 1: Validate the complete branch**

```bash
./scripts/ci/test-governance.sh
./scripts/ci/check-repository-shape.sh .
./scripts/ci/check-pr-title.sh "chore: establish repository engineering baseline"
git diff --check main...HEAD
git log --oneline main..HEAD
```

Expected:

- `Governance tests: 12 passed, 0 failed`;
- repository shape passes;
- PR title passes;
- whitespace check has no output;
- every branch commit subject follows the documented convention.

- [ ] **Step 2: Open the PR**

Title:

```text
chore: establish repository engineering baseline
```

The PR body must include these concrete impact statements:

```text
Architecture impact: Establishes repository governance only; no runtime architecture implementation.
Contract impact: None.
Database impact: None.
Security impact: Adds security-reporting guidance and least-privilege CI policy; no runtime security control is claimed.
Testing evidence: Governance shell test harness plus CI policy/repository/gate jobs.
Documentation: Adds repository, contribution, security, documentation-index, and governance guidance.
Breaking changes: None; no application runtime existed.
```

- [ ] **Step 3: Require successful CI evidence**

```text
CI / policy      success
CI / repository  success
CI / gate        success
```

Do not merge a failed, cancelled, or missing gate.

- [ ] **Step 4: Reject scope leakage**

The PR must contain none of:

```text
apps/
contracts/
deploy/
compose.yaml
Dockerfile
package.json
go.mod
Cargo.toml
```

If present, remove those changes and leave them for their owning plan.

---

### Task 8: Squash-merge the baseline and activate `main` protection

**Files:**
- No source changes unless PR review feedback requires documentation corrections.

**Interfaces:**
- Consumes: successful PR with visible `CI / gate`.
- Produces: protected `main` behavior matching the foundation spec.

- [ ] **Step 1: Squash-merge the PR**

Squash commit subject:

```text
chore: establish repository engineering baseline
```

Do not use merge-commit or rebase-merge mode.

- [ ] **Step 2: Configure repository merge settings**

```text
Allow squash merging: ON
Allow merge commits: OFF
Allow rebase merging: OFF
Automatically delete head branches: ON
```

- [ ] **Step 3: Activate the `main` ruleset**

```text
Target: main
Enforcement: Active
Require pull request: ON
Required approvals: 0
Require conversation resolution: ON
Required status check: CI / gate
Require branch up to date before merge: ON
Require linear history: ON
Block force pushes: ON
Block branch deletion: ON
Normal-development bypass actors: none
```

Do not enable required signed commits in this baseline; signature policy needs a separately verified contributor workflow and the initial root commit is unsigned.

- [ ] **Step 4: Verify repository merge settings non-destructively**

```bash
gh repo view kefyusuf/uptime-lab --json defaultBranchRef,mergeCommitAllowed,rebaseMergeAllowed,squashMergeAllowed,deleteBranchOnMerge
```

Expected semantic values:

```text
default branch: main
mergeCommitAllowed: false
rebaseMergeAllowed: false
squashMergeAllowed: true
deleteBranchOnMerge: true
```

Inspect the active ruleset through GitHub UI or authenticated GitHub API and confirm the PR requirement, `CI / gate`, up-to-date requirement, linear history, force-push block, and deletion block. Do not test protection by direct-pushing or force-pushing `main`.

- [ ] **Step 5: Verify from a clean clone**

```bash
git clone https://github.com/kefyusuf/uptime-lab.git uptime-lab-verify
cd uptime-lab-verify
./scripts/ci/test-governance.sh
./scripts/ci/check-repository-shape.sh .
git log -1 --format=%s
```

Expected:

```text
Governance tests: 12 passed, 0 failed
chore: establish repository engineering baseline
```

- [ ] **Step 6: Record operational evidence in the merged PR conversation**

Record:

```text
- CI / gate: passed on merged baseline revision
- Merge mode: squash
- main ruleset: active
- Required check: CI / gate
- Force push/deletion: blocked
- Clean-clone governance tests: passed
```

---

## Implementer Self-Review

Before completion, verify:

- [ ] No application runtime, contract, database, or Docker implementation entered this PR.
- [ ] Every repository-facing document is English.
- [ ] PR title and branch commits use the same Conventional Commit predicate.
- [ ] Governance tests are dependency-free and report `12 passed, 0 failed`.
- [ ] A local untracked `.env` is allowed and a tracked root `.env` is rejected.
- [ ] `CI / gate` is the stable branch-protection status.
- [ ] GitHub Actions permissions are read-only and checkout credentials are not persisted.
- [ ] Checkout is pinned to `3d3c42e5aac5ba805825da76410c181273ba90b1`.
- [ ] `main` protection is activated only after the gate exists and has succeeded.
- [ ] Required approvals remain `0` while only one active maintainer exists.
- [ ] Merge commits and rebase merge are disabled; squash merge is enabled.
- [ ] No root business `shared`, `common`, `utils`, or `helpers` directory exists.
- [ ] No license was silently selected.

## Exit Criteria

This plan is complete only when:

1. the repository-baseline PR is squash-merged;
2. `main` contains the human-readable governance baseline;
3. governance checks pass from a clean clone;
4. `CI / policy`, `CI / repository`, and `CI / gate` are successful;
5. `main` is protected by PR-first, up-to-date, linear-history, no-force-push, no-deletion rules with `CI / gate` required;
6. only GitHub Actions dependency automation is configured;
7. no runtime, Docker environment, API contract, persistence layer, or product feature is introduced.

The next implementation-plan gate is **Architecture Documentation Baseline**. It starts only after this plan is implemented, verified, and explicitly continued.