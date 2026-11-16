# R3-2 Durable Incident Semantics Scope Reassessment

**Date:** 2026-11-16
**Status:** Proposed scope and design direction; awaiting user review
**Verified baseline:** `main@92c4671`, after R3-1 landed (#98) and its handoff reconciliation (#99)
**Authorization:** Documentation only; no runtime implementation, migration, merge or deployment.

## Purpose and before/after

R3-1 landed assessment-only confirmation/recovery: `EvaluateAvailability` derives a newest-first streak from retained terminal `check_runs` at read time (N=3 consecutive target-failures confirm `unavailable`, M=2 consecutive target-successes recover to `available`), with no incident table, no counters and no new write path. That assessment is bounded twice over — by the newest-50 retention window and by the five-row sequence limit — and it leaves no durable record that an outage was ever confirmed. A confirmed outage that later recovers is indistinguishable, after the fact, from one that never happened; incident history does not exist.

The proposed increment materializes confirmed outages into **durable incident open/resolve state**, defined separately from the current availability read (R0 brief: "R3 must define incident semantics separately from the existing current availability read"). A confirmed outage opens a durable incident; a confirmed recovery resolves it; the open/resolve history persists independent of CheckRun evidence retention and across API restarts. A new bounded public read exposes incident history, and the detail page renders it. The slice is the mandated foundation for incident history and for R4 notification, and it lifts the retention-bounded confirmation limit by keeping durable per-Monitor confirmation state rather than re-deriving from a truncated window.

The availability read (R3-1) is **unchanged**: it stays the evidence-derived assessment. Incident state is a separate durable projection driven by the same confirmation/recovery transition semantics. Notification delivery is **not** in this slice (R4, deferred; R0-04 records no alert-delivery guarantee and R0-09 defers the first channel).

## The design tension (explicit)

Where does durable incident state live, and how is it maintained?

1. **Durable per-Monitor confirmation counters + incident open/resolve rows, maintained on the write path** (recommended). A new migration adds incident state; each terminal completion updates durable consecutive-failure/success counters and opens an incident at N, resolves it at M. Confirmation becomes unbounded (counters persist regardless of evidence retention) and incident history is durable. Cost: the first new migration and completion-side write path since R2 retention, plus per-Monitor serialization on completion.
2. **Re-derive incident transitions from retained evidence at write time, persist only the resulting incident rows.** No counters; the transition is recomputed from the retained window when a check completes. Lighter schema, but confirmation stays retention-bounded (the R3-1 limit is not lifted) and a truncated window can miss a transition.
3. **Fold incident state into the availability read.** Rejected by R0: incident semantics must be separate from the current availability read.

**Draft assumption requiring approval:** approach 1. The exact hook (completion path vs claim-transaction reconciliation), the state shape (counters as columns on `monitors` vs a separate confirmation-state table vs incident rows alone), and the coherence rule between the evidence-derived availability assessment and the counter-driven incident state under retention truncation/restart are **design decisions**, not settled here.

## Key semantics questions this scope must answer

- **Transition semantics reuse R3-1.** Success = `http_response` 2xx; failure = `http_response` non-2xx plus `dns_error`/`timeout`/`connect_error`/`tls_error`/`protocol_error`; inconclusive = `policy_rejected`/`internal_error`/`worker_timeout`. N=3 consecutive failures open an incident; M=2 consecutive successes resolve an open incident; an inconclusive result resets the streak (conservative, mirroring R3-1). N/M stay fixed repository-owned local conventions (R0-03), not per-Monitor configurable policy and not an admitted workload cap.
- **One open incident per Monitor.** A Monitor has at most one open incident at a time; a confirmed outage while an incident is already open does not open a second. Draft: enforce with a partial unique index mirroring `check_runs_one_pending_per_monitor_idx`. Whether a re-confirm after recovery opens a new incident row (history of distinct outages) is the intended behavior; the exact invariant is a design decision.
- **Write-path hook and serialization.** Incident state is per-Monitor. `CompleteCheck` currently locks only the CheckRun row (`FOR UPDATE`) under default isolation and does not reconcile; `ClaimDueCheck` runs explicit Read Committed reconciliation plus retention. Maintaining incident state on completion needs per-Monitor serialization (lock the Monitor or the open-incident row) so concurrent completion/claim/edit/delete/pause cannot double-open or lose a resolve. The hook choice and locking order are design decisions with real-PostgreSQL race evidence.
- **Unbounded confirmation vs retention.** Durable counters let an incident stay open and a streak accumulate even after older terminal evidence is retained-out. The availability read remains evidence-derived and bounded; under truncation or a long pause the two can diverge (e.g. availability `awaiting_confirmation`/`stale_result` while an incident is durably open). This divergence is a documented consequence of the R0-mandated separation; the design must state the coherence rule and the documented limitation, not silently couple them.
- **No historical backfill.** Incidents accrue from the migration forward; existing retained evidence is not replayed into incidents. A Monitor with no incident state starts with zeroed counters. Documented behavior, not a data migration.
- **Pause/edit/delete interaction.** Pause freezes new claims (no new completions, so incident state is frozen); a prolonged pause lets availability staleness dominate while an open incident persists. Delete removes the Monitor and its incidents (cascade, mirroring CheckRun evidence deletion). A target edit may mix targets within an incident's lifetime (documented limitation mirroring history/streak mixing); whether an edit resolves or annotates an open incident is a design decision.
- **Public read surface.** One new bounded read operation, `GET /monitors/{monitorId}/incidents`, mirroring the check-run history conventions: newest-first by open instant then incident UUID, default 20 / max 50 rows, a new versioned keyset cursor (distinct version prefix rejecting cross-resource cursors), 128-byte raw query budget, 245760-byte compact JSON budget, `no-store`, `HEAD` 405 with `Allow: GET`, 400 invalid query/cursor, 404 unknown Monitor, 500 sanitized persistence failure; empty history is a successful empty page. The current open incident is the newest item with no resolve instant. The exact incident item field set (open/resolve instants, triggering CheckRun evidence ids, opening reason, state) is a design decision.
- **Contract and runtime boundary.** One new public operation and its schemas/cursor; the availability contract and all ten existing operations are unchanged. No new result kinds; Rust and the internal Checker contract are unchanged (incidents are Go-owned, derived from completion facts the Checker already submits). The readiness set stays at four. One new migration (00005); 00001–00004 remain immutable.

## Alternatives

| Approach | Benefit | Cost |
|---|---|---|
| Durable counters + incident rows on the write path (recommended) | Unbounded confirmation; durable incident history across restart/retention; foundation for R4 | First new migration + completion-side write path since R2; per-Monitor serialization; availability/incident divergence to document |
| Re-derive transitions from retained evidence, persist incident rows only | Lighter schema; no counters | Confirmation stays retention-bounded (R3-1 limit not lifted); truncated window can miss a transition |
| Fold incident state into the availability read | Single source of truth | Rejected by R0: incident semantics must be separate from the current availability read |

## Proposed rules

1. Confirmed outages are materialized into durable incident open/resolve state, separate from the unchanged evidence-derived availability read; a confirmed recovery resolves the open incident.
2. Transition semantics reuse R3-1 exactly: success/failure/inconclusive classification, N=3 confirm, M=2 recover, inconclusive reset; N/M remain fixed local conventions (R0-03), <= the retention window and not per-Monitor policy.
3. Durable per-Monitor confirmation state is maintained on the write path so confirmation is unbounded by evidence retention and survives restart; the exact hook, state shape and serialization are design decisions qualified by real-PostgreSQL race evidence.
4. At most one open incident per Monitor; distinct outages accrue as separate incident history rows; no historical backfill from existing evidence.
5. One new bounded public read `GET /monitors/{monitorId}/incidents` mirrors the check-run history cursor/budget/`no-store`/`HEAD`/error conventions with a new versioned cursor; the availability contract and existing operations are unchanged.
6. The detail page renders current incident state and bounded incident history through the restricted gateway (read-only; no new mutation controls).
7. Delete cascades incidents; pause freezes incident state; a target edit may mix targets within an incident lifetime (documented limitation). The availability assessment and incident state may diverge under retention truncation/long pause; the design documents the coherence rule and limitation.
8. One new migration (00005) adds incident state; 00001–00004 stay immutable. No new result kinds; Rust and the internal contract unchanged; readiness set stays at four. No notification delivery, uptime percentage, maintenance window, per-Monitor configurable threshold, aggregation, ownership or remote access.

## Acceptance evidence required later

- Migration RED/GREEN: 00005 adds the incident state and indexes (including the one-open-incident invariant); immutable migration history preserved; goose up/down verified.
- Contract RED/GREEN: the new incident operation, schemas and versioned cursor; unchanged availability and existing operations; generated-type parity; query/byte-budget and `no-store`/`Allow`/`HEAD` rules.
- Domain RED/GREEN: incident transition policy (open at N, resolve at M, inconclusive reset, one-open invariant, counter semantics) with mutation checks.
- Real PostgreSQL RED/GREEN: incident open/resolve on completion; durability across API restart; unbounded confirmation surviving retention truncation; per-Monitor serialization under concurrent completion/claim/edit/delete/pause (parked-lock race proofs); one-open-incident invariant; delete cascade; no backfill; absence of any change to the availability read path.
- HTTP/gateway RED/GREEN: the incident read serialized through the gateway, budget/cursor rejection, sanitized errors, unchanged `no-store`/`Allow`/`HEAD`.
- Web tests: current incident and history render distinctly; refresh recomputes; no new mutation controls.
- Composed Chromium on 4173/4817: sustained failures open a durable incident, sustained successes resolve it, incident history is visible, the incident survives an API restart and evidence retention truncation, and the availability read remains the separate evidence-derived assessment.
- Required repository, contract, Go/race/real-PostgreSQL, Rust regression/audit, Web, Docker/browser and aggregate CI at the final implementation head; exact-main evidence after an authorized merge.

## Exclusions and next review

No notification delivery (R4), no uptime percentages, no maintenance windows, no per-Monitor configurable thresholds, no aggregation, no ownership or remote access, no change to the availability read semantics (R3-1), no new result kinds, no Rust or internal-contract change. R0-04 supplies the detection expectation (local detection, confirmation/recovery, durable incident state now proposed, no alert-delivery guarantee); R0-09 keeps the first notification channel deferred; the other R0 decisions remain as recorded for the Path-A local-evaluation scope. The external monitoring beta NO-GO judgment is unchanged.

Review the approach-1 decision (durable counters + incident rows on the write path), the reuse of R3-1 N/M transition semantics, the one-open-incident invariant, the completion-vs-claim hook and serialization direction, the availability/incident separation and divergence rule, the single new bounded incident read operation, and the no-backfill rule. Scope approval precedes detailed design; written design approval precedes an implementation plan; product execution then follows TDD -> branch/commit -> PR. Merging this document records a proposal unless product decisions are separately approved.

## Evidence pointers

- [Current handoff](../../devops/current-handoff.md), [R0 decision brief](../../roadmap/2026-10-18-r0-product-release-decision-brief.md) (R0-04/R0-09 and the incident-separation mandate), [post-R2 roadmap R3-1/R4](../../roadmap/2026-11-13-post-r2-roadmap.md), [R3-1 scope](2026-11-15-r3-1-confirmation-recovery-scope-reassessment.md) (approach 2 deferred to this slice), [R3-1 design](2026-11-15-r3-1-confirmation-recovery-design.md).
- `apps/api/internal/modules/monitoring/adapters/postgres/check_execution.go`: `ClaimDueCheck` (Read Committed reconciliation + `retainHistoryWindow`) and `CompleteCheck` (row-lock terminalization, default isolation, no reconciliation) — the candidate incident write-path hooks.
- `apps/api/internal/modules/monitoring/adapters/postgres/retention.go`: the newest-50 retained window bounding evidence-derived confirmation; the pipelined-batch write-path pattern to mirror.
- `apps/api/internal/modules/monitoring/domain/availability.go`: the R3-1 sequence policy (N=3/M=2, five-row limit, inconclusive reset) whose transition semantics incidents materialize; the read stays unchanged.
- `apps/api/internal/modules/monitoring/domain/check_result.go`: the result-kind vocabulary for success/failure/inconclusive classification.
- `apps/api/internal/modules/monitoring/adapters/postgres/monitor_history.go` and `application/history_cursor.go` / `application/get_monitor_history.go`: the bounded keyset history read, version-2 cursor and limit/lookahead conventions the incident read mirrors with a new version prefix.
- `apps/api/migrations/00002_create_monitoring_check_runs.sql`: the `check_runs_one_pending_per_monitor_idx` partial unique index pattern to mirror for one-open-incident-per-Monitor; `apps/api/migrations/` 00001–00004 (next is 00005).
- `contracts/openapi/public.yaml`: the `/monitors/{monitorId}/history` operation (cursor/budget/`no-store`/`HEAD`/error conventions) to mirror, and the unchanged availability operation.