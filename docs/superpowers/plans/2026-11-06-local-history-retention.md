# Local History Retention Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Bound the retained terminal evidence per Monitor: the claim transaction removes terminal CheckRuns beyond the newest 50 per Monitor, so the durable window never grows past one full history page.

**Architecture:** One retention statement inside the claim transaction's existing reconciliation phase (global, window-function ranked, pending-immune). No new operation, schema, fixture, migration, configuration or browser change; Rust and the internal contract are unchanged.

**Tech Stack:** Existing Go1.27.3/pgx/goose, PostgreSQL18, OpenAPI3.1.2, React/TypeScript/Vitest/Playwright, Node24.21.0/npm11.19.1. No new dependencies, no new migration.

**Spec:** [Local History Retention design](../specs/2026-11-06-local-history-retention-design.md), written at `07a98c8` stacking the [scope](../specs/2026-11-06-local-history-retention-scope-reassessment.md); both merged through PR #76 (`main@bda980f`). The user's approved continuation after the scope authorized preparing this plan.

**Status:** Plan awaiting review; no product task is executed. Preserve the prior Native execution method unless the user selects another method: this session implements the dependent tasks, then a fresh whole-branch reviewer checks the completed change. Plan approval is distinct from merge/deployment approval.

## Global Constraints

- Retention keeps the newest 50 terminal rows per Monitor (ordered `completed_at DESC, id DESC`) and removes terminal rows ranking above the cap; pending rows (`completed_at IS NULL`) are never ranked and never removed.
- The cap (50) is a repository-owned constant in the claim adapter, asserted by the checker's contract description and documented in the history operation description; it is not configurable at runtime and is not an R0-07 decision.
- Enforcement lives only inside the claim transaction's reconciliation phase, after the expired-pending reconciliation and before the candidate selection; no background reconciler, scheduler, second deletion path or reverse lock path is introduced.
- The history read, latest-result read, availability assessment, pause/resume, and Monitor deletion semantics are unchanged; the latest terminal row of a Monitor is never removed (rank 1).
- No new migration; the readiness set stays at four. Rust and `contracts/openapi/internal.yaml` unchanged. No existing public schema changes; only the history operation description gains the retained-window sentence and the generated types regenerate without structural change.
- The browser needs no changes: the history section, deletion control and scheduling control already tolerate rows appearing and disappearing between reads.
- Scope excludes age-based expiry, configurable caps, target-URL edit, undelete, aggregation/uptime math, incidents, notifications, ownership, remote access, quotas and multi-Checker redesign. English code/docs/copy; concise Turkish before/after updates.
- Record focused RED reason, GREEN result and commit per task. Tool/setup failure is blocked verification, not product RED. Preserve user-owned changes; explicit paths only in commits. Allowed scopes: web, api, checker, contracts, devops, architecture, docs, deps.

## Review Focus

- A Monitor with 55 terminal rows converges to exactly the newest 50 after one claim, and rows 51–55 are gone: Task 2.
- Pending-row immunity: a pending run never ranks and never disappears, including a just-expired pending row terminalized by the same reconciliation immediately before retention: Task 2.
- Per-Monitor partitioning: a second Monitor with few rows keeps every row while an over-cap Monitor converges; a global (unpartitioned) cap must be detectable: Task 2 mutation.
- In-flight completion race: a completion holding its CheckRun row lock while retention attempts removal resolves without deadlock and without losing the completion: Task 2 with a parked lock and pg_locks observation.
- The newest terminal row is never removed (rank 1), so latest-result and availability semantics are untouched: Task 2.
- The contract description states the retained window and the generated types stay structurally identical: Task 1.

## File map and execution prerequisites

Go module paths in tasks are relative to `apps/api/internal/modules/monitoring`; the pinned Go is 1.27.3 (`golang:1.27.3-alpine3.24` in `scripts/ci/run-go-postgres-tests.sh`). Real PostgreSQL uses `bash scripts/ci/run-go-postgres-tests.sh`; it owns its uniquely named stack. Windows execution uses per-invocation `MSYS_NO_PATHCONV=1` for the go-test container. The retention statement is a DELETE with variable args in an existing committed adapter file (`check_execution.go`) — the Mimosa pre-write scan blocked such formulations in NEW files during the deletion landing, but edits to the already-baselined file passed; if the edit is blocked, apply the deletion plan's pipelined-batch ruling.

Before runtime execution, verify clean/dirty state, applicable instructions, tools and actual Docker access. Keep the eventual feature PR stacked on the approved docs baseline; never merge a documentation or feature PR without approval.

No worktree or Docker resource is needed for writing this plan.

### Task 1: Contract description and generated types

