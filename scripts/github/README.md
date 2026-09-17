# GitHub Governance Bootstrap

This directory contains the one-time administrative bootstrap for repository governance.

## Purpose

`configure-governance.sh` closes the gap between repository-owned policy and GitHub-hosted repository settings. It configures and then verifies:

- squash merge enabled;
- merge commits disabled;
- rebase merge disabled;
- automatic deletion of merged head branches enabled;
- an active `main-protection` branch ruleset targeting `refs/heads/main`;
- pull requests required with zero mandatory approvals while there is one active maintainer;
- review-thread resolution required;
- squash as the only merge method allowed by the ruleset;
- `CI / gate` required and branches required to be up to date;
- linear history required;
- force pushes blocked;
- branch deletion blocked;
- no bypass actors.

## Requirements

- GitHub CLI (`gh`).
- An authenticated GitHub account or token with repository **Administration: write** permission for `kefyusuf/uptime-lab`.

The ChatGPT GitHub connector intentionally does not expose repository-administration writes, so this script exists to make the remaining privileged operation deterministic, reviewable, and repeatable.

## Run

```bash
./scripts/github/configure-governance.sh
```

The script is idempotent. If `main-protection` already exists, it updates that ruleset; otherwise it creates it. It exits non-zero when the authenticated account lacks admin access, when duplicate rulesets with the same name exist, or when post-write verification differs from the canonical policy.

## Test

```bash
./scripts/github/test-configure-governance.sh
```

The test uses a fake `gh` executable and validates both creation and update paths without modifying GitHub.
