# Current Monitor Availability Read Slice Design

**Status:** Review candidate; implementation not authorized
**Date:** 2026-10-02
**Repository:** `kefyusuf/uptime-lab`
**Base:** `main@64217a8d266b85c3979fe01f8288dfd631e0f836`
**Scope gate:** [Current Availability scope reassessment](2026-10-02-current-availability-scope-reassessment.md), landed in PR #44
**Decision state:** The user approved preparing this written specification from the proposed design direction. This written artifact requires review before implementation planning.

## 1. Purpose and agreed direction

Expose one Go-owned current availability assessment for a known Monitor using its latest terminal execution fact and an explicit time policy. The consumer is a public API caller today and a future Web client; presentation must not invent this policy.

The agreed direction is a dedicated `GET /monitors/{monitorId}/availability` read, `available/unavailable/unknown` vocabulary, fixed successful HTTP response policy, 120-second freshness, and no new persistence or Rust behavior.

Success means the caller can distinguish a recently successful check, a recently unsuccessful observation, and insufficient current evidence. This is a single-observation assessment from this Checker's perspective, not a confirmed incident, global reachability guarantee, uptime percentage, or SLA.

The roadmap research in open PR #45 is complementary. This design depends on the landed #44 scope gate, not on merging #45 or implementing its future phases.

## 2. Alternatives and chosen boundaries

| Approach | Trade-off | Decision |
|---|---|---|
| Dedicated availability subresource, derived on read | Adds one operation; preserves existing Monitor and raw-result compatibility | Selected |
| Add availability to Monitor or latest-result payloads | Couples changing time policy to immutable registration or raw execution facts | Rejected for this slice |
| Persist a current-state projection | Adds synchronization and schema obligations without a demonstrated need | Deferred |

Successful application response is selected over merely receiving any HTTP status. A fresh `500`, `401`, or `404` therefore does not count as available. Authenticated targets that normally return `401` are not configurable exceptions in this milestone.

Success means the observed HTTP status meets this rule. The Checker does not application-consume response bodies, so availability does not assert body content, successful page rendering, a complete download, or a successful multi-step user journey.

Rust already follows supported redirect statuses with a Location header within its existing safety budget. Go evaluates the status actually persisted after that execution. A residual `3xx` is not successful under this fixed policy. Any persisted non-2xx HTTP status, including 1xx, is `unavailable`; no protocol behavior is silently changed to improve this classification.

## 3. Current implementation and invariants

The existing public surface is Monitor registration/read and latest terminal result read. Internal claim/result operations, PostgreSQL schemas, migrations, probe safety, cadence, worker limits, and readiness remain unchanged.

The existing `LatestCheckResultRepository` distinguishes missing Monitor, known Monitor without terminal evidence, and one latest terminal result. Its SQL orders by `completed_at DESC, issued_at DESC, id DESC` and excludes pending rows.

The availability use case reuses that bounded persistence capability and validated terminal shapes. A newer pending run never replaces the previous terminal evidence. GET never claims work, reconciles expired runs, modifies rows, or scans public history.

## 4. Product policy

### Vocabulary

- `available`: fresh terminal evidence satisfies the fixed successful-response rule.
- `unavailable`: fresh terminal evidence fails that rule or records a probe/network failure.
- `unknown`: current evidence is absent, stale, temporally inconsistent, blocked by destination policy, or produced by a monitoring execution failure.

`unavailable` describes the observation under this fixed policy. DNS, connectivity, TLS, or protocol failures may reflect the observation path rather than an outage at the target itself. No incident confirmation or causal diagnosis is claimed.

### Evaluation precedence

1. Validate the Monitor ID and perform one bounded repository read.
2. A missing Monitor is `404`, not an availability object.
3. Persistence failure or malformed durable terminal shape is sanitized `500`, not product `unknown`.
4. Sample the injected Go clock once after successful repository resolution and terminal validation. Normalize the instant to UTC; production uses the existing microsecond precision.
5. A known Monitor without terminal evidence is `unknown/no_result`.
6. Terminal completion after the evaluation instant is `unknown/future_result`.
7. Terminal evidence older than 120 seconds is `unknown/stale_result`.
8. Otherwise apply the complete mapping below.

The clock must return a nonzero timestamp representable by the existing JSON timestamp convention. An invalid injected clock is an internal failure. A clock sampled after the read avoids treating a completion committed during the read as future evidence solely because evaluation started earlier.

Before returning evidence, the availability use case must also ensure its UTC completion timestamp is JSON-representable. Keep that availability-specific serialization safeguard from changing the existing latest-result contract or selecting product status inside the HTTP writer.

### Complete fresh-result mapping

