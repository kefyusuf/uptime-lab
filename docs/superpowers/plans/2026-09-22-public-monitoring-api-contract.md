# Public Monitoring API Contract Implementation Plan

**Status:** Review candidate
**Date:** 2026-09-22
**Repository:** `kefyusuf/uptime-lab`
**Scope:** Implement the landed Public Monitoring API Contract design as a first-class OpenAPI artifact with deterministic validation, semantic fitness checks, path-aware CI, canonical documentation, and controlled landing
**Base:** `main@676997eec698f257a835ae5d2895513a6bc4d56b`
**Authoritative design:** `docs/superpowers/specs/2026-09-22-public-monitoring-api-contract-design.md`

---

## 1. Purpose

The Public Monitoring API Contract design is landed.

This plan implements that design without implementing the Go public transport adapter.

The implementation phase will create the first public product contract artifact:

```text
contracts/openapi/public.yaml
```

and mechanically verify that it describes only the already-existing Monitoring application capability:

```text
POST /monitors
GET  /monitors/{monitorId}
```

The phase also establishes repository-owned contract fitness functions and a path-aware `public-contract` CI job.

The implementation MUST NOT expose the contract through the Go runtime yet.

---

## 2. Existing Authority

The following decisions are already landed and are not reopened by this plan:

- OpenAPI baseline is 3.1.2.
- Public and internal contracts remain separate.
- Public paths are exactly:
  - `POST /monitors`;
  - `GET /monitors/{monitorId}`.
- The Monitor representation contains exactly:
  - `id`;
  - `targetUrl`;
  - `createdAt`.
- Create input contains only required `targetUrl`.
- Duplicate target URLs remain valid.
- Registration remains non-idempotent.
- Successful registration returns `201 Created` and `Location`.
- Successful lookup returns `200 OK`.
- Errors use RFC 9457 Problem Details.
- Malformed JSON / invalid textual monitor ID -> 400.
- Unsupported request media type -> 415.
- Valid JSON that violates the create/domain contract -> 422.
- Monitor not found -> 404.
- Current generic application persistence failure -> 500.
- Authentication, authorization, CORS, rate limiting, lifecycle, scheduling, results, history, incidents, Rust, and Web remain deferred.
- Product handler readiness must eventually include schema compatibility, but this contract-artifact phase does not implement that runtime behavior.
- Go transport implementation occurs only after this contract artifact lands.

---

## 3. Phase Boundary

### In scope

- `contracts/openapi/public.yaml`;
- OpenAPI 3.1.2 structural/spec validation;
- deterministic reference resolution/bundling verification;
- repository-owned semantic contract fitness checks;
- negative semantic fixtures;
- path-aware public-contract change detection;
- a conditional `public-contract` GitHub Actions job;
- stable `CI / gate` integration;
- removal of the obsolete local-dev rule that forbids all root `contracts/`;
- continued prohibition of speculative `contracts/openapi/internal.yaml`;
- canonical documentation updates;
- whole-branch verification;
- external review;
- squash landing;
- fresh post-merge `main` CI.

### Out of scope

- Go HTTP product handlers;
- Monitoring runtime wiring in `cmd/api`;
- schema-readiness runtime implementation;
- migration/schema changes;
- public host-port exposure;
- `contracts/openapi/internal.yaml`;
- Rust source;
- React source;
- generated clients;
- SDK generation;
- root `package.json`;
- root Node workspace;
- npm lockfile;
- API mocking server;
- auth/authz;
- CORS;
- lifecycle/scheduler/result/history behavior.

---

## 4. Verification Tooling Decision

### 4.1 Redocly CLI

Use Redocly CLI for OpenAPI parsing, spec linting, and reference-resolving bundle validation.

Reviewed version at plan time:

```text
@redocly/cli 2.53.3
```

The implementation command is pinned exactly:

```bash
npx --yes @redocly/cli@2.53.3 lint --extends=spec contracts/openapi/public.yaml
```

and bundling is used to produce one canonical bundled JSON representation for repository-owned semantic checks. The bundle may retain internal component `$ref` values; the semantic checker therefore validates the bundled document structure and references rather than assuming full dereferencing.

Redocly's documented `spec` ruleset follows the OpenAPI specification, while the repository-owned checker enforces product-specific invariants.

Do not use `@latest`.

Plan-time sources:

- https://redocly.com/docs/cli/commands/lint
- https://redocly.com/docs/cli/guides/configure-rules
- https://redocly.com/docs/cli/changelog

