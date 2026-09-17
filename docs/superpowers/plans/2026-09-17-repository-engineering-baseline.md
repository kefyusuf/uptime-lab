# Repository Engineering Baseline Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Establish the first production-disciplined repository governance baseline for `uptime-lab`: contributor documentation, Git/PR conventions, policy checks, a stable GitHub Actions gate, dependency-update scaffolding, and protected `main` semantics without introducing application runtime code.

**Architecture:** This plan implements only **PR 1 — Repository engineering baseline** from the approved foundation design. Repository policy is encoded twice: human-readable rules in English documentation and mechanically enforced checks in small dependency-free shell scripts plus a minimal GitHub Actions workflow. Go, Rust, React, PostgreSQL, Docker Compose, OpenAPI runtime contracts, and product features remain outside this plan and receive separate implementation plans.

**Tech Stack:** Git, GitHub pull requests, GitHub Actions, Bash, Markdown, YAML, Dependabot.

**Spec:** `docs/superpowers/specs/2026-09-17-uptime-lab-foundation-design.md`

## Global Constraints

- The repository is a monorepo.
- `main` is the only long-lived branch; there is no permanent `develop` branch.
- After the initial empty-repository bootstrap commit, changes use short-lived branches and pull requests.
- Conventional Commits are required. Canonical form: `<type>(<scope>): <description>`; scope is optional and breaking changes may use `!` before `:`.
- Canonical commit types are `feat`, `fix`, `refactor`, `test`, `docs`, `ci`, `build`, `chore`, `perf`, and `revert`.
- Canonical scopes are `web`, `api`, `checker`, `contracts`, `devops`, `architecture`, and `docs`.
- Squash merge is the canonical merge strategy; merge commits and rebase merge are disabled for `main`.
- Repository-facing documentation, templates, ADRs, and contributor-facing text are written in English.
- GitHub Actions permissions default to read-only. This plan does not introduce write permissions or repository secrets.
- Third-party actions are avoided in this baseline. The only action used by CI is `actions/checkout`, pinned to immutable commit `3d3c42e5aac5ba805825da76410c181273ba90b1` (`v7.0.1`, resolved on 2026-09-17).
- The stable branch-protection status check is `CI / gate`.
- The baseline must not create application source trees merely to reserve future structure. No Go, Rust, React, PostgreSQL, Docker Compose, or product implementation is introduced here.
- A root-level business `shared`, `common`, `utils`, or `helpers` directory is forbidden.
- Licensing is intentionally not selected in this plan because the approved foundation specification does not lock a license and licensing has legal consequences for external reuse. The repository remains without a license until the owner makes that explicit decision; this does not block the engineering baseline.

---

## Plan Boundary and Follow-up Plans

The approved foundation specification spans multiple independently reviewable subsystems. They must not be implemented as one large change. The sequence is:

1. **This plan:** repository engineering baseline.
2. Architecture documentation baseline.
3. Docker-first local development environment.
4. Go control-plane / monitoring-module foundation.
5. Rust checker foundation.
6. React/TypeScript frontend foundation.
7. First end-to-end monitoring vertical slice.

Completion of this plan authorizes only step 1. It must not automatically start step 2.

---

## Target File Map for This Plan

```text
uptime-lab/
├── .github/
│   ├── CODEOWNERS                         # Initial ownership boundary.
│   ├── PULL_REQUEST_TEMPLATE.md           # Required PR evidence and impact sections.
│   ├── ISSUE_TEMPLATE/
│   │   ├── bug_report.yml                 # Structured defect intake.
│   │   ├── feature_request.yml            # Structured capability proposal intake.
│   │   └── config.yml                     # Disable unstructured blank issues.
│   ├── dependabot.yml                     # GitHub Actions update policy only.
│   └── workflows/
│       └── ci.yml                          # Stable baseline CI with `CI / gate`.
├── docs/
│   ├── README.md                           # Documentation index and ownership map.
│   ├── devops/
│   │   └── repository-governance.md        # Git, merge, branch-protection, CI policy.
│   └── superpowers/
│       ├── specs/
│       │   └── 2026-09-17-uptime-lab-foundation-design.md
│       └── plans/
│           └── 2026-09-17-repository-engineering-baseline.md
├── scripts/
│   └── ci/
│       ├── lib/
│       │   └── conventional.sh             # One canonical Conventional Commit predicate.
│       ├── check-pr-title.sh               # Validates squash-commit source title.
│       ├── check-commit-range.sh            # Validates branch commit subjects.
│       ├── check-repository-shape.sh        # Enforces permanent repository invariants.
│       └── test-governance.sh               # Dependency-free tests for policy scripts.
├── .editorconfig                           # Cross-editor text conventions.
├── .gitattributes                          # Deterministic line-ending policy.
├── .gitignore                              # Repository-wide secret/editor/OS ignores.
├── CONTRIBUTING.md                         # Contributor workflow and Definition of Done.
├── README.md                               # Product/repository entry point.
└── SECURITY.md                             # Security-reporting and initial exposure policy.
```

