# R3-1 Confirmation/Recovery Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the current availability assessment confirmation/recovery-aware without durable incident state: N=3 consecutive terminal failures confirm `unavailable`, M=2 consecutive successes recover to `available`, a first success with no preceding confirmed outage is available immediately, and below-threshold states surface as `unknown/awaiting_confirmation` and `unavailable/recovering`. The assessment derives from a bounded newest-first read of already-retained terminal evidence; no new persistence, migration, write path, result kind, public operation or Rust/internal-contract change.

**Architecture:** Go owns the sequence-based domain policy (`EvaluateAvailability` over `[]AvailabilityObservation`), one bounded recent-terminal read (`RecentTerminalByMonitorID`, limit N+M) reusing the latest-result three-key ordering so evidence index 0 equals the raw latest-result row, and the unchanged availability operation. The HTTP adapter is reason passthrough (no production change); composition is unchanged because the use case keeps the same repository interface, which gains one method. The browser extends the closed `Availability` union, decoder and reason copy. The gateway is unchanged (availability is already a forwarded read). Rust, the internal contract and the migration set are unchanged.

**Tech Stack:** Existing Go1.27.3/pgx/goose, PostgreSQL18, OpenAPI3.1.2, React/TypeScript/Vitest/Playwright, Node24.21.0/npm11.19.1. No new dependencies, no new migration.

**Spec:** [R3-1 Confirmation/Recovery design](../specs/2026-11-15-r3-1-confirmation-recovery-design.md), merged through PR #96 (`main@30017f8`), stacking the [scope](../specs/2026-11-15-r3-1-confirmation-recovery-scope-reassessment.md) merged through PR #95 (`main@1431965`). The user approved the five scope decisions and the written design, then authorized preparing this plan.

**Status:** Plan awaiting review; no product task is executed. Preserve the prior Native execution method unless the user selects another method: this session implements the dependent tasks, then a fresh whole-branch reviewer checks the completed change. Plan approval is distinct from merge/deployment approval.

## Global Constraints

- Assessment-only: no incident open/resolve rows, no notification, no uptime percentage, no maintenance window, no per-Monitor configurable threshold, no aggregation, no ownership/remote access. Thresholds are fixed domain constants `confirmationFailureThreshold = 3`, `recoverySuccessThreshold = 2` (R0-03 local conventions).
- Reason vocabulary grows by exactly two: `awaiting_confirmation` (unknown) and `recovering` (unavailable). Both carry evidence. The status vocabulary, the closed availability shape, `AvailableMonitorAvailability`, `NoResultMonitorAvailability` and every other public operation/schema are unchanged.
- Outcome mapping is reused verbatim: success = `http_response` 2xx; failure = `http_response` non-2xx plus `dns_error`/`timeout`/`connect_error`/`tls_error`/`protocol_error`; inconclusive = `policy_rejected`/`internal_error`/`worker_timeout`. No new result kind.
- Latest-observation precedence is preserved: invalid input -> `ErrInvalidAvailability`; empty sequence -> `unknown/no_result`; `CompletedAt.After(evaluatedAt)` -> `unknown/future_result`; age > 120s -> `unknown/stale_result`; inconclusive latest -> `unknown/policy_rejected` or `unknown/execution_failure`. Freshness applies only to the latest observation, never to older rows.
- Inconclusive reset: an inconclusive row yields unknown precedence when latest and truncates streak counting when older, so neither outage nor recovery confirms across an unobservable interval.
- Bounded read: `RecentTerminalByMonitorID(ctx, id, limit)` reuses the latest-result lateral `ORDER BY completed_at DESC, issued_at DESC, id DESC` (three keys) so index 0 equals `LatestTerminalByMonitorID`; `limit = N+M = 5`; served by `check_runs_latest_terminal_idx (monitor_id, completed_at DESC) WHERE completed_at IS NOT NULL`. The history read's two-key ordering is not reused. Monitor absent -> `ports.ErrMonitorNotFound`; present with no terminal rows -> empty slice. `LatestTerminalByMonitorID` and the raw latest-result route are unchanged.
- One Read Committed SELECT; no new write path, row locks or migration. The read is a single-statement snapshot; concurrent claim completions can change the streak between refreshes (existing snapshot rule). Retention-bounded lookback: streaks count only within the bounded read/newest-50; a truncated prior run counts as `< N`.
- Pause freezes the streak (no new claims); prolonged pause lets staleness dominate. Target edit may span a streak (history-mixing limitation; no per-row target field). Delete removes evidence -> `unknown/no_result`. In-flight claims complete and append terminal rows under existing rules.
- HTTP availability is reason passthrough (`assessment.Reason()`); `Cache-Control: no-store` on every matched response, `HEAD` 405 `Allow: GET`, unknown Monitor 404, malformed UUID 400 and sanitized 500 are unchanged. Gateway unchanged.
- No new migration; readiness set stays at four. Rust and `contracts/openapi/internal.yaml` unchanged. English code/docs/copy; concise Turkish before/after updates.
- Record focused RED reason, GREEN result and commit per task. Tool/setup failure is blocked verification, not product RED. Preserve user-owned changes; explicit paths only in commits. Allowed scopes: web, api, checker, contracts, devops, architecture, docs, deps.

