# Go Monitoring Foundation Implementation Plan

> **For agentic workers:** Execute this plan strictly task-by-task. Do not collapse tasks, do not start later tasks while an earlier task is RED, and perform the explicit project self-review after every task before committing.

**Goal:** Implement the first real Go Control Plane foundation for `uptime-lab`: a minimal Monitoring registration/read domain and application layer, Go-owned PostgreSQL persistence and migrations, operational HTTP health, a real API container in the existing Compose topology, and path-aware Go CI — without introducing public/internal product HTTP contracts, mutable monitor lifecycle, scheduling, Rust, or React.

**Architecture:** The Go API is one nested Go module at `apps/api`. Monitoring owns a minimal immutable `Monitor` aggregate (`MonitorID`, `TargetURL`, `CreatedAt`), exactly two application use cases (`RegisterMonitor`, `GetMonitor`), and a module-specific `MonitorRepository` port with create/read capabilities only. PostgreSQL is accessed through a pgx adapter. SQL migrations are embedded and applied explicitly by a separate goose-backed migration command; API startup never auto-migrates. The production API binary exposes only `/livez` and `/readyz`, and the existing four-service Compose graph replaces only the API placeholder with the real Go runtime.

**Tech Stack:** Go 1.27.1, standard library `uuid`, `net/http`, `log/slog`, pgx/v5 5.11.0, goose/v3 3.28.0, PostgreSQL 18.6, Alpine 3.24.2, Docker Compose >= 2.22.0, GitHub Actions, govulncheck 1.8.0, Bash.

**Spec:** `docs/superpowers/specs/2026-09-20-go-monitoring-foundation-design.md`

---

## Plan-Time Verified Pins

These pins were verified on 2026-09-20 and are the implementation baseline unless a security/release check performed immediately before the relevant task proves a newer compatible patch is required.

| Dependency / Tool | Plan-time pin | Verification source |
|---|---|---|
| Go toolchain | `1.27.1` | `https://go.dev/dl/` |
| Go standard-library UUID | Go 1.27.1 `uuid.NewV7()` | `https://pkg.go.dev/uuid@go1.27.1` |
| pgx | `github.com/jackc/pgx/v5 v5.11.0` | pgx changelog/tag |
| goose | `github.com/pressly/goose/v3 v3.28.0` | goose changelog/tag |
| govulncheck | `golang.org/x/vuln/cmd/govulncheck@v1.8.0` | pkg.go.dev |
| Go builder image | `golang:1.27.1-alpine3.24` | Docker Official Image |
| Runtime base image | `alpine:3.24.2` | existing repository pin |
| PostgreSQL | `postgres:18.6-alpine3.24` | existing repository pin |
| setup-go | `actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e` | v7.0.0 immutable commit |
| checkout | `actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1` | existing repository pin |

Implementation MUST re-check Go/pgx/goose/govulncheck patch-level freshness before the task that first introduces each dependency. Major/minor scope changes require returning to design review; patch updates may be accepted when compatible and explicitly recorded in task evidence.

---

## Global Constraints

- The landed Go Monitoring design is authoritative.
- Go is the only runtime allowed to own durable product state in PostgreSQL.
- Rust and browser PostgreSQL access remain forbidden.
- This milestone implements only `RegisterMonitor` and `GetMonitor`.
- Mutable monitor lifecycle is explicitly out of scope:
  - no `enabled`;
  - no `updated_at`;
  - no `SetMonitorEnabled`;
  - no `Save` repository capability;
  - no optimistic concurrency/version column/row-lock policy.
- No public monitor HTTP endpoint is allowed.
- No internal checker work/result endpoint is allowed.
- No OpenAPI file is allowed.
- No scheduler, due-work query, check result/history, incident, notification, event bus, outbox, broker, cache, auth, or billing code is allowed.
- No Go network probe implementation is allowed.
- Target registration validation is syntax/product validation only and MUST NOT be presented as SSRF safety.
- No ORM, sqlc, query builder, generic repository, DI container, Viper/config framework, HTTP framework, or logging framework.
- No root `go.mod` or `go.work`.
- No empty future-module scaffolding.
- API startup does not auto-apply migrations.
- Docker Compose remains exactly `web`, `db`, `api`, `checker`.
- Only `api` transitions from placeholder to real runtime in this milestone.
- No host application port.
- Web and Checker remain placeholders.
- PostgreSQL remains `postgres:18.6-alpine3.24` with `/var/lib/postgresql`.
- API image must be non-root, read-only-compatible, `init: true`, and health-checked through real `/readyz`.
- CI keeps read-only permissions, pinned actions, no `pull_request_target`, and stable required check `CI / gate`.
- Contributor-facing docs and code comments are English.
- Implementation branch is created only after this plan is reviewed and landed.
- Task 1 bootstraps the minimal path-aware `go-api` CI surface and opens the implementation PR as **draft** after its coherent commit.
- The draft implementation PR remains open through Tasks 2-8 so every task closes with fresh remote CI evidence on the exact pushed head; it is not marked review-ready and no external implementation review is requested before Task 9.
- PostgreSQL integration evidence is added to the existing `go-api` job in Task 4 and extended for the adapter in Task 5; canonical Docker evidence becomes Go-source-sensitive in Task 7.
- Exact Go 1.27.x and real-Docker claims must come from an environment that actually provides them. If the executing local environment lacks the reviewed Go/Docker versions, do not install or claim an ad-hoc substitute; from Task 1 onward, fresh draft-PR `go-api` / `local-dev` jobs provide the canonical exact-environment evidence.
- Local commands remain useful when a capable environment exists, but missing local capability is never reported as a passed local test; the corresponding remote job must execute the same required layer successfully.
- Every task ends with explicit self-review, one coherent Conventional Commit, and fresh draft-PR CI evidence once the draft PR exists, using scope `api`, `devops`, `docs`, or `architecture` as appropriate.
- If a task self-review is RED, revise before committing and do not advance.

---

## Review Focus

1. The first Go module must not create root Go ownership for the polyglot repository.
2. Existing Docker phase checks currently forbid `apps/**`; Task 1 must safely open only `apps/api/**` while keeping Web/Checker runtime source forbidden.
3. `TargetURL` validation must preserve the original accepted URL string and must not perform lossy normalization for uniqueness.
4. Duplicate target URLs must remain valid in both domain/application and PostgreSQL integration evidence.
5. UUID v7 is generated by Go before persistence; PostgreSQL never owns identity generation.
6. The adapter must not rely on undocumented mapping of the new standard-library `uuid.UUID` type; UUID text conversion at the adapter boundary should remain explicit and testable.
7. Adapter pgx errors must not become application contracts.
8. Migration up/down/up must be proven against real PostgreSQL.
9. API startup must never auto-run goose.
10. Transient PostgreSQL unavailability must make `/readyz` return 503 rather than forcing the already-started process to exit; invalid configuration still fails fast.
11. `/livez` must not depend on PostgreSQL.
12. Monitoring application services must not be dead-wired into the production API binary before a real product transport consumer exists.
13. The real API Compose service must retain `db -> api -> checker` ordering without adding host ports.
14. API Compose health must call real `/readyz`, not a readiness marker file.
15. The Docker fitness checker must stop forbidding `apps/api` but continue rejecting premature `apps/web`, `apps/checker`, contracts, root Go/Cargo/npm manifests, and unrelated runtime scaffolding.
16. Go CI integration tests use real PostgreSQL but do not require a product host port.
17. Go source changes must trigger both Go verification and the local-dev Docker smoke once the API image consumes `apps/api/**`.
18. The aggregate `CI / gate` must require Go success when relevant and accept Go skipped when irrelevant.
19. Dependabot Go-module updates are introduced with the Go manifest, not before.
20. Final docs must clearly say API/Monitoring foundation is implemented while product HTTP contracts, Rust, and Web remain unimplemented.