No empty `apps/`, `contracts/`, `deploy/`, or runtime-specific directories are created in this PR. They appear only when the plan that owns their first executable artifact is implemented.

---

### Task 1: Establish deterministic root repository metadata and navigation

**Files:**
- Create: `.editorconfig`
- Create: `.gitattributes`
- Create: `.gitignore`
- Create: `README.md`
- Create: `docs/README.md`

**Interfaces:**
- Consumes: approved foundation spec.
- Produces: canonical repository entry point, documentation index, text/line-ending policy, and safe ignore defaults used by every later plan.

- [ ] **Step 1: Create `.editorconfig` with repository-wide text rules**

Use exactly:

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

Rationale: LF is canonical in Git and containers; Markdown permits intentional two-space hard breaks; Makefiles require tabs.

- [ ] **Step 2: Create `.gitattributes` with deterministic normalization**

Use exactly:

```gitattributes
* text=auto eol=lf
*.bat text eol=crlf
*.cmd text eol=crlf
```

Do not add language-specific generated/binary rules before those files exist.

- [ ] **Step 3: Create `.gitignore` with only cross-repository baseline ignores**

Use exactly:

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

Language-specific build outputs such as `node_modules/`, `target/`, and Go binaries are added by the plan that introduces that ecosystem so ownership remains explicit.

- [ ] **Step 4: Create root `README.md`**

The README must contain these sections in this order:

```markdown
# uptime-lab

A production-disciplined uptime monitoring laboratory built to exercise clear boundaries between a React/TypeScript web client, a Go control plane, and a Rust checker runtime.

## Status

`uptime-lab` is in the foundation phase. The architecture is approved, but the application runtimes are not implemented yet. The repository should not be treated as a production-ready monitoring service.

## Architecture

- React + TypeScript: presentation and browser interaction.
- Go: modular-monolith control plane, domain/application rules, persistence ownership, and API semantics.
- Rust: bounded concurrent network probe execution through Ports and Adapters.
- PostgreSQL: durable application state owned exclusively by the Go control plane.
- Docker Compose: canonical local development topology once the Docker foundation is implemented.

The canonical architecture specification lives at `docs/superpowers/specs/2026-09-17-uptime-lab-foundation-design.md`.

## Engineering Principles

- Boundaries before abstractions.
- Contract-driven runtime integration.
- One owner per source of truth.
- Tests at the cheapest meaningful layer.
- Security and observability are design constraints, not release-time patches.
- No distributed infrastructure without a concrete requirement.

## Repository Workflow

`main` is the only long-lived branch. Changes use short-lived branches, pull requests, Conventional Commits, required CI, and squash merge.

See `CONTRIBUTING.md` and `docs/devops/repository-governance.md`.

## Documentation

Start at `docs/README.md`.

## Security

Read `SECURITY.md` before reporting a vulnerability or exposing an experimental build to untrusted networks.

## License

No open-source license has been selected yet. Until a license is explicitly added, normal copyright rules apply.
```

Do not add badges until the corresponding CI/release surface exists and has a stable URL.

- [ ] **Step 5: Create `docs/README.md` as the documentation ownership map**

It must explain:

```markdown
# Documentation

Project documentation is organized by responsibility and by cross-runtime architecture.

## Canonical Design

- Foundation design: `superpowers/specs/2026-09-17-uptime-lab-foundation-design.md`
- Implementation plans: `superpowers/plans/`

## Architecture

Architecture documentation describes end-to-end relationships across web, API, checker, persistence, security, and operations. Area-specific documentation must link back to those cross-area flows instead of becoming isolated silos.

## Area Ownership

- `architecture/`: system-level boundaries and cross-area flows.
- `frontend/`: React/TypeScript architecture and browser concerns.
- `backend/`: Go control-plane architecture and modular-monolith rules.
- `checker/`: Rust checker execution architecture.
- `devops/`: repository governance, local development, CI/CD, and release operations.
- `testing/`: cross-runtime test strategy and evidence model.
- `security/`: threat model and security controls.
- `operations/`: health, observability, and troubleshooting.
- `adr/`: material architecture decisions once ADR baseline work begins.

Directories are created when they first contain an owned document; empty documentation trees are not pre-created.
```

