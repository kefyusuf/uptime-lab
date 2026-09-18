# Documentation

Project documentation is organized by responsibility and by cross-runtime architecture.

## Canonical Design

- Foundation design: [`superpowers/specs/2026-09-17-uptime-lab-foundation-design.md`](superpowers/specs/2026-09-17-uptime-lab-foundation-design.md)
- Implementation plans: [`superpowers/plans/`](superpowers/plans/)

## Architecture

Start with the [Architecture index](architecture/README.md) for the canonical system context, runtime boundaries, dependency rules, data ownership, runtime/change flows, and ADRs.

Architecture documentation describes end-to-end relationships across web, API, checker, persistence, security, and operations. Area-specific documentation must link back to those cross-area flows instead of becoming isolated silos.

## Area Ownership

- `architecture/`: system-level boundaries and cross-area flows.
- `frontend/`: React/TypeScript architecture and browser concerns.
- `backend/`: Go control-plane architecture and modular-monolith rules.
- `checker/`: Rust checker execution architecture.
- `devops/`: repository governance, local development, CI/CD, and release operations.
- `testing/`: cross-runtime test strategy and evidence model.
- `security/`: threat model and security controls.
- `operations/`: health, observability, and troubleshooting.
- `adr/`: material architecture decisions once the ADR baseline exists.

Directories are created when they first contain an owned document; empty documentation trees are not pre-created.
