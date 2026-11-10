# Local Target-URL Edit Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add the tenth public operation, `PATCH /monitors/{monitorId}` with a closed `{targetUrl}` body, editing the target on the immutable Monitor identity through a validated domain transformation, with gateway PATCH guards and a form-based detail-page edit control.

**Architecture:** Go owns the validated target transformation (`Monitor.WithTargetURL`), the atomic single-row edit (one Read Committed transaction, existence probe + `UPDATE ... RETURNING` pipelined through the batch formulation) and the tenth public operation. The gateway adds PATCH to the explicit mutation set. The browser gains a form-based edit control following the deletion control's uncertain-outcome pattern. Rust, the internal contract and the migration set are unchanged.

**Tech Stack:** Existing Go1.27.3/pgx/goose, PostgreSQL18, OpenAPI3.1.2, React/TypeScript/Vitest/Playwright, Node24.21.0/npm11.19.1. No new dependencies, no new migration.

**Spec:** [Local Target-URL Edit design](../specs/2026-11-10-local-target-edit-design.md), written at `1fa09ab` stacking the [scope](../specs/2026-11-10-local-target-edit-scope-reassessment.md); both merged through PR #84 (`main@c8b21bc`). The user's approved continuation after the scope authorized preparing this plan.

**Status:** Plan awaiting review; no product task is executed. Preserve the prior Native execution method unless the user selects another method: this session implements the dependent tasks, then a fresh whole-branch reviewer checks the completed change. Plan approval is distinct from merge/deployment approval.

## Global Constraints

- Identity immutable: the Monitor UUID, creation time, evidence rows, scheduling state, pause/resume and deletion semantics are unchanged. The target becomes the single mutable field through `Monitor.WithTargetURL(targetURL) (Monitor, error)` — same syntactic validation as registration; policy rejection stays a probe-time fact.
- `PATCH /monitors/{monitorId}` succeeds with 200 and the closed Monitor shape (identity, new target, unchanged creation time); 400 (malformed UUID/body/target), 404 (unknown Monitor), 405 (`Allow: GET, PATCH`), 415 (media), 422 (target contract violation), 500 (sanitized persistence); every matched response uses `Cache-Control: no-store`; HEAD/OPTIONS unsupported.
- Body handling mirrors creation: `application/json` only, 16384-byte budget, strict single-document decode rejecting duplicate/unknown keys, alternate representations and trailing JSON; no query keys accepted on the resource.
- In-flight claims probe the target captured at claim time; the edit never rewrites claimed work, history rows, latest-result semantics, availability assessment, retention window or pending uniqueness. History gains no per-row target field (documented limitation).
- No new migration; the readiness set stays at four. Rust and `contracts/openapi/internal.yaml` unchanged. Every existing public schema unchanged.
- Gateway: PATCH joins the explicit mutation set (exact Host/Origin, body buffered at the creation budget 16384, unfinished-body deadline, sanitized errors, no arbitrary verb forwarding); internal routes stay closed.
- Browser: detail-page-only edit control, form-based with pre-filled validated input, one mutation in flight, no automatic retry, uncertain outcomes resolved through explicit refresh, stale responses fenced, independent of scheduling/deletion/history controls.
- Scope excludes history per-row target annotations, revision models, undelete, bulk edits, aggregation, incidents, notifications, ownership, remote access, quotas. English code/docs/copy; concise Turkish before/after updates.
- Record focused RED reason, GREEN result and commit per task. Tool/setup failure is blocked verification, not product RED. Preserve user-owned changes; explicit paths only in commits. Allowed scopes: web, api, checker, contracts, devops, architecture, docs, deps.

## Review Focus

- The domain transformation cannot bypass registration validation: Task 2 pins that an invalid target yields `ErrInvalidTargetURL` and a valid one preserves identity and creation time exactly.
- An edit racing a claim: the claim's pending work probes the target captured at claim time, not the edited one; Task 3 proves the ordering both ways (edit before claim → new target probed; claim before edit → old target probed and completed durably).
- The edit's UPDATE...RETURNING maps through `domain.NewMonitor` so a malformed persisted row is a persistence failure, not a silent success: Task 3.
- PATCH media/budget/grammar handling mirrors creation exactly: Task 4 pins duplicate/unknown keys, alternate representations, trailing JSON, 16384-byte budget and non-JSON media rejection with zero use-case calls.
- Gateway PATCH requires the exact Origin/body-buffering guards and rejects over-budget bodies before upstream: Task 5.
- The browser edit form cannot save an unvalidated target or lose an uncertain outcome: Task 6 pins the gating, the no-retry rule and the refresh resolution.
- Task 7's journey proves a claim after the edit probes the new target and old evidence rows remain durable across an API restart.

