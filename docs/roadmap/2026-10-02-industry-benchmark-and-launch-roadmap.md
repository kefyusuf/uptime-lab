# Industry Benchmark and Launch Roadmap

**Status:** Research-backed proposal; not an approved implementation plan
**Researched:** 2026-10-02
**Repository baseline:** `main@f50eca01fd74b6636113a7c8afd6126f8aeed879` after approved plan #55; inventory implementation is in draft #56 at `1386e98`, verified on 2026-10-05
**Working audience assumption:** invite-only, small-team beta before open registration

## 1. Decision summary

Preserve the existing Go/Rust/PostgreSQL architecture. Build toward a useful, bounded HTTP/HTTPS monitoring product through separate milestones for browser use, Monitor lifecycle/history, monitoring semantics, incident delivery, and operational readiness.

The Current Availability gate is complete: scope [#44](https://github.com/kefyusuf/uptime-lab/pull/44), design [#46](https://github.com/kefyusuf/uptime-lab/pull/46), plan [#47](https://github.com/kefyusuf/uptime-lab/pull/47), and implementation [#48](https://github.com/kefyusuf/uptime-lab/pull/48) have landed. Local Web can now consume Go's current assessment as well as raw terminal evidence. A remote demonstration, a dependable monitoring beta, and an open-registration SaaS still have different acceptance criteria.

R1's local browser capability is complete through #49/#50/#51/#52. The first bounded R2 increment now implements durable Monitor inventory and local detail navigation in [PR #56](https://github.com/kefyusuf/uptime-lab/pull/56), following approved scope #53, design #54 and [TDD plan #55](../superpowers/plans/2026-10-05-local-monitor-inventory.md). [Implementation-head CI37242623843](https://github.com/kefyusuf/uptime-lab/actions/runs/37242623843) passed Go/race/real PostgreSQL, contract and Web checks, canonical Docker smoke and actual browser journeys on default/custom ports. Final documentation-head checks, fresh independent review and merge remain gates. This increment does not complete R2 or resolve R0's release assumptions.

For this proposal, first launch means a protected, invite-only beta that can detect a defined target failure, notify its operator, show the evidence, and recover safely from an application or infrastructure failure. This audience is an explicit assumption awaiting confirmation. A single-owner self-hosted release can reduce account complexity; an open SaaS requires stronger isolation, abuse controls, support, and capacity evidence.

## 2. Research method and limits

Primary sources were read on the research date: official UptimeRobot, Better Stack, and Checkly product/documentation pages; OWASP security guidance; Google SRE guidance; and PostgreSQL documentation. This is a qualitative benchmark of representative products, not an exhaustive market survey, purchasing recommendation, compliance audit, or comparison of paid plan entitlements.

Repository findings come from local code, contracts, migrations, Compose, CI configuration, and canonical documents. The original #48 baseline's [exact-main CI](https://github.com/kefyusuf/uptime-lab/actions/runs/36985739087) passed Go static/unit/race/vulnerability, real PostgreSQL integration, public-contract, real Docker smoke, and repository/documentation checks. The 2026-10-04 refresh updates current Web and continuation state against the baseline recorded above; its narrower main-CI scope is described in the decision summary. This verifies local development capability; it does not verify deployed infrastructure, external account settings, or admitted production capacity. Better Stack confirmation/recovery, Checkly alerting, and OWASP object-authorization sources were reread in the original #48 refresh; the new inventory proposal separately cites the collection/pause sources read on 2026-10-04. Other benchmark observations retain the original research date. A feature documented by a vendor is evidence of that vendor's offering, not a universal industry requirement. The phases and launch gates below are project recommendations inferred from that evidence.

## 3. What mature monitoring products cover

| Capability family | Primary-source observation | Implication for uptime-lab |
|---|---|---|
| Core monitoring and operator workflow | UptimeRobot documents website/API checks, response-time monitoring, incidents, status pages, team access, and maintenance windows | A usable product needs management and communication around probe execution; matching every monitor type is unnecessary |
| Failure confirmation | Better Stack documents checks from multiple locations and confirmation/recovery periods | A failed execution should not automatically become a confirmed target outage |
| Actionable alerting | Checkly documents state transitions, failure thresholds, retries, escalation, recovery, notification channels, and notification logs | Incident state and delivery policy should be explicit and testable |
| Safe public APIs | OWASP describes object authorization and resource-consumption risks | UUIDs and bounded probes do not replace access control, quotas, and aggregate abuse limits |
| Service operation | Google SRE describes user-oriented SLOs and production readiness review | Measure the monitoring product's own reliability and decide who operates it |
| Durable recovery | PostgreSQL documents backup/restore and WAL-based point-in-time recovery | A persistent Compose volume is not sufficient recovery evidence |

The product scope observation uses [UptimeRobot's official feature overview](https://uptimerobot.com/). The confirmation comparison uses [Better Stack locations](https://betterstack.com/docs/uptime/locations-and-regions/) and [confirmation/recovery documentation](https://betterstack.com/docs/uptime/confirmation-and-recovery-period/). The alerting comparison uses [Checkly's official alerting documentation](https://www.checklyhq.com/docs/communicate/alerts/overview/).

These examples justify separating execution facts, availability decisions, incidents, and notification delivery. Their exact retry counts or region thresholds should not be copied without defining this project's users, detection budget, and capacity.

## 4. Repository gap analysis

| Area | Verified local baseline | Proposed next capability | Release significance |
|---|---|---|---|
| Architecture | Go owns product/persistence; Rust owns execution; explicit contracts and narrow ports | Retain ownership; evolve contracts through dedicated designs | Foundation already exists |
| Probe safety | Public-address policy, DNS pinning, redirect revalidation, TLS validation, bounded timeout/headers/concurrency | Review deployed egress, metadata access, destination fan-out, and abuse handling | Required before remote access |
| Monitor management | Immutable create/read and bounded durable inventory in PR #56; no ownership, pause, edit or deletion | Explicit lifecycle with defined in-flight work behavior; history and retention separately | Inventory improves local discovery; useful beta still needs management |
| Result visibility | Latest terminal result; exact failure vocabulary; no public history | Paginated bounded history and visible result age | Required for beta investigation |
| Availability semantics | Current read with fixed HTTP 200–299 success policy, explicit outcome reasons, future/stale precedence, and 120-second freshness | Configurable expected-status policy, confirmation/recovery and maintenance-aware incident semantics | Basic assessment landed; a confirmed outage and uptime percentage remain separate capabilities |
| Incidents/alerts | No incident state or notification delivery | Durable incident transitions and one reliable notification channel | Required for a monitoring beta |
| Frontend | React registration, bounded inventory, local detail navigation, reopen by UUID, current assessment/raw result and manual refresh; restricted loopback gateway | Deliberate management/history design after this inventory increment | Durable browser discovery proven on both ports; consumes Go policy |
| Access boundary | No authentication/authorization; public and internal adapters share one API listener | Identity/access boundary, explicit route exposure, internal caller protection | Required before remote beta |
| Operations | Health checks, sanitized logs, graceful shutdown, explicit migrations | Metrics, independent alerts, backups/restore, runbooks, release/rollback | Required before dependable live use |
| Capacity/data lifecycle | Four active probes in one logical Checker; fixed cadence; no retention | Load envelope, quotas, retention, growth and freshness measurement | Required before beta workload acceptance |
| Delivery governance | PR/CI, immutable migrations, architecture checks and runtime smoke tests | Deployment evidence and production readiness gate | Strong development process; production evidence remains separate |

Local evidence pointers:

- `apps/api/internal/modules/monitoring/application/check_execution.go`: cadence, deadline, probe policy.
- `apps/api/internal/modules/monitoring/adapters/postgres/check_execution.go`: due claims, completion, timeout reconciliation.
- `apps/api/internal/modules/monitoring/adapters/postgres/latest_check_result.go`: terminal-only read.
- `apps/api/internal/modules/monitoring/domain/availability.go`: Go-owned classification and freshness policy.
- `apps/api/internal/modules/monitoring/application/get_monitor_availability.go`: one validated terminal read followed by one clock sample.
- `apps/api/internal/modules/monitoring/adapters/http/availability.go`: closed assessment/evidence response and matched-route no-store behavior.
- `apps/checker/crates/probe-http/`: execution and destination safety.
- `apps/checker/crates/checker-core/src/worker.rs`: bounded worker orchestration.
- `apps/api/migrations/`: existing schema constraints/indexes.
- `compose.yaml` and `compose.web-local.yaml`: compiled Web, canonical unpublished ports and opt-in loopback-only Web exposure.
- `apps/web/` and `docs/testing/local-monitoring-web.md`: implemented local browser/gateway and composed acceptance evidence.
- `docs/backend/go-control-plane.md`, `docs/checker/rust-checker.md`, and `docs/devops/local-development.md`: canonical current-state guidance.

## 5. Important semantic and capacity decisions

### Target outage versus monitoring-system failure

An HTTP response is an execution fact, not necessarily success. The landed current assessment uses HTTP 200–299 as available and other valid statuses as unavailable; a `500` response therefore means `unavailable/unexpected_http_status`. This fixed policy is not configurable. Future completion produces `unknown/future_result`; evidence older than 120 seconds produces `unknown/stale_result`, with exactly 120 seconds fresh. No terminal result is `200 unknown/no_result` with evidence omitted. All matched availability responses use `Cache-Control: no-store`.

Fresh `policy_rejected` is `unknown/policy_rejected`, not target downtime. Fresh `worker_timeout` and `internal_error` are `unknown/execution_failure`. DNS, timeout, connect, TLS and protocol probe failures are `unavailable/probe_failure`. The browser must present these Go-owned decisions and their evaluation/completion timestamps rather than inventing another policy. The raw latest-result read retains its empty `204` for a known Monitor without a terminal result. Separate raw-result and availability requests may legitimately observe different CheckIDs.

The current implementation reconciles expired pending work during a claim or late completion. If the Checker stops claiming, latest-result reads do not reconcile pending rows. API readiness checks database/schema compatibility, while Checker readiness indicates loop startup. Neither proves that checks continue completing. Measure overdue work and result freshness independently; any reconciliation redesign belongs to a separate reviewed slice.

The current read is complete and deliberately excludes confirmation, recovery, maintenance exclusions, incident state, and any uptime percentage. A later incident/availability-history design must define these plus the denominator/window for any advertised uptime metric. Sample-based and elapsed-time availability are different metrics. Missing observations must not silently count as healthy. Notification transport retries are also different from rechecking a target for failure confirmation.

### Capacity and retention

The fixed cadence makes a Monitor eligible 60 seconds after its latest terminal completion, rather than guaranteeing an exact wall-clock check every minute. Actual detection delay includes scheduling, execution, persistence, and confirmation.

With four concurrent probes, an illustrative all-10-second workload has a theoretical probe service rate of 24 completions/minute before claim, delivery, database, and operational overhead. This is arithmetic, not a benchmark or a supported Monitor limit. Admission limits must come from measured sustained load, bursts, timeout-heavy targets, and recovery behavior with headroom.

For storage planning, a nominal once-per-minute schedule produces roughly 1,440 records per Monitor/day; 1,000 such Monitors would imply roughly 1.44 million records/day. The current completion-based cadence differs. Measure row/index size, query latency, backup growth, and retention costs before selecting retention or introducing partitioning.

Do not introduce multi-worker leases, brokers, Kubernetes, or partitioning as an automatic reaction to vendor feature breadth. They need concrete load or reliability evidence. Multiple public regions remain a later design because the current contract has no worker/region identity or regional aggregation semantics.

## 6. Proposed roadmap and acceptance gates

Phases represent capability dependencies, not calendar promises. Each phase needs its own reviewed design and implementation plan. Discovery for deployment and observability starts early even when its implementation lands later.

| Phase | Deliverable | Exit evidence |
|---|---|---|
| R0 — Product and release brief | Confirm audience, supported targets, workload cap, detection expectations, data policy, operator, hosting constraints, and budget; agree proposed beta scope | Written decisions; measurable acceptance criteria; no unspecified public exposure |
| R0a — Current availability read — complete | Go-owned current assessment landed through #44/#46/#47/#48, without new persistence or history/incidents/lifecycle | Contract 71 cases; domain/application/HTTP TDD; real PostgreSQL and Docker evidence; independent review fixes; exact-main CI green at b4f0ae5 |
| R1 — Local Web vertical slice — complete | React registration/detail/current-availability/raw-result on a loopback access path; explicit empty/error/unknown/stale presentation | #49/#50/#51/#52; reviewed implementation; real browser journey on4173/4817; four-operation gateway allowlist; internal routes inaccessible through Web; deterministic runtime smoke and inbound deadline regression coverage |
| R2 — Monitor management and evidence — first increment implemented, review pending | PR #56 adds bounded durable inventory/detail navigation; later slices must separately design pause/resume, edit/delete, bounded CheckRun history and retention | Inventory tuple pagination, response-byte bounds and composed browser evidence pass; final review/merge pending; later mutations need lifecycle/in-flight race proof and retention semantics |
| R3 — Monitoring decisions | Expected-status policy, confirmation/recovery, stale/unknown handling; maintenance semantics; durable incident open/resolve | Deterministic state-transition and replay evidence; Checker outage does not masquerade as healthy target or confirmed target outage |
| R4 — Reliable notifications | One channel, incident/recovery delivery, deduplication, bounded retry, failure visibility and manual retry policy | Provider outage/restart evidence; durable delivery tracking; acknowledged duplicates policy; no repeated probe on notification retry |
| R5 — Protected beta deployment | Identity/access model, ownership when accounts are separate, quotas, HTTPS ingress, secrets, environment isolation, backup/restore, metrics, runbooks, controlled release | Production readiness checklist below; staging fault/load/security evidence; restricted invitations |
| R6 — Broader public release | Open-registration abuse defenses, measured capacity, user data lifecycle, support process, selected public status-page scope; revisit regions | Beta reliability/cost evidence; no unresolved release blockers; public offering matches actual guarantees |
| R7 — Evidence-driven expansion | Multi-region/checker failover, TLS-expiry/heartbeat/DNS/TCP or browser checks, richer integrations/team roles | Concrete user need; separate contract, security, operational and cost assessment |

R0a is narrower than R3: a current availability read is not an incident confirmation, configurable expected-status policy, or notification system. R3 must explicitly reassess the landed semantics rather than invent incompatible duplicate state. R2 and R3 may be split into smaller PRs and design milestones. Maintenance is required before advertising maintenance-aware availability; it can be excluded from an initial beta only with an explicit product limitation and no misleading uptime report.

R4 should first assess whether a PostgreSQL-backed durable delivery queue fits the workload. A transactional outbox is a candidate if incident-to-notification atomicity requires it; no broker is selected here. Any webhook channel introduces another user-controlled outbound destination and needs its own safety policy.

R5 discovery starts in R0. Authentication must precede connecting shared users to a deployment; it is not a post-launch add-on. R1 may stay entirely local while later access/ownership semantics are designed. A read-only demonstration can launch earlier under a separate restricted scope but is not advertised as dependable monitoring.

## 7. Production readiness checklist

These are project-specific proposed launch gates, not a claim that every competitor uses the same implementation.

| Gate | Evidence required before invite-only monitoring beta |
|---|---|
| Access and isolation | Intended users authenticated; account/object permissions tested where applicable; no UUID-as-permission assumption; `/internal/checks/*` blocked at public ingress and protected from unintended internal callers |
| Abuse and resource control | Per-user/global Monitor and request quotas; bounded payloads, concurrent work, target fan-out and notifications; timeout-heavy load tests; actionable limit errors |
| Probe/network safety | Existing SSRF regression suite plus deployed egress/metadata checks; no production private-network bypass; secrets and sensitive target data kept out of logs |
| Browser/transport security | HTTPS and certificate renewal; session/token handling, CSRF/origin policy, explicit proxy routes, security headers, sanitized errors; CORS only when the chosen origin topology requires it |
| Data protection | Production secrets separated from local defaults; least-privilege DB/runtime access; automated off-host backup; restore drill to an isolated environment; agreed recovery point/time objectives and retention/deletion behavior |
| Deploy/migrate/rollback | Immutable release artifacts; staging promotion; explicit serialized migrations; compatibility between schema and application versions; readiness verification; rollback rehearsal; no automatic destructive database downgrade |
| Monitoring the monitor | API error/latency signals, scheduling lag, overdue work, result age, Checker restarts, delivery lag/errors, DB saturation/storage, backup freshness; an external watchdog that does not rely exclusively on this service |
| Operational ownership | Named operator, escalation path, incident and recovery runbooks, restore procedure, dependency update policy, release notes and change visibility |
| Product reliability | Measured failure-to-notification latency, recovery behavior, stale/unknown display, confirmation/deduplication, known limitations and bounded target support |
| Capacity/cost | Demonstrated admitted load with headroom; quotas based on evidence; retention/storage and notification cost bounds; defined overload behavior |

Object-level authorization follows [OWASP API1](https://api-security.owasp.org/editions/2023/en/0xa1-broken-object-level-authorization/). Aggregate quotas and resource bounds are motivated by [OWASP API4](https://api-security.owasp.org/editions/2023/en/0xa4-unrestricted-resource-consumption/). Probe/network review is grounded in the [OWASP SSRF guidance](https://cheatsheetseries.owasp.org/cheatsheets/Server_Side_Request_Forgery_Prevention_Cheat_Sheet.html); this document does not certify the implementation against that guidance.

For data recovery, select a backup method against agreed recovery needs. WAL archiving/PITR is a candidate when restoring to a recent point is required; a properly scheduled logical backup may suffice for a smaller declared recovery envelope. Test restoration rather than merely checking that a backup job ran. See [PostgreSQL backup/restore](https://www.postgresql.org/docs/current/backup.html) and [PITR](https://www.postgresql.org/docs/current/continuous-archiving.html).

## 8. Deployment sequence

1. Select hosting region/platform against budget, operator skills, workload and recovery requirements; no provider is chosen in this roadmap.
2. Prepare staging with the intended ingress, network isolation, secrets, database and backup strategy. Keep development and production data/credentials separate.
3. Run end-to-end registration, execution, confirmation, notification and recovery against controlled test targets. Include Checker loss, API/DB restart, notification outage, quota exhaustion, and restore drills.
4. Verify the exact candidate release, schema compatibility, external ingress restrictions, deployment rollback, and independent alert delivery.
5. Deploy a capped invite-only beta; verify live signals and real notification receipt. Observe against a declared evaluation window and workload rather than opening registration immediately.
6. Expand only after reliability, cost, user isolation and support evidence supports the next audience.

The API currently requires an exact repository-owned migration set for readiness. A rolling deployment across schema versions may therefore leave old instances unready. The deployment design must choose a controlled maintenance deployment or an explicitly redesigned compatibility strategy before claiming zero-downtime upgrades.

Docker Compose is a valid candidate for a bounded single-host beta if its failure/recovery limitations are explicit and tested. Managed services are also candidates. Neither Compose nor Kubernetes is by itself evidence of production readiness.

## 9. Reliability measures and open decisions

Separate the reliability of monitored targets from the reliability of uptime-lab. Proposed internal indicators are API availability/latency, check scheduling lag, terminal-result freshness, confirmed-failure-to-notification latency, delivery success, and restore time/data loss. Define numeric targets and measurement windows after establishing audience/load; do not invent a 99.9% promise from a passing smoke test.

This approach follows [Google SRE's user-centric SLO guidance](https://sre.google/workbook/implementing-slos/). A release review covering operational responsibility and reliability needs is informed by [Google's production readiness model](https://sre.google/sre-book/evolving-sre-engagement-model/); a small project need not reproduce Google's organizational process.

Decisions still needed:

- Single-owner self-hosted, invite-only shared team, or open-registration service?
- Supported target count and detection/notification delay expectation?
- Hosting budget, region and who responds when uptime-lab fails?
- First notification channel and handling of sensitive target URLs?
- History retention and acceptable recovery point/time?
- Single-region limitation for beta, or an explicit requirement for regional confirmation before launch?

R0a and R1 are complete for their stated local scopes. Inventory scope #53, design #54 and plan #55 are approved and merged; Native execution produced PR #56. The accepted first-oversized-item500 limitation blocks traversal at that item without skipping or truncating it. The immediate gate is final exact-head CI, fresh whole-branch review and user approval to merge the implementation. The inventory source refresh covers Better Stack collection pagination and pause semantics read on 2026-10-04; the broader benchmark retains its 2026-10-02 research date. Existing availability/detail behavior and local access constraints remain acceptance requirements. R0's beta audience, workload, operator, budget and recovery decisions remain unresolved. This roadmap update records the approved local increment and does not authorize deployment or external-service changes.