| Terminal fact | Status | Reason |
|---|---|---|
| `http_response`, status 200 through 299 inclusive | `available` | `successful_response` |
| `http_response`, status 100 through 199 or 300 through 599 | `unavailable` | `unexpected_http_status` |
| `dns_error` | `unavailable` | `probe_failure` |
| `timeout` | `unavailable` | `probe_failure` |
| `connect_error` | `unavailable` | `probe_failure` |
| `tls_error` | `unavailable` | `probe_failure` |
| `protocol_error` | `unavailable` | `probe_failure` |
| `policy_rejected` | `unknown` | `policy_rejected` |
| `internal_error` | `unknown` | `execution_failure` |
| `worker_timeout` | `unknown` | `execution_failure` |

Unknown or invalid result kinds must fail validation rather than acquire a default product mapping. The existing latest-result validator remains responsible for optional status/duration shape, valid CheckID, completion time, and normalized result bounds.

### Freshness

~~~text
age = evaluatedAt - completedAt
fresh: 0 <= age <= 120 seconds
stale: age > 120 seconds
future evidence: age < 0
~~~

The boundary at exactly 120 seconds is fresh. The first representable instant after it is stale. Never round age to whole seconds before comparison. Test duration arithmetic at nanosecond precision even though production timestamps are microsecond-normalized.

The 120-second window is an explicit first product policy. It accommodates the current completion-based 60-second cadence, 20-second deadline window, and some scheduling/delivery delay; it is not a promise that every check finishes on time or a proven production SLO. If cadence/deadline policy changes later, reassess freshness with it.

Freshness takes precedence over result classification: stale success, stale probe failure, and stale policy/execution failure all return `unknown/stale_result`. A fresh monitoring failure does not fall back to an older successful result.

### Stopped Checker behavior

Expired pending work is currently reconciled by claim or late completion, not by GET. If the Checker stops, an old terminal result eventually becomes stale without any read-side mutation. With no terminal result, availability remains `unknown/no_result` regardless of pending work. This route does not replace operational overdue-work monitoring or redesign reconciliation.

## 5. Public representation and HTTP semantics

Add exactly one operation:

~~~text
GET /monitors/{monitorId}/availability
operationId: getMonitorAvailability
~~~

An existing Monitor always receives `200` for a valid assessment, including `unknown/no_result`. Preserve the existing latest-result `204` behavior without modification.

The response is a closed JSON object with exactly these fields:

| Field | Requirement | Meaning |
|---|---|---|
| `status` | Required | `available`, `unavailable`, or `unknown` |
| `reason` | Required | One of the eight reasons in this design |
| `evaluatedAt` | Required | Go-owned UTC evaluation timestamp |
| `evidence` | Required if a terminal result exists; omitted for `no_result` | Closed object containing exactly `checkId` and `completedAt` |

The eight reasons are `successful_response`, `unexpected_http_status`, `probe_failure`, `no_result`, `future_result`, `stale_result`, `policy_rejected`, and `execution_failure`. OpenAPI must constrain allowed status/reason combinations, not merely declare independent enums. `no_result` permits no evidence; every other assessment requires evidence. Optional fields are omitted rather than serialized as null.

Evidence binds the assessment to a durable CheckRun. It does not repeat target URL, HTTP status, duration, or raw result kind; callers can read execution facts through the existing latest-result endpoint. Separate reads may observe different CheckIDs if completion occurs between requests, so consumers must not assume cross-request snapshot consistency.

Example fresh response:

~~~json
{
  "status": "available",
  "reason": "successful_response",
  "evaluatedAt": "2026-10-02T09:00:30Z",
  "evidence": {
    "checkId": "018f22d3-1d6a-7cc0-a37b-46fc3fafdcb3",
    "completedAt": "2026-10-02T09:00:00Z"
  }
}
~~~

Example without terminal evidence:

~~~json
{
  "status": "unknown",
  "reason": "no_result",
  "evaluatedAt": "2026-10-02T09:00:30Z"
}
~~~

HTTP outcomes:

| Case | Response |
|---|---|
| Known Monitor, valid assessment | `200 application/json` |
| Malformed UUID | `400 application/problem+json` |
| Missing Monitor | `404 application/problem+json` |
| Persistence, invalid durable evidence, or invalid clock | `500 application/problem+json` |
| Unsupported method, including HEAD | `405`, `Allow: GET` |
| Unknown/malformed route shape or extra segment | Existing not-found behavior |

The availability adapter adds `Cache-Control: no-store` to its matched-route responses, including errors. No cache, ETag, conditional GET, or UI polling policy is introduced. A returned object is an assessment at `evaluatedAt`, not a continuously updated state.

The existing Monitor schemas, three latest-result variants, Problem representation, public API version value, and internal contract remain unchanged. Public OpenAPI should contain exactly four paths after later implementation; no security/servers/deployment claims are added by this slice.

## 6. Go ownership and composition

The Monitoring domain owns the fixed status/reason vocabulary and pure in-memory classification/freshness policy. It receives explicit validated evidence and an explicit evaluation instant; it never reads the system clock, database, HTTP request, or configuration.

