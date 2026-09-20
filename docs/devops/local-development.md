# Local Development

## Status

Docker Compose is the canonical local-development substrate for `uptime-lab`.

The current foundation runs one PostgreSQL service plus process-level placeholders for Web, API, and Checker. It exists to establish repeatable local infrastructure, startup ordering, health semantics, persistence, and CI-verifiable lifecycle behavior before application runtimes are introduced.

This is a development environment, not a production deployment topology.

## Prerequisites

The host requires:

- Git;
- Docker Engine or Docker Desktop;
- Docker Compose >= 2.22.0.

No host Go, Rust, Node.js, Make, or `just` installation is required for the current foundation phase.

Verify Compose before starting:

```bash
docker compose version
```

## Canonical Topology

The root `compose.yaml` defines exactly four services:

```text
PostgreSQL (db)
      |
      | service_healthy
      v
     api
      |
      | service_healthy
      v
   checker

web starts independently
```

Current responsibilities:

- `db` is PostgreSQL and owns the local named volume `postgres-data`;
- `api` is a generic non-root placeholder that waits for healthy PostgreSQL;
- `checker` is a generic non-root placeholder that waits for healthy API;
- `web` is a generic non-root placeholder and has no hard startup dependency.

All services communicate only through the Compose-private network. No application host port is published in this phase.

## Start the Stack

From the repository root:

```bash
docker compose up -d --build --wait
```

`--wait` is intentional. Startup readiness is health-based rather than sleep-based.

The current placeholder services define explicit Compose healthchecks that evaluate their readiness marker. Docker container lifecycle supplies the process-running evidence; a merely running container without the readiness marker is not considered healthy. API waits for a healthy database, and Checker waits for a healthy API.

## Inspect Service State

```bash
docker compose ps
```

Expected services are:

```text
web
db
api
checker
```

The exact Compose-generated resource names depend on the active Compose project name.

## Follow Logs

Follow the complete stack:

```bash
docker compose logs -f
```

Follow one service when diagnosing startup behavior:

```bash
docker compose logs -f db
docker compose logs -f api
docker compose logs -f checker
docker compose logs -f web
```

Placeholder logs expose only lifecycle events such as `starting`, `ready`, and `stopping`. They do not represent application traffic.

## Health and Readiness

PostgreSQL health is based on `pg_isready`.

Web, API, and Checker health is based on the readiness marker:

```text
/run/uptime-lab/ready
```

The placeholder image validates `SERVICE_NAME` before creating the readiness marker. Only these identities are valid:

```text
web
api
checker
```

An invalid or missing identity exits before readiness.

There is intentionally no fake HTTP health endpoint and no placeholder network listener.

## PostgreSQL Access

Open an interactive `psql` session using the values resolved inside the `db` service:

```bash
docker compose exec db sh -lc 'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB"'
```

This command therefore follows supported `.env` overrides as well as the documented defaults.

The default values are development-only:

```text
database: uptime_lab
user:     uptime_lab
password: uptime_lab_local
```

These defaults are not production credentials and must not be reused as real secrets.

PostgreSQL 18+ data is persisted from:

```text
/var/lib/postgresql
```

The project intentionally does not use the older `/var/lib/postgresql/data` volume target.

## Local Persistence

Stop and remove containers while preserving local database state:

```bash
docker compose down
```

The project-scoped `postgres-data` named volume remains.

A subsequent:

```bash
docker compose up -d --build --wait
```

reuses that volume.

This persistence behavior is part of the repository-owned smoke contract.

## Destructive Reset

To remove containers **and delete the local PostgreSQL volume**:

```bash
docker compose down -v
```

This is destructive. Local database contents cannot be recovered from the removed Compose volume unless they were backed up separately.

After a destructive reset, the next startup creates a fresh PostgreSQL data directory.

## Optional Environment Overrides

The stack works without copying an environment file.

Repository defaults are embedded with Compose fallback syntax. `.env.example` documents the supported local overrides:

```dotenv
POSTGRES_DB=uptime_lab
POSTGRES_USER=uptime_lab
POSTGRES_PASSWORD=uptime_lab_local
```

A developer may create an untracked root `.env` with different local values.

