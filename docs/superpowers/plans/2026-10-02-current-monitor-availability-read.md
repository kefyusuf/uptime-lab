# Current Monitor Availability Read Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add one public, read-only Monitor availability assessment derived from terminal execution evidence and Go-owned time policy.

**Architecture:** A pure Monitoring domain policy classifies validated evidence. A new application use case reuses the existing latest-terminal read port and injected Clock. The existing public adapter and composition root expose it without new persistence or Rust behavior.

**Tech Stack:** Existing Go 1.27.1 module, stdlib HTTP, pgx/PostgreSQL, OpenAPI 3.1.2, existing Node contract checker and Bash/Docker verification; no new product dependencies.

**Spec:** [Current Monitor Availability Read Slice Design](../specs/2026-10-02-current-monitor-availability-read-design.md).

**Status:** Review candidate; no implementation authorized by this document.
**Base:** `docs/current-availability-design@183ecdec6094b14cb7dc7d58e929a3ceb5f4dd19`, PR #46. This plan PR targets that design branch so its review diff is one plan file. Implementation must start from the landed spec/plan on main, not from an unreviewed stack.

When #46 lands, rebase only the plan commit onto fresh main and retarget the plan PR to main; preserve a one-file plan diff and rerun exact-head CI. Do not merge the plan into the design branch as a shortcut.

## Global Constraints

- Exactly one new `GET /monitors/{monitorId}/availability`, operationId `getMonitorAvailability`; public path count becomes four.
- Status is `available`, `unavailable`, or `unknown`; use all eight spec reasons and only valid status/reason/evidence combinations.
- Fresh 200–299 -> available; every other valid HTTP status -> unavailable. No body-content or complete-download claim.
- DNS/timeout/connect/TLS/protocol failure -> unavailable. Policy/internal/worker failure -> unknown with the spec reasons.
- Known Monitor/no terminal -> 200 unknown/no_result; existing latest-result 204 is unchanged.
- Future completion -> unknown/future_result; age greater than 120 seconds -> unknown/stale_result; exactly 120 seconds is fresh.
- One terminal read, one clock sample after successful resolution/validation; UTC JSON-representable timestamps; no time rounding before comparison.
- GET performs no writes, locks, claims, or timeout reconciliation; newer pending work stays invisible.
- Matched availability responses include `Cache-Control: no-store`, including errors and 405; HEAD is 405 + `Allow: GET`.
- No migration, new read port, Rust/internal-contract/Compose topology change, dependencies, public exposure, history, lifecycle, incident or notification feature.
- Preserve existing Monitor/latest-result schemas, public version 0.1.0, narrow ownership, one DB pool, and English contributor copy.
- Keep RED failure evidence, focused GREEN output, and each commit SHA. Report each completed task's before/after contribution briefly to the user.

## Review Focus

- A clock moving backward yields future_result rather than an available result; Task 2 pins precedence and Task 3 pins injected clock usage.
- Timezone conversion crossing a JSON year boundary must produce sanitized 500, not a partially written 200; Task 3 pins timestamp validation before transport.
- Adjacent raw-result and availability requests can observe different CheckIDs; Task 5 pins each response to its own evidence rather than assuming a cross-request snapshot.
- A matched invalid UUID or wrong method still carries no-store; Task 4 pins error/method headers and route parsing.
- A delayed Docker run must reject stale_result even though it is unknown; Task 6 pins freshness/reason and CheckID correlation explicitly.

## File and responsibility map

| Responsibility | Create | Modify |
|---|---|---|
| Contract | Eight `contracts/fixtures/public/availability-*.json` files listed in Task 1 | `contracts/openapi/public.yaml`, `scripts/ci/check-public-contract.mjs`, `scripts/ci/test-check-public-contract.mjs` |
| Pure policy | `apps/api/internal/modules/monitoring/domain/availability.go`, `availability_test.go` | Domain `errors.go` |
| Application read | `apps/api/internal/modules/monitoring/application/get_monitor_availability.go`, `get_monitor_availability_test.go` | Application `errors.go` |
| Public adapter | `apps/api/internal/modules/monitoring/adapters/http/availability.go`, `availability_test.go` | Existing `handler.go` |
| Composition | None | `apps/api/internal/modules/monitoring/module.go`, `apps/api/cmd/api/main.go`, `main_integration_test.go` |
| Runtime smoke | None | `scripts/ci/smoke-local-dev.sh`, `scripts/ci/test-smoke-local-dev.sh` |
| Current truth | None | README, docs index, architecture module/data/runtime docs, backend/testing/devops guides listed in Task 7 |

