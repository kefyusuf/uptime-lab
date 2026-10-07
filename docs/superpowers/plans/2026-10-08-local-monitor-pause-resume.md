# Local Monitor Pause/Resume Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Durably pause new Monitor claims and resume the existing cadence from the local detail page while preserving already claimed work and result evidence.

**Architecture:** Go owns validated scheduling state, use cases and PostgreSQL transactions. Claim selection filters active Monitors and rereads state after locking. Web provides explicit desired-state commands through a guarded GET/PUT gateway; Rust and internal execution contracts remain unchanged.

**Tech Stack:** Existing Go1.27.1/pgx/goose, PostgreSQL18, OpenAPI3.1.2, React/TypeScript/Vitest/Playwright, Node24.21.0/npm11.19.1. No new dependencies.

**Spec:** [Local Monitor Pause/Resume design](../specs/2026-10-08-local-monitor-pause-resume-design.md), written at `a744ffb`; user's subsequent continuation approved preparation of this plan.

**Status:** Plan awaiting review; no product task is executed. Preserve the prior Native execution method unless the user selects another method: this session implements the dependent tasks, then a fresh whole-branch reviewer checks the completed change. Plan approval is distinct from merge/deployment approval.

## Global Constraints

- Exactly `active`/`paused`; new/preexisting Monitors active. Pause stops new claims after commit; already claimed work can execute/finish. No cancellation protocol/result kind.
- Resume uses 60 seconds after latest terminal completion; no forced probe. Preserve pending uniqueness, timeout reconciliation, completion/replay/conflict rules and terminal evidence.
- Add only `00004_add_monitor_scheduling_state.sql`: `paused boolean NOT NULL DEFAULT false`. Exact readiness migration set becomes four; no historical edits, rolling-upgrade claim or automatic downgrade.
- Dedicated GET/PUT `/monitors/{monitorId}/scheduling`; closed `{"state":"active"}`/`{"state":"paused"}`. PUT raw maximum 1024 bytes; strict keys/duplicates/JSON; JSON optional UTF-8 charset only; no content encoding. No query, including `?`; GET no body; HEAD/OPTIONS 405 `Allow: GET, PUT`; matched no-store.
- Preserve existing five public operations, Monitor/list/cursor/result/availability shapes and internal contract. Go owns policy, Rust execution, Web presentation.
- Gateway explicit GET/PUT scheduling allowlist; exact mutation Origin/Host protection and body forwarding. Preserve creation's 65536-byte cap, gateway 16384-byte headers, 262144-byte upstream response cap, 5/10/10-second deadlines and browser 12-second deadline.
- One in-flight scheduling request per view; no optimistic success, automatic PUT retry, polling or inventory fan-out. Reread is a snapshot, not cancellation/quiescence proof.
- Scope excludes history, edit/delete, maintenance, incidents, notifications, ownership, remote access, quotas and capacity qualification. English code/docs/copy; concise Turkish before/after updates.
- Record focused RED reason, GREEN result and commit per task. Tool/setup failure is blocked verification, not product RED. Preserve user-owned changes; explicit paths only in commits.

## Review Focus

- Duplicate/escaped/case-shifted JSON keys evade ordinary struct decoding: Task 4 tests semantic duplicate keys and exact property spelling.
- A claim uses an old candidate view while pause commits: Task 3 proves locked-state reread and both transaction orderings, including snapshot interleaving evidence without claiming it from timing alone.
- A same-state command races an opposite command: Task 3 proves same-state writes still acquire the row lock and return committed state.
- A lost PUT response followed by a GET is mistaken for canceled work: Task 6 tests uncertainty, zero automatic retry and reread copy/controls without promising quiescence.
- A paused Monitor's expired pending run blocks progress: Task 3 proves global timeout reconciliation and another Monitor's claim without changing result policy.

## File map and execution prerequisites

Go module paths in tasks are relative to `apps/api/internal/modules/monitoring`; Web paths are relative to `apps/web`. Root paths are written in full. Existing constructors remain available; extend composition through dedicated constructors rather than unrelated restructuring.

Go commands run in `apps/api`; npm commands run in `apps/web`; CI scripts run at repository root. Use installed pinned runtimes or existing Linux CI. On Windows, inspect `.cache/web-node24` and `.cache/web-npm11` before using their Node/npm paths; do not assume historical installations remain usable. Real PostgreSQL uses `bash scripts/ci/run-go-postgres-tests.sh`; it owns its uniquely named stack. Real browsers use `bash scripts/ci/run-web-browser-tests.sh` once per port. Windows Git Bash Docker path conversion must be handled through the supported native PowerShell equivalent if needed, without global changes.

