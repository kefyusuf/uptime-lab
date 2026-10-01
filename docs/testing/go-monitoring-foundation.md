# Go Monitoring Foundation Testing

## Purpose

This document maps Go-owned Monitoring responsibilities to the cheapest meaningful verification layer. Cross-runtime execution evidence is documented separately in [single-checker-execution-slice.md](single-checker-execution-slice.md).

## Evidence Ownership

| Responsibility | Primary evidence |
|---|---|
| Monitor/CheckID/result domain invariants | Go unit tests |
| Register/Get/GetLatest/Claim/Submit orchestration | application unit tests |
| Dependency direction | Go architecture fitness |
| Monitor + CheckRun schema/migrations | real PostgreSQL integration |
| atomic claim/completion/reconciliation | real PostgreSQL integration |
| public/internal HTTP mapping | adapter tests |
| production composition | real PostgreSQL `cmd/api` integration |
| livez/schema-aware readyz | httptest + PostgreSQL integration |
| landed migration immutability | repository fitness |
| race safety | `go test -count=1 -race ./...` |
| Docker bootstrap + execution | canonical Docker smoke |
| known Go vulnerabilities | pinned `govulncheck` |
| aggregate | path-aware jobs + `CI / gate` |

## Domain and Application Tests

Go tests protect:

- MonitorID and CheckID validity;
- UUID v7 production identity behavior;
- TargetURL registration syntax;
- Monitor invariants;
- closed CheckResult vocabulary;
- duration/HTTP-status shape constraints;
- fixed scheduling inputs;
- due-before/deadline derivation from one Go server time;
- no-work/error mapping;
- idempotent duplicate and conflict semantics;
- Go-owned `worker_timeout` behavior;
- latest-terminal read shape validation and stable no-result/not-found/persistence mapping.

Registration-time TargetURL validation is not execution-time SSRF evidence.

## Architecture Fitness

`scripts/ci/check-go-architecture.sh` uses Go package metadata to enforce domain/application/ports/adapter/platform direction.

The canonical harness reports:

~~~text
Go architecture tests: 24 passed, 0 failed
~~~

It also protects the internal Checker adapter from depending directly on PostgreSQL/platform implementation details.

## Migration and PostgreSQL Integration

Real PostgreSQL evidence covers:

- `monitoring.monitors`;
- `monitoring.check_runs`;
- migration up/down/up;
- schema compatibility/readiness;
- one pending CheckRun per Monitor;
- pending/terminal row constraints;
- atomic due claim;
- deterministic due ordering;
- expired pending reconciliation;
- exact duplicate completion;
- conflicting completion;
- late result -> `worker_timeout`;
- query cancellation/error mapping;
- deterministic latest-terminal ordering, pending invisibility, and side-effect-free reads.

Migration history fitness rejects modification/rename/deletion of already-landed SQL migrations.

## HTTP Runtime Tests

Public adapter tests protect `POST /monitors`, `GET /monitors/{monitorId}`, and `GET /monitors/{monitorId}/latest-result`.

Internal adapter tests protect:

~~~text
POST /internal/checks/claim
PUT  /internal/checks/{checkId}/result
~~~

They verify exact request/response classification, normalized result shapes, method/path behavior, and sanitized errors.

`cmd/api` integration proves real production composition, terminal CheckRun persistence, `204` no-result behavior, and public reads of both `http_response` and Go-owned `worker_timeout` shapes.

## Race Verification

~~~bash
cd apps/api
go test -count=1 -race ./...
~~~

Fresh race execution is required; cached evidence is not accepted.

## Docker Fitness

Local-development fitness protects the four-service topology, real API image, real Checker image, dependency/health ordering, no host application ports, explicit migration bootstrap, and hardening constraints.

Current local-development fixture suite:

~~~text
Local development tests: 49 passed, 0 failed
~~~

## Canonical Docker Smoke

The real Docker smoke retains:

- live-before-ready;
- explicit migration;
- real Monitor POST/GET;
- real Checker health;
- normal restart persistence;
- destructive reset/re-migration.

It also proves:

~~~text
Go -> Rust -> policy_rejected -> Go -> PostgreSQL -> public latest-result read
~~~

Detailed durable/log assertions belong to [single-checker-execution-slice.md](single-checker-execution-slice.md).

The fake-Docker control-flow suite remains:

~~~text
Local-dev smoke tests: 7 passed, 0 failed
~~~

## Vulnerability Scanning

CI runs pinned `govulncheck` against Go.

Rust vulnerability evidence belongs to the execution-slice guide and Checker job.

## Change Detection and CI

Go source changes trigger Go verification. Checker/local-dev/internal-contract detectors protect their own runtime surfaces.

The aggregate remains `CI / gate`.

## Useful Commands

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

~~~bash
./scripts/ci/test-check-go-architecture.sh
./scripts/ci/check-go-architecture.sh .
./scripts/ci/run-go-postgres-tests.sh
~~~

For the complete Go/Rust/Docker evidence matrix, see [single-checker-execution-slice.md](single-checker-execution-slice.md).

## Deferred Verification

Still intentionally absent:

- full CheckRun history and derived availability/status behavior;
- mutable Monitor lifecycle;
- multi-worker coordination;
- React UI flows;
- public ingress/authentication behavior.
