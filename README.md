# uptime-lab

A production-disciplined uptime monitoring laboratory built around explicit boundaries between a React/TypeScript web client, a Go Control Plane, a Rust Checker, and PostgreSQL.

## Status

uptime-lab is not a production-ready public monitoring service.

The **Single-Checker Execution Vertical Slice is implemented**. The repository now contains:

- the real Go Control Plane runtime;
- immutable Monitor registration/read behavior;
- Go-owned due-work coordination and CheckRun persistence;
- the internal Go/Rust claim/result contract;
- the real Rust Checker with bounded HTTP/HTTPS execution;
- deny-by-default production destination policy;
- PostgreSQL migrations for `monitoring.monitors` and `monitoring.check_runs`;
- schema-aware readiness;
- a real four-service Docker Compose topology;
- cross-runtime Docker evidence from claim through terminal CheckRun persistence.

The public Monitoring contract at `contracts/openapi/public.yaml` remains intentionally small: `POST /monitors` and `GET /monitors/{monitorId}`. There is still no public CheckRun/status/history API, mutable Monitor lifecycle, React runtime, authentication/authorization, CORS/rate limiting, public ingress, or application host port.

## Architecture

- **React + TypeScript** owns presentation and browser interaction. Its runtime remains deferred.
- **Go** owns product/domain coordination, public and internal API semantics, scheduling truth, PostgreSQL persistence, migrations, readiness, and CheckRun completion semantics.
- **Rust** owns bounded network execution, destination validation, HTTP/HTTPS probing, transport normalization, and result delivery. Rust never accesses PostgreSQL directly.
- **PostgreSQL** holds durable product state owned exclusively through Go. The Monitoring namespace contains `monitoring.monitors` and `monitoring.check_runs`.
- **Docker Compose** is the canonical local-development substrate. It runs PostgreSQL, the real Go API, the real Rust Checker, and a Web placeholder without publishing application host ports.

Start with the [architecture index](docs/architecture/README.md), the [Go Control Plane guide](docs/backend/go-control-plane.md), and the [Rust Checker guide](docs/checker/rust-checker.md).

## Local Development

See [docs/devops/local-development.md](docs/devops/local-development.md) for the canonical four-service topology, explicit migrations, persistence/reset semantics, worktree isolation, and cross-runtime smoke commands.

## Testing

- [Go Monitoring Foundation](docs/testing/go-monitoring-foundation.md)
- [Go Public Monitoring Transport](docs/testing/go-public-transport-adapter.md)
- [Public Monitoring Contract](docs/testing/public-monitoring-contract.md)
- [Single-Checker Execution Slice](docs/testing/single-checker-execution-slice.md)

The execution-slice guide separates internal-contract, Go/PostgreSQL, Rust policy/probe/client/worker, Docker cross-runtime, race, and vulnerability evidence.

## Engineering Principles

- Boundaries before abstractions.
- Contract-driven runtime integration.
- One owner per source of truth.
- Tests at the cheapest meaningful layer.
- Security and observability are design constraints, not release-time patches.
- No distributed infrastructure without a concrete requirement.

## Repository Workflow

`main` is the only long-lived branch. Changes use short-lived branches, pull requests, Conventional Commits, required CI, and squash merge.

See [CONTRIBUTING.md](CONTRIBUTING.md) and [docs/devops/repository-governance.md](docs/devops/repository-governance.md).

## Documentation

Start at [docs/README.md](docs/README.md).

## Security

Read [SECURITY.md](SECURITY.md) before reporting a vulnerability or exposing an experimental build to untrusted networks.

## License

No open-source license has been selected yet. Until a license is explicitly added, normal copyright rules apply.
