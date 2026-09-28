# Single-Checker Execution Slice Testing

## Purpose

This guide is the canonical verification map for the first real Go/Rust execution loop.

The implemented runtime path is:

~~~text
registered Monitor
  -> Go claim/scheduling
  -> Rust Checker
  -> bounded destination policy/probe
  -> normalized result submission
  -> Go completion
  -> PostgreSQL terminal CheckRun
~~~

The canonical private-target Docker evidence is summarized as:

~~~text
Go -> Rust -> policy_rejected -> Go -> PostgreSQL
~~~

Each layer owns different evidence. Passing one layer does not substitute for another.

## Evidence Layers

| Layer | Primary evidence |
|---|---|
| Internal OpenAPI contract | Redocly + repository semantic checker |
| Go domain/application | unit tests + race tests |
| Go execution persistence | real PostgreSQL integration |
| Go production composition | real PostgreSQL `cmd/api` integration |
| Rust dependency boundaries | Rust architecture fitness |
| Destination security policy | deterministic resolver/address-policy tests |
| HTTP/HTTPS probe behavior | Rust probe tests + local loopback/TLS fixtures |
| Control-plane client | contract mapping + transport/cancellation tests |
| Worker orchestration | deterministic worker/lifecycle tests |
| Cross-runtime execution | Canonical Docker smoke |
| Rust dependency security | `cargo audit` |
| Go dependency security | `govulncheck` |
| Aggregate branch state | path-aware CI + `CI / gate` |

## Internal Contract

`contracts/openapi/internal.yaml` is verified separately from runtime implementation.

It must expose exactly:

~~~text
POST /internal/checks/claim
PUT /internal/checks/{checkId}/result
~~~

Contract fixtures protect CheckWork and normalized result compatibility across Go and Rust.

The public contract remains a separate source at `contracts/openapi/public.yaml` and is not expanded with CheckRun/status/history operations.

## Go Unit and Application Evidence

Go tests protect:

- CheckID validation and UUID semantics;
- normalized result invariants;
- fixed cadence/timeout/redirect policy inputs;
- due-work orchestration;
- Go server-time ownership;
- stable no-work/not-found/conflict/persistence errors;
- exact duplicate completion semantics;
- late result -> `worker_timeout` ownership.

Fresh race verification is required:

~~~bash
cd apps/api
go test -count=1 -race ./...
~~~

## PostgreSQL Integration

Real PostgreSQL tests protect:

- additive `monitoring.check_runs` schema;
- pending/terminal row-shape constraints;
- one pending CheckRun per Monitor;
- due ordering and atomic claim;
- expired-pending reconciliation;
- exact duplicate completion;
- conflicting/late result behavior;
- migration up/down/up;
- schema-aware readiness.

Rust is not part of this database layer. Rust never connects to PostgreSQL.

## Rust Architecture and Policy

Architecture fitness protects crate dependency direction and rejects database ownership in Rust.

Destination-policy tests cover public/non-public address classification, mixed DNS answer rejection, IPv4-mapped IPv6 normalization, default-port policy, redirect validation, downgrade rejection, and validated-address binding.

The production policy has no private-network bypass.

## Rust Probe Integration

Probe tests use controlled local fixtures to verify transport behavior without weakening production policy.

Evidence includes:

- HTTP response classification;
- TLS certificate and hostname verification;
- bounded redirects;
- TCP-only address fallback;
- protocol/TLS terminal behavior;
- response-header bound;
- overall timeout;
- response body not application-consumed;
- ambient proxy configuration ignored.

## Control-Plane Client

Client tests protect:

- claim 200/204 mapping;
- invalid/rejected response classification;
- canonical result serialization;
- result PUT semantics;
- fixed operation deadline;
- shutdown cancellation;
- ambiguous transport classification.

Transport tests do not substitute for the Docker cross-runtime path.

## Worker Loop and Lifecycle

Worker tests prove:

- maximum four active probes;
- serial claims;
- slot reuse;
- no-work delay;
- bounded claim backoff;
- 20-second ambiguous-claim pause;
- result retries reuse the same prepared result;
- a result retry never re-runs the probe;
- shutdown cancels new work and bounded transport operations.

Lifecycle tests protect the Checker readiness marker and stable event formatting.

## Canonical Docker Smoke

The Canonical Docker smoke is the cross-runtime acceptance layer.

It runs the real four-service Compose topology:

~~~text
web      placeholder
db       PostgreSQL
api      real Go Control Plane
checker  real Rust Checker
~~~

A Monitor is registered with the container-local target:

~~~text
http://web/
~~~

Because Docker DNS resolves that hostname to a private container-network address, the production destination policy must return `policy_rejected` without a test-only bypass or internet dependency.

The smoke bounded-polls PostgreSQL and verifies:

- exactly one terminal CheckRun for the Monitor;
- zero pending CheckRuns;
- canonical CheckID;
- `result_kind = policy_rejected`;
- `http_status IS NULL`;
- bounded integer `duration_ms`;
- `completed_at IS NOT NULL`.

For the same CheckID/MonitorID, Checker logs must show strict order:

~~~text
check_claimed
-> probe_completed(result_kind=policy_rejected)
-> result_delivered
~~~

The same smoke retains the existing bootstrap/restart/reset evidence:

- API live but unready before migration;
- Goose metadata absent before explicit migration;
- explicit migration required;
- real Checker healthy;
- Monitor registration/read;
- normal restart preserves state;
- destructive volume reset removes state;
- re-migration is required after reset.

The fake-Docker shell harness verifies control flow and diagnostics only. It does not replace the real Docker run.

## Vulnerability Audits

Rust CI runs:

~~~bash
cd apps/checker
cargo audit --file Cargo.lock
~~~

Go CI runs the pinned `govulncheck` command outside the application dependency graph.

A finding is a failing verification gate until independently assessed.

## Useful Commands

Repository documentation fitness:

~~~bash
./scripts/ci/test-architecture-docs.sh
./scripts/ci/check-architecture-docs.sh .
~~~

Go:

~~~bash
cd apps/api
go test ./...
go test -count=1 -race ./...
go test -count=1 -tags=integration ./migrations
go test -count=1 -tags=integration ./internal/modules/monitoring/adapters/postgres
go test -count=1 -tags=integration ./cmd/api
~~~

Rust:

~~~bash
cd apps/checker
cargo fmt --all --check
cargo check --workspace --all-targets --locked
cargo clippy --workspace --all-targets --all-features --locked -- -D warnings
cargo test --workspace --all-targets --locked
cargo audit --file Cargo.lock
~~~

Docker:

~~~bash
./scripts/ci/test-smoke-local-dev.sh
./scripts/ci/smoke-local-dev.sh
~~~

## Non-Claims

This evidence does not claim:

- public network deployment readiness;
- authentication/authorization;
- mutable Monitor lifecycle;
- public CheckRun/status/history API;
- multi-worker coordination;
- broker/outbox behavior;
- private-network monitoring;
- React UI behavior.

Those remain outside this milestone.