---

## Review / Landing Model

Design is already merged:

```text
main@41a27c0
  └── docs/go-monitoring-foundation-plan
```

Plan PR title is locked to:

```text
docs(api): add Go monitoring foundation implementation plan
```

Plan landing sequence:

1. plan-only PR -> current `main`;
2. fresh CI;
3. external plan review;
4. resolve all findings;
5. squash merge plan;
6. verify merge-after `main` CI;
7. only then create `feat/go-monitoring-foundation` from the new `main`.

Implementation sequence:

```text
main + reviewed plan
  └── feat/go-monitoring-foundation
```

Implementation is executed Task 1 -> Task 9. Task 1 opens `feat/go-monitoring-foundation` as a **draft PR** after bootstrapping minimal Go CI. Tasks 2-8 advance that same draft PR only after fresh task-level CI succeeds. Task 9 performs the whole-branch review, updates the PR evidence/body, marks it review-ready, requests external review, and controls squash landing.

---

## Target File Map

Expected implementation ownership. Exact files may be reduced if a task proves a smaller structure is sufficient; additions outside this map require plan review.

```text
apps/api/
├── cmd/
│   ├── api/
│   │   └── main.go
│   └── migrate/
│       └── main.go
├── internal/
│   ├── modules/
│   │   └── monitoring/
│   │       ├── domain/
│   │       │   ├── errors.go
│   │       │   ├── monitor.go
│   │       │   ├── monitor_id.go
│   │       │   ├── monitor_test.go
│   │       │   ├── target_url.go
│   │       │   └── target_url_test.go
│   │       ├── application/
│   │       │   ├── errors.go
│   │       │   ├── get_monitor.go
│   │       │   ├── get_monitor_test.go
│   │       │   ├── register_monitor.go
│   │       │   └── register_monitor_test.go
│   │       ├── ports/
│   │       │   └── monitor_repository.go
│   │       ├── adapters/
│   │       │   └── postgres/
│   │       │       ├── repository.go
│   │       │       └── repository_integration_test.go
│   │       └── module.go
│   └── platform/
│       ├── config/
│       │   ├── config.go
│       │   └── config_test.go
│       ├── database/
│       │   ├── pool.go
│       │   └── pool_test.go
│       ├── httpserver/
│       │   ├── server.go
│       │   └── server_test.go
│       └── observability/
│           ├── logger.go
│           └── logger_test.go
├── migrations/
│   ├── embed.go
│   ├── provider.go
│   ├── provider_integration_test.go
│   └── 00001_create_monitoring_monitors.sql
├── Dockerfile
├── go.mod
└── go.sum

scripts/ci/
├── check-go-architecture.sh
├── test-check-go-architecture.sh
├── run-go-postgres-tests.sh
├── detect-go-api-changes.sh
└── test-detect-go-api-changes.sh

Existing files modified later:
- scripts/ci/check-local-dev.sh
- scripts/ci/test-check-local-dev.sh
- scripts/ci/detect-local-dev-changes.sh
- scripts/ci/test-detect-local-dev-changes.sh
- scripts/ci/smoke-local-dev.sh
- scripts/ci/test-smoke-local-dev.sh (only if control-flow evidence changes)
- compose.yaml
- .env.example
- .github/workflows/ci.yml
- .github/dependabot.yml
- README.md
- docs/README.md
- docs/architecture/container-view.md
- docs/architecture/module-boundaries.md
- docs/architecture/data-ownership.md
- docs/devops/local-development.md
- docs/backend/go-control-plane.md
- docs/testing/go-monitoring-foundation.md
```

Explicitly forbidden by this plan:

```text
contracts/**
apps/web/**
apps/checker/**
migrations/**                # root-level
go.mod                       # root-level
go.work                      # root-level
package.json                 # root-level
Cargo.toml                   # root-level
```

---

# Task 1 — Open the Go source phase and implement the Monitoring domain

**Goal:** Introduce the nested Go module and a real, tested Monitoring domain without creating empty scaffolding or leaving the existing Docker fitness checker falsely declaring all `apps/**` paths illegal.

**Files:**
- Create: `apps/api/go.mod`
- Create: `apps/api/internal/modules/monitoring/domain/errors.go`
- Create: `apps/api/internal/modules/monitoring/domain/monitor_id.go`
- Create: `apps/api/internal/modules/monitoring/domain/target_url.go`
- Create: `apps/api/internal/modules/monitoring/domain/monitor.go`
- Create: domain tests
- Create: `scripts/ci/detect-go-api-changes.sh`
- Create: `scripts/ci/test-detect-go-api-changes.sh`
- Modify: `scripts/ci/check-local-dev.sh`
- Modify: `scripts/ci/test-check-local-dev.sh`
- Modify: `.github/workflows/ci.yml`

**Locked interfaces:**

```go
type MonitorID struct {
    // wraps standard-library uuid.UUID
}

type TargetURL struct {
    // stores the accepted original string
}

type Monitor struct {
    ID        MonitorID
    TargetURL TargetURL
    CreatedAt time.Time
}
```

Constructors should make invalid states difficult to represent. Exact exported naming may be refined during TDD, but behavior is locked by the design.

- [ ] **Step 1: Re-check Go patch baseline**

Confirm Go 1.27.1 is still the current compatible Go 1.27 patch. If only a newer 1.27.x patch exists and no incompatibility is known, record the patch update in task evidence and use it consistently in `go.mod`, Docker, and later CI.

Do not silently move to Go 1.28 or another major/minor.

- [ ] **Step 2: Update the Docker phase fitness fixture before creating `apps/api`**

RED first: change the canonical fixture in `test-check-local-dev.sh` to include:

```text
apps/api/go.mod
```

and expect the current checker to fail.

Then change the phase-forbidden logic so:

Allowed:

```text
apps/api/**
```

Still forbidden:

```text
apps/web/**
apps/checker/**
contracts/**
migrations/**
root package.json
root go.mod
root go.work
root Cargo.toml
```

Replace the former negative case that treated `apps/api/go.mod` as forbidden with a representative future-runtime case such as `apps/web/package.json`.

Keep the existing topology/health hardening tests unchanged otherwise.

GREEN evidence:

```bash
./scripts/ci/test-check-local-dev.sh
```

Expected current count may remain 19 if the old forbidden case is replaced rather than extended. Record the exact count.

- [ ] **Step 3: Add the nested module and failing domain tests**

Create:

```go
module github.com/kefyusuf/uptime-lab/apps/api

go 1.27.1
```

Use the final verified 1.27.x patch if Step 1 updated it.

Write tests before implementations for:

1. UUID v7 identity can be constructed and compared;
2. nil/zero UUID is rejected as a MonitorID;
3. valid `http://example.com`;
4. valid `https://example.com/path?x=1`;
5. explicit port accepted;
6. missing scheme rejected;
7. non-http(s) scheme rejected;
8. missing hostname rejected;
9. embedded userinfo rejected;
10. fragment rejected;
11. accepted TargetURL returns the original input string without lossy normalization;
12. Monitor creation requires valid ID/TargetURL;
13. CreatedAt is normalized/preserved as UTC according to the chosen constructor contract.

Run RED:

```bash
cd apps/api
go test ./internal/modules/monitoring/domain/...
```

Expected: compile/test failure because production domain code is incomplete.

- [ ] **Step 4: Implement the minimum domain**

Use only standard library packages including `uuid`, `net/url`, `time`, and `errors` as needed.

Important constraints:

- use standard-library `uuid.NewV7()` only through the application ID dependency later, not hidden inside persistence;
- TargetURL validates syntax but performs no DNS/network access;
- no IP/private-network policy;
- no canonicalization for uniqueness;
- no Enabled/UpdatedAt/version fields;
- no domain event.

