# Local Monitor Delete Scope Reassessment

**Date:** 2026-10-27
**Status:** Proposed scope and design direction; awaiting user review
**Verified baseline:** `main@d975aefad3209cba94da9793ad9d1e5993f963c7`, after #59/#60/#64/#65
**Authorization:** Documentation only; no runtime implementation, migration, merge or deployment.

## Purpose and before/after

A local operator can now register, discover, inspect, pause and resume persisted Monitors, but cannot remove one. Wrong-target and finished evaluation Monitors accumulate permanently in the bounded inventory (default 20, maximum 50 rows), and every retained Monitor stays eligible for real probe work until manually abandoned.

The next proposed R2 increment adds durable local deletion of a Monitor together with its CheckRun evidence. Go owns the removal transaction and post-delete read semantics. Availability continues to describe terminal evidence for existing Monitors only; deletion must not fabricate a final verdict or alter any remaining Monitor's results.

**Context decision:** on 2026-10-27 the user selected validation path A (single-operator, local-only evaluation) from the [R0 decision brief](../../roadmap/2026-10-18-r0-product-release-decision-brief.md) and chose monitor deletion as the next dependency-bounded work item. The nine R0 decisions remain OPEN; deletion informs R0-07 (data retention/deletion expectations) without deciding it.

## Alternatives

| Approach | Benefit | Cost |
|---|---|---|
| Hard delete Monitor + all its CheckRuns in one transaction (recommended) | Complete, explainable removal; no tombstone state; no migration | Evidence disappears with the Monitor; no historical availability record remains for a deleted target |
| Soft delete / tombstone lifecycle state | Rows remain recoverable | New mutable lifecycle field, forever-growing table, mixes lifecycle with scheduling state, contradicts the closed immutable Monitor shape |
| Restrict deletion while a claimed run is pending | No in-flight completion conflicts | Operator must observe pending state and wait or cancel; cancellation is explicitly out of scope; complicates the common path |
| New migration altering the FK to `ON DELETE CASCADE` | One-statement delete | Moves child removal into schema ownership, requires a migration where none is otherwise needed, hides the removal rule from the adapter |

**Draft assumption requiring approval:** use the first approach with explicit child-first deletion inside the Go transaction. If recoverability or pending-run restriction is required, revise scope before detailed design.

## Proposed rules

1. Delete removes the Monitor row and **all** its CheckRuns (pending and terminal) in a single explicit Read Committed transaction: delete `monitoring.check_runs` for the Monitor first, then `monitoring.monitors` with not-found detection, and commit before success. No existence preflight followed by an unprotected delete.
2. No new migration is required; the existing `NO ACTION` foreign key on `check_runs.monitor_id` is satisfied by child-first ordering. Historical migrations remain immutable.
3. After deletion the Monitor is unknown: `GET /monitors/{monitorId}`, `/latest-result`, `/availability` and `/scheduling` return 404, and the inventory no longer lists it. No tombstone responses or "deleted" evidence views.
4. In-flight claimed work: a CheckRun claimed before deletion is removed with the Monitor; a later completion attempt returns the existing not-found conflict and is treated as a non-retryable no-op by the Checker. New claims never select a deleted Monitor because its row is gone. Completion semantics, result vocabulary and the internal contract do not change.
5. Delete, pause/resume and claims serialize on the same Monitor row. A claim that commits before delete keeps its pending run until the delete removes it; pause/resume on a deleted row returns not found; a delete that commits first prevents all later claims. Detailed ordering outcomes belong to the design and must be proven with real transaction interleaving, not timing observation.
6. A repeated `DELETE` on an already-deleted Monitor returns 404. The client may present a known 404 after a delete command as "already deleted" confirmation rather than failure; the exact browser rules belong to the design.
7. Deletion has no ownership or authorization boundary, consistent with the rest of the local-only surface, and remains loopback-only. A lost delete response is an uncertain outcome: the server work may or may not have committed; the client must not retry automatically.
8. Deleting one Monitor changes nothing for others: their results, cadence, pending uniqueness and scheduling state are untouched. No availability denominator or SLO claim is introduced.

## Architecture and contract direction

Go owns policy, persistence and read semantics; Rust executes claimed work without database access; Web displays Go state through the loopback gateway. Deletion becomes the eighth public operation, following the two scheduling operations:

| Operation | Proposed behavior |
|---|---|
| `DELETE /monitors/{monitorId}` | Remove the Monitor and its CheckRuns; success is an empty `204 No Content` |

