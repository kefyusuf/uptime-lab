
# Go Public Transport Adapter Implementation Plan

**Status:** Review candidate
**Date:** 2026-09-23
**Repository:** kefyusuf/uptime-lab
**Scope:** Implement the landed Go Public Transport Adapter design with schema-compatible readiness, exact public HTTP behavior, production composition, local-runtime bootstrap evolution, canonical documentation, and controlled landing
**Base:** main@970865ea42e3abbfb6687da0ae74a801896bc682
**Authoritative design:** docs/superpowers/specs/2026-09-23-go-public-transport-adapter-design.md
**Authoritative public contract:** contracts/openapi/public.yaml

---

## 1. Purpose

The Go Public Transport Adapter design is landed.

This plan makes the already-landed public Monitoring contract executable through the Go Control Plane while preserving every existing product boundary.

The implementation phase will make exactly these operations live inside the Go runtime:

~~~text
POST /monitors
GET  /monitors/{monitorId}
~~~

It will also close the schema-readiness requirement that must be satisfied before those handlers are considered ready.

The phase MUST NOT silently become a general API platform, public deployment milestone, mutable-monitor milestone, internal Checker milestone, Web milestone, or schema-evolution milestone.

---

## 2. Existing Authority

The implementation does not reopen these decisions:

- Go is the Control Plane and exclusive PostgreSQL owner.
- Monitoring owns monitor product semantics.
- The public OpenAPI artifact remains the transport source of truth.
- Public operations remain exactly POST /monitors and GET /monitors/{monitorId}.
- Monitor shape remains exactly id, targetUrl, createdAt.
- Create input remains exactly required targetUrl.
- Duplicate target URLs remain valid.
- Registration remains non-idempotent.
- POST success is 201 Created + Location + Monitor.
- GET success is 200 OK + Monitor.
- Malformed JSON -> 400.
- Invalid textual monitorId -> 400.
- Unsupported/missing/malformed request media type -> 415.
- Valid JSON that violates request/domain semantics -> 422.
- Monitor not found -> 404.
- Current stable application persistence failure -> 500.
- Errors use RFC 9457 Problem Details.
- targetUrl follows current domain parser semantics and intentionally has no RFC 3986 uri format constraint.
- HEAD must not silently execute GET.
- No authentication/authorization/CORS/rate-limit/public exposure is introduced.
- No internal Checker contract is introduced.
- No new persistent product state is introduced.
- API startup never applies migrations.
- Readiness never mutates schema.
- Schema readiness is based on repository migration state, not arbitrary manual-DDL drift detection.
- Landed SQL migrations become immutable repository history.
- Production createdAt is canonicalized to UTC microsecond precision before Monitor construction.
- Canonical Compose remains four services with no host application port.

---

## 3. Phase Boundary

### In scope

- migration-history immutability fitness;
- read-only migration-state compatibility checker;
- real PostgreSQL compatibility evidence;
- Monitoring HTTP adapter;
- request/response/problem mapping;
- method/path behavior including explicit HEAD rejection;
- generic platform HTTP composition seam;
- expanded Go architecture fitness;
- production Monitoring composition in cmd/api;
- safe UUID v7 ID generator wiring;
- microsecond UTC production clock;
- schema-aware /readyz;
- explicit local migration bootstrap evolution;
- Docker smoke of POST/GET through the real runtime;
- canonical documentation and documentation fitness;
- whole-branch verification;
- external review;
- squash landing;
- fresh post-merge main CI.

### Out of scope

- modifying contracts/openapi/public.yaml in the normal implementation path;
- modifying existing SQL migration contents;
- adding a new SQL migration;
- list/search/pagination;
- update/delete/enable/disable;
- idempotency-key support;
- new Monitor fields;
- new tables/columns/indexes;
- internal Checker API;
- scheduling/due-work/results/history;
- Rust source;
- React source;
- auth/authz;
- CORS;
- rate limiting;
- public host ports;
- ingress/TLS;
- generated OpenAPI server/client code;
- an HTTP framework;
- OpenTelemetry;
- request-ID product contract;
- custom error catalog.

If implementation appears to require a public-contract change or a schema migration, STOP and open a separate design/contract reassessment gate.

---

## 4. Implementation Discipline

### 4.1 Branch

After this plan lands, create:

~~~text
feat/go-public-transport-adapter
~~~

from the exact landed plan base.

Do not implement from the design or plan branch.

### 4.2 Pull request lifecycle

Open one draft implementation PR early.

Keep it draft while Tasks 1-6 are being implemented.

Every task closes with:

1. local/fixture verification;
2. explicit self-review;
3. one coherent commit;
4. fresh remote CI on that exact head;
5. no advancement until the required job set is GREEN.

Task 7 performs whole-branch verification and marks the PR review-ready.

Task 8 performs external review and landing.

### 4.3 RED -> GREEN rule

Tests may be written RED inside a task, but a task must never be committed/pushed in a deliberately failing state.

RED evidence is local/task-working-state evidence only.

Each pushed task head must be GREEN.

### 4.4 Scope-drift rule

At every task self-review search specifically for accidental introduction of:

- GET /monitors;
- PUT/PATCH/DELETE;
- lifecycle fields/actions;
- internal Checker routes;
- auth/security schemes;
- CORS;
- public host ports;
- new durable state;
- root Node project;
- generated API code;
- HTTP framework dependency.