Run GREEN:

```bash
go test ./internal/modules/monitoring/domain/...
go test ./...
test -z "$(gofmt -l .)"
```

If host Go is unavailable, execute equivalent commands with the pinned official Go builder image; do not install an unpinned host toolchain.

- [ ] **Step 5: Add repository-owned Go change detection with TDD**

Create:

```text
scripts/ci/detect-go-api-changes.sh
scripts/ci/test-detect-go-api-changes.sh
```

Interface:

```bash
./scripts/ci/detect-go-api-changes.sh <base-sha> <head-sha>
```

Successful stdout is exactly `true` or `false`.

Pin at least:

1. `apps/api/**/*.go` addition/change -> true;
2. `apps/api/go.mod` -> true;
3. future `apps/api/go.sum` -> true;
4. future `apps/api/Dockerfile` -> true;
5. future Go architecture checker path -> true;
6. future `scripts/ci/run-go-postgres-tests.sh` -> true;
7. detector self-change -> true;
8. workflow change -> true conservatively;
9. unrelated architecture/frontend documentation -> false;
10. relevant deletion -> true;
11. all-zero base -> true;
12. unavailable base -> true;
13. unavailable head -> non-zero.

RED the harness before the detector implementation, then GREEN it.

- [ ] **Step 6: Bootstrap minimal path-aware Go CI**

Extend the existing `changes` job with:

```text
local_dev
go_api
```

Run the Go detector self-test in `changes`.

Add conditional job:

```text
go-api
```

with:

- `needs: changes`;
- condition `needs.changes.outputs.go_api == 'true'`;
- `ubuntu-24.04`;
- repository read-only permissions;
- existing immutable checkout SHA;
- `persist-credentials: false`;
- immutable `actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e`;
- exact reviewed Go 1.27.x patch;
- `cache: false` while the module is stdlib-only and has no stable dependency checksum file;
- module root `apps/api`.

Do not point setup-go caching at a nonexistent `go.sum`. Task 4 enables nested-module caching after third-party dependencies create a committed `apps/api/go.sum`.

Minimum initial commands:

```bash
cd apps/api
test -z "$(gofmt -l .)"

go mod tidy
test -z "$(git status --porcelain -- go.mod go.sum)"

go mod verify
go vet ./...
go test ./...
```

The path-scoped `git status --porcelain` check is intentional: it catches both tracked manifest modifications and a newly created untracked `go.sum`. The invariant is that `go mod tidy` produces no uncommitted manifest delta.

Update `CI / gate` so `go-api` must be `success` when relevant and may be `skipped` when irrelevant.

No PostgreSQL service, integration-tagged test, race test, or govulncheck is added yet.

- [ ] **Step 7: Self-review**

Check:

- only `apps/api` was opened, not other runtimes;
- no generic base entity/value-object package;
- domain imports no pgx/goose/net/http/platform/application/adapter code;
- URL validation does not claim SSRF safety;
- no mutable lifecycle leaked back in;
- local-dev fitness checker is phase-consistent again;
- tests prove original URL preservation and duplicate-target possibility is not precluded;
- Go changes cannot bypass remote CI from this point forward;
- workflow permissions/action pins remain least-privilege and immutable.

- [ ] **Step 8: Commit**

```text
feat(api): add monitoring domain foundation
```

- [ ] **Step 9: Open the draft implementation PR and require fresh CI**

Create/open:

```text
head: feat/go-monitoring-foundation
base: main
draft: true
title: feat(api): establish Go monitoring foundation
```

The initial PR body states that implementation is incomplete and records Task 1 evidence only. Do not request external implementation review.

Fresh CI on the committed Task 1 head must show:

```text
policy       SUCCESS
repository   SUCCESS
changes      SUCCESS
go-api       SUCCESS
local-dev    SUCCESS
CI / gate    SUCCESS
```

`local-dev` is expected to run because Task 1 modifies its repository-owned checker. If detector behavior proves otherwise, investigate rather than changing the expected result casually.

STOP. Do not start Task 2 without the next execution instruction.

---

# Task 2 — Add Monitoring ports and application use cases with TDD

**Goal:** Implement exactly `RegisterMonitor` and `GetMonitor` over a narrow module-owned repository port.

**Files:**
- Create: `apps/api/internal/modules/monitoring/ports/monitor_repository.go`
- Create: `apps/api/internal/modules/monitoring/application/errors.go`
- Create: `apps/api/internal/modules/monitoring/application/register_monitor.go`
- Create: `apps/api/internal/modules/monitoring/application/register_monitor_test.go`
- Create: `apps/api/internal/modules/monitoring/application/get_monitor.go`
- Create: `apps/api/internal/modules/monitoring/application/get_monitor_test.go`

**Port capability:**

```text
MonitorRepository
├── Create(ctx, monitor)
└── ByID(ctx, id)
```

No `Save`, update, delete, list, generic repository, transaction manager, or unit of work.

- [ ] **Step 1: Write failing application tests**

Use an in-test fake repository, not a production mock package.

Pin cases:

### RegisterMonitor

1. valid target -> ID generated -> UTC creation time -> repository Create called once;
2. invalid TargetURL -> no repository call;
3. repository create error -> stable application persistence error;
4. ID generator returns invalid ID -> failure before persistence;
5. deterministic clock is used exactly as defined by the application contract;
6. duplicate target text is not pre-rejected by application logic.

### GetMonitor

1. existing ID -> monitor returned;
2. port not-found sentinel -> application `ErrMonitorNotFound`;
3. other repository failure -> stable application persistence error;
4. invalid/zero MonitorID is rejected before repository access if the domain constructor permits such a caller path.

RED:

```bash
cd apps/api
go test ./internal/modules/monitoring/application/...
```

- [ ] **Step 2: Define narrow dependencies**

Prefer function dependencies rather than service containers:

```go
type IDGenerator func() domain.MonitorID
type Clock func() time.Time
```

The real ID generator later wraps `uuid.NewV7()`; tests inject deterministic IDs.

The port owns an infrastructure-neutral not-found signal. Application maps that to its stable application error vocabulary.

Generic persistence errors must not expose pgx types or database strings as application contracts.

- [ ] **Step 3: Implement use cases**

Keep orchestration small:

```text
Register:
input string
  -> TargetURL
  -> generated MonitorID
  -> current UTC time
  -> Monitor
  -> repository.Create
  -> return Monitor

Get:
MonitorID
  -> repository.ByID
  -> map not-found/persistence errors
  -> return Monitor
```

No runtime composition or HTTP mapping.

GREEN:

```bash
go test ./internal/modules/monitoring/domain/... ./internal/modules/monitoring/application/...
go test ./...
go vet ./...
test -z "$(gofmt -l .)"
```

- [ ] **Step 4: Self-review**

Verify:

- application depends inward only;
- fake repository stays in tests;
- no product HTTP DTO;
- no mutable lifecycle;
- no production event bus;
- errors are stable and infrastructure-neutral;
- duplicate targets are allowed.

- [ ] **Step 5: Commit**

```text
feat(api): add monitoring application use cases
```

- [ ] **Step 6: Require fresh draft-PR CI**

On the exact Task 2 head require:

```text
policy       SUCCESS
repository   SUCCESS
changes      SUCCESS
go-api       SUCCESS
CI / gate    SUCCESS
```

`local-dev` may be skipped because the real API image does not consume `apps/api/**` yet.

STOP.

---

# Task 3 — Add Go architecture fitness functions

**Goal:** Mechanically enforce the Go dependency rules before persistence/platform complexity appears.

**Files:**
- Create: `scripts/ci/check-go-architecture.sh`
- Create: `scripts/ci/test-check-go-architecture.sh`
- Modify: `.github/workflows/ci.yml`

**Checker contract:**

