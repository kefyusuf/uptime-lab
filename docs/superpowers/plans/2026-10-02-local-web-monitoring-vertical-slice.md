# Local Web Monitoring Vertical Slice Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the existing monitoring loop usable through a local browser, with a restricted same-origin gateway and reproducible real-browser acceptance.

**Architecture:** React presents Go-owned Monitor, availability and execution facts. A Node HTTP gateway serves compiled assets and forwards exactly four public operations. Canonical Compose retains four unpublished services; an explicit override publishes only loopback Web access.

**Tech Stack:** React/TypeScript, Vite, Node 24.21.0/npm 11.19.1, Vitest/Testing Library, Playwright Chromium, existing Go/Rust/PostgreSQL and OpenAPI 3.1.2.

**Spec:** [Local Web Monitoring Vertical Slice Design](../specs/2026-10-02-local-web-monitoring-vertical-slice-design.md).

**Status:** Review candidate; implementation awaits plan approval and execution-method selection.
**Base:** `main@a12e14685a5ad99868236fad3c9b4af09b5887a0`, merged design #49. Main CI [37023576947](https://github.com/kefyusuf/uptime-lab/actions/runs/37023576947) passed at this base.

## Global Constraints

- Preserve all four existing Go public operations; no OpenAPI, SQL, Go/Rust product behavior or destination-policy change.
- Exactly four Compose services; canonical Compose publishes no ports. Only `compose.web-local.yaml` publishes `127.0.0.1:${UPTIME_LAB_WEB_PORT:-4173}:8080` for Web.
- Validate the configured port as an integer in 1024–65535; allowed browser origins are exactly `http://127.0.0.1:<port>` and `http://localhost:<port>`.
- Fixed upstream `http://api:8080`; strict raw-path/method allowlist, Host/Origin rules, no permissive CORS, redirects or environment-derived outbound proxy.
- Header limit 16 KiB; request body 64 KiB; response body 256 KiB; header/request/upstream deadlines 5/10/10 seconds, with cancellation and termination.
- Gateway API responses always no-store; preserve 204 and Go body/status semantics. Health is process/assets health, not API readiness.
- Non-root/read-only Web image; Node standard-library gateway without product logic; browser/server builds and imports remain separate.
- Frontend direction `app -> pages -> widgets -> features -> entities -> shared`; no empty layer shells or global business shared package.
- Manual refresh only; independent CheckIDs, Go-owned status/reason, explicit snapshot labels, no browser freshness classification.
- Creation is not idempotent. Never automatically retry; any unconfirmed outcome may have created a Monitor and explicit retry can duplicate it.
- English code/docs/product copy, accessible keyboard/label/error/status behavior, no browser inventory or persistent response cache.
- No remote rollout, authentication, listing/history/lifecycle, incident/notification or private-network monitoring.
- Retain RED evidence, focused GREEN output and commit SHA per task. Explain each completed task's before/after value briefly in Turkish.

## Review Focus

- An HTTP response after successful persistence can still be unconfirmed: gateway 502/504, server 5xx and malformed 201 must warn about duplicates; Task 4.
- Partial/late reads after refresh or navigation must not resurrect another Monitor or an old available badge; Task 5.
- Raw encoded/dot paths and duplicate Host headers must not become allowed routes after normalization; Task 2.
- Port collisions or custom ports across worktrees must not broaden bindings or desynchronize browser-origin checks; Task 6.
- Explicit migration and slow Checker startup must not make no-result acceptance race-dependent; Task 6 stages execution and bounds waiting.

## Verified version decisions

