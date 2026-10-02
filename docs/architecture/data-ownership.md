# Data Ownership

**Architecture state:** Committed

**Implementation state:** Implemented for Monitor registration/read, latest terminal CheckRun read, and single-Checker execution persistence.

Current availability is exposed by `GET /monitors/{monitorId}/availability` as a read-only assessment of the latest terminal CheckRun. It returns `status`, `reason`, UTC `evaluatedAt`, and terminal `evidence` (`checkId`, `completedAt`). A known Monitor without a terminal result returns `200 unknown/no_result` with evidence omitted; the raw latest-result route retains its empty `204`. Every matched availability response, including errors and `405`, uses `Cache-Control: no-store`; `HEAD` returns `405` with `Allow: GET`. No new persistence, reconciliation, history, or public deployment is introduced.

## Primary Ownership Rule

Go exclusively owns durable product state in PostgreSQL.

PostgreSQL is not a shared integration surface between Web, Go, and Rust.

## Runtime Access

### Web Client

React is not implemented. A future browser consumes public Go APIs and never accesses PostgreSQL directly.

### Rust Checker

The Rust Checker is implemented and obtains work/submits normalized results through the internal Go contract.

Rust never accesses PostgreSQL directly.

### Go Control Plane

Go owns schema access, migrations, scheduling truth, persistence mapping, and interpretation of durable CheckRun state.

The production API uses one PostgreSQL pool for Monitoring persistence and readiness composition.

## Monitoring Schema

The module-owned namespace is:

~~~text
monitoring
~~~

Current durable tables are:

~~~text
monitoring.monitors
monitoring.check_runs
~~~

`monitoring.monitors` stores immutable Monitor registration state.

`monitoring.check_runs` stores Go-owned execution identity/timing and terminal normalized results:

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

Database constraints enforce pending/terminal shape, bounds, timestamp ordering, and at most one pending CheckRun per Monitor.

There is no enabled flag, Monitor update/version column, monitor_states table, incidents table, lease owner, worker identity, or materialized status/history projection. The public latest-result read queries existing terminal `monitoring.check_runs`; it does not add duplicated status state.

## Persistence Boundary

Monitoring persistence is intentionally narrow.

Monitor persistence supports create/read.

Execution persistence supports atomic due claim and completion. Claim creates the pending CheckRun; Rust does not create or mutate database rows directly.

A separate latest-result read port selects the latest terminal CheckRun without locks, writes, or reconciliation. Pending rows remain invisible.

Availability reuses that same read port and samples the injected Go clock after
row validation. The assessment is computed in memory under the
[Go availability policy](../backend/go-control-plane.md#availability-policy);
it introduces no status table, migration, materialized projection, or SQL policy.

Late completion is resolved by Go into durable `worker_timeout` state using the stored deadline.

## Migrations

Versioned SQL lives under `apps/api/migrations`:

~~~text
00001_create_monitoring_monitors.sql
00002_create_monitoring_check_runs.sql
~~~

API startup does not apply migrations.

The separate `uptime-lab-migrate` binary owns explicit `up`, `down`, and `status` operations.

Readiness checks migration metadata read-only against the repository-owned embedded migration set. Landed migration files are protected as immutable history.

## Go Module Ownership

A business module's tables are implementation details of that module, not a cross-module API.

Cross-module SQL reads remain forbidden unless a later accepted architecture decision changes that rule.

## Cross-Module Integration

A future module may interact through a declared application interface or an event boundary justified by a concrete requirement.

No Kafka, RabbitMQ, Redis broker, outbox, or event-sourcing infrastructure is introduced by this milestone.

## Deferred Persistence Decisions

Still deferred:

- mutable Monitor lifecycle;
- full CheckRun history and materialized availability history projection;
- retention/archival;
- multi-worker leases/identity;
- broker/outbox topology;
- production deployment migration orchestration.

## Related Decisions

- [Container View](container-view.md)
- [Module Boundaries](module-boundaries.md)
- [Dependency Rules](dependency-rules.md)
- [Go Control Plane](../backend/go-control-plane.md)
- [Rust Checker](../checker/rust-checker.md)
- [ADR-0003](../adr/0003-contract-and-data-ownership.md)