## Review Focus

- The sequence policy cannot confirm an outage below N or recover below M: Task 3 pins N-1/N failure boundaries, M-1/M success boundaries, immediate first success, and recovery gating only after a confirmed prior outage.
- Inconclusive precedence and reset are not bypassed: Task 3 pins unknown precedence when latest and streak truncation when older; a mutation dropping the reset boundary must reproduce a specific failure.
- Evidence stays consistent with the raw latest-result route under `completed_at` ties: Task 2 pins index 0 == `LatestTerminalByMonitorID` and the three-key tie-break; reordering to the history two-key ordering must fail.
- The bounded read cannot over-fetch or mis-map absence: Task 2 pins the limit, monitor-not-found vs empty slice, and incomplete-row sanitization; removing the limit must reproduce a specific failure.
- The use case introduces no mutation and preserves error/clock mapping: Task 3 pins not-found/persistence translation, injected `evaluatedAt`, and evidence from index 0.
- New reasons serialize with unchanged transport guards: Task 4 pins `recovering`/`awaiting_confirmation` over actual TCP with no-store/Allow/HEAD/404/sanitization intact.
- The browser cannot accept a mismatched status/reason pairing or render an unknown reason: Task 5 pins closed decoding and the exhaustive reason map.
- Task 6's journey proves transient failure -> `awaiting_confirmation`, sustained failures -> `unavailable`, sustained successes -> `recovering` then `available`, across an API restart, with durable SQL linkage.

## File map and execution prerequisites

Go module paths in tasks are relative to `apps/api/internal/modules/monitoring`; Web paths are relative to `apps/web`. Root paths are written in full. `EvaluateAvailability` has exactly one production caller (`application/get_monitor_availability.go`); `ports.LatestCheckResultRepository` has exactly two implementers (the postgres `Repository` and the shared `fakeLatestCheckResultRepository` in `application/get_latest_check_result_test.go`), so the interface addition updates only that fake. Composition (`module.go`, `cmd/api/main.go`) is unchanged: the use case keeps the same interface and the `Repository` satisfies the added method. The recent-read adapter is a new file using the pipelined batch formulation if it issues multi-statement SQL (the Mimosa pre-write scan blocks variable-arg statements in new files); a single bounded SELECT may use `Query`. Pinned Go is 1.27.3 (`golang:1.27.3-alpine3.24`); Windows browser wrapper uses per-invocation `MSYS2_ARG_CONV_EXCL='/usr'`; real PostgreSQL uses `bash scripts/ci/run-go-postgres-tests.sh`; real browsers use `bash scripts/ci/run-web-browser-tests.sh` once per port. Local web validation must include `format:check`; hook implementations must not publish state synchronously inside effects.

Before runtime execution, verify clean/dirty state, applicable instructions, tools and actual Docker access. Keep the feature PR stacked on the approved docs baseline; never merge without approval.

No worktree or Docker resource is needed for writing this plan.

### Task 1: Availability reason enums and generated types

