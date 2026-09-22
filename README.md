# uptime-lab

A production-disciplined uptime monitoring laboratory built to exercise clear boundaries between a React/TypeScript web client, a Go control plane, and a Rust checker runtime.

## Status

uptime-lab is still in the foundation phase and is not a production-ready monitoring service.

The **Go Monitoring Foundation is implemented**: the repository contains the Go Control Plane runtime, immutable Monitoring registration/read application behavior, PostgreSQL persistence and migrations, operational health endpoints, Docker integration, and CI verification.

The public Monitoring OpenAPI contract is now defined as a source artifact at `contracts/openapi/public.yaml`, covering exactly `POST /monitors` and `GET /monitors/{monitorId}`. The Go runtime still exposes only `/livez` and `/readyz`; no public product handler or transport adapter is wired. The internal checker contract, mutable monitor lifecycle, scheduling/results, Rust checker runtime, React web runtime, authentication/CORS, and public exposure remain deferred. Web and Checker are still process-level placeholders in the canonical Docker Compose topology.

## Architecture

- **React + TypeScript** owns presentation and browser interaction. Its runtime is not implemented yet.
- **Go** owns the modular-monolith control plane, domain/application rules, PostgreSQL persistence ownership, migrations, and API semantics. The current runtime exposes operational health only.
- **Rust** owns bounded concurrent network probe execution through Ports and Adapters. Its runtime is not implemented yet.
- **PostgreSQL** holds durable application state owned exclusively by the Go Control Plane. The implemented Monitoring namespace currently contains only monitoring.monitors.
- **Docker Compose** is the canonical local-development substrate. It runs PostgreSQL, the real Go API, and placeholder Web/Checker processes without publishing application host ports.

The canonical architecture specification is [docs/superpowers/specs/2026-09-17-uptime-lab-foundation-design.md](docs/superpowers/specs/2026-09-17-uptime-lab-foundation-design.md).

The implemented Go foundation is documented in [docs/backend/go-control-plane.md](docs/backend/go-control-plane.md).

## Local Development

See [docs/devops/local-development.md](docs/devops/local-development.md) for prerequisites, canonical Compose commands, explicit migrations, persistence/reset semantics, worktree isolation, verification, and current limitations.

## Testing

See [docs/testing/go-monitoring-foundation.md](docs/testing/go-monitoring-foundation.md) for the evidence model covering domain/application tests, architecture fitness functions, PostgreSQL integration, runtime lifecycle tests, Docker smoke, race detection, and vulnerability scanning.

See [docs/testing/public-monitoring-contract.md](docs/testing/public-monitoring-contract.md) for the public Monitoring OpenAPI source contract, semantic fitness checks, and CI behavior. Contract verification does not imply Go transport conformance.

## Engineering Principles

- Boundaries before abstractions.
- Contract-driven runtime integration.
- One owner per source of truth.
- Tests at the cheapest meaningful layer.
- Security and observability are design constraints, not release-time patches.
- No distributed infrastructure without a concrete requirement.

## Repository Workflow

main is the only long-lived branch. Changes use short-lived branches, pull requests, Conventional Commits, required CI, and squash merge.

See [CONTRIBUTING.md](CONTRIBUTING.md) and [docs/devops/repository-governance.md](docs/devops/repository-governance.md).

## Documentation

Start at [docs/README.md](docs/README.md).

## Security

Read [SECURITY.md](SECURITY.md) before reporting a vulnerability or exposing an experimental build to untrusted networks.

## License

No open-source license has been selected yet. Until a license is explicitly added, normal copyright rules apply.
