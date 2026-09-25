# Go Control Plane

## Status

The Go Control Plane foundation is implemented.

The current runtime is intentionally narrow: it owns Monitoring domain/application/persistence, operational health, schema-aware readiness, and the public Monitoring HTTP transport. The internal Checker transport remains absent, and the canonical Compose topology does not expose an application host port.

Reviewed Go toolchain: Go 1.27.1.

## Scope

The implemented Go module is rooted at:

~~~text
apps/api
~~~

The repository-level public Monitoring contract is authoritative at `contracts/openapi/public.yaml`. `apps/api` serves exactly `POST /monitors` and `GET /monitors/{monitorId}` through the Monitoring HTTP adapter.

The runtime currently owns:

- immutable Monitoring domain values;
- RegisterMonitor and GetMonitor application use cases;
- a create/read Monitoring repository port;
- a pgx PostgreSQL adapter;
- versioned SQL migrations through goose;
- typed application configuration;
- structured JSON logging through log/slog;
- PostgreSQL pool construction;
- operational HTTP lifecycle;
- `GET /livez` and schema-aware `GET /readyz`;
- the public Monitoring HTTP adapter for `POST /monitors` and `GET /monitors/{monitorId}`;
- read-only repository-migration compatibility checks;
- explicit migration CLI;
- architecture/race/PostgreSQL/Docker/vulnerability CI evidence.

It does not currently own an internal Checker transport, mutable monitor lifecycle, scheduler, due-work query, result ingestion, history, incident handling, probe execution, authentication/CORS/rate limiting, or public ingress.

## Module Layout

~~~text
apps/api/
├── cmd/
│   ├── api/
│   └── migrate/
├── internal/
│   ├── modules/
│   │   └── monitoring/
│   │       ├── domain/
│   │       ├── application/
│   │       ├── ports/
│   │       ├── adapters/
│   │       │   ├── http/
│   │       │   └── postgres/
│   │       └── module.go
│   └── platform/
│       ├── config/
│       ├── database/
│       ├── httpserver/
│       └── observability/
└── migrations/
~~~

Domain/application/ports dependency direction is mechanically enforced by scripts/ci/check-go-architecture.sh.

## Monitoring Domain

### MonitorID

MonitorID wraps the Go standard-library uuid.UUID type and rejects the nil UUID.

Domain tests explicitly construct UUID v7 values with uuid.NewV7() and prove round-trip/equality behavior.

RegisterMonitor receives identity through an injected, error-returning application IDGenerator. Production composition supplies `uuid.NewV7()` and wraps the result with `domain.NewMonitorID`. Generator failure is returned before clock or persistence work; identity generation never moves into persistence or the transport DTO.

### TargetURL

TargetURL accepts syntactically valid absolute HTTP/HTTPS URLs with:

- a hostname;
- no embedded userinfo;
- no fragment.

Query strings and explicit ports are allowed.

The exact accepted input string is preserved.

This validation is **not** SSRF/execution safety.

It performs no:

- DNS resolution;
- private/reserved-network blocking;
- metadata-endpoint blocking;
- redirect-target validation;
- DNS-rebinding protection;
- execution timeout enforcement;
- response-size enforcement.

Those controls belong at the future Rust execution boundary and must be evaluated at execution time.

### Monitor

Monitor is immutable in this foundation and contains only:

~~~text
ID
TargetURL
CreatedAt
~~~

CreatedAt is normalized to UTC by the domain constructor.

There is no Enabled, UpdatedAt, version/concurrency field, lifecycle state, or domain event.

## Application Use Cases

### RegisterMonitor

The current orchestration is:

~~~text
raw target
  -> TargetURL validation
  -> injected MonitorID
  -> injected Clock
  -> immutable Monitor
  -> MonitorRepository.Create
  -> return Monitor
~~~

Invalid input fails before generated identity, clock, or persistence work. ID-generation failure also stops before clock/persistence.

Repository failures map to stable application persistence errors rather than exposing PostgreSQL/pgx details.

Duplicate target text is intentionally allowed.

### GetMonitor

The current orchestration is:

~~~text
MonitorID
  -> MonitorRepository.ByID
  -> stable not-found/persistence error mapping
  -> return Monitor
~~~

No list/update/delete operation exists.

## Persistence

The module-owned port is:

~~~text
MonitorRepository
├── Create(ctx, monitor)
└── ByID(ctx, id)
~~~

The concrete adapter uses pgxpool.Pool directly with hand-written SQL.

It:

- writes the application-assigned UUID;
- reads UUID as text and reconstructs MonitorID;
- maps pgx.ErrNoRows to the module-owned not-found signal;
- preserves TargetURL and CreatedAt semantics;
- respects contexts.