- [ ] **Step 6: Verify deterministic text hygiene**

Run:

```bash
git diff --check
```

Expected: exit code `0` with no output.

- [ ] **Step 7: Commit Task 1**

```bash
git add .editorconfig .gitattributes .gitignore README.md docs/README.md
git commit -m "chore: establish repository metadata baseline"
```

---

### Task 2: Define contributor, security, and repository-governance policy

**Files:**
- Create: `CONTRIBUTING.md`
- Create: `SECURITY.md`
- Create: `docs/devops/repository-governance.md`

**Interfaces:**
- Consumes: Git strategy, PR standard, CI architecture, security architecture, and Definition of Done from the foundation spec.
- Produces: the human-readable source used by contributors and the exact repository settings to activate after the baseline PR is merged.

- [ ] **Step 1: Create `CONTRIBUTING.md`**

The document must contain all of the following rules explicitly:

1. `main` is the only long-lived branch.
2. Branch names use one of `feat/`, `fix/`, `refactor/`, `test/`, `docs/`, `ci/`, or `chore/`; issue numbers are included once an issue exists.
3. Direct pushes to `main` are not part of the normal workflow.
4. Every commit subject follows Conventional Commits.
5. Pull-request titles follow Conventional Commits because the title becomes the squash commit subject.
6. Contributors rebase/update from `main` rather than creating merge commits in topic branches.
7. Documentation and contributor-facing copy are English.
8. PRs must explain architecture, contract, database, security, testing, documentation, and breaking-change impact; non-applicable sections say `None`.
9. A change is complete only when applicable implementation, tests, contracts, migrations, architecture checks, security considerations, observability, docs, and CI evidence agree.
10. Application changes must follow the approved spec and the active implementation plan rather than bypassing architectural gates.

Include this branch example block:

```text
feat/12-create-monitor
fix/31-checker-timeout
refactor/44-monitor-domain
test/52-probe-retry
docs/18-architecture
ci/23-path-aware-checks
chore/repository-bootstrap
```

Include these commit examples:

```text
feat(api): add monitor registration use case
fix(checker): enforce probe timeout
test(api): cover disabled monitor scheduling
ci(checker): add clippy verification
docs(architecture): document module boundaries
```

- [ ] **Step 2: Create `SECURITY.md`**

The document must state:

- the project is experimental during foundation work and unauthenticated builds are not suitable for arbitrary public internet exposure;
- suspected vulnerabilities should use GitHub's private vulnerability-reporting flow when available rather than a public issue;
- if private reporting is unavailable, exploit details must not be published before the maintainer has a private channel to receive them;
- secrets and real credentials must never be committed;
- logs must not intentionally expose credentials, tokens, or sensitive URL components;
- user-provided probe targets make SSRF, DNS rebinding, redirect validation, private/reserved address blocking, cloud metadata access, bounded response sizes, bounded duration, bounded ports, and bounded concurrency first-class threats;
- security fixes are expected to include regression evidence at the lowest meaningful layer.

Do not claim security guarantees that are not implemented yet. Use explicit future-tense wording for checker controls that belong to later plans.

- [ ] **Step 3: Create `docs/devops/repository-governance.md`**

Use these top-level sections:

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

The **Main Branch Ruleset** section must prescribe these exact target settings:

- target branch: `main`;
- enforcement: active;
- pull request required before merge;
- required approving review count: `0` while the repository has a single active maintainer;
- conversation resolution required;
- required status check: `CI / gate`;
- require branch to be up to date before merge: enabled once `CI / gate` exists on `main`;
- linear history required;
- force pushes blocked;
- branch deletion blocked;
- bypass actors: none for normal development;
- merge commits disabled;
- rebase merge disabled;
- squash merge enabled;
- automatically delete head branches after merge: enabled.

Explain that review-count `0` is intentional: a one-maintainer repository must still use PRs and CI without creating an impossible self-approval requirement. Strengthening human-review requirements is a later governance change when another maintainer exists.

