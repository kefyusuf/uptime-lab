# Local Development

## Status

Docker Compose is the canonical local-development substrate for uptime-lab.

The current topology runs:

- real PostgreSQL;
- the real Go Control Plane API runtime;
- placeholder Web;
- placeholder Checker.

This is a development environment, not a production deployment topology.

No application host ports are published.

## Prerequisites

The host requires:

- Git;
- Docker Engine or Docker Desktop;
- Docker Compose >= 2.22.0.

Docker is sufficient for the canonical local runtime. A host Go installation is optional.

Verify Compose:

~~~bash
docker compose version
~~~

## Canonical Topology

The root compose.yaml defines exactly four services:

~~~text
PostgreSQL (db)
      |
      | service_healthy
      v
Go Control Plane (api)
      |
      | service_healthy
      v
Checker placeholder

Web placeholder starts independently
~~~

Responsibilities:

- db is PostgreSQL and owns the project-scoped postgres-data named volume;
- api is the real non-root Go Control Plane runtime;
- checker is a hardened non-root placeholder that waits for healthy API;
- web is a hardened non-root placeholder with no hard startup dependency.

## Fresh Database Bootstrap

A fresh PostgreSQL volume intentionally leaves the API **live but unready** until migrations are applied explicitly. The canonical sequence verified by the real Docker smoke is:

~~~bash
docker compose build api
docker compose build web checker
docker compose up -d db api

docker compose exec -T api /usr/local/bin/uptime-lab-migrate up

docker compose up -d --wait --wait-timeout 60
docker compose ps
~~~

The important boundary is the order, not a sleep: start `db` + `api`, run the migration command explicitly inside the API container, then start/wait for the full four-service topology. Before migration, `/livez` is available while `/readyz` fails. After migration, API readiness succeeds and the Checker placeholder may become healthy.

## Follow Logs

Follow the Go API:

~~~bash
docker compose logs -f api
~~~

Or inspect individual services:

~~~bash
docker compose logs -f db
docker compose logs -f checker
docker compose logs -f web
~~~

API logs are structured JSON and include stable service/component/event fields.

## API Health

The API exposes operational health plus the live Monitoring product transport:

~~~text
GET  /livez
GET  /readyz
POST /monitors
GET  /monitors/{monitorId}
~~~

The Compose healthcheck calls the real readiness endpoint from inside the API container:

~~~text
http://127.0.0.1:8080/readyz
~~~

Semantics:

- `/livez` returns 200 without a database call;
- `/readyz` requires bounded PostgreSQL access **and** read-only compatibility with the repository-owned embedded migration set;
- readiness fails on a fresh/unmigrated database and does not create migration metadata or apply schema changes;
- `POST /monitors` and `GET /monitors/{monitorId}` are live inside the API container/network.

The repository-defined public Monitoring source contract remains `contracts/openapi/public.yaml`. The internal Checker product contract remains absent.

Because no host application port is published, health is normally observed through Compose health or container-local commands rather than host HTTP.

## Explicit Database Migrations

Migrations are **not** applied automatically by API startup.

Inspect migration status:

~~~bash
docker compose exec api /usr/local/bin/uptime-lab-migrate status
~~~

Apply pending migrations explicitly:

~~~bash
docker compose exec api /usr/local/bin/uptime-lab-migrate up
~~~

Rollback one migration when deliberately testing migration behavior:

~~~bash
docker compose exec api /usr/local/bin/uptime-lab-migrate down
~~~

The migration command is a separate binary packaged in the API image. It uses the same PostgreSQL environment mapping as the API service.

Do not treat docker compose down -v as a migration workflow.

## PostgreSQL Access

Open psql using the service-resolved values:

~~~bash
docker compose exec db sh -lc 'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB"'
~~~

Development defaults are:

~~~text
database: uptime_lab
user:     uptime_lab
password: uptime_lab_local
~~~

These defaults are development-only and must never be reused as production credentials.

PostgreSQL 18+ state is persisted from:

~~~text
/var/lib/postgresql
~~~

## Persistence

Stop containers while preserving local database state:

~~~bash
docker compose down
~~~

The project-scoped postgres-data volume remains.

Restart the already-built stack:

~~~bash
docker compose up -d --wait --wait-timeout 60
~~~

The existing migrated volume is reused, so no migration rerun is required for an unchanged migration set. Task 5 smoke also verifies that previously created Monitor state survives this normal down/up cycle.

## Destructive Reset

To remove containers **and delete local PostgreSQL state**:

~~~bash
docker compose down -v
~~~

This is destructive.

Use it only when intentionally resetting the local database. Normal schema evolution uses the explicit migration command instead.

## Optional Environment Overrides

The stack works without a copied environment file.