**Files:** Modify `contracts/openapi/public.yaml`, `scripts/ci/check-public-contract.mjs`, `scripts/ci/test-check-public-contract.mjs`; create `contracts/fixtures/public/availability-unavailable-recovering.json`, `contracts/fixtures/public/availability-unknown-awaiting-confirmation.json`; regenerate `apps/web/src/shared/api/public.generated.ts`.

**Interfaces:** `UnavailableMonitorAvailability.reason` enum gains `recovering`; `UnknownMonitorAvailability.reason` enum gains `awaiting_confirmation`. Both variants keep `additionalProperties: false` and required `evidence`. `AvailableMonitorAvailability`, `NoResultMonitorAvailability`, the status enum, the oneOf closure and all other operations are unchanged.

- [ ] Write mutation tests `unavailableReasonsIncludeRecovering`, `unknownReasonsIncludeAwaitingConfirmation`, `availabilityVariantsStayClosed`, `availabilityChangePreservesOtherOperations`: assert the two new reasons are accepted only in their own status variant with evidence, every other status/reason pairing is rejected, the closed shape and other operations are unchanged. Mutations adding a reason to the wrong variant, dropping required evidence or opening the shape must fail checking.
- [ ] Run root `node scripts/ci/test-check-public-contract.mjs`; record RED.
- [ ] Add only the two enum members and the two fixtures plus semantic assertions.
- [ ] Run semantic GREEN, pinned redocly lint/bundle and the Web `npm run generate:api`, `npm run check:generated`, `npm run typecheck` chain.
- [ ] Commit explicit contract/checker/generated files: `feat(contracts): extend availability reasons for confirmation/recovery`.

### Task 2: Bounded recent-terminal read

**Files:** Modify `ports/latest_check_result_repository.go`; create `adapters/postgres/recent_check_results.go`, `adapters/postgres/recent_check_results_integration_test.go`; modify the shared `fakeLatestCheckResultRepository` in `application/get_latest_check_result_test.go` to implement the added method. No use-case or composition change in this task.

**Interfaces:** `LatestCheckResultRepository` gains `RecentTerminalByMonitorID(context.Context, domain.MonitorID, int) ([]LatestCheckResultRecord, error)`. The `Repository` implements it with the latest-result lateral generalised to `LIMIT $2`, three-key ordering `completed_at DESC, issued_at DESC, id DESC`, `completed_at IS NOT NULL`, monitor existence probe mapping absence to `ports.ErrMonitorNotFound` and no terminal rows to an empty slice; incomplete terminal rows map to the same sanitized persistence error as `LatestTerminalByMonitorID`.

- [ ] Write real integration `TestRecentTerminalOrdersByThreeKeys`, `TestRecentTerminalRespectsLimit`, `TestRecentTerminalIndexZeroEqualsLatest`, `TestRecentTerminalUnknownMonitor`, `TestRecentTerminalEmptyWhenNoTerminalRows`, `TestRecentTerminalSanitizesIncompleteRow`, `TestRecentTerminalRetentionTruncatesLookback`: ordering and tie-break under equal `completed_at`; the limit bound; index 0 equals `LatestTerminalByMonitorID`; monitor-not-found vs empty slice; incomplete-row sanitization; retention truncation. A mutation removing the limit or reordering to the history two-key ordering must reproduce a specific failure.
- [ ] Run root `bash scripts/ci/run-go-postgres-tests.sh` on the actual RED head; require test failure for absent recent-read behavior.
- [ ] Implement the port method, adapter and shared fake update; no claim/completion/migration SQL changes; `LatestTerminalByMonitorID` unchanged.
- [ ] Repeat real SQL GREEN plus the existing latest-result/history/retention/claim/completion integration suites and `go vet ./...`.
- [ ] Commit explicit files: `feat(api): read bounded recent terminal check results`.

### Task 3: Sequence availability policy and use case

**Files:** Modify `domain/availability.go`, `domain/availability_test.go`, `application/get_monitor_availability.go`, `application/get_monitor_availability_test.go`.

