# Go Monitoring Foundation Design

**Status:** Review candidate
**Date:** 2026-09-20
**Repository:** `kefyusuf/uptime-lab`
**Scope:** Go Control Plane runtime foundation, initial Monitoring module, PostgreSQL persistence ownership, operational health, Docker integration, and Go-specific verification
**Base:** `main@7729da2decba01c1c95a8d8dcde7a72dc17ce762`

---

## 1. Purpose

This milestone establishes the first real product runtime in `uptime-lab`: the Go Control Plane.

The goal is not to expose a public product API yet. The goal is to make the previously committed Go architecture executable and mechanically verifiable by implementing:

- one Go module under `apps/api`;
- one real business module: `monitoring`;
- a minimal but real monitor domain model;
- application use cases with explicit inward dependencies;
- PostgreSQL persistence owned only by Go;
- reviewed SQL migrations;
- operational liveness/readiness endpoints;
- structured logging and graceful lifecycle behavior;
- Docker replacement of the current API placeholder;
- Go-specific architecture and CI fitness functions.

The milestone MUST NOT invent browser-facing or Rust-facing product contracts before the separate OpenAPI/contract foundation defines them.

---

## 2. Existing Architectural Authority

This design is subordinate to the existing committed architecture:

- Go is the Control Plane.
- Go exclusively owns durable product state in PostgreSQL.
- Monitoring is the first Go business capability.
- Rust never accesses PostgreSQL.
- The browser never accesses PostgreSQL.
- Cross-runtime communication is contract-driven.
- Go business logic must not live in HTTP handlers or persistence adapters.
- Generic repositories, global shared business models, service locators, and premature distributed infrastructure are forbidden.
- Docker Compose remains the canonical local runtime topology.

This milestone refines those decisions; it does not replace them.

---

## 3. Milestone Boundary

### In scope

- Go 1.27 language/toolchain baseline.
- `apps/api` as one Go module.
- Monitoring domain/application/ports/PostgreSQL adapter.
- Initial `monitoring.monitors` persistence schema.
- SQL migration tooling and a separate migration command.
- Operational HTTP server only.
- `/livez` and `/readyz`.
- PostgreSQL connectivity through `pgx/v5`.
- Structured logging with `log/slog`.
- Graceful shutdown.
- Real Go Docker image for the `api` Compose service.
- Local-development fitness-check updates.
- Go-specific CI and architecture tests.
- Documentation required to make the new runtime boundary explicit.

### Out of scope

- Public monitor CRUD HTTP endpoints.
- Internal Go-to-Rust work/result endpoints.
- OpenAPI files.
- Rust checker implementation.
- React implementation.
- Due-work scheduling.
- Probe execution.
- Check-result persistence.
- Monitor-state derivation from probe outcomes.
- Incidents or notifications.
- Domain/integration event dispatch.
- Outbox/broker infrastructure.
- Authentication or authorization.
- Metrics/tracing backend integration.
- Production deployment topology.
- ORM, generic repository framework, query builder, or code-generation layer.

These exclusions are intentional. The next contract phase must define transport semantics before product HTTP handlers are implemented.

---

## 4. Locked Decisions

### GMF-001 — Go version and module boundary

The API is one Go module:

```text
apps/api/go.mod
module github.com/kefyusuf/uptime-lab/apps/api
```

The baseline language/toolchain is Go 1.27.

At design review time, Go 1.27.1 is the current supported Go 1.27 patch release. The implementation plan MUST re-check and pin the current supported Go 1.27 patch level and exact Docker/tool action references rather than treating the design-time patch as permanently fixed.

No root `go.mod` or `go.work` is created while there is only one Go module.

Rationale:

- the repository remains polyglot rather than becoming Go-rooted;
- Go 1.27 is the current supported major baseline;
- one nested module keeps ownership and CI paths explicit;
- a workspace becomes useful only if a second Go module actually appears.

### GMF-002 — Standard-library-first HTTP and runtime services

The Control Plane uses the standard library for:

- `net/http`;
- `log/slog`;
- `context`;
- signal handling;
- `time`;
- `uuid`.

No HTTP framework, DI container, config framework, or logging framework is introduced in this milestone.