Immediately before Task 1 edits CI, re-check the latest compatible Redocly 2.x patch. Patch-only updates may replace 2.53.3 when recorded in task evidence. A major-version change returns to plan review.

### 4.2 Node runtime

Redocly CLI 2.x requires Node 20.19+, 22.12+, or later.

Use an explicit CI-only Node runtime and do not add a repository Node package manifest.

Reviewed Node runtime at plan time:

```text
Node.js 24.21.0 LTS
```

Reviewed setup action:

```text
actions/setup-node@820762786026740c76f36085b0efc47a31fe5020 # v7.0.0
```

The job MUST set:

```yaml
package-manager-cache: false
```

and the contract-validation commands MUST run with:

```text
REDOCLY_TELEMETRY=off
REDOCLY_SUPPRESS_UPDATE_NOTICE=true
```

This disables optional telemetry and update-notice network behavior during verification.

There is no package manifest/lockfile to cache and the repository does not need npm dependency caching for one exact `npx` tool execution.

Immediately before Task 1, re-check the current supported Node 24 LTS patch and setup-node immutable SHA. Patch-only Node updates are allowed with evidence.

### 4.3 Why no package.json

The contract implementation does not become a frontend/Node application.

Using one exact CLI package through `npx` avoids:

- a fake root JavaScript project;
- package-lock churn;
- npm Dependabot ownership unrelated to an application runtime;
- coupling the OpenAPI contract to future React tooling.

No `package.json` or npm lockfile is introduced in this phase.

---

## 5. Target File Map

Expected implementation paths across the complete implementation branch:

### Contract source

```text
contracts/openapi/public.yaml
```

### Contract fitness / change detection

```text
scripts/ci/detect-public-contract-changes.sh
scripts/ci/test-detect-public-contract-changes.sh
scripts/ci/check-public-contract.mjs
scripts/ci/test-check-public-contract.mjs
```

### Existing phase guard updates

```text
scripts/ci/check-local-dev.sh
scripts/ci/test-check-local-dev.sh
```

### CI

```text
.github/workflows/ci.yml
```

### Canonical documentation

Expected as needed after explicit stale-text search:

```text
README.md
docs/README.md
docs/architecture/container-view.md
docs/architecture/dependency-rules.md
docs/architecture/runtime-flows.md
docs/backend/go-control-plane.md
docs/testing/public-monitoring-contract.md
scripts/ci/check-architecture-docs.sh
scripts/ci/test-architecture-docs.sh
```

No other runtime/application path is expected.

---

# Task 1 — Public OpenAPI artifact + spec validation + path-aware CI bootstrap

**Goal:** Introduce the first public contract and ensure the exact task head has real remote OpenAPI validation before advancing.

**Files:**

- Create: `contracts/openapi/public.yaml`
- Create: `scripts/ci/detect-public-contract-changes.sh`
- Create: `scripts/ci/test-detect-public-contract-changes.sh`
- Modify: `scripts/ci/check-local-dev.sh`
- Modify: `scripts/ci/test-check-local-dev.sh`
- Modify: `.github/workflows/ci.yml`

## Step 1 — Create the implementation branch

Create:

```text
feat/public-monitoring-api-contract
```

from the exact landed plan base after this plan PR has merged.

Do not reuse the design or plan branch.

## Step 2 — Re-check external pins

Before editing CI, record:

- current compatible Redocly 2.x patch;
- current supported Node 24 LTS patch;
- exact `actions/setup-node` immutable SHA;
- existing checkout SHA remains valid.

Major Redocly or Node baseline changes return to plan review.

## Step 3 — Write detector tests first

Create detector tests covering at minimum:

### Must return true

- `contracts/openapi/public.yaml`;
- any path under `contracts/openapi/`, including a future/forbidden `internal.yaml`, so the contract gate cannot be bypassed;
- `scripts/ci/detect-public-contract-changes.sh`;
- `scripts/ci/test-detect-public-contract-changes.sh`;
- `scripts/ci/check-public-contract.mjs` even though it lands in Task 2;
- `scripts/ci/test-check-public-contract.mjs`;
- `.github/workflows/ci.yml`;
- zero/base-unavailable conservative cases.

### Must return false

- ordinary docs-only change;
- unrelated root Markdown;
- Go source only;
- Compose-only change.

The detector has no network dependency.

## Step 4 — GREEN the detector

Implement the narrow path detector.

It prints exactly:

```text
true
```

or:

```text
false
```

and follows the existing Go/local-dev detector conventions for zero SHA, missing head, and missing base behavior.