## File map and execution prerequisites

Go module paths in tasks are relative to `apps/api/internal/modules/monitoring`; Web paths are relative to `apps/web`. Root paths are written in full. Existing constructors remain available; extend composition through dedicated constructors rather than unrelated restructuring. The edit adapter is a new file using the pipelined batch formulation (the Mimosa pre-write scan blocks variable-arg statements in new files); the claim/history adapters are not renamed. Pinned Go is 1.27.3 (`golang:1.27.3-alpine3.24`); Windows browser wrapper uses per-invocation `MSYS2_ARG_CONV_EXCL='/usr'`; real PostgreSQL uses `bash scripts/ci/run-go-postgres-tests.sh`; real browsers use `bash scripts/ci/run-web-browser-tests.sh` once per port. Local web validation must include `format:check`; hook implementations must not publish state synchronously inside effects.

Before runtime execution, verify clean/dirty state, applicable instructions, tools and actual Docker access. Keep the feature PR stacked on the approved docs baseline; never merge without approval.

No worktree or Docker resource is needed for writing this plan.

### Task 1: editMonitorTarget contract and generated types

**Files:** Modify `contracts/openapi/public.yaml`, `scripts/ci/check-public-contract.mjs`, `scripts/ci/test-check-public-contract.mjs`; create `contracts/fixtures/public/monitor-target-edited.json`, `monitor-target-edit-unknown.json`, `monitor-target-edit-invalid.json`; regenerate `apps/web/src/shared/api/public.generated.ts`.

**Interfaces:** operation ID `editMonitorTarget` on the shared `/monitors/{monitorId}` path item; method PATCH with a closed `MonitorTargetEditRequest` (`targetUrl`, same description/rules as `CreateMonitorRequest.targetUrl`, `additionalProperties: false`, 16384-byte budget); success 200 with the existing closed `Monitor` schema; `400`/`404`/`415`/`422`/`500` problem responses; `405` with `Allow: GET, PATCH`; `no-store` on every matched response; the resource's `Allow` becomes exactly `GET, PATCH, DELETE`.

- [ ] Write mutation tests `editOperationIsClosed`, `editResponsesAreExact`, `editPreservesExistingOperations`: assert exactly one PATCH on the resource, request closure, 200 Monitor content, problem references, Allow exactness, budget, no-store on all matched responses and unchanged old operations. Mutations adding a wrong Allow, missing no-store or an extra method must fail checking.
- [ ] Run root `node scripts/ci/test-check-public-contract.mjs`; record RED.
- [ ] Add only the specified operation/schemas/fixtures and semantic assertions.
- [ ] Run semantic GREEN, redocly lint/bundle and the Web `npm run generate:api`, `npm run check:generated`, `npm run typecheck` chain.
- [ ] Commit explicit contract/checker/generated files: `feat(contracts): define monitor target edit operation`.

### Task 2: WithTargetURL transformation, port and use case

**Files:** Modify `domain/monitor.go` and `domain/monitor_test.go`; create `ports/monitor_target_edit_repository.go`, `application/edit_monitor_target.go` and `_test.go`.

**Interfaces:** `func (monitor Monitor) WithTargetURL(targetURL TargetURL) (Monitor, error)` — same validation as `NewMonitor`, preserving identity and creation time, rejecting the zero value's target; port `MonitorTargetEditRepository.EditTarget(context.Context, domain.MonitorID, domain.TargetURL) (domain.Monitor, error)`; constructor `NewEditMonitorTarget(ports.MonitorTargetEditRepository) EditMonitorTarget` with `Execute(ctx, id, rawTarget)` validating both inputs before the repository call and reusing the not-found/persistence translation.

- [ ] Add `TestMonitorWithTargetURLPreservesIdentityAndCreationTime`, `TestMonitorWithTargetURLRejectsInvalidTarget`, `TestMonitorWithTargetURLRejectsZeroValueTarget`: valid target returns a Monitor with the same identity/creation time and the new target; invalid/zero targets rejected without identity leakage.
- [ ] Add `TestEditMonitorTargetRejectsInvalidInputBeforeRepository`, `TestEditMonitorTargetMapsNotFoundAndPersistence`: zero MonitorID or invalid target cause zero repository calls; not-found passes through unchanged; operational failures map to the stable persistence error; canceled context preserved.
- [ ] Run `go test ./internal/modules/monitoring/domain ./internal/modules/monitoring/application`; record focused RED.
- [ ] Implement the transformation and use case.
- [ ] Run focused GREEN plus existing domain/application regression tests and `go vet ./...`.
- [ ] Commit explicit files: `feat(api): add monitor target edit use case`.

