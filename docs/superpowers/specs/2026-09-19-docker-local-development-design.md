# Docker-first Local Development Environment Design

**Status:** Review candidate
**Date:** 2026-09-19
**Repository:** `kefyusuf/uptime-lab`
**Scope:** Canonical Docker-first local development substrate, placeholder lifecycle, PostgreSQL local persistence, developer operations, local-dev validation, and CI smoke verification.
**Decision state:** Discussion design approved; this written specification requires explicit user review before implementation planning begins.

---

## 1. Purpose

This specification defines the first concrete local-development substrate for `uptime-lab`.

The phase is intentionally infrastructure-only. Its purpose is to prove that the repository has one reproducible Docker Compose topology that can start, reach health, expose deterministic lifecycle evidence, preserve local PostgreSQL state, and shut down cleanly before the Go, Rust, and React runtimes are implemented.

This phase MUST NOT create product/runtime implementation merely to make Docker look complete.

The resulting environment establishes a stable execution shell into which later runtime foundations can replace placeholders incrementally:

```text
Docker foundation
  -> Go foundation replaces api placeholder
  -> Rust foundation replaces checker placeholder
  -> Frontend foundation replaces web placeholder
```

The local-development substrate is part of the architecture, but it does not own product semantics.

---

## 2. Goals

The Docker-first local development phase MUST provide:

1. one canonical root `compose.yaml`;
2. four canonical Compose services: `web`, `api`, `checker`, and `db`;
3. deterministic health/readiness behavior;
4. a real PostgreSQL service using a project-scoped named volume;
5. generic process-level placeholders for `web`, `api`, and `checker`;
6. no host-installed Go, Rust, or Node requirement for the canonical path;
7. one documented Compose-native lifecycle;
8. zero-config local startup with optional environment overrides;
9. no default host port publishing;
10. worktree-friendly Compose resource naming;
11. repository-owned static local-dev fitness checks;
12. path-aware CI smoke verification using the same Compose topology as local development;
13. contributor-facing documentation that clearly distinguishes implemented Docker infrastructure from not-yet-implemented runtimes.

---

## 3. Non-goals

This phase MUST NOT implement or decide:

- Go application code;
- Rust application code;
- React/TypeScript application code;
- real HTTP health or readiness endpoints;
- public or internal API endpoint paths;
- OpenAPI schemas;
- database migrations or business tables;
- runtime-specific Dockerfiles;
- application-specific environment-variable models;
- production deployment topology;
- production secret management;
- production resource limits;
- reverse proxy or TLS termination;
- authentication;
- Redis;
- message brokers;
- Adminer, pgAdmin, or other database UIs;
- observability backends;
- private-network monitoring;
- real Compose Watch source mappings;
- production restart semantics.

The phase must not create `apps/**`, `contracts/**`, migrations, `go.mod`, `Cargo.toml`, or `package.json`.

---

## 4. Architectural context

The accepted system architecture remains:

```text
Web Client (React/TypeScript)
        |
        | Public product API
        v
Go Control Plane
        |
        | Owned persistence
        v
PostgreSQL

Rust Checker
        |
        | Internal control API
        v
Go Control Plane

Rust Checker
        |
        | Bounded probe execution
        v
External HTTP/HTTPS Target
```

This Docker phase does not change those ownership rules.

The following remain normative:

- Go is the Control Plane.
- Rust is the Execution Plane.
- React/TypeScript is the Web Client.
- Go exclusively owns durable product state in PostgreSQL.
- Rust never accesses PostgreSQL directly.
- The browser never accesses PostgreSQL directly.
- Cross-runtime communication is contract-driven.
- Docker Compose is infrastructure, not a business/runtime owner.

---

## 5. Canonical Compose topology

The canonical local graph is:

```text
                  private Compose network

        +------------------+
        |       web        |
        |   placeholder    |
        +------------------+
           independent startup


+------------------+   service_healthy   +------------------+
|        db        | ------------------> |       api        |
|   PostgreSQL     |                     |   placeholder    |
+------------------+                     +------------------+
                                               |
                                               | service_healthy
                                               v
                                       +------------------+
                                       |     checker      |
                                       |   placeholder    |
                                       +------------------+
```

