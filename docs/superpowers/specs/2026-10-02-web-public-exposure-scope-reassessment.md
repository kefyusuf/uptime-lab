# Web and Public Exposure Scope Reassessment

**Status:** Review candidate; scope recommendation only
**Date:** 2026-10-02
**Repository:** `kefyusuf/uptime-lab`
**Base:** `main@b4f0ae5e1372b77398013b94da37c0c8a6ecdf30`
**Purpose:** Select the next milestone after Current Monitor Availability Read, before detailed design or implementation.

**Research follow-up:** [Industry benchmark and launch roadmap](../../roadmap/2026-10-02-industry-benchmark-and-launch-roadmap.md) compares representative monitoring products, security/operations guidance, and local implementation evidence. Its proposed release gates extend this local Web scope: a browser slice is an integration milestone, not sufficient evidence for a dependable live monitoring service. Audience and release scope remain proposals awaiting review.

**Sequencing refresh:** Current Availability scope #44, design #46, plan #47 and implementation [#48](https://github.com/kefyusuf/uptime-lab/pull/48) are merged. The [exact-main CI](https://github.com/kefyusuf/uptime-lab/actions/runs/36985739087) is green at the base above. This local Web candidate is now the recommended next scope; its own scope, written design and plan still require review before implementation.

## 1. Intent and assumptions

The foundation describes a learning-oriented, production-disciplined monitoring application using React/TypeScript, Go, and Rust. The next milestone should make the implemented monitoring loop usable from a browser while preserving runtime ownership and contract discipline.

The user requested continuation from the current repository state. Local contributor use is the recommended initial audience, but it has not yet been explicitly confirmed. Internet-facing use, shared users, and deployment requirements remain open product decisions.

Success for the proposed scope means a contributor can register a target, open its Monitor, and observe Go's current availability assessment and the latest terminal execution fact through a local browser without container-local manual API calls.

## 2. Current implementation truth

The latest landed baseline is `b4f0ae5` (`feat(api): derive current monitor availability on read (#48)`). Local and remote main match, and the working tree was clean before this documentation refresh. Source, contracts, Compose, current documents and exact-main CI were inspected. That CI verifies Go/race, real PostgreSQL, public contract and real Docker smoke; it proves the development capability, not a deployed product or browser journey.

The implemented public contract contains:

~~~text
POST /monitors
GET  /monitors/{monitorId}
GET  /monitors/{monitorId}/latest-result
GET  /monitors/{monitorId}/availability
~~~

Go owns registration, scheduling, completion semantics, PostgreSQL persistence, migrations, and the public read model. Rust owns bounded HTTP/HTTPS execution and execution-time destination validation. PostgreSQL contains `monitoring.monitors` and `monitoring.check_runs`.

The latest-result read exposes execution facts only. Pending work is invisible; `204` means a known Monitor has no terminal result; `worker_timeout` omits duration and HTTP status. The separate availability read returns status/reason/evaluatedAt and terminal evidence when present. Its fixed Go policy treats fresh HTTP 200–299 as available, other HTTP statuses and probe failures as unavailable, policy/execution failures as unknown, future completion as unknown/future_result, and age greater than 120 seconds as unknown/stale_result. Exactly 120 seconds is fresh. No terminal result is 200 unknown/no_result without evidence. Matched responses use no-store.

Compose contains four services: Web placeholder, Go API, Rust Checker, and PostgreSQL. Application host ports are not published. React, authentication/authorization, public deployment, Monitor listing, history, and mutable lifecycle remain deferred.

The prior module-boundaries omission is corrected on main: both GetLatestCheckResult and GetMonitorAvailability, plus the pure policy and four-operation public surface, are documented. No separate cleanup of that old gap is needed.

## 3. Alternatives

| Option | Value | Cost and boundary impact |
|---|---|---|
| Local Web vertical slice — recommended | Makes existing registration, current assessment and raw-result behavior directly usable in a browser | Introduces React runtime, browser integration, and a carefully bounded local access path |
| Internet-facing Web release | Allows remote users to access the application | Requires audience, authentication/authorization, abuse controls, ingress/TLS, deployment, and operations decisions before exposure |
| Management/history/incident slice first | Adds investigation and operational workflows beyond the landed current assessment | Requires new contracts and lifecycle/incident/data policy; browser use remains deferred |

The local Web slice is recommended because the existing public contract already supports one complete, small user journey. It provides browser integration evidence before taking on a public-service security and operations boundary.

## 4. Recommended milestone: Local Web Monitoring Vertical Slice

### User journey

1. Open the application through an explicitly configured local browser endpoint.
2. Enter an HTTP/HTTPS target URL and create a Monitor through the existing public contract.
3. Navigate to a shareable-in-the-local-application Monitor detail URL containing its ID.
4. Read the immutable Monitor, Go-owned current availability, and its latest terminal execution fact.
5. Refresh these reads without recreating the Monitor or triggering an execution from the browser.
6. Reopen a known Monitor detail URL after a browser restart.

