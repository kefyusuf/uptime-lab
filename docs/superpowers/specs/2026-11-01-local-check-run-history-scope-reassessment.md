# Local Check-Run History Scope Reassessment

**Date:** 2026-11-01
**Status:** Proposed scope and design direction; awaiting user review
**Verified baseline:** `main@b4f0154f`, after #59/#60/#64/#65/#68/#69/#70
**Authorization:** Documentation only; no runtime implementation, migration, merge or deployment.

## Purpose and before/after

A local operator can register, discover, inspect, pause, resume and permanently delete Monitors, but the only durable execution evidence exposed through the product is the single latest terminal CheckRun (`latest-result`) and its availability assessment. Earlier terminal runs exist in `monitoring.check_runs` yet are invisible; an operator evaluating target behaviour cannot see the recent sequence of outcomes behind the current snapshot.

The next proposed R2 increment adds a bounded, terminal-only, keyset-paginated check-run history read on the detail page. Go owns the query, cursor and response budget; availability semantics are untouched; the immutable Monitor shape and the closed evidence views do not change.

**Context decision:** validation path A (single-operator, local-only evaluation) remains selected. With deletion landed, the historical evidence trail has a defined lifetime — this increment makes the retained window visible without introducing retention policy.

## Alternatives

| Approach | Benefit | Cost |
|---|---|---|
| Bounded keyset-paginated history read (recommended) | Reuses the proven inventory cursor/budget conventions and the existing terminal index; no new mutation semantics | No retention, no aggregation — the page is bounded and older evidence stays queryable only through future increments |
| Unbounded full history | Complete trail | No supported Monitor count or response budget; capacity stays unqualified; contradicts the bounded-response discipline |
| Extending `latest-result` with more rows | One fewer operation | Changes a closed schema and the empty-204 contract; couples the single-snapshot semantics to list semantics |
| Availability/uptime timeline or aggregation | Product-shaped reporting | R3 territory — mixes raw execution facts with verdict/aggregation design before incidents exist |

**Draft assumption requiring approval:** use the first approach. If aggregation or retention is the actual need, revise scope before detailed design.

## Proposed rules

1. A new bounded read `GET /monitors/{monitorId}/history` returns terminal CheckRuns for one Monitor, ordered by `completed_at DESC` then `check_id DESC`, default 20 and at most 50 rows per page, mirroring the inventory conventions: strict `limit`/`cursor` query grammar, 88-character unpadded base64url versioned keyset cursor encoding the persisted UTC microsecond completion instant and CheckRun UUID, and a bounded compact JSON response budget. An empty history is a successful empty page with a null cursor.
2. Pending CheckRuns stay invisible — the page contains terminal rows only, consistent with `latest-result`. Late, duplicate, conflict and `worker_timeout` rows appear exactly as the durable vocabulary stores them; no verdict, availability or uptime math is derived.
3. Unknown Monitor is 404; a known Monitor without terminal history is a successful empty page, deliberately distinct from the unknown-Monitor 404. The read never samples a clock and never mutates state.
4. Pagination is a strict position, not an identity lookup or snapshot guarantee, matching the inventory cursor contract: no ownership, no consistency promise across pages, no authorization boundary. Concurrent completions may shift page contents between reads.
5. No new migration: the existing `check_runs_latest_terminal_idx (monitor_id, completed_at DESC) WHERE completed_at IS NOT NULL` serves the ordering predicate; the readiness migration set stays at four. Rust, the internal Checker contract and every existing public schema remain unchanged.
6. The cursor is decode-only input: malformed, noncanonical, foreign-monitor or over-budget cursors are rejected before application work, following the inventory grammar. Deleting a Monitor removes its history with it; a cursor anchored on deleted evidence yields an empty continuation, not an error.

## Architecture and contract direction

Go owns query, cursor and budget; Rust executes claimed work without database access; Web displays Go state through the loopback gateway. The ninth public operation on a new sub-resource:

| Operation | Proposed behavior |
|---|---|
| `GET /monitors/{monitorId}/history` | Bounded terminal-history page: `{items, nextCursor}` with the closed row shape `{checkId, resultKind, httpStatus?, durationMs?, completedAt}` |

Row fields reuse the existing `latest-result` vocabulary without redefining it: the page item schema references the same discriminated result kinds, and no new result kinds or optional fields are invented. Response budget, `no-store` on every matched response, HEAD/unsupported-method rejection with exact `Allow: GET`, raw-path canonicality and alias rejection follow the resource and collection precedents. A dedicated inward port (analogous to the inventory repository) exposes a bounded page query with a validated anchor; composition follows the established dedicated-constructor pattern, and the gateway maps the route with the same strict query grammar the inventory uses today, reusing its validator rather than widening accepted input.

`contracts/openapi/public.yaml` gains the closed operation, the page/row schemas and fixtures (empty, single, full page, continuation); existing schemas are unchanged and Web types regenerate through the existing generator.

## Browser direction

The detail page gains a bounded history section composed only while the Monitor is known, styled after the existing evidence cards: one read on Monitor success plus explicit refresh, loading/empty/error states, and Next-page traversal with a Refresh control mirroring the inventory start-page behaviour. The section stays independent of `useMonitorReads`, the scheduling control and the deletion control — no fan-out, no polling, no optimistic insertion. A deleted Monitor's history control disappears with the rest of the detail view through the existing not-found tolerance.

## Acceptance evidence required later

- Contract RED/GREEN for the closed operation, cursor grammar, budget, unchanged existing schemas and generated parity.
- Real PostgreSQL RED/GREEN for terminal-only paging at the exact 20/50 boundaries, ordering ties on equal completion instants, continuation keys naming their last included row, unknown-Monitor and empty-history outcomes, cursor rejection grammar, and invariance under concurrent completion.
- HTTP/gateway RED/GREEN for method/no-store/Allow behavior, query grammar, aliases without redirect, budget enforcement and unchanged existing operations.
- Web tests for loading/empty/error states, one in-flight read, navigation fencing, independent evidence views and accessible copy.
- Actual composed Chromium on 4173/4817: register, claim through the real Checker, page through the real history across an API restart, verify durable linkage; transaction evidence proves ordering, browser timing alone does not.
- Required repository, contract/generated, Go/race/real-PostgreSQL, Rust regression/audit, Web, Docker/browser and aggregate CI at the final implementation head; exact-main evidence after an authorized merge. The populated-data EXPLAIN evidence remains a separate capacity-evidence minor.

## Exclusions and next review

No retention policy, target-URL edit, undelete, aggregation or uptime math, scheduled maintenance, incidents, notifications, ownership/accounts, audit trail, remote access, quotas or multi-Checker redesign. Rust and the internal contract remain unchanged. The nine R0 decisions remain OPEN; this read neither decides nor implies any of them.

Review the terminal-only rule, the inventory-mirroring cursor/budget grammar, the empty-page distinction from 404 and the detail-only section scope. Scope approval precedes detailed design; written design approval precedes an implementation plan. Product execution then follows TDD -> branch/commit -> PR. Merging this document records a proposal unless product decisions are separately approved.

## Evidence pointers

- [Current handoff](../../devops/current-handoff.md), [R0 decision brief](../../roadmap/2026-10-18-r0-product-release-decision-brief.md), [launch roadmap](../../roadmap/2026-10-02-industry-benchmark-and-launch-roadmap.md).
- `apps/api/migrations/00002_create_monitoring_check_runs.sql`: existing terminal ordering index.
- `apps/api/internal/modules/monitoring/ports/monitor_inventory_repository.go`: established anchor/page port shape.
- `apps/api/internal/modules/monitoring/adapters/postgres/monitor_inventory.go`: established keyset page query.
- `apps/web/server/inventory-query.ts` and `routes.ts`: established query grammar and route mapping to reuse.
- `apps/api/internal/modules/monitoring/adapters/postgres/monitor_deletion.go`: deletion removes history with the Monitor.