### 5.1 Service responsibilities

#### `db`

- official PostgreSQL image;
- real PostgreSQL readiness through `pg_isready`;
- project-scoped named volume for local data;
- private Compose network only;
- no host port by default.

#### `api`

- generic placeholder image;
- represents the future Go Control Plane process boundary only;
- hard dependency on healthy `db`;
- does not expose HTTP;
- does not invent a future API port;
- does not persist business data.

#### `checker`

- generic placeholder image;
- represents the future Rust Checker process boundary only;
- hard dependency on healthy `api`;
- does not access PostgreSQL;
- does not perform HTTP probes;
- does not expose a host port.

#### `web`

- generic placeholder image;
- represents the future Web Client process boundary only;
- starts independently;
- has no hard Compose dependency on `api`;
- does not expose a development server port in this phase.

### 5.2 Why Web starts independently

A future browser application logically consumes the API, but the frontend development process itself should not require API readiness in order to start.

The Docker phase therefore distinguishes:

```text
product communication dependency
!=
process startup dependency
```

Real frontend API-failure behavior belongs to the Frontend Foundation.

---

## 6. Compose file organization

The phase uses one canonical file:

```text
compose.yaml
```

This phase MUST NOT add:

- `compose.dev.yaml`;
- `compose.ci.yaml`;
- `compose.override.yaml`;
- development/CI profiles;
- production Compose configuration.

Local development and CI smoke verification use the same service graph.

Different behavior is expressed through lifecycle commands, not alternate topology files.

Profiles may be introduced later only for genuinely optional tools that are not part of the canonical product stack.

---

## 7. Placeholder image architecture

The three placeholder services share one implementation:

```text
deploy/docker/
└── placeholder/
    ├── Dockerfile
    └── entrypoint.sh
```

The image is parameterized by service identity, for example:

```text
SERVICE_NAME=web
SERVICE_NAME=api
SERVICE_NAME=checker
```

### 7.1 Placeholder responsibilities

The entrypoint performs only infrastructure lifecycle work:

1. validate the service name;
2. emit a startup lifecycle log;
3. create a deterministic readiness marker;
4. emit a ready lifecycle log;
5. remain as the foreground process;
6. receive SIGTERM/SIGINT;
7. remove readiness state;
8. emit a stopping lifecycle log;
9. exit cleanly.

Representative logs:

```text
service=api event=starting
service=api event=ready
service=api event=stopping
```

### 7.2 Placeholder forbidden behavior

The placeholder MUST NOT contain:

- an HTTP server;
- fake `/health` or `/ready` endpoints;
- business logic;
- application configuration semantics;
- fake API responses;
- runtime package managers;
- runtime-specific source trees;
- future contract assumptions.

The placeholder is temporary infrastructure and must remain easy to remove.

---

## 8. Placeholder security and lifecycle defaults

Generic placeholders use low-cost safe defaults:

- non-root runtime user;
- read-only root filesystem;
- writable readiness state only through tmpfs;
- `init: true`;
- explicit signal handling;
- no automatic restart policy.

Conceptually:

```yaml
read_only: true
tmpfs:
  - /run/uptime-lab
init: true
```

The readiness marker is located under:

```text
/run/uptime-lab/ready
```

### 8.1 Failure visibility

The phase intentionally does not use:

```yaml
restart: always
restart: unless-stopped
```

A failed process should remain visibly failed:

```text
healthy
-> process failure
-> exited/unhealthy
-> developer or CI sees failure
```

Restart policy is a later production/deployment decision.

### 8.2 PostgreSQL exception

The PostgreSQL container does not inherit the generic placeholder hardening contract.

The official PostgreSQL image keeps its own user, entrypoint, and writable data-directory semantics.

The Docker phase MUST NOT force the database through the generic placeholder wrapper.

---

## 9. Health and readiness semantics

The design distinguishes:

```text
container exists
!=
process running
!=
service ready
```

### 9.1 Database readiness

PostgreSQL readiness uses `pg_isready`.

### 9.2 Placeholder readiness