- [ ] **Step 4: Verify documentation hygiene**

Run:

```bash
git diff --check
```

Expected: exit code `0` with no output.

- [ ] **Step 5: Commit Task 2**

```bash
git add CONTRIBUTING.md SECURITY.md docs/devops/repository-governance.md
git commit -m "docs: define repository governance policies"
```

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
- Produces: consistent PR evidence, structured issue intake, and initial ownership routing.

- [ ] **Step 1: Create `.github/CODEOWNERS`**

Use:

```text
* @kefyusuf
```

Do not enable mandatory code-owner approval while there is only one active maintainer.

- [ ] **Step 2: Create `.github/PULL_REQUEST_TEMPLATE.md`**

Use exactly these headings and checklist semantics:

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

Use:

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

Use:

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

Use:

```yaml
blank_issues_enabled: false
contact_links:
  - name: Security vulnerability
    url: https://github.com/kefyusuf/uptime-lab/security
    about: Review the security policy before disclosing vulnerability details.
```

- [ ] **Step 6: Verify no malformed whitespace or secret-like local files were added**

Run:

```bash
git diff --check
git status --short
```

Expected: `git diff --check` exits `0`; `git status --short` lists only the five intended `.github` files before staging.

- [ ] **Step 7: Commit Task 3**

```bash
git add .github/CODEOWNERS .github/PULL_REQUEST_TEMPLATE.md .github/ISSUE_TEMPLATE
git commit -m "chore: add collaboration templates and ownership"
```

---

### Task 4: Build dependency-free governance policy checks using TDD

**Files:**
- Create: `scripts/ci/lib/conventional.sh`
- Create: `scripts/ci/check-pr-title.sh`
- Create: `scripts/ci/check-commit-range.sh`
- Create: `scripts/ci/check-repository-shape.sh`
- Create: `scripts/ci/test-governance.sh`

**Interfaces:**
- Consumes: Conventional Commit types/scopes and repository invariants from this plan.
- Produces:
  - `is_conventional_subject <subject>` shell function;
  - `check-pr-title.sh <title>` exit `0` for valid Conventional Commit title, `1` otherwise;
  - `check-commit-range.sh <base-sha> <head-sha>` exit `0` only when every commit subject in the range is valid;
  - `check-repository-shape.sh [root]` exit `0` only when permanent repository invariants hold.

- [ ] **Step 1: Write the failing governance test harness first**

Create `scripts/ci/test-governance.sh` initially with tests that call scripts which do not exist yet:

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
trap 'rm -rf "$TMP_ROOT"' EXIT
mkdir -p "$TMP_ROOT/docs/superpowers/specs"
touch "$TMP_ROOT/docs/superpowers/specs/2026-09-17-uptime-lab-foundation-design.md"
expect_success "valid repository shape" "$SCRIPT_DIR/check-repository-shape.sh" "$TMP_ROOT"
mkdir "$TMP_ROOT/shared"
expect_failure "reject forbidden root shared directory" "$SCRIPT_DIR/check-repository-shape.sh" "$TMP_ROOT"
rmdir "$TMP_ROOT/shared"
touch "$TMP_ROOT/.env"
expect_failure "reject tracked-style root .env presence" "$SCRIPT_DIR/check-repository-shape.sh" "$TMP_ROOT"
rm "$TMP_ROOT/.env"

TMP_GIT="$(mktemp -d)"
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

- [ ] **Step 2: Run the test harness and verify RED**

Run:

```bash
./scripts/ci/test-governance.sh
```

Expected: non-zero exit because `check-pr-title.sh`, `check-repository-shape.sh`, and `check-commit-range.sh` do not exist yet.

- [ ] **Step 3: Implement one canonical Conventional Commit predicate**

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

This file is a narrowly scoped CI helper, not an application `shared` package.

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

- [ ] **Step 6: Implement repository-shape invariants**

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

if [[ -f "$ROOT/.env" ]]; then
  printf 'Root .env must not exist in repository validation context. Use .env.example when introduced.\n' >&2
  exit 1
fi
```

This check intentionally enforces only permanent invariants. It must not hard-code an allow-list of future top-level directories.

- [ ] **Step 7: Make policy scripts executable**

Run:

```bash
chmod +x \
  scripts/ci/check-pr-title.sh \
  scripts/ci/check-commit-range.sh \
  scripts/ci/check-repository-shape.sh \
  scripts/ci/test-governance.sh