Third-party dependencies are accepted only where they solve concrete infrastructure problems:

- `github.com/jackc/pgx/v5` for PostgreSQL;
- `github.com/pressly/goose/v3` for SQL migrations.

Exact versions are pinned in implementation.

### GMF-003 — Go package topology

The target source shape is:

```text
apps/api/
├── cmd/
│   ├── api/
│   │   └── main.go
│   └── migrate/
│       └── main.go
├── internal/
│   ├── modules/
│   │   └── monitoring/
│   │       ├── domain/
│   │       ├── application/
│   │       ├── ports/
│   │       ├── adapters/
│   │       │   └── postgres/
│   │       └── module.go
│   └── platform/
│       ├── config/
│       ├── database/
│       ├── httpserver/
│       └── observability/
├── migrations/
│   ├── embed.go
│   └── 00001_create_monitoring_monitors.sql
├── Dockerfile
├── go.mod
└── go.sum
```

No empty future-module directories are created.

No `internal/platform/events`, product HTTP adapter, scheduler package, or checker-client package is created until a real requirement exists.

### GMF-004 — Dependency direction

The Monitoring module keeps the committed dependency direction:

```text
postgres adapter ─┐
                  v
                ports
                  ^
                  |
application ------+
    |
    v
  domain
```

Normative constraints:

- `domain` imports only standard-library packages required by domain semantics.
- `domain` must not import application, ports, adapters, platform, pgx, goose, or HTTP packages.
- `application` may depend on domain and declared monitoring ports.
- `ports` may depend on domain types but never on concrete infrastructure.
- `adapters/postgres` implements monitoring ports.
- `platform` contains technical lifecycle/infrastructure behavior and no Monitoring product policy.
- `cmd/*` is composition only.
- product decisions do not migrate into `main.go`, HTTP health handlers, SQL mapping code, or config parsing.

### GMF-005 — Initial Monitor domain model

The first durable aggregate is deliberately small:

```text
Monitor
├── MonitorID
├── TargetURL
└── CreatedAt
```

The foundation does not add mutable lifecycle/configuration fields such as enabled/disabled state, updated-at state, interval, timeout, next-due time, check policy, last result, incident state, or checker assignment fields.

Those concepts require later product/contract decisions.

#### MonitorID

- represented by a UUID;
- generated by Go before persistence;
- UUID v7 is the default generation algorithm;
- identity is not database-generated;
- the domain must not expose database row identifiers as a separate concept.

Go 1.27's standard-library `uuid` package is used; no third-party UUID dependency is needed.

#### Lifecycle mutation is deferred

Monitor registration is the only lifecycle transition implemented in this foundation.

Enable/disable behavior is intentionally deferred because this milestone has no public transport, scheduler, or other runtime consumer that needs mutable monitor lifecycle state. The later lifecycle/contract design MUST define, before implementation:

- the desired-state command semantics for enable/disable;
- no-op behavior, including whether an unchanged request preserves timestamps and avoids persistence writes;
- concurrent-write semantics, including optimistic concurrency, row locking, or another explicitly justified model;
- conflict/error behavior exposed by the application layer;
- application and PostgreSQL integration tests for concurrent and idempotent transitions.

This avoids introducing a version column, row-locking policy, or write semantics before a real lifecycle consumer exists.

No domain event is emitted in this milestone because there is no consumer yet.

### GMF-006 — TargetURL semantics

`TargetURL` is a Monitoring domain value object.

Registration validation MUST require:

- absolute URL;
- scheme exactly `http` or `https`;
- non-empty hostname;
- no embedded userinfo/credentials;
- no fragment.

Query strings and explicit ports are syntactically allowed.

The foundation MUST NOT claim that a syntactically valid target is safe to execute.

Specifically, Go registration does not perform or cache:

- DNS resolution;
- private/reserved-address blocking;
- cloud-metadata blocking;
- redirect-target safety;
- DNS-rebinding protection;
- execution timeout enforcement;
- response-size enforcement.

Those checks belong at the Rust execution boundary and must be re-evaluated at execution time.

Go may own product-level policy later, but it must not replace execution-time network validation.