### Task 3: Durable target edit adapter

**Files:** Create `adapters/postgres/monitor_target_edit.go`, `monitor_target_edit_integration_test.go`; no other adapter changes.

**Interfaces:** Repository implements the Task 2 port: one explicit Read Committed transaction, existence probe + `UPDATE monitoring.monitors SET target_url = $2 WHERE id = $1 RETURNING id, target_url, created_at` pipelined through the batch formulation, the returned row mapped through `domain.NewMonitor` (malformed persisted row = persistence failure), not-found detection inside the same transaction, binary binding per the deletion adapter's discipline.

- [ ] Write real integration `TestTargetEditPersistsAndReturnsMonitor`, `TestTargetEditUnknownMonitor`, `TestTargetEditSurvivesPauseResumeAndRetention`, `TestTargetEditRacingClaimProbesOldTarget`, `TestTargetEditRacingDelete`, `TestTargetEditConcurrentLastCommittedWins`: edit persists and returns the full shape; unknown Monitor 404 mapping; edit while paused keeps paused; a claim committed before the edit probes the old target and completes durably; a claim after the edit probes the new target; racing edits are last-committed-wins; deletion still removes everything.
- [ ] Run root `bash scripts/ci/run-go-postgres-tests.sh` on the actual RED head; require test failure for absent edit behavior — a mutation check (dropping the `RETURNING` mapping or the UPDATE's WHERE) must reproduce a specific failure.
- [ ] Implement the adapter; no claim/completion/migration SQL changes.
- [ ] Repeat real SQL GREEN plus the existing claim/completion/pause-resume/deletion/history/retention integration suites.
- [ ] Commit explicit files: `feat(api): persist monitor target edits`.

### Task 4: Public PATCH adapter and composition

**Files:** Create `adapters/http/target_edit.go`, `target_edit_test.go`, `apps/api/cmd/api/target_edit_transport_test.go`; modify `adapters/http/handler.go`, `module.go`, `apps/api/cmd/api/main.go`, relevant composition tests and `main_integration_test.go`.

**Interfaces:** `NewModuleWithTargetEdit(..., scheduling, deletion, history, targetEdit ports.MonitorTargetEditRepository)` adds field `EditMonitorTarget application.EditMonitorTarget`; HTTP constructor `NewHandlerWithTargetEdit(...existing..., targetEdit editMonitorTarget) *Handler`; private interface `editMonitorTarget` exposes the Task 2 Execute signature; the resource's method handling gains PATCH with `Allow: GET, PATCH` and the sub-resource `Allow` values are unchanged.

- [ ] Add `TestTargetEditValidationPrecedence`, `TestTargetEditBodyGrammar`, `TestTargetEditMethodsAndNoStore`, `TestTargetEditAliasesNeverRedirect`, `TestTargetEditCompositionServesMonitor`: PATCH media/budget/grammar mirrors creation (single closed document, duplicate/unknown keys, alternate representations, trailing JSON, 16384-byte budget, non-JSON media 415), invalid target 400/422, unknown Monitor 404, `no-store` on all matched responses, `Allow: GET, PATCH` exactness, zero use-case calls for rejected input, and the composed handler answering all ten public operations. Use actual TCP transport.
- [ ] Run `go test ./internal/modules/monitoring/adapters/http ./cmd/api`; record specific RED.
- [ ] Implement route recognition, body decoding, response and composition.
- [ ] Run focused/full Go GREEN, real composition PostgreSQL tests and public-contract fixture consistency.
- [ ] Commit explicit files: `feat(api): expose monitor target edits`.

### Task 5: Gateway PATCH mutation

**Files:** Modify Web `server/routes.ts`/`.test.ts`, `server/gateway.ts`/`.test.ts`, `server/server.ts`/`.test.ts`.

**Interfaces:** `RouteDecision` gains `'PATCH'` for the bare monitor resource; PATCH joins the explicit mutation set (Origin/boundary, body buffered at the creation budget 16384, unfinished-body deadline, sanitized errors); the shared raw-path canonicality and internal-route closure are unchanged.

- [ ] Add `mapsOnlyMonitorPatch`, `patchRequiresExactOrigin`, `patchForwardsBufferedBody`, `patchRejectsOverBudgetBody`, `patchKeepsInternalRoutesClosed`, `logsMonitorPatch`: canonical mapping, boundary/origin/body assertions mirroring the PUT/DELETE patterns, method logging recognized as PATCH.
- [ ] Run Web `npm test -- server/routes.test.ts server/gateway.test.ts server/server.test.ts`; record RED.
- [ ] Implement PATCH mutation handling at every existing mutation branch.
- [ ] Run focused GREEN, Web typecheck/lint/format:check/boundaries/build.
- [ ] Commit explicit files: `feat(web): guard and forward monitor target patches`.

### Task 6: Edit client and detail form control

**Files:** Create Web `features/edit-monitor-target/useMonitorTargetEdit.ts`/`.test.tsx`, `EditMonitorTargetControl.tsx`/`.test.tsx`, `index.ts`; modify `entities/monitor/model.ts`, `api.ts`/`.test.ts`, `index.ts`, `pages/MonitorDetailPage.tsx`/`.test.tsx` and typed fake-client fixtures.

**Interfaces:** `EditOutcome = {kind:'edited';data:Monitor} | {kind:'rejected';error:ClientError} | {kind:'uncertain';error:ClientError}`; `MonitorClient.editTarget(id, targetUrl, signal): Promise<EditOutcome>`; `useMonitorTargetEdit(id, client)` returns edit-form state (idle/editing/saving/saved/uncertain), `startEdit()`, `save(targetUrl)`, `cancel()`, `refresh()`; the control composes only while the Monitor is known; a saved edit updates the displayed target through the parent refresh path and keeps focus accessible.

- [ ] Add `editDecoderIsClosed`, `editClientNeverRetries`, `oneInFlightEdit`, `editFormGatesSave`, `editedTargetUpdatesDisplay`, `uncertainEditIsResolvedOnlyByExplicitRefresh`, `editSectionHiddenWhenMonitorUnknown`, `editStaysIndependentOfSchedulingDeletionHistory`: assert closed decoding, no automatic retry, form gating, one in-flight save, uncertain resolution, independence and stale fencing.
- [ ] Run focused Vitest tests; record RED.
- [ ] Implement decoder/client/control; preserve `requestJson` policy; no synchronous state publication inside effects (react-hooks gate); no optimistic insertion.
- [ ] Run focused and full Web GREEN including `format:check`; generated parity, typecheck, lint, boundaries and build; update existing mocks without weakening assertions.
- [ ] Commit explicit files: `feat(web): add monitor target edit controls`.

### Task 7: Composed restart/browser evidence and final review

**Files:** Modify `apps/web/e2e/local-monitoring.spec.ts`, `scripts/ci/verify-web-evidence.mjs`/`test-verify-web-evidence.mjs`, `scripts/ci/run-web-browser-tests.sh` with a `scripts/ci/web-target-edit-evidence.mjs` orchestrator as necessary; update `docs/backend/go-control-plane.md`, `docs/frontend/local-monitoring-web.md`, `docs/testing/local-monitoring-web.md`, `docs/testing/single-checker-execution-slice.md`, `docs/devops/current-handoff.md`, `docs/README.md`.

**Interfaces:** Extend the browser artifact with closed `targetEdit` evidence `{monitorId,oldTarget,newTarget,claimAfterEditTarget,readsAfterRestart}`: the journey registers a monitor, claims through the real Checker, edits the target through the real form, verifies a post-edit claim probes the NEW target (SQL-linked), old evidence rows remain durable, and the restarted API serves the edited target. Orchestrator owns durable SQL snapshots; the verifier validates UUID/target linkage and refuses missing checkpoints.

- [ ] Add the actual browser journey with RED assertions before acceptance preparation; verify durable linkage through SQL; preserve existing journeys.
- [ ] Run the root browser wrapper on both ports; record actual Chromium/SQL failure, not absent dependencies.
- [ ] Update canonical docs to implemented truth; keep historical specs/plans as decisions.
- [ ] Commit acceptance/docs: `test(api): qualify monitor target edit and browser journey`.
- [ ] Request one fresh whole-branch reviewer; fix material findings with RED/GREEN in the same feature branch.
- [ ] Require final exact-head affected checks plus aggregate gate; finalize the feature PR and seek merge approval. After an authorized merge, verify exact-main CI and record durable handoff evidence. No deploy.

## Self-review and execution boundary

Design sections 1–2 map to Tasks 2/6/7; section 3 to Tasks 2/3; section 4 to Task 3; section 5 to Tasks 1/4; section 6 to Task 5; section 7 to Task 6; section 8 to Tasks 1–7. Each Review Focus input has an owning test. No task changes Rust, the internal contract, or any migration; required regression gates still run when repository rules demand them.

Review this plan before execution. Preserve Native from the prior accepted workflow unless explicitly changed. No product code, migration, Docker stack or paid/external service is created by plan preparation. Missing actual edit evidence is an open acceptance item, never a fabricated pass.
