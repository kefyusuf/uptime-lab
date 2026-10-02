# Current Availability Scope Reassessment

**Status:** Review candidate
**Date:** 2026-10-02
**Repository:** `kefyusuf/uptime-lab`
**Base:** `main@6669aecaca4235c61e9367c6a518a163a3dcf276`
**Purpose:** Select the next product milestone after the landed Latest Check Result Public Read Slice without beginning design or implementation.

---

## 1. Current implementation truth

The repository now has a complete first monitoring loop and one bounded public execution-result read:

~~~text
register Monitor
  -> Go schedules due work
  -> Rust executes one bounded HTTP/HTTPS probe
  -> Go persists terminal CheckRun
  -> caller reads the latest terminal execution fact
~~~

Current public Monitoring operations are exactly:

~~~text
POST /monitors
GET  /monitors/{monitorId}
GET  /monitors/{monitorId}/latest-result
~~~

Current internal Checker operations remain exactly:

~~~text
POST /internal/checks/claim
PUT  /internal/checks/{checkId}/result
~~~

Durable Monitoring state remains:

~~~text
monitoring.monitors
monitoring.check_runs
~~~

The latest-result operation exposes one latest terminal execution fact only.

It deliberately does not expose a product availability verdict. A caller can observe facts such as:

~~~text
http_response
policy_rejected
timeout
connect_error
worker_timeout
~~~

but the product does not yet answer the higher-level question:

~~~text
what is this Monitor's current availability state?
~~~

The product gap has therefore changed again. Execution exists, durable evidence exists, and the latest terminal fact is public; product-level current availability semantics do not.

---

## 2. Decision criterion

The next milestone should add the smallest product semantic that turns existing execution evidence into a useful monitoring state without broadening into history, incidents, notifications, frontend, or public deployment.

Prefer work that:

1. derives value from the terminal CheckRun evidence already owned by Go;
2. keeps Rust responsible only for execution facts;
3. does not require a history collection or pagination;
4. does not introduce distributed infrastructure;
5. can be consumed later by a meaningful Web experience;
6. makes availability policy explicit instead of hiding it in UI or transport mapping;
7. minimizes new persistence and migration requirements;
8. preserves the current narrow public-contract discipline.

---

## 3. Candidate directions

### A. Full CheckRun history

Potential value:

- enables timelines and investigation;
- supports charts and later reporting;
- exposes more execution evidence.

Why not next:

- requires collection semantics, ordering, pagination/cursors, and retention expectations;
- increases API surface before a product-level current state exists;
- does not by itself answer whether a Monitor should presently be considered available.

**Decision:** defer.

### B. React Web foundation

Potential value:

- makes the project visibly usable in a browser;
- establishes the first real presentation runtime.

Why not next:

The backend currently exposes registration data plus raw execution facts, but no product-level availability semantic.

Starting React now would force one of two undesirable outcomes:

1. the UI displays low-level execution vocabulary directly as the product state; or
2. availability policy is invented inside the frontend.

Availability policy belongs to Go-owned product semantics, not presentation code.

**Decision:** defer until the selected backend semantic lands.

### C. Authentication / public ingress

Potential value:

- enables controlled external access;
- moves toward a deployable public service.

Why not next:

- canonical Compose still publishes no application host ports;
- the repository remains explicitly non-production-ready;
- exposing the system publicly before the product has a coherent current availability state expands operational/security scope ahead of product value.

**Decision:** defer to a later Web/public-exposure reassessment.

### D. Mutable Monitor lifecycle

Potential value:

- enable/disable, target changes, deletion, or cadence controls would make Monitor management richer.

Why not next:

- each operation introduces lifecycle and scheduling semantics;
- mutation breadth does not close the current observability gap;
- the project can already register, execute, persist, and read a Monitor result end to end.

**Decision:** defer.

### E. Current Monitor Availability Read Slice

Potential value:

- converts an already-public execution fact into a product-level current state;
- establishes the first explicit availability policy in the Go product boundary;
- gives a future Web client a stable semantic instead of low-level execution vocabulary;
- creates a useful foundation for later incidents and notifications without implementing either;
- can remain a single-Monitor current-state concern rather than a history concern.

