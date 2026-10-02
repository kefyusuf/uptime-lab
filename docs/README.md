# Documentation

Project documentation is organized by responsibility and by cross-runtime architecture.

Current availability is exposed by `GET /monitors/{monitorId}/availability` as a read-only assessment of the latest terminal CheckRun. It returns `status`, `reason`, UTC `evaluatedAt`, and terminal `evidence` (`checkId`, `completedAt`). A known Monitor without a terminal result returns `200 unknown/no_result` with evidence omitted; the raw latest-result route retains its empty `204`. Every matched availability response, including errors and `405`, uses `Cache-Control: no-store`; `HEAD` returns `405` with `Allow: GET`. No new persistence, reconciliation, history, or public deployment is introduced.

## Canonical Design

- Foundation design: [superpowers/specs/2026-09-17-uptime-lab-foundation-design.md](superpowers/specs/2026-09-17-uptime-lab-foundation-design.md)
- Go Monitoring Foundation design: [superpowers/specs/2026-09-20-go-monitoring-foundation-design.md](superpowers/specs/2026-09-20-go-monitoring-foundation-design.md)
- Public Monitoring API Contract design: [superpowers/specs/2026-09-22-public-monitoring-api-contract-design.md](superpowers/specs/2026-09-22-public-monitoring-api-contract-design.md)
- Single-Checker Execution Vertical Slice design: [superpowers/specs/2026-09-26-single-checker-execution-vertical-slice-design.md](superpowers/specs/2026-09-26-single-checker-execution-vertical-slice-design.md)
- Local Web Monitoring Vertical Slice design: [superpowers/specs/2026-10-02-local-web-monitoring-vertical-slice-design.md](superpowers/specs/2026-10-02-local-web-monitoring-vertical-slice-design.md); approved and merged in #49, with runtime implementation still pending.
- Implementation plans: [superpowers/plans/](superpowers/plans/)
- Local Web implementation plan candidate: [superpowers/plans/2026-10-02-local-web-monitoring-vertical-slice.md](superpowers/plans/2026-10-02-local-web-monitoring-vertical-slice.md); reviewed design translated into TDD tasks, awaiting plan approval and execution-method selection.
- Current continuation state: [devops/current-handoff.md](devops/current-handoff.md); verify its recorded base and live PR state before resuming.

Design/spec/plan documents preserve the decision history for the phase in which they were written. Current implementation truth is described by the canonical architecture, backend, checker, testing, and devops documents below.

## Roadmap Proposals

- [Industry benchmark and launch roadmap (2026-10-02)](roadmap/2026-10-02-industry-benchmark-and-launch-roadmap.md): researched product comparison, implementation gaps, proposed milestones, and live-release evidence, refreshed after Current Availability landed. This is a review candidate, not an approved implementation plan or production-readiness claim.
- [Local Web and exposure scope candidate](superpowers/specs/2026-10-02-web-public-exposure-scope-reassessment.md): next-scope recommendation for a local browser journey consuming the four existing public operations; remote release remains a separate decision.

## Implemented Runtime

- Go Control Plane: [backend/go-control-plane.md](backend/go-control-plane.md)
- Rust Checker: [checker/rust-checker.md](checker/rust-checker.md)
- Go Monitoring verification: [testing/go-monitoring-foundation.md](testing/go-monitoring-foundation.md)
- Go public transport verification: [testing/go-public-transport-adapter.md](testing/go-public-transport-adapter.md)
- Public Monitoring contract verification: [testing/public-monitoring-contract.md](testing/public-monitoring-contract.md)
- Single-Checker execution verification: [testing/single-checker-execution-slice.md](testing/single-checker-execution-slice.md)
- Canonical local runtime: [devops/local-development.md](devops/local-development.md)

The Go runtime serves public Monitor create/read, latest terminal result, and current availability, alongside the internal Checker claim/result contract. Go owns due-work scheduling truth, availability policy, and durable `monitoring.check_runs`; Rust owns bounded probe execution and never accesses PostgreSQL. The canonical Docker smoke proves the real Go -> Rust -> `policy_rejected` -> Go -> PostgreSQL -> public latest-result and availability read path.

The latest-result read is terminal-only: pending work is invisible, `204` means a known Monitor has no terminal result yet, and `resultKind` remains an execution fact rather than an up/down verdict. Full CheckRun history, materialized availability history, mutable Monitor lifecycle, React, authentication/authorization, CORS/rate limiting, ingress/TLS, and public network exposure remain deferred.

## Architecture

Start with the [Architecture index](architecture/README.md) for system context, runtime boundaries, dependency rules, data ownership, runtime/change flows, and ADRs.

Architecture documentation describes end-to-end relationships across Web, API, Checker, persistence, security, and operations. Area-specific documentation links back to those cross-area rules rather than redefining ownership.

## Local Development

Use [devops/local-development.md](devops/local-development.md) as the canonical operational guide for Docker Compose, explicit migrations, the real Checker, persistence/reset semantics, worktree isolation, and local verification.

## Area Ownership

- `architecture/`: system-level boundaries and cross-area flows.
- `backend/`: implemented Go Control Plane behavior and boundaries.
- `checker/`: implemented Rust Checker execution architecture and security policy.
- `testing/`: cross-runtime test strategy and implementation evidence.
- `frontend/`: React/TypeScript architecture only when that runtime is implemented.
- `devops/`: repository governance, local development, CI/CD, and release operations.
- `security/`: threat model and security controls when a dedicated document is warranted.
- `operations/`: health, observability, and troubleshooting when a dedicated document is warranted.
- `adr/`: accepted material architecture decisions.
- `superpowers/`: historical design specifications and implementation plans.

Directories are created when they first contain an owned current-state document; historical plans are not rewritten to look current.
