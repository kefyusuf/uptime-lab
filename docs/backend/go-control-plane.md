# Go Control Plane

## Availability policy

Go Monitoring samples its clock once after one successful, validated terminal
read. Unsupported durable shapes fail as sanitized persistence errors; an
invalid evaluation clock fails as a sanitized evaluation error.

| Fresh execution evidence | Status | Reason |
|---|---|---|
| HTTP 200–299 | available | successful_response |
| HTTP 100–199 or 300–599 | unavailable | unexpected_http_status |
| dns_error, timeout, connect_error, tls_error, protocol_error | unavailable | probe_failure |
| policy_rejected | unknown | policy_rejected |
| internal_error, worker_timeout | unknown | execution_failure |
| No terminal result | unknown | no_result |

Future completion takes precedence as `unknown/future_result`. Otherwise age
greater than 120 seconds takes precedence as `unknown/stale_result`. Exactly
120 seconds is fresh; comparison is not rounded. Every terminal assessment keeps
its evidence, even unknown. There is no fallback to an older successful check,
no body-content or complete-download claim, and no write or reconciliation.
Separate raw-result and availability requests can observe different CheckIDs;
each assessment identifies its own evidence. Public timestamps are normalized
to UTC and validated for JSON representation before the HTTP response is written.

Current availability is exposed by `GET /monitors/{monitorId}/availability` as a read-only assessment of the latest terminal CheckRun. It returns `status`, `reason`, UTC `evaluatedAt`, and terminal `evidence` (`checkId`, `completedAt`). A known Monitor without a terminal result returns `200 unknown/no_result` with evidence omitted; the raw latest-result route retains its empty `204`. Every matched availability response, including errors and `405`, uses `Cache-Control: no-store`; `HEAD` returns `405` with `Allow: GET`. No new persistence, reconciliation, history, or public deployment is introduced.

## Status

The Go Control Plane is implemented for public Monitor create/read, latest terminal execution-result read, and the Single-Checker execution loop.

It owns product semantics, scheduling truth, durable Monitor/CheckRun state, public/internal HTTP adapters, explicit migrations, and schema-aware readiness.

Reviewed Go toolchain: Go 1.27.1.

## Scope

The Go runtime is rooted at `apps/api`.

Authoritative contracts:

- public: `contracts/openapi/public.yaml`;
- internal: `contracts/openapi/internal.yaml`.

The public runtime surface is:

~~~text
POST /monitors
GET  /monitors
GET  /monitors/{monitorId}
GET  /monitors/{monitorId}/latest-result
GET  /monitors/{monitorId}/availability
~~~

The internal Checker surface is:

~~~text
POST /internal/checks/claim
PUT /internal/checks/{checkId}/result
~~~

Operational health remains:

~~~text
GET /livez
GET /readyz
~~~

The latest-result route exposes only the latest terminal CheckRun execution fact. Pending rows are invisible; a known Monitor with no terminal result maps to `204`. Full CheckRun history and materialized availability history remain deferred.

## Monitoring Domain

Go owns:

- MonitorID;
- CheckID;
- TargetURL registration syntax;
- immutable Monitor;
- normalized CheckResult vocabulary/invariants.

Registration-time TargetURL validation is not execution safety. The Rust Checker performs execution-time destination policy.

## Application Capabilities

### RegisterMonitor / GetMonitor

These preserve the existing immutable public Monitor create/read behavior.

### GetLatestCheckResult

This public read is side-effect free and uses a dedicated read-facing persistence port. It considers only terminal CheckRuns and orders them by `completed_at DESC, issued_at DESC, id DESC`.

A newer pending CheckRun does not hide the previous terminal result, and the read path never reconciles expired pending work. Public `resultKind` values are execution facts only; no up/down/healthy/degraded policy is derived. `worker_timeout` exposes neither `httpStatus` nor a fabricated `durationMs`.

### ClaimDueCheck

Go generates a UUID v7 CheckID and uses one server-time instant to derive scheduling inputs.

Current fixed policy:

~~~text
cadence        60 seconds
probe timeout  10 seconds
deadline       20 seconds
max redirects  3
~~~

The persistence boundary atomically reconciles expired pending work, chooses one due Monitor, and creates one pending CheckRun.

A Monitor with no terminal run is immediately eligible. Otherwise its latest terminal completion controls cadence eligibility.

### SubmitCheckResult