**Files:** Modify `contracts/openapi/public.yaml` (history operation description only), regenerate `apps/web/src/shared/api/public.generated.ts`.

**Interfaces:** The history operation description gains one sentence: the retained window keeps the newest 50 terminal rows per Monitor, so older terminal rows disappear over time and cursors anchored on removed rows continue with strictly older rows or an empty page. No schema, status, fixture or structural change.

- [ ] Update the history operation description; run `node scripts/ci/test-check-public-contract.mjs` (must stay 125/125) and `node scripts/ci/check-public-contract.mjs <bundle> .` against a fresh redocly bundle.
- [ ] Run Web `npm run generate:api`, `npm run check:generated`, `npm run typecheck`.
- [ ] Commit explicit contract/generated files: `feat(contracts): document retained history window`.

### Task 2: Retention statement and real-PostgreSQL evidence

**Files:** Modify `adapters/postgres/check_execution.go` (retention statement in the claim transaction), create `monitor_history_retention_integration_test.go`; no other adapter changes.

**Interfaces:** One statement after the expired-pending reconciliation: `DELETE FROM monitoring.check_runs WHERE completed_at IS NOT NULL AND id IN (SELECT id FROM (SELECT id, row_number() OVER (PARTITION BY monitor_id ORDER BY completed_at DESC, id DESC) AS rank FROM monitoring.check_runs WHERE completed_at IS NOT NULL) ranked WHERE rank > 50)` with the cap constant (50) named and asserted. The statement runs before the candidate selection in the same Read Committed transaction.

- [ ] Write real integration `TestRetentionConvergesToFiftyNewest`, `TestRetentionPreservesPendingRows`, `TestRetentionPartitionedByMonitor`, `TestRetentionNeverRemovesLatestTerminalRow`, `TestRetentionTieBreaksEqualCompletionInstants`, `TestRetentionRacingCompletionResolvesWithoutDeadlock`: a Monitor with 55 terminal rows converges to exactly the newest 50 after one claim (rows 51–55 removed by rank, ties ranked by check id descending); a pending run survives including a just-expired pending row terminalized by the same reconciliation; a second Monitor with two rows keeps both; the latest terminal row survives; a completion holding its CheckRun row lock (test-parked transaction plus pg_locks observation) resolves without deadlock and without losing the completion. Coordinated transactions and bounded synchronization; no arbitrary sleeps as race proof.
- [ ] Run root `bash scripts/ci/run-go-postgres-tests.sh` on the actual RED head; require test failure for absent retention behavior, not setup/compile-only evidence — mutation checks (dropping the `rank > 50` filter, removing the pending immunity, dropping `PARTITION BY monitor_id`) must each reproduce specific failures before GREEN is recorded.
- [ ] Implement the statement; no claim, completion or migration SQL changes beyond the one retention statement.
- [ ] Repeat real SQL GREEN plus the existing claim/completion/pause-resume/deletion/history integration suites; run Go unit/vet and required Linux race checks. No query performance claim from this suite.
- [ ] Commit explicit files: `feat(api): enforce history retention window`.

### Task 3: Regression evidence, docs and final review

**Files:** Update `docs/backend/go-control-plane.md`, `docs/testing/single-checker-execution-slice.md`, `docs/devops/current-handoff.md`, `docs/README.md` as necessary.

**Interfaces:** Documentation records the retained window as implemented truth: the cap, the enforcement point, the pending immunity and the interaction rules (in-flight completion, anchored cursors, latest-row safety).

- [ ] Run the full affected local checks (Go unit/vet, real-PostgreSQL suites, Web suite, contract checks); run actual GREEN on the canonical local smoke.
- [ ] Update canonical docs to implemented truth; keep historical specs/plans as decisions.
- [ ] Commit acceptance/docs: `test(api): qualify history retention`.
- [ ] Request one fresh whole-branch reviewer through the preserved Native workflow, with no inherited implementation history. Review all tasks together, emphasizing Review Focus. Fix material findings with RED/GREEN in the same feature branch.
- [ ] Require final exact-head affected checks plus aggregate gate; finalize the feature PR, summarize before/after and seek merge approval. After an authorized merge, verify exact-main CI and record durable handoff/activity evidence. No deploy.

## Self-review and execution boundary

Design sections 1–2 map to Task 2; section 3 to Task 2; section 4 to Task 2; section 5 to Task 1; section 6 to Tasks 2–3. Each Review Focus input has an owning test. No task changes Rust, the internal contract, or any migration; required regression gates still run when repository rules demand them.

Review this plan before execution. Preserve Native from the prior accepted workflow unless explicitly changed. No product code, migration, Docker stack or paid/external service is created by plan preparation. Missing actual retention evidence is an open acceptance item, never a fabricated pass.
