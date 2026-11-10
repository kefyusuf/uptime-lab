# Local Target-URL Edit Design

**Date:** 2026-11-10
**Status:** Written design for review; implementation not authorized
**Baseline:** `main@c8b21bc`; scope proposal merged through [PR #84](https://github.com/kefyusuf/uptime-lab/pull/84)
**Stage decision:** The user's approved continuation after the scope proposal authorizes this written design. It does not approve an implementation plan, merge or deployment.

## 1. Intent and boundaries

Add the tenth public operation, `PATCH /monitors/{monitorId}`, editing one Monitor's target URL on its existing immutable identity. The Monitor's UUID, creation time, evidence rows, scheduling state, pause/resume and deletion semantics are unchanged; the target becomes the single mutable field through a validated domain transformation. No retention change, no history per-row target annotations, no revision model, no aggregation, no incidents/notifications/ownership/remote access. Rust and the internal Checker contract remain unchanged; no migration is added and the readiness set stays at four.

## 2. Edit semantics

The edit passes the exact registration validation: the new target must satisfy the same `TargetURL` syntax rules as `CreateMonitorRequest` (absolute HTTP/HTTPS with a hostname, no userinfo or fragments; query strings and explicit ports preserved). Validation is syntactic — policy rejection stays a probe-time fact, exactly as at registration.

**In-flight claims probe the old target.** A claim snapshots the target at claim time (`ClaimedCheck.TargetURL`); already claimed work completes against that captured target and its rows remain durable evidence under the Monitor — the bounded, documented rule pause/resume established for already claimed work. Future claims probe the new target.

**History mixes targets.** The bounded history read has no per-row target field; rows recorded before and after an edit are indistinguishable in the page. Documented limitation, not a schema change. Latest-result and availability semantics are unchanged apart from future probes hitting the new target — no verdict math, no reclassification of old rows.

Edit while paused is allowed: scheduling state and target are independent fields. Concurrent edits are last-committed-wins without versioning. A lost edit response is an uncertain outcome; the client must not retry automatically.

## 3. Domain, application and persistence

The domain gains a validated transformation mirroring registration validation:

```go
func (monitor Monitor) WithTargetURL(targetURL TargetURL) (Monitor, error)
```

It returns a new immutable Monitor value with the same identity and creation time and the validated new target — the zero value cannot pass through, and the transformation cannot be constructed without a valid `TargetURL`. The closed Monitor shape's JSON contract changes only in the target value it returns, not in its schema.

A dedicated application use case `EditMonitorTarget` receives the validated MonitorID and raw target text, validates through the same `TargetURL` path registration uses, delegates once, and reuses the existing not-found/persistence translation and context cancellation mapping. No clock, no Checker path.

A dedicated port `MonitorTargetEditRepository` exposes `EditTarget(ctx, monitorID, targetURL) (domain.Monitor, error)`. The adapter runs one explicit Read Committed transaction containing an existence probe and one `UPDATE monitoring.monitors SET target_url = $2 WHERE id = $1 RETURNING id, target_url, created_at`, mapping the returned row through `domain.NewMonitor` — the same single-row atomic write pattern as the scheduling and deletion adapters. Not-found detection inside the same transaction; commit before success. The identifier and new target bind as binary parameters (`pgtype.UUID` and the validated text), matching the batch-formulation discipline that satisfies the Mimosa pre-write scan; a pipelined batch of probe and update mirrors the history adapter.

## 4. Serialization and consistency

| Ordering | Required outcome |
|---|---|
| Edit holds the Monitor row while a claim scans | `SKIP LOCKED` skips it; another Monitor can progress |
| Claim commits pending work before the edit acquires the row | The edit proceeds; the in-flight run probes the old target and its completion is durable |
| Edit commits before a claim scans | Future claims read the new target from the row |
| Edit vs pause/resume serialize on the row | Loser observes the committed state; paused state survives edits |
| Edit vs delete | One commits; the loser observes not found |
| Edit while paused | Allowed; target and scheduling are independent fields |

No reverse lock path: the edit takes only the Monitor row. The history read's keyset predicate, the retention window and the pending uniqueness constraint are unchanged.

## 5. Public HTTP contract

`PATCH /monitors/{monitorId}` with a closed `MonitorTargetEditRequest` schema (`targetUrl`, same syntactic rules as `CreateMonitorRequest.targetUrl`, `additionalProperties: false`). Success is 200 with the closed `Monitor` shape reflecting the updated target. Errors: 400 (malformed UUID, body, target), 404 (unknown Monitor), 405 (`Allow: GET, PATCH`), 415 (media), 422 (target contract violation), 500 (sanitized persistence failure); every matched response uses `no-store`; HEAD/OPTIONS unsupported with no response body.

Body handling mirrors creation: `application/json` only, bounded (16384 bytes), strict single-document decode rejecting duplicate/unknown keys, alternate representations and trailing JSON. Raw-path canonicality, alias rejection without redirect, query rejection (no query keys on the sub-resource-less resource) follow the resource precedents. `contracts/openapi/public.yaml` gains the operation, the request schema (reusing the target description) and fixtures (edited, invalid target, unknown Monitor); no existing schema changes; Web types regenerate through the existing generator.

## 6. Gateway boundary

PATCH joins the explicit mutation set: exact Host/Origin, body buffered with the creation budget (16384 bytes), unfinished-body deadline, sanitized boundary errors and no arbitrary verb forwarding. The resource mapping gains PATCH beside GET/DELETE with the shared raw-path canonicality. No internal route opens.

## 7. Browser direction

`entities/monitor` gains the edit request type and a `editTarget(id, targetUrl, signal)` client method returning a closed outcome: `{kind:'edited';data:Monitor}` on a valid 200, `{kind:'rejected';error}` on known 400/404/415/422, `{kind:'uncertain';error}` on transport/5xx/invalid. A `features/edit-monitor-target` control composes only while the Monitor is known: an "Edit target" action reveals a pre-filled validated input with Save/Cancel; one mutation in flight; no automatic retry; uncertain outcomes show an explicit message and the refresh path; a confirmed edit updates the displayed target and the focus stays accessible. The two-step confirmation pattern is reused for state clarity (edit form is itself the confirmation). Stale responses are fenced after navigation; the control stays independent of scheduling, deletion and history views.

## 8. Required acceptance and evidence limits

- Contract RED/GREEN for the closed operation, unchanged existing schemas and generated parity.
- Domain RED/GREEN for `WithTargetURL` validation and unchanged identity/creation time.
- Real PostgreSQL RED/GREEN: atomic edit with not-found detection, edit-vs-claim ordering (in-flight claims finish against the old target; future claims read the new one), edit-vs-pause/resume and edit-vs-delete serialization, edit while paused, concurrent last-committed-wins edits, unchanged history/latest/availability/retention semantics.
- HTTP/gateway RED/GREEN for precedence, grammar, media, budget, no-store, Allow, aliases without redirect, Origin/body-buffering guards and zero storage/upstream calls for rejected input.
- Web tests: edit form gating, uncertain-outcome resolution, one in-flight mutation, stale fencing, independence from other controls.
- Composed Chromium on 4173/4817: register, edit the target, verify future claims probe the new target and old evidence rows remain durable, across an API restart.
- Required repository, contract, Go/race/real-PostgreSQL, Rust regression/audit, Web, Docker/browser and aggregate CI at the final implementation head; exact-main evidence after an authorized merge.

## 9. Review and next stage

Review this written design, especially the domain transformation over an identity mutation, the in-flight old-target rule, the history-mixing limitation and the PATCH contract shape. Only after design approval may the implementation plan be produced; plan review precedes Native TDD execution. No deployment, merge or external-service change follows automatically from these approvals.
