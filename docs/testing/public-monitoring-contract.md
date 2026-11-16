# Public Monitoring Contract Testing

Current availability is exposed by `GET /monitors/{monitorId}/availability` as a read-only assessment of the bounded recent terminal sequence (newest-first, at most five rows). It returns `status`, `reason`, UTC `evaluatedAt`, and terminal `evidence` (`checkId`, `completedAt`). A known Monitor without a terminal result returns `200 unknown/no_result` with evidence omitted; the raw latest-result route retains its empty `204`. Every matched availability response, including errors and `405`, uses `Cache-Control: no-store`; `HEAD` returns `405` with `Allow: GET`. No new persistence, reconciliation, history, or public deployment is introduced.

## Purpose

This document is the canonical verification guide for the public Monitoring OpenAPI source artifact. It verifies the contract as a repository artifact. The Go runtime now serves the contract, but runtime transport conformance is verified separately in [go-public-transport-adapter.md](go-public-transport-adapter.md).

## Authoritative Source

The authoritative source is:

~~~text
contracts/openapi/public.yaml
~~~

The milestone contract is OpenAPI 3.1.2 with `info.version: 0.1.0` and exactly:

~~~text
POST /monitors
GET  /monitors
GET  /monitors/{monitorId}
GET  /monitors/{monitorId}/latest-result
GET  /monitors/{monitorId}/availability
GET  /monitors/{monitorId}/scheduling
PUT  /monitors/{monitorId}/scheduling
DELETE /monitors/{monitorId}
GET  /monitors/{monitorId}/history
PATCH /monitors/{monitorId}
GET  /monitors/{monitorId}/incidents
~~~

The Go runtime serves these eleven public operations through the Monitoring HTTP adapter in addition to `GET /livez` and schema-aware `GET /readyz`. The latest-result operation is terminal-only: `204` means a known Monitor has no terminal result, pending CheckRuns are not exposed, and the result vocabulary is an execution fact rather than an availability verdict. Canonical Compose publishes no application host ports, so this does not imply public network deployment. The internal Checker contract remains separate at `contracts/openapi/internal.yaml`.

Inventory regression checks pin GET+POST collection, the closed required items/nullable nextCursor envelope, limit default20/min1/max50, cursor88 characters,128 raw query bytes,245760 body bytes, no-store and405 Allow GET, POST. Page/empty/final fixtures preserve complete Monitor fields; empty pages cannot continue and fixture continuation equals the last included persisted key. Mutation tests retain existing operation checks. Run pinned Redocly lint/bundle before the bundled checker and regenerate/check TypeScript consumer types.

## Verification Layers

Verification is deliberately layered:

1. Redocly CLI validates OpenAPI syntax/spec conformance and bundles references.
2. `scripts/ci/check-public-contract.mjs` owns repository-specific semantic invariants over the bundled JSON.
3. `scripts/ci/test-check-public-contract.mjs` exercises the canonical contract, the public fixtures (including three latest-result and ten availability variants, and five incident variants — open, resolved, null-evidence, empty-page and invalid), and the negative invariant matrix; 183 semantic tests pass.
4. `scripts/ci/detect-public-contract-changes.sh` decides whether the path-aware CI job must run.
5. `CI / gate` accepts the conditional job only when it is either successful or legitimately skipped.

The repository-owned semantic checker does not parse YAML and does not replace generic OpenAPI validation.

## Exact Tool Model

CI uses:

~~~text
Node.js 24.21.0
@redocly/cli@2.53.3
OpenAPI 3.1.2
~~~

Redocly is executed through exact-version `npx`; no root `package.json`, npm lockfile, or generated API documentation is required.

CI disables Redocly telemetry and update notices:

~~~text
REDOCLY_TELEMETRY=off
REDOCLY_SUPPRESS_UPDATE_NOTICE=true
~~~

## Local Verification

From the repository root:

~~~bash
(
set -euo pipefail
tmp="$(mktemp)"
trap 'rm -f "$tmp"' EXIT

REDOCLY_TELEMETRY=off REDOCLY_SUPPRESS_UPDATE_NOTICE=true \
  npx --yes @redocly/cli@2.53.3 lint --extends=spec contracts/openapi/public.yaml

REDOCLY_TELEMETRY=off REDOCLY_SUPPRESS_UPDATE_NOTICE=true \
  npx --yes @redocly/cli@2.53.3 bundle contracts/openapi/public.yaml \
    --output "$tmp" --ext json

node scripts/ci/test-check-public-contract.mjs
node scripts/ci/check-public-contract.mjs "$tmp" .
./scripts/ci/test-detect-public-contract-changes.sh
)
~~~

Documentation fitness is verified separately:

~~~bash
./scripts/ci/test-architecture-docs.sh
./scripts/ci/check-architecture-docs.sh .
~~~

## Semantic Checker Ownership

The repository-owned checker mechanically protects the landed contract decisions, including:

- exact OpenAPI and contract versions;
- exact public path and operation set;
- exact operation IDs;
- request/response media and status sets;
- `Location` header shape;
- exact request, Monitor, Problem, and latest-result variant schema surfaces;
- exact `200/204/400/404/500` latest-result response semantics, including an empty `204`;
- exact `200/400/404/500` availability semantics and `Cache-Control: no-store` on every response;
- closed availability variants with valid status/reason pairs, required terminal evidence, and absent evidence for `no_result`;
- exact latest-result/availability fixtures, including all ten availability reasons, nested evidence UUIDs and UTC timestamps;
- exact `200/400/404/500` incident semantics, `Cache-Control: no-store` on every response, and `405` with `Allow: GET`;
- closed `MonitorIncidentPage`/`MonitorIncident` shapes with the two-reason `openingReason` enum (`unexpected_http_status`/`probe_failure`), optional `resolvedAt`/`openedByCheckId`/`resolvedByCheckId` omitted when null, default 20/max 50 limit, and the version-3 88-character base64url cursor;
- five exact incident fixtures (open, resolved, null-evidence, empty-page, invalid);
- absence of `httpStatus` on classified failures and both `httpStatus`/`durationMs` on `worker_timeout`;
- explicit absence of an RFC 3986 `uri` format on `targetUrl`, plus locked UUID, date-time, and URI-reference formats;
- absence of servers and security schemes;
- absence of a public UUID-version guarantee.

The negative harness is intentionally network-independent and uses Node standard-library modules only.

## What This Does Not Validate

Contract verification intentionally does not validate:

- Go transport conformance or handler routing (owned by the runtime adapter test layer);
- runtime request/response serialization (owned by the runtime adapter/integration layer);
- production MonitorID generator composition;
- migration/schema compatibility at readiness;
- authentication, authorization, or CORS;
- public host/network exposure;
- SSRF or probe-execution safety;
- the independently versioned internal Checker contract;
- generated clients or SDKs.

Those concerns require separate runtime/product gates.

## Contract Verification vs Go Transport Conformance

The public contract is **defined and served**, but the evidence layers remain intentionally separate.

A green `public-contract` job means the source artifact is valid and matches repository-owned semantic invariants. Runtime conformance is proven by Monitoring HTTP adapter tests, real PostgreSQL production-composition integration, schema-aware readiness evidence, and canonical Docker POST/GET/latest-result/availability smoke. Domain tests pin HTTP boundaries, every normalized result kind, future-before-stale precedence, the unrounded 120-second boundary, and the newest-first sequence policy (N=3 confirmation, M=2 recovery, the inconclusive reset boundary, walked-prefix-only validation, and freshness applied only to the latest row). Application tests pin one bounded recent read followed by one clock sample, immutable evidence, and safe UTC timestamp representation. The durable incident fold is pinned separately by domain tests (N=3 open, M=2 resolve, inconclusive reset, never re-open, first-success no-op) and real-PostgreSQL write-path integration; the bounded incident read is pinned by application/adapter tests (version-3 cursor canonicality, closed shape, null-evidence omission, keyset ordering) and the composed browser journey. Availability remains assessment-only and is never derived from incident state.

This separation keeps `contracts/openapi/public.yaml` authoritative without making the OpenAPI parser responsible for Go routing or persistence behavior.

## Change Detection

The public-contract detector returns `true` for changes to `contracts/openapi/public.yaml`, its detector/checker scripts, and the shared CI workflow. It returns `false` for unrelated docs-only, Go-only, and Compose-only changes.

A zero base SHA or unavailable base is handled conservatively as changed. An unavailable head is an error.

The public-contract detector owns `contracts/openapi/public.yaml` and its public semantic checker/fixtures. The internal contract has a separate detector/job, so an `internal.yaml`-only change does not masquerade as a public-contract change.

## CI Semantics

The `public-contract` GitHub Actions job:

- checks out the exact pull-request head;
- asserts the checked-out revision;
- uses Node.js 24.21.0;
- verifies Redocly CLI 2.53.3;
- lints and bundles `contracts/openapi/public.yaml`;
- runs the semantic negative harness;
- runs the semantic checker over bundled JSON.

The stable aggregate job remains `CI / gate`.

When public-contract detection is true, `public-contract` must succeed. When detection is false, the job may be skipped and the aggregate gate accepts that skipped result. Documentation fitness remains owned by the repository/documentation checks rather than by the OpenAPI parser.

For live runtime evidence, see [go-public-transport-adapter.md](go-public-transport-adapter.md).