```bash
./scripts/ci/check-go-architecture.sh <repo-root>
```

It uses the Go toolchain's package metadata (`go list`) and minimal repository path checks. Do not parse Go source with a home-grown parser when `go list` can provide imports.

- [ ] **Step 1: Build RED fixture harness**

Create isolated temporary Go modules under `$TMP`. The harness must not mutate the real `apps/api`.

Pin at least these cases:

1. canonical monitoring domain/application/ports layout passes;
2. domain -> application fails;
3. domain -> platform fails;
4. domain -> postgres adapter fails;
5. domain -> `net/http` fails;
6. application -> postgres adapter fails;
7. application -> platform fails;
8. ports -> postgres adapter fails;
9. root/shared business package fails;
10. root/common business package fails;
11. domain -> pgx fails using a local fake module/replacement so the test does not require network access.

Before the checker exists, RED harness must fail.

- [ ] **Step 2: Implement checker**

At minimum enforce real repository import prefixes:

```text
github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/domain
github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/application
github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/ports
github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/adapters
github.com/kefyusuf/uptime-lab/apps/api/internal/platform
```

Explicitly reject domain imports matching:

```text
net/http
github.com/jackc/pgx/
github.com/pressly/goose/
.../application
.../ports
.../adapters
.../platform
```

Application may import domain + ports + standard library but not adapters/platform.

Ports may import domain + standard library but no concrete infrastructure.

The checker must also reject root-level business buckets such as:

```text
apps/api/internal/shared
apps/api/internal/common
```

Do not reject frontend-local `shared` because that is outside the Go module.

- [ ] **Step 3: GREEN**

```bash
./scripts/ci/test-check-go-architecture.sh
./scripts/ci/check-go-architecture.sh .
cd apps/api
go test ./...
go vet ./...
```

- [ ] **Step 4: Wire architecture evidence into the existing `go-api` job**

The job created in Task 1 must now run:

```bash
./scripts/ci/test-check-go-architecture.sh
./scripts/ci/check-go-architecture.sh .
```

from the repository root in addition to the existing Go fmt/tidy/verify/vet/unit checks.

Do not create a second Go workflow/job for architecture.

- [ ] **Step 5: Self-review**

Check deterministic/no-network fixture behavior, clear failure messages, no external architecture framework, no false dependency restrictions that prevent adapters from depending inward, and remote CI execution on the exact branch head.

- [ ] **Step 6: Commit**

```text
test(api): enforce Go architecture boundaries
```

- [ ] **Step 7: Require fresh draft-PR CI**

Require `go-api=SUCCESS`, `local-dev=SUCCESS`, and `CI / gate=SUCCESS` on the exact Task 3 head. Task 3 modifies `.github/workflows/ci.yml`, which is already a local-dev detector path, so `local-dev` must not be skipped. Inspect the Go job to confirm both architecture harness and real-repository architecture check executed.

STOP.

---

# Task 4 — Add SQL migrations, embedded goose provider, and explicit migration command

**Goal:** Establish module-owned schema creation and explicit migration execution without API auto-migration.

**Files:**
- Modify: `apps/api/go.mod`
- Create/update: `apps/api/go.sum`
- Create: `apps/api/migrations/embed.go`
- Create: `apps/api/migrations/provider.go`
- Create: `apps/api/migrations/provider_integration_test.go`
- Create: `apps/api/migrations/00001_create_monitoring_monitors.sql`
- Create: `apps/api/cmd/migrate/main.go`
- Create: `scripts/ci/run-go-postgres-tests.sh`
- Modify: `.github/workflows/ci.yml`

**Plan-time dependency pins:**

```text
github.com/jackc/pgx/v5 v5.11.0
github.com/pressly/goose/v3 v3.28.0
```

Re-check compatible patch releases immediately before editing `go.mod`.

- [ ] **Step 1: Add failing migration tests**

Migration integration evidence must use real PostgreSQL.

Every test file that requires a live PostgreSQL instance MUST begin with the Go build constraint:

```go
//go:build integration
```

Normal `go test ./...` must remain DB-independent. Live-DB packages run only through explicit `-tags=integration` commands.

The SQL migration is logically:

```sql
-- +goose Up
CREATE SCHEMA monitoring;

CREATE TABLE monitoring.monitors (
    id uuid PRIMARY KEY,
    target_url text NOT NULL,
    created_at timestamptz NOT NULL
);

-- +goose Down
DROP TABLE monitoring.monitors;
DROP SCHEMA monitoring;
```

Do not use `IF NOT EXISTS` to silently adopt a schema owned by something else.

Do not add unique target URL, enabled, updated_at, version, indexes, scheduling or result tables.

Test requirements:

1. Up creates `monitoring`;
2. Up creates exactly expected monitor columns and PK;
3. no `enabled` / `updated_at` / version column;
4. Down removes table/schema;
5. Up after Down succeeds again;
6. goose metadata remains platform metadata outside `monitoring`.

- [ ] **Step 2: Add deterministic Docker-backed PostgreSQL test runner**

`scripts/ci/run-go-postgres-tests.sh` owns local integration orchestration without exposing a product host port.

It must:

- create an isolated `COMPOSE_PROJECT_NAME`;
- explicitly set isolated test DB/user/password values so root `.env` cannot alter test semantics;
- `docker compose up -d db --wait`;
- run the pinned `golang:1.27.1-alpine3.24` container attached to `$COMPOSE_PROJECT_NAME_default`;
- mount `apps/api` read-only when practical;
- use container-local writable `GOMODCACHE` / `GOCACHE`;
- set `PGHOST=db`, `PGPORT=5432`, test `PGDATABASE`, `PGUSER`, `PGPASSWORD`, `PGSSLMODE=disable`;
- run integration packages in an explicit order;
- always `docker compose down -v --remove-orphans` on success/failure.

For Task 4 it runs the migration integration package only. Task 5 extends it with adapter tests.

This script does not become application logic.

- [ ] **Step 3: Implement embedded migrations/provider**

Use:

```go
//go:embed *.sql
var FS embed.FS
```

Use the current goose Provider API with PostgreSQL dialect and embedded FS.

The provider must disable accidental global Go migration registry usage if the selected goose API supports that option.

For `database/sql`, derive pgx configuration from libpq-compatible environment configuration rather than concatenating credentials into a URL. The pgx stdlib bridge is allowed here because goose consumes `*sql.DB`.

- [ ] **Step 4: Implement migration command**

Supported command surface is deliberately small:

```text
uptime-lab-migrate up
uptime-lab-migrate down
uptime-lab-migrate status
```

Invalid/missing subcommands exit non-zero with concise usage.

The command:

- loads DB config;
- opens DB;
- applies exactly the requested migration action;
- logs structured failure without secrets;
- does not run inside `cmd/api`.

No reset/drop-all convenience command.

- [ ] **Step 5: GREEN**

```bash
cd apps/api
go mod tidy
go mod verify
test -z "$(gofmt -l .)"
go vet ./...
go test ./...
cd ../..
./scripts/ci/run-go-postgres-tests.sh
git diff --check
```

After `go mod tidy`, confirm only intended `go.mod` / `go.sum` changes.

- [ ] **Step 6: Extend `go-api` CI with real PostgreSQL migration evidence**

Add a CI-only PostgreSQL service to the existing `go-api` job:

```text
postgres:18.6-alpine3.24
```

with dedicated non-secret test database/user/password and `pg_isready` health options.

Because `go-api` runs directly on the `ubuntu-24.04` runner and connects through `127.0.0.1:5432`, the CI service MUST publish PostgreSQL to the runner host:

```yaml
ports:
  - 5432:5432
```

This host-port mapping exists only inside the ephemeral GitHub Actions job. It does **not** relax the product/local `compose.yaml` rule that application/database services remain unexposed to host ports.

Set test PG environment explicitly:

```text
PGHOST=127.0.0.1
PGPORT=5432
PGDATABASE=<ci test db>
PGUSER=<ci test user>
PGPASSWORD=<ci test password>
PGSSLMODE=disable
```

Once `apps/api/go.sum` is committed, update the existing setup-go step to enable cache deterministically:

```yaml
cache: true
cache-dependency-path: apps/api/go.sum
```

After the existing DB-independent checks run:

```bash
go test -tags=integration ./migrations
```

This service container and its host-port mapping are CI test infrastructure only; they do not change product/local Compose topology.

- [ ] **Step 7: Self-review**

Check schema minimality, explicit ownership, no auto-migrate, no product/local Compose host port, CI-only PostgreSQL publish scope, deterministic cleanup, no secret logging, and both local-script + remote-CI real-PostgreSQL evidence.

- [ ] **Step 8: Commit**

```text
feat(api): add monitoring database migrations
```

- [ ] **Step 9: Require fresh draft-PR CI**

The exact Task 4 head must have `go-api=SUCCESS` and `CI / gate=SUCCESS`. Inspect the Go job to confirm the migration integration test executed against the PostgreSQL service.

Because `.github/workflows/ci.yml` is local-dev relevant, `local-dev` is also expected to run and succeed on this task head.

STOP.

---

# Task 5 — Add pgx Monitoring repository and real PostgreSQL adapter evidence

**Goal:** Implement create/read persistence behind the Monitoring port with explicit domain mapping and no pgx leakage.

**Files:**
- Create: `apps/api/internal/modules/monitoring/adapters/postgres/repository.go`
- Create: `apps/api/internal/modules/monitoring/adapters/postgres/repository_integration_test.go`
- Create: `apps/api/internal/modules/monitoring/module.go`
- Modify: `scripts/ci/run-go-postgres-tests.sh`
- Modify: `.github/workflows/ci.yml`

- [ ] **Step 1: RED integration tests**

The adapter integration test file MUST use `//go:build integration` so ordinary unit/application runs remain DB-independent.

Pin cases:

1. Create then ByID round trip;
2. two monitors with identical `target_url` both persist;
3. ByID missing -> port not-found sentinel;
4. DB error does not escape as a pgx-specific application contract;
5. CreatedAt instant preserved in UTC semantics;
6. UUID round trip preserves exact UUID;
7. adapter never generates monitor identity;
8. adapter query context cancellation is respected where deterministic to test.

For UUID mapping, prefer explicit text conversion at the persistence boundary:

- insert `MonitorID.String()` with explicit PostgreSQL UUID coercion if needed;
- read UUID as text then use standard-library `uuid.Parse`.

Do not depend on undocumented direct scanning of the newly introduced stdlib UUID type.

- [ ] **Step 2: Implement repository**

Use `pgxpool.Pool` directly.

Queries remain hand-written and local to the adapter.

The adapter may define private row structures but they are not domain contracts.

Map:

```text
pgx.ErrNoRows -> ports.ErrMonitorNotFound
other DB error -> wrapped infrastructure error
```

Application layer continues mapping port errors into stable application errors.

No update/delete/list query.

- [ ] **Step 3: Add module-local composition boundary**

`monitoring/module.go` may expose a small constructor/value that groups `RegisterMonitor` and `GetMonitor` using a repository, ID generator and clock.

It MUST NOT:

- import platform packages;
- create pgx pools;
- read environment variables;
- be constructed in `cmd/api` yet;
- become a service locator.

Use it in module-level tests if useful so it is not speculative dead API.

- [ ] **Step 4: Extend real PostgreSQL runner**

Ordered integration sequence:

```text
migration integration tests
        ↓
adapter integration tests
```

Migration tests must leave schema in the Up state before adapter tests execute.

- [ ] **Step 5: GREEN**

```bash
cd apps/api
go test ./...
go vet ./...
test -z "$(gofmt -l .)"
cd ../..
./scripts/ci/check-go-architecture.sh .
./scripts/ci/run-go-postgres-tests.sh
```

- [ ] **Step 6: Extend remote integration evidence**

After migration integration succeeds, the existing `go-api` job must run:

```bash
go test -tags=integration ./internal/modules/monitoring/adapters/postgres
```

Keep migration and adapter packages explicit so schema preparation/order is visible.

- [ ] **Step 7: Self-review**

Check no pgx type leakage, no generic persistence abstraction, duplicate target support, explicit UUID mapping, no mutable lifecycle, and both migration + adapter integration evidence on the draft PR.

- [ ] **Step 8: Commit**

```text
feat(api): add monitoring postgres adapter
```

- [ ] **Step 9: Require fresh draft-PR CI**

Require `go-api=SUCCESS`, `local-dev=SUCCESS`, and `CI / gate=SUCCESS` on the exact Task 5 head, with both integration-tagged packages visible in Go job evidence. Task 5 modifies `.github/workflows/ci.yml`, so the existing local-dev detector must also run.

STOP.

---

# Task 6 — Add platform config, logging, DB pool, operational HTTP server, and API binary

**Goal:** Make the Control Plane process real while keeping product use cases unexposed and unwired until a later contract/transport phase.

**Files:**
- Create: `apps/api/internal/platform/config/config.go`
- Create: `apps/api/internal/platform/config/config_test.go`
- Create: `apps/api/internal/platform/observability/logger.go`
- Create: `apps/api/internal/platform/observability/logger_test.go`
- Create: `apps/api/internal/platform/database/pool.go`
- Create: `apps/api/internal/platform/database/pool_test.go`
- Create: `apps/api/internal/platform/httpserver/server.go`
- Create: `apps/api/internal/platform/httpserver/server_test.go`
- Create: `apps/api/cmd/api/main.go`

- [ ] **Step 1: Lock configuration behavior with RED tests**

Application settings:

```text
UPTIME_LAB_HTTP_ADDR
UPTIME_LAB_LOG_LEVEL
```

Local defaults:

```text
UPTIME_LAB_HTTP_ADDR=:8080
UPTIME_LAB_LOG_LEVEL=info
```

PostgreSQL uses libpq-compatible env:

```text
PGHOST
PGPORT
PGDATABASE
PGUSER
PGPASSWORD
PGSSLMODE
PGCONNECT_TIMEOUT
```

The config package owns application settings only. pgx config parsing owns PostgreSQL env interpretation where possible.

Tests must prove:

- defaults;
- accepted log levels;
- invalid log level fails;
- empty/invalid HTTP address fails according to chosen parser contract;
- secret values never appear in config string/log output.

- [ ] **Step 2: Add structured logger**

Use `log/slog` JSON handler.

Stable baseline fields include:

```text
service=api
component=<component>
event=<event>
```

Do not invent tracing IDs/metrics.

Tests should verify JSON parseability and that secret config is not emitted.

- [ ] **Step 3: Add pgx pool factory**

Use `pgxpool.ParseConfig("")` / equivalent current API so standard PG environment variables are honored.

Startup behavior:

- syntactically invalid DB config -> fail fast;
- pool construction itself does not require DB to be available;
- transient DB outage does not terminate an already-started API solely because readiness is false;
- pool is closed during shutdown.

Do not add hand-built connection-URL concatenation.

- [ ] **Step 4: Add operational HTTP server tests RED**

Required behavior:

### `GET /livez`

- 200;
- no DB call;
- minimal body;
- no product data.

### `GET /readyz`

- DB PingContext with bounded readiness timeout;
- 200 on success;
- 503 on DB failure/timeout;
- response does not expose DB error details/password/hostname;
- no schema/migration compatibility claim.

### HTTP method behavior

Non-GET requests receive a deterministic method-not-allowed response or the standard-library behavior chosen by the implementation; pin it in tests.

### Server lifecycle

