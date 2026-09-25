# Go Public Monitoring Transport Testing

## Purpose

This document is the canonical implementation and verification guide for the live Go public Monitoring transport. It records the runtime behavior that now exists without implying that the service is publicly reachable from the host or internet.

The authoritative product contract remains `contracts/openapi/public.yaml`.

## Live Runtime Surface

The Go API serves exactly these product operations:

~~~text
POST /monitors
GET  /monitors/{monitorId}
~~~

Operational health remains separate:

~~~text
GET /livez
GET /readyz
~~~

No application host ports are published by the canonical Compose topology. A live Go product handler is therefore not a public-deployment decision. Authentication, authorization, CORS, rate limiting, ingress/TLS, and internet exposure remain deferred.

The internal Checker contract remains deferred. Web and Checker remain placeholders, and no target probe execution occurs in this milestone.

## Transport Boundary

The Monitoring HTTP adapter is an outward adapter under:

~~~text
apps/api/internal/modules/monitoring/adapters/http
~~~

It depends on narrow application behavior, not on the concrete PostgreSQL adapter or platform package.

The generic platform HTTP server owns operational routes and delegates product traffic to the composed product handler. Platform code remains Monitoring-independent; `cmd/api` is the composition root that may depend on both.

Architecture fitness mechanically protects the relevant directions, including:

- platform -> Monitoring module/adapters is forbidden;
- Monitoring HTTP adapter -> PostgreSQL adapter is forbidden;
- Monitoring HTTP adapter -> pgx/goose is forbidden;
- Monitoring HTTP adapter -> platform is forbidden;
- domain/application -> net/http is forbidden.

The canonical architecture harness reports:

~~~text
Go architecture tests: 19 passed, 0 failed
~~~

## Request and Error Verification

Adapter tests protect exact routing and classification rather than relying on a framework default.

### POST /monitors

Success proves:

- 201 Created;
- `Content-Type: application/json`;
- `Location: /monitors/{id}`;
- response fields are exactly `id`, `targetUrl`, and `createdAt`;
- accepted target text is preserved;
- creation time is the application-supplied canonical UTC value.

Request classification proves:

| Condition | Result |
|---|---:|
| missing, malformed, or unsupported Content-Type | 415 |
| empty body, malformed JSON, or multiple JSON documents | 400 |
| valid JSON with wrong root/member/type shape | 422 |
| domain-invalid targetUrl | 422 |
| stable persistence failure | 500 |
| unexpected internal failure | 500 |

Problem responses use `application/problem+json` and do not expose raw internal/database error text.

### GET /monitors/{monitorId}

Tests prove:

- valid existing id -> 200 with the exact Monitor response shape;
- invalid textual UUID -> 400 before the use case executes;
- missing Monitor -> 404;
- persistence failure -> sanitized 500.

Known resource paths reject unsupported methods with 405 and the exact `Allow` header. HEAD does not implicitly execute GET. Unknown/trailing/nested paths remain 404.

## Production Identity and Time

Production composition owns identity and creation time:

- UUID v7 is generated with `uuid.NewV7()` and wrapped as a domain `MonitorID`;
- ID generation is error-returning and failure stops before clock/persistence work;
- creation time is normalized to UTC microsecond precision before Monitor construction.

This keeps POST and a later PostgreSQL-backed GET on the same externally observed `createdAt` instant.

## Schema-Aware Readiness

`/livez` remains database-independent.

`/readyz` is now a bounded **database + schema compatibility** signal. Compatibility is derived from repository-owned embedded migration sources and checked against existing Goose migration metadata.

The readiness path is read-only. It does not call migration `Up`/`Down`, create the Goose metadata table, or otherwise mutate schema.

Readiness requires:

- exactly one valid applied version-zero bootstrap row;
- every positive retained migration row is applied and unique;
- the applied positive DB version set exactly equals the embedded positive migration version set.

Readiness fails for:

- unreachable PostgreSQL;
- absent Goose metadata on a fresh database;
- invalid/missing/duplicate version-zero metadata;
- missing or rolled-back required migrations;
- duplicate or unapplied positive rows;
- unknown/ahead applied versions;
- read failures.

This is repository migration-state compatibility, not arbitrary manual-DDL drift detection.

## Migration Immutability

The version-based readiness model depends on landed SQL versions remaining stable.

The always-running repository gate therefore compares base -> head and rejects modification, rename, or deletion of any SQL migration already present in the base revision under:

~~~text
apps/api/migrations/*.sql
~~~

Adding a new migration version remains mechanically possible for a future schema-evolution phase. This milestone does not add one.

## Production Composition

`cmd/api` composes:

~~~text
PG environment
   |
   v
one pgxpool.Pool
   |-------------------------------|
   |                               |
   v                               v
Monitoring PostgreSQL repo     pgx stdlib wrapper
   |                               |
   v                               v
Monitoring Module             read-only migration compatibility
   |                               |
   v                               v
Monitoring HTTP adapter          /readyz
   |                               |
   +--------------+----------------+
                  v
        generic platform HTTP server
~~~

The database/sql wrapper reuses the shared pgx pool. Closing the wrapper does not close the underlying pool.

API startup does not apply migrations and does not perform a live schema query. The process can therefore be live while readiness is false.

## Explicit Local Bootstrap

A fresh database intentionally requires an explicit migration step. The real Docker smoke verifies this sequence:

~~~bash
docker compose build api
docker compose build web checker
docker compose up -d db api

docker compose exec -T api /usr/local/bin/uptime-lab-migrate up

docker compose up -d --wait --wait-timeout 60
docker compose ps
~~~

Before the migration command:

- API `/livez` becomes available;
- `/readyz` remains unavailable;
- Goose migration metadata is absent.

After the explicit migration, the full stack can become healthy. See [../devops/local-development.md](../devops/local-development.md) for operational usage.

## Real Docker Product Evidence

The canonical real Docker smoke proves:

1. fresh DB -> API live but unready;
2. migration metadata absent before migration;
3. explicit `uptime-lab-migrate up`;
4. full stack healthy;
5. real container-local POST creates a Monitor;
6. real GET returns the same id, targetUrl, and exact createdAt text;
7. normal `down` -> `up` preserves schema and Monitor state without rerunning migration;
8. `down -v` removes schema and product state;
9. fresh runtime returns to live-but-unready;
10. explicit migration restores readiness;
11. the Monitor created before destructive reset is absent afterward.

No host application port is required for this evidence.

## Verification Commands

Repository/documentation and architecture fitness:

~~~bash
./scripts/ci/test-architecture-docs.sh
./scripts/ci/check-architecture-docs.sh .
./scripts/ci/test-check-go-architecture.sh
./scripts/ci/check-go-architecture.sh .
./scripts/ci/test-check-migration-history.sh
~~~

Go verification:

~~~bash
cd apps/api
go test ./...
go test -count=1 -race ./...
go test -count=1 -tags=integration ./migrations
go test -count=1 -tags=integration ./internal/modules/monitoring/adapters/postgres
go test -count=1 -tags=integration ./cmd/api
~~~

Docker verification:

~~~bash
./scripts/ci/test-smoke-local-dev.sh
./scripts/ci/smoke-local-dev.sh
~~~

The stable repository aggregate remains `CI / gate`.

## Deferred

This milestone does not add:

- list/search/update/delete/enable/disable Monitoring operations;
- mutable monitor lifecycle or scheduler/result history;
- internal Checker HTTP/API contract;
- Rust probe execution;
- React Web runtime;
- authentication/authorization;
- CORS or rate limiting;
- public host ports, ingress, or TLS;
- execution-time SSRF protections.

Those require separate gates rather than being inferred from the existence of a live public-product transport adapter.
