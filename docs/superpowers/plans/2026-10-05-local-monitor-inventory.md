# Local Monitor Inventory Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Discover durable Monitors in bounded local pages and open the existing detail journey without retaining UUIDs.

**Architecture:** Go validates keyset reads through a narrow inventory port and PostgreSQL adapter; its HTTP adapter owns exact JSON budgeting. React displays one cancellable page through the restricted Node gateway. Rust execution, availability policy and existing operations remain unchanged.

**Tech Stack:** Existing Go1.27.1/pgx5.11.0/goose3.28.0, PostgreSQL18, OpenAPI3.1.2, React/TypeScript/Vitest/Playwright, pinned Node24.21.0/npm11.19.1. No new dependencies.

**Spec:** [Approved inventory design](../specs/2026-10-05-local-monitor-inventory-design.md).

**Status:** Review candidate; implementation awaits plan approval and execution-method confirmation.
**Base:** main@d93194358fb779e81bddaeb4add95e4aeb81b0b8, merged design #54. [Exact-main CI](https://github.com/kefyusuf/uptime-lab/actions/runs/37237401208) passed documentation/policy checks; runtime jobs were skipped.
**Execution recommendation:** Native: seven dependent tasks share interfaces; sequential implementation and one fresh whole-branch review. This recommendation does not authorize execution.

## Global Constraints

- Add only GET /monitors through GET /api/monitors; preserve four existing public operations, internal Checker contract, availability and CheckRun mutation behavior.
- Limit default20/max50; only optional canonical limit/cursor once each in either order; at most128 raw ASCII query bytes. Reject empty markers/values, duplicates/unknown keys, percent escapes, plus/semicolon/space/sign/leading-zero forms. Existing gateway POST/resource queries stay rejected.
- Order created_at DESC,id DESC; strict tuple lower bound, no anchor-row lookup or traversal snapshot/count/filter/sort.
- Cursor66 ASCII bytes ->88 raw base64url characters: version1, fixed UTC six-digit microsecond timestamp, canonical lowercase nonzero UUID. Exact date/encoding roundtrip; cursor comes from persisted time, never JS Date.
- Closed page items/nullable nextCursor; exact Monitor fields, at most requested rows. Empty success has null cursor; continuation uses last included row only.
- Inventory JSON body cap245760 bytes; reserve128 envelope/cursor bytes, charge each item plus inter-item comma, final serialization check before writing. Gateway262144-byte cap stays unchanged.
- Fetch at most limit+1/max51 candidates; project target text only at raw length <=245760; oversized values are typed markers. Count lookahead cannot prematurely reject a fitting page.
- Fitting prefix ends before excluded row. First eligible item cannot fit: sanitized500 with "A registered monitor exceeds the inventory response limit." No truncation, skipping or fabricated cursor. Traversal may stop there: approved limitation, not complete inventory for every database.
- Collection GET bodies rejected/no-store; unsupported method405 Allow GET, POST. Preserve existing Go POST query semantics. No target/cursor/raw-query logs.
- Preserve loopback access, fixed upstream, Host/Origin and GET-body checks,16KiB headers/64KiB request/256KiB response and5/10/10s gateway deadlines; browser12s timeout.
- Immutable00003 index migration only; exact migration-set readiness. Start API unready -> explicit migration -> readiness -> Checker. No historical migration edits or rolling-upgrade claim.
- One UI page20 rows/one active request; no backward stack, health fan-out, polling or POST retry. Existing frontend import direction and creation/detail behavior stay valid.
- English copy/code/docs; preserve user changes. No remote/lifecycle/history/incident/auth scope, new dependencies, global tooling or persistent configuration/memory writes.
- Per task record focused RED reason, GREEN output and commit SHA in ignored task evidence, and explain before/after briefly in Turkish. Missing tools mean blocked verification, not RED product evidence.

## Review Focus