.env.example documents the supported local PostgreSQL overrides:

~~~dotenv
POSTGRES_DB=uptime_lab
POSTGRES_USER=uptime_lab
POSTGRES_PASSWORD=uptime_lab_local
~~~

A developer may create an untracked root .env with different local values.

The official PostgreSQL image applies initialization variables only when the data directory is empty. Changing them after initialization does not rewrite an existing cluster.

To intentionally apply new initialization values, run `docker compose down -v` and then repeat the [Fresh Database Bootstrap](#fresh-database-bootstrap). A destructive reset removes both product data and migration metadata, so a one-step `docker compose up --wait` is intentionally insufficient on the fresh volume.

The root .env is gitignored. Never place production/shared secrets in .env.example.

## Worktree and Project Isolation

The repository defines no fixed Compose project name, container names, globally named networks, or globally named volumes.

For parallel worktrees, assign a unique project name before running the canonical bootstrap:

~~~bash
export COMPOSE_PROJECT_NAME=uptime-lab-my-branch
~~~

Use the same project name for build, up, exec, ps, logs, down, and down -v in that worktree.

The repository smoke script automatically selects an isolated project name when one is not supplied.

## Optional Host Go Workflow

Docker remains the canonical local runtime, but developers with the reviewed Go toolchain can run Go verification directly:

~~~bash
cd apps/api
go version
test -z "$(gofmt -l .)"
go mod tidy
test -z "$(git status --porcelain -- go.mod go.sum)"
go mod verify
go vet ./...
go test ./...
go test -count=1 -race ./...
~~~

The reviewed foundation toolchain is Go 1.27.1.

Real PostgreSQL integration and canonical Docker smoke are still required CI evidence; host-only unit tests do not replace them.

## Repository Verification

Local-dev checks:

~~~bash
./scripts/ci/test-detect-local-dev-changes.sh
./scripts/ci/test-check-local-dev.sh
./scripts/ci/test-smoke-local-dev.sh
./scripts/ci/check-local-dev.sh .
./scripts/ci/smoke-local-dev.sh
docker compose config --quiet
~~~

The checks cover:

- API-source-sensitive local-dev change detection;
- four-service topology invariants;
- exact/pinned API Docker stages;
- non-root/read-only API runtime;
- real schema-aware `/readyz` health;
- live-before-ready behavior on a fresh database;
- explicit migration bootstrap with no startup auto-migration;
- real container-local `POST /monitors` -> `GET /monitors/{monitorId}` with exact `createdAt` round trip;
- Checker-after-API startup ordering;
- PostgreSQL/product persistence across normal down/up;
- PostgreSQL/product reset across down -v;
- deterministic cleanup.

The fake-Docker harness validates smoke control flow only. The real smoke is authoritative runtime evidence.

The stable aggregate required workflow check is CI / gate.

## Troubleshooting

### API is unhealthy

Inspect:

~~~bash
docker compose ps
docker compose logs api
docker compose logs db
~~~

Remember that `/readyz` tests both database reachability and repository migration compatibility. On a fresh volume, keep the API running and apply migrations explicitly; do not weaken readiness or expect API startup to migrate.

### Migration status is unexpected

Run:

~~~bash
docker compose exec api /usr/local/bin/uptime-lab-migrate status
~~~

Do not delete the database volume merely to apply ordinary migrations.

### PostgreSQL credentials changed after initialization

Use the documented destructive reset only if you intentionally want a fresh local cluster:

~~~bash
docker compose down -v
~~~

Then repeat the [Fresh Database Bootstrap](#fresh-database-bootstrap), including the explicit migration step.

### Parallel worktrees interfere

Use a distinct COMPOSE_PROJECT_NAME for each worktree.

## Current Limitations

- Web is a placeholder, not React.
- Checker is a placeholder, not Rust.
- The Go runtime serves only the current create/read Monitoring product surface; list/update/delete/lifecycle operations remain absent.
- No application host port is exposed by Compose, so a live product transport is not the same as public network deployment.
- No internal Checker API exists.
- No mutable monitor lifecycle exists.
- No scheduler/due-work/result history exists.
- No probe execution occurs.
- No host application port is exposed.
- There is no production deployment contract in compose.yaml.

## Related Architecture

- [Architecture index](../architecture/README.md)
- [Container View](../architecture/container-view.md)
- [Module Boundaries](../architecture/module-boundaries.md)
- [Data Ownership](../architecture/data-ownership.md)
- [Go Control Plane](../backend/go-control-plane.md)
- [Go Monitoring Testing](../testing/go-monitoring-foundation.md)
- [Public Monitoring Contract Testing](../testing/public-monitoring-contract.md)
- [Repository Governance](repository-governance.md)
