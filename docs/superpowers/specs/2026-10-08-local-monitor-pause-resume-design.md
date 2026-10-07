# Local Monitor Pause/Resume Design

**Date:** 2026-10-08
**Status:** Written design for review; implementation not authorized
**Baseline:** `main@9b577ea3239803f0dac19ea4c687669228e4b339`; scope documentation at `a00d1e5` in [PR #59](https://github.com/kefyusuf/uptime-lab/pull/59)
**Stage decision:** The user's continuation after the scope proposal authorizes this written design using the recommended stop-new-claims semantics. It does not approve this new artifact, an implementation plan, merge or deployment.

## 1. Intent and boundaries

Allow a local operator to pause future work for a persisted Monitor and resume it without losing its identity or results. Preserve the Go/Rust ownership model, existing evidence views, local access restrictions and completion-based cadence.

The selected approach stops future claims while work claimed before pause may execute or finish afterward. Cancellation would require a separate Rust/internal-contract design. Browser-only suspension would leave the server claiming and fails the purpose. See the [scope alternatives](2026-10-08-local-monitor-pause-resume-scope-reassessment.md).

This increment introduces no edit/delete, history, retention, configurable cadence, scheduled maintenance, incidents, notifications, ownership, audit log, quotas or remote deployment. A manual pause does not alter any uptime denominator or fabricate availability. Product implementation remains behind written-design and implementation-plan review.

## 2. Scheduling semantics

Scheduling has exactly two states: `active` and `paused`. New and existing Monitors default to active. Pause is durable and stops new claims once committed. Work already claimed retains its existing deadline, completion/idempotency/conflict behavior, including timeout reconciliation. No cancellation message or result kind is added.

Resume preserves the current due rule: 60 seconds after the latest terminal completion. No terminal result means eligible at the next claim opportunity, unless a pending run exists. Resume neither forces an immediate check nor creates a CheckRun. Creation time, target URL, results and inventory ordering remain unchanged.

Availability and scheduling are independent snapshots. A paused Monitor can still show a fresh prior result, later `unknown/stale_result`, or `unknown/no_result`. Raw result and availability may observe different CheckIDs. The browser labels scheduling separately and never recomputes availability.

## 3. Domain, application and persistence

Add a validated domain `SchedulingState` with only active/paused values; its zero/invalid value cannot be persisted or returned. Keep the immutable `Monitor` constructor/shape unchanged. Add two narrow application use cases: `GetMonitorScheduling` and `SetMonitorScheduling`. They receive a validated MonitorID; the setter also receives a validated desired state. Neither samples a clock or invokes Checker execution.

A dedicated `MonitorSchedulingRepository` port provides `GetScheduling(ctx, id)` and `SetScheduling(ctx, id, state)`. Results are domain states, not SQL booleans or transport objects. Reuse the existing repository/application not-found translation; wrap operational errors without exposing them publicly. No existence preflight followed by a separate unprotected update is allowed.

Add `00004_add_monitor_scheduling_state.sql` with `paused boolean NOT NULL DEFAULT false` on `monitoring.monitors`. Existing inserts use the database default. Read adapters map false to active and true to paused. Historical migrations are immutable; the migration readiness set advances to four. The downgrade section drops only this column under the repository's existing migration convention; do not run it automatically or advertise data-preserving rollback. Old binaries may become unready under the exact-set rule. No rolling-upgrade guarantee is added.

The setter begins a Read Committed transaction, performs one `UPDATE monitoring.monitors SET paused = $2 WHERE id = $1 RETURNING paused`, validates the mapped result, and commits before returning success. No returned row means not found. Always set the requested value, including repeated same-state commands, so same-state requests still serialize with opposite commands. On failure, roll back; on commit uncertainty, return an error and let the client treat outcome as uncertain. Read uses one bounded row query and no write lock. No new secondary index is required by this design; performance/capacity remains unqualified.

## 4. Claim serialization and lock order

Preserve global expired-run reconciliation, pending uniqueness, terminal cadence/order and `FOR UPDATE OF monitor SKIP LOCKED`. Add `monitor.paused = false` to candidate selection. Make the claim transaction explicitly Read Committed, and after it locks a candidate, issue a second `SELECT paused FROM monitoring.monitors WHERE id = $1` while retaining the lock. Only insert pending work if that locked row is still active. If it is paused, commit reconciliation without inserting and return the existing no-due result; a later poll may select another Monitor.

The second read is deliberately a separate statement, not another expression in the original joined query. It prevents correctness from depending solely on the old candidate snapshot or joined-query predicate re-evaluation. The setter and claim hold conflicting Monitor row locks until transaction end. This yields the following outcomes:

| Ordering | Required outcome |
|---|---|
| Pause owns the row lock while claim scans | SKIP LOCKED skips it; another eligible Monitor can progress |
| Pause commits before claim obtains the row | Candidate filter or locked-state recheck prevents insertion |
| Claim locks/rechecks active first | Pause waits; claim commits pending work before pause commits |
| Claim transaction rolls back | No pending work escapes; pause may proceed |
| Resume commits after a skipped claim | A future poll applies the normal cadence/pending guard |

Pause/resume acquires only the Monitor row. It never locks or updates CheckRun rows. Completion remains CheckRun-only. Reconciliation may lock CheckRuns before the claim locks a Monitor; preserve that order and introduce no reverse path. In-flight timeout reconciliation remains independent of paused state. A stopped Checker still does not gain an independent reconciler.

PostgreSQL documents conflicting row locks and Read Committed per-statement visibility in [row-level locking](https://www.postgresql.org/docs/18/explicit-locking.html#LOCKING-ROWS) and [Read Committed](https://www.postgresql.org/docs/18/transaction-iso.html#XACT-READ-COMMITTED), read on 2026-10-08. Applying these primitives to this claim algorithm is a design inference; actual PostgreSQL concurrency tests remain required.

## 5. Public HTTP contract

Add two operations to `contracts/openapi/public.yaml`: `getMonitorScheduling` and `setMonitorScheduling` on `/monitors/{monitorId}/scheduling`. Both success and PUT request use the closed schema `MonitorScheduling` with one required enum property `state`. Success is 200 JSON `{"state":"active"}` or `{"state":"paused"}`. Do not add identity/timestamps or change any existing Monitor/list/result/availability schema. Internal OpenAPI is unchanged. Add public fixtures for both states and regenerate Web types through the existing generator.

GET has no body. PUT requires `application/json`, optionally `charset=utf-8` case-insensitively, with no other media parameters or content encoding. Bound raw input to 1024 bytes, including streamed bodies and whitespace. JSON member spelling is exact; decode token-wise to reject duplicate `state`, unknown fields, missing state, non-string state, null/non-object input and trailing JSON. Escaped spellings are decoded before duplicate/property comparison. Reject any state other than active/paused.

For a recognizable scheduling route, validation precedence is:

1. Set no-store; reject noncanonical raw path, any query (including empty `?`), encoded segments, slash/dot/backslash aliases or fragments with 400 before application work. Unknown resource shapes remain 404; do not redirect.
2. Reject methods other than GET/PUT with 405 and exact `Allow: GET, PUT`, including HEAD/OPTIONS. HEAD writes no body under actual server behavior.
3. Validate MonitorID with the existing UUID parser; malformed ID is 400. Existing accepted UUID text forms remain accepted; do not introduce unrelated ID canonicalization.
4. GET with Content-Length greater than zero or transfer encoding is 400. For PUT, reject declared length above 1024 with 413, media/encoding violations with 415, streamed overflow with 413, and malformed JSON/read errors with 400. HTTP server transport deadlines still bound unfinished input.
5. Run exactly one use case. Unknown Monitor is 404. Repository/mapping/commit errors become sanitized 500. Success follows a committed write or successful read.

Use existing `{title,status}` problem responses with `application/problem+json`; do not expose SQL, URLs or stack traces. Every matched response is no-store. GET sends no mutation. PUT adds no Location header. Seven public operations result; old five keep their behavior. Contract tests cover schema closure and transport-specific rules separately.

## 6. Gateway boundary

Allow only GET/PUT at `/api/monitors/{id}/scheduling`, translating to the corresponding Go route. Scheduling accepts no query. Preserve the existing route syntax for IDs, then let Go validate UUIDs; raw path aliases/escapes are rejected before forwarding. Do not proxy arbitrary verbs, resources or `/internal/*`.

Treat POST creation and PUT scheduling as the explicit set of mutations in Origin validation, body collection and proxy headers. PUT requires exactly one allowed Host and one Origin equal to that Host-derived configured loopback origin; reject duplicates, null/foreign Origin and cross-site Fetch Metadata. Require the scheduling media rule above. Do not widen creation's accepted input policy unintentionally.

Scheduling uses a route-specific 1024-byte input cap, enforced for Content-Length and streamed bytes before upstream forwarding; creation retains 65536 bytes. GET rejects body framing before forwarding. Retain 16384-byte header cap, 5-second header deadline, 10-second body/upstream deadline, 262144-byte upstream response cap and 12-second browser request deadline. PUT must forward JSON Content-Type and exact buffered Content-Length; no request credentials/cookies, Origin or arbitrary headers go upstream. Existing server logging recognizes PUT without recording body, targets or new sensitive data.

Local boundary rejection precedes route validation. Then route/path/query, method, mutation Origin, media/declared size/body validation and upstream forwarding follow existing gateway structure. Boundary rejection is 403, malformed local route/input 400, unsupported method 405, oversized input 413, unsupported media 415, unfinished body 408, invalid upstream/transport response 502 and upstream timeout 504. Go responses otherwise pass through the current sanitized proxy rules, with no-store. No upstream retry is introduced.

## 7. Browser state and uncertain writes

Add scheduling API/decoder ownership to `entities/monitor`, a `features/set-monitor-scheduling` hook/control, and composition in `MonitorDetailPage`. Respect current feature boundaries and dependency rules. The detail page starts one scheduling read once its Monitor exists; inventory introduces no new reads or fields. Both success responses decode exact object keys and enum values; other 2xx/malformed values fail closed.

Scheduling state is independent of `useMonitorReads`: loading, ready, writing, read-error, or uncertain. Display Active/Paused only for confirmed state; unknown/error states disable mutation. Do not optimistically change the state or represent writing as successful. One current-view request may update scheduling at a time. Disable scheduling refresh while writing, fence response generations on Monitor navigation, and abort obsolete client work. Evidence refresh remains independent.

Pause sends paused; Resume sends active. A valid 200 establishes a confirmed snapshot. A known 400/404/413/415 rejection shows the error and requires a fresh state read before another command; all timeout/disconnect/5xx/invalid responses yield uncertain state. Do not automatically retry PUT. Offer "Refresh scheduling state"; a successful GET enables a later explicit user command. A GET is only a current snapshot: after a lost response, server work may still commit later. Neither client abort nor reread proves cancellation or quiescence. Concurrent clients are last-committed-write wins, without version/ETag ordering guarantees.

Explain "Pausing stops new claims. Already claimed checks may still complete." Announce loading/write/error outcomes accessibly; preserve current detail heading focus, evidence cards and UTC timestamps. No polling, bulk controls or scheduled maintenance is introduced.

## 8. Required acceptance and evidence limits

Before implementation approval, the plan must map these requirements to meaningful RED/GREEN increments:

- Domain/application/contract fixtures: valid/invalid states, closed schema, repeated desired-state behavior, not found and operational failures, unchanged existing schemas/generated parity.
- PostgreSQL: default state for migrated old/new rows, state persistence, atomic unknown-ID writes, concurrent opposite writes, both pause/claim lock orderings, a candidate paused after its snapshot, skipped-lock progress, rollback and unchanged pending uniqueness. Use coordinated transactions and bounded synchronization rather than arbitrary sleeps; test rollback and commit boundaries with the actual claim adapter.
- Cadence: paused never claimed; resumed no-result Monitor eligible; fresh terminal/pending Monitor not prematurely claimed; eligibility at the current boundary. Normal/duplicate/late completion and global timeout reconciliation still work for paused Monitors.
- HTTP/socket/gateway: validation precedence, strict duplicate-key JSON, exact byte boundary, streamed overflow, no-store/Allow/HEAD, aliases without redirect, PUT Origin/header/body forwarding, unfinished-body deadline and zero storage/upstream calls for malformed/boundary-rejected input.
- Browser unit/state: independent evidence, one in-flight write, no retry, uncertain/reread behavior, navigation and stale responses, accessible state/control copy.
- Actual composed Chromium on 4173/4817: inventory -> detail -> pause -> read/reload -> resume; verify persisted state after API restart and with restarted Checker using the same database. Use an isolated stack and controlled terminal/claim inputs to assert no new CheckRun while paused and a real subsequent claim after resume. Separate race proof from observational browser proof. Preserve existing safety-policy rejection smoke; do not infer cancellation from a stable CheckRun count.
- Required repository, immutable migration, generated contract, Go/race/real PostgreSQL, Rust regression/audit, Web and local Docker/browser CI gates at final implementation head, followed by exact-main checks after an authorized merge. Existing populated EXPLAIN limitation remains outside performance qualification.

This document has no proposed-feature runtime proof. A documentation CI pass validates repository governance, not scheduling semantics. Public launch still requires the [roadmap's R0 and protected-beta gates](../../roadmap/2026-10-02-industry-benchmark-and-launch-roadmap.md).

## 9. Review and next stage

Review this written design, especially locked-state recheck, transport precedence, uncertain-write limitations and restart/race acceptance. Only after written-design approval may the detailed implementation plan be produced. Product implementation starts after that plan is reviewed and its execution method selected. No deployment, merge or external-service change follows automatically from these approvals.
