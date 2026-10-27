# Local Monitor Delete Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Durable, irreversible local deletion of a Monitor together with all of its CheckRun evidence, exposed through one guarded public operation and a confirmed detail-page control.

**Architecture:** Go owns the atomic child-first deletion transaction, post-delete read semantics and the sixth public operation. The gateway treats DELETE as a third explicit mutation. Rust, the internal contract and the migration set are unchanged.

**Tech Stack:** Existing Go1.27.2/pgx/goose, PostgreSQL18, OpenAPI3.1.2, React/TypeScript/Vitest/Playwright, Node24.21.0/npm11.19.1. No new dependencies, no new migration.

**Spec:** [Local Monitor Delete design](../specs/2026-10-27-local-monitor-delete-design.md), written at `3935e96`, stacking the [scope](../specs/2026-10-27-local-monitor-delete-scope-reassessment.md) at `3ea70e0`; both in PR #68. The user's continuation after the design authorized preparing this plan.

**Status:** Plan awaiting review; no product task is executed. Preserve the prior Native execution method unless the user selects another method: this session implements the dependent tasks, then a fresh whole-branch reviewer checks the completed change. Plan approval is distinct from merge/deployment approval.

## Global Constraints

- Deletion removes the Monitor row and every CheckRun referencing it, pending and terminal, in one explicit Read Committed transaction: pre-sweep children, `SELECT ... FOR UPDATE` the Monitor row, re-sweep children, delete the Monitor, commit before success. No tombstone, no undelete, no new result kind.
- After deletion all reads of the Monitor are 404 and the inventory excludes it. Repeated delete is 404. Other Monitors are untouched.
- No migration; the readiness migration set stays at four. No claim or completion SQL changes; reconciliation order, pending uniqueness and all result semantics are preserved.
- `DELETE /monitors/{monitorId}` succeeds with an empty `204`, no Location. Matched responses, including 204/404/405, use `Cache-Control: no-store`. Resource `Allow` becomes exactly `GET, DELETE`; collection and sub-resource Allow values unchanged. No query including a bare `?`; DELETE rejects declared or streamed body framing with 400 before application work.
- Preserve the existing seven public operations and their shapes; deletion adds the eighth. Internal OpenAPI unchanged. Rust and `contracts/openapi/internal.yaml` unchanged.
- Gateway DELETE joins the explicit mutation set: exact Host/Origin, cross-site Fetch Metadata protection, local body-framing rejection, no credentials/cookies/arbitrary headers upstream, no upstream retry, unchanged caps and deadlines.
- Browser: detail-only control, two-step confirmation, one in-flight delete, no automatic retry, `204`/known `404` map to deleted, everything else uncertain with explicit refresh resolution. No inventory actions, polling or bulk operations.
- Scope excludes edit/update, history, retention, incidents, notifications, ownership, remote access, quotas and capacity qualification. English code/docs/copy; concise Turkish before/after updates.
- Record focused RED reason, GREEN result and commit per task. Tool/setup failure is blocked verification, not product RED. Preserve user-owned changes; explicit paths only in commits.

## Review Focus

- A claim commits pending work between the pre-sweep and the Monitor lock: Task 3 proves the re-sweep removes it and the parent delete never hits the foreign key.
- Delete takes the Monitor lock while claim reconciliation holds a CheckRun lock: Task 3 exercises the overlap and shows the pre-sweep ordering prevents the inversion, with any lock-wait abort surfacing as sanitized 500 and atomic state.
- A completion arrives for a run the delete removed: Task 3 proves adapter-level 409 conflict mapping and Task 6/7 prove the Checker delivery is a single non-retried attempt (`result_delivery_failed`), with no Rust change.
- The resource `Allow` silently regresses or leaks to other resources: Task 4 asserts exact `Allow: GET, DELETE` on the monitor resource and unchanged values elsewhere.
- A lost DELETE response is mistaken for confirmed deletion: Task 6 forbids automatic retry and pins uncertain plus explicit refresh resolution; Task 7 proves committed deletion across an API restart.
- A known 404 after delete is presented as failure, or navigation happens before confirmation: Task 6 pins the deleted mapping and the two-step gate.

