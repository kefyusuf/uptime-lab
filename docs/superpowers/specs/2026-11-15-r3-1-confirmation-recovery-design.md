# R3-1 Confirmation/Recovery Design

**Date:** 2026-11-15
**Status:** Proposed written design; awaiting user review
**Verified baseline:** `main@1431965`, after #90/#91/#92/#93/#94/#95
**Scope:** [R3-1 Confirmation/Recovery scope reassessment](2026-11-15-r3-1-confirmation-recovery-scope-reassessment.md), approved by the user on 2026-11-15 (approach 1 derive-from-retained-evidence; N=3 / M=2; below-threshold `unknown/awaiting_confirmation` and `unavailable/recovering`; inconclusive reset; retention-bounded lookback).
**Authorization:** Documentation only; no runtime implementation, migration, merge or deployment.

## Decision summary

The availability assessment stops classifying a single latest observation and instead classifies a bounded newest-first sequence of terminal target outcomes. Confirmation gates only the down transition (N consecutive failures); recovery gates only the up-after-confirmed-outage transition (M consecutive successes); a first success without a preceding confirmed outage is available immediately. The sequence is read from already-durable retained `monitoring.check_runs` in one bounded read; there is no new table, column, migration, write path, result kind, public operation, or Rust/internal-contract change. The reason vocabulary grows by exactly two: `awaiting_confirmation` (unknown) and `recovering` (unavailable).

## Domain policy

`domain.EvaluateAvailability` changes from a single `*AvailabilityObservation` to a newest-first `[]AvailabilityObservation` (empty = no terminal result). Thresholds are domain constants `confirmationFailureThreshold = 3` and `recoverySuccessThreshold = 2` (fixed local conventions, R0-03; not per-Monitor policy).

Classification per observation reuses the existing outcome mapping: success = `http_response` 2xx; failure = `http_response` non-2xx plus `dns_error`/`timeout`/`connect_error`/`tls_error`/`protocol_error`; inconclusive = `policy_rejected`/`internal_error`/`worker_timeout`.

Algorithm (pure, no I/O):

1. Validate `evaluatedAt`; empty sequence -> `unknown/no_result`.
2. Validate every observation the algorithm inspects (the latest for precedence, and each row walked for the streak) for kind/status/`CompletedAt` invariants exactly as today; any invalid row -> `ErrInvalidAvailability`. Freshness (future/stale) is evaluated only on the latest observation, never on older rows.
3. Freshness and inconclusive precedence on the latest observation are unchanged and short-circuit: `future_result` / `stale_result` (>120s) -> `unknown` with that reason; inconclusive latest -> `unknown/policy_rejected` or `unknown/execution_failure`. The streak is irrelevant in these cases.
4. Otherwise the latest is a target outcome. Walk newest-first, stopping at the first inconclusive observation (the reset boundary) or the end of the sequence, and compute `leadingFailures` and `leadingSuccesses` (consecutive runs from index 0).
5. Decide:
   - `leadingFailures >= N` -> `unavailable` with the latest failure reason (`unexpected_http_status` / `probe_failure`).
   - `0 < leadingFailures < N` -> `unknown/awaiting_confirmation`.
   - `leadingSuccesses >= 1`: inspect the older segment immediately after the leading success run; count its opening consecutive failure run (`priorFailures`).
     - `priorFailures >= N` (a confirmed outage immediately precedes the recovery): `leadingSuccesses >= M` -> `available/successful_response`; else `unavailable/recovering`.
     - `priorFailures < N` (no confirmed outage precedes): `available/successful_response` immediately.
6. Evidence always references the latest terminal observation (sequence index 0), unchanged.

The inconclusive reset boundary means a `policy_rejected`/`internal_error`/`worker_timeout` row both yields unknown precedence when latest and truncates streak counting when older, so neither outage nor recovery can be confirmed across an unobservable interval.

## Persistence read

Add one bounded read, reusing the established latest-terminal lateral and its three-key tie-break:

- New port method `RecentTerminalByMonitorID(ctx, monitorID, limit) ([]ports.LatestCheckResultRecord, error)` on the latest-check-result repository interface, implemented alongside `LatestTerminalByMonitorID` (same `LEFT JOIN LATERAL ... WHERE completed_at IS NOT NULL ORDER BY completed_at DESC, issued_at DESC, id DESC LIMIT $2` shape, `LIMIT 1` generalised to the bound).
- The three-key ordering (`completed_at DESC, issued_at DESC, id DESC`) is reused deliberately so sequence index 0 is exactly the row `LatestTerminalByMonitorID` returns; evidence and the raw latest-result route stay consistent under `completed_at` ties. The history read's two-key ordering (`completed_at DESC, id DESC`) is intentionally not reused here.
- `limit = N + M` (5). Worst case needed is `(M-1)` leading successes plus `N` older failures (=4) or `N` leading failures (=3); 5 is a safe bound and is far below the newest-50 retention window. The partial index `check_runs_latest_terminal_idx (monitor_id, completed_at DESC) WHERE completed_at IS NOT NULL` serves the filter and primary ordering; the `issued_at`/`id` tie-break is a bounded in-memory sort over `completed_at` ties.
- Monitor absent -> `ports.ErrMonitorNotFound`; monitor present with no terminal rows -> empty slice (not an error). Incomplete terminal rows map to the same sanitized persistence error as today.
- `LatestTerminalByMonitorID` (single row) is unchanged and continues to serve the raw latest-result route.
- One Read Committed SELECT; no new write path, no row locks, no migration. The read is a single-statement snapshot; concurrent claim completions can change the streak between refreshes (the existing snapshot rule).