Any occurrence outside negative tests/documented deferred text fails the task review.

---

## 5. Reviewed Repository / Tooling Baseline

Implementation starts from the landed repository pins unless the plan gate is explicitly reopened.

~~~text
Go                      1.27.1
PostgreSQL image        postgres:18.6-alpine3.24
pgx                     github.com/jackc/pgx/v5 v5.11.0
goose                   github.com/pressly/goose/v3 v3.28.0
govulncheck             v1.8.0
actions/checkout        3d3c42e5aac5ba805825da76410c181273ba90b1
actions/setup-go        b7ad1dad31e06c5925ef5d2fc7ad053ef454303e
Node contract runtime   24.21.0
Redocly CLI             2.53.3
~~~

This milestone does not require a new dependency.

Before Task 1 starts, confirm these repository pins have not changed on main.

A deliberate major/baseline dependency change returns to plan review.

---

## 6. Target File Map

Expected complete-branch change surface.

### Migration history / schema readiness

~~~text
Create:
scripts/ci/check-migration-history.sh
scripts/ci/test-check-migration-history.sh
apps/api/migrations/compatibility.go
apps/api/migrations/compatibility_test.go
apps/api/migrations/compatibility_integration_test.go

Modify:
.github/workflows/ci.yml
~~~

### Monitoring HTTP adapter

~~~text
Create:
apps/api/internal/modules/monitoring/adapters/http/handler.go
apps/api/internal/modules/monitoring/adapters/http/handler_test.go
~~~

Small helper files may be split inside the same package only when doing so materially improves clarity. Do not create a framework-like transport tree.

### Architecture / platform HTTP seam

~~~text
Modify:
apps/api/internal/platform/httpserver/server.go
apps/api/internal/platform/httpserver/server_test.go
scripts/ci/check-go-architecture.sh
scripts/ci/test-check-go-architecture.sh
~~~

### Production composition

Expected modifications:

~~~text
apps/api/cmd/api/main.go
apps/api/internal/modules/monitoring/application/register_monitor.go
apps/api/internal/modules/monitoring/application/register_monitor_test.go
~~~

Additional existing Monitoring tests may require narrow fixture signature updates if IDGenerator becomes error-returning.

No product/domain fields are added.

### Local runtime

~~~text
Modify:
scripts/ci/smoke-local-dev.sh
scripts/ci/test-smoke-local-dev.sh
scripts/ci/check-local-dev.sh              only if a current invariant needs wording/shape adjustment
scripts/ci/test-check-local-dev.sh         only when checker behavior changes
docs/devops/local-development.md           Task 6 only
~~~

compose.yaml is expected to remain unchanged.

If implementation proves compose.yaml must change, stop and self-review whether the change violates the landed four-service/no-host-port design before proceeding.

### Canonical documentation

Expected as needed after stale-text search:

~~~text
README.md
docs/README.md
docs/architecture/container-view.md
docs/architecture/dependency-rules.md
docs/architecture/runtime-flows.md
docs/backend/go-control-plane.md
docs/devops/local-development.md
docs/testing/go-monitoring-foundation.md
docs/testing/public-monitoring-contract.md
docs/testing/go-public-transport-adapter.md
scripts/ci/check-architecture-docs.sh
scripts/ci/test-architecture-docs.sh
~~~

### Files not expected to change