- Accepted target grows under HTML escaping: Task4 checks exact bytes and first-item500.
- Equal timestamp UUID ties/pre-persistence nanoseconds lose rows: Tasks2/3 check canonical persisted microsecond cursor and tuple traversal.
- Oversized lookahead/nonexistent anchor spoils a fitting page: Tasks3/4 check existence separately from inclusion.
- Query normalization broadens gateway: Tasks4/5 check raw target grammar on both entry points.
- Cancelled Next overwrites Refresh or becomes empty: Task6 checks generation guards, retry anchor and explicit errors.

## File map and commands

Go paths below are relative to apps/api/internal/modules/monitoring unless otherwise stated. TypeScript paths are relative to apps/web. No module-wide refactoring is proposed.

Go test commands run in apps/api. Root shell scripts run at repository root using Git Bash or Linux CI. Web npm commands run in apps/web with Node24.21.0/npm11.19.1. Existing Windows parity runtime is .cache/web-node24/node-v24.21.0-win-x64; set PATH only for the command and use root .cache/web-npm11/node_modules/npm/bin/npm-cli.js with --prefix apps/web. Go1.27.1 is installed locally.

Docker is absent locally. Mandatory real PostgreSQL/browser RED and GREEN must run through existing Linux CI on the actual execution branch; use a temporary draft execution PR for focused tests before fixing behavior when necessary. Reuse that draft as the final implementation PR. Fake CLI output or compile-only RED is not SQL/browser proof. No global Docker installation.

### Task1: Closed collection contract

**Files:** Modify contracts/openapi/public.yaml, scripts/ci/check-public-contract.mjs, scripts/ci/test-check-public-contract.mjs; create contracts/fixtures/public/monitor-inventory-{page,empty,final}.json; regenerate apps/web/src/shared/api/public.generated.ts.

**Interfaces:** operationId listMonitors; schema MonitorInventoryPage with required closed items:Monitor[] and nextCursor:string|null; limit default20/min1/max50 and cursor88-character base64url pattern. GET200/400/500/405 media/cache/method semantics from design; preserve existing schemas and POST.

- [ ] Add semantic checker/mutation tests inventoryEnvelopeIsClosed, inventoryQueryBoundsAreExact, inventoryPreservesExistingOperations. Fixture assertions:

      assert.deepEqual(empty.items, []);
      assert.equal(empty.nextCursor, null);
      assert.equal(list.operationId, 'listMonitors');
      assert.equal(list.parameters.find(p => p.name === 'limit').schema.maximum, 50);

  Reject extra methods/envelope fields, missing nextCursor, invalid continuation and empty-page continuation; preserve all old checks.
- [ ] Run root node scripts/ci/test-check-public-contract.mjs; record RED for absent operation/schema or accepted invalid mutation.
- [ ] Add operation/schema/fixtures and extend checker with exact GET+POST collection. Describe raw lexical requirements beyond OpenAPI's parameter schema. Use legal fixed-format cursor fixtures, no invented wire shapes.
- [ ] Run focused GREEN, existing CI Redocly2.53.3 lint/bundle and check-public-contract.mjs on bundled JSON; npm run generate:api then npm run check:generated/typecheck. No package changes.
- [ ] Commit Task1 files: feat(contracts): define bounded monitor inventory read.

### Task2: Cursor, inventory port and Go use case

**Files:** Create ports/monitor_inventory_repository.go, application/inventory_cursor.go, application/inventory_cursor_test.go, application/list_monitors.go, application/list_monitors_test.go; modify application/errors.go.

**Interfaces:**

    // ports
    type InventoryAnchor struct { CreatedAt time.Time; ID domain.MonitorID }
    type InventoryCandidate struct {
        ID domain.MonitorID; CreatedAt time.Time; TargetURL string
        TargetBytes int64; Oversized bool
    }
    type MonitorInventoryRepository interface {
        Candidates(context.Context, *InventoryAnchor, int, int) ([]InventoryCandidate, error)
    }
    // application
    type InventoryInput struct { Limit int; Cursor string }
    type InventoryCandidates struct { Items []ports.InventoryCandidate; Limit int }
    func EncodeInventoryCursor(ports.InventoryAnchor) (string, error)
    func DecodeInventoryCursor(string) (ports.InventoryAnchor, error)
    func NewListMonitors(ports.MonitorInventoryRepository) ListMonitors
    func (ListMonitors) Execute(context.Context, InventoryInput) (InventoryCandidates, error)