## Use case

`application.GetMonitorAvailability` switches from the single-row read to `RecentTerminalByMonitorID(ctx, id, N+M)`, maps records to newest-first `[]domain.AvailabilityObservation` (same validation and timestamp-safety checks as today), takes evidence from index 0, and calls the sequence-based `domain.EvaluateAvailability`. `ErrMonitorNotFound` and persistence-error mapping are unchanged; `evaluatedAt` still comes from the injected clock. No mutation is introduced.

## Contract, HTTP and gateway

- `contracts/openapi/public.yaml`: `UnavailableMonitorAvailability.reason` enum gains `recovering`; `UnknownMonitorAvailability.reason` enum gains `awaiting_confirmation`. Both variants already require `evidence`, which both new reasons carry. `AvailableMonitorAvailability`, `NoResultMonitorAvailability`, the status vocabulary, the closed shape, and all other operations are unchanged. Regenerate the TypeScript consumer and keep generated parity.
- HTTP adapter: reason passthrough only; `Cache-Control: no-store` on every matched response, `HEAD` 405 with `Allow: GET`, unknown Monitor 404, and sanitized errors are unchanged.
- Gateway: unchanged; availability is already a forwarded read with the existing Host/Origin guards.

## Web

- `entities/monitor/decode.ts`: accept `recovering` under `unavailable` and `awaiting_confirmation` under `unknown` (both with evidence); reject any other status/reason pairing as today.
- `entities/monitor/AvailabilityCard.tsx`: extend the exhaustive `reasons` map with human-readable strings for the two new reasons (e.g. awaiting confirmation: "Failures have not yet reached the confirmation threshold."; recovering: "The target is succeeding again but has not yet reached the recovery threshold.").
- `shared/api/public.generated.ts`: regenerated from the updated contract.
- No new controls; refresh recomputes the assessment; the existing snapshot/uncertainty messaging is unchanged.

## Interactions and documented limitations

- **Retention-bounded lookback:** the streak and the prior-outage run are counted only within the bounded read (and thus within newest-50). If retention truncates before N failures, confirmation is unreachable and the assessment stays `awaiting_confirmation`; a truncated prior run counts as `< N` (no confirmed outage). Documented, not a schema change.
- **Pause:** no new claims while paused, so the streak freezes; a prolonged pause lets the >120s staleness rule dominate (`unknown/stale_result`).
- **Target edit:** history mixes targets, so a streak may span an edit (mirrors the history-mixing limitation); no per-row target field is added.
- **Delete:** removes the evidence; the assessment returns `unknown/no_result`.
- **In-flight claims:** already-claimed work completes and appends terminal rows under the existing rules; the assessment simply observes them on the next refresh.

## Test strategy (TDD gates)

- Domain RED/GREEN: threshold boundaries (N-1/N failures, M-1/M successes), immediate first success, recovery gating after a confirmed outage, inconclusive latest precedence, inconclusive reset boundary, future/stale/no_result precedence, invalid inputs, retention-truncated prior run.
- Real PostgreSQL RED/GREEN: recent-terminal ordering and three-key tie-break, limit bound, monitor-not-found vs empty, incomplete-row sanitization, retention truncation preventing confirmation, pause-frozen streak, edit mixing, delete -> no_result, single-statement snapshot under concurrent completion; mutation checks (remove the limit / reorder the tie-break / drop the inconclusive reset each reproduce a specific failure).
- Contract RED/GREEN: extended enums, unchanged closed shapes and other operations, generated parity, pinned Redocly lint/bundle.
- HTTP/gateway RED/GREEN: new reasons serialized, unchanged no-store/Allow/HEAD/404/sanitization.
- Web RED/GREEN: decode accepts the two new pairings and rejects mismatches; AvailabilityCard renders them; refresh recomputes.
- Composed Chromium on 4173/4817: a single transient failure shows `awaiting_confirmation` (not outage); sustained failures confirm `unavailable`; sustained successes show `recovering` then `available`; across an API restart.
- Required repository, contract, Go/race/real-PostgreSQL, Rust regression/audit, Web, Docker/browser and aggregate CI at the final implementation head; exact-main evidence after an authorized merge.

## Exclusions

No durable incident open/resolve state, no notification delivery, no uptime percentages, no maintenance windows, no per-Monitor configurable thresholds, no aggregation, no ownership or remote access, no new migration or result kind. Durable incident state remains a later-slice design decision.

## Evidence pointers

- [Scope reassessment](2026-11-15-r3-1-confirmation-recovery-scope-reassessment.md), [current handoff](../../devops/current-handoff.md), [R0 brief](../../roadmap/2026-10-18-r0-product-release-decision-brief.md).
- `apps/api/internal/modules/monitoring/domain/availability.go`: the policy to extend to a sequence.
- `apps/api/internal/modules/monitoring/application/get_monitor_availability.go`: the use case to switch to the bounded recent read.
- `apps/api/internal/modules/monitoring/adapters/postgres/latest_check_result.go`: the lateral/tie-break to generalise.
- `apps/api/internal/modules/monitoring/adapters/postgres/monitor_history.go`: the two-key history ordering deliberately not reused.
- `apps/api/migrations/00002_create_monitoring_check_runs.sql`: `check_runs_latest_terminal_idx`, the partial index serving the bounded read.
- `apps/api/internal/modules/monitoring/adapters/postgres/retention.go`: the newest-50 window bounding lookback.
- `contracts/openapi/public.yaml`: the availability reason enums to extend.
- `apps/web/src/entities/monitor/decode.ts` and `AvailabilityCard.tsx`: the browser decode/render touchpoints.