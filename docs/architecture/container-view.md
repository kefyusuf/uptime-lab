# Container View

**Architecture state:** Committed

**Implementation state:** Partial. PostgreSQL, the Go Control Plane, schema-aware readiness, and the public Monitoring HTTP transport are implemented. Web and Checker remain placeholders; the internal Checker contract and public network exposure are deferred.

## Purpose

This document is the C4 Level 2 view for uptime-lab. It distinguishes the committed end-state ownership relationships from the runtime surface that is actually implemented today.

## Containers and Responsibilities

### Web Client — React + TypeScript

The Web Client owns browser presentation and user interaction in the committed architecture. The React runtime is not implemented yet; the current Compose web service is a non-root placeholder.

The future browser will consume the defined public Monitoring contract at `contracts/openapi/public.yaml`. The Go transport now serves the contracted `/monitors` operations, but the React browser runtime is not implemented and Compose publishes no application host ports.

The browser never accesses PostgreSQL directly.

### Control Plane — Go

The Go Control Plane is implemented as the current real application runtime.

Implemented responsibilities include:

- immutable Monitoring domain values;
- RegisterMonitor and GetMonitor application use cases;
- a Monitoring create/read repository port;
- PostgreSQL persistence for monitoring.monitors;
- explicit goose-backed migrations;
- typed runtime configuration and structured logging;
- operational `GET /livez` and `GET /readyz` endpoints;
- read-only migration-set schema compatibility for readiness;
- the public Monitoring HTTP adapter for exactly `POST /monitors` and `GET /monitors/{monitorId}`;
- production composition of the Monitoring PostgreSQL repository, module, and HTTP adapter;
- graceful server lifecycle and bounded readiness checks.

The production API binary constructs the Monitoring repository/module/HTTP adapter and composes it behind the generic platform HTTP server. API startup does not apply migrations and does not require a successful schema query; schema state is evaluated by `/readyz`.

Go exclusively owns durable product state in PostgreSQL.

The Control Plane does not own low-level network probe execution.

### Checker / Execution Plane — Rust

The Rust execution runtime is not implemented yet. The current Compose checker service is a hardened placeholder that starts only after the real Go API becomes healthy.

The committed Rust runtime will own bounded network execution and will obtain work/submit normalized results through a future internal Go contract.

Rust never accesses PostgreSQL directly.

### PostgreSQL

PostgreSQL is the durable application store owned exclusively through Go.

The implemented application schema is intentionally minimal:

~~~text
monitoring.monitors
├── id uuid PRIMARY KEY
├── target_url text NOT NULL
└── created_at timestamptz NOT NULL
~~~

No enabled flag, updated_at/version field, scheduling table, check_runs table, or monitor_states table is implemented in this foundation.

### External HTTP/HTTPS Target

The monitored target remains external and untrusted.

Current Go TargetURL validation is only syntactic registration validation. No probe is executed in this milestone, and URL acceptance must not be interpreted as SSRF safety. DNS resolution, private/reserved-network blocking, redirect safety, rebinding protection, timeouts, and response budgets belong to the future Rust execution boundary.

### Docker Compose

Docker Compose is the implemented canonical local-development topology.

It runs exactly four services:

~~~text
web      placeholder
db       PostgreSQL
api      real Go Control Plane + Monitoring HTTP
checker  placeholder
~~~

No application host ports are published. API starts after healthy PostgreSQL but may remain live and unready until explicit migrations make the schema compatible. Checker waits for real API readiness.

## Current Runtime Diagram

~~~mermaid
flowchart LR
    Caller["Container-local API caller"]
    Web[Web placeholder]
    Go["Go Control Plane<br/>health + Monitoring HTTP"]
    Checker[Checker placeholder]
    DB[(PostgreSQL)]

    Caller -->|POST /monitors + GET /monitors/{monitorId}| Go
    Go -->|owned persistence| DB
    DB -->|service health prerequisite| Go
    Go -->|schema-aware service health prerequisite| Checker
~~~

## Committed Cross-Runtime Relationships

Current contract/transport state:

- Public contract: defined (`contracts/openapi/public.yaml`).
- Go public transport adapter: implemented.
- Internal checker contract: deferred.
- Public network exposure: deferred; Compose publishes no application host ports.

The following architectural relationships remain committed but are not all implemented yet:

| Relationship | Current state |
|---|---|
| Web -> Go: Public product API | Contract and Go transport implemented; Web caller and public network exposure remain deferred. |
| Rust -> Go: Internal control API | Deferred; Rust is still a placeholder. |
| Go -> PostgreSQL: Owned persistence | Implemented for Monitoring create/read state. |
| Rust -> Target: Bounded probe execution | Deferred; no probe execution runtime exists. |

Cross-runtime communication is contract-driven.

PostgreSQL is never used as a cross-runtime integration bus.

## Operational Health

The implemented Go HTTP surface contains operational health plus the two contracted Monitoring operations.

- `GET /livez` proves the process HTTP server is alive and does not touch PostgreSQL.
- `GET /readyz` performs a bounded PostgreSQL check and a read-only exact compatibility check against repository-owned embedded migration versions.
- `POST /monitors` creates a Monitor through the Monitoring application boundary.
- `GET /monitors/{monitorId}` reads a Monitor through the same boundary.

Readiness is false for an unreachable database, absent/invalid migration metadata, missing required versions, or unknown/ahead versions. The readiness path does not apply migrations or mutate schema.

## Ownership Rules

- Web owns presentation and interaction, not product persistence.
- Go owns business/domain coordination and durable product state.
- Rust owns bounded network execution, not product persistence.
- External targets are never trusted as internal system components.
- No browser or checker code may use PostgreSQL as an integration shortcut.

## Security Boundaries

The committed system has three trust transitions:

1. browser input entering a future public Go product boundary;
2. internal work/result data crossing a future Go/Rust contract;
3. untrusted target/DNS/network behavior entering the future Rust execution boundary.

The Go public Monitoring handler and PostgreSQL persistence boundary are implemented, but Compose does not expose an application host port. Authentication, CORS, rate limiting, ingress/TLS, Web, Rust execution, and the internal Checker contract remain outside this milestone.

## Related Decisions

- [Module Boundaries](module-boundaries.md)
- [Data Ownership](data-ownership.md)
- [ADR-0001: Multi-runtime monorepo](../adr/0001-multi-runtime-monorepo.md)
- [ADR-0002: Control plane and execution plane](../adr/0002-control-plane-and-execution-plane.md)
- [ADR-0003: Contract and data ownership](../adr/0003-contract-and-data-ownership.md)
