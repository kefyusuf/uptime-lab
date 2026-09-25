# Public Monitoring Contract Testing

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
GET  /monitors/{monitorId}
~~~

The Go runtime serves these two operations through the Monitoring HTTP adapter in addition to `GET /livez` and schema-aware `GET /readyz`. Canonical Compose publishes no application host ports, so this does not imply public network deployment. The internal Checker contract remains undefined and unimplemented.

## Verification Layers

Verification is deliberately layered:

1. Redocly CLI validates OpenAPI syntax/spec conformance and bundles references.
2. `scripts/ci/check-public-contract.mjs` owns repository-specific semantic invariants over the bundled JSON.
3. `scripts/ci/test-check-public-contract.mjs` exercises one canonical positive fixture and the negative invariant matrix.
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
- exact request, Monitor, and Problem schema surfaces;
- explicit absence of an RFC 3986 `uri` format on `targetUrl`, plus locked UUID, date-time, and URI-reference formats;
- absence of servers and security schemes;
- absence of speculative `contracts/openapi/internal.yaml`;
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
- the future internal Checker contract;
- generated clients or SDKs.

Those concerns require separate runtime/product gates.

## Contract Verification vs Go Transport Conformance

The public contract is **defined and served**, but the evidence layers remain intentionally separate.

A green `public-contract` job means the source artifact is valid and matches repository-owned semantic invariants. Runtime conformance is proven by Monitoring HTTP adapter tests, real PostgreSQL production-composition integration, schema-aware readiness evidence, and canonical Docker POST/GET smoke.

This separation keeps `contracts/openapi/public.yaml` authoritative without making the OpenAPI parser responsible for Go routing or persistence behavior.

## Change Detection

The public-contract detector returns `true` for changes under `contracts/openapi/`, its detector/checker scripts, and the shared CI workflow. It returns `false` for unrelated docs-only, Go-only, and Compose-only changes.

A zero base SHA or unavailable base is handled conservatively as changed. An unavailable head is an error.

The detector intentionally includes future/forbidden files under `contracts/openapi/` so a speculative internal contract cannot bypass verification.

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