Add application.ErrInvalidInventoryQuery; reuse ErrPersistence. Limit0 is invalid inside Execute: HTTP supplies20 if omitted. Invoke Candidates with limit+1 and245760, validate returned bounds/key/order/anchor/raw-length-marker consistency. Domain-validate normal targets; oversized marker is not a fabricated Monitor. No HTTP/JSON imports. Context cancellation remains cancellation.

- [ ] Add TestInventoryCursorCanonicalRoundtrip, TestInventoryCursorRejectsAlternativeSpellings, TestListMonitorsBoundsBeforeRepository, TestListMonitorsRejectsInvalidCandidates, TestListMonitorsCancellationAndSanitization. Task-owned fake records arguments/calls:

      token, err := EncodeInventoryCursor(anchor)
      if err != nil || len(token) != 88 { t.Fatal(token, err) }
      got, err := DecodeInventoryCursor(token)
      if err != nil || !got.CreatedAt.Equal(anchor.CreatedAt) || got.ID != anchor.ID { t.Fatal(got, err) }

  Assert decoded length66,123456000ns roundtrip, invalid version/date/UUID/zero time/submicrosecond input, padded/noncanonical base64 bits/excess bytes; limit20 calls fake21/245760, limit51 never calls it. Malformed candidate/error detail must not escape.
- [ ] Run go test ./internal/modules/monitoring/application -run 'Inventory|ListMonitors' -count=1; record focused RED.
- [ ] Implement signatures/positional cursor validation and bounded read use case; keep JSON budgeting out of application. Keys always validated; oversized targets skip normal target construction.
- [ ] Run focused GREEN, go test ./internal/modules/monitoring/... and root bash scripts/ci/check-go-architecture.sh .
- [ ] Commit Task2 files: feat(api): add bounded monitor inventory use case.

### Task3: Actual PostgreSQL retrieval and immutable index

**Files:** Create adapters/postgres/monitor_inventory.go, adapters/postgres/monitor_inventory_integration_test.go, apps/api/migrations/00003_add_monitor_inventory_order_index.sql, apps/api/migrations/monitor_inventory_integration_test.go; update migration expectation tests only as required.

**Interfaces:** Repository.Candidates implements Task2 port. Validate direct maxRows1..51/maxTargetBytes245760 before SQL. Two parameterized SELECT shapes: first page or strict (created_at,id)<anchor; ORDER BY created_at DESC,id DESC; LIMIT maxRows. Project octet_length plus CASE-selected target or NULL/oversized marker. Index monitoring.monitors_inventory_order_idx is nonunique B-tree(created_at DESC,id DESC).

- [ ] Add TestInventoryIndexMigration, TestInventoryEqualTimeUUIDOrder, TestInventoryPersistedMicroseconds, TestInventoryNonexistentAnchorAndConcurrentInsert, TestInventoryOversizedProjectionAndBounds, TestInventoryCancellationDoesNotMutateRuns; reuse existing openIntegrationPool/resetMonitors/mustMonitor:

      if len(first) != 2 || first[0].ID.String() != higherID { t.Fatal(first) }
      if huge[0].TargetURL != "" || !huge[0].Oversized || huge[0].TargetBytes <= 245760 { t.Fatal(huge) }

  Same-time IDs endingcb3 precede cb2;1µs difference, persisted clock rounding, valid nonexistent/beyond-tail anchor, concurrent inserts above/below cursor,51-row cap and CheckRun snapshot unchanged. Prove huge text is not returned through projection.