The request carries no body: a declared or streamed body, any query including a bare `?`, noncanonical raw paths, encoded segments and slash/dot aliases are rejected with 400 before application work, following the scheduling resource's precedence pattern. Unsupported methods, including HEAD/OPTIONS, return 405 with the exact `Allow` value for the resource. Every matched response, including errors, uses `Cache-Control: no-store`. Validation reuses the existing UUID parser and sanitized `{title,status}` problem responses: malformed ID 400, unknown Monitor 404, repository failure 500. Success has no body and no Location header.

`contracts/openapi/public.yaml` gains one closed `deleteMonitor` operation with fixtures for removal and unknown-Monitor outcomes; existing Monitor/list/result/availability/scheduling schemas are unchanged and Web types are regenerated through the existing generator. A dedicated inward application use case and repository port own the transaction; composition follows the established dedicated-constructor pattern without restructuring legacy constructors.

Gateway rules: `DELETE` joins creation POST and scheduling PUT as the third explicit mutation. It requires the same exact Host/Origin and cross-site Fetch Metadata protection, rejects body framing before forwarding, forwards no credentials/cookies or arbitrary headers, and produces sanitized local boundary errors (403 boundary, 400 malformed route, 405 method, 408 unfinished body, 502/504 upstream). Existing size limits, deadlines and response caps remain unchanged. No arbitrary-verb or `/internal/*` proxying is introduced.

## Browser direction

Add a delete control to the detail page only; the inventory gains no per-row actions, bulk operations or fan-out reads. Deletion is irreversible, so the control requires an explicit confirmation step and copy stating that the Monitor and its evidence are removed permanently. One mutation may be in flight; the control is disabled while writing and after confirmed deletion navigates back to the inventory. Transport failures show an uncertain outcome with an explicit refresh path and no automatic retry. The detail page must also tolerate the Monitor becoming not found through another client's deletion while it is being viewed.

## Acceptance evidence required later

- Contract/domain/application RED/GREEN for the closed operation, not-found handling, unchanged existing schemas and generated parity.
- Real PostgreSQL RED/GREEN for child-first atomicity, no orphan CheckRuns, repeated-delete 404, and the delete/claim/pause transaction orderings, including a claimed run removed by a later delete and a completion attempt observing not found.
- HTTP/gateway RED/GREEN for method/no-store/Allow behavior, UUID and raw-path rejection, body-framing rejection, exact Origin protection on DELETE, zero upstream calls for locally rejected input and unchanged existing operations.
- Web tests for confirmation gating, one in-flight mutation, no retry, uncertain outcomes, navigation fencing and not-found tolerance.
- Actual composed browsers on 4173/4817: register, inspect, delete, confirm 404 reads and inventory exclusion across an API restart, plus a Checker in-flight journey where a claimed run is deleted before completion. Transaction evidence proves races; browser timing alone does not.
- Required repository, contract/generated, Go/race/real-PostgreSQL, Rust regression, Web, Docker/browser and aggregate CI at the final implementation head; exact-main evidence after an authorized merge.

These are acceptance requirements, not an approved implementation plan. Proposed behavior has no runtime evidence yet.

## Exclusions and next review

No target URL edit/update, undelete, archive, bulk delete, configurable retention, CheckRun history views, scheduled maintenance, incident/uptime aggregation, notifications, ownership/accounts, audit trail, remote access, quotas or multi-Checker redesign. Rust and the internal contract remain unchanged. The deferred populated-data EXPLAIN evidence remains a separate capacity-evidence minor. R0-01 through R0-09 remain OPEN.

Review the hard-delete rule, child-first transaction, empty `204` success, detail-only confirmation UI and post-delete 404 semantics. Scope approval precedes detailed design; written design approval precedes an implementation plan. Product execution then follows TDD -> branch/commit -> PR. Merging this document records a proposal unless product decisions are separately approved.

## Evidence pointers

- [Current handoff](../../devops/current-handoff.md), [R0 decision brief](../../roadmap/2026-10-18-r0-product-release-decision-brief.md), [launch roadmap](../../roadmap/2026-10-02-industry-benchmark-and-launch-roadmap.md).
- `apps/api/migrations/00002_create_monitoring_check_runs.sql`: `NO ACTION` foreign key requiring child-first deletion.
- `apps/api/internal/modules/monitoring/adapters/postgres/check_execution.go`: claim/completion transactions, not-found conflict path.
- `apps/api/internal/modules/monitoring/adapters/postgres/monitor_scheduling.go`: established atomic single-row write pattern.
- `apps/api/internal/modules/monitoring/adapters/http/scheduling.go`: established raw-path precedence and no-store pattern.
- `apps/web/server/gateway.ts`, `proxy.ts`, `routes.ts`, `server.ts`: current mutation guards, forwarding and transport bounds.
