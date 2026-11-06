# Local Check-Run History Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Expose the retained terminal CheckRun window of one Monitor as a bounded, keyset-paginated read (`GET /monitors/{monitorId}/history`, ninth public operation) and a detail-page history section, mirroring the inventory cursor/budget conventions.

**Architecture:** Go owns the version-`2|` cursor, the existence-probe-plus-page Read Committed transaction over the existing terminal index, and the ninth public operation. The gateway reuses the inventory query validator for the shared grammar. Rust, the internal contract and the migration set are unchanged.

**Tech Stack:** Existing Go1.27.3/pgx/goose, PostgreSQL18, OpenAPI3.1.2, React/TypeScript/Vitest/Playwright, Node24.21.0/npm11.19.1. No new dependencies, no new migration.

**Spec:** [Local Check-Run History design](../specs/2026-11-01-local-check-run-history-design.md), written at `2d511b0` stacking the [scope](../specs/2026-11-01-local-check-run-history-scope-reassessment.md); both merged through PR #71 (`main@1185930`). The user's continuation after the design authorized preparing this plan.

**Status:** Plan awaiting review; no product task is executed. Preserve the prior Native execution method unless the user selects another method: this session implements the dependent tasks, then a fresh whole-branch reviewer checks the completed change. Plan approval is distinct from merge/deployment approval.

## Global Constraints

- Terminal-only: pending CheckRuns never appear; rows carry the existing result vocabulary without verdict, availability or uptime math.
- Ordering `completed_at DESC, check_id DESC`; default 20 rows, maximum 50; empty history is a successful empty page with a null cursor; unknown Monitor is 404.
- The cursor is a version-`2|` 66-byte/88-character keyset position over completed-instant + CheckRun UUID; cross-resource (version-`1|`) cursors are rejected as malformed; decode validates strict round-trip re-encode, canonical timestamp layout and UUID shape before any storage call.
- One explicit Read Committed transaction per read: existence probe plus page query, so a concurrent deletion resolves as a consistent page or a 404 — never rows for a vanished Monitor. While a Monitor exists every terminal row under an anchored cursor persists; no removed-anchor case exists in this increment.
- No new migration; the readiness set stays at four. Rust and `contracts/openapi/internal.yaml` unchanged. Every existing public schema unchanged.
- `GET /monitors/{monitorId}/history` succeeds with 200 JSON `{items,nextCursor}`; matched responses including 404/405 use `Cache-Control: no-store`; unsupported methods answer `Allow: GET`; no query key beyond optional canonical `limit` (1..50, default 20) and optional 88-character `cursor`, each at most once in either order; raw query at most 128 ASCII bytes; response budget 245760 bytes via the existing extension convention.
- Gateway maps the sub-resource with the shared query validator (no widened input), keeps GET free of the Origin requirement and internal routes closed.
- Browser: detail-page-only history section, one read on Monitor success plus explicit Refresh and Next traversal, one in-flight read, stale-response fencing, no polling or fan-out, independent of scheduling/deletion controls.
- Scope excludes retention, target-URL edit, aggregation/uptime math, incidents, notifications, ownership, remote access, quotas and multi-Checker redesign. English code/docs/copy; concise Turkish before/after updates.
- Record focused RED reason, GREEN result and commit per task. Tool/setup failure is blocked verification, not product RED. Preserve user-owned changes; explicit paths only in commits. Allowed scopes: web, api, checker, contracts, devops, architecture, docs, deps — `monitoring` is not a valid scope.

## Review Focus

- A version-`1|` inventory cursor presented to the history route decodes as malformed rather than acting as a foreign position: Task 2 pins the version separation.
- Equal-completion rows tie-break on `check_id`: Task 3 proves deterministic ordering across the page boundary with rows sharing one persisted microsecond instant.
- A claim commits a pending run for the paged Monitor mid-read: Task 3 proves the pending row never appears (terminal-only) and the continuation key names its last included row.
- Deletion races a history read: Task 3 proves page-or-404 atomicity through the shared snapshot, never rows for a vanished Monitor.
- The budget rule: a first oversized item returns fixed 500 and blocks traversal without truncation or skipping, mirroring the inventory rule exactly: Task 4.
- The gateway rejects history query tricks with the shared validator and never forwards an over-grammar query: Task 5.
- A stale history response after navigation or refresh must not replace a newer view; the section stays independent of scheduling and deletion state: Task 6.

