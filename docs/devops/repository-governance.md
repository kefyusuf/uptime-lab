# Repository Governance

## Purpose

This document defines the canonical Git, pull-request, CI, merge, and repository-protection policy for `uptime-lab`. It complements `CONTRIBUTING.md` with the exact operational settings maintainers must apply on GitHub.

## Branch Model

`main` is the only long-lived branch. All normal development uses short-lived branches and pull requests. There is no permanent `develop` branch.

## Branch Naming

Use the prefixes `feat/`, `fix/`, `refactor/`, `test/`, `docs/`, `ci/`, and `chore/`. Include an issue number when one exists, for example `feat/12-create-monitor`.

## Commit Policy

Commit subjects and pull-request titles follow Conventional Commits. Canonical types and scopes are defined in `CONTRIBUTING.md` and enforced by repository CI scripts.

## Pull Request Policy

A pull request must describe scope and evidence explicitly. Architecture, contract, database, security, testing, documentation, and breaking-change impact are required sections; use `None` when a dimension is not affected.

Pull requests are the normal path into `main` even while there is a single maintainer.

## Merge Policy

Squash merge is canonical. Merge commits and rebase merge are disabled. The PR title becomes the squash commit subject and therefore must pass the same Conventional Commit predicate as branch commits.

Head branches should be deleted automatically after merge.

## Required CI Gate

The stable required status check is:

```text
CI / gate
```

The gate aggregates the policy and repository-validation jobs so branch protection depends on one stable check name while lower-level checks can evolve.

## Main Branch Ruleset

The target GitHub settings are:

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

Required approvals are intentionally `0` while there is one active maintainer. Pull requests and CI remain mandatory, but the repository must not create an impossible self-approval requirement. Increase the human-review requirement when another active maintainer is available.

## GitHub Actions Security

- Workflow permissions default to read-only.
- Write permissions are granted only to jobs that demonstrably require them.
- Actions are pinned to immutable full commit SHAs where practical.
- Checkout credentials are not persisted when CI does not need to push.
- Secrets are not exposed to untrusted fork execution contexts.
- `pull_request_target` is not used for untrusted code execution.

## Dependency Updates

Dependency automation is added only for ecosystems that exist in the repository. The repository baseline starts with GitHub Actions updates; npm, Go modules, Cargo, and Docker updates are introduced with the manifests they own.

## Bootstrap Exception

The empty repository had no Git ref, so the canonical foundation design was allowed to become the initial root commit on `main`. That was the only planned direct-to-main bootstrap exception. All normal implementation changes use branches and pull requests.

## Verification

Before merging repository-governance changes:

1. run the dependency-free governance test harness;
2. validate repository invariants;
3. validate the PR title;
4. confirm `git diff --check` is clean;
5. require a successful `CI / gate` once the gate exists.

Repository-protection settings are activated only after the required check exists and has succeeded on `main`.

## Related Architecture Decisions

The foundation design locks the monorepo model, `main` plus short-lived branches, PR-first workflow, Conventional Commits, squash merge, path-aware CI, a stable aggregate gate, and least-privilege Actions policy. ADRs for material repository-governance changes are introduced by the architecture-documentation baseline rather than retroactively inventing decisions in this task.
