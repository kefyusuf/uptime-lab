# R0 Product and Release Decision Brief

**Status:** DECIDED for the Path-A local-evaluation scope (recorded 2026-11-13) — external monitoring beta remains NO-GO; Path-B items deferred; no release, deployment, or remote-exposure authorization
**Prepared:** 2026-10-18
**Verified code baseline:** `main@2e4db4f8a8cbb5b794f68dc52a4e8550b429210b`, [post-merge CI #38974461560](https://github.com/kefyusuf/uptime-lab/actions/runs/38974461560) successful (preparation baseline; the 2026-11-13 decision record was approved against `main@5658a71`)
**Source of phase requirements:** [Industry benchmark and launch roadmap](2026-10-02-industry-benchmark-and-launch-roadmap.md), especially R0 and the release gates

## Purpose and boundary

Record the **minimum product, operator and recovery decisions** needed to choose a subsequent, bounded milestone. This is a decision worksheet, not an ADR, implementation authorization, release plan, accepted capacity guarantee, production readiness certification, or approval to expose the service remotely.

Local development behavior is already demonstrated: Go and Rust exercise real execution/persistence; React offers loopback-only registration, bounded inventory, detail, current availability, raw latest result, persisted Pause/Resume, delete, bounded check-run history and target-URL edit. R0a, R1 and R2 are complete for their local scopes (ten public operations). Incident decisions (R3), delivery (R4), and protected beta readiness (R5) are not complete.

**Release judgment: NO-GO for a remotely accessible monitoring beta.** There is no approved remote audience, admitted workload, authentication/ownership boundary, external ingress model, notification service, production backup/restore drill, or operator incident process. Passing local Docker and Chromium CI does not close these gates. Nothing below changes that judgment.

## Candidate validation paths (not selected)

| Candidate | Purpose | Boundary |
|---|---|---|
| A — Single-operator, local-only evaluation | Validate the current real Monitor workflow and collect evidence without adding identities or remote ingress | Loopback-only; no external users, alert-delivery guarantee, public SLO, or production availability claim |
| B — Protected invite-only beta | Offer dependable monitored checks to a deliberately bounded set of operators | Requires selected audience and limits plus R3 confirmed incident semantics, R4 reliable notification, and R5 access, network, security, recovery and operations gates |
| C — Open-registration service | Public signup and multi-user operations | **Not an R0 default.** Requires additional R6 abuse, isolation, support and capacity evidence |

**Recommended order for discussion:** first decide whether A is sufficient for evidence gathering or whether B is genuinely required for the next validation. This is a recommendation, **not** an approved choice. Choosing A cannot be used to describe the product as a protected monitoring beta.

**Recorded 2026-10-27:** the user selected Path A as the validation direction for the next increment and directed a local monitor-deletion scope proposal. The nine R0 decisions below remain OPEN; the Path A selection alone does not approve any release, exposure or beta description.

**Recorded 2026-11-13:** the user approved the minimal Path-A values for all nine R0 decisions (see [Recorded Path-A decision values](#recorded-path-a-decision-values-2026-11-13)), as proposed in the [post-R2 roadmap](2026-11-13-post-r2-roadmap.md) (#89) and reconciled in #90. This records product scope for single-operator local evaluation only. It does not approve any release, remote exposure, beta description, production migration, or deployment; the external monitoring beta remains NO-GO. A later Path-B pivot reopens the affected items with full acceptance evidence. The 2026-10-27 paragraph above is preserved as the historical record of that date.

## R0 decisions requiring explicit approval

All entries are **DECIDED for the Path-A local-evaluation scope** (recorded 2026-11-13); the chosen values and exclusions are in [Recorded Path-A decision values](#recorded-path-a-decision-values-2026-11-13). A Path-B pivot reopens the affected items. A question or candidate is not an implementation requirement until the user records a chosen value and its rationale.

| ID | Decision owner must settle | Minimum recorded acceptance evidence | Status |
|---|---|---|---|
| R0-01 Audience and use case | Who is the first operator/user, and are they evaluating a local tool or depending on delivered alerts? | One named audience, intended job-to-be-done, explicitly excluded audience, selected validation path | DECIDED (Path A) |
| R0-02 Exposure and identity | Local loopback only, single-operator private deployment, or a shared invite-only service? Who can view/mutate a Monitor? | Intended network entry point and access/ownership boundary; no UUID-as-authorization assumption | DECIDED (Path A) |
| R0-03 Targets and admission | Supported target protocols and restrictions; maximum active Monitors, check attempts, and expected overload behavior | Test-derived safe cap and representative healthy/slow/timeout-heavy workload; quota/rejection behavior specified before remote beta | DECIDED (Path A) |
| R0-04 Detection and alert expectations | Required failure-detection window, false-alarm policy, outage confirmation/recovery and whether/when operator notification is needed | Measurable latency/window and operator-visible semantics; separate probe facts from incident and delivery outcomes | DECIDED (Path A) |
| R0-05 Operator and support | Who receives alerts about *uptime-lab itself*, responds to incidents and restores service? | Named accountable operator, escalation/contact window and manual failure procedure | DECIDED (Path A) |
| R0-06 Hosting and budget | Region, hosting model, spending ceiling and operational constraints | Approved monthly cost envelope and owner; provider choice deferred until requirements justify it | DECIDED (Path A) |
| R0-07 Data and privacy | Sensitive target URL handling, log exposure, CheckRun/history retention and deletion expectations | Written retention and sensitive-data policy, including what cannot appear in logs or notifications | DECIDED (Path A) |
| R0-08 Recovery and release limits | Acceptable data loss (RPO), recovery time (RTO), backup strategy and explicit maintenance/downtime expectations | Chosen numeric objectives; subsequent isolated restore/rollback rehearsal required before protected beta | DECIDED (Path A) |
| R0-09 Geography and notifications | Single-region limitation, any regional confirmation expectation, and preferred *first* notification channel if beta is selected | Stated limitations and channel choice, plus later design review of credentials, cost and outbound-destination policy | DECIDED (Path A) |

Current implemented policy — 60-second eligibility after latest terminal completion, 10-second probe timeout, and four active probe slots in one logical Checker — **does not imply** a 60-second end-to-end detection guarantee or an approved Monitor count. Do not invent a supported workload, availability SLO, recovery objective, hosting provider, quota or retention period here.

## Recorded Path-A decision values (2026-11-13)

These values are user-approved for single-operator local evaluation only. They are product-scope decisions, not release authorization, capacity guarantees, or production-readiness claims. The "Minimum recorded acceptance evidence" column above still describes what a future Path-B decision must satisfy.

| ID | Recorded Path-A value | Explicit exclusions |
|---|---|---|
| R0-01 Audience and use case | Single operator (the repository owner) evaluating the real local Monitor workflow; job-to-be-done is local uptime-monitoring evaluation and evidence gathering; validation path A | External users, multi-user teams, dependence on delivered alerts |
| R0-02 Exposure and identity | Loopback-only through the restricted local gateway; no remote ingress; no ownership/authz boundary beyond the local gateway; UUIDs are not authorization | Shared invite-only service, remote access, accounts/identity |
| R0-03 Targets and admission | HTTP/HTTPS targets only; four probe slots, 60-second completion-based eligibility and 10-second timeout are local conventions, not an admitted cap or supported Monitor count | Remote admission cap, quotas, overload SLO (revisit for Path B) |
| R0-04 Detection and alert expectations | Local-evaluation detection only; no alert-delivery guarantee; R3-1 assessment-only confirmation/recovery (fixed consecutive-failure and recovery thresholds, exact values a design decision); durable incident state deferred | Notification, uptime percentage, maintenance windows, per-Monitor configurable policy |
| R0-05 Operator and support | Repository owner is the accountable operator; manual local failure procedure per the local-development guide (restart/reset) | External escalation/contact window, on-call |
| R0-06 Hosting and budget | No deployment authorized; local Docker Compose only; no hosting budget or provider | Region/hosting/spending decisions (revisit for Path B) |
| R0-07 Data and privacy | Newest-50 terminal-row retention is a repository-owned local convention; targets stored as complete text; logs sanitized; no sensitive URL or evidence leaves the local system | Beta retention/deletion policy, sensitive-data compliance claims |
| R0-08 Recovery and release limits | No RPO/RTO while local-only; local Compose volume; backup/restore and rollback rehearsal deferred | Numeric recovery objectives, production backup (required before Path B) |
| R0-09 Geography and notifications | Single-region local; no regional confirmation; first notification channel deferred | Channel choice, regional aggregation (revisit for Path B with R4) |

Measurable acceptance for the Path-A scope is the existing local evidence: real-PostgreSQL integration, composed Chromium journeys and exact-main CI for the landed operations. No new numeric SLO, capacity or recovery objective is introduced here.

## R0 exit / next-design gate

**Recorded 2026-11-13:** R0 is **DECIDED for the Path-A local-evaluation scope** — the nine values above are user-approved with the stated exclusions. This records **product scope only**, not a production release authorization. The external monitoring beta remains NO-GO; a Path-B pivot reopens the affected items with full acceptance evidence.

R0 can be proposed as **DECIDED** for a broader scope only after the affected items above have explicit user-approved values, concrete exclusions and measurable acceptance conditions in this document or a linked approved decision record. This records **product scope only**, not a production release authorization.

After that decision, select **one** dependency-bounded next task and write its own scope/design/implementation plan before code. R3 must define incident semantics separately from the existing current availability read; R4 must justify exactly one notification channel and its delivery guarantees; R5 must independently qualify authentication, secure ingress, quotas, backup/restore, rollback, and operators. The order follows the selected validation path rather than presupposing a full SaaS launch.

## Evidence required before any external monitoring beta

- Observed target-failure confirmation, recovery, notification receipt and late/missing execution handling under realistic load
- Access and internal-route isolation, SSRF/egress checks, HTTPS and safe secrets/log handling in the intended environment
- Admitted workload/overload measurements and retention/storage costs, rather than theoretical concurrency arithmetic
- Backup freshness, isolated real restore, staged migration/rollback, internal watchdog and incident response evidence
- Exact release-candidate verification, explicit deployment approval and post-deployment observation

These are **later release gates**, not work items authorized by this R0 discussion. No production migration, deployment, external provider connection, new runtime feature, release tag or branch merge is authorized by this proposal.
