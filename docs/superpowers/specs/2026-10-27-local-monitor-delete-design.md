# Local Monitor Delete Design

**Date:** 2026-10-27
**Status:** Written design for review; implementation not authorized
**Baseline:** `main@d975aefad3209cba94da9793ad9d1e5993f963c7`; scope proposal at `3ea70e0` in [PR #68](https://github.com/kefyusuf/uptime-lab/pull/68)
**Stage decision:** The user's continuation after the scope proposal authorizes this written design. It does not approve an implementation plan, merge or deployment.

## 1. Intent and boundaries

Allow a local operator to permanently remove a persisted Monitor together with all of its CheckRun evidence, preserving the Go/Rust ownership model, the closed immutable Monitor shape, and the existing evidence views for every remaining Monitor.

Deletion is irreversible. This increment introduces no edit/update, undelete, archive, history, retention policy, ownership, authorization, audit trail or remote exposure. Rust and the internal Checker contract remain unchanged; no migration is added, so the readiness migration set stays at four.

## 2. Deletion semantics

Deleting a Monitor removes its row and **every** CheckRun referencing it — pending and terminal — in one atomic transaction. Nothing is fabricated: no final availability verdict, no tombstone views, no retained evidence. After deletion the Monitor is unknown to all reads, and the inventory no longer lists it.

One Monitor's deletion changes nothing for others: their rows, runs, cadence, pending uniqueness and scheduling state are untouched. A repeated delete returns 404. A lost response is an uncertain outcome: the server work may or may not have committed, and the client must not retry automatically.

## 3. Domain, application and persistence

Add one narrow application use case, `DeleteMonitor`, receiving a validated `domain.MonitorID`; it samples no clock and invokes no Checker. A dedicated `MonitorDeletionRepository` port provides `Delete(ctx, id) error`. Reuse `ports.ErrMonitorNotFound`, the existing application not-found/persistence translation, and context cancellation mapping. Composition follows the established dedicated-constructor pattern: `NewModuleWithDeletion(...)` adds a `DeleteMonitor` field, and the HTTP adapter gains a private single-method interface; legacy constructors remain available.

The adapter transaction is explicitly Read Committed and proceeds in four statements:

1. **Pre-sweep:** `DELETE FROM monitoring.check_runs WHERE monitor_id = $1` — best-effort removal of all current children before any Monitor lock is taken.
2. **Lock:** `SELECT id FROM monitoring.monitors WHERE id = $1 FOR UPDATE` — no returned row means not found; the held lock blocks new claims (their candidate scan uses `SKIP LOCKED`) and pause/resume writes.
3. **Re-sweep:** repeat the child `DELETE` — idempotent; it removes a pending run any claim managed to commit between statements 1 and 2. No new child can appear after this point, because a claim requires the Monitor row lock this transaction already holds.
4. **Remove:** `DELETE FROM monitoring.monitors WHERE id = $1`, then commit before returning success.

The pre-sweep is load-bearing, not redundant. The claim transaction locks CheckRun rows (expired-run reconciliation) before locking its Monitor row; a delete that took the Monitor lock first and then waited on a CheckRun lock would invert that order and admit a deadlock cycle. Sweeping children before acquiring the Monitor lock keeps every participant's hold-and-wait order compatible. Without the pre-sweep, a claim committing a pending run between the sweep and the lock would also violate the `NO ACTION` foreign key at statement 4.

On any statement failure the transaction rolls back. On commit uncertainty the adapter returns an error and the client treats the outcome as uncertain. The pre-sweep and re-sweep apply to this application's deadlock reasoning only; PostgreSQL may still abort a participant in rare overlapping windows, and every abort path is safe because both transactions are atomic.

## 4. Serialization outcomes

| Ordering | Required outcome |
|---|---|
| Delete holds the Monitor lock while a claim scans | `SKIP LOCKED` skips it; another eligible Monitor can progress |
| Claim commits pending work before the delete acquires the lock | The re-sweep removes that run; delete commits; the Checker's later completion observes not found |
| Delete commits before a claim scans | The candidate query cannot select the removed row |
| Claim holds the lock (mid-transaction) when delete starts | The delete's lock statement waits; after the claim commits, the delete proceeds; after a claim rollback, the delete proceeds |
| Pause/resume writes before or after a delete commit | Loser of the ordering observes not found (zero-row update or absent lock row) |
| Two deletes race | One commits; the other observes not found |

A Checker completion for a run removed by a delete receives the existing not-found problem response, `404 "The check run was not found."` — the landed adapter maps unknown CheckRun completion to 404 and reserves 409 for terminal-row conflicts. Verified reading of the runtime: the Rust client maps any 4xx to `SubmitError::Rejected`, and the worker treats that as a final non-retryable delivery (`result_delivery_failed` event) before continuing its loop. No Rust change is required.

Actual PostgreSQL interleaving tests remain required; this section is design inference, not evidence. Lock-order reasoning must be proven under the real isolation level, including an overlapping claim reconciliation, rather than assumed from statement ordering.

## 5. Public HTTP contract

Add one operation to the existing `/monitors/{monitorId}` path item: `deleteMonitor`, method `DELETE`. Success is an empty `204 No Content` with no Location header. Unknown Monitor is 404; malformed UUID or raw path is 400; repository failure is a sanitized 500. Every matched response, including errors and 204, uses `Cache-Control: no-store`.

Validation precedence mirrors the scheduling resource: set no-store first; reject noncanonical raw paths, any query (including a bare `?`), encoded segments, aliases and fragments with 400; reject methods other than GET/DELETE with 405 and exact `Allow: GET, DELETE`; validate the UUID; reject a DELETE carrying declared length or transfer encoding with 400; then run exactly one use case. HEAD writes no body under actual server behavior.

The resource's `Allow` value changes from `GET` to `GET, DELETE`; this is a deliberate, contract-visible change to existing 405 responses and existing transport tests asserting the old value must be updated, not preserved. Collection and sub-resource `Allow` values are unchanged. Internal OpenAPI, the seven existing operation schemas and the internal contract are untouched; Web types are regenerated through the existing generator, with fixtures added per convention.

## 6. Gateway boundary

`DELETE` joins creation POST and scheduling PUT as the third explicit mutation. The route decision union adds `DELETE` for the explicitly mapped monitor resource only; no arbitrary-verb or `/internal/*` proxying is introduced.

DELETE requires exactly one allowed Host and one Origin equal to the configured loopback origin, with duplicate/foreign Origin and cross-site Fetch Metadata rejected. Body framing — declared length or streamed bytes — is rejected locally before upstream forwarding; the upstream request is bodyless. No credentials, cookies, Origin or arbitrary headers go upstream. Existing header/response caps, all deadlines and sanitized boundary errors (403 boundary, 400 malformed route/input, 405 method, 408 unfinished input, 502/504 upstream) remain unchanged. No upstream retry.

## 7. Browser state and uncertain deletes

Add scheduling-style API/decoder ownership to `entities/monitor` (`deleteMonitor(id, signal)` returning a closed outcome) and a `features/delete-monitor` hook/control composed only in `MonitorDetailPage`. The client outcome is `{kind:'deleted'}` or `{kind:'uncertain';error:ClientError}`: a valid 204 confirms deletion, and a known 404 is presented as deletion too — the Monitor is absent either way, and distinguishing "deleted" from "never existed" is neither possible nor useful locally. Every other known rejection, 5xx, malformed or transport failure is uncertain. No automatic retry.

The control is two-step: an initial "Delete monitor" action reveals an explicit confirmation ("Delete permanently" / "Cancel") with copy stating that the Monitor and all of its check evidence are removed permanently. One mutation may be in flight; the control is disabled while writing; a confirmed deletion navigates back to the inventory. On an uncertain outcome the page shows an explicit message and offers refresh: a follow-up read returning 404 resolves the uncertainty as deleted; a 200 resolves it as not deleted. The detail page must also tolerate the Monitor becoming not found mid-view through another client's deletion. Evidence refresh and scheduling state remain independent until a deletion is confirmed. No optimistic removal, polling or bulk controls.

## 8. Required acceptance and evidence limits

Before implementation approval, the plan must map these requirements to meaningful RED/GREEN increments:

- Contract/domain/application: closed operation, empty 204 shape, unknown-Monitor mapping, unchanged existing schemas, generated parity, and the deliberate `Allow: GET, DELETE` change.
- PostgreSQL: child-first atomicity with no orphan CheckRuns, pre-sweep and re-sweep necessity (a claim committing between sweep and lock is removed), repeated-delete 404, both claim/delete orderings, delete vs pause/resume orderings, racing deletes, and reconciliation-overlap safety — including observing how a lock-wait or deadlock abort surfaces as a sanitized 500 and never as partial state. Coordinated transactions and bounded synchronization; no arbitrary sleeps.
- HTTP/socket/gateway: precedence, body-framing rejection, no-store on 204 and errors, exact Allow values, aliases without redirect, DELETE Origin protection, zero upstream/storage calls for locally rejected input, unchanged existing operations.
- Browser unit/state: two-step confirmation, one in-flight mutation, no retry, uncertain resolution via refresh, navigation fencing, not-found tolerance, independent evidence.
- Actual composed Chromium on 4173/4817: register, inspect, delete, confirm 404 reads and inventory exclusion across an API restart; plus a Checker in-flight journey where a claimed run is deleted before completion and the delivery failure is non-retryable. Transaction evidence proves races; browser timing alone does not.
- Required repository, contract/generated, Go/race/real-PostgreSQL, Rust regression/audit, Web, Docker/browser and aggregate CI at the final implementation head; exact-main checks after an authorized merge. The populated-data EXPLAIN limitation remains outside this increment.

This document has no proposed-feature runtime proof. No deployment or production migration follows from it.

## 9. Review and next stage

Review this written design, especially the pre-sweep/lock/re-sweep ordering and its deadlock reasoning, the empty 204 success shape, the deliberate Allow change, the 404-as-deleted client mapping, and the two-step confirmation scope. Only after design approval may the implementation plan be produced; plan review precedes Native TDD execution. No deployment, merge or external-service change follows automatically from these approvals.