Candidate product journey:

~~~text
register Monitor
  -> Checker executes
  -> Go persists terminal CheckRun
  -> Go applies explicit current-availability policy
  -> caller reads the Monitor's current availability state
~~~

**Decision:** select this direction.

---

## 4. Selected next milestone

The next milestone is:

~~~text
Current Monitor Availability Read Slice
~~~

This reassessment does **not** define or authorize:

- a route;
- a response schema;
- status vocabulary;
- HTTP-status classification rules;
- failure-to-status mappings;
- staleness thresholds;
- persistence changes;
- implementation.

The next gate must be a dedicated design that defines how Go-owned product semantics derive one Monitor's current availability from existing execution evidence.

The design target is conceptually:

~~~text
Monitor
  + latest terminal execution fact
  + explicit time/policy rules
  -> current product availability state
~~~

No API shape is authorized by this document.

---

## 5. Design blockers that must be resolved before implementation planning

### 5.1 Availability vocabulary

The design must choose the smallest coherent product vocabulary.

Candidate concepts may include states such as:

~~~text
available
unavailable
unknown
~~~

or another explicitly justified vocabulary.

This reassessment does not approve `up/down`, `healthy/unhealthy`, `degraded`, or any other concrete enum.

The vocabulary must describe product state, not execution mechanics.

### 5.2 HTTP response semantics

The design must explicitly decide how an `http_response` execution fact contributes to availability.

Questions include:

- whether availability means successful TCP/TLS/HTTP reachability or application-level success;
- which HTTP status classes, if any, count as available;
- whether redirects have already been normalized sufficiently for product policy;
- whether the policy is fixed for this milestone or configurable.

These decisions must not be inferred from conventional assumptions.

### 5.3 Failure-result semantics

The design must decide how each existing terminal execution kind contributes to current availability:

~~~text
dns_error
policy_rejected
timeout
connect_error
tls_error
protocol_error
internal_error
worker_timeout
~~~

In particular, the design must distinguish where necessary between:

- target unavailability;
- execution infrastructure failure;
- policy/configuration rejection;
- absence of sufficient evidence.

Do not automatically collapse every non-`http_response` result into one unavailable state.

### 5.4 No-result semantics

The design must define the state for a known Monitor with no terminal CheckRun.

Current public behavior already distinguishes this as:

~~~text
204 on latest-result
~~~

The availability design must decide whether the corresponding product state is unknown, pending-initial-evidence, absent, or represented another way.

This must not rewrite the latest-result contract implicitly.

### 5.5 Freshness and staleness

A latest terminal result is historical evidence, even when it is the newest evidence.

The design must define whether current availability remains valid indefinitely or becomes stale after an explicit rule.

Questions include:

- whether cadence provides the freshness basis;
- whether a fixed threshold is acceptable for the first slice;
- whether no configurable cadence means staleness should remain out of scope;
- how clock/time ownership is tested deterministically.

Do not invent a stale-state rule merely because a current timestamp is available.

### 5.6 Pending-run semantics

The latest-result slice deliberately hides pending CheckRuns.

The availability design must decide whether a newer pending run affects current availability.

Default scope pressure should preserve:

~~~text
current availability derived from terminal evidence only
~~~

unless product coherence proves pending state is required.

A GET must not reconcile or mutate execution state.

### 5.7 Derived versus persisted state

The design must decide whether current availability is:

1. derived on read from existing Monitor/CheckRun truth; or
2. persisted as additional product state.

Default preference is derivation from existing truth.

A new column, table, materialized state, event stream, or migration requires explicit proof that correct bounded derivation is insufficient.

Convenience is not sufficient justification.

### 5.8 Public contract shape

Only after product semantics are fixed may the design select the smallest public representation.

Candidate shapes may include:

- a dedicated Monitor-scoped current-state subresource;
- a bounded extension to an existing read representation;
- another explicitly justified shape.

The existing Monitor create/read payload compatibility and latest-result contract must remain stable unless the design proves a breaking change is unavoidable.

### 5.9 Ownership boundary

The product availability decision belongs to Go.

Rust continues to own:

~~~text
bounded execution
destination policy
network/protocol normalization
result delivery
~~~

