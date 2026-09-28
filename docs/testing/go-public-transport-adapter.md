# Go Public Monitoring Transport Testing

## Purpose

This guide owns verification of the live public Monitor create/read transport. It does not treat the public adapter as the owner of Checker execution semantics.

The authoritative public contract is `contracts/openapi/public.yaml`.

## Live Public Runtime Surface

~~~text
POST /monitors
GET  /monitors/{monitorId}
~~~

Operational health is separate:

~~~text
GET /livez
GET /readyz
~~~

No application host ports are published. A live public-contract handler inside the Compose network is not a public-deployment decision.

The internal Checker contract is implemented. It is a separate contract and adapter surface documented in [single-checker-execution-slice.md](single-checker-execution-slice.md). The Rust Checker is also implemented; neither expands the public OpenAPI surface.

## Transport Boundary

The public Monitoring adapter depends on application behavior, not PostgreSQL or generic platform implementation details.

Architecture fitness protects public HTTP adapter -> PostgreSQL/platform prohibitions and domain/application -> `net/http` prohibitions.

Current Go architecture fitness:

~~~text
Go architecture tests: 24 passed, 0 failed
~~~

## Request and Error Verification

### POST /monitors

Success protects 201, JSON response, `Location`, exact response fields, target text preservation, and canonical UTC creation time.

Malformed media/syntax/shape/domain input is classified deterministically. Persistence/internal errors are sanitized Problem Details.

### GET /monitors/{monitorId}

Tests protect existing/malformed/missing/error cases, explicit method handling, and unknown/trailing/nested paths.

## Production Identity and Time

Production Monitor IDs are UUID v7 generated above persistence.

Creation time is normalized to UTC microsecond precision before Monitor construction.

## Schema-Aware Readiness

`/livez` is database-independent.

`/readyz` is a bounded database + migration-compatibility signal.

The readiness path is read-only.

It never applies migrations or creates migration metadata. Applied migration versions must exactly match repository-owned embedded versions, including both Monitor and CheckRun migrations.

## Migration Immutability

CI rejects modification, rename, or deletion of landed SQL migration files present in the base revision. Schema evolution uses a new migration version.

## Production Composition

`cmd/api` creates one shared pgx pool and composes:

- public Monitoring adapter;
- internal Checker adapter;
- read-only migration compatibility;
- generic platform HTTP server.

The public transport remains isolated from execution-specific persistence details.

## Explicit Local Bootstrap

~~~bash
docker compose build api
docker compose build web checker
docker compose up -d db api
docker compose exec -T api /usr/local/bin/uptime-lab-migrate up
docker compose up -d --wait --wait-timeout 60
~~~

Before migration, API is live but unready and Goose metadata is absent. After migration the full stack becomes healthy.

## Real Docker Public-Transport Evidence

The canonical smoke still proves explicit migration bootstrap, real container-local Monitor POST/GET, normal restart persistence, destructive reset, and re-migration.

The same smoke continues into real Checker execution, but CheckRun assertions are owned by [single-checker-execution-slice.md](single-checker-execution-slice.md).

## Verification Commands

~~~bash
./scripts/ci/test-architecture-docs.sh
./scripts/ci/check-architecture-docs.sh .
./scripts/ci/test-check-go-architecture.sh
./scripts/ci/check-go-architecture.sh .
./scripts/ci/test-check-migration-history.sh
~~~

~~~bash
cd apps/api
go test ./...
go test -count=1 -race ./...
go test -count=1 -tags=integration ./migrations
go test -count=1 -tags=integration ./internal/modules/monitoring/adapters/postgres
go test -count=1 -tags=integration ./cmd/api
~~~

The aggregate remains `CI / gate`.

## Deferred

This public transport still does not add:

- public CheckRun/status/history;
- list/search/update/delete/enable/disable Monitor operations;
- mutable Monitor lifecycle;
- React;
- authentication/authorization;
- CORS/rate limiting;
- public host ports, ingress, or TLS.

Execution-time destination security exists in Rust and must not be mistaken for public deployment security.