~~~text
contracts/openapi/public.yaml
apps/api/migrations/*.sql
compose.yaml                     expected unchanged
contracts/openapi/internal.yaml  must remain absent
~~~

A necessary change to the first two categories returns to design/contract reassessment.

---

# Task 1 — Migration immutability + read-only schema compatibility

**Goal:** Close the operational precondition before public handler wiring: landed migrations are immutable and readiness can verify the required schema without mutating the database.

**Primary files:**

- Create scripts/ci/check-migration-history.sh
- Create scripts/ci/test-check-migration-history.sh
- Create apps/api/migrations/compatibility.go
- Create apps/api/migrations/compatibility_test.go
- Create apps/api/migrations/compatibility_integration_test.go
- Modify .github/workflows/ci.yml

## Step 1 — Create the implementation branch and draft PR

Create feat/go-public-transport-adapter from the exact main commit containing this plan.

Open a draft PR with:

~~~text
feat(api): expose public monitoring transport
~~~

Do not add handler/runtime implementation in Task 1.

## Step 2 — Write migration-history guard tests first

The guard compares BASE_SHA -> HEAD_SHA.

It protects SQL files already present in the base revision.

Tests must prove:

### Allowed

- unrelated file change;
- new SQL migration path added in head when absent from base;
- migration provider/helper Go change;
- documentation change.

### Forbidden

- modify an SQL migration present in base;
- delete an SQL migration present in base;
- rename/move an SQL migration present in base;
- replace an old migration path with another file.

The simplest correct invariant is content/path identity for every base-tracked:

~~~text
apps/api/migrations/*.sql
~~~

Every such base path must still exist at HEAD with the same blob content.

New migration paths are not rejected by this checker; this milestone simply does not add one.

Zero-SHA initial-history cases may have no prior immutable set.

A missing non-zero base or missing head must fail closed.

## Step 3 — GREEN the migration-history checker

Implement a network-independent Git-only checker.

Suggested interface:

~~~text
./scripts/ci/check-migration-history.sh <base-sha> <head-sha>
~~~

Success prints a stable success line or no output.

Failure must identify the protected migration path without dumping SQL contents.

## Step 4 — Run migration immutability in the repository job

Add test + check execution to the always-running repository job.

Use the same PR/push base/head range already used by repository/changes policy.

Reason:

Migration immutability is a repository-history invariant and MUST NOT depend on go-api path detection.

The workflow modification will deliberately trigger the normal downstream path-aware jobs on the implementation branch.

Do not create a new required GitHub job solely for this check.

## Step 5 — Design the schema compatibility type before DB code

Create a small compatibility component in the existing migrations package.

Expected shape is conceptually:

~~~text
type CompatibilityChecker struct { ... }

func NewCompatibilityChecker(db *sql.DB) (*CompatibilityChecker, error)
func (checker *CompatibilityChecker) Check(ctx context.Context) error
~~~

Names may vary slightly, but the public surface stays narrow.

The checker must be runtime read-only.

## Step 6 — Derive expected versions from the embedded migration source

Use the pinned goose migration source parsing rather than hand-maintaining a hard-coded version integer.

A safe implementation path:

- construct the existing goose Provider with the embedded FS and supplied database/sql handle;
- use Provider.ListSources only to derive repository-known versions;
- do not call Provider status/version/pending methods in readiness;
- do not call Provider.Up/Down;
- do not close the transient Provider if doing so would close the shared database/sql handle.

Filter expected versions to positive migration versions.

Reject duplicate/invalid repository source version metadata at checker construction.

The checker constructor may fail startup only for invalid local embedded migration metadata; it must not query PostgreSQL at construction time.

## Step 7 — Read DB migration metadata with the read-only Store API

Use the pinned goose database Store API directly.

Read the existing goose version table using ListMigrations or equivalent read-only Store behavior.

Do not call APIs that ensure/create the version table.

Do not issue DDL/DML from Check.

## Step 8 — Implement metadata integrity validation

For the pinned Provider lifecycle, require:

- exactly one version-zero bootstrap row;
- version zero is applied;
- each positive version occurs at most once;
- every retained positive row is applied;
- duplicate/false/unapplied/ambiguous records fail;
- positive DB version set exactly equals positive embedded expected set.

Failure must be a stable internal readiness error; do not expose raw SQL/driver detail through /readyz later.

Do not silently deduplicate suspicious rows.

## Step 9 — Unit-test pure compatibility logic

Unit tests should cover at minimum:

- exact set -> compatible;
- missing zero -> incompatible;
- duplicate zero -> incompatible;
- zero marked false -> incompatible;
- missing required version -> incompatible;
- unknown/ahead version -> incompatible;
- duplicate positive version -> incompatible;
- false/unapplied positive row -> incompatible;
- source version duplication -> construction failure.

Keep set-validation logic testable without PostgreSQL.

## Step 10 — Real PostgreSQL integration tests

Use the existing integration environment.

Prove:

1. fresh database with no goose table -> Check fails;
2. calling Check on the fresh database does not create goose_db_version;
3. after explicit provider.Up -> Check passes;
4. provider.Down -> Check fails;
5. explicit unknown applied metadata -> Check fails;
6. duplicate metadata -> Check fails;
7. false/unapplied metadata -> Check fails;
8. invalid/missing zero -> Check fails;
9. Check does not change row count/content in the goose metadata table.

Restore/reset database state between cases deterministically.

Do not rely on test order.

## Step 11 — Task 1 local verification

At minimum:

~~~bash
./scripts/ci/test-check-migration-history.sh
./scripts/ci/check-migration-history.sh <base> <head>

cd apps/api
go test ./migrations
go test -count=1 -tags=integration ./migrations
~~~

Also run:

~~~bash
./scripts/ci/test-check-go-architecture.sh
./scripts/ci/check-go-architecture.sh .
~~~

to ensure the new migrations package behavior does not affect dependency fitness.

## Step 12 — Task 1 self-review

Explicitly verify:

- no handler exists yet;
- no public route is live;
- no SQL migration changed;
- no schema change;
- no DB mutation in compatibility Check;
- no Provider status/pending/version call in readiness;
- migration immutability is in the repository job;
- new SQL migration addition remains mechanically possible for future phases;
- no contract change.

## Step 13 — Commit + exact-head CI

Suggested commit:

~~~text
feat(api): add schema compatibility readiness foundation
~~~

Require fresh remote CI on the exact head.

Because .github/workflows/ci.yml changed, require:

~~~text
policy           SUCCESS
repository       SUCCESS
changes          SUCCESS
go-api           SUCCESS
local-dev        SUCCESS
public-contract  SUCCESS
CI / gate        SUCCESS
~~~

Do not start Task 2 until GREEN.

---

# Task 2 — Public Monitoring HTTP adapter

**Goal:** Implement the exact two-operation contract behind an isolated Monitoring transport adapter with exhaustive request/error tests.

**Primary files:**

- Create apps/api/internal/modules/monitoring/adapters/http/handler.go
- Create apps/api/internal/modules/monitoring/adapters/http/handler_test.go

No production runtime wiring yet.

## Step 1 — Define narrow use-case interfaces in the adapter

The handler owns small interfaces matching only:

~~~text
RegisterMonitor.Execute(ctx, rawTarget)
GetMonitor.Execute(ctx, monitorID)
~~~

Concrete PostgreSQL types must not appear.

Do not create shared transport interfaces at the repository root.

## Step 2 — Write RED handler tests before implementation

Cover exact success/error behavior.

### POST success

Prove:

- only POST /monitors executes RegisterMonitor;
- 201;
- Content-Type exactly application/json;
- Location exactly /monitors/{id};
- response JSON contains exactly id, targetUrl, createdAt;
- targetUrl text is unmodified;
- createdAt uses canonical UTC value supplied by application result.

### POST transport classification

Prove:

- missing Content-Type -> 415 Problem;
- unsupported Content-Type -> 415 Problem;
- malformed Content-Type -> 415 Problem;
- application/json with valid parameters is accepted;
- empty body -> 400 Problem;
- malformed JSON -> 400 Problem;
- multiple JSON documents -> 400 Problem;
- root scalar/array/null -> 422 Problem;
- missing targetUrl -> 422 Problem;
- extra property -> 422 Problem;
- non-string targetUrl -> 422 Problem.

### POST domain/application mapping

Prove:

- domain.ErrInvalidTargetURL -> 422;
- application.ErrPersistence -> 500;
- unexpected internal error -> 500;
- no raw error string leaks.

### GET

Prove:

- GET /monitors/{id} -> 200 exact Monitor JSON;
- invalid textual UUID -> 400 Problem and GetMonitor is not executed;
- application.ErrMonitorNotFound -> 404;
- application.ErrPersistence -> 500;
- unexpected error -> 500.

### Unsupported methods

For both resource shapes:

- HEAD does not execute GET;
- GET /monitors returns 405 Method Not Allowed, sets Allow: POST, and does not become a list operation;
- POST /monitors/{monitorId} returns 405 Method Not Allowed, sets Allow: GET, and does not execute an application use case;
- PUT/PATCH/DELETE/OPTIONS do not execute an application use case on either known resource shape;
- method-not-allowed responses set the exact Allow header for the known resource path;
- trailing-slash/nested paths outside the two exact resource shapes remain 404.

Unknown paths remain 404.

## Step 3 — Implement deterministic JSON syntax vs shape classification

Do not classify by brittle decoder error-string text.

Preferred sequence:

1. validate Content-Type using mime.ParseMediaType;
2. decode one JSON document into generic JSON representation;
3. require EOF after the first document;
4. validate exact object shape and targetUrl string type;
5. call RegisterMonitor.

Malformed JSON is 400.

Valid JSON with wrong product/request shape is 422.

Do not normalize targetUrl.

Do not impose a body-size constraint or 413 in this milestone.

## Step 4 — Implement explicit path/method routing

Do not use a GET method pattern that can implicitly treat HEAD as GET.

Use explicit path recognition plus method guards.

Recognize exactly:

~~~text
/monitors
/monitors/{single-id-segment}
~~~

Do not accept nested extra segments as the monitor resource.

## Step 5 — Implement success DTO mapping

Transport struct contains exactly:

~~~text
id
targetUrl
createdAt
~~~

createdAt serializes the already-canonical UTC instant with RFC 3339 fractional precision preserved.

No domain struct is encoded directly.

No extra metadata.

## Step 6 — Implement sanitized Problem Details

Use application/problem+json.

Emit only the landed base fields needed now:

~~~text
type
title
status
detail
~~~

Use type about:blank.

Omit instance until a request/correlation identity contract exists.

Marshal response bytes before writing status where practical so serialization failure cannot produce a half-written success shape.

Do not expose internal errors.

## Step 7 — Task 2 verification

At minimum:

~~~bash
cd apps/api
go test ./internal/modules/monitoring/adapters/http
go test -race ./internal/modules/monitoring/adapters/http
~~~

Then full Go unit/race suite.

## Step 8 — Task 2 self-review

Verify:

- only two product operations;
- no HTTP framework;
- no postgres/pgx/goose/platform import from the handler package;
- HEAD does not call GetMonitor;
- targetUrl remains raw;
- exact status/media mapping;
- no auth/CORS/rate limit;
- no production wiring yet;
- OpenAPI unchanged.

## Step 9 — Commit + exact-head CI

Suggested commit:

~~~text
feat(api): add public monitoring HTTP adapter
~~~

Require all branch-triggered jobs GREEN.

---

# Task 3 — Generic platform HTTP seam + architecture fitness

**Goal:** Let the generic platform server host a product handler while preserving operational-route ownership and mechanically protect dependency direction.

**Primary files:**

- Modify apps/api/internal/platform/httpserver/server.go
- Modify apps/api/internal/platform/httpserver/server_test.go
- Modify scripts/ci/check-go-architecture.sh
- Modify scripts/ci/test-check-go-architecture.sh

No cmd/api Monitoring production wiring yet.

## Step 1 — Evolve the readiness abstraction

Replace the DB-specific naming with a generic readiness capability, conceptually:

~~~text
type ReadinessChecker interface {
    Check(context.Context) error
}
~~~

/readyz remains bounded by the existing readiness timeout.

The platform server must not know goose, migrations, pgx, or Monitoring.

## Step 2 — Add a generic product handler seam

The server constructor may accept an http.Handler for product traffic.

Operational endpoints remain owned by platform:

~~~text
/livez
/readyz
~~~

They must take precedence over the product handler.

Unknown non-operational paths may delegate to the product handler, which decides its own 404/405 behavior.

Nil-handler behavior must be explicit and safe in tests; do not create a placeholder product endpoint.

## Step 3 — Update platform tests

Prove:

- /livez unaffected;
- /readyz calls the generic readiness checker;
- readiness timeout behavior preserved;
- nil/unavailable readiness -> 503;
- product handler receives product paths;
- operational routes are not shadowed by product handler;
- graceful shutdown behavior unchanged.

## Step 4 — Expand architecture checker tests first

Add fixture tests that fail when:

- platform imports any Monitoring package;
- application imports net/http;
- Monitoring HTTP adapter imports postgres adapter;
- Monitoring HTTP adapter imports platform;
- Monitoring HTTP adapter imports pgx;
- Monitoring HTTP adapter imports goose.

Positive fixture must allow:

- cmd/api to import both platform and Monitoring adapters;
- Monitoring HTTP adapter to import application/domain.

## Step 5 — GREEN architecture fitness

Implement only the required rules.

Do not create generic architecture machinery beyond current package prefixes.

## Step 6 — Task 3 self-review

Verify:

- platform remains business-module agnostic;
- no Monitoring production composition yet;
- no readiness DB query introduced into httpserver;
- architecture checker protects the new adapter boundary;
- operational semantics unchanged.

## Step 7 — Commit + exact-head CI

Suggested commit:

~~~text
refactor(api): add generic product HTTP composition seam
~~~

Require exact-head CI GREEN.

---

# Task 4 — Production Monitoring composition

**Goal:** Wire real Monitoring application/persistence/HTTP behavior into cmd/api with safe ID/time composition and schema-compatible readiness.

**Primary files:**

Expected:

- Modify apps/api/cmd/api/main.go
- Modify apps/api/internal/modules/monitoring/application/register_monitor.go
- Modify application tests/fixtures affected by IDGenerator signature

No Compose change yet.

## Step 1 — Make ID generation error-safe

Current IDGenerator returns MonitorID directly.

For production composition, avoid panic-based wrapping of uuid.NewV7.

Evolve the internal application generator contract to:

~~~text
func() (domain.MonitorID, error)
~~~

or an equivalently explicit error-safe form.

RegisterMonitor must propagate generator failure as an internal application failure path.

Do not map generator failures to 422; only user-controlled target validation is client-invalid.

Update all application tests deterministically.

This is an internal refactor, not a public contract change.

## Step 2 — Production UUID v7 generator

Composition wraps:

~~~text
uuid.NewV7()
~~~

through domain.NewMonitorID.

No client ID and no DB-generated ID.

Add a narrow unit test/helper test if needed to prove non-zero v7 output without coupling public contract to UUID version.

The public response still promises UUID format only.

## Step 3 — Production microsecond UTC clock

Composition supplies conceptually:

~~~text
func() time.Time {
    return time.Now().UTC().Truncate(time.Microsecond)
}
~~~

Do not change the domain Monitor constructor's general UTC normalization contract merely for one persistence adapter.

Add tests proving production clock output is UTC and has no sub-microsecond remainder.

## Step 4 — Reuse the existing pgx pool for database/sql migration metadata access

Use the pinned pgx stdlib bridge:

~~~text
stdlib.OpenDBFromPool(pool)
~~~

Do not create a second independent PostgreSQL pool.

Lifecycle requirements:

- close the database/sql wrapper during shutdown;
- verify closing it does not close the underlying pgx pool with the pinned pgx behavior;
- pool remains the Monitoring repository owner.

## Step 5 — Construct schema compatibility checker without DB query at startup

Construct the checker from:

- shared database/sql wrapper;
- embedded migrations.

Construction may fail on invalid local migration metadata.

It must not make startup depend on live PostgreSQL reachability.

## Step 6 — Compose Monitoring

In cmd/api:

~~~text
pool
 -> postgres Repository
 -> error-safe UUID v7 generator
 -> microsecond UTC clock
 -> Monitoring Module
 -> Monitoring HTTP Handler
 -> platform Server
~~~

Pass the schema compatibility checker as readiness capability.

No migration Up call.

No internal Checker route.

## Step 7 — Preserve live != ready

A process with unreachable/unmigrated DB may still start and serve /livez.

Do not Ping/schema-check before server startup.

The live DB/schema query happens only through readiness requests.

## Step 8 — Production-composition test evidence

At unit/integration level prove at minimum:

- RegisterMonitor generator error does not panic;
- generator error is not treated as target validation;
- microsecond timestamp round-trips through PostgreSQL unchanged;
- POST result createdAt and subsequent GET createdAt represent the exact same instant;
- API readiness checker failure produces 503 through platform server;
- migrated schema makes readiness pass.

Prefer tests at component boundaries rather than introducing a large dependency-injection framework around main.

## Step 9 — Task 4 self-review

Verify:

- cmd/api is the only new place coupling platform + Monitoring adapters;
- no startup migration;
- one PostgreSQL pool;
- schema checker read-only;
- Monitor ID/time server-owned;
- no contract/schema change;
- no public exposure.

## Step 10 — Commit + exact-head CI

Suggested commit:

~~~text
feat(api): compose public monitoring runtime
~~~

Require full exact-head CI GREEN.

At this head product handlers are live inside the process, but canonical local bootstrap/smoke is not yet updated. Do not mark the PR review-ready.

---

# Task 5 — Explicit local bootstrap + real Docker product smoke

**Goal:** Adapt canonical local development to schema-aware readiness without auto-migration or adding services, and prove the live product routes through the real container runtime.

**Primary files:**

- Modify scripts/ci/smoke-local-dev.sh
- Modify scripts/ci/test-smoke-local-dev.sh
- Modify local-dev checker/tests only if necessary

compose.yaml is expected unchanged.

## Step 1 — Update the fake-Docker smoke harness first

Test the new control flow before editing real smoke.

The harness must verify the canonical sequence includes:

1. build/start db + api without waiting for api readiness;
2. explicit migration command inside api container;
3. full-stack up --wait after migration;
4. product POST;
5. product GET;
6. persistent down/up behavior;
7. destructive down -v fresh-schema behavior;
8. explicit migration again after destructive reset;
9. deterministic cleanup.

## Step 2 — Evolve real bootstrap

Fresh bootstrap must not use:

~~~text
docker compose up -d --build --wait
~~~

as the first command, because schema-aware /readyz is intentionally false before migration.

Use the landed design sequence, verifying actual Compose behavior:

~~~text
start db + api
run explicit uptime-lab-migrate up
start/wait for full stack
~~~

Do not weaken /readyz to make old bootstrap commands pass.

## Step 3 — Prove fresh DB unready before migration

Before migration:

- API container/process is running;
- /livez returns 200;
- /readyz does not return 200;
- goose version metadata remains absent until migration command runs.

Do not wait long enough for Compose health failure to become the control mechanism; use bounded direct container-local probes.

## Step 4 — Apply migration explicitly

Run:

~~~text
/usr/local/bin/uptime-lab-migrate up
~~~

inside the running API container.

Then start/wait for the complete four-service stack.

Assert:

- API healthy;
- /readyz = ok;
- Checker healthy after API.

## Step 5 — Product POST/GET smoke

Using container-local HTTP only:

POST one monitor with application/json.

Capture the returned monitor ID.

GET the same monitor by returned ID.

Assert at minimum:

- POST succeeds;
- GET succeeds;
- same id;
- same targetUrl;
- same createdAt text/instant where shell tooling can safely compare;
- no host port required.

Do not install curl/jq solely for smoke.

Use the smallest deterministic tools already present in the API runtime or another existing container.

If parsing becomes brittle, adjust the smoke mechanism rather than adding a runtime package casually.

## Step 6 — Persistence across normal down/up

Create/retain the existing persistence probe and monitor.

After docker compose down followed by full up --wait on the existing volume:

- schema remains ready without re-running migration;
- persisted probe/monitor remains available;
- product GET remains successful.

## Step 7 — Destructive reset behavior

After down -v:

- fresh DB makes API unready before migration;
- prior product/probe state is absent;
- explicit migration command is required again;
- full stack becomes healthy after migration.

This is intentional behavior, not a failure.

## Step 8 — Local-dev topology invariants

Continue to require:

- exactly db/api/checker/web;
- no host ports;
- checker depends on healthy API;
- API depends on healthy DB;
- no migration service;
- no profile workaround;
- no auto-migration entrypoint.

## Step 9 — Task 5 verification

Run:

~~~bash
./scripts/ci/test-check-local-dev.sh
./scripts/ci/test-smoke-local-dev.sh
./scripts/ci/check-local-dev.sh .
./scripts/ci/smoke-local-dev.sh
docker compose config --quiet
~~~

Also run full Go tests.

## Step 10 — Task 5 self-review

Verify:

- four services remain;
- no host port;
- migrations explicit;
- readiness not weakened;
- fresh bootstrap deterministic;
- persisted restart deterministic;
- product smoke uses real runtime;
- no Web/Rust implementation.

## Step 11 — Commit + exact-head CI

Suggested commit:

~~~text
test(local-dev): verify public monitoring runtime
~~~

Require local-dev, go-api, public-contract, and CI / gate GREEN on exact head.

---

# Task 6 — Canonical documentation + documentation fitness

**Goal:** Make repository documentation truthfully describe the now-live container-internal Go public transport and schema-aware readiness without implying public internet deployment.

**Primary files:** canonical docs only.

## Step 1 — Search stale current-state statements

Find claims such as:

- API exposes only /livez and /readyz;
- no /monitors endpoint exists;
- Monitoring is not wired into production runtime;
- readiness checks connectivity only;
- full-stack startup is one docker compose up --wait;
- public contract has no transport implementation.

Historical specs/plans stay historical.

## Step 2 — Add canonical implementation guide

Create:

~~~text
docs/testing/go-public-transport-adapter.md
~~~

Document:

- exact runtime operations;
- handler/error verification;
- schema-readiness model;
- migration immutability policy;
- explicit local bootstrap;
- production composition boundary;
- Docker smoke evidence;
- what remains deferred.

## Step 3 — Update backend/runtime docs

Update canonical docs so they state:

- POST /monitors + GET /monitors/{monitorId} are served by Go;
- no host public port is implied;
- internal Checker contract remains absent;
- readiness includes schema compatibility;
- startup does not migrate;
- target execution still does not occur.

## Step 4 — Update local-development docs

Document exact verified bootstrap commands from Task 5.

Document normal restart vs down -v behavior.

Do not document an unverified command sequence.

## Step 5 — Update documentation fitness

Require:

- new testing guide exists;
- docs index links it;
- canonical docs distinguish live public Go transport from public network exposure;
- readiness is described as schema-aware;
- internal contract remains deferred.

Add negative fixture cases for these distinctions where practical.

## Step 6 — Task 6 self-review

Verify:

- docs do not claim auth/security/public deployment;
- docs do not imply Rust/Web exists;
- docs do not say startup auto-migrates;
- docs distinguish product transport from internal Checker;
- historical plans/specs are not rewritten.

## Step 7 — Commit + exact-head CI

Suggested commit:

~~~text
docs(api): document public monitoring transport
~~~

Require exact-head CI GREEN.

---

# Task 7 — Whole-branch verification + review-ready PR

**Goal:** Verify the complete implementation branch as one product increment and prepare a high-signal external review candidate.

No new product implementation should be introduced here unless verification finds a defect.

## Step 1 — Static scope review

Compare main...HEAD.

Expected capability-bearing paths are limited to:

- migrations compatibility helper/tests;
- Monitoring HTTP adapter/tests;
- application generator safety refactor;
- platform HTTP generic seam;
- cmd/api composition;
- CI/history checks;
- local-dev smoke;
- canonical docs.

Reject unexpected changes to:

~~~text
contracts/openapi/public.yaml
apps/api/migrations/*.sql
contracts/openapi/internal.yaml
apps/web/
apps/checker/
root package manifests
~~~

## Step 2 — Capability search

Search added code/docs for collection/lifecycle/security drift.

The collection-route search must distinguish the forbidden exact collection operation from the approved item route:

~~~text
forbidden collection operation: exact GET /monitors
approved item operation:        GET /monitors/{monitorId}
~~~

Also search for:

~~~text
PUT
PATCH
DELETE
enabled
updatedAt
securitySchemes
Authorization
CORS
scheduler
due work
results
history
internal.yaml
ports:
~~~

A hit is acceptable only when it is:

- the approved GET /monitors/{monitorId} route declaration;
- a negative test;
- an explicit deferred statement;
- an existing operational context.

An exact collection-route GET /monitors declaration outside negative/deferred evidence is scope drift.

Otherwise review the hit as scope drift.

## Step 3 — Full repository verification

Run all canonical test/check surfaces available locally.

At minimum:

~~~text
go static checks
go architecture fitness
all Go unit tests
race tests
migration integration
postgres repository integration
schema compatibility integration
migration-history fitness tests
local-dev topology tests
local-dev smoke harness
real Docker local-dev smoke
public-contract semantic tests
architecture documentation fitness
repository/governance checks
~~~

No ad-hoc substitute counts as canonical evidence when remote CI exists.

## Step 4 — Exact contract protection

Verify:

~~~text
contracts/openapi/public.yaml
~~~

is byte-identical to main unless this phase was explicitly returned to contract reassessment.

Verify all existing SQL migration blobs are byte-identical to main.

## Step 5 — Review final architecture

Confirm mechanically and manually:

- platform imports no Monitoring;
- HTTP adapter imports no postgres/platform/pgx/goose;
- domain/application remain transport-free;
- cmd/api owns composition;
- one PostgreSQL pool;
- readiness path cannot mutate DB;
- startup does not query required schema;
- explicit migration bootstrap remains the only schema-apply path.

## Step 6 — Review final HTTP semantics

Confirm:

~~~text
POST /monitors  -> 201/400/415/422/500
GET /monitors/{id} -> 200/400/404/500
~~~

Confirm HEAD cannot execute GET.

Confirm 405/404 behavior does not create extra OpenAPI operations.

Confirm response fields/media types exactly match contract.

## Step 7 — Review security boundary

Confirm:

- no host application port;
- no auth/authz;
- no CORS;
- no rate limiting;
- no execution/probing;
- no SSRF-safety claim;
- sanitized Problem details;
- no DB errors returned.

## Step 8 — Mark PR review-ready

Update PR body with:

- exact head SHA;
- architecture impact;
- runtime impact;
- DB impact;
- contract impact;
- security impact;
- migration-readiness evidence;
- Docker evidence;
- test counts;
- explicit deferred boundaries.

Mark non-draft only after whole-branch self-review is GREEN.

## Step 9 — Fresh review-ready CI

Require a fresh exact-head review-ready run.

Because workflow and runtime paths changed, require:

~~~text
policy           SUCCESS
repository       SUCCESS
changes          SUCCESS
public-contract  SUCCESS
go-api           SUCCESS
local-dev        SUCCESS
CI / gate        SUCCESS
~~~

Do not begin external landing gate until all are GREEN.

---

# Task 8 — External review + exact-head landing gate

**Goal:** Close independent review and land the runtime without beginning the next product milestone.

## Step 1 — Request external review

Request review on the exact review-ready implementation head.

Every finding is untrusted input until independently verified.

### Valid finding

~~~text
verify
-> minimal fix
-> self-review
-> affected tests
-> coherent review-fix commit
-> fresh exact-head CI
~~~

### Invalid/stale finding

Reply with specific current code/design evidence.

No unresolved actionable thread may remain.

## Step 2 — Re-run scope review after every valid fix

Review fixes must not smuggle in:

- extra routes;
- contract changes;
- migration SQL changes;
- auth/CORS;
- host ports;
- internal Checker contract;
- new state;
- framework/generator.

## Step 3 — Landing conditions

Squash merge only when all are true:

1. exact-head CI GREEN;
2. public-contract job explicitly GREEN;
3. go-api GREEN;
4. local-dev GREEN;
5. migration-history guard GREEN;
6. whole-branch self-review GREEN;
7. external review closed;
8. unresolved actionable thread count = 0;
9. PR mergeable;
10. branch is 0 behind current main.

Squash title:

~~~text
feat(api): expose public monitoring transport
~~~

Use expected-head SHA protection.

## Step 4 — Fresh main push CI

After merge require current main to equal the squash commit.

Require fresh push CI:

~~~text
policy           SUCCESS
repository       SUCCESS
changes          SUCCESS
public-contract  SUCCESS
go-api           SUCCESS
local-dev        SUCCESS
CI / gate        SUCCESS
~~~

Inspect at minimum:

- migration-history checker evidence;
- schema compatibility integration evidence;
- Go architecture fitness;
- handler tests/race suite;
- PostgreSQL integration;
- real Docker local-dev smoke;
- public-contract semantic check.

## Step 5 — Phase closure

After post-merge GREEN:

- Go Public Transport Adapter implementation phase is CLOSED.
- Do not add public host exposure automatically.
- Do not add auth/CORS automatically.
- Do not create internal.yaml.
- Do not start Rust/Web automatically.

The next product/scope reassessment gate must be chosen from actual product value after the runtime is landed.

STOP.

---

## 7. Task Commit Model

Expected normal task commits:

~~~text
Task 1  feat(api): add schema compatibility readiness foundation
Task 2  feat(api): add public monitoring HTTP adapter
Task 3  refactor(api): add generic product HTTP composition seam
Task 4  feat(api): compose public monitoring runtime
Task 5  test(local-dev): verify public monitoring runtime
Task 6  docs(api): document public monitoring transport
~~~

Tasks 7 and 8 should normally add no implementation commit unless verification/review finds a real defect.

The final PR may therefore contain approximately six planned implementation commits plus minimal verified review fixes.

Do not optimize for a predetermined commit count at the expense of coherent changes.

---

## 8. Final Exit Criteria

The implementation phase is complete only when all are true:

### Contract

- contracts/openapi/public.yaml remains authoritative and semantically GREEN;
- exactly two public operations are live;
- no contract drift occurred.

### Runtime

- POST registration is live;
- GET by ID is live;
- exact status/media/error mappings are implemented;
- HEAD does not execute GET;
- UUID/time are server-owned;
- createdAt is microsecond-canonical and persistence-stable.

### Readiness

- /livez remains DB-independent;
- /readyz is schema-compatible;
- compatibility check is read-only;
- goose Provider mutation-prone status APIs are not used;
- migration metadata integrity is enforced;
- expected/applied positive migration sets match exactly.

### Migration governance

- landed SQL migrations are mechanically immutable;
- no SQL migration was modified in this phase;
- no new schema state was introduced.

### Architecture

- platform is Monitoring-independent;
- HTTP adapter is independent of postgres/platform/pgx/goose;
- application/domain remain transport-free;
- cmd/api owns production composition;
- one shared PostgreSQL pool is used.

### Local runtime

- fresh DB starts live-but-unready;
- explicit migration makes API ready;
- full four-service stack becomes healthy;
- product POST/GET works container-locally;
- normal down/up preserves readiness/data;
- down -v requires explicit migration bootstrap again;
- no host application port exists.

### Security/scope

- no auth/CORS/rate-limit/public ingress;
- no internal Checker contract;
- no Rust/Web implementation;
- no new product lifecycle/state.

### Evidence

- whole-branch self-review GREEN;
- external review closed;
- unresolved actionable threads = 0;
- expected-head squash merge;
- fresh main CI fully GREEN.

---

## 9. Plan Self-Review

### Scope alignment

PASS.

Every implementation task maps directly to a landed GPTA decision. No new product capability is scheduled.

### Sequencing

PASS.

Schema readiness lands before product runtime composition. Handler behavior is isolated before production wiring. Local bootstrap changes only after live readiness semantics exist.

### Dependency direction

PASS.

The plan prevents platform -> Monitoring coupling and preserves cmd/api as composition root.

### Contract safety

PASS.

OpenAPI is expected unchanged. Any required public-contract edit forces reassessment rather than silent implementation drift.

### Persistence safety

PASS.

No SQL migration change is planned. Migration source immutability becomes an always-running repository invariant before schema readiness is relied upon.

### Readiness safety

PASS.

Compatibility logic is read-only, real-PostgreSQL tested, and explicitly avoids mutation-prone goose Provider status APIs.

### Timestamp safety

PASS.

Production time is canonicalized before domain construction, so POST/GET persistence round-trip cannot create sub-microsecond public drift.

### HTTP safety

PASS.

Syntax/shape/domain errors are separated, HEAD is explicitly guarded, unsupported methods do not create product operations, and error payloads remain sanitized.

### Local-development safety

PASS.

The plan changes bootstrap order rather than weakening readiness or adding auto-migration.

### Security

PASS.

Executable handlers remain container-local in canonical Compose and do not imply public deployment approval.

### CI / evidence

PASS.

Every pushed task head is GREEN, final review-ready evidence is exact-head, and post-merge main CI is mandatory.

### Execution safety

PASS.

This plan branch contains documentation only. No implementation may start until this plan itself is reviewed and landed.