The Monitoring application adds `GetMonitorAvailability`. It validates inputs, reads `LatestCheckResultRepository` once, validates terminal evidence using the existing application validation boundary, samples the existing injected `Clock` once, invokes domain policy, and returns an invariant-preserving read model.

Reuse the existing latest-result persistence port and SQL. No new repository hierarchy, read port, table, index, transaction, migration, or materialized state is needed. Share existing validation inside the Monitoring application where appropriate; do not widen ports or change latest-result behavior to accommodate the new route.

The public HTTP adapter maps the new application capability and owns exact JSON/header/error mapping. Status selection and clock policy must not move into the handler.

`Module` and `cmd/api` wire the new use case with the existing repository, Clock, pool, and public handler. ExecutionModule, platform server, readiness, internal Checker adapter, and Rust composition remain unchanged.

## 7. TDD and verification requirements for the later plan

This specification selects verification obligations; it is not an executable implementation plan. Each behavior change follows RED -> GREEN -> refactor, with a meaningful failing assertion demonstrated before implementing that behavior. Documentation-only changes use document fitness checks.

| Layer | Required behavior evidence |
|---|---|
| Contract | Exact fourth path and GET operation; 200/400/404/500 outcomes; cache header; closed payloads; status/reason/evidence combinations; existing schema compatibility |
| Domain | Every terminal kind; all HTTP status boundaries; no-result policy; fresh/stale/future precedence; exactly-120-second and fractional-second boundaries |
| Application | Missing Monitor, no terminal result, persistence/cancellation failure, invalid durable shapes, invalid clock; one repository read; one clock sample on valid resolutions; UTC instants |
| HTTP adapter | Exact JSON field sets and omitted evidence; UUID/method/path negatives; stable sanitized errors; no-store; existing Monitor/latest-result behavior |
| PostgreSQL | Existing bounded query and no-mutation evidence retained; previous terminal/newer pending scenario; deterministic latest ordering |
| Production composition | Real public route using the existing pool/repository; no-result unknown, HTTP success/failure, worker-timeout unknown, and deterministic stale behavior where the existing harness can inject time |
| Docker | Extend current deterministic smoke to prove policy-rejected execution maps to unknown availability with matching CheckID; preserve restart/reset and raw latest-result assertions |
| Whole branch | Relevant contract, Go unit/race/real-PostgreSQL, architecture, documentation, local-dev/Docker, vulnerability, and exact-head CI evidence |

Do not rely on sleeps to cross a freshness boundary. Inject time in unit/application tests. Use existing composition test seams or controlled durable fixture timestamps for integration evidence; do not add a production clock override just for tests.

The Docker smoke uses the existing `http://web/` destination-policy rejection. It must assert `status=unknown`, `reason=policy_rejected`, and evidence matching the persisted latest-result CheckID while the evidence is fresh. Bounded smoke waiting must fail clearly if evidence becomes stale instead of treating any unknown result as success. No internet dependency, policy bypass, new service, or host application port is introduced.

## 8. Compatibility, security, and non-goals

This is an additive contract change with one operation. Latest-result remains an execution fact; availability becomes a separate product assessment. There is no new target input, probe operation, or network reachability.

Public ingress/authentication, CORS/rate limiting, React, Monitor lifecycle/list, history/retention, configurable expected statuses/cadence, incidents/confirmation, notification delivery, uptime percentages, multi-region/multi-checker, brokers/outbox, and private-network monitoring remain deferred.

The route is still unauthenticated within the current container-local development topology. This specification does not authorize publishing the API or claim production readiness. Reason values and evidence are closed, sanitized product data; raw errors and internal coordination details remain private.

Current-state docs must be updated only during the later implementation after behavior exists, including stale module-boundaries summaries where applicable. This design PR contains only this specification and does not rewrite current-state docs to advertise an unimplemented endpoint.

## 9. Review and execution gates

Before accepting this design, explicitly review:

1. Single-observation application-success meaning; all non-2xx responses are unsuccessful.
2. Unknown monitoring/policy failures and every failure mapping.
3. Fixed 120-second window, inclusive boundary, and future-evidence handling.
4. Dedicated route, closed status/reason combinations, evidence shape, and no-result 200.
5. Domain/application ownership, one-read reuse, and no persistence change.
6. Meaningful TDD and real composition/Docker evidence without sleeps or network-policy relaxation.

After the user reviews this written spec, prepare a separate implementation plan. Review that plan and select execution method before product implementation. A conversation-level design approval does not authorize an unwritten plan or code.

Stop and reopen design if implementation needs Rust/internal-contract changes, migrations/materialized state, GET-side reconciliation, pending exposure, host ports, auth/public exposure, history, lifecycle, incident confirmation, or configurable product policy. Do not bypass branch/PR/CI gates.