```

- [ ] **Step 8: Run the governance test harness and verify GREEN**

Run:

```bash
./scripts/ci/test-governance.sh
```

Expected final line:

```text
Governance tests: 11 passed, 0 failed
```

- [ ] **Step 9: Verify the scripts against the actual repository**

Run:

```bash
./scripts/ci/check-repository-shape.sh .
./scripts/ci/check-pr-title.sh "chore: establish repository engineering baseline"
git diff --check
```

Expected: all commands exit `0`.

- [ ] **Step 10: Commit Task 4**

```bash
git add scripts/ci
git commit -m "test: add repository governance checks"
```

---

### Task 5: Add the stable GitHub Actions baseline gate

**Files:**
- Create: `.github/workflows/ci.yml`

**Interfaces:**
- Consumes: policy scripts from Task 4.
- Produces: stable required check `CI / gate` and baseline jobs `CI / policy` and `CI / repository`.

- [ ] **Step 1: Create `.github/workflows/ci.yml`**

Use:

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

Security properties of this workflow are intentional:

- no secret is required;
- `GITHUB_TOKEN` has read-only `contents` permission;
- checkout does not persist credentials;
- untrusted PR title text is passed through an environment variable, not interpolated into shell source;
- no `pull_request_target` event is used;
- no third-party action is introduced;
- superseded runs are cancelled.

- [ ] **Step 2: Perform local static checks before pushing**

Run:

```bash
./scripts/ci/test-governance.sh
./scripts/ci/check-repository-shape.sh .
./scripts/ci/check-pr-title.sh "chore: establish repository engineering baseline"
git diff --check
```

Expected: all commands exit `0`.

- [ ] **Step 3: Commit Task 5**

```bash
git add .github/workflows/ci.yml
git commit -m "ci: establish stable repository gate"
```

- [ ] **Step 4: Push the topic branch and verify the workflow on GitHub**

Push:

```bash
git push -u origin chore/repository-engineering-baseline
```

After the PR exists, required observed checks are:

```text
CI / policy
CI / repository
CI / gate
```

`CI / gate` must be successful only when both upstream jobs are successful.

---

### Task 6: Add conservative dependency automation for the infrastructure that exists

**Files:**
- Create: `.github/dependabot.yml`

**Interfaces:**
- Consumes: current GitHub Actions workflow.
- Produces: weekly update PRs for GitHub Actions only. npm, Go, Cargo, and Docker ecosystems are added by the plan that first creates their manifests.

- [ ] **Step 1: Create `.github/dependabot.yml`**

Use:

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

Do not declare npm, Go modules, Cargo, or Docker update entries before their dependency files exist.

- [ ] **Step 2: Re-run governance checks**

Run:

```bash
./scripts/ci/test-governance.sh
./scripts/ci/check-repository-shape.sh .
git diff --check
```

Expected: all commands exit `0`.

- [ ] **Step 3: Commit Task 6**

```bash
git add .github/dependabot.yml
git commit -m "chore: configure GitHub Actions dependency updates"
```

---

### Task 7: Open and validate the repository-baseline pull request

**Files:**
- No new source files.
- Review all files created by Tasks 1–6.

**Interfaces:**
- Consumes: all baseline tasks.
- Produces: one reviewable PR whose title can become the canonical squash commit.

- [ ] **Step 1: Validate the complete branch locally**

Run:

```bash
./scripts/ci/test-governance.sh
./scripts/ci/check-repository-shape.sh .
./scripts/ci/check-pr-title.sh "chore: establish repository engineering baseline"
git diff --check main...HEAD
git log --oneline main..HEAD
```

Expected:

- governance tests report `11 passed, 0 failed`;
- repository shape exits `0`;
- PR-title validation exits `0`;
- whitespace validation produces no output;
- every listed commit subject follows the documented convention.

- [ ] **Step 2: Open the pull request with the canonical title**

PR title:

```text
chore: establish repository engineering baseline
```

PR body must use the repository template and state these impacts explicitly:

```text
Architecture impact: Establishes governance only; no application architecture implementation.
Contract impact: None.
Database impact: None.
Security impact: Adds reporting guidance and least-privilege CI baseline; no runtime security control is claimed.
Testing evidence: Governance shell test harness plus GitHub Actions policy/repository/gate jobs.
Documentation: Adds README, contribution, security, documentation index, and repository-governance guidance.
Breaking changes: None; repository had no application implementation.
```

- [ ] **Step 3: Verify CI evidence on the PR**

Required outcome:

```text
CI / policy      success
CI / repository  success
CI / gate        success
```

Do not merge with a failed, cancelled, or missing gate.

- [ ] **Step 4: Review the diff for scope leakage**

The PR must contain no files under:

```text
apps/
contracts/
deploy/
```

and no:

```text
compose.yaml
Dockerfile
package.json
go.mod
Cargo.toml
```

If any of these appear, remove them from this PR and return them to the plan that owns them.

---

### Task 8: Merge the baseline and activate `main` repository protection

**Files:**
- No repository source changes are required by this task unless review feedback changes documentation.

**Interfaces:**
- Consumes: successful baseline PR with `CI / gate` visible on GitHub.
- Produces: protected `main` semantics matching the approved foundation specification.

- [ ] **Step 1: Squash-merge the baseline PR**

Use the PR title as the squash commit subject:

```text
chore: establish repository engineering baseline
```

Do not use merge-commit or rebase-merge mode.

- [ ] **Step 2: Configure repository merge settings**

In GitHub repository settings, set:

```text
Allow squash merging: ON
Allow merge commits: OFF
Allow rebase merging: OFF
Automatically delete head branches: ON
```

- [ ] **Step 3: Create/activate the `main` ruleset**

Configure the ruleset exactly as documented in `docs/devops/repository-governance.md`:

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
Bypass actors for normal development: none
```