## File map and execution prerequisites

Go module paths in tasks are relative to `apps/api/internal/modules/monitoring`; Web paths are relative to `apps/web`. Root paths are written in full. Existing constructors remain available; extend composition through dedicated constructors rather than unrelated restructuring.

Go commands run in `apps/api`; npm commands run in `apps/web`; CI scripts run at repository root. Use installed pinned runtimes or existing Linux CI. On Windows, inspect `.cache/web-node24` and `.cache/web-npm11` before using their Node/npm paths; do not assume historical installations remain usable. Real PostgreSQL uses `bash scripts/ci/run-go-postgres-tests.sh`; it owns its uniquely named stack. Real browsers use `bash scripts/ci/run-web-browser-tests.sh` once per port. Windows Git Bash Docker path conversion must be handled through the supported native PowerShell equivalent if needed, without global changes.

Before runtime execution, verify clean/dirty state, applicable instructions, tools and actual Docker access. Keep PR #68 as documentation review; create a dedicated `feat/local-monitor-delete` execution branch stacked on the approved docs baseline. If documentation remains unmerged, use an explicit stacked base and report it rather than silently assuming main contains the plan. Never merge #68 or the feature PR without approval. Reuse one draft feature PR for exact-head real SQL/browser RED/GREEN when local execution is unavailable.

No worktree or Docker resource is needed for writing this plan. If execution needs a worktree, use `<repo>/.worktrees/<name>` for manually created checkouts, `.git/info/exclude` for exclusion, or the managed worktree lifecycle. Task stacks use unique Compose projects; remove only task-owned volumes/containers/images on completion. Never remove main-stack volumes.

### Task 1: deleteMonitor contract and generated types

**Files:** Modify `contracts/openapi/public.yaml`, `scripts/ci/check-public-contract.mjs`, `scripts/ci/test-check-public-contract.mjs`; create `contracts/fixtures/public/monitor-delete-confirmed.json` and `monitor-delete-unknown.json`; regenerate `apps/web/src/shared/api/public.generated.ts`.

**Interfaces:** operation ID `deleteMonitor` on the existing `/monitors/{monitorId}` path item; success `204` with no content; `404` and `400` use the existing problem schema. Transport lexical constraints (body framing, query, Allow) are documented as such, not invented OpenAPI validation guarantees.

- [ ] Write mutation tests `deleteOperationIsClosed`, `deleteResponsesAreExact`, `deletePreservesExistingOperations`: assert exactly one DELETE method on the resource, 204 has no content schema, 404/400 reference the problem schema, no request body schema, and old operation schemas unchanged. Mutations adding a request body, 201 response or extra method must fail checking.
- [ ] Run root `node scripts/ci/test-check-public-contract.mjs`; record RED for the missing delete operation or an accepted bad mutation.
- [ ] Add only the specified operation/fixtures and semantic assertions. Describe the empty 204, no Location, body-framing and no-query rules explicitly in descriptions.
- [ ] Run semantic GREEN and the existing public-contract CI lint/bundle/check path; run Web `npm run generate:api`, `npm run check:generated`, `npm run typecheck`.
- [ ] Commit explicit contract/checker/generated files: `feat(contracts): define monitor delete operation`.

### Task 2: Deletion use case and port

**Files:** Create `application/delete_monitor.go` and `_test.go`, `ports/monitor_deletion_repository.go`.

**Interfaces:** Port method `Delete(context.Context, domain.MonitorID) error`. Constructor `NewDeleteMonitor(ports.MonitorDeletionRepository) DeleteMonitor`; `Execute(ctx, id) error` mirrors the port. Reuse `ports.ErrMonitorNotFound`, existing application not-found/persistence translation and context cancellation mapping. No new domain sentinel.

