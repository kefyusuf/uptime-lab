# Web and Public Exposure Scope Reassessment

**Status:** Review candidate; scope recommendation only
**Date:** 2026-10-02
**Repository:** `kefyusuf/uptime-lab`
**Base:** `main@6669aecaca4235c61e9367c6a518a163a3dcf276`
**Purpose:** Select the next milestone after Latest Check Result Public Read, before detailed design or implementation.

**Research follow-up:** [Industry benchmark and launch roadmap](../../roadmap/2026-10-02-industry-benchmark-and-launch-roadmap.md) compares representative monitoring products, security/operations guidance, and local implementation evidence. Its proposed release gates extend this local Web scope: a browser slice is an integration milestone, not sufficient evidence for a dependable live monitoring service. Audience and release scope remain proposals awaiting review.

**Sequencing correction after remote discovery:** The open [scope PR #44](https://github.com/kefyusuf/uptime-lab/pull/44) proposes Current Monitor Availability Read before Web. This local Web candidate is a later milestone and does not replace or authorize bypassing that gate. If #44 lands, its dedicated design and implementation plan come first.

## 1. Intent and assumptions

The foundation describes a learning-oriented, production-disciplined monitoring application using React/TypeScript, Go, and Rust. The next milestone should make the implemented monitoring loop usable from a browser while preserving runtime ownership and contract discipline.

The user requested continuation from the current repository state. Local contributor use is the recommended initial audience, but it has not yet been explicitly confirmed. Internet-facing use, shared users, and deployment requirements remain open product decisions.

Success for the proposed scope means a contributor can register a target, open its Monitor, and observe its latest terminal execution result through a local browser without container-local manual API calls.

## 2. Current implementation truth

The latest landed local commit is `6669aec` (`feat(api): expose latest check result (#43)`). The working tree was clean at assessment time. This assessment inspected source, contracts, Compose, and documentation; it did not execute tests or verify remote CI or branch freshness.

The implemented public contract contains:

~~~text
POST /monitors
GET  /monitors/{monitorId}
GET  /monitors/{monitorId}/latest-result
~~~

Go owns registration, scheduling, completion semantics, PostgreSQL persistence, migrations, and the public read model. Rust owns bounded HTTP/HTTPS execution and execution-time destination validation. PostgreSQL contains `monitoring.monitors` and `monitoring.check_runs`.

The latest-result read exposes execution facts only. Pending work is invisible; `204` means a known Monitor has no terminal result; `worker_timeout` omits duration and HTTP status. There is no derived up/down policy.

Compose contains four services: Web placeholder, Go API, Rust Checker, and PostgreSQL. Application host ports are not published. React, authentication/authorization, public deployment, Monitor listing, history, and mutable lifecycle remain deferred.

The current `docs/architecture/module-boundaries.md` still omits `GetLatestCheckResult` from its application/public adapter summary. This documentation gap should be corrected in a separate focused documentation change; it does not imply missing runtime behavior.

## 3. Alternatives

| Option | Value | Cost and boundary impact |
|---|---|---|
| Local Web vertical slice — recommended | Makes existing registration and latest-result behavior directly usable in a browser | Introduces React runtime, browser integration, and a carefully bounded local access path |
| Internet-facing Web release | Allows remote users to access the application | Requires audience, authentication/authorization, abuse controls, ingress/TLS, deployment, and operations decisions before exposure |
| Availability/history slice first | Adds richer monitoring information | Requires new product semantics and possibly contract/persistence evolution; browser use remains deferred |

The local Web slice is recommended because the existing public contract already supports one complete, small user journey. It provides browser integration evidence before taking on a public-service security and operations boundary.

## 4. Recommended milestone: Local Web Monitoring Vertical Slice

### User journey

1. Open the application through an explicitly configured local browser endpoint.
2. Enter an HTTP/HTTPS target URL and create a Monitor through the existing public contract.
3. Navigate to a shareable-in-the-local-application Monitor detail URL containing its ID.
4. Read the immutable Monitor and its latest terminal result.
5. Refresh the result without recreating the Monitor or triggering an execution from the browser.
6. Reopen a known Monitor detail URL after a browser restart.

A Monitor list is not required for this first journey. A detail URL containing the returned ID supplies retrieval without inventing a browser-owned inventory or extending the Go contract.

### Presentation semantics

The UI must distinguish loading, successful Monitor creation, invalid input, missing Monitor, transport/server failure, no terminal result, and a terminal execution fact.

Display the target URL, latest result kind, completion time, and available duration/HTTP status. Label `204` as no completed result yet; do not imply that no work exists or that the target is healthy. HTTP responses, including error status codes, remain execution facts. Do not derive availability from `resultKind` or HTTP status.

Manual refresh is sufficient for the scope recommendation. Any automatic polling policy must be explicitly bounded in the detailed design, including cancellation and error behavior.

### Ownership and integration constraints

- React owns presentation and browser interaction.
- Go remains the only owner of product semantics and durable state.
- The browser consumes only the three existing public operations.
- The browser never calls Checker claim/result endpoints or accesses PostgreSQL.
- Preserve the committed frontend dependency direction: `app -> pages -> widgets -> features -> entities -> shared`. Create layers only when they contain a concrete responsibility.
- Preserve Go/Rust execution behavior, production destination policy, and explicit migration workflow.
- Preserve the four logical Compose services; replace the Web placeholder only through the reviewed implementation design.

### Local access boundary

The detailed design must select the local browser endpoint and same-origin integration strategy. A candidate is a loopback-bound Web endpoint with an explicit allowlist of the existing public API routes proxied to Go. This is a proposal, not a selected server or proxy implementation.

If this approach is selected, it must reject internal execution routes and avoid blanket proxying of API paths. Go API, Checker, and PostgreSQL must remain unpublished on the host. A loopback endpoint does not provide authentication; remote access, shared machines, browser-origin abuse protections, and proxy behavior require explicit treatment in the design.

The design must decide whether local access belongs in canonical Compose or an opt-in development override. Existing smoke assumptions and the `http://web/` destination-policy rejection scenario must be reassessed before replacing the placeholder.

## 5. Excluded capabilities

This proposed milestone excludes internet-facing deployment, authentication/authorization, multi-user ownership, Monitor listing, full CheckRun history, derived availability, incidents/notifications, mutable Monitor lifecycle, configurable cadence, private-network monitoring, and multi-checker coordination.

A requirement for any of these capabilities reopens scope rather than silently expanding the first Web slice.

## 6. Evidence required by the later implementation plan

- Frontend import-boundary and type/build checks.
- Contract-shaped browser client behavior for Monitor creation/read and the three latest-result variants.
- UI evidence for `204`, missing Monitor, invalid input, and failed requests.
- Browser acceptance of create -> detail -> latest-result -> reopen detail against real Go/PostgreSQL composition.
- Deterministic Go/Rust execution evidence, retaining the production destination policy and avoiding dependence on an external website for acceptance.
- Local integration evidence that public operations are reachable through the selected browser endpoint and internal claim/result routes are unreachable through it.
- Compose/startup/readiness checks and current-state documentation updates appropriate to the chosen access strategy.

Exact test tools, runtime dependencies, server/proxy configuration, and CI changes belong in the written design and implementation plan. No passing test or CI result is claimed by this assessment.

## 7. Next decision and sequencing

Confirm the intended audience and approve or revise the recommended scope. Respect the Current Availability gate in #44 before scheduling this Web milestone. Later prepare a dedicated Local Web Monitoring Vertical Slice design, including local access, frontend organization, browser client semantics, lifecycle, verification, and smoke compatibility.

~~~text
Latest Check Result Public Read — implemented
  -> Current Availability scope/design/plan — pending #44 and subsequent reviews
  -> Web/public-exposure scope reassessment — this candidate
  -> Local Web Monitoring Vertical Slice design — after scope review
  -> reviewed implementation plan
  -> implementation and verification
  -> separate internet-facing deployment reassessment when needed
~~~

This document authorizes no runtime, OpenAPI, SQL migration, dependency, Compose, or CI implementation. It is a reviewable scope proposal and does not claim approval, remote landing, or successful CI.