## Step 5 — Evolve the old local-dev phase guard

The current local-dev checker still forbids the root `contracts` directory because that was correct before any contract artifact existed.

Remove only that obsolete blanket prohibition.

Keep all still-valid forbidden root paths, including:

```text
migrations
package.json
go.mod
go.work
Cargo.toml
```

Update local-dev checker tests so a canonical fixture containing:

```text
contracts/openapi/public.yaml
```

passes.

Do not make `check-local-dev.sh` responsible for validating OpenAPI semantics.

The new public-contract gate owns contract shape and the continued prohibition of speculative internal contract artifacts.

## Step 6 — Create the OpenAPI artifact

Create `contracts/openapi/public.yaml` with:

```yaml
openapi: 3.1.2
info:
  title: uptime-lab Public API
  version: 0.1.0
```

No `servers` entry is required because deployment/public exposure remains deferred.

The document contains exactly two paths:

```text
/monitors
  POST

/monitors/{monitorId}
  GET
```

Use stable operation IDs:

```text
registerMonitor
getMonitor
```

The initial component schemas are exactly:

```text
CreateMonitorRequest
Monitor
Problem
```

### CreateMonitorRequest

- object;
- required `targetUrl`;
- exactly one declared property: `targetUrl`;
- unknown request members rejected with `additionalProperties: false`;
- `targetUrl` is a string with `format: uri`;
- do not add a URL regex or enum that attempts to duplicate the Go parser;
- descriptions must explicitly preserve the domain boundary:
  - absolute HTTP/HTTPS;
  - hostname required;
  - no userinfo;
  - no fragment;
  - query/port allowed;
  - original accepted text preserved;
  - registration is not SSRF approval.

Do not encode a brittle regex that becomes stricter/different from the Go domain parser.

### Monitor

- object;
- required `id`, `targetUrl`, `createdAt`;
- exactly those three declared properties;
- `additionalProperties: false`;
- `id` string, `format: uuid`; the public contract MUST NOT promise a UUID version because the current domain accepts any non-zero UUID and no production Monitor composition currently binds a v7-only generator;
- `targetUrl` string, `format: uri`, preserving the accepted value;
- `createdAt` string, `format: date-time`, with the description stating that emitted values are UTC-normalized.

### Problem

Represent only the RFC 9457 base members selected by the design:

```text
type
title
status
detail
instance
```

Use these schemas:

- `type`: string, `format: uri-reference`;
- `title`: string;
- `status`: integer, minimum 100, maximum 599;
- `detail`: string;
- `instance`: string, `format: uri-reference`.

Do not add repository-specific validation extensions or numeric error codes.

The contract constrains its own emitted Problem object to those selected fields and does not define custom extension members in this milestone; no custom error envelope is introduced.

## Step 7 — Define operation responses exactly

### POST /monitors

Request:

```text
required requestBody
Content-Type: application/json
CreateMonitorRequest
```

Responses exactly:

```text
201
400
415
422
500
```

The 201 response:

- body: `application/json` -> `Monitor`;
- `Location` header points to `/monitors/{monitorId}`;
- `Location` uses a string schema with `format: uri-reference`;
- no 200 alternative;
- no 409 duplicate-target behavior.

Error responses use:

```text
application/problem+json
Problem
```

### GET /monitors/{monitorId}

The path parameter:

- name: `monitorId`;
- in: path;
- required: true;
- type: string;
- format: uuid.

Responses exactly:

```text
200
400
404
500
```

The 200 response body is `application/json` -> `Monitor`.

Errors use `application/problem+json` -> `Problem`.

## Step 8 — Keep security and deployment absent

The first OpenAPI document MUST NOT contain:

- root or operation `security`;
- `components.securitySchemes`;
- deployment-specific `servers`;
- CORS metadata;
- internal checker operations;
- webhook/callback surfaces;
- lifecycle/list/history endpoints.

## Step 9 — Add path-aware CI

Before adding the contract job, make PR checkout semantics explicit across the existing workflow.

Every `actions/checkout` step in `.github/workflows/ci.yml` MUST retain `fetch-depth: 0` and `persist-credentials: false`, and MUST set:

```yaml
ref: ${{ github.event.pull_request.head.sha || github.sha }}
```

This ensures pull-request jobs validate the contributor branch head rather than GitHub's synthetic merge revision. Push-to-main jobs continue to validate `github.sha`.

Do not change the existing base/head range semantics used for diff detection.