The existing latest-result port, PostgreSQL query, migrations, platform server, internal adapter and Checker code stay unchanged. `latestCheckResultFromRecord` remains the shared application validation seam, reused directly without changing its behavior.

## Task 0: Establish implementation preconditions

**Interfaces:** Consumes the approved spec/plan and landed main; produces a clean short-lived implementation branch and baseline evidence.

- [ ] Confirm written spec and plan review, selected execution method, landed commits, fresh main CI, and no unresolved actionable review findings. Do not silently treat a skipped CodeRabbit status as a review.
- [ ] Resolve the execution environment before claiming acceptance: check `go version`, Node/Redocly versions from CI, `docker version`, and `docker compose version`. Docker was not in this session's PATH during planning; no install or runtime check is claimed here. Use the existing container/CI harness when native prerequisites are unavailable, and keep local RED evidence for the layers that can run.
- [ ] From a clean checkout create `feat/current-monitor-availability-read`; record the base SHA and open one draft implementation PR. Attach it to this chat.
- [ ] Run baseline public-contract, Go, and applicable architecture checks. Record failures rather than attributing them to the feature automatically. Use an isolated Compose project for destructive migration/smoke tests.

## Task 1: Specify the additive public contract

**Files:** Public YAML, checker/test harness, and fixtures `availability-available.json`, `availability-unavailable-http.json`, `availability-unavailable-probe.json`, `availability-unknown-no-result.json`, `availability-unknown-stale.json`, `availability-unknown-future.json`, `availability-unknown-policy.json`, `availability-unknown-execution.json` under `contracts/fixtures/public/`.

**Interfaces:** Produces schemas `MonitorAvailability`, `AvailableMonitorAvailability`, `UnavailableMonitorAvailability`, `UnknownMonitorAvailability`, `NoResultMonitorAvailability`, and `MonitorAvailabilityEvidence`. `MonitorAvailability` is a four-way oneOf; evidence requires exactly checkId/completedAt. All variants require status/reason/evaluatedAt; only no_result omits evidence.

- [ ] RED the exact fourth operation, response codes 200/400/404/500, UUID parameter, closed schemas, reason pairing, required evidence and no-store headers. Preserve old schema assertions. Reuse the harness's `expectReject`/fixture mutation conventions, including:

```js
expectReject('availability path missing fails', (d) => {
  delete d.paths['/monitors/{monitorId}/availability'];
}, 'public path set');
```

Add equivalent named mutations for `available` with `execution_failure`, no_result with evidence, evidence-based unknown without evidence, null evidence, extra fields, and absent cache header. The checker must validate fixture combinations and exact field sets as well as schema declarations; no new validation library.

- [ ] Run `node scripts/ci/test-check-public-contract.mjs`; demonstrate failures attributable to the missing operation/invariants, not merely syntax or tool setup.
- [ ] GREEN the schema/checker/fixtures. Require available/successful_response, unavailable/{unexpected_http_status,probe_failure}, unknown/{future_result,stale_result,policy_rejected,execution_failure}, and unknown/no_result variants. No security/servers additions or legacy shape changes.
- [ ] Run Node tests plus pinned Redocly lint/bundle/checker commands from `.github/workflows/ci.yml` (Node 24.21.0, Redocly 2.53.3); expect zero failures and a checker exit code of 0.
- [ ] Commit only Task 1 files: `feat(contracts): define current monitor availability read`. Before: raw-result contract only. After: exact public product-assessment contract exists, runtime not yet wired.

## Task 2: Implement pure domain availability policy

**Files:** Domain `availability.go`, `availability_test.go`, `errors.go`.

**Interfaces:** Produces `AvailabilityStatus` and `AvailabilityReason` string types; immutable `Availability` with `Status() AvailabilityStatus` and `Reason() AvailabilityReason`; `AvailabilityObservation{Kind CheckResultKind, HTTPStatus *int, CompletedAt time.Time}`; `EvaluateAvailability(observation *AvailabilityObservation, evaluatedAt time.Time) (Availability, error)`; `ErrInvalidAvailability`. Nil observation is the explicit no-result input. The function reads no clock and mutates no input.