A Monitor list is not required for this first journey. A detail URL containing the returned ID supplies retrieval without inventing a browser-owned inventory or extending the Go contract.

### Presentation semantics

The UI must distinguish loading, successful Monitor creation, invalid input, missing Monitor, transport/server failure, no terminal result, all availability statuses/reasons, and a terminal execution fact.

Display the target URL, Go's assessment/reason/evaluation time and evidence, plus raw result kind/completion time and optional duration/HTTP status. Label the raw-result `204` as no completed result yet; present availability's `200 unknown/no_result` without fabricating evidence. Explain stale/future/policy/execution reasons rather than displaying all unknown outcomes as target downtime. Do not reimplement availability from resultKind, HTTP status, or a browser-owned freshness threshold. The written design must decide how an old on-screen assessment is labelled between manual refreshes without presenting it as a fresh server decision.

Raw-result and availability requests have separate snapshots and may observe different CheckIDs. Their evidence must remain separately identified; the UI must not combine raw status/duration from one CheckRun into another assessment or treat an ID mismatch as a broken server contract. Read failure must not become a healthy/available badge, and a previous assessment must not be presented as a newly successful read.

Manual refresh is sufficient for the scope recommendation. Any automatic polling policy must be explicitly bounded in the detailed design, including cancellation and error behavior.

### Ownership and integration constraints

- React owns presentation and browser interaction.
- Go remains the only owner of product semantics and durable state.
- The browser consumes only the four existing public operations; availability policy remains in Go.
- The browser never calls Checker claim/result endpoints or accesses PostgreSQL.
- Preserve the committed frontend dependency direction: `app -> pages -> widgets -> features -> entities -> shared`. Create layers only when they contain a concrete responsibility.
- Preserve Go/Rust execution behavior, production destination policy, and explicit migration workflow.
- Preserve the four logical Compose services; replace the Web placeholder only through the reviewed implementation design.

### Local access boundary

The detailed design must select the local browser endpoint and same-origin integration strategy. A candidate is a loopback-bound Web endpoint with an explicit allowlist of the existing public API routes proxied to Go. This is a proposal, not a selected server or proxy implementation.

If this approach is selected, it must reject internal execution routes and avoid blanket proxying of API paths. Go API, Checker, and PostgreSQL must remain unpublished on the host. A loopback endpoint does not provide authentication; remote access, shared machines, browser-origin abuse protections, and proxy behavior require explicit treatment in the design.

The design must decide whether local access belongs in canonical Compose or an opt-in development override. Existing smoke assumptions and the `http://web/` destination-policy rejection scenario must be reassessed before replacing the placeholder.

## 5. Excluded capabilities

This proposed milestone excludes internet-facing deployment, authentication/authorization, multi-user ownership, Monitor listing, full CheckRun history, new or configurable availability policy, incidents/notifications, mutable Monitor lifecycle, configurable cadence, private-network monitoring, and multi-checker coordination. Reading the already implemented Go assessment is included.

A requirement for any of these capabilities reopens scope rather than silently expanding the first Web slice.

## 6. Evidence required by the later implementation plan

- Frontend import-boundary and type/build checks.
- Contract-shaped browser client behavior for Monitor creation/read, the three latest-result variants, and every availability status/reason/evidence variant.
- UI evidence for raw-result `204`, availability `unknown/no_result`, stale/future/policy/execution reasons, missing Monitor, invalid input, failed reads and separate-request CheckID differences.
- Browser acceptance of create -> detail -> availability/raw-result -> refresh -> reopen detail against real Go/PostgreSQL composition.
- Deterministic Go/Rust execution evidence, retaining the production destination policy and avoiding dependence on an external website for acceptance.
- Local integration evidence that public operations are reachable through the selected browser endpoint and internal claim/result routes are unreachable through it.
- Compose/startup/readiness checks and current-state documentation updates appropriate to the chosen access strategy.

Exact test tools, runtime dependencies, server/proxy configuration, and CI changes belong in the written design and implementation plan. The green availability baseline is existing evidence; no passing browser test or Web runtime CI is claimed for this proposed capability.

## 7. Next decision and sequencing

Confirm the intended audience and approve or revise this updated scope. Current Availability is complete. Prepare a dedicated Local Web Monitoring Vertical Slice design only after scope review, including local access, frontend organization, browser client semantics, component/request lifecycle, verification, and smoke compatibility. This approval does not select the audience, hosting or safeguards for a later remote beta.

~~~text
Latest Check Result Public Read — implemented
  -> Current Availability scope/design/plan/implementation — complete #44/#46/#47/#48
  -> Web/public-exposure scope reassessment — this candidate
  -> Local Web Monitoring Vertical Slice design — after scope review
  -> reviewed implementation plan
  -> implementation and verification
  -> separate internet-facing deployment reassessment when needed
~~~

This document authorizes no runtime, OpenAPI, SQL migration, dependency, Compose, or CI implementation. It is a reviewable scope proposal and does not claim Web approval, implementation, or remote deployment.