For jobs that make exact-head evidence claims, add a cheap post-checkout assertion:

```bash
test "$(git rev-parse HEAD)" = "$EXPECTED_HEAD_SHA"
```

with:

```text
EXPECTED_HEAD_SHA = github.event.pull_request.head.sha || github.sha
```

This exact-revision assertion must run before the job's substantive validation steps.

Then extend the `changes` job output with:

```text
public_contract
```

Run detector tests in the `changes` job.

Add conditional job:

```text
public-contract
```

The job:

1. checks out with `persist-credentials: false`;
2. sets up the exact reviewed Node runtime through immutable `actions/setup-node`;
3. disables package-manager caching;
4. asserts the expected Redocly version;
5. runs with `REDOCLY_TELEMETRY=off` and `REDOCLY_SUPPRESS_UPDATE_NOTICE=true`:
   ```bash
   npx --yes @redocly/cli@<exact> lint --extends=spec contracts/openapi/public.yaml
   ```
6. runs Redocly `bundle` with the same environment controls to a temporary JSON output so references must resolve.

No generated artifact is committed.

No `npm install`, package manifest, or lockfile is introduced.

## Step 10 — Update the aggregate gate

Retain the stable job identity:

```text
CI / gate
```

The gate MUST retain:

```yaml
if: ${{ always() }}
```

and MUST depend on exactly the relevant prerequisite jobs:

```text
policy
repository
changes
local-dev
go-api
public-contract
```

Its result logic must read all six `needs.*.result` values, including `needs.public-contract.result`.

Required matrix:

- `policy` = `success`;
- `repository` = `success`;
- `changes` = `success`;
- `local-dev` = `success` or `skipped`;
- `go-api` = `success` or `skipped`;
- `public-contract` = `success` or `skipped`.

The public-contract detector harness MUST cover both detector-true and detector-false diffs. The workflow review must verify that detector-false yields a skipped `public-contract` job that is accepted by the aggregate gate, while detector-true requires that job to succeed.

For this implementation branch, Task 1 must produce `public-contract = success`.

Because Task 1 changes `.github/workflows/ci.yml` and local-dev phase-guard scripts, expect Go/local-dev detection according to the existing detector rules; do not weaken those detectors merely to save CI.

## Step 11 — Local/cheap GREEN

At minimum:

```bash
bash -n scripts/ci/detect-public-contract-changes.sh
./scripts/ci/test-detect-public-contract-changes.sh
./scripts/ci/test-check-local-dev.sh
./scripts/ci/check-local-dev.sh .
git diff --check
```

If the executing local environment has a compatible Node runtime:

```bash
REDOCLY_TELEMETRY=off REDOCLY_SUPPRESS_UPDATE_NOTICE=true \
  npx --yes @redocly/cli@<exact> lint --extends=spec contracts/openapi/public.yaml
REDOCLY_TELEMETRY=off REDOCLY_SUPPRESS_UPDATE_NOTICE=true \
  npx --yes @redocly/cli@<exact> bundle contracts/openapi/public.yaml --output /tmp/uptime-lab-public-openapi.json --ext json
```

Do not claim local Redocly evidence if the runtime is unavailable.

## Step 12 — Self-review

Check:

- exactly two paths/two operations;
- no internal contract;
- no auth/security;
- no server/public exposure assumption;
- no list/update/delete/lifecycle;
- no schema/db change;
- no package manifest;
- local-dev guard changed only as required;
- stable gate semantics preserved, including `if: always()` and success/skipped handling for all conditional jobs;
- public-contract detector true/false cases are covered;
- pull-request checkout uses the explicit PR head SHA rather than the merge ref;
- exact-head assertions run before substantive validation;
- public Monitor ID is documented only as UUID, with no unsupported UUID-version guarantee;
- actions immutable-pinned;
- exact tool versions.

If RED, revise before commit.

## Step 13 — Commit

Use one coherent Task 1 commit:

```text
feat(contracts): add public monitoring contract
```

## Step 14 — Open draft implementation PR

Open:

```text
head: feat/public-monitoring-api-contract
base: main
title: feat(contracts): establish public monitoring API contract
draft: true
```

The PR body must state explicitly:

- contract artifact only;
- no Go transport implementation;
- no internal contract;
- no persistence change;
- no public deployment/auth claim.

## Step 15 — Require exact-head remote evidence

Task 1 does not close until the workflow run is tied to the current PR head SHA and every validating checkout uses that same SHA.

Required evidence:

1. GitHub PR metadata reports the current head SHA.
2. The workflow run `head_sha` equals that PR head SHA.
3. Required jobs log a successful exact-revision assertion after checkout.
4. The workflow file uses `ref: ${{ github.event.pull_request.head.sha || github.sha }}` for PR-capable checkout steps.

Only after those checks pass may the following job results be reported as exact-head evidence:

```text
policy           SUCCESS
repository       SUCCESS
changes          SUCCESS
public-contract  SUCCESS
CI / gate        SUCCESS
```

If `go-api` or `local-dev` are triggered by the changed CI/guard paths, they must also succeed.

Inspect the public-contract logs for:

- exact Node version;
- exact Redocly version;
- OpenAPI lint success;
- bundle/reference-resolution success.

STOP.

---

# Task 2 — Repository-owned semantic contract fitness + negative fixtures

**Goal:** Move beyond generic OpenAPI validity and mechanically enforce the landed product/architecture decisions.

**Files:**

- Create: `scripts/ci/check-public-contract.mjs`
- Create: `scripts/ci/test-check-public-contract.mjs`
- Modify: `.github/workflows/ci.yml`

## Step 1 — Define the checker boundary

The repository-owned checker consumes the canonical bundled JSON produced by Redocly `bundle`.

It does not parse YAML itself and does not assume Redocly fully dereferenced internal component references.

This intentionally separates:

```text
YAML/OpenAPI parsing + $ref resolution -> Redocly
product-specific invariants             -> repository-owned Node script
```

The checker uses only Node standard-library modules.

No npm dependency is added.

## Step 2 — Write negative fixtures first

The test harness creates temporary JSON fixture documents derived from a minimal valid contract model.

At minimum prove rejection of:

- OpenAPI version other than exactly 3.1.2;
- `info.version` other than exactly 0.1.0 for this milestone;
- extra public path;
- `GET /monitors`;
- PUT/PATCH/DELETE lifecycle mutation;
- missing POST /monitors;
- missing GET /monitors/{monitorId};
- wrong operation ID;
- wrong response status set;
- missing or non-URI-reference 201 Location header;
- missing/optional POST request body;
- targetUrl without `format: uri`;
- Monitor id without `format: uuid`;
- Monitor createdAt without `format: date-time`;
- wrong success media type;
- missing `application/problem+json` on an error response;
- extra request property;
- missing `additionalProperties: false` for request/Monitor shapes;
- extra Monitor field;
- missing required Monitor field;
- root security;
- operation security;
- `components.securitySchemes`;
- `servers`;
- extra component schema;
- speculative internal contract file under `contracts/openapi/internal.yaml`.

Also prove one canonical fixture passes.

## Step 3 — Implement semantic checker

The checker must mechanically assert:

### Document identity

```text
openapi == 3.1.2
info.version == 0.1.0
servers absent
security absent
```

### Exact path set

```text
/monitors
/monitors/{monitorId}
```

### Exact operations

```text
POST /monitors           -> registerMonitor
GET /monitors/{monitorId} -> getMonitor
```

### Exact schemas

```text
CreateMonitorRequest
Monitor
Problem
```

The checker also enforces the locked standard formats:

```text
CreateMonitorRequest.targetUrl -> uri
Monitor.id                     -> uuid (no public UUID-version guarantee)
Monitor.targetUrl              -> uri
Monitor.createdAt              -> date-time
POST 201 Location              -> uri-reference
Problem.type                   -> uri-reference
Problem.instance               -> uri-reference
Problem.status                 -> integer 100..599
```

### Exact operation/media/status contracts

Match the landed design and Task 1 artifact rules.

### Forbidden internal artifact

Fail if:

```text
contracts/openapi/internal.yaml
```

exists.

The checker must produce a clear invariant-specific error rather than a generic exception stack.

## Step 4 — Integrate into public-contract CI

After Redocly lint/bundle:

```bash
node scripts/ci/test-check-public-contract.mjs
node scripts/ci/check-public-contract.mjs "$RUNNER_TEMP/public-openapi.json" .
```

The task must not make the public-contract job depend on Go, PostgreSQL, or Docker.

## Step 5 — GREEN

At minimum:

```bash
node scripts/ci/test-check-public-contract.mjs
git diff --check
```

Canonical semantic check runs through the same bundled JSON route used in CI.

## Step 6 — Self-review

Check that the semantic checker:

- enforces design decisions, not arbitrary style preferences;
- does not duplicate OpenAPI parsing;
- has representative negative fixtures;
- cannot silently ignore an internal contract file;
- does not encode future transport features;
- remains deterministic and network-independent after Redocly has produced the bundle.

