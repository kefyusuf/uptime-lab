# uptime-lab

A production-disciplined uptime monitoring laboratory built to exercise clear boundaries between a React/TypeScript web client, a Go control plane, and a Rust checker runtime.

## Status

`uptime-lab` is in the foundation phase. The architecture is approved, but the application runtimes are not implemented yet. The repository should not be treated as a production-ready monitoring service.

## Architecture

- **React + TypeScript** owns presentation and browser interaction.
- **Go** owns the modular-monolith control plane, domain/application rules, persistence ownership, and API semantics.
- **Rust** owns bounded concurrent network probe execution through Ports and Adapters.
- **PostgreSQL** will hold durable application state owned exclusively by the Go control plane.
- **Docker Compose** is the canonical local-development substrate. The current Docker foundation runs PostgreSQL plus process-level placeholders; Go, Rust, and React runtimes remain unimplemented.

The canonical architecture specification is [`docs/superpowers/specs/2026-09-17-uptime-lab-foundation-design.md`](docs/superpowers/specs/2026-09-17-uptime-lab-foundation-design.md).

## Local Development

See [`docs/devops/local-development.md`](docs/devops/local-development.md) for prerequisites, canonical Compose commands, persistence/reset semantics, worktree isolation, verification, and current limitations.

## Engineering Principles

- Boundaries before abstractions.
- Contract-driven runtime integration.
- One owner per source of truth.
- Tests at the cheapest meaningful layer.
- Security and observability are design constraints, not release-time patches.
- No distributed infrastructure without a concrete requirement.

## Repository Workflow

`main` is the only long-lived branch. Changes use short-lived branches, pull requests, Conventional Commits, required CI, and squash merge.

See [`CONTRIBUTING.md`](CONTRIBUTING.md) and [`docs/devops/repository-governance.md`](docs/devops/repository-governance.md) once the repository-governance baseline is present.

## Documentation

Start at [`docs/README.md`](docs/README.md).

## Security

Read [`SECURITY.md`](SECURITY.md) before reporting a vulnerability or exposing an experimental build to untrusted networks.

## License

No open-source license has been selected yet. Until a license is explicitly added, normal copyright rules apply.
