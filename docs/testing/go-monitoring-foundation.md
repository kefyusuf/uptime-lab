# Go Monitoring Foundation Testing

## Purpose

This document maps each implemented foundation responsibility to its cheapest meaningful verification layer and identifies the CI evidence that protects it.

## Evidence Ownership

| Responsibility | Primary evidence |
|---|---|
| Monitoring domain invariants | Go unit tests |
| RegisterMonitor / GetMonitor orchestration | fake-port application unit tests |
| Dependency direction | go-list architecture fitness fixtures |
| Migration schema/up-down-up | real PostgreSQL integration |
| pgx repository mapping/errors | real PostgreSQL integration |
| Runtime config/logging/pool | Go unit tests |
| /livez and schema-aware /readyz semantics | httptest + real PostgreSQL integration |
| public Monitoring HTTP request/response/error behavior | adapter unit tests |
| production Monitoring composition | real PostgreSQL cmd/api integration |
| landed migration immutability | Git-history repository fitness |
| graceful HTTP lifecycle | loopback lifecycle test |
| race safety | go test -count=1 -race ./... |
| API Docker image/topology | local-dev fitness cases |
| fresh live-before-ready + explicit migration bootstrap | canonical Docker Compose smoke |
| real container-local Monitoring POST/GET | canonical Docker Compose smoke |
| PostgreSQL/product persistence/reset | canonical Docker Compose smoke |
| known Go vulnerabilities | pinned govulncheck |
| full branch aggregation | path-aware go-api/local-dev/public-contract jobs + CI / gate |

## Domain Tests

The domain suite covers:

- UUID value wrapping and nil rejection;
- UUID v7 construction/round trip through the standard-library UUID implementation;
- syntactic HTTP/HTTPS TargetURL acceptance;
- invalid scheme/hostname/userinfo/fragment rejection;
- exact accepted URL-string preservation;
- Monitor constructor invariants;
- UTC creation-time semantics.

These tests intentionally do **not** claim SSRF safety.

No test should treat TargetURL acceptance as proof that the future execution plane may safely connect to that target.

## Application Tests

Application tests use an in-test fake MonitorRepository.

RegisterMonitor evidence includes:

- valid target -> generated ID -> clock -> Create;
- invalid target stops before dependencies/persistence;
- invalid generated ID stops before persistence;
- deterministic clock behavior;
- stable persistence error mapping;
- duplicate target text is not pre-rejected.

GetMonitor evidence includes:

- successful read;
- stable not-found mapping;
- stable persistence error mapping;
- invalid identity rejected before repository access where applicable.

No production mocking framework is needed.

## Architecture Fitness

scripts/ci/check-go-architecture.sh uses go list package metadata.

Negative fixtures prove that:

- domain cannot import application, ports, adapters, platform, net/http, pgx, or goose;
- application cannot import adapters/platform/pgx/goose;
- ports cannot import application/adapters/platform/pgx/goose;
- root internal/shared and internal/common business buckets are forbidden.

The canonical harness currently reports:

~~~text
Go architecture tests: 19 passed, 0 failed
~~~

## Migration and Schema-Compatibility Integration

Migration integration uses a real PostgreSQL service and forces fresh execution with -count=1.

It verifies:

- Up creates the Monitoring schema/table;
- monitoring.monitors has exactly id, target_url, created_at;
- id is the primary key;
- mutable lifecycle/version columns are absent;
- goose metadata is not stored in the monitoring schema;
- Down removes the application schema/table;
- Up succeeds again after Down;
- compatibility fails without creating Goose metadata on a fresh database;
- compatibility succeeds only for the exact repository-owned applied migration set;
- missing/rolled-back, duplicate, false/unapplied, invalid-zero, and unknown/ahead metadata remain unready;
- compatibility checks leave migration metadata unchanged.

This is database-side evidence and must never be accepted from Go's test cache. A separate Git-history fitness check also rejects modification, rename, or deletion of already-landed SQL migrations.

## PostgreSQL Adapter Integration

The adapter integration suite uses the same real PostgreSQL path and verifies:

1. Create -> ByID round trip.
2. Duplicate target URLs are allowed.
3. Missing ByID maps to ports.ErrMonitorNotFound.
4. Application-facing persistence failures remain stable and sanitized.
5. CreatedAt preserves the instant with UTC domain semantics.
6. UUID round trip preserves the exact identity.
7. Persistence never generates/replaces monitor identity.
8. Query context cancellation is respected.

## Runtime Tests

### Config

Tests cover:

- defaults;
- accepted log levels;
- invalid log level;
- HTTP-address validation;
- secret-safe config rendering.

### Observability

Logger tests verify JSON parseability and stable baseline fields without ambient DB-secret leakage.