Before runtime execution, verify clean/dirty state, applicable instructions, tools and actual Docker access. Keep PR #59 as documentation review; create a dedicated `feat/local-monitor-pause-resume` execution branch from the approved docs baseline. If documentation remains unmerged, use an explicit stacked base and report it rather than silently assuming main contains the plan. Never merge #59 or the feature PR without approval. Reuse one draft feature PR for exact-head real SQL/browser RED/GREEN when local execution is unavailable.

No worktree or Docker resource is needed for writing this plan. If execution needs a worktree, use `<repo>/.worktrees/<name>` for manually created checkouts, `.git/info/exclude` for exclusion, or the managed worktree lifecycle. Task stacks use unique Compose projects; remove only task-owned volumes/containers/images on completion. Never remove main-stack volumes.

### Task 1: Closed scheduling contract and generated types

**Files:** Modify `contracts/openapi/public.yaml`, `scripts/ci/check-public-contract.mjs`, `scripts/ci/test-check-public-contract.mjs`; create `contracts/fixtures/public/monitor-scheduling-active.json` and `monitor-scheduling-paused.json`; regenerate `apps/web/src/shared/api/public.generated.ts`.

**Interfaces:** operation IDs `getMonitorScheduling` and `setMonitorScheduling`; schema `MonitorScheduling`, required closed `state` enum active/paused, GET/PUT 200. Error/status/media/no-store/Allow behavior follows design section 5; transport lexical constraints are documented as such, not invented OpenAPI validation guarantees.

- [ ] Write mutation tests `schedulingResourceIsClosed`, `schedulingMethodsAndBodyLimitAreExact`, `schedulingPreservesExistingOperations`: assert enum equals `["active","paused"]`, required equals `["state"]`, additionalProperties false, request/success use the same schema, and old operation schemas unchanged. Mutations introducing a toggle/extra property/method or omitted no-store must fail checking.
- [ ] Run root `node scripts/ci/test-check-public-contract.mjs`; record RED for the missing scheduling resource or an accepted bad mutation.
- [ ] Add only the specified schema/operations/fixtures and semantic assertions. Describe 1024-byte limit, duplicate keys, raw paths and no-query rules explicitly.
- [ ] Run semantic GREEN and the existing public-contract CI lint/bundle/check path; run Web `npm run generate:api`, `npm run check:generated`, `npm run typecheck`.
- [ ] Commit explicit contract/checker/generated files: `feat(contracts): define monitor scheduling resource`.

### Task 2: Validated state, scheduling port and application use cases

**Files:** Create `domain/scheduling_state.go` and `_test.go`, `ports/monitor_scheduling_repository.go`, `application/get_monitor_scheduling.go` and `_test.go`, `application/set_monitor_scheduling.go` and `_test.go`; modify `domain/errors.go` only for the invalid-state sentinel.

**Interfaces:** `type SchedulingState string`; constants `SchedulingActive`, `SchedulingPaused`; `ParseSchedulingState(string) (SchedulingState,error)`; `SchedulingState.Valid() bool`. Port methods `GetScheduling(context.Context, domain.MonitorID) (domain.SchedulingState,error)` and `SetScheduling(context.Context, domain.MonitorID, domain.SchedulingState) (domain.SchedulingState,error)`. Constructors `NewGetMonitorScheduling(ports.MonitorSchedulingRepository) GetMonitorScheduling`, `NewSetMonitorScheduling(ports.MonitorSchedulingRepository) SetMonitorScheduling`. Execute signatures mirror the port. Reuse `ports.ErrMonitorNotFound`, application not-found/persistence and context cancellation mapping.

- [ ] Add `TestSchedulingStateClosed`, `TestSchedulingUseCasesRejectInvalidInputBeforeRepository`, `TestSchedulingUseCasesMapNotFoundAndPersistence`, `TestSchedulingUseCasesRejectInvalidRepositoryState`: zero/unknown state rejected; invalid ID/state causes zero calls; no clock/Checker call; successful state exact; canceled context preserved; invalid repository output never succeeds. Setter returning a different valid state is a persistence failure, not successful confirmation of the requested state.
- [ ] Run `go test ./internal/modules/monitoring/domain ./internal/modules/monitoring/application`; record focused RED. Missing symbols establish only unit/interface RED.
- [ ] Implement validation/delegation and stable error translation; leave immutable Monitor untouched.
- [ ] Run focused GREEN plus existing domain/application regression tests.
- [ ] Commit explicit files: `feat(monitoring): add scheduling state use cases`.

