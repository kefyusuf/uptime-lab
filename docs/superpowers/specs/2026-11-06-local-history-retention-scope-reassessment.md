# Local History Retention Scope Reassessment

**Date:** 2026-11-06
**Status:** Proposed scope and design direction; awaiting user review
**Verified baseline:** `main@56a56c6f`, after #59/#60/#64/#65/#68/#69/#70/#71/#72/#73/#74
**Authorization:** Documentation only; no runtime implementation, migration, merge or deployment.

## Purpose and before/after

A local operator's terminal evidence is now visible through the bounded history read, but it grows without bound: every claimed check adds one terminal row, and nothing ever removes rows for a live Monitor. Over weeks of local evaluation the retained window grows monotonically, and the bounded history page only bounds what is *shown*, not what is *kept*.

The next proposed R2 increment adds a durable **count-based retained window**: Go removes terminal rows beyond a fixed per-Monitor cap during the existing claim reconciliation, so the retained evidence stays bounded without introducing a background reconciler, a scheduler, or any configuration surface.

**R0 boundary:** the nine R0 decisions remain OPEN, and age-based or deployment-grade retention (time windows, sensitive-URL handling, legal holds) is precisely the R0-07 territory that must stay decided-by-the-user. This increment therefore proposes a **count-based cap** as a local data-hygiene rule with a fixed repository-owned value — it does not decide R0-07, does not add configuration, and does not alter any read semantics. If R0-07 later selects a different policy, this mechanism is replaced or removed by its own scoped increment.

## Alternatives

| Approach | Benefit | Cost |
|---|---|---|
| Count-based cap enforced in the existing claim reconciliation (recommended) | Bounded storage with zero new infrastructure; reuses the Go-owned claim transaction that already reconciles expired pending runs; no configuration | The cap value is fixed in code (local convention, not a policy decision); rows are removed silently from the history read's window |
| Age-based expiry (rows older than N days) | Matches the intuitive retention idea | Requires a decided time window — that is R0-07's open decision; also clocks vs paused Monitors create surprising semantics |
| Dedicated background reconciler | Decoupled from claims | A stopped Checker already does not gain an independent reconciler by design; a second deletion path multiplies the audit surface |
| Defer until R0-07 | No premature decisions | Unbounded growth continues throughout the entire local evaluation period that path A exists to support |

**Draft assumption requiring approval:** use the first approach with a fixed cap of **50 terminal rows per Monitor** (mirroring the history page maximum — the retained window equals one full history page). If a different cap or a policy-shaped design is wanted, revise scope before detailed design.

## Proposed rules

1. The cap keeps the **newest 50 terminal rows per Monitor** and permanently removes older ones (regardless of result kind); pending rows are never touched by retention.
2. Enforcement lives inside the existing claim transaction, immediately after the expired-pending reconciliation and before the candidate selection, in the same Read Committed transaction: one `DELETE` removing terminal rows outside the newest-50 window for every Monitor touched by the claim — reusing the Go-owned path so no second deletion mechanism, background job, or configuration surface exists.
3. The history read, latest-result read, availability assessment, pause/resume and deletion semantics are unchanged: retention removes rows that the bounded history read may or may not have shown, and a cursor anchored on a row that retention later removed yields strictly older rows or an empty continuation under the existing keyset predicate (no removed-anchor error exists in this design, matching the history design's corrected semantics).
4. The cap value (50) is a repository-owned constant asserted by the checker and documented in the contract descriptions; it is not configurable at runtime and is not an R0-07 decision.
5. Deletion of a Monitor still removes all of its rows atomically (retention changes nothing about that path); pause/resume and scheduling semantics are untouched; no new result kinds, no verdict or uptime math.
6. No new migration: the removal statement is served by the existing indexes; the readiness migration set stays at four. Rust and the internal Checker contract remain unchanged.

## Architecture and contract direction

Go owns the retention removal exactly as it owns the expired-pending reconciliation: same transaction, same adapter, same ownership rules. The claim adapter gains one statement after the expired-pending reconciliation; its correctness conditions (ordering, per-Monitor window, pending-row immunity) are pinned by real-PostgreSQL tests with the same rigor as the deletion and history suites. The public contract gains **no new operation** — retention changes only what the existing history read returns over time, so the contract's history description gains an explicit retained-window sentence and the generated types regenerate without structural change.

No new fixtures, schemas or generated-type shapes are required beyond description text. The bounded history section in the browser needs no changes: it already renders whatever the durable window contains.

## Browser direction

No browser changes. The history section, deletion control and scheduling control already tolerate rows appearing and disappearing between reads (positions, not snapshots), and the retained window only changes which rows exist over time. No new section, control or copy is introduced.

## Acceptance evidence required later

- Real PostgreSQL RED/GREEN: a Monitor with more than 50 terminal rows converges to exactly 50 after a claim (newest kept), pending rows survive retention, other Monitors are untouched, deletion still removes everything, and the history read over a retained window pages correctly at the 20/50 boundaries.
- Mutation checks proving each condition is load-bearing (dropping the per-Monitor window, the pending immunity or the newest-first ordering must reproduce specific failures).
- Claim-path regression evidence: existing claim/completion/pause-resume/deletion suites remain green, including the deletion-race and history-race proofs.
- HTTP/gateway regression evidence: the history read's grammar, budget and status behavior unchanged.
- Required repository, Go/race/real-PostgreSQL, Rust regression/audit, Web, Docker/browser and aggregate CI at the final implementation head; exact-main evidence after an authorized merge. The populated-data EXPLAIN evidence remains a separate capacity-evidence minor, now with the retained window making the populated dataset naturally bounded.

## Exclusions and next review

No age-based expiry, no configurable cap, no target-URL edit, no undelete, no aggregation or uptime math, no scheduled maintenance, incidents, notifications, ownership/accounts, audit trail, remote access, quotas or multi-Checker redesign. R0-07 stays OPEN — this increment implements a local data-hygiene mechanism, not the deployment retention policy. The nine R0 decisions remain OPEN.

Review the count-based cap value, the claim-transaction enforcement point, the pending-row immunity and the explicit non-decision on R0-07. Scope approval precedes detailed design; written design approval precedes an implementation plan. Product execution then follows TDD -> branch/commit -> PR. Merging this document records a proposal unless product decisions are separately approved.

## Evidence pointers

- [Current handoff](../../devops/current-handoff.md), [R0 decision brief](../../roadmap/2026-10-18-r0-product-release-decision-brief.md), [launch roadmap](../../roadmap/2026-10-02-industry-benchmark-and-launch-roadmap.md).
- `apps/api/internal/modules/monitoring/adapters/postgres/check_execution.go`: the claim transaction's existing reconciliation phase.
- `apps/api/migrations/00002_create_monitoring_check_runs.sql`: durable row shape and indexes.
- `docs/superpowers/specs/2026-11-01-local-check-run-history-design.md`: the history read's corrected removed-anchor semantics.
