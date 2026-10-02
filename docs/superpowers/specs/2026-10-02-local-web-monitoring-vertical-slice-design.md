# Local Web Monitoring Vertical Slice Design

**Status:** Written design candidate; awaiting review before implementation planning
**Date:** 2026-10-02
**Base:** `main@873c1a3e8948a36852f04025022392ced86db455`
**Scope:** [Web scope reassessment](2026-10-02-web-public-exposure-scope-reassessment.md)
**Roadmap:** [Industry benchmark and launch roadmap](../../roadmap/2026-10-02-industry-benchmark-and-launch-roadmap.md)

## 1. Agreed intent and current evidence

The user approved merging roadmap #45 and proceeding with its proposed local Web scope. This design serves a contributor on their own machine: register a target, reopen its Monitor by URL, and inspect the Go-owned availability assessment and latest execution fact. It does not select an audience or deployment model for a later remote service.

The base includes four public Go operations, real PostgreSQL persistence, a real Rust Checker, and a placeholder Web service. [Main CI run 37009186917](https://github.com/kefyusuf/uptime-lab/actions/runs/37009186917) passed at this base. No React runtime or browser acceptance is currently implemented. This document proposes the new runtime; passing documentation checks are not Web execution evidence.

Success is a reproducible browser journey through the built Web image and real Go/PostgreSQL topology, preserving Checker destination policy and explicit migrations. The first slice requires no Monitor list, account, history, incident policy, or browser-owned inventory.

## 2. Alternatives and selected proposal

| Approach | Benefit | Cost or constraint |
|---|---|---|
| Built React assets plus a bounded Node HTTP gateway — proposed | One Web image, testable route/origin rules, reuse the repository's pinned Node toolchain | Repository owns a small static server and gateway; security behavior must be tested explicitly |
| Vite development server as canonical Web runtime | Convenient hot reload | Development-server behavior becomes the application access boundary; built-bundle acceptance needs a separate server |
| Built assets plus Nginx | Established static serving and proxy implementation | Adds another image/tool configuration and route/origin rules outside the TypeScript test boundary |

Use React/TypeScript with Vite for the browser build, npm with a committed app-local lockfile, and Node 24.21.0 for build/runtime/CI parity with existing contract jobs. The gateway uses Node standard HTTP/filesystem facilities and contains no product logic. TypeScript compiles browser and server separately. `vite preview` is not the canonical application server; the [Vite guidance](https://vite.dev/guide/static-deploy) describes it as a local build-preview tool.

Proposed testing tools are Vitest, React Testing Library, and Playwright Chromium. Generate adapter-facing public API types from `contracts/openapi/public.yaml` with `openapi-typescript`; runtime decoding still validates response shape. Exact dependency versions, generator version, Docker image tag/digest and compatibility evidence must be pinned in the written implementation plan before installation. No product dependency is added by this design PR.

## 3. Topology and local access

~~~mermaid
flowchart LR
  B[Local browser] -->|127.0.0.1:4173; opt-in mapping| W[Web: built React assets and Node gateway]
  W -->|Four public operations only| G[Go API:8080]
  G --> P[(PostgreSQL)]
  R[Rust Checker] -->|Internal claim and completion| G
~~~

Replace the Web placeholder in canonical `compose.yaml`; keep exactly four services and no canonical host ports. Add an explicitly named `compose.web-local.yaml` override that publishes only `127.0.0.1:4173:8080` for Web. The browser URL is `http://127.0.0.1:4173`; `http://localhost:4173` is also an allowed origin when resolved to IPv4. API, Checker and PostgreSQL remain unpublished. IPv6 publication, LAN access and arbitrary hostnames are outside this slice.

For simultaneous worktrees, the override accepts `UPTIME_LAB_WEB_PORT` with default 4173. Validate an integer port in 1024–65535 at startup; the same value constructs exact allowed origins and the host mapping. Fail on invalid configuration rather than fall back to a wildcard. Document the paired Compose files and isolated project names/volumes. Using canonical Compose alone remains valid for smoke and container-local workflows.

The Web process listens on container port 8080, serves compiled assets, and forwards only to the fixed `http://api:8080` origin. Browser requests cannot supply an upstream URL or target host. No SSR, public proxy, WebSocket, Checker client, database client, or additional service is introduced.

Web `/healthz` is a process/assets health check: 200 only when the built entry and asset manifest are available. It never proxies health routes to Go. It accepts container-local requests without the browser Host rule, returns only a fixed status body, and exposes no operational data. Web starts independently of API readiness; failed API reads produce UI errors. Explicit migration and API/Checker startup sequencing remain unchanged.

## 4. Gateway contract and safeguards

The browser uses an `/api` prefix to separate application routes from public API resources. Strip this prefix only after matching the raw request target against the allowlist:

| Browser method and resource | Fixed Go resource |
|---|---|
| `POST /api/monitors` | `POST /monitors` |
| `GET /api/monitors/{id}` | `GET /monitors/{id}` |
| `GET /api/monitors/{id}/latest-result` | `GET /monitors/{id}/latest-result` |
| `GET /api/monitors/{id}/availability` | `GET /monitors/{id}/availability` |

The ID segment accepts only nonempty ASCII letters, digits and hyphens; Go retains UUID validation and its 400 response. Reject queries, encoded segments, dot segments, backslashes, doubled separators, absolute-form URLs and trailing separators before URL normalization. Recognized resources with unsupported methods return 405 and the exact `Allow` value; other `/api` paths return 404. `/internal`, `/readyz`, `/livez`, and unknown paths never reach Go. Unknown API paths must not fall through to the React shell.

For all application/API routes, require an exact Host value derived from the two configured local browser origins. Never trust `Forwarded` or `X-Forwarded-*`. If Origin is present, require an exact allowed origin matching the request Host; reject `null`. Reject `Sec-Fetch-Site: cross-site`. Creation additionally requires that Origin be present and match, and accepts only `application/json` (optional charset). Reject missing/foreign Origin and form media types before forwarding. There is no CORS opt-in or permissive OPTIONS response. Host/origin checks limit browser-origin abuse and DNS rebinding; they do not authenticate local processes or users sharing the machine.

Bound headers to 16 KiB, creation bodies to 64 KiB, and upstream response bodies to 256 KiB, including chunked bodies. Use 5-second header, 10-second inbound request, and 10-second total upstream deadlines; enforce timeout termination, not just notification. Abort the upstream request when the browser disconnects. Configure no environment-derived outbound proxy. Disable redirect following and reject unexpected upstream redirects as 502. Node's [HTTP documentation](https://nodejs.org/docs/latest-v24.x/api/http.html) distinguishes request/header timeouts and lower-level socket handling; the implementation must test the chosen deadline behavior.

Forward only the method, matched path, JSON body when applicable, and necessary `Accept`/`Content-Type`; never forward cookies, authorization, origin, forwarded or hop-by-hop headers. Preserve normal Go status and body bytes, including 204 and problem responses. Response headers are allowlisted: `Content-Type`, `Allow`, and a validated creation `Location` rewritten from `/monitors/{id}` to `/api/monitors/{id}`. Unexpected Location shapes fail closed as 502. Do not forward cookies. Explicitly use `Cache-Control: no-store` on all gateway API responses, including locally generated errors.

Gateway failures use fixed application-owned error messages: 400 for malformed request targets, 403 for host/origin rejection, 413 for request size, 415 for media type, 502 for invalid/unavailable upstream, and 504 for upstream deadline. Never include internal hostnames, exception messages, SQL, submitted bodies, or target credentials. Emit bounded operational logs for route category, method, status and duration; do not log raw targets or response bodies.

Serve only manifest-listed built assets with correct content types; reject path traversal and do not expose source maps or arbitrary filesystem files. Return the React shell only for `/` and `/monitors/{id}` on GET/HEAD; unknown asset/application paths return 404. HTML uses no-store; fingerprinted assets may use immutable caching. Apply `X-Content-Type-Options: nosniff`, `Referrer-Policy: no-referrer`, and CSP allowing scripts/styles/connections only from self, no inline/eval, no objects, no framing and no base URL override. No service worker or browser persistence of server responses is introduced.

The runtime container uses a non-root user, a read-only filesystem, no source/development dependencies, and `init: true`. SIGTERM stops accepting requests and closes remaining work after a bounded grace period. Loopback HTTP is a local integration boundary; TLS, authentication, authorization, multi-user isolation, quotas and production operations remain remote-release prerequisites.

## 5. Frontend responsibilities and dependency rules

Create only layers with concrete responsibilities under `apps/web/src`:

| Layer | Responsibility |
|---|---|
| `app` | Application composition, two-route navigation, common styles |
| `pages` | Registration and Monitor detail route orchestration |
| `features/create-monitor` | Form state, one explicit creation submission and completion navigation |
| `features/refresh-monitor` | Manual refresh request lifecycle |
| `entities/monitor` | Monitor/read models, public-contract decoding, assessment and raw-result presentation |
| `shared/api` | Fetch transport, generated wire types, HTTP/transport error representation |

No `widgets` directory or generic `shared/common/utils` business package is needed initially. Enforce `app -> pages -> widgets -> features -> entities -> shared`, allowing higher layers to skip unused intermediate layers. Prevent lower layers importing higher ones and cross-feature private imports; expose entity/feature public entrypoints. Keep Node server code in `apps/web/server`, outside browser imports. Browser code cannot import Node modules or server configuration.

Use component-local state and small request hooks, not a global state store or a server-state cache. Navigation supports initial load, browser back/forward and reopening the detail URL. Server fallback serves only the two recognized page shapes. No local Monitor list is inferred from browser storage. Contract generation is deterministic and checked for drift; generated types never substitute for runtime validation.

## 6. Screen flow and request lifecycle

The registration screen has a labelled target URL field, help text explaining HTTP/HTTPS and execution-time destination policy, an explicit Create button, and an Open Monitor by ID field. Validate required input for usability while preserving Go as the contract authority. Target URLs are displayed as escaped text, not active probe links or HTML. Render only fields necessary for this journey; no charts or invented uptime percentages.

Creation is not idempotent: disable submission while pending, never retry automatically, and navigate using the validated Monitor ID from the successful 201 body. A timeout, disconnected response, gateway 502/504, server 5xx, or invalid successful response may follow a persisted Monitor. Treat every unconfirmed creation outcome as `The Monitor may have been created`; explain that explicit retry can create a duplicate. Distinguish definitive input/access rejection from an unconfirmed outcome; do not claim rollback or registration failure merely because the final response cannot confirm success. A successful create makes no claim that the target has been probed or is healthy.

Detail first loads the immutable Monitor. On a successful read, request availability and latest result independently. Separate cards show (a) Go's assessment/reason/evaluatedAt and its evidence CheckID/completedAt, and (b) latest raw execution fact, CheckID/completedAt and contract-optional HTTP status/duration. Never combine evidence across cards. Separate requests can legitimately return different CheckIDs.

Refresh refetches Monitor and both reads without scheduling work. No automatic polling is added. Each navigation/refresh owns abort controllers and a generation token; navigating away or refreshing cancels older requests, and late responses cannot replace newer state. Each card has independent loading, success, empty and error states. On refresh, remove the old active verdict while loading; on failure show an error, not a previous healthy badge. A Monitor 404 takes precedence over result cards; do not display a success card for a Monitor currently reported missing. Invalid IDs and unavailable/server responses have distinct messages.

Every displayed assessment is labelled `Assessment at <evaluatedAt>` and `Snapshot; refresh to reassess`, immediately on initial success and while left open. The browser never decides when that assessment becomes stale, never updates the timestamp locally, and never derives a status from raw facts. Times retain their UTC value and accessible human-readable formatting. Raw cards use `Completed at`, not an assessment label.

| Go availability | Required presentation |
|---|---|
| `available / successful_response` | Available, successful response, server evaluation time and evidence |
| `unavailable / unexpected_http_status` | Unavailable, unexpected HTTP status, evidence |
| `unavailable / probe_failure` | Unavailable, probe failed, evidence |
| `unknown / policy_rejected` | Unknown, execution blocked by destination policy, evidence |
| `unknown / execution_failure` | Unknown, execution could not provide an assessment, evidence |
| `unknown / stale_result` | Unknown, Go reports result too old, evidence |
| `unknown / future_result` | Unknown, Go reports future completion time, evidence |
| `unknown / no_result` | Unknown, no completed result yet, no evidence fields |

Latest-result 204 has its own `No completed result yet` state. For resultKind variants, HTTP responses show status/duration, failure variants show permitted facts, and worker_timeout omits absent fields. Zero duration is valid. Decoders reject malformed JSON, unknown enum variants, incompatible reason/status/evidence combinations and invalid optional-field shapes as response errors. They do not recalculate the 120-second policy or classify HTTP codes. Add fixture parity tests against committed contract fixtures.

Use semantic headings, keyboard-operable forms/buttons, visible focus, programmatic labels and error associations, non-color status text, and restrained live-region announcements. After navigation focus the detail heading; after creation validation focus the first invalid field. A narrow viewport stacks cards without losing identifiers or controls. Product copy is English for this slice; no localization system is introduced.

## 7. Verification design and TDD evidence

The later written plan divides implementation into failing behavior tests, minimal changes and passing checks before meaningful commits. The design PR adds no runtime tests because it adds no runtime behavior.

- Gateway tests exercise actual local HTTP requests against a fixed test upstream: all four routes, method rejection, internal-route rejection, malformed raw paths, hostile/missing origins, Host rebinding, media types, body/response limits, redirects, deadlines, disconnect cancellation, Location validation, header filtering, no-store and static traversal/fallback.
- Frontend tests use contract fixtures and controlled request ordering: creation success/422/uncertain timeout, gateway 502/504, server 5xx and invalid 201 creation responses with duplicate-risk messages and no automatic retry; all raw and availability variants, missing optional fields versus zero values, independent CheckIDs, partial read failure, Monitor 404, refresh/navigation cancellation, old response suppression, malformed responses and snapshot labels.
- Boundary checker tests prove forbidden imports fail, including server-to-browser leakage and reversed/cross-feature private imports. Typecheck, lint, formatting, deterministic generation, build and dependency audit are required.
- Real browser acceptance uses Playwright Chromium against the built Web image with real Go/PostgreSQL. Register `http://web/`, confirm immediate detail/reopen behavior, start the real Checker, then manually refresh until bounded test-side waiting observes policy_rejected and unknown/policy_rejected with matching persistence evidence. Production destination policy remains enabled. Stage the no-terminal browser case before Checker startup to avoid a race. Unit/component fixtures cover HTTP success and other outcomes; no external website is an acceptance dependency.
- Preserve the existing canonical Docker smoke: explicit migrations, API readiness transition, real Rust execution and PostgreSQL persistence. The `http://web/` policy-rejection target remains valid even when Web begins serving port 8080: rejection occurs before any private-network connection. Verify Web health and asset serving separately.
- Inspect resolved canonical/override Compose configuration: exactly four services, only the override's loopback Web mapping, no API/Checker/DB mappings, coherent configured port/origins. Exercise gateway rejection through the running image, not just handler mocks.
- Run current Go/public-contract checks when their monitored dependencies change. Add a Web change detector and job, connect Web-related contract/Compose/script/lockfile changes to relevant browser/integration checks, and include Web job success or justified skip in the existing exact-head CI gate.

CI installs pinned Chromium/system dependencies explicitly following [Playwright CI guidance](https://playwright.dev/docs/ci); the production image contains no browser. Browser failures retain bounded traces without secret/target-body dumps. Build the same Web Dockerfile for canonical local use and CI; do not substitute `vite dev` for acceptance.

## 8. Documentation, delivery and exclusions

Implementation updates current architecture/container/runtime flows, local-development commands, CI/change detection, and a first owned `docs/frontend` document only when React exists. Record this design as a candidate in the index; current documents must continue saying React is deferred until implementation lands. Add a project handoff with actual branch/head, completed checks and next gate when implementation planning finishes.

Use a dedicated implementation branch after approval of this written design and the written plan. Deliver reviewable TDD commits and a PR with actual local/CI evidence; merging still requires the user's concrete authorization. No runtime, dependency, OpenAPI or SQL change belongs in this design PR.

Excluded: remote deployment, user identity/authorization, shared-machine isolation, Monitor listing/history/lifecycle, automatic polling, incident/notification behavior, configurable availability/cadence, private-network probes, multi-checker coordination, infrastructure rollout and production-readiness claims. Adding any of these reopens scope.

The remaining approval is this written design. Exact package/image pins and ordered tasks are the implementation plan's next deliverable, not unresolved product decisions hidden in implementation.