## Step 7 — Commit

```text
test(contracts): enforce public contract invariants
```

## Step 8 — Require exact-head remote evidence

Require:

```text
public-contract  SUCCESS
CI / gate        SUCCESS
```

Inspect logs for both semantic checker tests and canonical check.

STOP.

---

# Task 3 — Canonical documentation and documentation fitness

**Goal:** Make current documentation truthfully distinguish “public contract artifact exists” from “public runtime transport exists.”

**Files:**

- Modify: `README.md`
- Modify: `docs/README.md`
- Modify: `docs/architecture/container-view.md`
- Modify: `docs/architecture/dependency-rules.md`
- Modify: `docs/architecture/runtime-flows.md`
- Modify: `docs/backend/go-control-plane.md`
- Create: `docs/testing/public-monitoring-contract.md`
- Modify: `scripts/ci/check-architecture-docs.sh`
- Modify: `scripts/ci/test-architecture-docs.sh`

Update any other stale canonical implementation-state line found by explicit search, but do not rewrite unrelated historical design/spec/plan documents.

## Step 1 — Update root/current-state documentation

Docs must say:

- the public Monitoring OpenAPI contract is implemented as a source artifact;
- the Go API still exposes only `/livez` and `/readyz`;
- no public product handler is wired yet;
- internal Checker contract remains undefined/unimplemented;
- Rust/Web remain placeholders;
- auth/CORS/public exposure remain deferred.

Do not say “public API is implemented” when only its contract artifact exists.

## Step 2 — Update architecture relationships

Container/dependency/runtime-flow docs must distinguish:

```text
Public contract: defined
Go public transport adapter: deferred
Internal contract: deferred
```

The conceptual Create Monitor flow may now reference the concrete public contract operation semantically, but runtime execution remains conceptual until a Go handler exists.

Execute Due Check and result flows remain internal-contract conceptual only.

## Step 3 — Add testing/runbook documentation

Create `docs/testing/public-monitoring-contract.md`.

Document:

- authoritative source path;
- OpenAPI version;
- Redocly exact tool model;
- semantic checker ownership;
- detector behavior;
- exact local commands;
- what the checker intentionally does not validate;
- distinction between contract verification and Go transport conformance;
- CI job/gate semantics.

No generated HTML/API docs are required.

## Step 4 — Evolve documentation fitness

Require the new testing document through the existing documentation fitness layer.

Update fixture tests before changing the required-file list.

The documentation checker should also require the canonical docs index to link to the public contract/testing guide.

Do not make architecture-doc checks parse OpenAPI.

## Step 5 — Search for stale claims

Explicitly search for statements equivalent to:

- “no public product contract exists”;
- “OpenAPI remains entirely future/deferred”;
- “contracts directory is forbidden”.

Historical design/plan statements remain untouched when clearly scoped to their original phase.

Current canonical docs must be corrected.

## Step 6 — GREEN

At minimum:

```bash
./scripts/ci/test-architecture-docs.sh
./scripts/ci/check-architecture-docs.sh .
git diff --check
```

## Step 7 — Self-review

Check:

- documentation does not imply handlers exist;
- no auth/public exposure claim;
- internal contract still deferred;
- no stale phase-forbidden contract text in canonical docs;
- contract source remains authoritative;
- historical decision documents remain historical.

## Step 8 — Commit

```text
docs(contracts): document public monitoring contract
```

## Step 9 — Require exact-head remote evidence

Require repository/documentation checks GREEN and stable aggregate gate GREEN.

STOP.

---

# Task 4 — Whole-branch verification + implementation PR finalization

**Goal:** Verify the entire implementation branch against the landed design and this plan before requesting final external review.

No new product capability may be introduced in this task.

## Step 1 — Static whole-branch scope review

The complete `main...HEAD` diff may contain only approved contract/verification/docs paths.

Explicitly reject implementation under:

```text
apps/api/**
apps/web/**
apps/checker/**
migrations/**
compose.yaml
deploy/**
```

and root runtime manifests:

```text
package.json
package-lock.json
pnpm-lock.yaml
yarn.lock
Cargo.toml
go.mod
go.work
```

Explicitly reject:

```text
contracts/openapi/internal.yaml
```

Search for accidental capability terms in actual implementation:

```text
GET /monitors
PATCH /monitors
PUT /monitors
DELETE /monitors
enabled
updatedAt
check_runs
monitor_states
securitySchemes
oauth
bearer
apiKey
scheduler
due work
result submission
```

