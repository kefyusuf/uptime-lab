# Local Monitor Inventory Scope Reassessment

**Status:** Scope proposal; design and implementation are not approved
**Date:** 2026-10-04
**Verified base:** `main@75200ab969369d4a1d300c31558c0b3e1cc06fd4`
**Purpose:** Select the first bounded R2 increment after the local Web journey, without changing runtime behavior.

## 1. Current evidence and user intent

The user wants careful continuation toward a useful monitoring product, guided by the industry roadmap, with TDD, short-lived branches, commits, PRs, and brief before/after explanations. The current audience remains the previously approved local browser user. A later beta audience, budget, operator, notification channel and recovery envelope remain undecided.

PRs [#51](https://github.com/kefyusuf/uptime-lab/pull/51) and [#52](https://github.com/kefyusuf/uptime-lab/pull/52) are merged. [Exact-main CI](https://github.com/kefyusuf/uptime-lab/actions/runs/37221902477) passed the Web gate, canonical Docker smoke and real browser/Go/Checker/PostgreSQL journeys on ports 4173 and 4817. The tested PR52 head passed 144 app tests and its tree equals the merged base. Unchanged Go/Rust/contracts were skipped in that main run; earlier full-runtime evidence is recorded separately in the handoff. This is local-development evidence, not production qualification.

The browser can register, reopen by UUID, read independent current-availability/raw-result snapshots, and refresh manually. It cannot discover existing Monitors after losing their IDs. Go's public collection only accepts POST. There are no accounts, Monitor list, history, lifecycle mutations, incidents or notifications.

Verified entry points:

- `contracts/openapi/public.yaml`: four public operations, no collection GET.
- `apps/api/internal/modules/monitoring/ports/monitor_repository.go`: Create and ByID only.
- `apps/api/internal/modules/monitoring/adapters/postgres/repository.go`: immutable registration and ID lookup.
- `apps/api/migrations/00001_create_monitoring_monitors.sql`: UUID primary key, target URL and creation time; no creation-order index or ownership column.
- `apps/web/server/routes.ts`: collection POST only; raw query strings and percent escapes are rejected before forwarding.

## 2. Industry comparison and selection

On 2026-10-04, [Better Stack's official pagination documentation](https://betterstack.com/docs/uptime/api/pagination/) describes paginated resource collections, including Monitor lists. Its [pause documentation](https://betterstack.com/docs/uptime/pausing-monitors-and-maintenances/) treats pausing as a distinct operation that stops monitoring. These are representative product capabilities, not requirements to copy its page sizes, API representation or lifecycle semantics.

The repository already supports registration and single-Monitor investigation. A bounded inventory closes the retrieval gap with fewer semantic changes than pause/resume, history or alerts. This sequencing is a project recommendation inferred from the sources and repository evidence.

| Candidate | User value | Cost and decision |
|---|---|---|
| Local durable inventory — recommended | Find registered targets and open the existing detail journey without retaining UUIDs | Add a bounded Go-owned collection read and local browser integration; no scheduling mutation |
| Pause/resume first | Stop unwanted recurring work | Requires an explicit policy for due claims, pending executions, late completion and resume; defer to a separate lifecycle design |
| History or incidents/notifications first | Investigate past results or receive failure alerts | Requires retention, ordering or durable transition/delivery semantics; defer to later R2/R3/R4 increments |

A browser-local list of recently created IDs is not sufficient: it misses durable Monitors created in other sessions and loses state when browser storage is cleared. An unbounded collection dump is also unsuitable. Prefer server-owned pagination and bounded rendering.

## 3. Recommended milestone

The Local Monitor Inventory Slice should let the local user:

1. Open a bounded page of registered Monitors from the application's start view.
2. Read each Monitor's exact target text, ID and UTC creation time.
3. Move through pages explicitly, refresh from the first page, and distinguish empty, loading, end-of-list and failed reads.
4. Select a Monitor and use the existing detail URL and independent snapshot reads.
5. Register a new Monitor through the existing form, then return to a freshly read first page without duplicating registration.

The list is inventory, not a health dashboard: rows do not trigger availability/result fan-out, show a fabricated status, aggregate uptime, or schedule checks. No polling, search, sorting controls, total count, bulk actions or browser-owned durable inventory are proposed.

## 4. Constraints for the written design

Go remains authoritative for collection validation, ordering and persistence. React owns presentation, cancellation and navigation. Rust execution and the internal claim/result contract remain unchanged.

The design must resolve these items before implementation:

- **Ordering and traversal:** Recommend descending immutable creation time with UUID as a unique tie-breaker and keyset traversal. Equal timestamps, new registrations between requests, deleted or nonexistent cursor anchors, and exhausted pages require explicit semantics. No snapshot isolation across an entire browser traversal is promised; refresh restarts it. Define cursor encoding, length, validation, timestamp precision and versioning rather than accepting arbitrary client SQL/order choices.
- **Contract and limits:** Define the additive collection GET, closed page envelope, default/maximum page size, continuation representation, no-store responses and deterministic invalid-input errors. Reject unknown, duplicate, malformed or excessive query parameters. Empty inventory is a successful empty page, not an unknown Monitor or transport failure.
- **Response bytes:** Row count alone is insufficient. Existing targets are text and can approach the registration body budget; several such rows can exceed the gateway's 256 KiB upstream response cap. Define a compatible byte budget and explicit oversized-item behavior without silently truncating target values, skipping records, loosening all gateway limits or producing a non-advancing page. Do not assume 50 rows are safe or retroactively invalidate stored Monitors.
- **Persistence:** Verify the chosen ordering against real PostgreSQL. A composite index may require a new immutable migration. Do not edit historical migrations or add speculative fields. Establish query/row bounds and cancellation without claiming production capacity from a small test.
- **Gateway:** The current rejection of every query string must become an exact collection-only rule if pagination uses queries. Preserve raw-path rejection, fixed upstream, method allowlists, header/body/response/deadline bounds and origin policy. Do not blanket-enable query strings or internal routes. Cursor/target data must not be placed in logs.
- **UI requests:** Bound loaded rows and concurrent requests; prevent late responses from replacing a newer page or refresh. A failed read must not become an empty inventory or a current successful page. Preserve exact target text and existing detail/creation semantics; render URLs as text rather than following user-controlled targets.
- **Access boundary:** A list enumerates all durable Monitors and their potentially sensitive URLs. It has no account isolation in the existing system. Retain the opt-in loopback Web endpoint and unpublished API/Checker/PostgreSQL. Local access is not authentication; do not share or expose it remotely. Remote/shared-account use requires authorization and ownership scoped before ordering and limits.

## 5. Explicit exclusions

Pause/resume, edit/delete, cadence configuration, CheckRun history, retention enforcement, availability aggregation, expected-status configuration, incidents, notifications, authentication/ownership, remote deployment and multi-checker coordination are excluded. They remain roadmap work, rather than completion claims attached to inventory.

R0 product/release decisions remain open. R1 is complete for its local scope. Inventory is only the first proposed part of R2; it does not complete Monitor management or make a dependable monitoring beta.

## 6. Evidence expected from the later TDD plan

- Closed public-contract fixtures for valid/empty/final pages, ordering, cursor/query errors, byte limits and unchanged existing operations.
- Go application tests for validation, cancellation and sanitized persistence errors; no calls to scheduling or result mutation.
- Real PostgreSQL tests for identical creation timestamps, page boundaries, concurrent registrations, durable readback, index/migration compatibility and absence of CheckRun mutation.
- Gateway tests for precisely allowed collection query forms, duplicate/unknown parameters, path/encoding tricks, forbidden internal routes, methods, response-size/deadline bounds and unchanged POST behavior.
- Browser adapter/component tests for bounded navigation, empty/error distinction, refresh, cancellation, out-of-order responses and exact target presentation.
- Real Docker/Chromium journey: register several controlled targets, reopen with no retained IDs, traverse inventory, select a durable Monitor, and preserve raw `204` versus availability `unknown/no_result` before Checker startup. Retain existing canonical smoke and default/custom-port evidence.
- Exact-head CI and a reviewed implementation; local fixtures cannot substitute for real PostgreSQL or composed browser evidence.

Tests must first demonstrate failure for the behavior they introduce, then pass with the smallest correct implementation. This documentation-only scope change has no new product behavior to test.

## 7. Next gate

Review the inventory scope and alternatives. Approval permits preparing its written cross-runtime design; it does not approve an absent design or implementation plan. The written design must then be reviewed before the TDD plan and its execution method are selected. Implementation follows that plan on a separate branch/PR.

This proposal changes no source, public/internal contract, migration, dependency, runtime configuration, CI behavior or deployment.