### Task 3: Durable state and serialized claim eligibility

**Files:** Create `apps/api/migrations/00004_add_monitor_scheduling_state.sql`, `apps/api/migrations/monitor_scheduling_integration_test.go`, `adapters/postgres/monitor_scheduling.go`, `adapters/postgres/monitor_scheduling_integration_test.go`; modify `apps/api/migrations/compatibility_test.go`/`compatibility_integration_test.go` where the exact set is asserted, `adapters/postgres/check_execution.go`, `check_execution_integration_test.go` and query unit checks as required by actual SQL changes.

**Interfaces:** Repository implements Task 2 port. Getter one row; setter explicit Read Committed transaction with unconditional desired-state UPDATE RETURNING, not-found detection, validation and commit before success. Claim remains the existing port: explicit Read Committed, active predicate, same Monitor row lock, separate `SELECT paused` after locking and before INSERT; paused recheck commits reconciliation and returns existing no-due result. No Monitor-to-CheckRun locking path.

- [ ] Write real integration `TestSchedulingMigrationDefaultsExistingAndNewMonitors`, `TestSchedulingAtomicWritesAndRestartRead`, `TestSchedulingSameStateSerializesOppositeWrite`, `TestPausePreventsNewClaim`, `TestClaimBeforePauseRetainsPendingRun`, `TestPauseLockAllowsAnotherMonitorClaim`, `TestResumeKeepsCadenceAndPendingGuard`, `TestPausedCompletionAndTimeoutReconciliation`. Assert old/new defaults false, fresh connection reads paused, absent ID changes zero rows, exact completion-boundary eligibility, normal/duplicate/late completion rules, and paused expired work terminalizes while an active Monitor progresses.
- [ ] Add coordinated actual-adapter concurrency evidence using test-owned sessions and bounded channel/context synchronization. Observe lock ownership/waits through PostgreSQL, then explicitly commit/rollback each holder; no arbitrary sleeps as race proof. A test-only pgx tracer may gate query boundaries without adding production hooks. Assert the post-lock read is a separate statement before INSERT. Exercise a candidate snapshot overlapping a pause write; report the observed interleaving, and do not label a pre-query or post-commit gate as proof of an in-statement snapshot race. If deterministic executor interleaving cannot be demonstrated with existing instrumentation, retain that requirement as unresolved review evidence rather than silently dropping it.
- [ ] Run root `bash scripts/ci/run-go-postgres-tests.sh` on the actual RED head; require test failure for absent column/eligibility behavior, not setup/compile-only evidence.
- [ ] Implement migration/state adapter and minimal claim changes; retain reconciliation order, unique pending constraint and all result semantics.
- [ ] Repeat real SQL GREEN, migration compatibility and existing claim/completion suites; run Go unit/vet and required Linux race checks. No query performance claim from this suite.
- [ ] Commit explicit files: `feat(monitoring): persist and serialize monitor pause resume`.

### Task 4: Public scheduling adapter and composition

**Files:** Create `adapters/http/scheduling.go`, `scheduling_test.go`, `apps/api/cmd/api/scheduling_transport_test.go`; modify `adapters/http/handler.go`, `module.go`, `apps/api/cmd/api/main.go`, relevant composition tests and `main_integration_test.go`.

**Interfaces:** `NewModuleWithScheduling(repository ports.MonitorRepository, latest ports.LatestCheckResultRepository, inventory ports.MonitorInventoryRepository, idGenerator application.IDGenerator, clock application.Clock, scheduling ports.MonitorSchedulingRepository) Module`; add Module fields `GetMonitorScheduling application.GetMonitorScheduling` and `SetMonitorScheduling application.SetMonitorScheduling`. HTTP constructor: `NewHandlerWithScheduling(register registerMonitor, get getMonitor, latest getLatestCheckResult, availability getMonitorAvailability, list listMonitors, schedulingGet getMonitorScheduling, schedulingSet setMonitorScheduling) *Handler`. Its two new private interfaces expose Task 2 Execute signatures. Preserve legacy constructors. The platform dispatcher retains raw product paths; operational routes remain unchanged.