References explaining deferral are allowed.

## Step 2 — Run contract verification

Using the reviewed exact tool versions:

```bash
./scripts/ci/test-detect-public-contract-changes.sh
REDOCLY_TELEMETRY=off REDOCLY_SUPPRESS_UPDATE_NOTICE=true \
  npx --yes @redocly/cli@<exact> lint --extends=spec contracts/openapi/public.yaml
REDOCLY_TELEMETRY=off REDOCLY_SUPPRESS_UPDATE_NOTICE=true \
  npx --yes @redocly/cli@<exact> bundle contracts/openapi/public.yaml --output /tmp/uptime-lab-public-openapi.json --ext json
node scripts/ci/test-check-public-contract.mjs
node scripts/ci/check-public-contract.mjs /tmp/uptime-lab-public-openapi.json .
```

Record:

```bash
node --version
npx --yes @redocly/cli@<exact> --version
```

If local Node is unavailable, do not claim local results; exact-head `public-contract` CI logs are canonical evidence.

## Step 3 — Verify affected repository/local-dev fitness

Because this branch intentionally changes the old contract prohibition and CI graph:

```bash
./scripts/ci/test-check-local-dev.sh
./scripts/ci/check-local-dev.sh .
./scripts/ci/test-detect-local-dev-changes.sh
./scripts/ci/test-detect-go-api-changes.sh
./scripts/ci/test-governance.sh
./scripts/github/test-configure-governance.sh
./scripts/ci/test-architecture-docs.sh
./scripts/ci/check-architecture-docs.sh .
./scripts/ci/check-repository-shape.sh .
./scripts/ci/check-pr-title.sh "feat(contracts): establish public monitoring API contract"
git diff --check main...HEAD
```

Use the real current base SHA rather than a stale local `main` ref when necessary.

## Step 4 — Whole-branch self-review

Review the complete branch for:

- design exactness;
- only two operations;
- correct media/status mapping;
- RFC 9457 shape;
- no security/public exposure;
- no internal contract;
- no product runtime wiring;
- no persistence change;
- no root Node project;
- Redocly exact pin;
- Node/setup action exact pin;
- semantic negative fixtures;
- contract detector correctness;
- local-dev phase guard safely evolved;
- aggregate gate correctness;
- documentation truthfulness;
- no generated artifacts committed.

If RED, fix coherently and repeat all affected evidence.

## Step 5 — Finalize draft implementation PR

Update the existing implementation PR body with:

- Architecture impact;
- Contract impact;
- Runtime impact: none;
- Database impact: none;
- Security impact;
- Tooling/pins;
- Testing evidence;
- CI evidence by task;
- Documentation impact;
- Breaking changes: none to an existing public runtime because no public product transport existed;
- Explicit non-idempotent registration note;
- Explicit “no internal contract” note;
- Next gate: Go Public Transport Adapter design.

Mark the draft PR review-ready only after the body matches the exact head.

## Step 6 — Require fresh review-ready CI

The final implementation head must show:

```text
policy           SUCCESS
repository       SUCCESS
changes          SUCCESS
public-contract  SUCCESS
CI / gate        SUCCESS
```

Because the complete branch changed CI and local-dev phase guards, `go-api` and `local-dev` are expected to execute under current detector rules; if they execute, both must succeed.

Do not alter detector rules merely to make them skip.

STOP.

---

# Task 5 — External review + exact-head landing gate

**Goal:** Close independent review and land the first public contract without silently beginning transport implementation.

## Step 1 — Request external review

Request review on the exact review-ready implementation head.

Every finding is untrusted review input until independently verified.

### Valid finding

```text
verify
-> minimal fix
-> task self-review
-> affected local/remote tests
-> coherent commit
-> fresh exact-head CI
```

### Invalid/stale finding

Reply with specific current-code/contract evidence.

No unresolved actionable review thread may remain before merge.

## Step 2 — Re-run whole-branch scope check after findings

Review fixes must not smuggle in:

- Go handlers;
- runtime wiring;
- internal contract;
- auth/security scheme;
- extra Monitor fields;
- lifecycle/list/history capability;
- new persistent state;
- root Node project.

## Step 3 — Landing conditions

Squash merge only when all are true:

1. exact-head CI green;
2. `public-contract` explicitly succeeded;
3. whole-branch self-review green;
4. external review closed;
5. unresolved actionable thread count = 0;
6. branch is mergeable;
7. branch is not behind current `main`.