A placeholder becomes healthy only when:

- its expected process is alive; and
- its readiness marker exists.

A container being merely `running` is insufficient evidence.

### 9.3 Dependency conditions

Only true hard startup dependencies use:

```text
condition: service_healthy
```

Required graph:

```text
api     -> db healthy
checker -> api healthy
web     -> no hard dependency
```

### 9.4 Forbidden startup patterns

The canonical environment MUST NOT use arbitrary startup sleeps such as:

```text
sleep 5
sleep 10
```

It MUST NOT require a central wait script to compensate for missing health semantics.

---

## 10. Canonical developer workflow

Supported host prerequisites:

```text
Git
Docker Engine or Docker Desktop
Docker Compose >= 2.22.0
```

The canonical developer path does not require host-installed:

- Go;
- Cargo/Rust;
- Node.js;
- npm/pnpm;
- Make;
- just;
- a custom project CLI.

### 10.1 Startup

```bash
docker compose up -d --build --wait
```

The exit status of this command is the primary local stack-readiness evidence.

### 10.2 Inspection

```bash
docker compose ps
docker compose logs -f
```

### 10.3 Shutdown

```bash
docker compose down
```

This preserves the database named volume.

### 10.4 Destructive reset

```bash
docker compose down -v
```

This is explicitly destructive local-development behavior.

### 10.5 No wrapper API

This phase does not introduce canonical commands such as:

```text
make dev
just dev
./scripts/dev
npm run dev
```

A convenience wrapper may be considered later only when repeated cross-platform orchestration pain becomes concrete.

---

## 11. Compose Watch policy

Compose Watch is the preferred future development-loop mechanism.

The project supports a minimum Compose version of:

```text
Docker Compose >= 2.22.0
```

because the future local-development model expects `develop.watch` capability.

However, this Docker foundation MUST NOT add fake watch mappings before runtime source trees exist.

The progression is:

```text
Docker foundation
  -> establish Compose Watch as preferred mechanism
  -> no fake source mappings

Go foundation
  -> add API watch rules against real Go source

Rust foundation
  -> add Checker watch/rebuild rules against real Rust source

Frontend foundation
  -> add Web watch rules against real frontend source
```

This removes ambiguity between supporting Compose Watch and inventing placeholder source paths.

---

## 12. Network and host exposure

The canonical topology uses Compose-private networking.

This phase publishes no host-facing ports.

Specifically, it MUST NOT reserve or publish values such as:

```text
3000
5173
8080
5432
```

Real host-facing Web/API ports are selected only when those runtimes exist.

The Rust Checker is not expected to require a host-facing application port by default.

### 12.1 Database inspection

Canonical database inspection is container-native:

```bash
docker compose exec db psql -U uptime_lab -d uptime_lab
```

A host database-client port may be considered later as an explicit optional developer aid, but it is not part of the default topology.

---

## 13. Resource naming and worktree isolation

The design relies on native Compose project scoping.

The Compose file MUST NOT define:

- fixed `container_name`;
- a root fixed project `name:`;
- globally named network resources;
- globally named PostgreSQL volumes.

Logical resources remain simple:

```text
services:
  web
  api
  checker
  db

volumes:
  postgres-data
```

Compose generates project-scoped physical resources.

This supports simultaneous worktrees and parallel CI/local environments without manual naming logic.

A developer may explicitly set `COMPOSE_PROJECT_NAME` when useful, but it is not part of the required canonical workflow.

---

## 14. PostgreSQL local persistence

The database uses a project-scoped named volume.

Normal shutdown:

```bash
docker compose down
```

preserves local data.

Explicit reset:

```bash
docker compose down -v
```

removes the project volume.

### 14.1 Forbidden persistence model

The phase MUST NOT bind-mount PostgreSQL data from a host path such as:

```text
./.data/postgres
```

This avoids cross-platform filesystem and permission coupling.

### 14.2 Integration-test isolation

Future integration tests MUST NOT depend on this persistent local-development volume.

Integration tests will use their own ephemeral PostgreSQL lifecycle.

---

## 15. Local configuration model

The canonical local environment is zero-config.

