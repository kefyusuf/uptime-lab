# Local Monitor Pause/Resume Scope Reassessment

**Date:** 2026-10-08
**Status:** Proposed scope and design direction; awaiting user review
**Verified baseline:** `main@9b577ea3239803f0dac19ea4c687669228e4b339`, after #56/#58/#57
**Authorization:** Documentation only; no runtime implementation, migration, merge or deployment.

## Purpose and before/after

Inventory now lets a local operator discover persisted Monitors and navigate to evidence. Every Monitor remains eligible for the current claim policy; the operator cannot suspend future probes while retaining identity and results.

The next proposed R2 increment adds durable pause/resume on the local detail page. Go owns scheduling eligibility. Availability continues to describe terminal evidence with the existing freshness policy; pause must not fabricate a healthy result or introduce a new availability verdict.

## Alternatives

| Approach | Benefit | Cost |
|---|---|---|
| Stop new claims; already claimed work may finish (recommended) | Durable control with the current Checker protocol | Work claimed before pause may execute or finish afterward; explain this in UI |
| Cancel already claimed work too | Stronger stop semantics | Requires cancellation protocol, Rust changes, result semantics and more races; a wider increment |
| Disable checking only in browser | Small UI change | Server continues claiming; does not meet the purpose |

**Draft assumption requiring approval:** use the first approach. An unanswered optional question about claimed work is not approval. If cancellation is required, revise scope before detailed design.

## Proposed rules

1. New and preexisting Monitors default to active. Paused state survives API/Checker restarts.
2. Pause stops new claims after its transaction commits. Claim and pause serialize on the same Monitor row: a claim committed first remains valid, even if execution has not started; a pause committed first prevents the next claim. Avoid unprotected read-then-write eligibility checks.
3. Pause/resume updates only Monitor scheduling state, without locking or mutating CheckRuns. Existing claim reconciliation can lock CheckRuns before a Monitor; adding the opposite locking order risks lock inversion.
4. Already claimed work retains deadline, completion, replay and conflict rules. Existing timeout reconciliation still includes expired pending runs of paused Monitors. No cancellation or independent reconciler is introduced.
5. Resume preserves the existing 60-second eligibility interval after the latest terminal completion. It does not promise an immediate probe. Without a terminal result, the Monitor is eligible at the next claim opportunity, subject to the existing pending-run guard.
6. Identity, target URL, creation time, inventory ordering and results remain unchanged. Scheduling and availability are separate snapshots; a paused Monitor may become `unknown/stale_result` under current policy.

Real transaction/race tests must prove these rules under the actual PostgreSQL isolation level. Existing locks alone are not evidence that a new predicate is correct.

## Architecture and contract direction

Go owns policy/persistence; Rust executes claimed work without database access; Web displays Go state through the loopback gateway. Prefer a separate scheduling resource over changing the closed immutable Monitor response or introducing a toggle:

| Operation | Proposed behavior |
|---|---|
| `GET /monitors/{monitorId}/scheduling` | Read `{"state":"active"}` or `{"state":"paused"}` |
| `PUT /monitors/{monitorId}/scheduling` | Set that explicit desired state; return persisted state with 200, including a repeated same-state write |

Both objects are closed. Reject unknown/duplicate keys, empty/null bodies, unsupported states and extra JSON values. Proposed mutation body limit: 1024 bytes, including streamed bodies, before storage work. Accept no query, including a trailing `?`. GET accepts no body. HEAD and other methods return 405 with `Allow: GET, PUT`. Matched responses, including errors, use no-store. Follow existing UUID validation, raw-path rejection and sanitized problem patterns: invalid input 400, unknown Monitor 404, excessive body 413, unsupported media type 415, repository failure 500. Detailed fixtures/error precedence belong to the later design.

A dedicated inward scheduling repository port provides existence-aware reads and atomic writes. Proposed persistence is `paused boolean NOT NULL DEFAULT false` on `monitoring.monitors`, in the next additive migration; historical migrations remain immutable. Current Monitor/create/list shapes, cursor encoding and CheckRun vocabulary do not change. Domain policy does not depend on database or transport types.

Readiness must include the new migration in its exact migration set. This does not establish rolling schema compatibility or zero-downtime deployment.