- [ ] Add `TestSchedulingHTTPValidationPrecedence`, `TestSchedulingStrictJSON`, `TestSchedulingByteLimit`, `TestSchedulingMethodsAndNoStore`, `TestSchedulingAliasesNeverRedirect`, `TestSchedulingCompositionPersistsState`. Assert design section 5 precedence, valid/invalid UUIDs, unknown Monitor, duplicate `state` including `"st\u0061te"`, `State`, unknown keys, trailing/null JSON, charset/encoding, 1024-byte whitespace-padded object accepted and 1025 rejected, chunked overflow, bodyless GET framing, HEAD/OPTIONS Allow/no-store and no Location. Malformed requests produce zero use-case calls; actual composed Go handler supports all seven public operations.
- [ ] Run `go test ./internal/modules/monitoring/adapters/http ./cmd/api`; record specific RED. Use actual TCP transport for alias/HEAD behavior, not only recorder tests.
- [ ] Implement raw-route recognition, strict token decoding, limited body read, problem/status mapping and composition. Do not refactor unrelated existing HTTP resources or widen accepted inputs.
- [ ] Run focused/full Go GREEN, real composition PostgreSQL tests and public-contract fixture consistency.
- [ ] Commit explicit files: `feat(api): expose bounded monitor scheduling controls`.

### Task 5: PUT gateway security and transport

**Files:** Modify Web `server/routes.ts`/`.test.ts`, `server/gateway.ts`/`.test.ts`, `server/proxy.ts`, `server/server.ts`/`.test.ts`.

**Interfaces:** Existing `RouteDecision` permits only GET/POST/PUT for explicitly mapped resources. Scheduling GET/PUT owns 1024-byte body cap; other routes preserve limits. Creation POST and scheduling PUT are the explicit mutation set for Origin, body forwarding and safe method logging; no generalized arbitrary-verb proxy.

- [ ] Add `mapsOnlySchedulingGetPut`, `putRequiresExactOrigin`, `putForwardsExactBufferedJson`, `schedulingHasRouteSpecificByteLimit`, `unfinishedPutClosesBeforeUpstream`, `schedulingKeepsInternalRoutesClosed`: reject absent/null/duplicate/foreign Origin and cross-site metadata, body/query/alias/method/media errors; assert zero captured upstream calls. Stream 1025 bytes ->413, accept exactly 1024, retain creation 65536. Real unfinished PUT socket ->408/no-store/closed connection before upstream; verify valid JSON Content-Type/Length and no cookies/arbitrary headers forwarded. Existing GET/POST/deadline and sanitized error tests remain.
- [ ] Run Web `npm test -- server/routes.test.ts server/gateway.test.ts server/server.test.ts`; record RED.
- [ ] Implement explicit mutation handling at every existing POST-specific branch and route-specific caps. Keep header/body/upstream/browser deadlines and response cap unchanged.
- [ ] Run focused GREEN, Web typecheck/lint/boundaries/build and existing gateway/socket regression tests.
- [ ] Commit explicit files: `feat(web): guard and forward scheduling mutations`.

### Task 6: Scheduling client and independent detail controls

**Files:** Create Web `entities/monitor/scheduling.ts`/`.test.ts`, `features/set-monitor-scheduling/useMonitorScheduling.ts`/`.test.tsx`, `MonitorSchedulingControl.tsx`/`.test.tsx`, `index.ts`; modify `entities/monitor/model.ts`, `api.ts`/`.test.ts`, `index.ts`, `pages/MonitorDetailPage.tsx`/`.test.tsx` and typed fake-client fixtures in existing tests. Change styles only where control accessibility/layout requires it.

**Interfaces:** `MonitorScheduling = {state:'active'|'paused'}`; `decodeMonitorScheduling(unknown):MonitorScheduling`. `MonitorClient.getScheduling(id,signal):Promise<ReadResult<MonitorScheduling>>`; `setScheduling(id,state,signal):Promise<SchedulingOutcome>` where outcome is `{kind:'confirmed';data:MonitorScheduling}`, `{kind:'rejected';error:ClientError}` or `{kind:'uncertain';error:ClientError}`. `useMonitorScheduling(id,client)` returns state (loading/ready/writing/read-error/uncertain), `refresh()` and `setState(active|paused)`; control composes it only after a Monitor is known.

- [ ] Add `schedulingDecoderIsClosed`, `schedulingClientNeverRetries`, `schedulingWriteFailureIsUncertain`, `oneInFlightSchedulingRequest`, `staleResponsesCannotReplaceNewMonitor`, `evidenceReadsRemainIndependent`, `uncertainReadbackIsOnlyASnapshot`. Assert only valid200 confirms the requested state; mismatched valid state/other2xx is uncertain. Known400/404/413/415 requires reread; transport/5xx/malformed is uncertain. No PUT on reread or automatic rerender. Loading/error/write disable mutation, refresh disabled during write, obsolete response ignored, evidence remains usable, one initial read only after Monitor success, no inventory scheduling calls.
- [ ] Run focused Vitest tests; record RED.
- [ ] Implement exact decoder/client outcome mapping and separate feature state machine. Preserve requestJson deadlines/abort/cache/redirect/credentials policy. Display the spec's exact pause explanation and "Refresh scheduling state"; do not promise that abort/reread cancels server work. No optimistic success or polling.
- [ ] Run focused and full Web GREEN; generated parity, typecheck, lint, format, boundaries and build. Update existing mocks only to satisfy the expanded client, without weakening old assertions.
- [ ] Commit explicit files: `feat(web): add durable pause resume detail controls`.

