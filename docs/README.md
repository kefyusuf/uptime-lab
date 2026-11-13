# Documentation

Project documentation is organized by responsibility and by cross-runtime architecture.

Current availability is exposed by `GET /monitors/{monitorId}/availability` as a read-only assessment of the latest terminal CheckRun. It returns `status`, `reason`, UTC `evaluatedAt`, and terminal `evidence` (`checkId`, `completedAt`). A known Monitor without a terminal result returns `200 unknown/no_result` with evidence omitted; the raw latest-result route retains its empty `204`. Every matched availability response, including errors and `405`, uses `Cache-Control: no-store`; `HEAD` returns `405` with `Allow: GET`. No new persistence, reconciliation, history, or public deployment is introduced.

## Canonical Design

- Foundation design: [superpowers/specs/2026-09-17-uptime-lab-foundation-design.md](superpowers/specs/2026-09-17-uptime-lab-foundation-design.md)
- Go Monitoring Foundation design: [superpowers/specs/2026-09-20-go-monitoring-foundation-design.md](superpowers/specs/2026-09-20-go-monitoring-foundation-design.md)
- Public Monitoring API Contract design: [superpowers/specs/2026-09-22-public-monitoring-api-contract-design.md](superpowers/specs/2026-09-22-public-monitoring-api-contract-design.md)
- Single-Checker Execution Vertical Slice design: [superpowers/specs/2026-09-26-single-checker-execution-vertical-slice-design.md](superpowers/specs/2026-09-26-single-checker-execution-vertical-slice-design.md)
- Local Web Monitoring Vertical Slice design: [superpowers/specs/2026-10-02-local-web-monitoring-vertical-slice-design.md](superpowers/specs/2026-10-02-local-web-monitoring-vertical-slice-design.md); approved in #49, implemented in #51, with permanent inbound deadline coverage in #52.
- Implementation plans: [superpowers/plans/](superpowers/plans/)
- Completed Local Web implementation plan: [superpowers/plans/2026-10-02-local-web-monitoring-vertical-slice.md](superpowers/plans/2026-10-02-local-web-monitoring-vertical-slice.md); historical TDD plan executed through #51; current evidence is in the Web verification guide and handoff.
- Approved Local Monitor Inventory design: [superpowers/specs/2026-10-05-local-monitor-inventory-design.md](superpowers/specs/2026-10-05-local-monitor-inventory-design.md); approved and merged in #54, including oversized-item traversal limitation.
- Completed Local Monitor Inventory implementation plan: [superpowers/plans/2026-10-05-local-monitor-inventory.md](superpowers/plans/2026-10-05-local-monitor-inventory.md); merged in #55, executed through Native TDD and landed in [PR56](https://github.com/kefyusuf/uptime-lab/pull/56). Final evidence is recorded in the handoff.
- Current continuation state: [devops/current-handoff.md](devops/current-handoff.md); verified #59/#60 landed, with R0 release decisions unresolved. Always revalidate live `main` before new work.

Design/spec/plan documents preserve the decision history for the phase in which they were written. Current implementation truth is described by the canonical architecture, backend, checker, testing, and devops documents below.

## Roadmap Proposals