## File map and execution prerequisites

Go module paths in tasks are relative to `apps/api/internal/modules/monitoring`; Web paths are relative to `apps/web`. Root paths are written in full. Existing constructors remain available; extend composition through dedicated constructors rather than unrelated restructuring.

Go commands run in `apps/api`; npm commands run in `apps/web`; CI scripts run at repository root. Use installed pinned runtimes or existing Linux CI — the pinned Go is 1.27.3 (`golang:1.27.3-alpine3.24` in `scripts/ci/run-go-postgres-tests.sh`). On Windows, inspect `.cache/web-node24` and `.cache/web-npm11` before using their Node/npm paths; run the browser wrapper with per-invocation `MSYS2_ARG_CONV_EXCL='/usr'` for container paths. Real PostgreSQL uses `bash scripts/ci/run-go-postgres-tests.sh`; it owns its uniquely named stack. Real browsers use `bash scripts/ci/run-web-browser-tests.sh` once per port.

Before runtime execution, verify clean/dirty state, applicable instructions, tools and actual Docker access. Keep the eventual feature PR stacked on the approved docs baseline; never merge a documentation or feature PR without approval. Reuse one draft feature PR for exact-head real SQL/browser RED/GREEN when local execution is unavailable. Pre-write scan note: the history adapter is SELECT-only and SELECT-with-bound-args files have passed the Mimosa pre-write scan; if any formulation is blocked, the deletion plan's pipelined-batch ruling applies. Local web validation must include `format:check` (prettier) alongside typecheck/lint/boundaries/build, and hook implementations must not publish state synchronously inside effects (react-hooks gate).

No worktree or Docker resource is needed for writing this plan. If execution needs a worktree, use `<repo>/.worktrees/<name>` for manually created checkouts, `.git/info/exclude` for exclusion, or the managed worktree lifecycle. Task stacks use unique Compose projects; remove only task-owned volumes/containers/images on completion. Never remove main-stack volumes.

### Task 1: getMonitorHistory contract and generated types

**Files:** Modify `contracts/openapi/public.yaml`, `scripts/ci/check-public-contract.mjs`, `scripts/ci/test-check-public-contract.mjs`; create `contracts/fixtures/public/monitor-history-empty.json`, `monitor-history-page.json`, `monitor-history-continuation.json`; regenerate `apps/web/src/shared/api/public.generated.ts`.

**Interfaces:** operation ID `getMonitorHistory` on the new sub-resource `/monitors/{monitorId}/history`; success 200 with closed `MonitorHistoryPage` (`items` ≤ 50 of closed `MonitorHistoryEntry` — `checkId` uuid, `resultKind`, optional `httpStatus`/`durationMs`, `completedAt` — and `nextCursor` null-or-88-char); `400`/`404`/`405`/`500` use the existing problem schema; `x-max-body-bytes: 245760` response budget; `no-store` on every matched response; `Allow: GET` on 405; query grammar documented as transport lexical constraints, not OpenAPI validation guarantees.

- [ ] Write mutation tests `historyOperationIsClosed`, `historyResponsesAreExact`, `historyPreservesExistingOperations`: assert exactly one GET on the sub-resource, the page/entry schema closure, item cap 50, nullable 88-char cursor, budget exactness, problem references and unchanged old operations. Mutations adding a request body, non-nullable cursor, extra method or status must fail checking.
- [ ] Run root `node scripts/ci/test-check-public-contract.mjs`; record RED for the missing history resource or an accepted bad mutation.
- [ ] Add only the specified schemas/operations/fixtures and semantic assertions; describe the terminal-only rule, cursor grammar and budget explicitly.
- [ ] Run semantic GREEN and the existing public-contract CI lint/bundle/check path; run Web `npm run generate:api`, `npm run check:generated`, `npm run typecheck`.
- [ ] Commit explicit contract/checker/generated files: `feat(contracts): define monitor history operation`.

### Task 2: Version-2 cursor, history port and use case

**Files:** Create `application/history_cursor.go` and `_test.go`, `ports/monitor_history_repository.go`, `application/get_monitor_history.go` and `_test.go`.