The following development-only defaults are acceptable:

```text
POSTGRES_DB=uptime_lab
POSTGRES_USER=uptime_lab
POSTGRES_PASSWORD=uptime_lab_local
```

Compose may use default interpolation:

```text
${POSTGRES_DB:-uptime_lab}
${POSTGRES_USER:-uptime_lab}
${POSTGRES_PASSWORD:-uptime_lab_local}
```

These values are not treated as production secrets.

### 15.1 Optional overrides

The repository may include:

```text
.env.example
```

while:

```text
.env
```

remains gitignored.

Copying `.env.example` is optional, not a prerequisite for canonical startup.

### 15.2 Deferred configuration

Runtime-specific environment variables remain deferred until the corresponding runtime foundation exists.

This phase does not select a production secret manager or Docker Secrets workflow.

---

## 16. Image/version policy

The phase uses:

- an official PostgreSQL image;
- an official minimal base image for placeholders.

Rules:

- floating `latest` is forbidden;
- versions are explicit and source-controlled;
- implementation planning verifies the concrete current image versions;
- digest pinning is preferred where it does not create disproportionate maintenance friction;
- this design specification does not lock a concrete digest.

No production image policy is inferred from the placeholder base image.

---

## 17. Local-development fitness checks

The phase introduces repository-owned validation:

```text
scripts/ci/
├── check-local-dev.sh
└── test-check-local-dev.sh
```

### 17.1 Static invariants

`check-local-dev.sh` must mechanically verify at least:

- services `web`, `api`, `checker`, and `db` exist;
- no host `ports:` mappings exist;
- no `container_name`;
- no fixed root Compose project name;
- `db` uses a named volume;
- `api` waits for healthy `db`;
- `checker` waits for healthy `api`;
- `web` has no hard startup dependency;
- `web`, `api`, and `checker` use the same generic placeholder contract;
- placeholders use `init: true`;
- placeholders use a read-only root filesystem;
- placeholder readiness state uses tmpfs;
- placeholders have no auto-restart policy;
- no floating `latest` image;
- no runtime source/scaffold enters the Docker phase.

The implementation should prefer dependency-free repository-owned shell checks while they remain deterministic and readable.

If reliable YAML inspection becomes too brittle without a parser, that is a design-review trigger rather than permission to silently add a framework/toolchain.

### 17.2 Test harness

`test-check-local-dev.sh` uses isolated fixtures.

Representative required cases:

1. canonical fixture passes;
2. missing checker service fails;
3. published host port fails;
4. `container_name` fails;
5. API without DB `service_healthy` fails;
6. Checker without API `service_healthy` fails;
7. Web hard dependency fails;
8. floating `latest` tag fails;
9. missing placeholder hardening invariant fails;
10. fixed/global resource naming fails.

The exact final count is locked by the implementation plan, but positive and representative negative evidence are mandatory.

---

## 18. Path-aware CI design

The repository keeps one stable required gate:

```text
CI / gate
```

Docker foundation extends CI to:

```text
PR
├── policy
├── repository
├── changes
├── local-dev
└── CI / gate
```

### 18.1 Repository-owned change detection

The design prefers repository-owned diff logic rather than a new third-party path-filter action.

The `changes` job produces a deterministic signal such as:

```text
local_dev=true|false
```

Initial relevant paths include:

- `compose.yaml`;
- `deploy/docker/**`;
- `.env.example`;
- local-dev validation scripts;
- `docs/devops/local-development.md`;
- `.github/workflows/ci.yml`.

Future runtime foundations may add their source paths to local-dev relevance when container builds depend on them.

### 18.2 Local-dev smoke job

When relevant, CI performs real Compose verification:

```bash
docker compose version
docker compose config --quiet
docker compose build
docker compose up -d --wait --wait-timeout 60
docker compose ps
```

Cleanup always runs:

```bash
docker compose down -v --remove-orphans
```

with workflow semantics equivalent to:

```yaml
if: always()
```

### 18.3 CI does not run Compose Watch

`docker compose watch` is an interactive long-running developer loop and is not CI evidence.

