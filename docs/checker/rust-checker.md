# Rust Checker

**Runtime state:** Implemented for the Single-Checker Execution Vertical Slice.

Rust Checker: implemented.

## Responsibility

The Rust Checker is the bounded Execution Plane. Go remains the Control Plane and owns scheduling truth, product semantics, and durable state.

Rust never accesses PostgreSQL directly.

The production Checker:

- claims at most one work item per claim request;
- performs bounded HTTP/HTTPS execution;
- normalizes protocol/network outcomes into the closed result vocabulary;
- submits the normalized result to Go;
- owns no durable product state.

## Workspace Boundaries

The workspace under `apps/checker` is split into four narrow crates:

~~~text
checker-core          worker orchestration, ports, work/result abstractions
probe-http            destination policy + bounded HTTP/HTTPS probe adapter
control-plane-client  internal HTTP transport and contract mapping
checker               production process composition, lifecycle, readiness/logging
~~~

Dependency direction is inward toward `checker-core`. The core does not depend on concrete HTTP transport, PostgreSQL, or Go implementation code.

## Internal Control-Plane Contract

The authoritative internal contract is `contracts/openapi/internal.yaml` and contains exactly:

~~~text
POST /internal/checks/claim
PUT /internal/checks/{checkId}/result
~~~

A claim returns either one `CheckWork` or no work. The work item carries the Go-owned CheckID/MonitorID, target URL, a fixed 10-second probe timeout, and a fixed maximum of three redirects.

Rust may submit these normalized result kinds:

~~~text
http_response
dns_error
policy_rejected
timeout
connect_error
tls_error
protocol_error
internal_error
~~~

`worker_timeout` is Go-owned and is never submitted by Rust.

Raw library errors, headers, response bodies, resolved IP addresses, stack traces, and credentials do not cross this contract.

## Worker Loop

The production worker uses fixed milestone policy:

~~~text
maximum active probes      4
no-work delay              1 second
ambiguous claim pause      20 seconds
claim backoff              250ms -> 500ms -> 1s
result retry backoff       250ms -> 500ms
~~~

Claims are serial while filling available execution slots.

Each claimed CheckID is probed exactly once. The canonical result payload is prepared once and may be retried only for bounded transport/deadline delivery failures; retries do not re-run the probe.

An ambiguous claim transport failure or claim deadline pauses further claims for 20 seconds because Go may already have committed a pending CheckRun.

## Production Destination Policy

Production destination policy rejects private and non-public addresses.

Execution is deny-by-default:

- only HTTP and HTTPS are accepted;
- only default ports are accepted: HTTP 80 and HTTPS 443;
- loopback, private, link-local, unique-local, multicast, reserved, documentation, benchmarking, and other non-global ranges are rejected;
- IPv4-mapped IPv6 is evaluated as the effective IPv4 address;
- a hostname is rejected if any resolved address is forbidden;
- DNS resolution happens once per request/redirect hop;
- connection attempts are bound to that validated address set;
- connect-time DNS re-resolution is not used;
- direct connections are used; ambient proxy configuration is ignored.

For multiple validated addresses, fallback is allowed only after TCP establishment failure. TLS, protocol, and HTTP terminal outcomes do not trigger address fallback.

## Redirects, TLS, and Resource Bounds

Every redirect destination is fully re-parsed, re-resolved, and revalidated.

The probe enforces:

- at most three redirects;
- HTTPS -> HTTP downgrade rejection;
- original authority preservation for HTTP Host, TLS SNI, and hostname verification;
- ordinary certificate and hostname verification;
- no skip-verify mode;
- 10-second overall probe timeout;
- response headers bounded to 64 KiB;
- response body not application-consumed.

There is no production private-network bypass.

## Lifecycle and Health

Production composition is in `apps/checker/crates/checker`.

The process reads the internal Go endpoint from:

~~~text
UPTIME_LAB_CONTROL_PLANE_URL=http://api:8080
~~~

The Checker health contract uses:

~~~text
/run/uptime-lab/ready
~~~

The readiness marker is written only after the worker loop emits `worker_started`. Readiness means configuration and runtime dependencies were constructed and the worker loop started; it does not mean a monitored target is reachable.

Stable worker events include only relevant fields such as:

~~~text
event
check_id
monitor_id
result_kind
duration_ms
~~~

## Persistence and Exposure Boundaries

Go exclusively owns durable product state.

Rust never accesses PostgreSQL directly.

The Checker does not publish a public product endpoint, and canonical Compose publishes no application host ports.

No application host ports are published.

No public CheckRun history or derived availability/status API exists.

## Current Limitations

This milestone intentionally remains narrow:

- one logical Checker process;
- no worker identity or lease protocol;
- no broker/outbox;
- no private-network monitoring;
- no proxy-enabled production probe;
- no mutable Monitor lifecycle;
- no public CheckRun history or derived availability/status surface;
- no React runtime;
- no public ingress/authentication/CORS/rate-limit policy.

These are separate product/design gates, not implied next steps.

## Verification

See [Single-Checker Execution Slice Testing](../testing/single-checker-execution-slice.md) for the evidence layers covering the internal contract, Go persistence/scheduling, Rust architecture/security, worker behavior, Docker cross-runtime execution, and vulnerability audit.

Related architecture:

- [Container View](../architecture/container-view.md)
- [Runtime Flows](../architecture/runtime-flows.md)
- [Dependency Rules](../architecture/dependency-rules.md)
- [Data Ownership](../architecture/data-ownership.md)
