# R3-2 Durable Incident Semantics Design

**Date:** 2026-11-16
**Status:** Proposed written design; awaiting user review
**Verified baseline:** `main@b3e09bf`, after R3-1 (#98), its handoff reconciliation (#99) and the R3-2 scope proposal (#100)
**Scope:** [R3-2 Durable Incident Semantics scope reassessment](2026-11-16-r3-2-durable-incident-semantics-scope-reassessment.md), approved by the user on 2026-11-16 (full vertical slice including the public read; approach 1 durable counters + incident rows; availability read unchanged; notification deferred to R4).
**Authorization:** Documentation only; no runtime implementation, migration, merge or deployment.

## Decision summary

Confirmed outages are materialized into durable incident open/resolve state, maintained on the Go write path and defined separately from the unchanged evidence-derived availability read (R0 incident-separation mandate). A per-Monitor confirmation fold (consecutive failure/success counters plus an open-incident pointer) lives in a new `monitoring.incident_state` table; distinct outages accrue as rows in a new `monitoring.incidents` table. Every terminalization folds the outcome: N=3 consecutive target-failures open an incident, M=2 consecutive target-successes resolve the open incident, and an inconclusive result resets both counters without resolving. Confirmation becomes unbounded by evidence retention and survives restart. One new bounded public read `GET /monitors/{monitorId}/incidents` exposes incident history with the check-run history cursor/budget conventions and a distinct version-3 cursor. The availability read, the ten existing operations, the result-kind vocabulary, Rust and the internal contract are unchanged. One new migration (00005); 00001–00004 stay immutable.

## Design decisions resolved

The scope flagged the hook, state shape, serialization, coherence rule and item field set as design decisions. This design resolves them:

1. **Approach 1 confirmed.** Durable counters + incident rows on the write path; confirmation is unbounded (counters persist regardless of retention). Approach 2 (re-derive from retained evidence) keeps the R3-1 bound and is rejected; approach 3 (fold into availability) is rejected by R0.
2. **Hook: every terminalization path.** `CompleteCheck` folds its normal-completion and late-timeout outcomes; `ClaimDueCheck` reconciliation folds its expired-pending `worker_timeout` outcomes as a bulk inconclusive reset. Idempotent-duplicate, conflict and not-found paths do not terminalize and therefore do not fold (preserving completion idempotency).
3. **State shape: two new tables, not columns on `monitors`.** A per-Monitor `incident_state` fold row plus an `incidents` history table. Counters on `monitors` are rejected because `CompleteCheck` would then lock the `monitors` row, creating a lock-order cycle with `ClaimDueCheck` (see Write path).
4. **Serialization: the existing CheckRun row lock, no new Monitor lock.** The one-pending-per-Monitor invariant plus the `FOR UPDATE` CheckRun lock already serialize per-Monitor terminalizations; the fold reads-modifies-writes `incident_state` inside that same transaction. Adding a `monitors` lock to `CompleteCheck` would deadlock against `ClaimDueCheck`'s Monitor-then-pending-INSERT order and is explicitly avoided.
5. **Incident identity is application-assigned.** `SubmitCheckResult` mints a candidate `IncidentID` per completion and passes it through `CompleteCheckInput`; the adapter consumes it only when the policy opens an incident. `Repository` stays pool-only (`NewRepository(pool)` unchanged), mirroring Monitor/CheckRun identity discipline. Reconciliation never opens an incident, so it needs no candidate id.
6. **Item field set:** `incidentId`, `openedAt`, `resolvedAt` (null while open), `openingReason` (`unexpected_http_status`/`probe_failure`, durable), `openedByCheckId` and `resolvedByCheckId` (nullable evidence links). Open/resolved state is derived from `resolvedAt` presence; no redundant state enum. Evidence links use `ON DELETE SET NULL` so retention clears them without losing the durable incident window. A window-only minimal shape is the lighter alternative if the user prefers to trim evidence enrichment.
7. **Coherence rule:** availability and incidents are separate projections of the same terminal evidence sharing identical transition semantics, but over different windows (availability: bounded newest-5 plus latest-only freshness; incidents: durable unbounded fold, no freshness rule). Divergence under truncation/long pause is intentional and documented, not silently coupled.
8. **Read ordering and cursor:** newest-first by `opened_at DESC, id DESC`; a distinct version-3 keyset cursor (`3|` prefix) rejecting inventory (`1|`) and history (`2|`) tokens.
9. **Edit/delete/pause:** a target edit does not touch incident state (a streak may span an edit, documented mixing); delete sweeps incidents and state alongside CheckRuns; pause freezes the fold while an open incident persists.

**Scope drift corrected here:** the scope's "readiness set stays at four" was carried over from R3-1 (which added no migration). `migrations.CompatibilityChecker` derives its expected version set dynamically from `provider.ListSources()`, so 00005 is automatically included: `/readyz` still requires goose version 0 plus every embedded migration applied, but the expected set grows from four to five. The mechanism is unchanged; no test hardcodes the count (verified). A follow-up reconciliation should fix the scope wording.

## Domain policy

New pure domain policy, no I/O:

- `domain.IncidentID`: a UUID value object mirroring `CheckID`/`MonitorID` (`NewIncidentID`, `ParseIncidentID`, `String`, `UUID`).
- `domain.IncidentState`: `FailureStreak int`, `SuccessStreak int`, `OpenIncidentID *IncidentID`; a zeroed state is the no-prior-outage start.
- `domain.IncidentReason`: `unexpected_http_status` / `probe_failure` (the failure classification captured at open).
- Shared outcome classification: extract the kind→outcome mapping currently private in `availability.go` (`classifyAvailabilityObservation`) into a shared classifier returning success/failure/inconclusive plus the failure reason, used by both `EvaluateAvailability` and the incident fold. This is a behavior-preserving refactor; the existing R3-1 availability suite is the regression guard guaranteeing identical semantics. Success = `http_response` 2xx; failure = `http_response` non-2xx plus `dns_error`/`timeout`/`connect_error`/`tls_error`/`protocol_error`; inconclusive = `policy_rejected`/`internal_error`/`worker_timeout`.
- `domain.ApplyIncidentOutcome(state, outcome, reason, at, checkID, candidateID) (IncidentState, IncidentAction)`:
  - **inconclusive:** `FailureStreak=0`, `SuccessStreak=0`, `OpenIncidentID` unchanged; action none.
  - **success:** `FailureStreak=0`, `SuccessStreak++`; if open and `SuccessStreak >= M` -> action resolve(`OpenIncidentID`, at, checkID), `OpenIncidentID=nil`, `SuccessStreak=0`; else action none.
  - **failure:** `SuccessStreak=0`, `FailureStreak++`; if not open and `FailureStreak >= N` -> action open(candidateID, at, reason, checkID), `OpenIncidentID=candidateID`; else action none.
  - Thresholds reuse the R3-1 domain constants `confirmationFailureThreshold=3` / `recoverySuccessThreshold=2` (fixed local conventions, R0-03). A first success with no open incident is action none (no confirmation gate on the up transition); an already-open incident is never re-opened.

`IncidentAction` is none / open / resolve; the adapter persists it. The policy is total and deterministic so the durable fold and the evidence-derived assessment agree on every transition.

## Persistence schema (migration 00005)

`apps/api/migrations/00005_create_monitoring_incidents.sql`, goose Up/Down, additive only:

- `monitoring.incidents`: `id uuid PRIMARY KEY`, `monitor_id uuid NOT NULL REFERENCES monitoring.monitors(id)`, `opened_at timestamptz NOT NULL`, `resolved_at timestamptz NULL`, `opening_reason text NOT NULL` (CHECK in the two reasons), `opened_by_check_id uuid NULL REFERENCES monitoring.check_runs(id) ON DELETE SET NULL`, `resolved_by_check_id uuid NULL REFERENCES monitoring.check_runs(id) ON DELETE SET NULL`; CHECK `resolved_at IS NULL OR resolved_at >= opened_at`.
- `incidents_one_open_per_monitor_idx`: partial unique index `ON monitoring.incidents (monitor_id) WHERE resolved_at IS NULL`, mirroring `check_runs_one_pending_per_monitor_idx` to enforce at most one open incident per Monitor.
- `incidents_monitor_opened_idx`: `ON monitoring.incidents (monitor_id, opened_at DESC, id DESC)` serving the keyset read.
- `monitoring.incident_state`: `monitor_id uuid PRIMARY KEY REFERENCES monitoring.monitors(id)`, `failure_streak integer NOT NULL DEFAULT 0` (CHECK >= 0), `success_streak integer NOT NULL DEFAULT 0` (CHECK >= 0), `open_incident_id uuid NULL REFERENCES monitoring.incidents(id)`. Created after `incidents` (no FK cycle: `incidents` never references `incident_state`).
- Down drops `incident_state` then `incidents`. The `monitor_id` FKs have no cascade (deletion sweeps manually, mirroring CheckRuns); the evidence FKs use `ON DELETE SET NULL` (retention-safe, DB-enforced, no retention-adapter change).
- No data backfill: the migration creates tables only. State rows accrue lazily from the first completion forward; pre-migration Monitors start zeroed.

## Write path

**`CompleteCheck`** (`adapters/postgres/check_execution.go`): the locking `SELECT` additionally fetches `monitor_id`. After a path that actually terminalizes a previously-pending run (normal completion, or late `worker_timeout`), inside the same transaction and still holding the CheckRun `FOR UPDATE` lock:

1. Read `incident_state` for `monitor_id`; absent -> zeroed state (lazy init, no backfill).
2. Classify the terminalized outcome with the shared classifier (late timeout -> inconclusive).
3. `domain.ApplyIncidentOutcome` -> new state + action, using the application-supplied candidate `IncidentID`.
4. Upsert `incident_state` (`INSERT ... ON CONFLICT (monitor_id) DO UPDATE`).
5. open -> `INSERT incidents` (candidate id, monitor_id, opened_at=completedAt, opening_reason, opened_by_check_id=checkID, resolved_at NULL); resolve -> `UPDATE incidents SET resolved_at=completedAt, resolved_by_check_id=checkID WHERE id=openIncidentID`.

The duplicate-match, conflict and not-found paths return without folding, so an idempotent re-delivery never double-counts. `CompleteCheck` keeps default isolation and never locks `monitors`.

**`ClaimDueCheck` reconciliation** (same file): the expired-pending `UPDATE ... SET result_kind='worker_timeout'` additionally resets the fold for exactly the monitors it terminalized — `failure_streak=0`, `success_streak=0`, `open_incident_id` unchanged (inconclusive never resolves). Implementation captures the reconciled `monitor_id`s (`UPDATE ... RETURNING monitor_id`) and bulk-resets their `incident_state`, mirroring the pipelined-batch write discipline; exact SQL is plan-phase detail. Reconciliation runs before retention and candidate selection, unchanged in position.

**Serialization and lock order (real-PostgreSQL race evidence required):** both writers acquire locks in the order CheckRun -> incident_state -> incidents; only `ClaimDueCheck` then locks `monitors`, and it does so with `FOR UPDATE ... SKIP LOCKED` (never waits). `CompleteCheck` never locks `monitors`, so it never waits on a claim's monitor lock. The one-pending unique index means a claim's pending INSERT for M can only conflict with M's single pending run, which `CompleteCheck` holds while terminalizing; because `CompleteCheck` wants only `incident_state` (not `monitors`), it completes and releases the run, letting the INSERT proceed — no cycle. Per-Monitor, reconciliation-reset and completion-fold are mutually exclusive per run (the CheckRun row lock decides which terminalizes), so the fold is never lost or doubled. Parking a run-row lock and observing `pg_locks` waits is the tracer proof, mirroring the history delete-race evidence.

## Persistence read

New `adapters/postgres/monitor_incidents.go` `Page`, mirroring `monitor_history.go` exactly: one Read Committed transaction, a `pgx.Batch` existence probe (`monitors`) plus keyset page (`incidents`), binary uuid binding (`pgtype.UUID`), `maxRows` 1..51 (limit+1 lookahead). First page `ORDER BY opened_at DESC, id DESC LIMIT $2`; anchored page adds `(opened_at, id) < ($2, $3)`. Monitor absent -> `ports.ErrMonitorNotFound`; present with no incidents -> empty slice (successful empty page). The probe-first batch ordering gives the same consistent pre-delete-page-or-404 snapshot rule as history. New port `MonitorIncidentRepository.Page(ctx, monitorID, *IncidentAnchor, int) ([]IncidentRecord, error)` with `IncidentAnchor{OpenedAt time.Time, IncidentID domain.IncidentID}`.

## Use cases and composition

- `application.GetMonitorIncidents` mirrors `GetMonitorHistory`: validate `MonitorID`, limit 1..50, decode the version-3 cursor, call `Page(limit+1)`, map records to a closed incident item shape, sanitize invalid storage rows to `ErrPersistence`; `ErrMonitorNotFound` mapping unchanged.
- `application/incident_cursor.go` mirrors `history_cursor.go`: raw `"3|" + openedAt(microsecond layout) + "|" + incidentID`, 66 chars, 88-char unpadded base64url, strict canonical re-encode check; the `3|` prefix rejects inventory/history tokens.
- `application.SubmitCheckResult` gains an `IncidentIDGenerator` dependency and mints a candidate `IncidentID` per completion, passed via `ports.CompleteCheckInput.IncidentID`. `ClaimDueCheck` is unchanged at the application layer (reconciliation reset needs no id).
- `module.go`: `Module` gains `GetMonitorIncidents`; `NewModuleWithIncidents(...)` extends `NewModuleWithTargetEdit`. `NewExecutionModule` gains the incident id generator for `SubmitCheckResult`. `cmd/api composeMonitoring` wires the incident repository (the same pool-only `Repository`) and the generator, mirroring the existing check-id generator.
- `ports.CompleteCheckInput` gains `IncidentID domain.IncidentID`; the `CheckExecutionRepository` method set is unchanged. New `MonitorIncidentRepository` port. `Repository` construction is unchanged.

## Contract, HTTP and gateway

- `contracts/openapi/public.yaml`: add `/monitors/{monitorId}/incidents` GET (`getMonitorIncidents`), `x-max-body-bytes: 245760`, `monitorId` path param, `limit` (1..50, default 20) and `cursor` (88-char base64url, version 3) query params, responses 200 `MonitorIncidentPage` / 400,404,500 `Problem` / 405 `Allow: GET` with no body, every matched response `no-store`. New closed schemas `MonitorIncidentPage{items,nextCursor}` and `MonitorIncident{incidentId,openedAt,resolvedAt?,openingReason,openedByCheckId?,resolvedByCheckId?}` (`additionalProperties: false`; optional fields omitted when null, mirroring history's `httpStatus`/`durationMs`). The availability operation, its reason enums and all ten existing operations are unchanged. Regenerate the TypeScript consumer and keep generated parity.
- `adapters/http/incidents.go` `serveIncidents` mirrors `serveHistory`: exact path match, `ContentLength` 0 / no `TransferEncoding`, 128-byte ASCII query with only `limit`/`cursor` each once, GET-only else 405 `Allow: GET`, `no-store` on every response, 400 invalid query/UUID, 404 unknown Monitor, 500 sanitized. `encodeIncidentPage` mirrors `encodeHistoryPage` (128-byte base overhead, per-item cost plus comma, 245760-byte budget, lookahead-driven `nextCursor`). Route registration mirrors history in `handler.go`.
- Gateway: the incident read is a forwarded GET through the restricted gateway with the existing Host/Origin guards; no new mutation route, browser controls unchanged.

## Web

- `shared/api/public.generated.ts` regenerated from the contract.
- `entities/monitor`: a closed incident-page/incident decoder accepting the new shape and rejecting mismatches (mirroring the history decoder), and an incident-history section component rendering each window (opened/resolved instants, an open badge when `resolvedAt` is absent, opening reason, duration) plus current incident state on the detail page.
- The detail page gains the incident-history section beside the existing history section; `AvailabilityCard` and the availability decoder are unchanged.
- Contract fixtures cover the incident shapes, open vs resolved, null evidence links and invalid responses; refresh recomputes; no new mutation controls.

## Availability/incident coherence and divergence

Both projections share the identical success/failure/inconclusive classification and N/M thresholds, so they agree on every transition when observing the same evidence. They differ in window and freshness:

- Availability is the unchanged R3-1 bounded newest-5 read with latest-only future/stale (>120s) precedence; it asserts nothing durable.
- Incidents are a durable unbounded fold with no freshness rule; an open incident persists across pause, restart and retention.

Documented divergences: a long pause makes availability `unknown/stale_result` while an incident stays durably open; retention can remove the confirming/resolving CheckRun evidence (clearing the nullable evidence links via `ON DELETE SET NULL`) while the incident window persists; availability can "forget" a confirmed outage once its failure evidence is retained away and replaced, while incident history durably records it. This separation is the R0 mandate, not a coupling bug; the incident read is never derived from the availability assessment.

## Interactions and documented limitations

- **Retention:** counters are unbounded and survive truncation; incident windows are durable. Evidence links degrade to null when their CheckRun is retained away. The availability read stays bounded (unchanged).
- **Pause:** no new claims/completions, so the fold freezes; an open incident persists; availability staleness may dominate separately.
- **Target edit:** `EditTarget` does not touch incident state; a streak/incident may span a target change (documented mixing mirroring history/streak mixing). An edit neither resolves nor annotates an open incident.
- **Delete:** the deletion batch additionally sweeps `incidents` and `incident_state` for the Monitor (before the lock and again under it, mirroring the CheckRun double-sweep); deletion is irreversible and removes incident evidence. Availability returns `no_result`; the incident read returns 404 (Monitor gone).
- **Restart:** incident state and history are durable; the fold resumes from the persisted counters. No in-memory state.
- **No backfill:** incidents accrue from the migration forward; existing retained evidence is not replayed.
- **In-flight claims:** already-claimed work completes and folds under the same rules; the read observes the result on the next refresh.

## Test strategy (TDD gates)

- Migration RED/GREEN: 00005 up/down, immutable 00001–00004, schema/CHECK shapes, the one-open partial unique index, evidence FK `ON DELETE SET NULL`, no FK cycle, goose up/down and the dynamic compatibility checker accepting the five-migration set.
- Domain RED/GREEN: `ApplyIncidentOutcome` boundaries (N-1/N open, M-1/M resolve), inconclusive reset (open persists), immediate first success, no re-open while open, counter resets, shared-classifier parity with R3-1; mutation checks (drop the reset / lower a threshold / re-open while open each reproduce a specific failure). The existing availability suite stays green (behavior-preserving refactor guard).
- Real PostgreSQL RED/GREEN: open/resolve on normal completion; late-timeout and reconciliation inconclusive reset; idempotent duplicate does not re-fold; unbounded confirmation surviving retention truncation; one-open invariant under concurrent completion; per-Monitor serialization and absence of the Monitor-lock deadlock (parked-lock + `pg_locks` tracer); evidence-link SET NULL on retention; delete sweep of incidents+state; edit mixing; pause freeze; durability across restart; lazy zeroed init / no backfill; incident read keyset ordering, cursor canonicality, limit/lookahead, monitor-not-found vs empty page, snapshot under concurrent completion; absence of any change to the availability read path.
- Contract RED/GREEN: the new operation/schemas/version-3 cursor, unchanged availability and existing operations, generated parity, pinned Redocly lint/bundle, query/byte-budget and `no-store`/`Allow`/`HEAD` rules.
- HTTP/gateway RED/GREEN: incident read serialized through the gateway, query-grammar/cursor/budget rejection, sanitized errors, unchanged `no-store`/`Allow`/`HEAD`/404.
- Web RED/GREEN: incident decoder accepts the closed shape and rejects mismatches; the history section renders open vs resolved distinctly; refresh recomputes; no new controls.
- Composed Chromium on 4173/4817: sustained failures open a durable incident, sustained successes resolve it, incident history is visible and survives an API restart and evidence retention truncation, and the availability read remains the separate evidence-derived assessment.
- Required repository, contract, Go/race/real-PostgreSQL, Web, Docker/browser and aggregate CI at the final implementation head (checker/internal-contract path-skipped: Rust and the internal contract unchanged); exact-main evidence after an authorized merge.

## Exclusions

No notification delivery (R4), no uptime percentages, no maintenance windows, no per-Monitor configurable thresholds, no aggregation, no ownership or remote access, no change to the availability read semantics (R3-1), no new result kinds, no Rust or internal-contract change, no historical backfill. The external monitoring beta NO-GO judgment is unchanged.

## Evidence pointers

- [Scope reassessment](2026-11-16-r3-2-durable-incident-semantics-scope-reassessment.md), [R3-1 design](2026-11-15-r3-1-confirmation-recovery-design.md), [current handoff](../../devops/current-handoff.md), [R0 brief](../../roadmap/2026-10-18-r0-product-release-decision-brief.md) (R0-04/R0-09 and the incident-separation mandate).
- `apps/api/internal/modules/monitoring/adapters/postgres/check_execution.go`: `ClaimDueCheck` (Read Committed reconciliation + `retainHistoryWindow` + Monitor `FOR UPDATE SKIP LOCKED` + pending INSERT) and `CompleteCheck` (CheckRun row-lock terminalization, default isolation) — the two fold hooks and the lock-order analysis.
- `apps/api/internal/modules/monitoring/adapters/postgres/retention.go`: the newest-50 window and pipelined-batch write discipline; evidence FK `ON DELETE SET NULL` keeps retention unchanged.
- `apps/api/internal/modules/monitoring/domain/availability.go`: the R3-1 sequence policy and private outcome classifier to share; thresholds `confirmationFailureThreshold`/`recoverySuccessThreshold`.
- `apps/api/internal/modules/monitoring/domain/check_result.go`: the result-kind vocabulary.
- `apps/api/internal/modules/monitoring/adapters/postgres/monitor_history.go`, `application/history_cursor.go`, `application/get_monitor_history.go`, `adapters/http/history.go`: the bounded keyset read, version-2 cursor, limit/lookahead and HTTP budget/grammar conventions the incident read mirrors with version 3.
- `apps/api/internal/modules/monitoring/adapters/postgres/monitor_deletion.go`: the manual child double-sweep to extend to incidents+state.
- `apps/api/internal/modules/monitoring/adapters/postgres/monitor_target_edit.go`: the edit that leaves incident state untouched (mixing).
- `apps/api/internal/modules/monitoring/adapters/postgres/repository.go`: pool-only `NewRepository`, application-assigned identity discipline.
- `apps/api/internal/modules/monitoring/application/check_execution.go`, `ports/check_execution_repository.go`, `module.go`: `SubmitCheckResult`/`CompleteCheckInput` candidate-id wiring and the progressive module constructors.
- `apps/api/migrations/00002_create_monitoring_check_runs.sql`: the partial unique index pattern; `apps/api/migrations/compatibility.go`: the dynamic expected-version set (readiness grows four->five).
- `contracts/openapi/public.yaml`: the `/monitors/{monitorId}/history` operation to mirror and the unchanged availability operation.