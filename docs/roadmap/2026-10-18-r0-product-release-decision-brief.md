# R0 Product and Release Decision Brief

**Status:** PROPOSED — discussion aid only; no R0 product/release decisions approved
**Prepared:** 2026-10-18
**Verified code baseline:** `main@2e4db4f8a8cbb5b794f68dc52a4e8550b429210b`, [post-merge CI #38974461560](https://github.com/kefyusuf/uptime-lab/actions/runs/38974461560) successful
**Source of phase requirements:** [Industry benchmark and launch roadmap](2026-10-02-industry-benchmark-and-launch-roadmap.md), especially R0 and the release gates

## Purpose and boundary

Record the **minimum product, operator and recovery decisions** needed to choose a subsequent, bounded milestone. This is a decision worksheet, not an ADR, implementation authorization, release plan, accepted capacity guarantee, production readiness certification, or approval to expose the service remotely.

Local development behavior is already demonstrated: Go and Rust exercise real execution/persistence; React offers loopback-only registration, bounded inventory, detail, current availability, raw latest result and persisted Pause/Resume. R0a and R1 are complete for their local scopes; two R2 increments (inventory #56, Pause/Resume #60) have landed. The remaining R2 items, incident decisions (R3), delivery (R4), and protected beta readiness (R5) are not complete.

**Release judgment: NO-GO for a remotely accessible monitoring beta.** There is no approved audience, admitted workload, authentication/ownership boundary, external ingress model, notification service, production backup/restore drill, or operator incident process. Passing local Docker and Chromium CI does not close these gates. Nothing below changes that judgment.

## Candidate validation paths (not selected)

| Candidate | Purpose | Boundary |
|---|---|---|
| A — Single-operator, local-only evaluation | Validate the current real Monitor workflow and collect evidence without adding identities or remote ingress | Loopback-only; no external users, alert-delivery guarantee, public SLO, or production availability claim |
| B — Protected invite-only beta | Offer dependable monitored checks to a deliberately bounded set of operators | Requires selected audience and limits plus R3 confirmed incident semantics, R4 reliable notification, and R5 access, network, security, recovery and operations gates |
| C — Open-registration service | Public signup and multi-user operations | **Not an R0 default.** Requires additional R6 abuse, isolation, support and capacity evidence |

**Recommended order for discussion:** first decide whether A is sufficient for evidence gathering or whether B is genuinely required for the next validation. This is a recommendation, **not** an approved choice. Choosing A cannot be used to describe the product as a protected monitoring beta.

## R0 decisions requiring explicit approval

All entries are **OPEN**. A question or candidate is not an implementation requirement until the user records a chosen value and its rationale.

| ID | Decision owner must settle | Minimum recorded acceptance evidence | Status |
|---|---|---|---|
| R0-01 Audience and use case | Who is the first operator/user, and are they evaluating a local tool or depending on delivered alerts? | One named audience, intended job-to-be-done, explicitly excluded audience, selected validation path | OPEN |
| R0-02 Exposure and identity | Local loopback only, single-operator private deployment, or a shared invite-only service? Who can view/mutate a Monitor? | Intended network entry point and access/ownership boundary; no UUID-as-authorization assumption | OPEN |
| R0-03 Targets and admission | Supported target protocols and restrictions; maximum active Monitors, check attempts, and expected overload behavior | Test-derived safe cap and representative healthy/slow/timeout-heavy workload; quota/rejection behavior specified before remote beta | OPEN |
| R0-04 Detection and alert expectations | Required failure-detection window, false-alarm policy, outage confirmation/recovery and whether/when operator notification is needed | Measurable latency/window and operator-visible semantics; separate probe facts from incident and delivery outcomes | OPEN |
| R0-05 Operator and support | Who receives alerts about *uptime-lab itself*, responds to incidents and restores service? | Named accountable operator, escalation/contact window and manual failure procedure | OPEN |
| R0-06 Hosting and budget | Region, hosting model, spending ceiling and operational constraints | Approved monthly cost envelope and owner; provider choice deferred until requirements justify it | OPEN |
| R0-07 Data and privacy | Sensitive target URL handling, log exposure, CheckRun/history retention and deletion expectations | Written retention and sensitive-data policy, including what cannot appear in logs or notifications | OPEN |
| R0-08 Recovery and release limits | Acceptable data loss (RPO), recovery time (RTO), backup strategy and explicit maintenance/downtime expectations | Chosen numeric objectives; subsequent isolated restore/rollback rehearsal required before protected beta | OPEN |
| R0-09 Geography and notifications | Single-region limitation, any regional confirmation expectation, and preferred *first* notification channel if beta is selected | Stated limitations and channel choice, plus later design review of credentials, cost and outbound-destination policy | OPEN |

Current implemented policy — 60-second eligibility after latest terminal completion, 10-second probe timeout, and four active probe slots in one logical Checker — **does not imply** a 60-second end-to-end detection guarantee or an approved Monitor count. Do not invent a supported workload, availability SLO, recovery objective, hosting provider, quota or retention period here.

## R0 exit / next-design gate

R0 can be proposed as **DECIDED** only after the nine items above have explicit user-approved values, concrete exclusions and measurable acceptance conditions in this document or a linked approved decision record. This records **product scope only**, not a production release authorization.

After that decision, select **one** dependency-bounded next task and write its own scope/design/implementation plan before code. R3 must define incident semantics separately from the existing current availability read; R4 must justify exactly one notification channel and its delivery guarantees; R5 must independently qualify authentication, secure ingress, quotas, backup/restore, rollback, and operators. The order follows the selected validation path rather than presupposing a full SaaS launch.

## Evidence required before any external monitoring beta

- Observed target-failure confirmation, recovery, notification receipt and late/missing execution handling under realistic load
- Access and internal-route isolation, SSRF/egress checks, HTTPS and safe secrets/log handling in the intended environment
- Admitted workload/overload measurements and retention/storage costs, rather than theoretical concurrency arithmetic
- Backup freshness, isolated real restore, staged migration/rollback, internal watchdog and incident response evidence
- Exact release-candidate verification, explicit deployment approval and post-deployment observation

These are **later release gates**, not work items authorized by this R0 discussion. No production migration, deployment, external provider connection, new runtime feature, release tag or branch merge is authorized by this proposal.
