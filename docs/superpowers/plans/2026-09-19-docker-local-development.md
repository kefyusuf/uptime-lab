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
- Readiness is health-based; no arbitrary startup sleeps or central wait script.
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

## Stacked Review Model

```text
main
  └── docs/docker-local-development-design
        └── docs/docker-local-development-plan
              └── devops/docker-local-development
```

- Design PR #9 remains spec-only.
- The plan PR targets `docs/docker-local-development-design`.
- The implementation PR targets `docs/docker-local-development-plan`.
- No implementation starts until this plan is explicitly approved.
- Landing order is design -> plan -> implementation, each retargeted to `main` with fresh CI before squash merge.

---

## Target File Map

```text
compose.yaml
.env.example

deploy/docker/
└── placeholder/
    ├── Dockerfile
    └── entrypoint.sh

scripts/ci/
├── detect-local-dev-changes.sh
├── test-detect-local-dev-changes.sh
├── check-local-dev.sh
├── test-check-local-dev.sh
├── smoke-local-dev.sh
└── test-smoke-local-dev.sh

docs/devops/
└── local-development.md

.github/workflows/ci.yml
README.md
docs/README.md
```

No file under `apps