**Interfaces:** `EncodeHistoryCursor`/`DecodeHistoryCursor` over the raw `2|completed-instant|checkId` shape (66 bytes, 88 base64url characters, strict decode, canonical re-encode equality, microsecond-layout timestamp, UUID validation) with the stable invalid-cursor sentinel; port `MonitorHistoryRepository.Page(context.Context, domain.MonitorID, *ports.HistoryAnchor, int) ([]ports.HistoryEntry, error)` where a nil anchor names the first page; constructor `NewGetMonitorHistory(ports.MonitorHistoryRepository) GetMonitorHistory` with `Execute(ctx, id, rawCursor, limit)` mirroring the port and reusing `ports.ErrMonitorNotFound`, application not-found/persistence mapping and context cancellation.

- [ ] Add `TestHistoryCursorRoundTripAndRejections`: version-`2|` round trip; version-`1|` and foreign-shape tokens rejected; non-canonical re-encode, bad timestamp layout, bad UUID, wrong length and zero instant rejected before any repository call.
- [ ] Add `TestGetMonitorHistoryRejectsInvalidInputBeforeRepository`, `TestGetMonitorHistoryMapsNotFoundAndPersistence`: zero MonitorID, over-limit, zero-limit and malformed cursors cause zero repository calls; canceled context preserved; not-found passes through unchanged; operational failures map to the stable persistence error without leaking infrastructure detail.
- [ ] Run `go test ./internal/modules/monitoring/application`; record focused RED.
- [ ] Implement validation/delegation; leave immutable Monitor untouched.
- [ ] Run focused GREEN plus existing domain/application regression tests and `go vet ./...`.
- [ ] Commit explicit files: `feat(api): add monitor history cursor and use case`.

### Task 3: Durable history page adapter

**Files:** Create `adapters/postgres/monitor_history.go`, `monitor_history_integration_test.go`; modify existing adapter integration tests only where their fixtures must share the stack.

**Interfaces:** Repository implements the Task 2 port: one explicit Read Committed transaction containing the Monitor existence probe and the page query `WHERE monitor_id = $1 AND completed_at IS NOT NULL [AND (completed_at, id) < ($2, $3)] ORDER BY completed_at DESC, id DESC LIMIT $4`, binary binding per the deletion adapter's discipline, not-found detection inside the same transaction.

- [ ] Write real integration `TestHistoryFirstPageOrdersTerminalRows`, `TestHistoryPagingBoundariesTwentyAndFifty`, `TestHistoryContinuationNamesLastIncludedRow`, `TestHistoryTieBreaksEqualCompletionInstants`, `TestHistoryExcludesPendingRows`, `TestHistoryEmptyAndUnknownMonitor`, `TestHistoryRejectsForeignAndMalformedCursors`, `TestHistoryConcurrentCompletionShiftsPosition`, `TestDeletionRacingHistoryResolvesPageOr404`: assert exact ordering and tie-break, boundary sizes, terminal-only visibility under a mid-read claim, empty-vs-404 distinction, version-`2|` enforcement against a version-`1|` token, cursor position semantics under concurrent completion, and page-or-404 atomicity when deletion races the read. Coordinated sessions and bounded synchronization; no arbitrary sleeps as race proof.
- [ ] Run root `bash scripts/ci/run-go-postgres-tests.sh` on the actual RED head; require test failure for absent paging behavior, not setup/compile-only evidence — a mutation check (dropping the existence probe or the keyset predicate) must reproduce a specific failure before GREEN is recorded.
- [ ] Implement the adapter; no claim, completion or migration SQL changes.
- [ ] Repeat real SQL GREEN plus existing deletion/claim/scheduling suites; run Go unit/vet and required Linux race checks. No query performance claim from this suite.
- [ ] Commit explicit files: `feat(api): page durable check-run history`.

### Task 4: Public history adapter and composition

**Files:** Create `adapters/http/history.go`, `history_test.go`, `apps/api/cmd/api/history_transport_test.go`; modify `adapters/http/handler.go`, `module.go`, `apps/api/cmd/api/main.go`, relevant composition tests and `main_integration_test.go`.

