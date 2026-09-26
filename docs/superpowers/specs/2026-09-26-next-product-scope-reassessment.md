# Next Product Scope Reassessment

**Status:** Review candidate
**Date:** 2026-09-26
**Repository:** `kefyusuf/uptime-lab`
**Base:** `main@8cee60c128830bf173e47e70516cbb70a4ad1a3e`
**Purpose:** Select the next product milestone after the Go Public Transport Adapter phase without beginning implementation.

---

## 1. Current implementation truth

The repository now has a real Go Monitoring create/read runtime:

~~~text
POST /monitors
GET  /monitors/{monitorId}
GET  /livez
GET  /readyz
~~~

Current guarantees include:

- PostgreSQL-owned Monitor persistence;
- immutable landed SQL migration history;
- read-only schema-aware readiness;
- explicit migration bootstrap;
- production UUID v7 identity generation;
- persistence-stable UTC microsecond creation time;
- exact public OpenAPI create/read contract;
- real container-local POST -> GET Docker evidence;
- no application host port in canonical Compose.

The following remain intentionally absent:

- Rust Checker runtime;
- internal Go <-> Rust contract;
- check scheduling / due-work coordination;
- check-result persistence;
- monitor status/history;
- React Web runtime;
- public network exposure;
- authentication/authorization;
- CORS and rate limiting.

A registered Monitor therefore still does not execute a check. The next milestone should close that product gap rather than add breadth around an inert monitor.

---

## 2. Decision criterion

The next milestone should maximize **new uptime-monitoring product truth** while preserving the repository's current engineering discipline.

The decision should prefer work that:

1. turns a registered target into observable monitoring behavior;
2. exercises the committed Go/Rust runtime boundary;
3. resolves high-risk network-execution security semantics before public exposure;
4. creates reusable product evidence rather than scaffolding;
5. avoids premature distributed infrastructure or UI breadth.

---

## 3. Candidate directions

### A. React Web foundation

Potential value:

- visible create/read UI;
- first browser runtime;
- frontend architecture enforcement.

Why not next:

- there is no check result or status to present;
- the public API has no monitor collection/list operation;
- the UI would mainly front an incomplete create/read backend;
- it does not advance the core uptime-monitoring behavior.

**Decision:** defer until the backend can produce useful monitoring state.

### B. Public exposure + authentication/security edge

Potential value:

- host/browser reachability;
- identity and ownership;
- request-size/rate-limit policy.

Why not next:

- exposure hardens access to a product that still does not perform checks;
- authentication remains a deliberate foundation non-goal;
- the latest external review explicitly found no need to expose the current container-internal transport.

**Decision:** defer until there is a complete product journey worth exposing.

### C. More Monitoring CRUD / lifecycle

Potential value:

- list/update/delete;
- enable/disable;
- configurable policy.

Why not next:

- more configuration does not make the current Monitor execute;
- it risks broadening public contract and persistence before execution requirements are known;
- lifecycle semantics should be informed by actual checker/scheduling behavior.

**Decision:** defer user-configurable lifecycle breadth.

### D. Observability infrastructure

Potential value:

- metrics/tracing/log aggregation.

Why not next:

- current structured logging and health semantics are sufficient for the existing runtime;
- useful checker metrics and correlation semantics are easier to define after real execution exists;
- large observability infrastructure remains an explicit foundation non-goal.

**Decision:** keep observability conventions, defer infrastructure.

### E. Single-checker execution vertical slice

Potential value:

- registered monitors begin performing real HTTP/HTTPS checks;
- exercises the committed Go Control Plane <-> Rust Execution Plane boundary;
- introduces the first real internal contract;
- forces explicit scheduling/result/idempotency semantics;
- forces SSRF/DNS/redirect/resource-budget decisions before public exposure;
- creates the data needed for later status/history and Web work.

This is the first candidate that materially changes the product from "target registry" into an uptime-monitoring system.

**Decision:** select this direction.

---

## 4. Selected next milestone

The next milestone is:

~~~text
Single-Checker Execution Vertical Slice
~~~

This reassessment does **not** authorize implementation.

The immediate next gate is a dedicated design that defines the smallest coherent journey:

~~~text
registered Monitor
  -> Go decides work is due
  -> Rust receives one bounded work item
  -> Rust executes a safe HTTP/HTTPS probe
  -> Rust submits one normalized result
  -> Go persists the result
  -> repository evidence proves the journey
~~~

The design must preserve the current ownership rule:

~~~text
Go   = product truth, scheduling/coordination, persistence
Rust = bounded network execution
~~~

Rust must not access PostgreSQL.

---

## 5. Design blockers that must be resolved before implementation planning

### 5.1 Work identity and result idempotency

The design must define a stable work/check identity before a result crosses the process boundary.

It must answer:

- who generates the check/work ID;
- how a retried result submission avoids duplicate durable results;
- what happens when the checker crashes after receiving work;
- whether duplicate execution is tolerated in the single-checker milestone.

Do not introduce a broker merely to solve this.

### 5.2 Scheduling ownership and due-work semantics

Go owns scheduling/coordination.

The design must decide the minimum first-slice policy:

- fixed server-owned cadence vs persisted policy;
- how "due" is calculated;
- whether a durable next-due marker is required;
- how work retrieval prevents a tight loop from issuing the same due monitor repeatedly.

User-configurable intervals are not automatically in scope.

### 5.3 Internal contract shape