Go validates CheckID/result shape, supplies completion time, and atomically completes the pending CheckRun.

Exact duplicate canonical results are idempotent success.

Conflicting duplicates are rejected.

A result arriving at/after deadline causes Go-owned `worker_timeout` terminalization.

Rust cannot submit `worker_timeout`.

## Persistence Ports

Monitoring owns narrow Monitor, latest-result read, and execution persistence interfaces.

`LatestCheckResultRepository` is separate from `CheckExecutionRepository`. The former performs the bounded terminal read; execution persistence remains limited to claim/completion behavior. No generic Unit of Work or repository hierarchy is introduced.

## Database Schema

Go exclusively owns durable product state in PostgreSQL.

The Monitoring schema contains:

~~~text
monitoring.monitors
monitoring.check_runs
~~~

`monitoring.check_runs` contains:

~~~text
id
monitor_id
issued_at
deadline_at
completed_at
result_kind
http_status
duration_ms
~~~

Database constraints protect valid pending/terminal shapes, bounds, timestamp order, and one pending run per Monitor.

## Migrations and Readiness

Migrations are repository-owned SQL under `apps/api/migrations` and are applied only through the separate migration binary:

~~~text
uptime-lab-migrate up
uptime-lab-migrate down
uptime-lab-migrate status
~~~

API startup never auto-migrates.

The production runtime uses a read-only migration compatibility checker for `/readyz`. Compatibility requires the applied Goose migration set to exactly match repository-owned embedded migration versions.

Landed migration SQL is immutable; schema evolution requires a new migration version.

## HTTP Composition

The generic platform server owns operational routing.

`cmd/api` composes:

~~~text
one pgx pool
  |-> Monitoring PostgreSQL repository
  |     |-> public Monitoring module/adapter (create/read/latest terminal result)
  |     |-> execution application
  |           |-> internal Checker adapter
  |
  |-> stdlib wrapper
        |-> read-only migration compatibility checker

public + internal product router
  -> generic platform HTTP server
~~~

Platform-owned `/livez` and `/readyz` keep precedence.

The Go process opens no second PostgreSQL pool for execution or readiness.

## Internal Checker Adapter

The internal adapter maps the two contract operations to `ClaimDueCheck` and `SubmitCheckResult`.

It returns one exact CheckWork or 204 for no work and accepts only the normalized Rust result vocabulary.

Application/database errors are mapped to stable sanitized HTTP responses; raw persistence details do not cross the contract.

## Platform Runtime

Configuration includes the existing HTTP/logging and standard PG environment.

`/livez` remains database-independent.

`/readyz` is bounded, database/schema-aware, and read-only.

Graceful server shutdown remains driven by process cancellation.

## Docker

The API image remains non-root and read-only-compatible and contains both API and migration binaries.

Canonical Compose publishes no application host ports.

The Rust Checker reaches the internal API through `http://api:8080` inside the Compose network.

## CI

The path-aware Go job verifies formatting/tidy/mod/vet, architecture fitness, unit/race tests, real PostgreSQL migrations/persistence/composition, and `govulncheck`.

Current architecture fitness:

~~~text
Go architecture tests: 24 passed, 0 failed
~~~

Cross-runtime Docker acceptance is documented separately in [../testing/single-checker-execution-slice.md](../testing/single-checker-execution-slice.md).

## Bounded inventory read

ListMonitors uses a dedicated bounded port and an88-character raw-base64url position containing version1, a fixed microsecond UTC timestamp and a lowercase nonzero UUID. Separate parameterized first/continuation queries use `(created_at DESC,id DESC)` and the nonunique `monitoring.monitors_inventory_order_idx`, added by00003. Raw targets are projected only at most245760 bytes; larger values return explicit metadata markers. The use case validates at most51 candidates, order and strict anchor bounds. HTTP accounts for escaped JSON,128 framing bytes and all commas, returning complete-prefix continuation or the approved first-item500. A cursor requires no anchor-row lookup and provides no ownership, authentication or snapshot guarantee. CheckRun state is unchanged.

## Deferred

Still deferred:

- public Monitor operations beyond create/inventory/read/latest-result/availability;
- full CheckRun history and materialized availability history;
- mutable Monitor lifecycle;
- multi-worker coordination/leases;
- broker/outbox;
- React;
- authentication/authorization;
- CORS/rate limiting;
- public ingress/TLS/deployment topology.