**Interfaces:** `EvaluateAvailability(observations []AvailabilityObservation, evaluatedAt time.Time) (Availability, error)` (newest-first; empty = no result); constants `confirmationFailureThreshold = 3`, `recoverySuccessThreshold = 2`; reasons `AvailabilityAwaitingConfirmation AvailabilityReason = "awaiting_confirmation"`, `AvailabilityRecovering AvailabilityReason = "recovering"`. The use case calls `RecentTerminalByMonitorID(ctx, id, N+M)`, maps records to newest-first observations with the existing validation/timestamp-safety checks, takes evidence from index 0, and keeps `ErrMonitorNotFound`/persistence translation and the injected clock.

- [ ] Add `TestEvaluateAvailabilityConfirmationBoundary`, `TestEvaluateAvailabilityRecoveryBoundary`, `TestEvaluateAvailabilityImmediateFirstSuccess`, `TestEvaluateAvailabilityRecoveryOnlyAfterConfirmedOutage`, `TestEvaluateAvailabilityInconclusivePrecedence`, `TestEvaluateAvailabilityInconclusiveResetBoundary`, `TestEvaluateAvailabilityFreshnessAndNoResultPrecedence`, `TestEvaluateAvailabilityInvalidRow`, `TestEvaluateAvailabilityTruncatedPriorRun`: N-1 -> `awaiting_confirmation`, N -> `unavailable`; M-1 -> `recovering`, M -> `available` after a confirmed outage; first success without prior outage -> `available`; inconclusive latest precedence and older reset; future/stale/no_result; invalid row -> `ErrInvalidAvailability`; truncated prior run counts `< N`. Update existing single-observation tests to the slice signature without weakening assertions.
- [ ] Add `TestGetMonitorAvailabilityDerivesFromRecentSequence`, `TestGetMonitorAvailabilityEvidenceIsIndexZero`, `TestGetMonitorAvailabilityMapsNotFoundAndPersistence`, `TestGetMonitorAvailabilityEmptySequenceIsNoResult`: the use case reads the bounded sequence, evidence is index 0, error/clock mapping preserved, empty -> `no_result`, zero repository mutation.
- [ ] Run `go test ./internal/modules/monitoring/domain ./internal/modules/monitoring/application`; record focused RED.
- [ ] Implement the sequence policy and use case switch.
- [ ] Run focused GREEN plus existing domain/application regression and `go vet ./...`.
- [ ] Commit explicit files: `feat(api): assess confirmed availability over recent evidence`.

### Task 4: HTTP availability and composition evidence

**Files:** Modify `adapters/http/availability_test.go`, `apps/api/cmd/api/main_integration_test.go` and relevant composition tests. No production HTTP adapter or composition change (reason passthrough; same repository interface).

**Interfaces:** The composed handler serves `recovering` (unavailable) and `awaiting_confirmation` (unknown) with evidence over actual TCP; `no-store` on every matched response, `HEAD` 405 `Allow: GET`, unknown Monitor 404, malformed UUID 400 and sanitized 500 unchanged; the raw latest-result route is unchanged.

- [ ] Add `TestAvailabilitySerializesRecovering`, `TestAvailabilitySerializesAwaitingConfirmation`, `TestAvailabilityTransportGuardsUnchanged`, `TestCompositionServesConfirmedAvailability`: new reasons serialize with evidence; transport guards intact; the composed handler answers availability from recent evidence while latest-result stays single-row. Use actual TCP transport and real composition PostgreSQL where the suite requires it.
- [ ] Run `go test ./internal/modules/monitoring/adapters/http ./cmd/api`; record specific RED.
- [ ] Add the test evidence; if any production passthrough gap is found, fix it minimally with RED/GREEN (none expected).
- [ ] Run focused/full Go GREEN, real composition PostgreSQL tests and public-contract fixture consistency.
- [ ] Commit explicit files: `test(api): qualify confirmed availability transport and composition`.

### Task 5: Web availability union, decoder and reason copy

**Files:** Modify Web `entities/monitor/model.ts`, `entities/monitor/decode.ts`, `entities/monitor/decode.test.ts`, `entities/monitor/AvailabilityCard.tsx`, `entities/monitor/AvailabilityCard.test.tsx` and typed fake-client fixtures; `shared/api/public.generated.ts` already regenerated in Task 1.