- explicit ReadHeaderTimeout;
- explicit ReadTimeout/WriteTimeout/IdleTimeout appropriate to health-only surface;
- explicit MaxHeaderBytes;
- SIGTERM/SIGINT context drives graceful Shutdown;
- bounded shutdown timeout;
- no panic on repeated/cancelled shutdown paths.

Use `httptest` for handlers and a loopback ephemeral listener for graceful lifecycle tests where needed.

- [ ] **Step 5: Implement API composition root**

Conceptually only:

```text
typed config
  -> slog logger
  -> pgx pool
  -> operational HTTP server
```

DO NOT construct:

```text
Monitoring repository
Monitoring module
RegisterMonitor
GetMonitor
```

inside production `cmd/api` yet because there is no real transport consumer.

The Monitoring code remains real/tested library code until the later contract/transport phase.

- [ ] **Step 6: GREEN**

```bash
cd apps/api
go test ./...
go test -race ./...
go vet ./...
test -z "$(gofmt -l .)"
cd ../..
./scripts/ci/check-go-architecture.sh .
```

If race testing is unavailable in a local container due CGO/toolchain constraints, record that limitation; final GitHub Actions on Ubuntu MUST provide race evidence.

- [ ] **Step 7: Self-review**

Check no product endpoints, no dead Monitoring wiring, readiness/liveness distinction, no secret leakage, graceful shutdown, and DB outage behavior.

- [ ] **Step 8: Commit**

```text
feat(api): add operational control plane runtime
```

- [ ] **Step 9: Require fresh draft-PR CI**

Require `go-api=SUCCESS` and `CI / gate=SUCCESS` on the exact Task 6 head. The job must cover the new runtime/config/http tests plus all previously wired integration evidence.

STOP.

---

# Task 7 — Replace the API placeholder in Docker Compose and evolve local-dev fitness checks

**Goal:** Integrate the real Go API into the canonical four-service local topology without weakening existing Docker invariants.

**Files:**
- Create: `apps/api/Dockerfile`
- Modify: `compose.yaml`
- Modify: `.env.example` only if additional documented local knobs are genuinely required
- Modify: `scripts/ci/check-local-dev.sh`
- Modify: `scripts/ci/test-check-local-dev.sh`
- Modify: `scripts/ci/detect-local-dev-changes.sh`
- Modify: `scripts/ci/test-detect-local-dev-changes.sh`
- Modify: `scripts/ci/smoke-local-dev.sh`
- Modify: `scripts/ci/test-smoke-local-dev.sh` only if control-flow evidence changes

**Pinned images:**

```text
builder: golang:1.27.1-alpine3.24
runtime: alpine:3.24.2
db:      postgres:18.6-alpine3.24
```

Use final verified Go 1.27.x patch consistently.

- [ ] **Step 1: RED Docker fitness cases for real API**

Update the canonical checker fixture to represent:

```text
web      placeholder
db       postgres
api      real Go build
checker  placeholder
```

Pin exactly the existing invariant cases plus representative API-specific failures. Target at least these additional cases:

1. API still uses generic placeholder build -> fail;
2. API build context/dockerfile does not point to real API image -> fail;
3. API missing `init: true` -> fail;
4. API missing `read_only: true` -> fail;
5. API missing `PGHOST=db` -> fail;
6. API missing required PG database/user mapping -> fail;
7. API missing `UPTIME_LAB_HTTP_ADDR` -> fail;
8. API healthcheck does not call `/readyz` -> fail;
9. API healthcheck still uses placeholder marker -> fail;
10. API Dockerfile builder uses floating tag -> fail;
11. API Dockerfile runtime uses floating tag -> fail;
12. API Dockerfile lacks non-root USER -> fail;
13. `apps/web` runtime scaffold -> fail;
14. `apps/checker` runtime scaffold -> fail;
15. root `go.mod` -> fail.

Keep all applicable existing cases for host ports, project/container names, volume path, dependency ordering, profiles, restart, placeholder Web/Checker health, and DB `pg_isready`.

The exact total case count is locked when Task 7 begins and recorded before production checker changes.

- [ ] **Step 2: Add multi-stage API Dockerfile**

Builder requirements:

- exact Go image;
- module-first copy for dependency caching;
- `go mod download`;
- copy API source;
- `CGO_ENABLED=0`;
- `-trimpath`;
- disable accidental VCS stamping when source context lacks `.git` if needed;
- build both:
  - `uptime-lab-api`;
  - `uptime-lab-migrate`.

Runtime requirements:

- exact Alpine pin;
- CA certificates;
- non-root UID/GID 10001;
- both binaries copied read-only/executable;
- API binary is default entrypoint;
- no shell startup wrapper unless a concrete lifecycle need appears;
- no source/toolchain in runtime layer.

- [ ] **Step 3: Replace only API Compose service**

Required conceptual service:

```yaml
api:
  build:
    context: .
    dockerfile: apps/api/Dockerfile
  environment:
    UPTIME_LAB_HTTP_ADDR: ":8080"
    UPTIME_LAB_LOG_LEVEL: info
    PGHOST: db
    PGPORT: "5432"
    PGDATABASE: <mapped from POSTGRES_DB default/override>
    PGUSER: <mapped from POSTGRES_USER default/override>
    PGPASSWORD: <mapped from POSTGRES_PASSWORD default/override>
    PGSSLMODE: disable
  init: true
  read_only: true
  healthcheck:
    test: real loopback HTTP request to http://127.0.0.1:8080/readyz
  depends_on:
    db:
      condition: service_healthy
```

No `ports:`.

Use a runtime-available deterministic health client. The preferred plan is explicit BusyBox `wget` from Alpine, e.g. an exec-form healthcheck that fails on non-2xx. Verify actual behavior in smoke before locking syntax.

Remove placeholder-specific API readiness marker/tmpfs contract. Do not weaken Web/Checker placeholder contracts.

Checker continues:

```text
checker depends_on api: service_healthy
```

- [ ] **Step 4: Update local-dev change detection**

Once the image consumes `apps/api/**`, any API source change must return `local_dev=true`.

Add detector fixture cases for:

- `apps/api` source addition -> true;
- `apps/api` source deletion -> true;
- unrelated backend/design docs -> false unless another relevant path changed.

Keep zero/unavailable-base conservative behavior.

- [ ] **Step 5: Extend smoke evidence**

Existing smoke already proves PostgreSQL persistence/reset.

Add real API assertions after `compose up --wait`:

- API health is healthy;
- Checker can become healthy only after API;
- container-local `/livez` returns success;
- container-local `/readyz` returns success;
- no host port is required.

Do not introduce a fake product API call.

Canonical explicit migration operation is now documentable/executable:

```bash
docker compose exec api /usr/local/bin/uptime-lab-migrate up
```

Do not make smoke silently auto-migrate unless the smoke specifically needs schema evidence. If migration is exercised, it must be an explicit smoke step whose intent is visible in the script.

- [ ] **Step 6: GREEN**

```bash
./scripts/ci/test-detect-local-dev-changes.sh
./scripts/ci/test-check-local-dev.sh
./scripts/ci/test-smoke-local-dev.sh
./scripts/ci/check-local-dev.sh .
./scripts/ci/smoke-local-dev.sh
```

Real Docker smoke is mandatory before Task 7 closes.

Also:

```bash
docker compose config --quiet
docker compose build api
```

- [ ] **Step 7: Self-review**

Check four-service topology, no host ports, API health is real readiness, no auto-migration, read-only/non-root behavior, placeholder Web/Checker unchanged, and local-dev detector now covers API source.

- [ ] **Step 8: Commit**

```text
build(api): run control plane in local Docker
```

- [ ] **Step 9: Require fresh draft-PR CI**

For the exact Task 7 head:

```text
go-api       SUCCESS
local-dev    SUCCESS
CI / gate    SUCCESS
```

