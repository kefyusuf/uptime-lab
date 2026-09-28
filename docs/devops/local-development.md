# Local Development

## Status

Docker Compose is the canonical local-development substrate.

The current topology runs:

- real PostgreSQL;
- real Go Control Plane;
- real Rust Checker;
- placeholder Web.

This is a development topology, not a production deployment contract.

No application host ports are published.

## Prerequisites

- Git
- Docker Engine or Docker Desktop
- Docker Compose >= 2.22.0

Verify:

~~~bash
docker compose version
~~~

## Canonical Topology

The root `compose.yaml` defines exactly four services:

~~~text
PostgreSQL (db)
      |
      | service_healthy
      v
Go Control Plane (api)
      |
      | service_healthy
      v
Rust Checker (checker)

Web placeholder starts independently
~~~

The Checker uses the real repository image and:

~~~text
UPTIME_LAB_CONTROL_PLANE_URL=http://api:8080
~~~

Both API and Checker are non-root/read-only-compatible. Checker readiness uses `/run/uptime-lab/ready`.

## Fresh Database Bootstrap

A fresh PostgreSQL volume intentionally leaves API live but unready until migrations are explicitly applied:

~~~bash
docker compose build api
docker compose build web checker
docker compose up -d db api

docker compose exec -T api /usr/local/bin/uptime-lab-migrate up

docker compose up -d --wait --wait-timeout 60
docker compose ps
~~~

Before migration:

- `/livez` becomes available;
- `/readyz` remains unavailable;
- Goose metadata is absent.

After explicit migration, API becomes ready and the real Checker starts.

API startup never applies migrations.

## Follow Logs

~~~bash
docker compose logs -f api
docker compose logs -f checker
docker compose logs -f db
docker compose logs -f web
~~~

Checker logs expose stable execution events such as `check_claimed`, `probe_completed`, and `result_delivered` with CheckID/MonitorID where relevant.

## Runtime HTTP Surfaces

Public Monitor transport inside the container network:

~~~text
POST /monitors
GET  /monitors/{monitorId}
~~~

Internal Checker transport:

~~~text
POST /internal/checks/claim
PUT  /internal/checks/{checkId}/result
~~~

Operational health:

~~~text
GET /livez
GET /readyz
~~~

The internal source contract is `contracts/openapi/internal.yaml`.

Because no application host port is published, these live transports do not imply public network deployment.

## Explicit Database Migrations

Migrations are never applied automatically by API startup.

~~~bash
docker compose exec api /usr/local/bin/uptime-lab-migrate status
docker compose exec api /usr/local/bin/uptime-lab-migrate up
docker compose exec api /usr/local/bin/uptime-lab-migrate down
~~~

Normal schema evolution uses migrations, not volume deletion.

## PostgreSQL Access

~~~bash
docker compose exec db sh -lc 'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB"'
~~~

Development defaults:

~~~text
database: uptime_lab
user:     uptime_lab
password: uptime_lab_local
~~~

Do not reuse development credentials in production.

## Persistence and Restart

Preserve data:

~~~bash
docker compose down
docker compose up -d --wait --wait-timeout 60
~~~

The existing migrated volume is reused; no migration rerun is required when the migration set is unchanged.

The canonical smoke verifies Monitor registration/execution before normal restart and retains Monitor state after restart.

## Destructive Reset

Delete local PostgreSQL state:

~~~bash
docker compose down -v
~~~

After destructive reset, API returns to live-but-unready until the explicit migration command runs again. Previous Monitor and CheckRun state is absent.

## Optional Environment Overrides

The stack works without a copied environment file.

`.env.example` documents local PostgreSQL overrides:

~~~dotenv
POSTGRES_DB=uptime_lab
POSTGRES_USER=uptime_lab
POSTGRES_PASSWORD=uptime_lab_local
~~~

Initialization variables affect a fresh PostgreSQL data directory only.

## Worktree and Project Isolation

No fixed Compose project name, container name, globally named network, or globally named volume is used.

For parallel worktrees:

~~~bash
export COMPOSE_PROJECT_NAME=uptime-lab-my-branch
~~~

Use the same project name for build/up/exec/logs/down commands in that worktree.

## Host Verification

Go:

~~~bash
cd apps/api
go test ./...
go test -count=1 -race ./...
~~~

Rust:

~~~bash
cd apps/checker
cargo fmt --all --check
cargo check --workspace --all-targets --locked
cargo clippy --workspace --all-targets --all-features --locked -- -D warnings
cargo test --workspace --all-targets --locked
~~~

Real PostgreSQL and real Docker remain required CI evidence.

## Repository Verification

~~~bash
./scripts/ci/test-detect-local-dev-changes.sh
./scripts/ci/test-check-local-dev.sh
./scripts/ci/test-smoke-local-dev.sh
./scripts/ci/check-local-dev.sh .
./scripts/ci/smoke-local-dev.sh
docker compose config --quiet
~~~

The canonical Docker smoke proves the real execution path using:

~~~text
targetUrl = http://web/

Go claim
-> Rust Checker
-> production private-address policy
-> policy_rejected
-> Go result submission
-> PostgreSQL terminal CheckRun
~~~

The target resolves to the private Compose network. Production policy must reject it; there is no test-only private-network bypass or public-internet dependency.

The smoke bounded-polls PostgreSQL, verifies one terminal `policy_rejected` CheckRun and zero pending rows, and correlates the same CheckID/MonitorID with Checker events:

~~~text
check_claimed
-> probe_completed(policy_rejected)
-> result_delivered
~~~

The fake-Docker harness validates shell control flow and diagnostics only. The real smoke is authoritative runtime evidence.

## Troubleshooting

### API is unhealthy

Inspect `docker compose ps` and API/DB logs. On a fresh volume, keep API running and apply migrations explicitly.

### Checker is unhealthy

~~~bash
docker compose ps
docker compose logs checker
docker compose logs api
~~~

Checker health means its worker loop started after configuration/runtime construction; it does not mean an external target is reachable.

### Migration status is unexpected

~~~bash
docker compose exec api /usr/local/bin/uptime-lab-migrate status
~~~

### Parallel worktrees interfere

Use a distinct `COMPOSE_PROJECT_NAME`.

## Current Limitations

- Web remains a placeholder; React is not implemented.
- Public Monitor surface remains create/read only.
- No public CheckRun/status/history endpoint exists.
- No mutable Monitor lifecycle exists.
- Only one logical Checker process is supported.
- Production probe execution rejects private/non-public destinations.
- No application host port is exposed.
- Authentication/authorization, CORS/rate limiting, ingress/TLS, and production deployment topology remain deferred.

## Related Architecture

- [Architecture index](../architecture/README.md)
- [Container View](../architecture/container-view.md)
- [Runtime Flows](../architecture/runtime-flows.md)
- [Data Ownership](../architecture/data-ownership.md)
- [Go Control Plane](../backend/go-control-plane.md)
- [Rust Checker](../checker/rust-checker.md)
- [Single-Checker Execution Testing](../testing/single-checker-execution-slice.md)
- [Repository Governance](repository-governance.md)