CI validates build, configuration, health-based startup, state inspection, and cleanup.

### 18.4 Aggregate gate behavior

The aggregate gate requires:

```text
policy      success
repository  success
changes     success
local-dev   success OR legitimate skip
```

Docker-related changes must not reach a green `CI / gate` if `local-dev` fails.

---

## 19. Documentation model

Canonical local-development documentation is:

```text
docs/devops/local-development.md
```

It owns:

- prerequisites;
- supported Compose floor;
- canonical startup;
- service topology;
- health/readiness semantics;
- service inspection;
- logs;
- database inspection;
- persistence behavior;
- destructive reset behavior;
- optional `.env` overrides;
- worktree/project isolation;
- Compose Watch future model;
- troubleshooting;
- current limitations.

Other documentation links to this file instead of duplicating operational instructions.

The root README and/or `docs/README.md` may add a short navigation link only.

---

## 20. Current limitations to document explicitly

After this phase, the stack contains placeholders rather than functional application runtimes.

Documentation MUST state that:

- Web is not a React application yet;
- API is not a Go server yet;
- Checker is not a Rust worker yet;
- no public/internal API exists yet;
- no product HTTP port is exposed;
- no migrations or monitoring tables exist;
- no real probe execution occurs;
- no application Compose Watch mapping exists yet.

These are intentional phase boundaries, not missing work inside this phase.

---

## 21. Failure semantics

The following are hard failures:

- invalid Compose configuration;
- image build failure;
- unhealthy PostgreSQL;
- API placeholder never becoming healthy;
- Checker placeholder never becoming healthy;
- placeholder process exit;
- static local-development invariant failure;
- Docker smoke cleanup failure when cleanup itself is broken.

Canonical startup must return non-zero when required services do not reach health within the configured wait budget.

The design does not hide failures behind restart loops or arbitrary sleep-based retries.

---

## 22. Implementation file boundary

The implementation plan derived from this design may create or modify only:

```text
compose.yaml
.env.example

deploy/docker/placeholder/**

docs/devops/local-development.md

scripts/ci/check-local-dev.sh
scripts/ci/test-check-local-dev.sh
narrow repository-owned change-detection scripts under scripts/ci/**

.github/workflows/ci.yml

README.md
docs/README.md
```

The following are forbidden for this phase:

```text
apps/**
contracts/**
migrations/**
package.json
go.mod
Cargo.toml
```

Any requirement to cross this boundary reopens the design gate.

---

## 23. ADR impact

No new ADR is required for this phase.

Docker Compose as the canonical local-development environment is already accepted by foundation decision FD-014.

This design refines that accepted direction rather than replacing it.

A new ADR would be required if implementation discovered a need to change the architectural direction, for example:

- replacing Docker Compose with another orchestrator;
- requiring host runtime toolchains for the canonical workflow;
- exposing PostgreSQL as a shared integration surface;
- coupling production and local topology into one deployment model.

---

## 24. Acceptance criteria

The Docker-first Local Development Environment is complete only when all criteria below are satisfied.

### 24.1 Topology

- canonical root `compose.yaml` exists;
- `web`, `api`, `checker`, and `db` exist;
- no real Go/Rust/React runtime scaffold exists;
- services use the private Compose network;
- no host port is published.

### 24.2 Dependency and health

- DB uses real PostgreSQL readiness;
- API waits for healthy DB;
- Checker waits for healthy API;
- Web starts independently;
- all four services can reach healthy state;
- arbitrary startup sleeps are absent.

### 24.3 Placeholder lifecycle

- one generic placeholder image is used;
- placeholder processes are non-root;
- placeholder root filesystems are read-only;
- readiness state is tmpfs-backed;
- signal handling is explicit;
- no auto-restart policy exists;
- no fake HTTP listener exists.

### 24.4 Persistence

- DB data uses a project-scoped named volume;
- normal `docker compose down` preserves data;
- `docker compose down -v` performs explicit reset;
- no host bind-mounted database data directory exists.

### 24.5 Configuration

- canonical startup works without a required `.env`;
- optional overrides are documented;
- tracked files contain no real secret;
- application-specific configuration is not invented.

