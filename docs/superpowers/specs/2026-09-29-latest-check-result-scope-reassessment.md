# Latest Check Result Scope Reassessment

**Status:** Review candidate
**Date:** 2026-09-29
**Repository:** `kefyusuf/uptime-lab`
**Base:** `main@0afced3bff109c63c01e3e07eba3d8331c90432c`
**Purpose:** Select the next product milestone after the landed Single-Checker Execution Vertical Slice without beginning implementation.

---

## 1. Current implementation truth

The repository now has a complete first execution loop:

~~~text
registered Monitor
  -> Go claim/scheduling
  -> Rust Checker
  -> bounded HTTP/HTTPS execution
  -> normalized result submission
  -> Go completion
  -> PostgreSQL terminal CheckRun
~~~

Current public product operations remain exactly:

~~~text
POST /monitors
GET  /monitors/{monitorId}
~~~

Current internal Checker operations remain exactly:

~~~text
POST /internal/checks/claim
PUT  /internal/checks/{checkId}/result
~~~

Durable Monitoring state now contains:

~~~text
monitoring.monitors
monitoring.check_runs
~~~

A terminal CheckRun already records the execution facts required by the first slice:

~~~text
id
monitor_id
issued_at
deadline_at
completed_at
result_kind
http_status
duration_ms
~~~

The canonical Docker smoke proves the real Go -> Rust -> policy_rejected -> Go -> PostgreSQL path.

The product gap has therefore changed. Monitoring execution now exists, but callers still cannot observe any execution result through the public contract.

---

## 2. Decision criterion

The next milestone should expose the smallest useful piece of the monitoring truth that already exists.

Prefer work that:

1. converts existing durable CheckRun evidence into user-observable product value;
2. does not invent availability semantics that the execution slice deliberately deferred;
3. preserves the current narrow public contract discipline;
4. requires no new distributed infrastructure;
5. creates a useful backend surface before React or public deployment work;
6. minimizes new persistence and migration requirements.

---

## 3. Candidate directions

### A. Full CheckRun history

Potential value:

- users can inspect every execution;
- enables timelines and later charts;
- prepares for richer UI.

Why not next:

- immediately requires ordering, pagination, retention expectations, and response-shape policy;
- broadens the public API before the minimum current-result use case is proven;
- creates more product surface than is needed to make monitoring output observable.

**Decision:** defer.

### B. Derived Monitor availability status

Potential value:

- gives a simple `up` / `down` style product signal;
- is directly useful for dashboards and alerts.

Why not next:

The execution slice explicitly separated:

~~~text
execution fact
~~~

from:

~~~text
product availability/state policy
~~~

A derived status would require new product decisions such as:

- whether all 2xx/3xx/4xx/5xx responses have the same availability meaning;
- whether transport failures and policy rejections map to the same product state;
- how stale results are treated;
- what status means before the first terminal run;
- whether a currently pending run affects displayed state.

Those decisions deserve their own design evidence.

**Decision:** defer derived availability semantics.

### C. Latest terminal CheckRun public read slice

Potential value:

- exposes real monitoring output without inventing higher-level status semantics;
- uses already-persisted product facts;
- provides the smallest useful bridge from execution to later Web work;
- keeps history/pagination out of scope;
- makes the next product journey observable end to end.

Candidate product journey:

~~~text
register Monitor
  -> Checker executes
  -> Go persists terminal CheckRun
  -> caller reads the latest terminal execution fact
~~~

**Decision:** select this direction.

### D. React Web foundation

Potential value:

- visible browser workflow;
- establishes the first real frontend runtime.

Why not next:

The backend still has no public execution-result read surface. Building React now would either display only Monitor registration data or force API design to happen implicitly inside frontend work.

**Decision:** defer until the selected read slice lands.

### E. Authentication / public ingress

Potential value:

- enables deployment-facing access control and public reachability.

Why not next:

- canonical Compose still publishes no application host port;
- the repository is not production-deployed;
- there is still a more direct product-value gap: callers cannot read the monitoring result already produced.

**Decision:** defer to the later Web/public-exposure reassessment.

---

## 4. Selected next milestone

The next milestone is:

~~~text
Latest Check Result Public Read Slice
~~~

This reassessment does **not** authorize implementation.

The next gate must be a dedicated design that defines how one Monitor's latest terminal execution fact becomes public without turning execution facts into derived availability policy.

The design target is conceptually:

~~~text
Monitor identity
  -> Go reads latest terminal CheckRun
  -> public representation exposes a bounded execution fact
~~~

No route or schema shape is authorized by this reassessment.

---

## 5. Design blockers that must be resolved before implementation planning

### 5.1 Meaning of "latest"

The design must define the authoritative ordering for terminal CheckRuns.

It must decide whether "latest" is based on:

- terminal completion time;
- issuance time;
- CheckID ordering;
- another explicitly justified invariant.

The rule must remain deterministic and compatible with future history reads.

### 5.2 No-result semantics

The public behavior must distinguish:

- Monitor does not exist;
- Monitor exists but has never completed a check;
- Monitor has a latest terminal CheckRun.