- [ ] After initial missing-method RED, add a minimal compiling Candidates stub returning an empty slice and leave00003 absent. Run root bash scripts/ci/run-go-postgres-tests.sh for actual SQL RED (expected missing index/wrong rows, not compilation). If blocked locally, save blocked result and obtain that behavioral RED via Linux draft CI's existing integration job before fixing behavior.
- [ ] Implement migration/projection/parameterized queries, close rows and map corruption/cancellation. Do not mutate00001/00002. Update exact migration set/readiness assertions.
- [ ] Repeat actual integration GREEN; verify index presence and representative EXPLAIN, not mandatory index selection for tiny fixtures. Verify missing00003 unready -> explicit migration -> ready. Run immutable migration/architecture checks.
- [ ] Commit Task3 files: feat(api): persist bounded monitor inventory traversal.

### Task4: Public HTTP budgeting and composition

**Files:** Create adapters/http/inventory.go, adapters/http/inventory_test.go; modify adapters/http/handler.go, adapters/http/handler_test.go, apps/api/internal/modules/monitoring/module.go and apps/api/cmd/api/{main.go,main_test.go,main_integration_test.go}.

**Interfaces:** parseInventoryQuery(rawQuery string)(application.InventoryInput,error); caller distinguishes absent query from empty marker using original RequestURI. encodeInventoryPage([]ports.InventoryCandidate,int)([]byte,error); errInventoryItemTooLarge maps to approved fixed500 detail. NewHandlerWithInventory(register registerMonitor,get getMonitor,getLatest getLatestCheckResult,getAvailability getMonitorAvailability,list listMonitors)*Handler; listMonitors interface Execute(context.Context,application.InventoryInput)(application.InventoryCandidates,error). Preserve old constructors. NewModuleWithInventory(repository ports.MonitorRepository,latest ports.LatestCheckResultRepository,inventory ports.MonitorInventoryRepository,idGenerator application.IDGenerator,clock application.Clock) Module adds ListMonitors field; preserve NewModule and wire the new constructor in composition root.

- [ ] Add TestInventoryQueryRawGrammar, TestInventoryPageByteBudgetAndComma, TestInventoryFirstOversizedReturns500, TestInventoryOversizedLookaheadDoesNotFailPrefix, TestInventoryCollectionMethodCacheAndBody and actual cmd/api inventory readback:

      body, err := encodeInventoryPage(candidates, 20)
      if err != nil || len(body) > 245760 { t.Fatal(len(body), err) }

  Decode and assert exact complete targets/cursor from last included row. Pin128/129 query bytes,1/20/50 vs0/51, both parameter orders, duplicates/encodings, GET-body rejection, raw path normalization tricks. HEAD/OPTIONS return405 Allow GET, POST without invoking inventory retrieval. Measure JSON boundary/one byte beyond including commas/Unicode/HTML,50000 ampersands, fitting prefix, first oversize and oversized count-lookahead. Never emit200/partial JSON before validated final size; empty/null/no-store/errors/Allow and legacy POST queries retain intended behavior.
- [ ] Run go test ./internal/modules/monitoring/adapters/http ./cmd/api -run Inventory -count=1; record focused RED.
- [ ] Implement exact raw query/body/method checks, sanitized errors and bounded encoder:128 envelope reserve plus per-item bytes/commas; no skipped row; first oversize500. Wire port through module/handler, no input logs.
- [ ] Run focused/full go test ./..., go vet ./..., go test -race ./...; real cmd/api integration GREEN, public-contract checks and existing pinned govulncheck gate.
- [ ] Commit Task4 files: feat(api): serve byte-bounded monitor inventory pages.

### Task5: Exact collection-only gateway query allowance

**Files:** Create apps/web/server/inventory-query.ts, inventory-query.test.ts; modify server/routes.ts, routes.test.ts, gateway.test.ts. Preserve config/proxy limits.

**Interfaces:** validateInventoryQuery(rawQuery:string):boolean validates lexical grammar only; matchApiRoute retains RouteDecision and forwards validated collection GET path/query. Unsupported exact collection methods Allow GET, POST. Go owns semantic cursor validation.

