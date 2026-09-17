# Contributing to uptime-lab

`uptime-lab` uses a deliberately strict contribution workflow so architecture, tests, documentation, and delivery evidence remain aligned as the repository grows.

## Branch Model

`main` is the only long-lived branch. Normal development must not push directly to `main`.

Use short-lived branches with one of these prefixes:

```text
feat/
fix/
refactor/
test/
docs/
ci/
chore/
```

When work has an issue, include the issue number in the branch name.

Examples:

```text
feat/12-create-monitor
fix/31-checker-timeout
refactor/44-monitor-domain
test/52-probe-retry
docs/18-architecture
ci/23-path-aware-checks
chore/repository-bootstrap
```

Keep branches current with `main` by rebasing or otherwise updating without introducing merge commits into the topic branch.

## Commit Policy

Every commit subject must follow Conventional Commits:

```text
<type>(<scope>): <description>
```

Allowed types are `feat`, `fix`, `refactor`, `test`, `docs`, `ci`, `build`, `chore`, `perf`, and `revert`.

Canonical scopes are `web`, `api`, `checker`, `contracts`, `devops`, `architecture`, and `docs`. The scope may be omitted when the change is genuinely repository-wide.

Examples:

```text
feat(api): add monitor registration use case
fix(checker): enforce probe timeout
test(api): cover disabled monitor scheduling
ci(checker): add clippy verification
docs(architecture): document module boundaries
```

Pull-request titles follow the same format because the PR title becomes the squash commit subject on `main`.

## Pull Requests

Each pull request must represent one coherent, reviewable outcome. The standard PR template requires explicit statements for:

- architecture impact,
- contract impact,
- database impact,
- security impact,
- testing evidence,
- documentation impact,
- breaking changes.

Write `None` when a section does not apply instead of leaving the impact ambiguous.

All contributor-facing documentation and repository copy are written in English.

## Definition of Done

A change is complete only when all applicable evidence agrees. Depending on the change, that includes:

- implementation,
- automated tests,
- API/contract updates,
- database migrations,
- architecture fitness checks,
- security considerations,
- observability changes,
- documentation changes,
- successful CI evidence.

Compiling successfully is not sufficient evidence on its own.

## Architecture and Planning Gates

Implementation must follow the approved architecture specification and the active implementation plan for the work. Do not bypass documented architecture or approval gates by folding unrelated changes into a convenient PR.

The canonical foundation design is `docs/superpowers/specs/2026-09-17-uptime-lab-foundation-design.md`.