The design must decide whether "no completed result yet" is represented by:

- an empty/nullable representation;
- a dedicated response status;
- another explicit contract shape.

This must not be confused with an error or an "unknown/down" availability status.

### 5.3 Pending-run visibility

The current durable model can contain one pending CheckRun per Monitor.

The design must explicitly decide whether the first public read slice:

1. exposes only the latest terminal result; or
2. also exposes that a run is currently pending.

Default scope pressure should favor terminal result only unless pending state is required for a coherent contract.

### 5.4 Public result vocabulary

The design must decide which execution facts are product-safe and stable enough to expose.

Candidate fields already owned by Go include:

~~~text
checkId
resultKind
httpStatus?
durationMs
completedAt
~~~

Internal coordination fields such as claim deadlines must not leak merely because they exist in persistence.

The public representation must not reuse PostgreSQL rows or internal Checker DTOs directly.

### 5.5 Availability semantics remain separate

The selected milestone exposes execution facts, not an `up` / `down` verdict.

The design must preserve the execution-slice rule that Rust does not decide product availability and must not accidentally introduce that policy through naming or DTO structure.

If the design discovers that a derived status is strictly required for a coherent public representation, that becomes a separate explicit decision rather than an incidental mapping.

### 5.6 Public contract shape

The design must choose the smallest coherent public contract shape only after product semantics are fixed.

Candidate shapes may include:

- extending the existing Monitor read representation;
- a Monitor-scoped latest-result subresource.

This reassessment does not select between them.

The design must consider compatibility with the existing exact create/read contract and later history expansion.

### 5.7 Persistence/query shape

The design must verify whether the current `monitoring.check_runs` schema can serve the selected read efficiently and deterministically.

Prefer:

- a narrow Go-owned read query;
- no new table;
- no duplicated monitor state;
- no migration unless query correctness or bounded performance proves one necessary.

Do not add a materialized status table merely for convenience.

### 5.8 Contract and architecture evidence

The design must define the minimum verification layers for:

- public OpenAPI semantics;
- Go application read behavior;
- PostgreSQL latest-terminal query semantics;
- HTTP adapter mapping;
- real PostgreSQL composition evidence;
- canonical Docker evidence that execution becomes publicly observable.

Rust behavior should not change unless the design identifies a concrete requirement.

---

## 6. Explicitly deferred from the selected milestone

Keep these out unless the design proves one is strictly required:

- full CheckRun history;
- pagination/cursors;
- retention policy;
- aggregated uptime percentages;
- latency charts/percentiles;
- derived `up` / `down` availability state;
- incidents;
- notifications;
- status pages;
- React implementation;
- public ingress;
- authentication/authorization;
- CORS/rate limiting;
- Monitor list/update/delete/enable/disable;
- target mutation;
- configurable cadence/retry policy;
- multi-checker coordination;
- brokers/outbox;
- new probe types;
- private-network monitoring.

This milestone should make one existing terminal execution result observable, not turn the repository into a complete monitoring product.

---

## 7. Sequencing

The preferred order is now:

~~~text
Single-Checker Execution Vertical Slice
  -> Latest Check Result scope reassessment
  -> Latest Check Result design
  -> reviewed implementation plan
  -> implementation
  -> Web/public-exposure reassessment
~~~

A separate availability/history reassessment may be opened after the latest-result slice provides real product evidence.

---

## 8. Gate acceptance criteria

This reassessment is GREEN only if review agrees that:

1. the execution slice is complete and no rework is required to begin the next product decision;
2. the highest-value immediate gap is public observability of an already-persisted execution result;
3. the next milestone is **Latest Check Result Public Read Slice**;
4. full history remains deferred;
5. derived availability/status policy remains deferred;
6. this document authorizes no OpenAPI, Go, SQL, Rust, Compose, or CI implementation;
7. the current Go/Rust ownership boundaries remain unchanged;
8. no new table or migration is assumed by this scope decision;
9. React and public deployment remain deferred;
10. the next repository change after this gate is a dedicated latest-result design document.

---

## 9. Self-review

### Product value

PASS.

The selected direction exposes the first monitoring output that the system already produces durably.

### Architecture

PASS.

Go remains the owner of public product semantics and PostgreSQL state. Rust remains execution-only.

### Contract discipline

PASS.

No route or response schema is selected here. Public contract shape is deferred to the design gate.

### Persistence discipline

PASS.

The current CheckRun model is treated as the source of truth. No new table or migration is authorized.

### Security

PASS at scope level.

The milestone does not expand network execution, host exposure, or authentication boundaries.

### YAGNI

PASS.

History, pagination, availability policy, React, incidents, notifications, brokers, and mutable Monitor lifecycle remain outside the milestone.

### Execution safety

PASS.

This branch must contain only this scope reassessment document.

---

## 10. Explicit STOP boundary

After this document is reviewed and landed:

~~~text
NEXT:
Latest Check Result Public Read Slice design
~~~

Do not begin implementation planning, modify `contracts/openapi/public.yaml`, add SQL migrations, or change runtime code until that design gate is separately opened and reviewed.