Constant names: statuses `AvailabilityAvailable`, `AvailabilityUnavailable`, `AvailabilityUnknown`; reasons `AvailabilitySuccessfulResponse`, `AvailabilityUnexpectedHTTPStatus`, `AvailabilityProbeFailure`, `AvailabilityNoResult`, `AvailabilityFutureResult`, `AvailabilityStaleResult`, `AvailabilityPolicyRejected`, `AvailabilityExecutionFailure`.

- [ ] RED `TestEvaluateAvailabilityHTTPStatusBoundaries` at 100/199/200/204/299/300/399/400/499/500/599, every normalized kind, nil observation, invalid kind/status shape, and invalid evaluated/completed times. Assert no input mutation.
- [ ] RED `TestEvaluateAvailabilityFreshnessBoundary` and `TestEvaluateAvailabilityFutureAndStalePrecedence`. Pin these exact assertions:

```go
got, err := EvaluateAvailability(&AvailabilityObservation{
    Kind: CheckResultHTTPResponse, HTTPStatus: &status200, CompletedAt: completedAt,
}, completedAt.Add(120*time.Second))
if err != nil || got.Status() != AvailabilityAvailable { t.Fatalf("exact boundary: %v %v", got, err) }
got, err = EvaluateAvailability(&AvailabilityObservation{
    Kind: CheckResultHTTPResponse, HTTPStatus: &status200, CompletedAt: completedAt,
}, completedAt.Add(120*time.Second + time.Nanosecond))
if err != nil || got.Reason() != AvailabilityStaleResult { t.Fatalf("past boundary: %v %v", got, err) }
```

Also pin future success/probe failure/policy failure to future_result, stale policy/internal/worker failure to stale_result, equivalent timezone instants, and negative age after a backward clock step. Initialize `status200 := 200` and a fixed UTC `completedAt` in the test.

- [ ] From `apps/api`, run `go test ./internal/modules/monitoring/domain -run TestEvaluateAvailability -count=1`; first demonstrate missing policy/invariant RED, then implement minimum pure classification and rerun for GREEN.
- [ ] Refactor only duplication inside this policy; retain unrounded duration comparison and fixed `120*time.Second`. Return ErrInvalidAvailability for unsupported/invalid input instead of default unknown.
- [ ] Commit: `feat(api): define current availability policy`. Before: normalized execution facts. After: deterministic in-memory assessment with explicit freshness and no I/O.

## Task 3: Add the application read and invariant-preserving model

**Files:** Application `get_monitor_availability.go`, `get_monitor_availability_test.go`, `errors.go`.

**Interfaces:** Consumes Task 2, `ports.LatestCheckResultRepository`, `Clock`, and existing `latestCheckResultFromRecord`. Produces `NewGetMonitorAvailability(repository ports.LatestCheckResultRepository, clock Clock) GetMonitorAvailability`; `Execute(context.Context, domain.MonitorID) (MonitorAvailability, error)`; model accessors `Status() domain.AvailabilityStatus`, `Reason() domain.AvailabilityReason`, `EvaluatedAt() time.Time`, `Evidence() (MonitorAvailabilityEvidence, bool)`. Evidence is a copy-valued struct with `CheckID domain.CheckID` and `CompletedAt time.Time`; model fields remain private. Add `ErrAvailabilityEvaluation` for invalid clock/policy evaluation.

- [ ] RED table tests named `TestGetMonitorAvailabilityOutcomes`: every terminal kind/status class, no-result, missing Monitor, canceled read/persistence error, invalid MonitorID, invalid durable kind/status/duration/CheckID/time shape. Invalid source rows map to ErrPersistence; missing -> ErrMonitorNotFound. Preserve the existing raw-result validation unchanged.
- [ ] RED `TestGetMonitorAvailabilitySamplesClockAfterRead`, using existing `fakeLatestCheckResultRepository` conventions and a repository fake with an ordered callback. Assert one repository read and one clock call after successful terminal validation or known/no-result resolution; zero clock calls on invalid input/read/row errors.
- [ ] RED `TestGetMonitorAvailabilityTimestampSafety`: zero clock, UTC year outside 0..9999, a local timestamp whose UTC conversion crosses that range, future result, and equivalent non-UTC instants. Assert sanitized application error before transport; do not clamp time or round age.
- [ ] Run `go test ./internal/modules/monitoring/application -run TestGetMonitorAvailability -count=1` from `apps/api` for RED; implement direct port resolution, shared terminal validation, availability-specific timestamp safety before clock sampling, policy call, and immutable model for GREEN.
- [ ] Rerun existing GetLatestCheckResult tests and the full application package; expect both new/legacy suites passing. Commit: `feat(api): derive monitor availability on read`. Before: pure policy only. After: one repository snapshot and injected clock produce public-safe evidence.