Squash title:

```text
feat(contracts): establish public monitoring API contract
```

Use expected-head SHA protection for the merge.

## Step 4 — Require fresh main push CI

After merge require current `main` to point at the squash commit and fresh push CI to close GREEN.

At minimum:

```text
policy           SUCCESS
repository       SUCCESS
changes          SUCCESS
public-contract  SUCCESS
CI / gate        SUCCESS
```

Any Go/local-dev jobs triggered by the landed CI/guard diff must also succeed.

Inspect public-contract logs again for exact tool versions and all validation layers.

## Step 5 — Close the phase

After post-merge CI is green:

- Public Monitoring API Contract artifact phase is CLOSED.
- Do not create handlers automatically.
- Do not create `internal.yaml`.
- Do not begin Web/Rust work.

The next safe product gate is:

```text
Go Public Transport Adapter — product/scope + design gate
```

That future gate must decide runtime mapping/composition and the schema-compatible readiness implementation before product handlers become live.

STOP.

---

## 6. Final Exit Criteria

The public contract implementation phase is complete only when all are true:

1. `contracts/openapi/public.yaml` exists.
2. It declares OpenAPI 3.1.2.
3. `info.version` is 0.1.0.
4. It has exactly POST /monitors and GET /monitors/{monitorId}.
5. No list/update/delete/lifecycle path exists.
6. Create request contains only targetUrl.
7. Monitor contains only id/targetUrl/createdAt.
8. Duplicate-target semantics are not contradicted.
9. Registration is documented as non-idempotent.
10. POST success is 201 with Location + Monitor.
11. GET success is 200 + Monitor.
12. Error status/media mappings match the landed design.
13. Problem uses only the selected RFC 9457 base surface.
14. No public security scheme is invented.
15. No servers/public-deployment assumption is invented.
16. `contracts/openapi/internal.yaml` does not exist.
17. Redocly spec validation is exact-version pinned and green.
18. Redocly bundle/reference resolution is green.
19. Repository-owned semantic checker is green.
20. Negative semantic fixtures are green.
21. Contract change detector is green.
22. Public-contract CI is path-aware.
23. Stable CI / gate includes public-contract.
24. Old local-dev blanket contracts prohibition is safely removed.
25. Local-dev still forbids unrelated deferred root runtime manifests.
26. No package.json/npm lockfile is introduced.
27. No Go/Rust/Web source is introduced.
28. No database migration/schema change is introduced.
29. Canonical docs distinguish contract existence from runtime exposure.
30. Exact-head external review is closed.
31. Squash landing succeeds with expected-head protection.
32. Fresh main push CI is green.
33. Go Public Transport Adapter remains a separate future gate.

---

## 7. Plan Self-Review

### Scope alignment

PASS.

The plan implements only the landed public contract artifact and its verification/documentation surface.

### Product capability safety

PASS.

No new domain/application behavior is created. The artifact describes only RegisterMonitor/GetMonitor behavior that already exists.

### Contract-first sequencing

PASS.

The OpenAPI artifact lands before any Go HTTP handler or Web consumer.

### Public/internal separation

PASS.

The internal Checker contract remains forbidden and separately gated.

### Persistence safety

PASS.

No migration or durable-state change occurs.

### Runtime safety

PASS.

No Go runtime wiring, public host port, auth, or CORS implementation occurs.

### Tooling/YAGNI

PASS.

One exact Redocly CLI invocation and one CI-only Node runtime are sufficient. No root Node project, dependency graph, generated client, or documentation renderer is introduced.

### Validation layering

PASS.

Generic OpenAPI parsing/spec correctness is delegated to Redocly. Product-specific invariants are repository-owned and tested separately against resolved JSON.

### CI architecture

PASS.

A dedicated path-aware contract job is added without weakening the existing Go/local-dev detectors or stable aggregate gate.

### Brownfield safety

PASS.

The obsolete blanket `contracts/` prohibition is removed only when the real public artifact lands, while contract-specific checks take over responsibility for allowed contract shape.

### Documentation safety

PASS.

Canonical docs will explicitly distinguish a defined contract artifact from an implemented public runtime adapter.

### External dependency freshness

PASS.

The plan records current reviewed versions but requires Task 1 to re-check patch-level freshness before modifying CI.

### Diff hygiene

PASS by plan.

The plan PR itself must contain exactly this plan document.

### Execution safety

PASS.

Each implementation task ends with explicit self-review and remote evidence before the next task. No handler implementation is authorized by this plan.