- [ ] Add Vitest cases collectionGETAcceptsOnlyBoundedQueries, rejectsEncodedDuplicateUnknownQueriesBeforeUpstream, preservesPOSTOriginAndResourceQueryRejection, rejectsGETBodiesAndInternalPaths:

      expect(matchApiRoute('/api/monitors?limit=20','GET')).toEqual({kind:'forward',path:'/monitors?limit=20',method:'GET'});
      expect(matchApiRoute('/api/monitors?limit=020','GET')).toEqual({kind:'reject',status:400});

  Pin88-character cursor, both query orders, second question mark/empty separators,%,+,foreign/duplicate Host, Origin, no internal forwarding, no blanket query allowance and zero upstream calls on rejected inputs.
- [ ] Run npm test -- server/inventory-query.test.ts server/routes.test.ts server/gateway.test.ts; record RED for new collection/query behavior.
- [ ] Implement helper and raw collection-only matching; preserve GET bodies/POST origin and old rejection/redirect/response/deadline behavior. Change old collection GET405 expectation only.
- [ ] Run focused/full Web GREEN, typecheck/lint/format/boundaries; permanent incomplete-header/unfinished-body tests remain passing.
- [ ] Commit Task5 files: feat(web): allow bounded inventory queries through gateway.

### Task6: Strict browser client and bounded inventory feature

**Files:** Create src/entities/monitor/inventory.ts, inventory.test.ts; src/features/monitor-inventory/{useMonitorInventory.ts,useMonitorInventory.test.tsx,MonitorInventory.tsx,MonitorInventory.test.tsx,index.ts}. Modify entity model.ts/api.ts/api.test.ts/index.ts, pages/CreateMonitorPage.tsx/CreateMonitorPage.test.tsx, app/styles.css. Update MonitorClient doubles in app/App.test.tsx, pages/MonitorDetailPage.test.tsx, features/refresh-monitor/useMonitorReads.test.tsx and features/create-monitor/CreateMonitorForm.test.tsx.

**Interfaces:** MonitorInventoryPage{items:Monitor[];nextCursor:string|null}; MonitorClient.listMonitors(input:{limit:number;cursor:string|null},signal:AbortSignal):Promise<ReadResult<MonitorInventoryPage>>. decodeInventoryPage(value:unknown,limit:number):MonitorInventoryPage throws ResponseDecodeError. decodeInventoryProblem(value:unknown):'oversized'|null recognizes only the closed approved500 Problem shape/detail; display fixed client copy, never arbitrary server text. useMonitorInventory(client:MonitorClient) returns {state:LoadState<MonitorInventoryPage>,cursor:string|null,next():void,refresh():void,retry():void}; MonitorInventory({client,onNavigate}:{client:MonitorClient;onNavigate:(path:string)=>void}) composes it into existing start page. Entity API validates caller input and constructs canonical query; cursor remains opaque text.

- [ ] Add tests inventoryRejectsMalformedEnvelope, nextReplacesPage, refreshBeatsLateNext, retryUsesAttemptedAnchor, firstVersusCursorEmpty, creationAndUUIDReopenSurviveInventoryFailure:

      expect(() => decodeInventoryPage({items:[],nextCursor:token},20)).toThrow(ResponseDecodeError);
      expect(listRequests.at(-1)?.input).toEqual({limit:20,cursor:null});
      expect(screen.queryByText(oldTarget)).not.toBeInTheDocument();

  Pin extra keys/duplicate IDs/too many rows/invalid timestamp/cursor, approved oversized500 versus arbitrary500 details (never reflect raw text), inventory204 rejection,12s timeout, single active request/unmount cancellation/generation guard, old response ignored, no per-row health or POST retry, keyboard and modified-click links.
- [ ] Run npm test -- src/entities/monitor/inventory.test.ts src/entities/monitor/api.test.ts src/features/monitor-inventory src/pages/CreateMonitorPage.test.tsx; record RED.
- [ ] Implement strict decoder/default20/one replacing page. Clear stale rows while pending; Next only with continuation, Refresh first page, Retry attempted anchor. Fixed design empty messages; explicit read/oversize errors, exact target wrapping text and local detail links. Existing registration -> detail and UUID reopen remain; remount start reads first page. No cursor localStorage/history, outbound target links, UI sorting/classification.
- [ ] Run focused/full Web GREEN, generated parity, typecheck/lint/format/boundaries/build/audit and existing create/detail/narrow-viewport checks.
- [ ] Commit Task6 files: feat(web): add cancellable local monitor inventory.