There is no ORM, sqlc layer, generic repository, Unit of Work, transaction manager, Save abstraction, update/delete/list query, or speculative concurrency layer.

## Database Schema

Go exclusively owns durable product state in PostgreSQL.

The current Monitoring schema is exactly:

~~~sql
CREATE SCHEMA monitoring;

CREATE TABLE monitoring.monitors (
    id uuid PRIMARY KEY,
    target_url text NOT NULL,
    created_at timestamptz NOT NULL
);
~~~

target_url is deliberately not unique.

## Migrations

Migrations are versioned SQL under apps/api/migrations.

The separate binary supports:

~~~text
uptime-lab-migrate up
uptime-lab-migrate down
uptime-lab-migrate status
~~~

The API process does not auto-migrate at startup.

The migration command is packaged into the same Docker image as the API binary and must be run explicitly.

Landed SQL migrations are immutable repository history: CI rejects modification, rename, or deletion of a migration present in the base revision while allowing schema evolution through new migration versions.

Runtime schema compatibility is checked read-only from Goose migration metadata against the repository-owned embedded migration version set. The check does not run `Up`/`Down`, create the Goose table, or otherwise mutate schema.

## Platform Runtime

### Configuration

Application settings:

~~~text
UPTIME_LAB_HTTP_ADDR
UPTIME_LAB_LOG_LEVEL
~~~

Defaults:

~~~text
:8080
info
~~~

PostgreSQL configuration is parsed by pgx from standard PG environment variables rather than manually concatenating a connection URL.

### Logging

The API uses log/slog with a JSON handler.

Stable baseline fields include:

~~~text
service=api
component=<component>
event=<event>
~~~

Runtime logs do not intentionally serialize database secrets.

### Database Pool

pgxpool.ParseConfig and pgxpool.NewWithConfig build the pool.

Construction validates configuration but does not require an immediately reachable database. Live DB availability belongs to readiness.

### Operational and Product HTTP

The runtime serves exactly:

~~~text
GET  /livez
GET  /readyz
POST /monitors
GET  /monitors/{monitorId}
~~~

`/livez`:

- returns 200;
- does not query PostgreSQL.

`/readyz`:

- uses the existing bounded readiness timeout;
- requires PostgreSQL access plus an exact read-only match between applied Goose migration metadata and repository-owned embedded positive migration versions;
- treats missing/invalid version-zero metadata, missing migrations, duplicate/unapplied rows, and unknown/ahead versions as unready;
- returns 503 with sanitized output on connectivity or compatibility failure;
- never applies migrations or creates migration metadata.

The Monitoring adapter serves the two operations defined in `contracts/openapi/public.yaml`. It explicitly rejects undocumented methods, including implicit HEAD behavior, maps request/application failures to the contracted Problem Details statuses, and never exposes raw PostgreSQL/pgx errors.

### Shutdown

SIGINT/SIGTERM cancellation drives bounded graceful http.Server shutdown. The PostgreSQL pool is closed during process shutdown.

## Production Composition Boundary

cmd/api is the composition root:

~~~text
typed config
  -> structured logger
  -> one pgx pool
       |-> Monitoring PostgreSQL repository -> monitoring.Module -> Monitoring HTTP adapter
       |-> pgx stdlib wrapper -> read-only migration compatibility checker -> /readyz
  -> generic platform HTTP server
~~~

The stdlib wrapper reuses the same pgx pool; production does not open a second PostgreSQL pool for readiness. Construction validates local configuration/migration metadata but does not query live schema state. API startup never runs migrations.

## Docker

The API image is:

- multi-stage;
- pinned to reviewed Go/Alpine images;
- CGO-disabled;
- built with -trimpath and -buildvcs=false;
- non-root USER 10001:10001;
- read-only-compatible;
- packaged with both API and migration binaries.

Compose publishes no application host ports.

## CI

The path-aware go-api job verifies:

~~~text
fmt
tidy cleanliness
mod verify
vet
architecture fitness (19 canonical cases)
unit/application/HTTP/runtime tests
fresh race
real PostgreSQL migration + compatibility integration
real PostgreSQL adapter integration
real cmd/api composition integration
govulncheck v1.8.0
~~~

The stable aggregate required check is CI / gate.

See [../testing/go-monitoring-foundation.md](../testing/go-monitoring-foundation.md) for the Go evidence ownership map and [../testing/public-monitoring-contract.md](../testing/public-monitoring-contract.md) for public-contract verification.

## Deferred

The following remain explicitly deferred:

- internal Go/Rust Checker contract;
- mutable public Monitoring operations beyond create/read;
- mutable lifecycle and concurrency semantics;
- scheduling/due work;
- result ingestion/history;
- Rust execution runtime;
- React frontend runtime;
- production deployment topology.
