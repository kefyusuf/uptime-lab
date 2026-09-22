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
| /livez and /readyz semantics | httptest unit tests |
| graceful HTTP lifecycle | loopback lifecycle test |
| race safety | go test -count=1 -race ./... |
| API Docker image/topology | local-dev fitness cases |
| real API readiness/start ordering | canonical Docker Compose smoke |
| PostgreSQL persistence/reset | canonical Docker Compose smoke |
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
Go architecture tests: 11 passed, 0 failed
~~~

## Migration Integration

Migration integration uses a real PostgreSQL service and forces fresh execution with -count=1.

It verifies:

- Up creates the Monitoring schema/table;
- monitoring.monitors has exactly id, target_url, created_at;
- id is the primary key;
- mutable lifecycle/version columns are absent;
- goose metadata is not stored in the monitoring schema;
- Down removes the application schema/table;
- Up succeeds again after Down.

This is database-side evidence and must never be accepted from Go's test cache.

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

- /livez is DB-independent;
- /readyz pings DB with a bounded timeout;
- readiness failure returns sanitized 503;
- non-GET behavior is deterministic;
- explicit HTTP resource/time bounds exist;
- context cancellation drives graceful shutdown;
- repeated/cancelled shutdown paths do not panic.

## Race Verification

The canonical command is:

~~~bash
cd apps/api
go test -count=1 -race ./...
~~~

-count=1 is intentional so CI records fresh race evidence rather than accepting test-cache results.

## Docker Fitness

scripts/ci/test-check-local-dev.sh pins the canonical topology and negative cases.

The current Task 7+ contract verifies 37 cases, including:

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

- compose config;
- explicit API image build;
- healthy PostgreSQL;
- healthy real API;
- container-local /livez=ok;
- container-local /readyz=ok;
- Checker healthy after API;
- PostgreSQL state survives normal down/up;
- PostgreSQL state is reset by down -v;
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

Public OpenAPI artifact verification is no longer deferred; it is owned by [public-monitoring-contract.md](public-monitoring-contract.md). Go transport conformance remains deferred because no public handler is implemented.

There are intentionally no tests yet for:

- public product HTTP handler/transport conformance;
- internal checker API;
- mutable monitor lifecycle;
- scheduling/due work;
- result ingestion/history;
- Rust probe execution;
- React UI flows.

Those runtime layers are added only when their implementations exist.
