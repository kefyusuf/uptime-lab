# Documentation

Project documentation is organized by responsibility and by cross-runtime architecture.

## Canonical Design

- Foundation design: [superpowers/specs/2026-09-17-uptime-lab-foundation-design.md](superpowers/specs/2026-09-17-uptime-lab-foundation-design.md)
- Go Monitoring Foundation design: [superpowers/specs/2026-09-20-go-monitoring-foundation-design.md](superpowers/specs/2026-09-20-go-monitoring-foundation-design.md)
- Public Monitoring API Contract design: [superpowers/specs/2026-09-22-public-monitoring-api-contract-design.md](superpowers/specs/2026-09-22-public-monitoring-api-contract-design.md)
- Implementation plans: [superpowers/plans/](superpowers/plans/)

Design/spec/plan documents preserve the decision history for the phase in which they were written. Current implementation truth is described by the canonical architecture, backend, testing, and devops documents below.

## Implemented Foundation

- Go Control Plane: [backend/go-control-plane.md](backend/go-control-plane.md)
- Go Monitoring verification: [testing/go-monitoring-foundation.md](testing/go-monitoring-foundation.md)
- Public Monitoring contract verification: [testing/public-monitoring-contract.md](testing/public-monitoring-contract.md)
- Canonical local runtime: [devops/local-development.md](devops/local-development.md)

The Go Monitoring Foundation implements immutable Register/Get behavior and PostgreSQL persistence. The public Monitoring OpenAPI contract now exists as the source artifact `contracts/openapi/public.yaml`, but the Go runtime still exposes only operational `/livez` and `/readyz`; no public product handler is wired. The internal checker contract remains undefined and unimplemented. Authentication, CORS, and public network exposure remain deferred.

## Architecture

Start with the [Architecture index](architecture/README.md) for the canonical system context, runtime boundaries, dependency rules, data ownership, runtime/change flows, and ADRs.

Architecture documentation describes end-to-end relationships across web, API, checker, persistence, security, and operations. Area-specific documentation links back to those cross-area flows instead of becoming isolated silos.

## Local Development

Use [devops/local-development.md](devops/local-development.md) as the canonical operational guide for the Docker Compose environment, explicit migrations, persistence/reset semantics, worktree isolation, and local-dev verification.

## Area Ownership

- architecture/: system-level boundaries and cross-area flows.
- backend/: implemented Go Control Plane behavior and boundaries.
- testing/: cross-runtime test strategy and implementation evidence.
- frontend/: React/TypeScript architecture and browser concerns when that runtime is implemented.
- checker/: Rust checker execution architecture when that runtime is implemented.
- devops/: repository governance, local development, CI/CD, and release operations.
- security/: threat model and security controls.
- operations/: health, observability, and troubleshooting.
- adr/: accepted material architecture decisions.
- superpowers/: historical design specifications and implementation plans.

Directories are created when they first contain an owned document; speculative runtime documentation is not pre-created.