Concurrent opposite commands use the final committed state. A delayed retry can overwrite a later opposite command. Gateway/UI must never automatically retry an uncertain PUT. A lost response does not prove failure or cancellation of a transaction.

## Browser and gateway direction

Add state and Pause/Resume controls to detail only; no inventory fields, per-row reads, bulk operations or health fan-out. One bounded scheduling read loads state. Until it succeeds, controls are disabled and state is unknown; availability/raw views remain independently usable.

Allow one mutation in flight per view, disable controls during it, and show confirmed state only after a valid response. Fence stale responses after Monitor navigation or newer requests. Abort cannot undo a committed mutation. On timeout/disconnect/invalid response, show uncertain state, do not retry PUT automatically, and offer an explicit state refresh before further action.

Explain: "Pausing stops new claims. Already claimed checks may still complete."

Explicitly allow scheduling GET/PUT through the gateway. PUT requires exact Host/Origin and cross-site protection, JSON media validation, bounded buffering, inbound deadlines, upstream timeout and sanitized errors. Current guards and proxy body forwarding are POST-specific; changing the route union alone would leave PUT unprotected or discard its body. Include method logging in the assessment. Keep internal routes inaccessible and arbitrary proxy access closed.

## Acceptance evidence required later

- Contract/domain/application RED/GREEN for valid states, repeated writes, unknown Monitors and unchanged immutable Monitor responses.
- Real PostgreSQL RED/GREEN for migration defaults, durable state, active-only claims, both pause/claim transaction orderings, resume cadence, pending uniqueness and independent Monitor progress with skipped locks.
- Completion/timeout regression evidence for paused Monitors, including late completion/idempotent replay and absence of new lock inversion.
- HTTP/gateway RED/GREEN for JSON/size/query/path/method rejection, exact Origin protection on PUT, body forwarding, no-store, deadlines and sanitized errors; malformed or boundary-rejected requests reach no storage/upstream work.
- Web tests for loading/error/unknown/uncertain state, one in-flight mutation, no retry, stale-response fencing and independent evidence views.
- Actual composed browsers on 4173/4817: discover, pause, reload/restart, verify durable state and no new claims over a bounded observation, resume and observe a subsequent real claim. Transaction evidence proves races; browser waiting alone does not.
- Relevant required repository, migration, contract/generated, Go/race/PostgreSQL, Rust, Web, Docker/browser and aggregate CI on the final implementation head; exact-main evidence after an authorized merge.

These are acceptance requirements, not an approved implementation plan. Proposed behavior has no runtime evidence yet.

## Exclusions and next review

No cancellation, edit/delete, configurable cadence, history/retention, scheduled maintenance, incident/uptime aggregation, notifications, ownership/accounts, audit trail, remote access, quotas or multi-Checker redesign. Manual pause is not a maintenance exclusion from an uptime denominator. R0 audience/workload/operator/budget/recovery and protected-beta release gates remain unresolved; the deferred inventory EXPLAIN minor remains a capacity-evidence limitation.

Review the stop-new-claims rule, separate scheduling resource, resume cadence and detail-only UI scope. Scope approval precedes detailed design; written design approval precedes an implementation plan. Product execution then follows TDD -> branch/commit -> PR. Merging this document records a proposal unless product decisions are separately approved.

## Evidence pointers

- [Current handoff](../../devops/current-handoff.md), [launch roadmap](../../roadmap/2026-10-02-industry-benchmark-and-launch-roadmap.md), [local Web](../../frontend/local-monitoring-web.md) and [verification](../../testing/local-monitoring-web.md).
- `apps/api/internal/modules/monitoring/domain/monitor.go`: immutable Monitor.
- `apps/api/internal/modules/monitoring/adapters/postgres/check_execution.go`: claim/timeout/completion transactions and locks.
- `apps/api/internal/modules/monitoring/application/check_execution.go`: cadence/deadline policy.
- `apps/api/migrations/`: immutable history; no lifecycle state at baseline.
- `apps/web/server/gateway.ts`, `proxy.ts`, `routes.ts`, `server.ts`: current mutation guards, forwarding and transport bounds.