### Database Pool

Tests verify invalid configuration fails, pool construction does not require live connectivity, and the pool can be closed cleanly.

### HTTP Server

httptest and loopback tests verify:

- `/livez` is DB-independent;
- `/readyz` delegates to a bounded generic readiness checker and returns sanitized 503 on failure;
- platform routing gives `/livez` and `/readyz` precedence over the generic product handler;
- a nil product handler fails safely with 404;
- explicit HTTP resource/time bounds exist;
- context cancellation drives graceful shutdown;
- repeated/cancelled shutdown paths do not panic.

### Public Monitoring HTTP Adapter

The dedicated adapter suite verifies the exact two-operation contract, explicit method/path handling including HEAD rejection, media/syntax/shape classification, sanitized RFC 9457 Problem Details, success DTO shape, target text preservation, and application-error mapping. See [go-public-transport-adapter.md](go-public-transport-adapter.md) for the complete evidence map.

## Race Verification

The canonical command is:

~~~bash
cd apps/api
go test -count=1 -race ./...
~~~

-count=1 is intentional so CI records fresh race evidence rather than accepting test-cache results.

## Docker Fitness

scripts/ci/test-check-local-dev.sh pins the canonical topology and negative cases.

The current local-dev topology contract verifies 37 cases, including:

- exactly four services;
- no host application ports;
- real API Dockerfile/build context;
- pinned builder/runtime images;
- non-root/read-only API;
- real /readyz healthcheck;
- Checker-after-API dependency;
- Web/Checker remain placeholders;
- no future Web/Checker runtime scaffold;
- no root Go manifest;
- both API and migration binaries.

## Canonical Docker Smoke

scripts/ci/smoke-local-dev.sh uses real Docker/Compose.

It explicitly verifies:

- compose config and explicit image builds;
- a fresh database starts `db` + `api` with `/livez` available and `/readyz` unavailable;
- Goose metadata is absent before explicit migration;
- `uptime-lab-migrate up` is the explicit schema transition;
- the full stack becomes healthy afterward;
- real container-local `POST /monitors` creates a Monitor;
- real `GET /monitors/{monitorId}` returns the same id, targetUrl, and exact createdAt text;
- normal down/up preserves schema, probe state, and Monitor state without rerunning migration;
- down -v removes schema/product state and returns the API to live-but-unready;
- explicit migration restores readiness after reset;
- cleanup.

The fake-Docker harness validates smoke control flow but does not replace the real Docker smoke.

## Vulnerability Scanning

CI installs:

~~~text
golang.org/x/vuln/cmd/govulncheck@v1.8.0
~~~

outside the application module dependency graph and runs:

~~~bash
govulncheck ./...
~~~

A vulnerability finding is a failing Go verification gate until independently assessed/resolved.

## Change Detection

Go changes trigger go-api verification.

Because the real Docker image consumes apps/api/**, API source additions/deletions also trigger local-dev verification.

Workflow changes conservatively trigger both relevant surfaces.

## Canonical CI

The repository-level aggregate remains `CI / gate`. `go-api`, `local-dev`, and `public-contract` are path-aware jobs; each must succeed when its detector fires and may be skipped when unrelated. Policy, repository, and change-detection jobs remain required.

For public-contract verification details, see [public-monitoring-contract.md](public-monitoring-contract.md).

## Useful Commands

Go verification:

~~~bash
cd apps/api
test -z "$(gofmt -l .)"
go mod tidy
test -z "$(git status --porcelain -- go.mod go.sum)"
go mod verify
go vet ./...
go test ./...
go test -count=1 -race ./...
~~~

Architecture:

~~~bash
./scripts/ci/test-check-go-architecture.sh
./scripts/ci/check-go-architecture.sh .
~~~

Real PostgreSQL integration:

~~~bash
./scripts/ci/run-go-postgres-tests.sh
~~~

Local Docker:

~~~bash
./scripts/ci/test-detect-local-dev-changes.sh
./scripts/ci/test-check-local-dev.sh
./scripts/ci/test-smoke-local-dev.sh
./scripts/ci/check-local-dev.sh .
./scripts/ci/smoke-local-dev.sh
docker compose config --quiet
~~~

## Deferred Verification

Public OpenAPI artifact verification is no longer deferred; it is owned by [public-monitoring-contract.md](public-monitoring-contract.md). Go transport conformance is now implemented and documented in [go-public-transport-adapter.md](go-public-transport-adapter.md).

There are intentionally no tests yet for:

- internal Checker API;
- mutable monitor lifecycle;
- scheduling/due work;
- result ingestion/history;
- Rust probe execution;
- React UI flows.

Those runtime layers are added only when their implementations exist.