The official PostgreSQL image applies `POSTGRES_DB`, `POSTGRES_USER`, and `POSTGRES_PASSWORD` initialization behavior only when the database data directory is empty. Once `postgres-data` contains an initialized cluster, changing those values does not retrofit the existing cluster.

To apply changed initialization values through the canonical local workflow, intentionally reset local database state:

```bash
docker compose down -v
docker compose up -d --build --wait
```

The reset is destructive and removes the local PostgreSQL volume.

The root `.env` is gitignored and must not be committed.

Do not place production credentials, shared team secrets, or external-service credentials in `.env.example`.

## Worktree and Project Isolation

The repository does not define a fixed Compose project name, fixed container names, globally named networks, or globally named volumes.

That is deliberate: resources remain project-scoped, but isolation exists only when each Compose invocation resolves to a distinct project name. The ordinary single-worktree flow may use Compose's directory-derived project name.

Parallel worktrees are supported only with distinct Compose project names. When worktree directory basenames may collide, assign a unique project name explicitly:

```bash
COMPOSE_PROJECT_NAME=uptime-lab-my-branch docker compose up -d --build --wait
```

Use the same project name for subsequent `ps`, `logs`, `down`, and `down -v` commands for that worktree.

The repository smoke script creates an isolated project name automatically when one is not supplied.

## Compose Watch

Compose Watch is the preferred future development loop once real Web, API, and Checker source trees exist.

There are currently no `develop.watch` mappings.

Adding speculative watch paths before runtime source exists would create a false contract between Compose and directories that do not yet exist. Watch rules must therefore be introduced together with the runtime foundation that owns them.

## CI Verification

The Docker foundation is verified by repository-owned scripts rather than a separate YAML parsing toolchain:

```bash
./scripts/ci/test-detect-local-dev-changes.sh
./scripts/ci/test-check-local-dev.sh
./scripts/ci/check-local-dev.sh .
./scripts/ci/test-smoke-local-dev.sh
./scripts/ci/smoke-local-dev.sh
```

The checks cover:

- local-dev path-change detection;
- topology and phase invariants;
- Compose version floor;
- configuration validation;
- image build;
- health-based startup;
- PostgreSQL persistence across normal `down`;
- PostgreSQL reset across `down -v`;
- cleanup after both success and failure.

The real smoke requires Docker access. The fake-Docker harness validates smoke control flow independently of Docker availability.

The aggregate required workflow check remains `CI / gate`. Path-aware local-dev workflow integration is part of this Docker-foundation implementation and must be green before the implementation is landed.

## Troubleshooting

### Docker Compose is too old

If the smoke script reports that Compose is below 2.22.0, update Docker Desktop or the Docker Compose plugin before continuing.

### A service does not become healthy

Inspect state and logs:

```bash
docker compose ps
docker compose logs db
docker compose logs api
docker compose logs checker
docker compose logs web
```

Do not replace health checks with arbitrary startup sleeps.

### PostgreSQL credentials were overridden

Inspect your untracked root `.env` or shell environment. Compose environment variables override the development defaults.

If PostgreSQL was already initialized before those values changed, use the documented destructive `docker compose down -v` reset before expecting new initialization values to take effect.

### Local database state should be preserved

Use:

```bash
docker compose down
```

Do not use `-v`.

### Local database state should be discarded

Use:

```bash
docker compose down -v
```

This removes the project-scoped database volume.

### Parallel worktrees interfere with each other

Assign a distinct `COMPOSE_PROJECT_NAME` to each worktree and use it consistently for all Compose commands.

## Current Limitations

Web is a placeholder, not React.

API is a placeholder, not Go.

Checker is a placeholder, not Rust.

No product HTTP API exists.

No host application port is exposed.

No monitoring migrations/tables exist.

No probe execution occurs.

No real `develop.watch` mappings exist.

There is no production deployment contract in `compose.yaml`.

## Related Architecture

System-level ownership and cross-runtime boundaries remain canonical in:

- [Architecture index](../architecture/README.md);
- [Container View](../architecture/container-view.md);
- [Dependency Rules](../architecture/dependency-rules.md);
- [Data Ownership](../architecture/data-ownership.md);
- [Repository Governance](repository-governance.md).

The Docker environment implements local process topology and lifecycle only. It does not redefine the architectural ownership of the future React/TypeScript Web Client, Go Control Plane, Rust Execution Plane, or PostgreSQL durable product state.