No lossy URL normalization is introduced merely to create a uniqueness key.

### GMF-007 — Duplicate targets are allowed

`target_url` is not unique.

Multiple monitors may legitimately target the same URL once different policies, ownership, or presentation metadata exist.

The foundation must not accidentally make future distinct monitors impossible through a premature unique constraint.

### GMF-008 — Initial application use cases

The Monitoring application layer initially owns exactly these product operations:

1. `RegisterMonitor`
2. `GetMonitor`

No enable/disable mutation, scheduler, due-work query, result submission, history query, or incident transition use case exists yet.

The use cases are callable from Go tests and module composition, but are not exposed over a product HTTP contract in this milestone.

### GMF-009 — Monitoring ports

The first infrastructure port is specific to Monitoring:

```text
MonitorRepository
├── Create(ctx, monitor)
└── ByID(ctx, id)
```

The exact Go signatures are finalized in the implementation plan, but the capability boundary above is locked.

There is no generic `Save` or lifecycle-update method in this milestone because no aggregate mutation use case exists. The later lifecycle design must add a persistence capability only after its concurrency and no-op semantics are explicit; a direct field-level method that bypasses domain lifecycle policy remains disallowed.

There is no generic `Repository[T]`, generic Unit of Work, global transaction manager, or shared persistence base interface.

ID/time acquisition may be injected into application constructors as narrow function dependencies for deterministic tests. They do not justify a global service container.

### GMF-010 — PostgreSQL ownership and schema

Go remains the only owner of durable product state.

The first module-owned namespace is:

```text
monitoring
```

The first table is:

```text
monitoring.monitors
```

The initial logical schema is:

| Column | Type | Rule |
|---|---|---|
| `id` | `uuid` | primary key, supplied by Go |
| `target_url` | `text` | not null |
| `created_at` | `timestamptz` | not null |

No additional index is added without a query that needs it.

No `check_runs`, `monitor_states`, `incidents`, or scheduling table is created.

The persistence adapter uses hand-written SQL through `pgx/v5` / `pgxpool`.

No ORM or `sqlc` is introduced for one small table.

### GMF-011 — Persistence/domain mapping

Database rows are adapter representations, not domain types.

The PostgreSQL adapter must:

- map rows into domain objects explicitly;
- translate no-row behavior into a Monitoring-level not-found error;
- prevent pgx-specific errors/types from becoming application contracts;
- preserve UTC timestamps;
- use context-aware queries;
- close rows/resources correctly.

The initial persistence surface is create/read only, so no transaction abstraction or concurrency token is introduced.

A transaction or optimistic-concurrency mechanism is introduced only when a concrete mutable use case requires it and its semantics have been designed explicitly.

### GMF-012 — Migration strategy

Migrations are versioned SQL source under `apps/api/migrations`.

Use `goose/v3` as a pinned library through a separate migration command:

```text
apps/api/cmd/migrate
```

Rules:

- API startup does not auto-apply migrations.
- Docker Compose does not gain a fifth permanent migration service.
- migration files are embedded or otherwise packaged reproducibly with the migration command;
- SQL migrations are preferred over Go migrations;
- migration up/down behavior is exercised against ephemeral PostgreSQL in CI;
- migration metadata is platform-owned, not Monitoring business data.

The implementation plan must define the canonical local command for applying migrations from the API image.

The future production deployment strategy may run the migration command separately before rollout; that deployment concern is not solved here.

### GMF-013 — Operational HTTP surface only

The first HTTP server exposes operational endpoints only:

```text
GET /livez
GET /readyz
```

No `/monitors`, public API, internal checker API, or temporary debug product endpoint is allowed.

Semantics:

- `/livez`: process HTTP server is alive;
- `/readyz`: the process is initialized and PostgreSQL connectivity succeeds within a bounded timeout.

Because this milestone exposes no product HTTP endpoint, readiness does not yet claim full business-schema compatibility. Before product endpoints are exposed, the contract/API foundation must decide whether startup/readiness also verifies migration compatibility.

Health responses remain operational and intentionally minimal; they are not OpenAPI product contracts.

### GMF-014 — HTTP server hardening

Even though only health endpoints exist, the server must use explicit operational limits:

- bounded read-header timeout;
- bounded read/write behavior appropriate to the health surface;
- bounded idle timeout;
- explicit maximum header size where supported;
- graceful shutdown on SIGTERM/SIGINT;
- shutdown timeout.

No host application port is published by Docker Compose in this milestone.

### GMF-015 — Configuration

Configuration is read once at startup into typed configuration.

The foundation may use namespaced application settings such as:

```text
UPTIME_LAB_HTTP_ADDR
UPTIME_LAB_LOG_LEVEL
```

PostgreSQL connection settings should use a clear, documented mechanism compatible with `pgx`; standard `PGHOST`, `PGPORT`, `PGDATABASE`, `PGUSER`, and `PGPASSWORD` are preferred over constructing an unsafe URL from separately overridden values.

Rules:

- domain/application packages do not call `os.Getenv`;
- invalid required configuration fails fast;
- local defaults are development-only;
- secrets are never logged.

No Viper or equivalent configuration framework is introduced.

### GMF-016 — Logging and observability foundation

Use `log/slog` with structured JSON output.

Baseline fields should be stable and low-cardinality, such as:

- service;
- component;
- event;
- severity where not already encoded by the logger.

Do not log target credentials or secrets.

No metrics backend, tracing exporter, collector, or OpenTelemetry dependency is introduced yet.

Operational logs must make startup, readiness failure, migration-command failure, and graceful shutdown diagnosable.

### GMF-017 — Docker transition

The existing four-service topology remains:

```text
web
db
api
checker
```

This milestone replaces only the `api` placeholder with the real Go Control Plane.

After the transition:

- `web` remains a generic placeholder;
- `db` remains PostgreSQL;
- `api` is the real Go runtime;
- `checker` remains a generic placeholder and still waits for healthy API.

The API image must be multi-stage, pinned, non-root, and compatible with the existing read-only/tmpfs/init hardening model.

The API healthcheck must verify the real `/readyz` behavior, not a synthetic marker file.

No host port is published.

The existing local-development fitness checker must be revised so it no longer requires `api` to use the generic placeholder image.

### GMF-018 — Local migration behavior

Canonical `docker compose up -d --build --wait` starts the infrastructure/runtime topology but does not silently mutate schema through API startup.

Applying Monitoring migrations is an explicit developer operation.

The canonical documentation must state:

- how to run migrations through the API image;
- that migrations are required before exercising Monitoring persistence;
- that PostgreSQL `.env` initialization semantics remain unchanged;
- that `docker compose down -v` is destructive and is not a normal migration workflow.

### GMF-019 — Testing strategy

Use the cheapest meaningful layer.

#### Domain tests

Must cover at minimum:

- valid HTTP target;
- valid HTTPS target;
- missing/unsupported scheme rejection;
- missing hostname rejection;
- userinfo rejection;
- fragment rejection;
- UUID identity behavior;
- monitor creation timestamp behavior.

#### Application tests

Use an in-memory/fake Monitoring port to verify:

- successful registration;
- repository failure propagation/mapping;
- get-by-ID;
- not-found behavior;
- deterministic ID/time dependencies.

#### PostgreSQL adapter integration tests

Run against real PostgreSQL and verify:

- migration up creates the schema/table;
- monitor insert/read round trip;
- duplicate target URLs are allowed;
- not-found mapping;
- created-at timestamp preservation;
- migration down/up behavior in an ephemeral database.

Mocks of the PostgreSQL wire protocol are not accepted as the primary adapter evidence.

#### Runtime tests

Must verify:

- `/livez` success;
- `/readyz` success with reachable DB;
- `/readyz` failure with unavailable DB;
- graceful shutdown;
- invalid configuration fails fast.

### GMF-020 — Architecture fitness functions

The Go foundation must introduce a repository-owned, deterministic architecture check using Go-native metadata such as `go list`.

At minimum it must mechanically reject:

- Monitoring domain importing pgx/goose/net/http/platform/adapters/application;
- Monitoring application importing adapters/platform;
- ports importing concrete infrastructure;
- adapters being imported by domain;
- a new root/shared/common business package that bypasses module ownership.

The check must have its own positive and representative negative fixtures/tests.