- [ ] Add `TestDeleteMonitorRejectsInvalidInputBeforeRepository`, `TestDeleteMonitorMapsNotFoundAndPersistence`: zero MonitorID rejected with zero repository calls; no clock/Checker dependency; canceled context preserved; not-found passed through unchanged; operational errors wrapped without public detail.
- [ ] Run `go test ./internal/modules/monitoring/application`; record focused RED.
- [ ] Implement validation/delegation; leave immutable Monitor untouched.
- [ ] Run focused GREEN plus existing application regression tests.
- [ ] Commit explicit files: `feat(monitoring): add monitor deletion use case`.

### Task 3: Durable serialized deletion transaction

**Files:** Create `adapters/postgres/monitor_deletion.go`, `monitor_deletion_integration_test.go`; modify existing adapter integration tests only where their fixtures must share the stack.

**Interfaces:** Repository implements the Task 2 port: explicit Read Committed transaction executing pre-sweep `DELETE monitoring.check_runs WHERE monitor_id = $1`, `SELECT id FROM monitoring.monitors WHERE id = $1 FOR UPDATE` with not-found detection, idempotent re-sweep, parent `DELETE`, commit before success. Rollback on failure; commit uncertainty returns an error.

- [ ] Write real integration `TestDeleteRemovesMonitorAndAllCheckRuns`, `TestDeleteRepeatedIsNotFound`, `TestDeleteNoOrphanCheckRuns`, `TestReSweepRemovesClaimCommittedBetweenSweepAndLock`, `TestClaimBeforeDeleteRemovesPendingRun`, `TestDeleteBeforeClaimPreventsClaim`, `TestDeleteSerializesWithPauseResume`, `TestRacingDeletesOneCommitsOtherNotFound`, `TestReconciliationOverlapRemainsAtomic`, `TestCompletedRunRemovalKeepsTerminalEvidence`: assert child-first atomicity, no orphans, exact lock ordering, a claim committing between sweep and lock is removed by the re-sweep without a foreign-key failure, delete vs pause/resume both orders, and reconciliation overlap where a lock-wait or deadlock abort surfaces as an error with fully committed or fully rolled-back state. Coordinated sessions and bounded channel/context synchronization; no arbitrary sleeps as race proof.
- [ ] Run root `bash scripts/ci/run-go-postgres-tests.sh` on the actual RED head; require test failure for absent deletion/serialization behavior, not setup/compile-only evidence.
- [ ] Implement the four-statement transaction; no claim, completion or migration SQL changes.
- [ ] Repeat real SQL GREEN plus existing claim/completion/pause-resume integration suites; run Go unit/vet and required Linux race checks. No query performance claim from this suite.
- [ ] Commit explicit files: `feat(monitoring): serialize durable monitor deletion`.

### Task 4: Public delete adapter and composition

**Files:** Create `adapters/http/delete.go`, `delete_test.go`, `apps/api/cmd/api/delete_transport_test.go`; modify `adapters/http/handler.go`, `module.go`, `apps/api/cmd/api/main.go`, relevant composition tests and `main_integration_test.go`; update existing transport tests asserting the resource `Allow` value.

**Interfaces:** `NewModuleWithDeletion(repository, latest, inventory, idGenerator, clock, deletion ports.MonitorDeletionRepository) Module` adds field `DeleteMonitor application.DeleteMonitor`. HTTP constructor: `NewHandlerWithDeletion(register registerMonitor, get getMonitor, latest getLatestCheckResult, availability getMonitorAvailability, list listMonitors, schedulingGet getMonitorScheduling, schedulingSet setMonitorScheduling, deletion deleteMonitor) *Handler`; private interface `deleteMonitor` exposes the Task 2 Execute signature. Preserve legacy constructors. The platform dispatcher retains raw product paths.