- [Industry benchmark and launch roadmap (2026-10-02)](roadmap/2026-10-02-industry-benchmark-and-launch-roadmap.md): researched product comparison, implementation gaps, proposed milestones, and live-release gates, refreshed after Local Web landed. Later phases remain proposals, not approved implementation plans or production-readiness claims.
- [R0 product and release decision brief (2026-10-18)](roadmap/2026-10-18-r0-product-release-decision-brief.md): proposed decision worksheet with unapproved audience, workload, operator, data/recovery and release gates; no production or feature authorization.
- [Local Web and exposure scope](superpowers/specs/2026-10-02-web-public-exposure-scope-reassessment.md): historical selection of the implemented local browser journey; remote release remains a separate decision.
- [Local Monitor Inventory scope](superpowers/specs/2026-10-04-local-monitor-inventory-scope-reassessment.md): approved first R2 increment through #53, implemented on PR56 after design54/plan55 approval. Remaining lifecycle/history work and remote release gates are separate.
- [Local Monitor Pause/Resume scope](superpowers/specs/2026-10-08-local-monitor-pause-resume-scope-reassessment.md): historical R2 scope approved and implemented, with [feature PR #60](https://github.com/kefyusuf/uptime-lab/pull/60) squash-merged. See the current handoff for landed SHA, CI and remaining release gates.
- [Local Monitor Pause/Resume design](superpowers/specs/2026-10-08-local-monitor-pause-resume-design.md): transaction serialization, strict scheduling contract, gateway mutation guards and browser uncertainty rules; subsequent continuation authorized the plan.
- [Local Monitor Pause/Resume implementation plan](superpowers/plans/2026-10-08-local-monitor-pause-resume.md): historical seven-task Native TDD plan executed through merged PR #60, independently reviewed and exact-main CI green. A production deployment or migration was not performed.
- [Local Monitor Delete scope](superpowers/specs/2026-10-27-local-monitor-delete-scope-reassessment.md): historical R2 scope proposal after the user selected validation path A; durable local hard delete of a Monitor and its CheckRun evidence, approved and implemented through feature PR #69.
- [Local Monitor Delete design](superpowers/specs/2026-10-27-local-monitor-delete-design.md): historical design for the landed pre-sweep/lock/re-sweep deletion transaction, serialization outcomes, eighth public operation with empty 204, gateway DELETE mutation guards and browser confirmation rules; its deleted-run completion status was corrected to the landed 404 not-found mapping during execution.
- [Local Monitor Delete implementation plan](superpowers/plans/2026-10-27-local-monitor-delete.md): historical seven-task Native TDD plan executed through feature PR #69; the delete-side tracer gate was realized as a claim-side gate plus pg_locks observation, recorded in the feature PR. Remaining finish gates: fresh whole-branch review and final CI.
- [Local Check-Run History scope](superpowers/specs/2026-11-01-local-check-run-history-scope-reassessment.md): historical R2 scope proposal approved and implemented through feature PR #73; bounded terminal-only keyset-paginated check-run history read on the detail page, mirroring inventory cursor/budget conventions.
- [Local Check-Run History design](superpowers/specs/2026-11-01-local-check-run-history-design.md): historical design for the landed version-2 keyset cursor, existence-probe-plus-page Read Committed transaction over the existing terminal index, ninth public operation with inventory-mirrored query grammar and budget, and a detail-page history section.
- [Local Check-Run History implementation plan](superpowers/plans/2026-11-01-local-check-run-history.md): historical seven-task Native TDD plan executed through feature PR #73, squash-merged to `main@6c49b2a` with all-ten-job exact-main CI.
- [Populated-Data EXPLAIN Evidence scope](superpowers/specs/2026-11-10-populated-explain-scope-reassessment.md): historical evidence-only scope proposal executed through the explain feature branch; records and pins query-plan evidence for the exercised queries over the retention-bounded dataset with explicit non-qualification statements. Evidence document: [populated-explain-evidence](testing/populated-explain-evidence.md).
- [Local Target-URL Edit scope](superpowers/specs/2026-11-10-local-target-edit-scope-reassessment.md): historical R2 scope proposal approved and merged through PR #84; target-URL edit on the immutable Monitor identity (tenth public operation, PATCH with a closed targetUrl body) with in-flight old-target semantics surfaced explicitly.
- [Local Target-URL Edit design](superpowers/specs/2026-11-10-local-target-edit-design.md): historical design for the landed identity-immutable target transformation (WithTargetURL), tenth public operation PATCH with a closed targetUrl body, in-flight old-target semantics and edit-vs-claim/delete/pause serialization; merged through PR #85.
- [Local Target-URL Edit implementation plan](superpowers/plans/2026-11-10-local-target-edit.md): historical seven-task Native TDD plan (contract, domain/use case, durable adapter, HTTP, gateway, browser controls, journey evidence) merged through PR #86, executed through feature [PR #87](https://github.com/kefyusuf/uptime-lab/pull/87) squash-merged to `main@04a6c554` with all-ten-job exact-main CI.
- [Local History Retention scope](superpowers/specs/2026-11-06-local-history-retention-scope-reassessment.md): historical R2 scope proposal approved and implemented through feature PR #78; count-based retained window (newest 50 terminal rows per Monitor) enforced inside the existing claim reconciliation.
- [Local History Retention design](superpowers/specs/2026-11-06-local-history-retention-design.md): historical design for the landed window-function retention statement inside the claim transaction's reconciliation phase, pending-row immunity, in-flight-completion race resolution and the explicit R0-07 non-decision.
- [Local History Retention implementation plan](superpowers/plans/2026-11-06-local-history-retention.md): historical three-task Native TDD plan executed through feature PR #78, squash-merged to `main@352ee83` with all-ten-job exact-main CI.
- [Local Check-Run History implementation plan](superpowers/plans/2026-11-01-local-check-run-history.md): historical seven-task Native TDD plan executed through feature PR #73, squash-merged to `main@6c49b2a` with all-ten-job exact-main CI.

- [Post-R2 roadmap (2026-11-13)](roadmap/2026-11-13-post-r2-roadmap.md): proposed effort-ordered tracks after R2 completion — housekeeping, minimal Path-A R0 decisions, portfolio readiness, then confirmation/recovery; R4/R5 remain deferred and the external-beta NO-GO judgment is unchanged. Proposal only; no implementation or release authorization.

## Implemented Runtime

- Local Monitoring Web: [frontend/local-monitoring-web.md](frontend/local-monitoring-web.md)
- Web verification: [testing/local-monitoring-web.md](testing/local-monitoring-web.md)

- Go Control Plane: [backend/go-control-plane.md](backend/go-control-plane.md)
- Rust Checker: [checker/rust-checker.md](checker/rust-checker.md)
- Go Monitoring verification: [testing/go-monitoring-foundation.md](testing/go-monitoring-foundation.md)
- Go public transport verification: [testing/go-public-transport-adapter.md](testing/go-public-transport-adapter.md)
- Public Monitoring contract verification: [testing/public-monitoring-contract.md](testing/public-monitoring-contract.md)
- Single-Checker execution verification: [testing/single-checker-execution-slice.md](testing/single-checker-execution-slice.md)
- Canonical local runtime: [devops/local-development.md](devops/local-development.md)

The Go runtime serves public Monitor create/read, bounded inventory, latest terminal result, current availability, and local scheduling GET/PUT, alongside the internal Checker claim/result contract. Go owns due-work scheduling truth, availability policy, and durable `monitoring.check_runs`; Rust owns bounded probe execution and never accesses PostgreSQL. The canonical Docker smoke proves the real Go -> Rust -> `policy_rejected` -> Go -> PostgreSQL -> public latest-result and availability read path.

The latest-result read is terminal-only: pending work is invisible, `204` means a known Monitor has no terminal result yet, and `resultKind` remains an execution fact rather than an up/down verdict. React now displays local snapshots through its restricted gateway. Full CheckRun history, materialized availability history, mutable Monitor lifecycle, authentication/authorization, ingress/TLS and remote network exposure remain deferred.

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
