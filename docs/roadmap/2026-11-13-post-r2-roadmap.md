# Post-R2 Roadmap

**Status:** PROPOSED — discussion aid only; no implementation, release, production migration, or remote-exposure authorization
**Prepared:** 2026-11-13
**Verified baseline:** `main@7dc0738` after R2 completion (#87/#88) and dependency bumps #66 (goose), #79 (pgx), #61 (upload-artifact) and #62 (setup-node), all squash-merged with green CI; the bumps do not change runtime behavior
**Sources:** [industry benchmark and launch roadmap](2026-10-02-industry-benchmark-and-launch-roadmap.md), [R0 product and release decision brief](2026-10-18-r0-product-release-decision-brief.md), [current handoff](../devops/current-handoff.md)

## 1. Landed state

R0a (current availability read), R1 (local Web vertical slice) and R2 (monitor management and evidence) are complete for their approved local scopes. Ten public operations are landed: create, inventory, read, latest-result, availability, scheduling GET/PUT, delete, history and target edit, plus the newest-50 retained history window and populated-data EXPLAIN evidence. Every landing is local-development acceptance, not deployed-system certification.

Path A (single-operator, local-only evaluation) remains the selected validation direction (recorded 2026-10-27). The nine R0 decisions are recorded DECIDED for the Path-A local-evaluation scope (2026-11-13) in the [R0 brief](2026-10-18-r0-product-release-decision-brief.md); Path-B items stay deferred. **Release judgment unchanged: NO-GO for a remotely accessible monitoring beta.** This proposal does not alter that judgment.

Canonical-document drift observed after #88:

- Root `README.md` still describes five public operations and defers history/lifecycle capabilities that have since landed.
- `docs/README.md` carries a duplicate Local Check-Run History plan entry and a stale "full history / mutable lifecycle deferred" sentence in the implemented-runtime section.
- `docs/devops/current-handoff.md` header still records the 2026-11-10 baseline and omits #84–#88 from the merged-PR list.

## 2. Proposed tracks (effort-ordered)

Sizes are relative effort, not calendar promises: S = one bounded session, M = one scope→design→plan→feature chain at the established R2 cadence, L = multiple chains.

| Track | Deliverable | Size | Depends on | Exit evidence |
|---|---|---|---|---|
| H — Housekeeping | Canonical doc reconciliation + local artifact cleanup | S | — | Docs-only PR; repository checks green; no runtime change |
| D — R0 minimal decision pass | Path-A-scoped values for R0-01…R0-09 | S | — | R0 brief records DECIDED for the local-evaluation scope only |
| P — Portfolio readiness | License selection + README demo | S | User license decision | Docs/assets PR; no runtime change |
| R3-1 — Confirmation/recovery | First monitoring-decisions slice | M | D (R0-04 minimal values) | TDD chain; real PostgreSQL + composed browser evidence; independent review; exact-main CI |
| R4 — Reliable notification | One channel with durable delivery | L | D (R0-04/R0-09), R3-1 | Separate scoped decision; provider-outage/restart evidence |
| R5 — Protected beta | Access, ingress, quotas, recovery, operations | L | Full R0 + explicit Path B decision | Production-readiness checklist in the launch roadmap |

Recommended order: **H → D → P → R3-1**. H and P are docs-only and may be batched or split; D unlocks R3-1; R4/R5 stay deferred behind explicit scoped decisions.

## 3. Track details

### H — Housekeeping and doc reconciliation

- Reconcile root `README.md` and `docs/README.md` to the ten-operation landed state; remove the duplicate history-plan index entry; refresh the handoff header (date, baseline, merged-PR list).
- Remove local scratch artifacts after user confirmation: untracked `explain-capture.tmp` / `explain-v2.tmp`, the empty `apps/api;C` directory (a known Git Bash/Docker path-conversion artifact), and the leftover `editdbg1` Compose stack still running from the target-edit debugging session.
- Exit: a docs-only PR with repository/documentation checks green; no runtime, contract, or migration change. Artifact removal is local cleanup, not a repository change, except where a `.gitignore` entry is warranted.

### D — R0 minimal decision pass (Path A scope)

Record explicit, minimal values for the nine R0 decisions **scoped to local evaluation only**, with concrete exclusions:

- R0-01/02: single operator, loopback-only entry, no external users, no ownership boundary beyond the local gateway.
- R0-03: existing bounded local workload; no admitted remote cap; the four-slot/60-second policy remains a local convention, not a supported workload.
- R0-04: detection expectations for local evaluation, including the confirmation/recovery direction that R3-1 will design against; no alert-delivery guarantee.
- R0-05/06: the repository owner is the operator; no hosting budget because no deployment is authorized.
- R0-07: the newest-50 retention stays a repository-owned local convention; sensitive-URL/log policy written down for local use.
- R0-08/09: no RPO/RTO commitment while local-only; single-region and notification-channel decisions explicitly deferred with Path B.

Exit: the R0 brief marks these items DECIDED for the Path A scope only. This records product scope, not release authorization; any later Path B pivot reopens the affected items.

### P — Portfolio readiness

- **License decision (user):** the repository currently has no license, which blocks reuse and weakens the portfolio presentation. Choose a license (e.g. MIT or Apache-2.0) or explicitly record "no license" as a decision; add `LICENSE` and update the README section.
- **README demo:** capture a short screenshot sequence or GIF of the real composed local journey (register → detail → history → edit → delete) using the existing 4173 loopback topology; surface the existing architecture diagrams from `docs/architecture/` at README level.
- Exit: a docs/assets PR; no runtime change; demo media produced from the real composed stack, not mocked UI.

### R3-1 — Confirmation/recovery (first monitoring-decisions slice)

The landed availability read treats a single terminal failure as `unavailable` immediately. The first R3 slice should separate a probe fact from a confirmed target outage:

- Candidate semantics: a fixed confirmation threshold (N consecutive terminal failures) before a confirmed-outage assessment, and a recovery threshold (M consecutive successes) before returning to available; `unknown/stale` and execution-failure precedence preserved.
- Explicitly out of scope for the first slice: notification delivery, uptime percentages, maintenance windows, configurable per-Monitor policy, and incident persistence beyond what the assessment requires. Whether durable incident open/resolve state belongs to slice 1 or slice 2 is a design decision.
- Governance: scope proposal → written design → TDD plan → feature PR, with real-PostgreSQL race/serialization evidence, composed Chromium journey evidence, fresh whole-branch review and exact-main CI, mirroring the R2 cadence.
- Depends on D: R0-04 must record the detection/confirmation expectation the design is judged against.

### R4 / R5 — deferred

R4 (one notification channel, durable delivery, deduplication, bounded retry) and R5 (identity/access, HTTPS ingress, quotas, backup/restore, rollback, operators) remain as defined in the launch roadmap. Neither starts without an explicit scoped decision; R5 additionally requires a Path B pivot and the full production-readiness checklist. A PostgreSQL-backed delivery queue/outbox is a candidate to assess at R4 time, not a selected design.

## 4. Explicit non-authorizations

This proposal authorizes no runtime feature, contract change, migration, release tag, deployment, production migration, remote exposure, or external provider connection. Track H/P doc work, Track D decision recording, and Track R3-1 implementation each require their own approval through the established scope→design→plan→PR chain. CI continues to run on GitHub-hosted `ubuntu-24.04`; no runner-fleet change is proposed.

## 5. Open decisions for the user

1. License choice (or explicit "no license" decision) — blocks Track P.
2. R0 minimal Path-A values in Track D — settled (2026-11-13): the user approved the values, recorded DECIDED for the Path-A local-evaluation scope in the [R0 brief](2026-10-18-r0-product-release-decision-brief.md).
3. Confirmation of local artifact cleanup in Track H (`.tmp` files, `apps/api;C`, `editdbg1` stack).
4. Whether R3-1 includes durable incident state or stops at the confirmed assessment — settled in its scope proposal, informed by the R0-04 value.