Metadata and peer/engine ranges were read from the [official npm registry](https://registry.npmjs.org/) on 2026-10-02. This is compatibility metadata, not an installed dependency graph, audit pass or runtime test. Task 1 must establish those facts; stop and report incompatible/audited dependencies rather than bypass peer checks or silently change pins.

| Dependency | Exact pin | Use |
|---|---|---|
| `react`, `react-dom` | 19.3.0 | Only production package dependencies |
| `typescript` | 5.9.3 | `openapi-typescript` requires `^5.x`; latest TypeScript 7 is excluded |
| `vite` / `@vitejs/plugin-react` | 8.3.2 / 6.1.1 | Build; plugin accepts Vite 8; optional compiler plugins omitted |
| `vitest` / `jsdom` | 5.0.3 / 30.1.1 | Node and DOM tests; engines accept Node 24.21.0 |
| `@testing-library/react` / `@testing-library/dom` | 16.3.3 / 10.4.2 | Component tests; React 19/DOM 10 peers |
| `@testing-library/user-event` / `@testing-library/jest-dom` | 14.6.7 / 7.0.1 | User interaction and assertions |
| `@playwright/test` | 1.63.0 | Version-matched Chromium from this package's install CLI |
| `openapi-typescript` | 7.13.0 | Wire type generation |
| `eslint` / `@eslint/js` | 10.11.0 / 10.0.1 | Flat configuration |
| `typescript-eslint` / `eslint-plugin-react-hooks` | 8.71.0 / 7.1.1 | TS5.9 and ESLint10 compatible peers |
| `prettier` | 3.9.9 | Formatting |
| `@types/react` / `@types/react-dom` / `@types/node` | 19.3.0 / 19.3.0 / 24.19.1 | Match runtime major versions |

Use exact package versions without caret/tilde, `packageManager: npm@11.19.1`, strict Node engine `24.21.0`, `.npmrc` with `engine-strict=true`, and app-local `package-lock.json`. Never use `--force` or `--legacy-peer-deps`. Install npm 11.19.1 explicitly in the build image if its bundled npm differs.

Docker build and runtime base: `node:24.21.0-bookworm-slim@sha256:0e0ff40c39bc087845bfb27465a0df4ea419520094bc35842ff83dd8cbe6f9b6`, manifest digest verified from the official `library/node` registry. Use Debian glibc for native Vite tooling compatibility; no Alpine/native-binary assumption. Verify the digest still resolves before implementation and fail rather than substitute a floating image. Host Node 26.8.2 is not parity evidence; execute the app commands in pinned Node 24.21.0 on this Windows host via the image or supported pinned runtime.

Docker CLI is currently unavailable on this host. During approved execution, use an ignored task-local `.cache/web-node24` runtime from `https://nodejs.org/dist/v24.21.0/node-v24.21.0-win-x64.zip`, verify SHA256 `158f7685b44de51f6c0df1d153526cbcd3e1bc739a8dfc607721cef75de9e541` before extraction, and keep runtime/PATH changes scoped to task commands. Do not install Docker or alter persistent host configuration as a workaround. Tasks3/6 must explicitly record local Docker checks as blocked; their real-image/Compose/browser evidence remains mandatory through Task7's Linux CI. Focused pinned-Node RED/GREEN tests permit code commits while that evidence is pending, never a claim that integration passed. An implementation PR is not merge-ready until those exact-head real CI checks pass.

## File and interface map

| Area | Files/responsibility |
|---|---|
| Toolchain and contract adapter | `apps/web/package.json`, lockfile, `.npmrc`, `tsconfig.json`, `tsconfig.server.json`, `vite.config.ts`, `vitest.config.ts`, `eslint.config.mjs`, `.prettierrc.json`; `src/shared/api/{public.generated.ts,http.ts}`; `src/entities/monitor/{model.ts,decode.ts,api.ts,index.ts}` |
| Gateway | `apps/web/server/{config.ts,routes.ts,proxy.ts,gateway.ts}` and sibling `*.test.ts` |
| Asset runtime | `apps/web/server/{assets.ts,server.ts,main.ts}`, sibling tests, `apps/web/Dockerfile`, `.dockerignore` |
| Registration/navigation | `apps/web/index.html`, `src/app/{App.tsx,router.ts,styles.css}`, `src/pages/CreateMonitorPage.tsx`, `src/features/create-monitor/{CreateMonitorForm.tsx,index.ts}` and sibling tests |
| Detail and refresh | `src/pages/{MonitorDetailPage.tsx,MonitorDetailPage.test.tsx}`, `src/features/refresh-monitor/{useMonitorReads.ts,useMonitorReads.test.tsx,index.ts}`, `src/entities/monitor/{AvailabilityCard.tsx,LatestResultCard.tsx}` and sibling tests |
| Architecture checks | `scripts/ci/{check-web-architecture.mjs,test-check-web-architecture.mjs}` |
| Real composition/acceptance | `compose.yaml`, new `compose.web-local.yaml`, `.env.example`, `apps/web/playwright.config.ts`, `apps/web/e2e/local-monitoring.spec.ts`, `scripts/ci/{run-web-browser-tests.sh,test-run-web-browser-tests.sh,check-web-compose.mjs,test-check-web-compose.mjs}` |
| CI/current truth | `.github/workflows/ci.yml`, `scripts/ci/{detect-web-changes.sh,test-detect-web-changes.sh,detect-local-dev-changes.sh,test-detect-local-dev-changes.sh,check-local-dev.sh,test-check-local-dev.sh,smoke-local-dev.sh,test-smoke-local-dev.sh,check-architecture-docs.sh,test-architecture-docs.sh}`; current docs listed in Task 7 |

Generated wire types live only in the adapter. `model.ts` defines `Monitor {id:string,targetUrl:string,createdAt:string}`, explicit contract-shaped `LatestResult` variants and `Availability` discriminated by valid status/reason/evidence combinations. Export `ClientError {kind:'http'|'transport'|'invalid_response',status?:number,message:string}`, `ReadResult<T> = {kind:'success',data:T}|{kind:'error',error:ClientError}` and `CreateOutcome = {kind:'created',monitor:Monitor}|{kind:'rejected',message:string}|{kind:'uncertain',message:string}`. Presentation imports these models through the entity entrypoint, never generated wire types.

## Task 1: Contract-shaped browser client and enforceable boundaries

**Files:** Toolchain/adapter and architecture-check files above; tests `src/entities/monitor/{decode.test.ts,api.test.ts}`, `src/shared/api/http.test.ts`, `src/test/setup.ts`.

**Interfaces:** `decodeMonitor(value:unknown):Monitor`, `decodeLatestResult(value:unknown):LatestResult`, `decodeAvailability(value:unknown):Availability` throw `ResponseDecodeError` on invalid shape. `createMonitorClient(fetchImpl:typeof fetch):MonitorClient` produces `createMonitor(targetUrl:string,signal:AbortSignal):Promise<CreateOutcome>`, `getMonitor(id:string,signal:AbortSignal):Promise<ReadResult<Monitor>>`, `getLatestResult(id:string,signal:AbortSignal):Promise<ReadResult<LatestResult|null>>`, `getAvailability(id:string,signal:AbortSignal):Promise<ReadResult<Availability>>`; export that named `MonitorClient` interface from the entity entrypoint. `http.ts` exports `requestJson(fetchImpl:typeof fetch,input:string,init:RequestInit):Promise<{status:number,value:unknown}>`; value is null only for bodyless 204. Client requests use relative `/api` URLs, `cache:'no-store'`, no redirects and an AbortSignal.

- [ ] Create the exact manifest/configuration and lockfile as setup for this client deliverable. Use strict TS/browser DOM and separate Node server configs, Vite external build assets/manifest, Vitest Node default and jsdom for `*.test.tsx`, and exact dependency pins above. Do not create unused app layers. Run `npm install --package-lock-only`, then `npm ci`; record resolution/audit failures honestly.
- [ ] Write RED decoder/transport tests including `accepts_all_committed_fixtures`, `rejects_unknown_reason_or_invalid_evidence`, `preserves_zero_duration_and_absent_worker_timeout_fields`, `distinguishes_204_from_bad_json`, `never_reclassifies_http_or_120_second_evidence` and `rejects_unknown_fields_and_invalid_dates_ids`. Read existing fixtures from the repository; add mutations for negative cases. Example assertion: `expect(decodeAvailability(noResult)).not.toHaveProperty('evidence')`; `expect(()=>decodeAvailability({...noResult,evidence:{}})).toThrow(ResponseDecodeError)`.
- [ ] Run `npm test -- src/entities/monitor/decode.test.ts src/entities/monitor/api.test.ts src/shared/api/http.test.ts`; record the missing implementation/assertion failure, not installation failure as RED.
- [ ] Implement decoders from the current closed schemas, all eight availability reasons and three raw variants; never use RFC URL validation to reject a server-preserved target. Generate wire types with `openapi-typescript ../../contracts/openapi/public.yaml -o src/shared/api/public.generated.ts`; add `generate:api` and a deterministic temp-output comparison `check:generated` script. Transport sanitizes errors, caps client waiting at 12 seconds with composed cancellation, and treats 204 without JSON parsing. Creation maps 400/403/405/413/415/422 to definitive rejection; every other non-201 or invalid 201 result is uncertain, never retried.
- [ ] RED architecture-check harness cases for lower-to-higher imports, cross-feature private imports, browser-to-server/Node imports, aliases, dynamic imports and valid downward public entrypoints; then implement `checkWebArchitecture(root:string):string[]` in `check-web-architecture.mjs` using TypeScript 5.9 parser/import resolution. Resolve the pinned compiler using `createRequire` anchored at the repository's `apps/web/package.json`; root scripts cannot resolve app-local dependencies merely by changing cwd. Temporary fixture roots remain the analysis input, not the compiler installation root. Run `node ../../scripts/ci/test-check-web-architecture.mjs` from `apps/web` to GREEN. No regex-only checker that overlooks import forms.
- [ ] Run focused tests, `npm run typecheck`, `npm run lint`, `npm run check:generated`, `npm audit --audit-level=high`, and the boundary checker; record actual counts. Commit `feat(web): add contract-shaped monitor client and boundaries`.

## Task 2: Restricted HTTP gateway

**Files:** Gateway files/sibling tests; reuse the Task 1 test/toolchain. No Compose change yet.

**Interfaces:** `loadWebConfig(env:NodeJS.ProcessEnv):WebConfig` validates `UPTIME_LAB_WEB_PORT` (default4173). Export `WebConfig {browserPort:number,allowedOrigins:readonly string[],upstreamOrigin:string,maxHeaderBytes:number,maxRequestBytes:number,maxResponseBytes:number,headerTimeoutMs:number,requestTimeoutMs:number,upstreamTimeoutMs:number}` with the spec values. `matchApiRoute(rawTarget:string,method:string):RouteDecision` returns `forward {path,method}`, `reject {status,allow?}` or `not_api`; matching precedes normalization. `checkBrowserBoundary(req:IncomingMessage,config:WebConfig,requireOrigin:boolean):{status:403}|null` exports shared Host/Origin validation for app and API routes. `createGateway(config:WebConfig):RequestListener` handles `/api` only and uses `http.request` with fixed direct origin, no environment proxy. Tests may inject a fixed loopback upstream and shorter deadlines through constructor arguments, never browser input or production environment.

- [ ] Write RED actual-HTTP tests `forwards_only_four_operations`, `rejects_encoded_dot_absolute_query_paths`, `rejects_duplicate_or_foreign_host`, `post_requires_matching_origin_and_json`, `rejects_cross_site_and_null_origin`, `filters_headers_and_rewrites_location`, `preserves_204_problem_and_no_store`, `bounds_chunked_requests_and_responses`, `aborts_on_disconnect_and_deadline`, and `rejects_redirects_without_following`. Assert a rejected request never reaches the upstream spy; test accepted lower/upper UUID text plus invalid unreserved ID reaching Go's 400.
- [ ] Run `npm test -- server/config.test.ts server/routes.test.ts server/gateway.test.ts server/proxy.test.ts`; record intended RED failures.
- [ ] Implement config/routes/gateway/proxy interfaces with the spec's exact byte/deadline constants. Inspect `rawHeaders` for duplicate Host; compare Origin to Host and the exact configured set. Route/method/host failures are sanitized. Body/header/response limits terminate work; no forwarded auth/cookies/origin, hop-by-hop headers, unknown Location or redirects. Proxy response body/status is preserved within limits; API no-store is unconditional. Rejecting an incomplete oversized request must not hang the connection.
- [ ] Run focused actual-HTTP tests, typecheck/lint/boundary checks. Commit `feat(web): restrict the local public API gateway`.

## Task 3: Built asset server and reproducible Web image

**Files:** Asset/runtime files/sibling tests, Dockerfile/.dockerignore; minimal `index.html` and browser entry required for serving. UI behavior arrives in Tasks 4–5.

**Interfaces:** `loadAssets(distDir:string):AssetIndex` loads a Vite build manifest and contained files; `serveAsset(req,res,index):boolean` serves recognized assets/page shapes or returns false. `createWebServer(config:WebConfig,assets:AssetIndex):Server` composes health, gateway and static routes; `main.ts` starts port8080 and shuts down within 5 seconds on SIGTERM. Server compiled to `dist-server`, browser to `dist`; no browser/server import crossover.

- [ ] Write RED `serves_only_manifest_assets`, `page_fallback_does_not_swallow_api_or_unknown_assets`, `rejects_encoded_and_symlink_traversal`, `sets_csp_and_cache_headers`, `health_requires_complete_build_assets`, `health_exception_exposes_no_app_api_route`, `shutdown_aborts_remaining_work`. Build a temporary fixture manifest; assert CSP forbids inline/eval/object/frame/base overrides and `nosniff`/`no-referrer` are present.
- [ ] Run `npm test -- server/assets.test.ts server/server.test.ts`; record RED.
- [ ] Implement interfaces, manifest/path containment (including realpath/symlink checks), exact page fallback and method behavior, asset MIME allowlist, required HTML/assets health and fixed status body. Reuse Task 2's `checkBrowserBoundary` for all app routes except `/healthz`. Add `build` for `tsc` plus Vite, and `start` for `node dist-server/main.js`.
- [ ] Create a multi-stage pinned Node image: build with `npm ci`, compile, and copy only `dist`, `dist-server` and necessary runtime metadata. React is bundled; no npm modules are required by the stdlib runtime. Use UID/GID10001, port8080, init at Compose level, read-only operation; no source maps/source/test/development dependencies in the final image.
- [ ] Run focused tests/typecheck/lint/build; `docker build -t uptime-lab-web-task3 -f apps/web/Dockerfile .`, then start the image with `--read-only --user 10001:10001 --publish 127.0.0.1:4173:8080`. Verify `/healthz`, HTML/assets/CSP and rejected `/api/internal/checks/claim`; stop/remove only this task-owned container. Commit `feat(web): serve built assets in a bounded non-root runtime`.

## Task 4: Register and reopen a Monitor

**Files:** Registration/navigation files and `src/app/main.tsx`, `src/features/create-monitor/CreateMonitorForm.test.tsx`, `src/app/router.test.ts`; reuse client and server.

**Interfaces:** `CreateMonitorForm({client,onCreated}:{client:MonitorClient,onCreated:(monitor:Monitor)=>void})`; `parsePage(path:string):{kind:'create'}|{kind:'detail',id:string}|{kind:'missing'}`; `navigate(path:string):void` updates history and route notification. `App` supplies a stable client and composes pages. `CreateMonitorPage` supports Open Monitor by ID without storing inventory.

- [ ] RED labelled/keyboard submission, empty input focus, preservation of exact target text, submission lock, validated201 navigation,422 field error, no automatic retry, Open Monitor URL and back/forward route tests. Parameterize `unconfirmed_create_warns_before_explicit_retry` over timeout, disconnect, gateway502/504,server500 and malformed201; assert `The Monitor may have been created`, duplicate warning and exactly one POST. Native form submission must not bypass the client.
- [ ] Run `npm test -- src/features/create-monitor/CreateMonitorForm.test.tsx src/app/router.test.ts`; record RED.
- [ ] Implement form, navigation and registration page. Use text input rather than strict browser `type=url` validation that could contradict Go's preserved target syntax. Render fixed errors, never problem-body HTML or submitted target links. Keep pending/uncertain/rejected states distinct, announce results accessibly, and permit only an explicit next submission.
- [ ] Run component/router tests and typecheck/lint/boundaries/build. Commit `feat(web): register monitors and navigate by identity`.

## Task 5: Independent assessment/result reads and manual refresh

**Files:** Detail/refresh/card files in the map, card sibling tests; wire the existing detail page route in App.

**Interfaces:** `useMonitorReads(id:string,client:MonitorClient):{monitor:LoadState<Monitor>,availability:LoadState<Availability>,latest:LoadState<LatestResult|null>,refresh:()=>void}`; `LoadState<T>` is idle/loading/success(data)/error(ClientError). `AvailabilityCard({state})`, `LatestResultCard({state})` are presentation only. A generation token controls all reads; only a current successful Monitor allows its subordinate reads.

- [ ] RED all fixture states,204 versus unknown/no_result, absent versus zero fields, different CheckIDs, rawfact versus availability reason labels, old available clearing on refresh, partial read failures, Monitor404 precedence, rapid refresh/navigation and old-response suppression. Example: resolve B's Monitor after navigating A→B, then resolve A's late availability; `expect(screen.queryByText(A.checkId)).not.toBeInTheDocument()`. Resolve an old available request after the current request fails; assert no active Available badge.
- [ ] RED accessibility/snapshot tests: `Assessment at`, `Snapshot; refresh to reassess`, raw `Completed at`, textual status, visible focus, labelled controls, detail-heading focus, and narrow viewport card stacking. Advancing browser timers must not alter Go status or evaluatedAt.
- [ ] Run `npm test -- src/pages/MonitorDetailPage.test.tsx src/features/refresh-monitor/useMonitorReads.test.tsx src/entities/monitor/AvailabilityCard.test.tsx src/entities/monitor/LatestResultCard.test.tsx`; record RED.
- [ ] Implement generation/cancellation and separate card state machines. Refetch Monitor on refresh, clear active old results, independently settle successful subordinate reads, and let current Monitor errors/404 hide result cards. Abort on unmount/navigation. Do not classify codes/time, poll automatically, join CheckIDs or cache responses. Add simple responsive CSS and semantic English copy; preserve UTC values.
- [ ] Run focused component/hook tests and all app checks/build. Commit `feat(web): display server assessment and raw execution snapshots`.

## Task 6: Loopback Compose and real browser acceptance

**Files:** Composition/acceptance files in the map; modify local-dev checker/tests and smoke/tests where they currently require a placeholder. Do not weaken production destination policy.

**Interfaces:** `checkWebCompose(canonical:unknown,local:unknown,port:number):string[]` validates resolved Compose JSON; `run-web-browser-tests.sh` owns a unique project and volume, accepts `UPTIME_LAB_WEB_PORT` default4173, cleans its resources on EXIT/INT/TERM. `test-run-web-browser-tests.sh` uses a fake Docker CLI only to validate orchestration/cleanup, never as real acceptance evidence. Playwright project uses baseURL from the same configured port, one worker for staged runtime state, bounded assertions and no retries masking flaky behavior.

- [ ] RED resolved-config mutation tests for canonical ports,non-loopback/API/DB/Checker mappings, extra service,invalid/default/custom port and divergent allowed origins. RED existing local-dev harness expectations for the replacement Web image and HTTP assets health; preserve non-root/read-only/no-canonical-port invariants.
- [ ] Implement canonical Web image/health configuration with default port env and independent startup; add the explicit local override and optional `.env.example` port. Checker/API/DB remain unpublished. Run `docker compose config --format json` for both canonical and paired override and feed actual JSON to the checker, not text heuristics alone. Retain existing canonical checker assertions and update its obsolete placeholder-only Web assertions.
- [ ] RED orchestration harness cases for migration failure, browser failure, interrupted startup, cleanup, port conflict, two independently captured card IDs and a newer completion arriving before DB verification; durable captured rows must still pass. Implement runner: build; start db/api/web; assert API unready before migration; run existing migration executable; wait API ready; run the browser registration/no-result phase before Checker starts; start real Checker; run refresh/reopen phase. Browser phase1 writes only its created ID and target to a task-owned artifact for phase2; avoid a guessed identity or DB seed masquerading as browser registration.
- [ ] Write `local-monitoring.spec.ts` phase1: create `http://web/`, assert detail target/id, availability unknown/no_result and raw empty; save ID. Phase2 reopens same ID and clicks Refresh with test-side bounded waiting (60seconds) until raw policy_rejected and availability unknown/policy_rejected appear. Capture each card's CheckID/completedAt separately; the runner queries durable terminal rows by each captured ID and Monitor ID, then verifies policy_rejected and the corresponding displayed facts/timestamps. Do not compare either ID to a later `latest` query or require cross-card equality: the live Checker may advance the terminal result between reads. No external website needed.
- [ ] Add image-level rejection checks through the loopback endpoint: internal routes,wrong Host/foreign Origin,form POST and invalid encoded path produce rejection without upstream activity. Verify styles/assets and back/forward/reload in the built image. Confirm fixed/custom port runs (sequential) and explicit collision failure never publish a wildcard.
- [ ] Run harness/config tests, `bash scripts/ci/run-web-browser-tests.sh`, and original `bash scripts/ci/smoke-local-dev.sh` against real Docker. Preserve the `http://web/` preconnection policy rejection, readiness transitions and persistence. Pin `npm exec -- playwright install --with-deps chromium` to the installed app package; do not install global/latest Playwright. Record browser and Docker evidence separately from fixture tests. Commit `feat(web): integrate loopback access and real browser acceptance`.

## Task 7: Exact-head CI and implemented documentation truth

**Files:** CI files in the map; `docs/README.md`, `docs/architecture/{README.md,system-context.md,container-view.md,module-boundaries.md,dependency-rules.md,runtime-flows.md,change-flow.md}`, `docs/devops/{local-development.md,current-handoff.md}`, new `docs/frontend/local-monitoring-web.md`, new `docs/testing/local-monitoring-web.md`.

**Interfaces:** `detect-web-changes.sh <base> <head>` prints exactly true/false using existing fail-safe Git conventions. Include `apps/web/*`, both Compose files, `.env.example`, public contract/fixtures, Web checks/browser runner, API/Checker runtime paths, relevant local smoke/detectors and workflow. Local-dev detector gains Web build paths and override. Workflow `changes` exposes `web`; new `web` job uses exact head, pinned Node/npm, frozen lockfile, required scripts and real-browser runner; `CI / gate` requires web success or justified skipped.

- [ ] RED detector harness cases including missing base,zero base,deleted/renamed files,public contract fixture,API/Checker change,Web lockfile/Compose override,workflow and docs-only false. RED gate/governance checks for missing Web result or cancelled/failed Web job. Keep workflow action SHA pinning and existing Go/Rust/contract jobs intact.
- [ ] Implement detector and job; use Node24.21.0/npm11.19.1, `npm ci`, check:generated,typecheck,lint,format:check,boundaries,tests,build and `npm audit --audit-level=high`. Install matching Chromium/system libraries explicitly and run real browser composition. A 30-minute bounded Web job uploads traces on failure; do not dump target/response bodies or credentials. Connect both public contract and Compose changes to required Web tests. Workflow changes already activate existing runtime checks; preserve that coverage.
- [ ] RED architecture-doc harness for missing Web ownership/read flow/access boundaries and stale placeholder/React-deferred current summaries. Update current truth only now that tasks1–6 exist. Keep historical design/plan statements historical. Document migration/startup,canonical versus override,port conflicts,manual refresh/snapshot semantics,uncertain creation,independent CheckIDs and loopback limits.
- [ ] Run relevant detector/governance/docs harnesses and all required app checks once. Re-run real smoke/browser only for changes affecting their behavior or unresolved failure; no repetitive broad reruns after passing evidence.
- [ ] Commit `ci(web): verify the browser slice and document runtime ownership`. Push implementation branch, create a PR using the repository template, attach it to this chat, and await exact-head CI. Record failures and fixes without claiming readiness from compilation alone.
- [ ] Obtain whole-branch independent correctness/security/test review through the selected execution workflow, resolve findings with narrow RED/GREEN evidence, update handoff to actual branch/head and counts, and present the concrete implementation PR for merge authorization. Do not deploy or merge it autonomously.

## Self-review and execution handoff

Coverage: design sections1–2→Task1; topology→Tasks3/6; gateway/safeguards→Tasks2/3/6; frontend responsibilities→Tasks1/4/5; screens/lifecycles→Tasks4/5; verification/delivery→Tasks6/7. All five Review Focus conditions have explicit owning tests. Task interfaces use the same MonitorClient, ReadResult, LoadState, CreateOutcome and WebConfig names throughout. No product capability or remote-release gate was added.

Required app scripts produced by Tasks1–3: `test` (`vitest run`), `typecheck` (browser+server), `lint`, `format:check`, `generate:api`, `check:generated`, `check:boundaries`, `build`, `start`, `test:e2e`. Commands in tasks are run from `apps/web` unless rooted `scripts/ci` commands explicitly run from the repository. Generated/temporary evidence must remain task-owned and ignored; do not record false GREEN counts in this plan.

The recommended execution method is Native: implement these tightly coupled tasks in this session using `superpowers:executing-plans`, with a fresh whole-branch reviewer before merge. Subagent-driven execution is also available if the user prefers per-task independent implementation/review. Approval of this plan and selection of the method are still pending. Continue on a new short-lived `feat/local-web-monitoring` branch from landed main, with a clean tree, applicable instruction check and verified base; use the worktree skill if isolation is needed. This plan does not itself authorize runtime implementation.