- [ ] Add `TestDeleteValidationPrecedence`, `TestDeleteMethodsAndNoStore`, `TestDeleteBodyFramingRejected`, `TestDeleteAliasesNeverRedirect`, `TestDeleteCompositionRemovesAllReads`: assert canonical raw path only, query/fragment/encoded-segment/alias rejection 400, non-DELETE 405 with exact `Allow: GET, DELETE` including HEAD/OPTIONS, no-store on 204/404/405, malformed UUID 400, unknown Monitor 404, declared/streamed body 400, zero use-case calls for every rejected input, and the composed handler answering all eight public operations with the deleted Monitor unknown to get/latest/availability/scheduling/inventory. Use actual TCP transport for alias/HEAD/body behavior, not only recorder tests.
- [ ] Run `go test ./internal/modules/monitoring/adapters/http ./cmd/api`; record specific RED including the stale `Allow: GET` assertions.
- [ ] Implement raw-route method recognition, body-framing rejection, empty 204, problem mapping and composition. Do not refactor unrelated existing HTTP resources.
- [ ] Run focused/full Go GREEN, real composition PostgreSQL tests and public-contract fixture consistency.
- [ ] Commit explicit files: `feat(api): expose durable monitor deletion`.

### Task 5: DELETE gateway mutation

**Files:** Modify Web `server/routes.ts`/`.test.ts`, `server/gateway.ts`/`.test.ts`, `server/server.ts`/`.test.ts`.

**Interfaces:** The `RouteDecision` forward union gains `'DELETE'` for the explicitly mapped monitor resource; scheduling and creation routes unchanged. DELETE joins the explicit mutation set for Origin validation and safe method logging; no body buffering path is added because DELETE forwards bodyless.

- [ ] Add `mapsOnlyMonitorDelete`, `deleteRequiresExactOrigin`, `deleteRejectsBodyFraming`, `deleteRejectsQueryAndAliases`, `deleteKeepsInternalRoutesClosed`: reject absent/null/duplicate/foreign Origin and cross-site metadata, query including bare `?`, aliases, declared/streamed body, and internal routes; assert zero captured upstream calls for rejected input. Existing GET/POST/PUT scheduling, deadline and sanitized error tests remain.
- [ ] Run Web `npm test -- server/routes.test.ts server/gateway.test.ts server/server.test.ts`; record RED.
- [ ] Implement explicit DELETE mutation handling at every existing POST/PUT-specific branch; keep caps, deadlines and forwarding rules unchanged.
- [ ] Run focused GREEN, Web typecheck/lint/boundaries/build and existing gateway/socket regression tests.
- [ ] Commit explicit files: `feat(web): guard and forward monitor deletion`.

### Task 6: Delete client and detail controls

**Files:** Create Web `features/delete-monitor/useMonitorDelete.ts`/`.test.tsx`, `DeleteMonitorControl.tsx`/`.test.tsx`, `index.ts`; modify `entities/monitor/model.ts`, `api.ts`/`.test.ts`, `index.ts`, `pages/MonitorDetailPage.tsx`/`.test.tsx` and typed fake-client fixtures in existing tests. Change styles only where control accessibility/layout requires it.

**Interfaces:** `DeleteOutcome = {kind:'deleted'} | {kind:'uncertain';error:ClientError}`; `MonitorClient.deleteMonitor(id,signal):Promise<DeleteOutcome>`. `useMonitorDelete(id,client)` returns state (idle/confirming/deleting/uncertain), `requestDelete()`, `confirmDelete()`, `cancel()`, `refresh()`; control composes it only while a Monitor is known. Confirmed deletion navigates to the inventory; the exact two-step copy and uncertainty message follow the design.

- [ ] Add `deleteClientMaps204And404ToDeleted`, `deleteClientUncertainOtherwise`, `deleteClientNeverRetries`, `oneInFlightDelete`, `confirmGateBlocksDirectDelete`, `confirmedDeleteNavigatesToInventory`, `uncertainDeleteIsResolvedOnlyByExplicitRefresh`, `controlHiddenWhenMonitorUnknown`. Assert a valid 204 and a known 404 map to deleted; transport/5xx/malformed/other known rejections are uncertain; no automatic retry or rerender-driven delete; the confirm step blocks the first press; refresh during writing is disabled; a follow-up read resolves uncertainty (404 deleted, 200 not deleted); evidence and scheduling views stay independent.
- [ ] Run focused Vitest tests; record RED.
- [ ] Implement exact client outcome mapping and the feature state machine; preserve requestJson deadlines/abort/cache/redirect/credentials policy. No optimistic removal or polling.
- [ ] Run focused and full Web GREEN; generated parity, typecheck, lint, format, boundaries and build. Update existing mocks only to satisfy the expanded client, without weakening old assertions.
- [ ] Commit explicit files: `feat(web): add delete monitor detail controls`.