### Task7: Composed acceptance and current-state documentation

**Files:** Modify apps/web/e2e/local-monitoring.spec.ts; scripts/ci/run-web-browser-tests.sh, test-run-web-browser-tests.sh, verify-web-evidence.mjs, test-verify-web-evidence.mjs, smoke-local-dev.sh, test-smoke-local-dev.sh. Update canonical docs listed below and explicit operation/index fitness expectations only.

**Interfaces:** Preserve WEB_BROWSER_PHASE register/result and existing captured id/target/raw/availability evidence. Extend artifact with inventory:{ids:string[],selectedId:string}. Add verifyInventoryEvidence(captured:unknown,monitors:unknown):void in verify-web-evidence.mjs and preserve existing verifyEvidence(capture,rows). Read actual inventory Monitor rows separately; never join a later latest result as captured proof.

- [ ] Add browser/orchestration/evidence tests: register >=21 controlled http://web/ targets before Checker, reload start with no retained IDs,20 rows then Next/select captured ID, existing no-result detail semantics, then existing result phase. Pin exact durable target/ID, independent raw/availability CheckIDs; missing/wrong inventory DB row fails verifier.
- [ ] Run root bash scripts/ci/test-run-web-browser-tests.sh and node scripts/ci/test-verify-web-evidence.mjs for focused RED; obtain actual Docker/browser RED in Linux draft CI. Orchestration fixtures alone are insufficient.
- [ ] Extend phased runner and evidence validator; retain random Compose project, loopback/port collision, unready-before-migration, compiled/nonroot/read-only assertions and bounded cleanup of task-owned volumes. Update canonical smoke only for new index/migration expectations.
- [ ] Verify actual Linux root commands: bash scripts/ci/run-go-postgres-tests.sh; bash scripts/ci/smoke-local-dev.sh; bash scripts/ci/run-web-browser-tests.sh; UPTIME_LAB_WEB_PORT=4817 bash scripts/ci/run-web-browser-tests.sh. Require real default/custom-port and canonical smoke at final head.
- [ ] Update README.md, docs/README.md, docs/architecture/{module-boundaries,runtime-flows,change-flow}.md, docs/backend/go-control-plane.md, docs/frontend/local-monitoring-web.md, docs/testing/{public-monitoring-contract,go-public-transport-adapter,local-monitoring-web}.md, docs/devops/{local-development,current-handoff}.md to actual behavior/evidence and oversized500 limitation. Preserve historical designs/plans.
- [ ] Run architecture/docs/repository/immutable migration/whitespace checks and final affected Go/PostgreSQL/public-contract/Web/local-dev jobs plus CI gate. Change CI only if existing detection/evidence coverage demonstrably misses new paths, with focused regression tests; no new mechanism by default.
- [ ] Commit acceptance/docs: test(web): verify durable inventory browser journey. Finalize/attach one implementation PR (reuse draft), request merge only after fresh whole-branch review and exact-head required CI. No deployment.

## Self-review and execution handoff

Coverage is mapped to all ten design sections: interfaces/ownership Task2, contract Task1, cursor Tasks2/3, row/byte budget Tasks3/4, migration Task3, browser Task6, gateway Task5, actual evidence Task7, exclusions/global gates above. Cross-task types/signatures and five Review Focus tests are explicitly assigned. No implementation code is included in this documentation PR.

Before execution inspect clean Git and attachments, then prepare an isolated execution checkout through the established worktree skill; do not reuse archived Web paths. The design is approved; this plan requires user review and execution-method confirmation. Native is recommended for seven tightly connected tasks: primary agent implements sequential TDD, then one fresh whole-branch reviewer. Subagent-driven is the alternative with fresh implementer/reviewer contexts per task. Either choice retains identical scope/evidence and does not authorize remote deployment.