### 24.6 Developer workflow

- `docker compose up -d --build --wait` succeeds;
- `docker compose ps` provides inspection evidence;
- `docker compose logs -f` is documented;
- `docker compose down` succeeds;
- supported minimum Compose version is documented as 2.22.0 or newer.

### 24.7 CI

- static local-dev tests are green;
- Compose configuration is validated;
- images build in CI;
- stack reaches healthy state in CI;
- cleanup always executes;
- local-dev smoke is path-aware;
- aggregate `CI / gate` remains the required stable check.

### 24.8 Documentation

- canonical local-development document exists;
- zero-config startup is documented;
- persistence/reset behavior is explicit;
- worktree/project isolation is explained;
- Compose Watch future behavior is explained without fake mappings;
- current placeholder limitations are explicit.

---

## 25. Decisions locked by this specification

Once this written specification is approved, the following decisions are locked for the Docker foundation:

- **DL-001:** Docker Compose is the canonical local-development environment.
- **DL-002:** The phase is infrastructure-only; no real Go/Rust/React scaffold.
- **DL-003:** Web/API/Checker use one generic process-level placeholder; no fake HTTP endpoints.
- **DL-004:** Hard startup dependency graph is DB -> API -> Checker; Web starts independently.
- **DL-005:** Canonical host requirements are Git + Docker + Docker Compose >= 2.22.0; host Go/Rust/Node are not required.
- **DL-006:** PostgreSQL local state uses a project-scoped named volume; reset is explicit through `down -v`.
- **DL-007:** Default topology is private-network only; no host ports are published.
- **DL-008:** Readiness is service-specific and deterministic; canonical startup uses Compose `--wait`; arbitrary sleeps are forbidden.
- **DL-009:** A single generic placeholder image is used; runtime-specific placeholder Dockerfiles are forbidden.
- **DL-010:** Local DB configuration uses zero-config development defaults with optional gitignored `.env` override; no real secrets.
- **DL-011:** Canonical lifecycle is direct `docker compose`; no Make/just/custom CLI abstraction.
- **DL-012:** Local-dev CI is path-aware and provides real Compose smoke evidence under the stable `CI / gate`.
- **DL-013:** The phase uses one root `compose.yaml`; no dev/CI override or profile matrix.
- **DL-014:** Placeholders are non-root, read-only, tmpfs-readiness based, use `init: true`, and have no auto-restart policy.
- **DL-015:** Fixed `container_name`, fixed Compose project name, and globally named Compose resources are forbidden.
- **DL-016:** Compose Watch is the preferred future dev-loop mechanism; real watch rules are deferred until runtime source exists.
- **DL-017:** Local development and CI smoke validate the same Compose service graph.
- **DL-018:** `docs/devops/local-development.md` is the canonical operational documentation for this subsystem.

---

## 26. Phase maturity after landing

After this phase lands:

```text
Repository governance               Implemented
Architecture documentation          Implemented
Docker local-development substrate  Implemented

Go Control Plane                    Committed / not implemented
Rust Execution Plane                Committed / not implemented
React Web Client                    Committed / not implemented
Public/Internal contracts           Committed concept / not implemented
Product vertical slice              Not implemented
```

The Docker phase must not be interpreted as application readiness.

---

## 27. Implementation and review sequence

After explicit written-spec approval:

1. invoke the writing-plans workflow;
2. create a detailed Docker-first Local Development Environment implementation plan;
3. keep the plan infrastructure-only;
4. execute the plan on a short-lived branch;
5. require task-level self-review after each architectural/implementation decision;
6. run static local-dev tests and real Compose smoke verification;
7. open a PR against `main`;
8. require fresh `CI / gate` success before merge;
9. squash-merge only through the active repository governance rules.

No Go, Rust, React, contract, or product implementation begins as part of this phase.

---

## 28. Next gate

This specification is now the review boundary.

After the user explicitly approves the written specification, the next step is only:

```text
Docker-first Local Development Environment implementation plan
```

The next project phase after Docker foundation eventually lands is:

```text
Go Monitoring Foundation — scope/design gate
```

That phase does not start automatically.