### Task 7: Composed restart/browser evidence and final review

**Files:** Modify `apps/web/e2e/local-monitoring.spec.ts`, `scripts/ci/verify-web-evidence.mjs`/`scripts/ci/test-verify-web-evidence.mjs`, browser orchestration tests as necessary; update `docs/backend/go-control-plane.md`, `docs/frontend/local-monitoring-web.md`, `docs/testing/local-monitoring-web.md`, `docs/testing/single-checker-execution-slice.md`, `docs/devops/current-handoff.md`, roadmap/index continuation references.

**Interfaces:** Extend the browser artifact with closed deletion evidence `{monitorId,deletedState,readsAfterDelete,inventoryExcludes,checkRunsBeforeDelete,checkRunsAfterDelete,deliveryFailures,restarts}`: exact `deleted` state string, UUID monitor linkage, empty CheckRun set after delete, recorded non-retryable delivery failure for the in-flight journey, and UTC `restarts` checkpoints from Docker inspect proving an API restart between delete and read verification. Orchestrator owns durable SQL snapshots/restart checkpoints; the verifier validates UUID/state/CheckRun linkage and refuses missing checkpoints. Existing inventory/scheduling/raw/availability artifact fields remain unchanged.

- [ ] Add the actual browser journey: inventory -> detail -> delete with confirmation -> reload -> 404 reads and inventory exclusion across an API restart; plus a Checker in-flight journey where a claimed run is deleted before completion and exactly one non-retried delivery failure is observed. Control claim timing through isolated fixture preparation, never a product safety bypass. Preserve existing scheduling/inventory/policy-rejected journeys and independently validate durable rows. Add verifier mutation tests rejecting missing restart checkpoints, orphan CheckRuns and fabricated deletion evidence. Fake CLI checks are orchestration proof only.
- [ ] Run root browser wrapper on both ports with RED assertions before acceptance preparation; record actual Chromium/SQL failure, not absent dependencies. No replay of old browser artifact counts as current-head proof.
- [ ] Implement the isolated preparation/checkpoint capture and minimal verification needed for the journeys. Emit failures within bounded deadlines, clean only task-owned resources, preserve the same DB volume across intra-test restarts.
- [ ] Run actual GREEN on `UPTIME_LAB_WEB_PORT=4173` and `4817`, canonical local smoke, and all affected required CI gates. Update canonical docs to implemented truth; keep historical specs/plans as decisions.
- [ ] Commit acceptance/docs: `test(api): qualify monitor deletion and browser journey`.
- [ ] Request one fresh whole-branch reviewer through the preserved Native workflow, with no inherited implementation history and the configured most capable reviewer. Review all seven tasks together, emphasizing Review Focus, lock/race evidence and contract/gateway/UI consistency. Fix material findings with RED/GREEN in the same feature branch; record any unresolved acceptance evidence explicitly.
- [ ] Require final exact-head affected checks plus aggregate gate; finalize the same feature PR, summarize before/after and seek merge approval. After an authorized merge, verify exact-main CI, archive/remove only owned merged worktrees, clean task stacks/images, and record durable handoff/activity evidence. No deploy.

## Self-review and execution boundary

Design sections 1–2 map to Tasks 2/3/6; section 3 to Tasks 2/3/4; section 4 to Tasks 3/7; section 5 to Tasks 1/4; section 6 to Task 5; section 7 to Task 6; section 8 to Tasks 1–7. Each Review Focus input has an owning test. No task changes Rust, the internal contract, or any migration; required regression gates still run when repository rules demand them.

Review this plan before execution. Preserve Native from the prior accepted workflow unless explicitly changed: implementation in this session through `superpowers:executing-plans`, then a fresh whole-branch review. No product code, migration, Docker stack or paid/external service is created by plan preparation. Missing actual interleaving evidence is an open acceptance item, never a fabricated pass.