Do not enable required signed commits in this baseline; the root bootstrap commit was created unsigned, and signature policy needs a separate verified contributor workflow decision.

- [ ] **Step 4: Verify the repository settings without attempting a destructive direct push**

Using GitHub UI or authenticated GitHub CLI, confirm:

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

Then inspect the active `main` ruleset and verify `CI / gate`, pull-request requirement, linear history, force-push blocking, and deletion blocking are present. Do not test protection by force-pushing or by committing directly to `main`.

- [ ] **Step 5: Verify `main` from a clean checkout**

Run from a temporary directory:

```bash
git clone https://github.com/kefyusuf/uptime-lab.git uptime-lab-verify
cd uptime-lab-verify
./scripts/ci/test-governance.sh
./scripts/ci/check-repository-shape.sh .
git log -1 --format=%s
```

Expected:

```text
Governance tests: 11 passed, 0 failed
```

and the latest commit subject is:

```text
chore: establish repository engineering baseline
```

- [ ] **Step 6: Record completion evidence in the PR conversation**

Post a short completion note containing:

```text
- CI / gate: passed on merged baseline revision
- Merge mode: squash
- main ruleset: active
- Required check: CI / gate
- Force push/deletion: blocked
- Clean-clone governance tests: passed
```

This is operational evidence, not a new source commit.

---

## Self-Review Checklist for the Implementer

Before declaring this plan complete, verify all of the following:

- [ ] No application runtime or Docker implementation entered the baseline PR.
- [ ] Every repository-facing document is English.
- [ ] PR title and branch commits pass the same Conventional Commit predicate.
- [ ] `scripts/ci/test-governance.sh` has deterministic local tests and requires no third-party package manager.
- [ ] `CI / gate` is the only status intended for branch-protection coupling at this stage.
- [ ] GitHub Actions use read-only permissions and do not persist checkout credentials.
- [ ] The pinned checkout action is `3d3c42e5aac5ba805825da76410c181273ba90b1`.
- [ ] `main` is protected only after the baseline gate exists and has run successfully.
- [ ] Required approving review count remains `0` while only one active maintainer exists.
- [ ] Merge commits and rebase merges are disabled; squash merge is enabled.
- [ ] No root business `shared`, `common`, `utils`, or `helpers` directory exists.
- [ ] No open-source license was silently selected by the implementation worker.

---

## Exit Criteria

This plan is complete when:

1. the repository-baseline PR has been squash-merged;
2. `main` contains the human-readable governance baseline;
3. the governance scripts pass from a clean clone;
4. GitHub Actions exposes successful `CI / policy`, `CI / repository`, and `CI / gate` checks;
5. `main` has active PR-first, linear-history, no-force-push, no-deletion protection with `CI / gate` required;
6. only GitHub Actions dependency automation is configured;
7. no application runtime, Docker environment, contract, persistence, or product feature has been introduced.

The next implementation-plan gate is **Architecture Documentation Baseline**. It begins only after this plan is implemented, verified, and explicitly continued.