Neither `go-api` nor `local-dev` may be skipped now that the real API Docker image consumes `apps/api/**`.

Inspect local-dev logs for real API readiness and full canonical smoke.

STOP.

---

# Task 8 — Harden Go CI with race, vulnerability scanning, and dependency automation

**Goal:** Complete the already-active path-aware `go-api` CI surface with race/vulnerability evidence and Go dependency automation without duplicating test topology.

**Files:**
- Modify: `.github/workflows/ci.yml`
- Modify: `.github/dependabot.yml`

**Existing CI authority from earlier tasks:**

By Task 8, `go-api` already provides:

- exact Go toolchain;
- fmt/tidy cleanliness/mod verify/vet;
- unit/application/runtime tests;
- architecture harness + real architecture check;
- CI-only PostgreSQL service;
- migration integration;
- PostgreSQL adapter integration;
- stable `CI / gate` aggregation.

Task 8 hardens that job; it does not recreate it.

**Pinned inputs:**

```text
actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e # v7.0.0
Go 1.27.x reviewed patch
govulncheck v1.8.0
PostgreSQL service image postgres:18.6-alpine3.24
```

- [ ] **Step 1: Re-check patch pins**

Re-check Go 1.27.x and govulncheck compatible patch/version before editing CI. Patch-only updates are recorded in task evidence; major/minor changes return to design/plan review.

- [ ] **Step 2: Add race evidence to the existing Go job**

Run after ordinary DB-independent tests:

```bash
go test -race ./...
```

The integration-tagged PostgreSQL tests remain explicit and separate; do not accidentally include them in the race command unless a deliberate later decision proves that useful and stable.

- [ ] **Step 3: Add pinned govulncheck**

Install outside the application module dependency graph:

```bash
GOBIN="$RUNNER_TEMP/bin" go install golang.org/x/vuln/cmd/govulncheck@v1.8.0
"$RUNNER_TEMP/bin/govulncheck" ./...
```

Run from `apps/api`.

Do not add `golang.org/x/vuln` to application `go.mod`.

- [ ] **Step 4: Audit the final Go job order**

The final job order must make failures diagnosable:

```text
checkout/setup
  -> fmt
  -> tidy cleanliness
  -> mod verify
  -> vet
  -> architecture harness/check
  -> unit/application/runtime tests
  -> race
  -> migration integration
  -> adapter integration
  -> govulncheck
```

PostgreSQL service configuration remains CI-only.

- [ ] **Step 5: Audit aggregate-gate semantics**

`CI / gate` must still require:

```text
policy
repository
changes
local-dev
go-api
```

Rules:

- baseline jobs must succeed;
- `go-api` may be skipped only for an irrelevant diff;
- `local-dev` may be skipped only for an irrelevant diff;
- this implementation branch must produce both jobs as success once API Docker integration exists.

- [ ] **Step 6: Add Go Dependabot ecosystem**

Extend `.github/dependabot.yml`:

```text
package-ecosystem: gomod
directory: /apps/api
weekly schedule
```

Use the repository's existing weekly schedule style.

Do not add npm/Cargo ecosystems.

- [ ] **Step 7: GREEN**

At minimum:

```bash
./scripts/ci/test-detect-go-api-changes.sh
./scripts/ci/test-detect-local-dev-changes.sh
git diff --check
```

Statically verify:

- pinned checkout/setup-go;
- `persist-credentials: false`;
- `contents: read`;
- no `pull_request_target`;
- stable gate name exactly `CI / gate`;
- govulncheck version is exact;
- Go Dependabot points only at `/apps/api`.

- [ ] **Step 8: Self-review**

Check no duplicated workflow logic, no application dependency pollution from tooling, no secret exposure, and no relaxation of integration/local-dev evidence.

- [ ] **Step 9: Commit**

```text
ci(api): harden Go control plane verification
```

- [ ] **Step 10: Require fresh draft-PR CI**

For the exact Task 8 head:

```text
policy       SUCCESS
repository   SUCCESS
changes      SUCCESS
go-api       SUCCESS
local-dev    SUCCESS
CI / gate    SUCCESS
```

Inspect Go job logs for race + govulncheck in addition to all prior layers.

STOP.

---

# Task 9 — Update canonical documentation, perform whole-branch verification, and finalize the implementation PR

**Goal:** Reconcile documentation with the implemented runtime, run whole-branch verification against the reviewed design/plan, finalize the existing draft implementation PR, close external review, and control landing.

**Files:**
- Modify: `README.md`
- Modify: `docs/README.md`
- Modify: `docs/architecture/container-view.md`
- Modify: `docs/architecture/module-boundaries.md`
- Modify: `docs/architecture/data-ownership.md`
- Modify: `docs/devops/local-development.md`
- Create: `docs/backend/go-control-plane.md`
- Create: `docs/testing/go-monitoring-foundation.md`
- Update any other stale canonical implementation-state line discovered by explicit search, but do not rewrite unrelated architecture.

- [ ] **Step 1: Update implementation-state documentation**

Docs must state:

- Go Control Plane foundation is implemented;
- Monitoring register/get domain/application/persistence exists;
- API exposes only operational `/livez` + `/readyz`;
- no public product API yet;
- no internal checker API yet;
- no mutable monitor lifecycle yet;
- Rust/Web remain placeholders;
- Go exclusively owns PostgreSQL;
- explicit migration command;
- no API auto-migration.

- [ ] **Step 2: Document canonical local commands**

At minimum:

```bash
docker compose up -d --build --wait
docker compose ps
docker compose logs -f api

docker compose exec api /usr/local/bin/uptime-lab-migrate status
docker compose exec api /usr/local/bin/uptime-lab-migrate up

docker compose exec db sh -lc 'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB"'

docker compose down
docker compose down -v
```

Explain that `down -v` destroys local PostgreSQL state and is not the normal migration workflow.

Document host-run Go commands as optional; Docker remains the canonical local runtime.

- [ ] **Step 3: Document testing ownership**

`docs/testing/go-monitoring-foundation.md` maps evidence:

```text
domain -> unit
application -> fake-port unit
architecture -> go-list fitness fixtures
migration -> real PostgreSQL
adapter -> real PostgreSQL
runtime -> unit/lifecycle
Docker -> canonical Compose smoke
CI -> go-api + local-dev + CI / gate
```

- [ ] **Step 4: Run static scope review**

Main-to-branch diff MUST contain only approved implementation/verification/doc paths.

Explicitly reject:

```text
contracts/**
apps/web/**
apps/checker/**
root go.mod
root go.work
root package.json
root Cargo.toml
root migrations/**
```

Search for accidental scope terms:

```text
SetMonitorEnabled
enabled boolean
updated_at
/monitors
due work
check_runs
monitor_states
Kafka
RabbitMQ
Redis
OpenAPI product implementation
```

References in design/deferred documentation are allowed; production implementation is not.

- [ ] **Step 5: Run full Go verification**

Run this command set in an environment providing the reviewed Go 1.27.x toolchain. If the executing local environment does not provide that toolchain, do not claim these as local results; use the exact current-head `go-api` job as canonical evidence and inspect every corresponding step.

```bash
cd apps/api
test -z "$(gofmt -l .)"
go mod tidy
test -z "$(git status --porcelain -- go.mod go.sum)"
go mod verify
go vet ./...
go test ./...
go test -race ./...
govulncheck ./...
cd ../..

./scripts/ci/test-check-go-architecture.sh
./scripts/ci/check-go-architecture.sh .
./scripts/ci/run-go-postgres-tests.sh
```

Record exact versions:

```bash
go version
govulncheck -version
```

- [ ] **Step 6: Run full Docker verification**

Run locally only when a real compatible Docker/Compose environment is available. Regardless of local availability, the exact current-head `local-dev` job must execute the same canonical topology/smoke layer successfully; CI evidence is mandatory and is not replaced by a fake-Docker harness.