## Task 4: Expose the isolated public HTTP operation

**Files:** Public adapter `availability.go`, `availability_test.go`, existing `handler.go`.

**Interfaces:** Consume Task 3 through private `getMonitorAvailability` interface with the exact Execute signature. Add `NewHandlerWithAvailability(register registerMonitor, get getMonitor, getLatest getLatestCheckResult, getAvailability getMonitorAvailability) *Handler`; preserve both existing constructors. `availabilityMonitorIDPathSegment(string) (string, bool)` recognizes only the exact nested shape; `(*Handler).serveAvailability(http.ResponseWriter, *http.Request, string)` owns transport mapping only.

- [ ] RED `TestAvailabilityHTTPResponses`: all status/reason shapes, exact keys, nested evidence keys, omitted evidence for no_result, invalid UUID 400, missing 404, internal 500, and sanitized Problem responses. Assert `Cache-Control == "no-store"` for each matched-route case.
- [ ] RED `TestAvailabilityHTTPMethodsAndPaths`: POST/PUT/DELETE/HEAD -> 405 plus Allow: GET and no-store; trailing slash, extra segment, empty ID and unknown suffix retain 404 behavior. GET route with no configured availability capability stays unavailable. Pin old create/read/latest-result payloads unchanged.
- [ ] From `apps/api`, run `go test ./internal/modules/monitoring/adapters/http -run TestAvailability -count=1`; implement the constructor, strict routing, exact writer/header mapping and rerun for GREEN. Do not derive status or read time in the HTTP handler.
- [ ] Run all public adapter tests and Task 1 contract tests. Commit: `feat(api): expose monitor availability transport`. Before: application model. After: isolated contracted HTTP behavior, production wiring still pending.

## Task 5: Wire production composition and prove database-backed behavior

**Files:** `module.go`, `cmd/api/main.go`, `cmd/api/main_integration_test.go`; retain the existing PostgreSQL integration tests unchanged unless a new concrete read assertion is necessary.

**Interfaces:** Add `Module.GetMonitorAvailability application.GetMonitorAvailability`; keep `NewModule(repository, latestResultRepository, idGenerator, clock)` parameter types/signature unchanged and construct the new capability from the existing read repository and Clock. Keep `composeMonitoring(*pgxpool.Pool) (http.Handler, *migrations.CompatibilityChecker, *sql.DB, error)` unchanged; select Task 4's constructor.

- [ ] Extend `TestProductionMonitoringCompositionAgainstPostgreSQL` for RED: known Monitor before completion -> unknown/no_result; completed HTTP 204 -> available; HTTP 500 -> unavailable; forced late completion -> unknown/execution_failure; missing Monitor -> 404. Assert CheckID/completedAt against the durable row, evaluatedAt present/parseable, no-store, and exact JSON shape.
- [ ] Add deterministic stale/future evidence cases using controlled DB rows/timestamps and the real production composition. Preserve issued/deadline/completion constraints. Do not sleep 120 seconds or add a production clock override.
- [ ] Add a newer pending run and prove prior terminal assessment remains visible without row mutation. Preserve existing raw-result ordering/no-write tests. Create another terminal result between two requests and assert each response's evidence is internally correct; do not require the earlier raw-result response and later availability response to share a CheckID.
- [ ] Use `bash scripts/ci/run-go-postgres-tests.sh` for RED, then minimal composition wiring for GREEN; this runs migrations, repository, and cmd/api integration against an isolated real PostgreSQL. CI's preconfigured PG environment may run the same tagged Go commands directly.
- [ ] Run Go unit/race, production integration, and Go architecture checks. No second pool or persistence-policy SQL. Commit: `feat(api): compose current availability read`. Before: isolated handler. After: real API/database path works with existing state.

## Task 6: Add deterministic cross-runtime availability evidence

**Files:** Existing smoke script and fake-Docker test harness.

**Interfaces:** Add `assert_availability_public_read()` after `assert_latest_result_public_read()` on the existing `http://web/` path. Reuse MONITOR_ID/CHECK_RUN_ID; preserve raw result, Checker log, restart and reset evidence. Add string-in/stdout-out helpers `json_availability_status`, `json_availability_reason`, `json_availability_evidence_check_id`, `json_availability_completed_at`, `json_availability_evaluated_at`, `json_availability_keys`, and `json_availability_evidence_keys`. Key helpers return sorted comma-separated keys. Extend the existing awk/JSON helper approach for this closed nested-evidence shape; pin reordered fields, fractional timestamps and extra nested keys in harness tests. No new tool dependency.