The design must define the first authoritative:

~~~text
contracts/openapi/internal.yaml
~~~

only after the work/result semantics are settled.

The contract must be intentionally internal and must not become a browser API.

At minimum it must cover:

- work acquisition;
- work identity;
- monitor/target identity needed by execution;
- execution budget fields;
- normalized result submission;
- deterministic error/status behavior.

The design must avoid copying Go database rows or Rust implementation structs into the contract.

### 5.4 Minimal durable result model

The first execution slice needs durable result evidence.

The design must decide the smallest schema addition, likely around:

~~~text
monitoring.check_runs
~~~

It must explicitly decide whether a separate durable schedule/state structure is needed now.

Do not add incidents, notifications, status pages, or a generalized event store.

### 5.5 HTTP probe security boundary

Before Rust performs user-directed network access, the design must specify enforceable execution rules for at least:

- HTTP/HTTPS only;
- DNS resolution validation;
- loopback rejection;
- private/link-local/reserved address rejection;
- cloud metadata protection;
- redirect destination revalidation;
- redirect-count bound;
- DNS rebinding/connection-time address policy;
- explicit target-port policy;
- request timeout;
- response-byte bound;
- bounded concurrency.

Tests must use controlled local servers and deterministic address fixtures rather than external internet dependencies.

### 5.6 Result vocabulary

The result model must be normalized enough that Go does not depend on Rust HTTP-client implementation details.

The design must distinguish product-relevant facts such as:

- success/failure;
- HTTP status when available;
- latency/duration;
- normalized failure category;
- checked-at timestamps.

Raw library error strings must not cross the internal contract.

### 5.7 Public observability boundary

The execution milestone must decide how much result state becomes observable to product callers.

Two acceptable design outcomes are possible:

1. persist and verify results internally while keeping the public contract unchanged; or
2. add the minimum public status/history representation required to prove a useful product journey.

This must be an explicit design decision. It must not appear accidentally through DTO reuse.

### 5.8 Local runtime replacement

The design must specify how the current Checker placeholder is replaced without destabilizing the canonical topology.

Constraints:

- keep the existing four-service topology if practical;
- no broker;
- no Kubernetes;
- no direct Rust -> PostgreSQL;
- checker health/lifecycle must be real;
- Docker smoke must prove real Go -> Rust -> controlled target -> Go behavior.

---

## 6. Explicitly deferred from the selected milestone

Unless the design proves one is strictly required, keep these out:

- multi-checker distribution;
- leases designed for horizontal worker fleets;
- Kafka/RabbitMQ/NATS/Redis work queues;
- transactional outbox;
- incidents;
- notifications;
- status pages;
- accounts/teams/multi-tenancy;
- billing;
- browser authentication;
- public host exposure;
- configurable retry policies;
- user-configurable monitoring intervals;
- TCP/DNS/TLS/ICMP probes;
- React implementation;
- generalized event sourcing/CQRS.

The first slice should prove one safe HTTP/HTTPS monitoring loop, not every future capability.

---

## 7. Why this sequencing is preferred

The selected order is:

~~~text
current create/read runtime
  -> execution-slice design
  -> reviewed implementation plan
  -> implementation
  -> result/status product reassessment
  -> Web/public-exposure reassessment
~~~

This sequencing prevents three common failures:

1. **UI-first incompleteness** — building presentation before useful monitoring state exists;
2. **security-late execution** — performing outbound user-directed requests before SSRF/resource rules are designed;
3. **distributed-infrastructure drift** — introducing queues/leases/brokers before a single checker has proven the workload.

---

## 8. Gate acceptance criteria

This scope reassessment is GREEN only if review agrees that:

1. the current highest-value gap is missing check execution;
2. Single-Checker Execution Vertical Slice is the next design target;
3. this document authorizes no Rust/Go/schema/contract implementation;
4. internal contract design follows product/work semantics rather than preceding them;
5. Go retains scheduling/coordination and persistence ownership;
6. Rust remains execution-only and never accesses PostgreSQL;
7. security rules are designed before outbound probing;
8. multi-worker/broker/public-exposure/UI breadth remain deferred;
9. the next repository change after this gate is a dedicated execution-slice design document.

---

## 9. Self-review

### Product value

PASS.

This direction closes the largest gap between the repository's current target registry and its intended uptime-monitoring behavior.

### Architecture

PASS.

It follows the already-committed Control Plane / Execution Plane split rather than creating a new architectural direction.

### Security

PASS at scope level.

The selected milestone makes execution security a design prerequisite instead of a post-implementation hardening task.

### YAGNI

PASS.

The decision explicitly rejects multi-worker coordination, brokers, broad lifecycle CRUD, and infrastructure expansion until the first checker loop proves them necessary.

### Contract discipline

PASS.

`contracts/openapi/internal.yaml` is not created by this reassessment. Its shape must follow reviewed work/result semantics.

### Persistence discipline

PASS.

This document does not add or modify SQL migrations. Any result/schedule schema is deferred to the dedicated design and later implementation plan.

### Execution safety

PASS.

This branch must contain only this reassessment document. No runtime, contract, schema, Compose, or CI implementation is authorized here.

---

## 10. Explicit STOP boundary

After this document is reviewed and landed:

~~~text
NEXT:
Single-Checker Execution Vertical Slice design
~~~

Do not begin implementation planning or runtime code until that design gate is separately opened and reviewed.