**Interfaces:** `NewModuleWithHistory(repository, latest, inventory, idGenerator, clock, scheduling, history)` adds field `GetMonitorHistory application.GetMonitorHistory` — the history port is separate from the inventory and deletion ports, so the constructor chains `NewModuleWithDeletion` (which requires the scheduling port) rather than a narrower base, per the deletion landing's nil-use-case lesson. HTTP constructor: `NewHandlerWithHistory(...existing capabilities..., history getMonitorHistory) *Handler`; private interface `getMonitorHistory` exposes the Task 2 Execute signature; a sub-resource path helper recognizes `/monitors/{monitorId}/history` canonically (raw-path, query, fragment and alias rejection without redirect follow the scheduling precedent). Preserve legacy constructors.

- [ ] Add `TestHistoryValidationPrecedence`, `TestHistoryQueryGrammar`, `TestHistoryMethodsAndNoStore`, `TestHistoryAliasesNeverRedirect`, `TestHistoryBudget`, `TestHistoryCompositionServesPages`: assert canonical raw path only, strict limit/cursor grammar (default 20, 1..50, one occurrence each, 128-byte raw query, foreign version-`1|` tokens 400), unknown Monitor 404, empty page 200 with null cursor, methods other than GET 405 `Allow: GET` with `no-store`, zero use-case calls for every rejected input, budget rule mirroring the inventory fixed-500-on-first-oversized-item behavior, and the composed handler answering all nine public operations. Use actual TCP transport for alias/HEAD behavior.
- [ ] Run `go test ./internal/modules/monitoring/adapters/http ./cmd/api`; record specific RED.
- [ ] Implement route recognition, query decoding, budget enforcement, page response and composition. Do not refactor unrelated existing HTTP resources.
- [ ] Run focused/full Go GREEN, real composition PostgreSQL tests and public-contract fixture consistency.
- [ ] Commit explicit files: `feat(api): expose monitor history reads`.

### Task 5: Gateway history route

**Files:** Modify Web `server/routes.ts`/`.test.ts`; no changes to `gateway.ts`, `proxy.ts` or `server.ts` unless validation reveals a shared-path necessity.

**Interfaces:** `RouteDecision` gains the `/api/monitors/{id}/history` GET sub-resource mapping reusing the shared limit/cursor validator verbatim — no widened input, no new mutation class, GET requires no Origin.

- [ ] Add `mapsOnlyMonitorHistory`, `historyKeepsQueryGrammar`, `historyKeepsInternalRoutesClosed`: forward canonical history GETs with the query preserved byte-for-byte, reject over-grammar queries (duplicate keys, unknown keys, noncanonical decimals, 129-byte raw queries, percent escapes), aliases, fragments and internal routes; assert existing operations unchanged.
- [ ] Run Web `npm test -- server/routes.test.ts`; record RED.
- [ ] Implement the sub-resource mapping; keep every existing route and cap unchanged.
- [ ] Run focused GREEN, Web typecheck/lint/format:check/boundaries/build.
- [ ] Commit explicit files: `feat(web): map monitor history route`.

### Task 6: History client and detail section

**Files:** Create Web `entities/monitor/history.ts`/`.test.ts`, `features/monitor-history/useMonitorHistory.ts`/`.test.tsx`, `MonitorHistorySection.tsx`/`.test.tsx`, `index.ts`; modify `entities/monitor/model.ts`, `api.ts`/`.test.ts`, `index.ts`, `pages/MonitorDetailPage.tsx`/`.test.tsx` and typed fake-client fixtures in existing tests.

**Interfaces:** `MonitorHistoryPage = {items: HistoryEntry[]; nextCursor: string | null}` with `HistoryEntry = {checkId, resultKind, httpStatus?, durationMs?, completedAt}`; `MonitorClient.getHistory(id, cursor, signal): Promise<ReadResult<MonitorHistoryPage>>`; `useMonitorHistory(id, client)` returns page state (loading/ready/empty/read-error), `refresh()` and `next()` mirroring the inventory hook's stale-response fencing; section composes it only after a Monitor is known and stays independent of scheduling/deletion state.