- [ ] RED harness cases `case_availability_wrong_check_id`, `case_availability_stale_reason`, `case_availability_unavailable_status`, `case_availability_missing_evidence`, and `case_availability_invalid_timestamp`. Extend fake responses/log assertions so the full smoke must issue the new route. Reject HTTP success with wrong JSON as well as request failures.
- [ ] Run `bash scripts/ci/test-smoke-local-dev.sh` for RED; implement exact top-level/nested keys, unknown/policy_rejected, matching CheckID and completion instant, parseable UTC timestamps, and `0 <= evaluatedAt-completedAt <= 120 seconds` for GREEN. PostgreSQL timestamp arithmetic can validate age without introducing a host date dependency.
- [ ] Ensure the bounded run rejects stale_result instead of passing any unknown status. The normal smoke's one-terminal/no-pending preconditions supply correlation; do not widen or reset the fixture to hide failures.
- [ ] Run the harness and real `bash scripts/ci/smoke-local-dev.sh`; assert fresh evidence, production safety, restart persistence and destructive reset in the isolated Compose project. Preserve four services and unpublished application ports.
- [ ] Commit: `test(devops): prove current availability runtime read`. Before: raw execution smoke. After: persisted Go/Rust evidence is linked to the public product assessment.

## Task 7: Update current-state documents and complete verification

**Files:** `README.md`, `docs/README.md`, `docs/backend/go-control-plane.md`, `docs/architecture/module-boundaries.md`, `docs/architecture/data-ownership.md`, `docs/architecture/runtime-flows.md`, `docs/architecture/container-view.md`, `docs/testing/public-monitoring-contract.md`, `docs/testing/single-checker-execution-slice.md`, `docs/devops/local-development.md`. Change architecture fitness scripts/tests only when an implemented new invariant or stale enforced assertion requires it.

- [ ] Update current truth only after runtime GREEN: fourth operation, raw fact versus availability, all mapping/freshness rules, unknown/no-result 200, unchanged latest-result 204, no-store, and no public-deployment claim. Correct existing stale module-boundaries summaries. Preserve historical specs/plans as historical.
- [ ] Run `bash scripts/ci/test-architecture-docs.sh`, `bash scripts/ci/check-architecture-docs.sh .`, `bash scripts/ci/check-repository-shape.sh .`, link checks, and `git diff --check`; expect exit 0. If a fitness script changes, RED its regression test first.
- [ ] Commit: `docs(docs): document current availability read`. Before: runtime-only increment. After: canonical docs and fitness describe the implemented capability.
- [ ] Run final relevant suites once on final head: public Redocly/Node contract; `go mod verify`, `go vet ./...`, `go test ./...`, `go test -count=1 -race ./...`; `bash scripts/ci/run-go-postgres-tests.sh`; Go architecture scripts; Docker smoke/harness; pinned `govulncheck` from CI; repository/documentation checks. Go commands run from `apps/api`, repository scripts from root. Check gofmt and tidy drift with existing CI steps.
- [ ] Compare final branch against implementation base: migrations, Rust, internal contract, Compose and CI workflow unchanged; no unexpected dependency changes. Mark implementation PR review-ready only after relevant exact-head CI and independent review evidence. Review approval is not permission to bypass unresolved findings.
- [ ] Obtain explicit merge approval, squash with expected-head protection, and verify fresh push CI on resulting main. Do not assume permission to merge from permission to open a PR.

## Plan review and execution handoff

This plan is documentation only. User approval of the written specification permits this plan, not product implementation before plan review. Review the task boundaries, interfaces, failure mapping, freshness boundary, verification/environment requirements, and commit shape.

Choose execution after reviewing the plan: Native (`superpowers:executing-plans`) or Subagent-driven (`superpowers:subagent-driven-development`). Native is recommended for this bounded Go slice because the tasks share a small read model and existing adapters. No subagents are dispatched by this planning task. Independent final review remains a separate execution requirement.

Design/plan PRs must land through the repository workflow before the implementation branch starts. Their merge requires explicit user approval and completed review/CI evidence. This plan introduces no application behavior, dependency, contract or schema changes.