Rust must not begin returning `up`, `down`, or another product verdict merely to simplify Go.

PostgreSQL remains durable product storage; it does not become the owner of availability policy through SQL-only business rules.

### 5.10 Verification evidence

The design must define the minimum evidence required for:

- product-policy unit tests;
- deterministic clock/freshness tests if freshness enters scope;
- mapping of every supported terminal result kind;
- missing-Monitor and no-terminal-result semantics;
- public contract compatibility;
- PostgreSQL read behavior if persistence access changes;
- production Go composition;
- canonical Docker evidence only if the approved design creates a new observable runtime behavior.

No Rust, migration, Compose, or CI change should be assumed.

---

## 6. Explicitly deferred from the selected milestone

Keep these out unless the design proves one is strictly required:

- full CheckRun history;
- pagination/cursors;
- retention policy;
- uptime percentages;
- latency aggregation, percentiles, or charts;
- incidents;
- incident lifecycle;
- notifications;
- alert delivery;
- public status pages;
- React implementation;
- browser authentication;
- public ingress;
- TLS termination;
- CORS/rate limiting;
- Monitor list/update/delete/enable/disable;
- target mutation;
- configurable cadence;
- configurable availability policy;
- configurable retry policy;
- multi-checker coordination;
- brokers/outbox;
- new probe types;
- private-network monitoring;
- new persistence tables/materialized status unless design evidence requires them.

The milestone should define and expose one current availability semantic, not become a complete monitoring platform.

---

## 7. Sequencing

The preferred order is now:

~~~text
Single-Checker Execution Vertical Slice
  -> Latest Check Result Public Read Slice
  -> Current Availability scope reassessment
  -> Current Availability design
  -> reviewed implementation plan
  -> implementation
  -> next product reassessment
~~~

After the selected slice lands, the next reassessment may compare Web/public exposure, history, and Monitor lifecycle using the then-current product evidence.

---

## 8. Gate acceptance criteria

This reassessment is GREEN only if review agrees that:

1. the Latest Check Result Public Read Slice is complete and requires no rework before the next product decision;
2. the current highest-value gap is absence of an explicit product-level current availability state;
3. the next milestone is **Current Monitor Availability Read Slice**;
4. availability policy remains Go-owned;
5. Rust continues to expose execution facts only;
6. full history and pagination remain deferred;
7. React/Web work remains deferred until backend availability semantics are explicit;
8. auth/public ingress remains deferred;
9. mutable Monitor lifecycle remains deferred;
10. this document selects no route, response schema, status enum, HTTP-status mapping, freshness threshold, or persistence shape;
11. no migration, table, index, cache, broker, or materialized state is assumed;
12. this document authorizes no OpenAPI, Go, SQL, Rust, Compose, CI, or current-state documentation implementation;
13. the next repository change after this gate is a dedicated Current Availability design document.

---

## 9. Self-review

### Product value

PASS.

The selected direction turns already-visible execution evidence into the smallest missing monitoring product semantic.

### Architecture

PASS.

Go remains the owner of product policy and durable orchestration. Rust remains execution-only.

### Contract discipline

PASS.

No route, DTO, enum, or HTTP response behavior is selected here.

### Persistence discipline

PASS.

Existing Monitor/CheckRun truth is the default derivation source. No new persistence is authorized.

### Security

PASS at scope level.

The milestone does not expand network reachability, host exposure, authentication boundaries, or probe capabilities.

### YAGNI

PASS.

History, frontend, auth, incidents, notifications, mutable lifecycle, configurability, and distributed infrastructure remain outside the milestone.

### Execution safety

PASS.

This branch must contain only this scope reassessment document.

---

## 10. Explicit STOP boundary

After this document is reviewed and landed:

~~~text
NEXT:
Current Monitor Availability Read Slice design
~~~

Do not begin implementation planning.

Do not modify:

- `contracts/openapi/public.yaml`;
- Go application/runtime code;
- PostgreSQL adapters or migrations;
- Rust;
- Compose;
- CI;
- current-state documentation for unimplemented behavior.

The design gate must first resolve product semantics, ownership, time/freshness behavior, public-contract shape, persistence strategy, and verification evidence.