- [ ] Add `historyDecoderIsClosed`, `historyClientNeverRetries`, `historyClientPreservesCursorBytes`, `historySectionReadsOnceThenExplicitControls`, `historyEmptyPageIsDistinctFromError`, `historyStaleResponsesCannotReplaceNewMonitor`, `historySectionHiddenWhenMonitorUnknown`, `historyKeepsEvidenceIndependent`: assert closed decoding, no automatic retry or polling, cursor bytes preserved exactly across pages, one in-flight read, navigation fencing, empty-page-vs-error distinction and no scheduling/deletion coupling.
- [ ] Run focused Vitest tests; record RED.
- [ ] Implement exact decoder/client/section; preserve `requestJson` deadlines/abort/cache/redirect/credentials policy; no synchronous state publication inside effects (react-hooks gate) and no optimistic insertion.
- [ ] Run focused and full Web GREEN including `format:check`; generated parity, typecheck, lint, boundaries and build. Update existing mocks only to satisfy the expanded client, without weakening old assertions.
- [ ] Commit explicit files: `feat(web): add monitor history detail section`.

### Task 7: Composed restart/browser evidence and final review

**Files:** Modify `apps/web/e2e/local-monitoring.spec.ts`, `scripts/ci/verify-web-evidence.mjs`/`scripts/ci/test-verify-web-evidence.mjs`, `scripts/ci/run-web-browser-tests.sh` with a `scripts/ci/web-history-evidence.mjs` orchestrator as necessary; update `docs/backend/go-control-plane.md`, `docs/frontend/local-monitoring-web.md`, `docs/testing/local-monitoring-web.md`, `docs/testing/single-checker-execution-slice.md`, `docs/devops/current-handoff.md`, roadmap/index continuation references.

**Interfaces:** Extend the browser artifact with closed `history` evidence `{monitorId,firstPageIds,secondPageIds,orderConfirmed,readsAfterRestart}`: UUID linkage to durable rows, ordered completion instants non-increasing across pages, a real second page through a real continuation cursor, and post-restart read statuses. Orchestrator owns durable SQL snapshots; the verifier validates ordering, linkage and page integrity and refuses missing evidence; verifier mutation tests reject fabricated order, broken linkage and missing pages.

- [ ] Add the actual browser journey: register, claim through the real Checker across multiple cadence windows to produce more than one terminal row, then inventory -> detail -> history page 1 -> Next -> page 2 across an API restart, verifying durable linkage and ordering through SQL. Preserve existing journeys. Add verifier mutation tests as above. Fake CLI checks are orchestration proof only.
- [ ] Run root browser wrapper on both ports with RED assertions before acceptance preparation; record actual Chromium/SQL failure, not absent dependencies. No replay of old browser artifact counts as current-head proof.
- [ ] Implement the isolated preparation and minimal verification needed; clean only task-owned resources.
- [ ] Run actual GREEN on `UPTIME_LAB_WEB_PORT=4173` and `4817`, canonical local smoke, and all affected required CI gates. Update canonical docs to implemented truth; keep historical specs/plans as decisions.
- [ ] Commit acceptance/docs: `test(api): qualify monitor history and browser journey`.
- [ ] Request one fresh whole-branch reviewer through the preserved Native workflow, with no inherited implementation history and the configured most capable reviewer. Review all seven tasks together, emphasizing Review Focus, cursor/budget evidence and contract/gateway/UI consistency. Fix material findings with RED/GREEN in the same feature branch; record any unresolved acceptance evidence explicitly.
- [ ] Require final exact-head affected checks plus aggregate gate; finalize the same feature PR, summarize before/after and seek merge approval. After an authorized merge, verify exact-main CI, archive/remove only owned merged worktrees, clean task stacks/images, and record durable handoff/activity evidence. No deploy.

## Self-review and execution boundary

Design sections 1–2 map to Tasks 2/3/6; section 3 to Tasks 2/3; section 4 to Task 3; section 5 to Tasks 1/4; section 6 to Task 5; section 7 to Task 6; section 8 to Tasks 1–7. Each Review Focus input has an owning test. No task changes Rust, the internal contract, or any migration; required regression gates still run when repository rules demand them.

Review this plan before execution. Preserve Native from the prior accepted workflow unless explicitly changed: implementation in this session through `superpowers:executing-plans`, then a fresh whole-branch review. No product code, migration, Docker stack or paid/external service is created by plan preparation. Missing actual paging evidence is an open acceptance item, never a fabricated pass.