### Task 7: Composed restart/browser evidence and final review

**Files:** Modify `apps/web/e2e/local-monitoring.spec.ts`, `scripts/ci/run-web-browser-tests.sh`, `verify-web-evidence.mjs`/`test-verify-web-evidence.mjs`, browser orchestration tests as necessary; update `docs/backend/go-control-plane.md`, `docs/frontend/local-monitoring-web.md`, `docs/testing/local-monitoring-web.md`, `docs/testing/single-checker-execution-slice.md`, `docs/devops/current-handoff.md`, roadmap/index continuation references.

**Interfaces:** Extend the existing browser artifact with closed `scheduling` evidence `{monitorId,pausedState,resumedState,checkIdsBeforePause,checkIdsWhilePaused,checkIdAfterResume,restarts}`. States are exact active/paused strings, ID arrays contain UUIDs; `restarts` contains closed `api` and `checker` objects, each with UTC-string `beforeStartedAt` and `afterStartedAt` from Docker inspect. Verify after is strictly later; container ID alone cannot prove a restart. Orchestrator owns durable SQL snapshots/restart checkpoints; verifier validates UUID/state/CheckRun linkage and refuses missing checkpoints. Existing inventory/raw/availability artifact fields remain unchanged. Pause CheckRun baseline is captured after PUT commit, including any earlier pending run; completion of that run is allowed.

- [ ] Add actual browser journey: inventory -> detail -> Pause -> reload state; orchestrator restarts API with the same DB, restarts Checker, verifies paused state/no new CheckRun IDs over bounded claim opportunities, then Resume -> subsequent real claim. Control due conditions through isolated fixture preparation, never a product safety bypass. Preserve private-target `policy_rejected` smoke and independently validate durable rows. Add verifier mutation tests rejecting missing restart checkpoints, changed Monitor IDs, new paused CheckRuns and unlinked resumed claims. Fake CLI checks are orchestration proof only.
- [ ] Run root browser wrapper on both ports with RED assertions before acceptance preparation; record actual Chromium/SQL failure, not absent dependencies. No replay of old browser artifact counts as current-head proof.
- [ ] Implement the isolated preparation/checkpoint capture and minimal verification needed for the journey. Distinguish allowed pending completion from forbidden new insertion. Emit failures within bounded deadlines, clean only task-owned resources, preserve same DB volume across intra-test restarts.
- [ ] Run actual GREEN on `UPTIME_LAB_WEB_PORT=4173` and `4817`, canonical local smoke, and all affected required CI gates. Update canonical docs to implemented truth and evidence; keep historical specs/plans as decisions.
- [ ] Commit acceptance/docs: `test(monitoring): qualify pause resume persistence and browser journey`.
- [ ] Request one fresh whole-branch reviewer through the preserved Native workflow, with no inherited implementation history and the configured most capable reviewer. Review all seven tasks together, emphasizing Review Focus, lock/race evidence and contract/gateway/UI consistency. Fix material findings with RED/GREEN in the same feature branch; record any unresolved acceptance evidence explicitly.
- [ ] Require final exact-head affected checks plus aggregate gate; finalize the same feature PR, summarize before/after and seek merge approval. After an authorized merge, verify exact-main CI, archive/remove only owned merged worktrees, clean task stacks/images, and record durable handoff/activity evidence. No deploy.

## Self-review and execution boundary

Spec sections 1–2 map to Tasks 2/3/6; section 3 to Tasks 2/3/4; section 4 to Task 3; section 5 to Tasks 1/4; section 6 to Task 5; section 7 to Task 6; section 8 to Tasks 1–7. Each Review Focus input has an owning test. No task changes Rust or the internal contract; required regression gates still run when repository rules demand them.

Review this plan before execution. Preserve Native from the prior accepted workflow unless explicitly changed: implementation in this session through `superpowers:executing-plans`, then a fresh whole-branch review. No product code, migration, Docker stack or paid/external service is created by plan preparation. Missing actual snapshot-interleaving evidence is an open acceptance item, never a fabricated pass.