**Interfaces:** `Availability` unavailable reason union gains `'recovering'`; unknown reason union gains `'awaiting_confirmation'` (both with evidence). `decodeAvailability` accepts `unavailable/recovering` and `unknown/awaiting_confirmation` with evidence and rejects any other pairing. The exhaustive `reasons` map gains human-readable copy for both (awaiting confirmation: failures below the confirmation threshold; recovering: successes below the recovery threshold after an outage).

- [ ] Add `decodeAcceptsRecovering`, `decodeAcceptsAwaitingConfirmation`, `decodeRejectsMismatchedNewReasons`, `cardRendersRecovering`, `cardRendersAwaitingConfirmation`: closed decoding accepts the two new pairings with evidence and rejects mismatches (e.g. `available/recovering`, `unknown/recovering`, missing evidence); the card renders both reasons; refresh recomputes. Update existing mocks without weakening assertions.
- [ ] Run focused Vitest tests; record RED.
- [ ] Implement the union/decoder/copy changes; preserve `requestJson` policy; no synchronous state publication inside effects (react-hooks gate); no new controls.
- [ ] Run focused and full Web GREEN including `format:check`; generated parity, typecheck, lint, boundaries and build.
- [ ] Commit explicit files: `feat(web): render confirmation/recovery availability`.

### Task 6: Composed restart/browser evidence and final review

**Files:** Modify `apps/web/e2e/local-monitoring.spec.ts`, `scripts/ci/verify-web-evidence.mjs`/`test-verify-web-evidence.mjs`, `scripts/ci/run-web-browser-tests.sh` with a `scripts/ci/web-confirmation-recovery-evidence.mjs` orchestrator as necessary; update `docs/backend/go-control-plane.md`, `docs/architecture/runtime-flows.md`, `docs/testing/public-monitoring-contract.md`, `docs/testing/local-monitoring-web.md`, `docs/devops/current-handoff.md`, `docs/README.md`.

**Interfaces:** Extend the browser artifact with closed `confirmationRecovery` evidence `{monitorId, transientAssessment, confirmedAssessment, recoveringAssessment, recoveredAssessment, readsAfterRestart}`: the journey registers a public-target monitor, drives a single transient failure -> `awaiting_confirmation` (not outage), sustained failures -> `unavailable`, sustained successes -> `recovering` then `available`, with durable SQL linkage of the streak rows, and the restarted API serves the recovered assessment. Orchestrator owns durable SQL snapshots; the verifier validates UUID/reason linkage and refuses missing checkpoints.

- [ ] Add the actual browser journey with RED assertions before acceptance preparation; verify durable linkage through SQL; preserve existing journeys.
- [ ] Run the root browser wrapper on both ports; record actual Chromium/SQL failure, not absent dependencies.
- [ ] Update canonical docs to implemented truth (assessment-only confirmation/recovery, two new reasons, bounded lookback limitation); keep historical specs/plans as decisions; mark this plan historical in `docs/README.md`.
- [ ] Commit acceptance/docs: `test(api): qualify confirmation/recovery and browser journey`.
- [ ] Request one fresh whole-branch reviewer; fix material findings with RED/GREEN in the same feature branch.
- [ ] Require final exact-head affected checks plus aggregate gate; finalize the feature PR and seek merge approval. After an authorized merge, verify exact-main CI and record durable handoff evidence. No deploy.

## Self-review and execution boundary

Design "Domain policy" maps to Task 3; "Persistence read" to Task 2; "Use case" to Task 3; "Contract, HTTP and gateway" to Tasks 1/4 (gateway unchanged); "Web" to Task 5; "Interactions and documented limitations" to Tasks 2/3/6; "Test strategy" to Tasks 1–6. Each Review Focus input has an owning test. No task changes Rust, the internal contract, any migration, the gateway, or the raw latest-result route; required regression gates still run when repository rules demand them. Composition is unchanged because the use case keeps the same repository interface.

Review this plan before execution. Preserve Native from the prior accepted workflow unless explicitly changed. No product code, migration, Docker stack or paid/external service is created by plan preparation. Missing actual confirmation/recovery evidence is an open acceptance item, never a fabricated pass.