Do not introduce a large external architecture framework when Go package metadata is sufficient.

### GMF-021 — Go CI

CI remains path-aware and preserves the stable required check:

```text
CI / gate
```

The existing `changes` job should gain a Go/API change signal or an equivalent repository-owned detector.

A conditional Go job must cover, at minimum:

- formatting check;
- `go mod tidy` cleanliness;
- `go mod verify`;
- `go vet ./...`;
- architecture fitness tests;
- unit/application tests;
- race-enabled tests where supported;
- PostgreSQL migration/adapter integration tests;
- vulnerability scanning with a pinned Go vulnerability tool;
- API Docker build through the canonical local topology or equivalent reviewed Docker path.

The aggregate gate treats the Go job as required when relevant and validly skipped when irrelevant.

Third-party GitHub Actions remain SHA-pinned with `persist-credentials: false`.

### GMF-022 — Documentation changes when implementation lands

The implementation PR must update stale implementation-state text in:

- `README.md`;
- `docs/architecture/container-view.md`;
- `docs/architecture/module-boundaries.md`;
- `docs/architecture/data-ownership.md`;
- `docs/devops/local-development.md`;
- appropriate backend/testing documentation.

The documents must say that Go/Monitoring is implemented while Rust/Web product runtimes remain unimplemented.

The implementation must not rewrite committed architectural ownership merely to match incidental code structure.

### GMF-023 — Events remain deferred

The foundation does not publish `MonitorCreated` or speculative lifecycle events merely because the conceptual architecture mentions them. Enable/disable events remain deferred together with the lifecycle capability itself.

Events are introduced when a real reaction/consumer exists.

This avoids creating an event bus with no product behavior to decouple.

### GMF-024 — Contract phase remains separate

After this milestone, the next product-facing phase should define the public Monitoring API contract before implementing business HTTP handlers.

The internal Go-to-Rust contract must likewise be designed before due-work/result handlers exist.

The sequence is intentionally:

```text
Go Monitoring domain + persistence foundation
        ↓
public/internal contract design
        ↓
Go transport adapters
        ↓
Rust/Web foundations as appropriate
```

This preserves contract-driven cross-runtime development without forcing transport shapes before the owning application semantics exist.

---

## 5. Initial Schema Intent

The first migration should conceptually produce:

```sql
CREATE SCHEMA monitoring;

CREATE TABLE monitoring.monitors (
    id uuid PRIMARY KEY,
    target_url text NOT NULL,
    created_at timestamptz NOT NULL
);
```

This SQL is illustrative of the locked logical schema. The implementation plan owns exact migration syntax, migration comments, rollback safety, and migration metadata configuration.

No uniqueness constraint on `target_url` is allowed in this milestone.

---

## 6. Initial Composition Model

The API composition root is responsible only for dependencies that the runtime actually consumes in this milestone.

Conceptually:

```text
config
  ↓
structured logger
  ↓
PostgreSQL pool
  ↓
operational HTTP server
```

Monitoring domain, application, ports, migrations, and the PostgreSQL adapter are real implementation deliverables and receive executable unit/integration evidence, but the production API binary does not construct otherwise-unused Monitoring application services merely to prove wiring.

Monitoring application services are added to the runtime composition root when the separate contract/transport phase introduces a real consumer such as a public or internal HTTP adapter.

This avoids dead wiring, blank-identifier dependencies, and a composition root that pretends an unexposed product path exists.

The composition root must not become a service locator.

---

## 7. Error Boundary

The initial error vocabulary is module-owned, not transport-owned.

At minimum Monitoring needs stable distinctions for:

- invalid target URL;
- monitor not found;
- persistence/unavailable failure.

pgx errors must terminate at the persistence adapter boundary.

HTTP status mapping is deferred with the public contract.

Database error strings are never returned as product errors.

---

## 8. Security Boundary

This milestone intentionally separates two classes of validation.

### Go product-input validation

Go validates stable product syntax:

- HTTP/HTTPS scheme;
- host presence;
- no userinfo;
- no fragment.

### Rust execution safety

Future Rust execution must validate the actual network destination at execution time, including every redirect and DNS resolution boundary.