```bash
./scripts/ci/test-detect-local-dev-changes.sh
./scripts/ci/test-check-local-dev.sh
./scripts/ci/test-smoke-local-dev.sh
./scripts/ci/check-local-dev.sh .
./scripts/ci/smoke-local-dev.sh
docker compose config --quiet
```

Evidence must include:

- exact static topology case count;
- change detector counts;
- DB healthy;
- real API healthy through `/readyz`;
- checker becomes healthy after API;
- Web remains placeholder/independent;
- persistence/reset;
- cleanup.

- [ ] **Step 7: Run repository governance verification**

```bash
./scripts/ci/test-governance.sh
./scripts/github/test-configure-governance.sh
./scripts/ci/test-architecture-docs.sh
./scripts/ci/check-architecture-docs.sh .
./scripts/ci/check-repository-shape.sh .
./scripts/ci/check-pr-title.sh "feat(api): establish Go monitoring foundation"
git diff --check main...HEAD
```

Adapt the last command to the actual local base SHA when executed; do not compare against a stale branch.

- [ ] **Step 8: Whole-branch self-review**

Separate from task-level reviews.

Review the complete branch against all GMF decisions, with particular focus on:

- only Register/Get product behavior;
- no mutable lifecycle;
- no product endpoints;
- no SSRF-safety overclaim;
- no cross-runtime persistence access;
- no dead Monitoring wiring in production API;
- no auto-migration;
- pgx/goose isolation;
- real readiness;
- non-root/read-only Docker;
- CI result aggregation;
- documentation truthfulness;
- no accidental frameworks/abstractions.

If RED, fix within Task 9 using coherent commits and repeat all affected verification.

- [ ] **Step 9: Commit documentation**

```text
docs(api): document Go monitoring foundation
```

If Task 9 required a production/test fix, each fix gets its own Conventional Commit before the docs commit when that preserves review clarity.

- [ ] **Step 10: Finalize the existing draft implementation PR**

The PR already exists from Task 1:

```text
head: feat/go-monitoring-foundation
base: main
title: feat(api): establish Go monitoring foundation
```

Update the draft PR body to explicitly report:

- Architecture impact;
- Contract impact: no product/OpenAPI transport added;
- Database impact: monitoring schema/table + explicit migration;
- Security impact: TargetURL syntax vs execution-time SSRF boundary;
- Testing evidence;
- Docker evidence;
- Documentation impact;
- Breaking changes: None;
- Deferred lifecycle/concurrency note;
- exact Go/pgx/goose/govulncheck pins;
- per-task CI evidence summary from Tasks 1-8.

After the body reflects the current head, mark the PR review-ready. Do not change branch/base/title merely to retrigger CI.

- [ ] **Step 11: Require fresh PR CI**

Fresh PR CI must show:

```text
policy       SUCCESS
repository   SUCCESS
changes      SUCCESS
go-api       SUCCESS
local-dev    SUCCESS
CI / gate    SUCCESS
```

For this implementation PR, `go-api` and `local-dev` MUST NOT be skipped.

Inspect Go job logs for:

- fmt;
- tidy cleanliness;
- mod verify;
- vet;
- architecture;
- unit;
- race;
- migration integration;
- adapter integration;
- govulncheck.

Inspect local-dev logs for real API Docker health and full smoke.

- [ ] **Step 12: External review gate**

Request external review on the current implementation head.

Every actionable finding is independently verified against current code.

- valid finding -> fix -> self-review -> affected tests -> commit -> fresh CI;
- invalid/stale finding -> reply with evidence;
- no unresolved review threads before merge.

If configured reviewers are genuinely service-unavailable, do not silently bypass. Apply only the repository's documented service-unavailable exception pattern with explicit compensating evidence.

- [ ] **Step 13: Squash landing**

Only after:

- fresh PR CI green;
- whole-branch review green;
- external review gate closed;
- current head mergeable.

Squash title:

```text
feat(api): establish Go monitoring foundation
```

After merge, require fresh `main` push CI:

```text
policy       SUCCESS
repository   SUCCESS
changes      SUCCESS
go-api       SUCCESS
local-dev    SUCCESS
CI / gate    SUCCESS
```

Then close the Go Monitoring Foundation implementation phase.

Do NOT start public/internal contract implementation automatically.

---

## Final Exit Criteria

The Go Monitoring Foundation is complete only when all are true:

1. `apps/api` is the only Go module and uses the reviewed Go 1.27.x patch.
2. Monitoring implements only immutable registration/read semantics.
3. `TargetURL` tests prove design validation and no execution-safety overclaim exists.
4. Monitor IDs are Go-generated UUID v7.
5. Duplicate target URLs are permitted.
6. Repository port is create/read only.
7. PostgreSQL owns exactly the planned `monitoring.monitors` schema/table for this phase.
8. Migration up/down/up is real-PostgreSQL verified.
9. API startup never auto-migrates.
10. pgx adapter real-PostgreSQL round trip is green.
11. pgx errors do not leak as application contracts.
12. Go architecture fitness tests are green, including negative fixtures.
13. API exposes only `/livez` and `/readyz`.
14. `/livez` is DB-independent.
15. `/readyz` is bounded and DB-aware.
16. Production API binary does not dead-wire unconsumed Monitoring services.
17. API Docker image is pinned, multi-stage, non-root, and read-only-compatible.
18. Compose remains exactly four services with no host app ports.
19. Checker waits for real API health.
20. Web and Checker remain placeholders.
21. Go changes trigger `go-api` CI.
22. Go image/source changes trigger canonical local-dev smoke.
23. govulncheck is pinned and green.
24. Go Dependabot is configured only for `/apps/api`.
25. Canonical docs accurately describe implementation and deferrals.
26. Implementation PR fresh CI has `go-api`, `local-dev`, and `CI / gate` success.
27. External-review gate is closed.
28. Merge-after main CI is green.
29. Public/internal product contracts, lifecycle mutation, scheduling, Rust and React remain deferred.

---

## Plan Self-Review

### Scope alignment

PASS.

The plan implements only the landed Go Monitoring foundation. Public/internal contracts, mutable lifecycle, scheduling/results, Rust and Web remain deferred.

### Design / ADR consistency

PASS.

Go remains the Control Plane and only PostgreSQL owner. Cross-runtime contract rules are preserved. No browser/Rust database shortcut appears.

### Dependency direction

PASS.

Domain/application/ports/adapters/platform boundaries are introduced incrementally and mechanically checked before infrastructure complexity grows.

### YAGNI

PASS.

No ORM, sqlc, framework, DI container, event bus, transaction abstraction, lifecycle concurrency machinery, extra service, or speculative module is introduced.

### Docker compatibility

PASS.

The four-service topology remains. Only API becomes real. Product host ports remain absent.

### Persistence safety

PASS.

Schema is minimal, duplicate targets remain valid, migrations are explicit, and mutable write concurrency is deferred rather than under-specified.

### Verification layering

PASS.

Domain/application use cheap DB-independent tests; architecture uses package metadata; integration-tagged migration/adapter tests use real PostgreSQL; runtime uses focused lifecycle tests; Docker uses canonical Compose; CI aggregates Go and Docker evidence.

### CI/security

PASS.

Actions remain immutable-pinned/read-only. Go verification is path-aware. No secrets or privileged deployment behavior is introduced.

### Diff hygiene

PASS by plan.

The plan PR itself must contain only this plan document.

### Execution safety

PASS.

Each task has a RED/GREEN boundary, explicit self-review, atomic commit, and STOP point. Minimal path-aware Go CI and the draft implementation PR are established in Task 1 so every subsequent task can produce remote evidence on its exact head. The PR remains draft until the final whole-branch gate in Task 9.