Registration success MUST NOT be interpreted as SSRF-safety approval.

The Go Control Plane must not perform the monitoring network request itself to compensate for the checker not existing yet.

---

## 9. Verification Layers

The implementation is not considered complete unless evidence exists at all applicable layers:

| Layer | Required evidence |
|---|---|
| Source shape | only approved Go/Monitoring/runtime files |
| Go dependency direction | architecture fitness tests |
| Domain | focused unit tests |
| Application | use-case tests with narrow fakes |
| Migration | real PostgreSQL up/down/up |
| Persistence adapter | real PostgreSQL integration tests |
| Operational HTTP | live/ready lifecycle tests |
| Docker | API replaces placeholder and becomes healthy |
| Compose | db -> real api -> placeholder checker ordering |
| CI | conditional Go job + local-dev job + stable aggregate gate |
| Documentation | implementation-state and local-runbook updates |

---

## 10. Explicit Non-Decisions

This design does not decide:

- public endpoint paths or DTOs;
- internal checker endpoint paths or DTOs;
- pagination;
- monitor display names;
- enable/disable lifecycle semantics and concurrency control;
- polling interval;
- user-configurable timeout;
- retry policy;
- next-due scheduling schema;
- checker leases;
- check-result/history schema;
- status/incident transition policy;
- retention;
- authentication;
- authorization;
- CORS policy;
- production ingress;
- deployment migration orchestration;
- OpenTelemetry;
- cache;
- queue/broker.

A future design must justify each before implementation.

---

## 11. Implementation Gate

This document is design-only.

No Go application source, `go.mod`, migrations, Dockerfile, Compose mutation, CI mutation, or backend runtime documentation change may be introduced on this design branch.

Implementation requires a separate approved implementation plan.

Expected stack after this design is approved:

```text
main
  └── docs/go-monitoring-foundation-design
        └── docs/go-monitoring-foundation-plan
              └── backend/go-monitoring-foundation
```

---

## 12. Design Exit Criteria

This design gate is GREEN only if review agrees that:

1. the milestone creates real Monitoring domain/application/persistence behavior rather than empty scaffolding;
2. no public/internal product transport contract is prematurely implemented;
3. the initial schema is minimal and module-owned;
4. duplicate target URLs remain possible;
5. Go does not claim SSRF/network-execution safety;
6. Rust/browser/PostgreSQL ownership rules remain intact;
7. generic repository/DI/config/framework abstractions are not introduced;
8. migrations are explicit and not auto-applied by API startup;
9. Docker keeps the four-service topology while only API becomes real;
10. verification covers architecture, domain, application, migration, persistence, runtime, Docker, and CI;
11. future lifecycle/scheduling/result/contract work remains possible without rewriting this foundation.

---

## 13. Self-Review

### Scope alignment

PASS.

The design implements only the first Go/Monitoring foundation and does not absorb Rust, Web, OpenAPI, scheduling, results, incidents, or deployment concerns.

### Existing architecture consistency

PASS.

Go remains Control Plane and sole PostgreSQL owner. Rust/browser database access remains forbidden. Cross-runtime contracts remain separate.

### Dependency direction

PASS.

Domain/application/ports/adapters/platform responsibilities preserve the committed inward dependency model.

### YAGNI / unnecessary complexity

PASS.

No framework, ORM, sqlc, broker, cache, event bus, generic repository, DI container, extra Compose service, speculative future module, or dead runtime wiring is introduced.

### Greenfield/brownfield safety

PASS.

The existing Docker topology remains four services. Only the API placeholder is replaced. Web/checker remain placeholders and existing PostgreSQL persistence rules remain valid.

### Security boundary

PASS.

Go performs only stable product-input validation. Execution-time SSRF/DNS/redirect safety remains explicitly owned by the future Rust runtime.

### Verification evidence design

PASS.

The design requires real PostgreSQL migration/adapter tests, Go architecture tests, runtime health tests, real Docker integration, and path-aware CI.

### Contract safety

PASS.

No fake or temporary product endpoint is introduced before the OpenAPI contract gate.

### Diff hygiene

PASS by design.

The design branch is expected to contain exactly this specification file.
