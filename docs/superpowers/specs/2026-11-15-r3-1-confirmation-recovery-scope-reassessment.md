# R3-1 Confirmation/Recovery Scope Reassessment

**Date:** 2026-11-15
**Status:** Proposed scope and design direction; awaiting user review
**Verified baseline:** `main@5d602f4`, after #90/#91/#92/#93/#94
**Authorization:** Documentation only; no runtime implementation, migration, merge or deployment.

## Purpose and before/after

Today `EvaluateAvailability` classifies only the single latest terminal observation: one transient failure (timeout, connect error, non-2xx response) immediately asserts `unavailable`, and one success immediately asserts `available`. A flapping target therefore produces alternating up/down assessments with no notion of a confirmed outage or a sustained recovery, and a single blip is indistinguishable from a real outage.

The proposed increment separates a probe fact from a confirmed target outage: N consecutive terminal target-failures confirm `unavailable`, and M consecutive terminal target-successes recover to `available`; below-threshold streaks are surfaced as provisional rather than asserted as outage or recovery. The slice is **assessment-only**: the streak is derived at read time from already-durable retained CheckRun evidence. No incident table, no counters, no new write path and no notification are introduced.

## The design tension (explicit)

Where does consecutive-streak state live?

1. **Derive at assessment time from retained terminal `check_runs`** (newest-50 window) — no new persistence; the assessment remains a pure read over durable evidence. Cost: lookback is bounded by retention and the availability read examines up to the threshold rows instead of one.
2. **Durable per-Monitor confirmation state** (counters or incident open/resolve rows) — a new migration plus a write path on claim completion; enables unbounded confirmation and incident history. Heavier, and belongs to a later slice.
3. **Keep single-observation classification** — no confirmation; transient failures keep asserting outage.

**Draft assumption requiring approval:** approach 1. The first R3 slice stops at the confirmed assessment; durable incident open/resolve state is explicitly deferred to a later slice (the roadmap R3-1 design decision resolved toward assessment-only for slice 1). If unbounded confirmation or incident history is wanted now, revise scope before detailed design.

## Key semantics questions this scope must answer

- **Which terminal kinds are target outcomes.** Success = `http_response` with 2xx; failure = `http_response` non-2xx plus `dns_error`/`timeout`/`connect_error`/`tls_error`/`protocol_error` (the current unavailable set). Inconclusive = `policy_rejected`/`internal_error`/`worker_timeout` (the current unknown set). Draft: an inconclusive latest result keeps its unknown precedence, and inconclusive rows reset the streak — conservative, because neither outage nor recovery can be confirmed while the target is not observable.
- **Below-threshold status and reason.** Draft: a failure streak shorter than N yields `unknown` with a new reason `awaiting_confirmation` (do not assert outage); after a confirmed outage, a success streak shorter than M yields `unavailable` with a new reason `recovering` (stay down until recovered); at M, `available/successful_response`. A first-ever success needs no confirmation (`available` immediately): confirmation gates only the down transition, recovery gates only the up-after-outage transition.
- **Freshness and no-result precedence preserved.** `future_result`, `stale_result` (>120s) and `no_result` still yield `unknown` regardless of any streak; a stale latest observation can neither confirm nor recover. The evidence object still references the latest terminal CheckRun.
- **Retention-bounded lookback.** The streak is computed newest-first over retained terminal rows; if the newest-50 window truncates before N failures, confirmation is unreachable and the assessment stays `awaiting_confirmation`. Thresholds must be <= 50 and are fixed local conventions (draft N=3, M=2), not per-Monitor configurable policy and not an admitted workload cap (R0-03).
- **Snapshot semantics.** The assessment is recomputed on refresh; concurrent claim completions can change the streak between reads (the existing snapshot rule). No new write path and no locking beyond the existing read transaction.
- **Pause freezes the streak** (no new claims while paused); a prolonged pause lets the >120s staleness rule dominate (`unknown/stale_result`). Documented behavior, not a schema change.
- **Edit and delete interaction.** History mixes targets across a target edit, so a streak may span an edit (documented limitation mirroring history-mixing); deletion removes the evidence and returns `no_result`.
- **Contract surface.** The reason vocabulary extends by exactly two (`awaiting_confirmation`, `recovering`); the status vocabulary and the closed response shape are unchanged. No new public operation, no new result kinds, Rust and the internal contract unchanged, no migration, readiness set stays at four.

## Alternatives

| Approach | Benefit | Cost |
|---|---|---|
| Derive the streak from retained evidence at read time (recommended) | No new persistence or migration; assessment stays a pure read; reuses durable evidence and the existing snapshot rule | Lookback bounded by newest-50; availability read examines up to threshold rows |
| Durable counters / incident open-resolve state | Unbounded confirmation; incident history | New migration plus a completion-time write path; heavier; deferred to a later slice |
| Keep single-observation classification | Simplest; no change | Transient failures keep asserting outage; no confirmation/recovery product value |

## Proposed rules

1. The availability assessment gains confirmation/recovery over consecutive terminal target outcomes derived newest-first from retained `monitoring.check_runs`; no new table, column, migration or write path.
2. Fixed local conventions: N=3 consecutive target-failures confirm `unavailable`; M=2 consecutive target-successes recover to `available`. Both are repository-owned local conventions (R0-03), not per-Monitor policy and not an admitted cap; both <= the retention window (50).
3. Success = `http_response` 2xx; failure = `http_response` non-2xx and `dns_error`/`timeout`/`connect_error`/`tls_error`/`protocol_error`; inconclusive = `policy_rejected`/`internal_error`/`worker_timeout`. An inconclusive latest result keeps unknown precedence; inconclusive rows reset the streak.
4. A below-threshold failure streak yields `unknown/awaiting_confirmation`; a confirmed outage keeps the existing failure reason (`unexpected_http_status`/`probe_failure`) reflecting the latest fact; a post-outage success streak below M yields `unavailable/recovering`; at M, `available/successful_response`. A first success without a prior confirmed outage yields `available/successful_response` immediately.
5. Freshness and no-result precedence are unchanged (`future_result`, `stale_result` >120s, `no_result` -> `unknown`); the evidence object still references the latest terminal CheckRun.
6. No new public operation; the availability response shape is unchanged and the reason vocabulary grows by two (contract-visible, requiring contract and generated-parity updates); `no-store`, `Allow` and `HEAD` rules unchanged; gateway and browser controls unchanged (the browser renders the new reasons).
7. Retention, pause/resume, delete, edit, history and latest-result semantics are unchanged; streak lookback is bounded by newest-50 (documented limitation); pause freezes the streak; a target edit may mix targets within a streak (documented limitation).
8. Rust and the internal contract are unchanged; no new result kinds; the readiness set stays at four.

## Acceptance evidence required later

- Contract RED/GREEN for the extended reason vocabulary, unchanged closed shape and generated parity, and unchanged existing operations.
- Domain RED/GREEN for the streak policy: thresholds, inconclusive reset, freshness/no-result precedence, immediate first success, recovery gating, retention-truncated lookback.
- Real PostgreSQL RED/GREEN: newest-first streak derivation over retained rows; retention truncation preventing confirmation; pause-frozen streak; edit mixing; delete -> `no_result`; concurrent-completion snapshot behavior; absence of any new write path.
- HTTP/gateway RED/GREEN: unchanged `no-store`/`Allow`/`HEAD`, new reasons serialized, sanitized errors.
- Web tests: `awaiting_confirmation` and `recovering` render distinctly; refresh recomputes; no new controls.
- Composed Chromium on 4173/4817: a single transient failure does not assert outage; sustained failures confirm `unavailable`; sustained successes recover to `available`; across an API restart.
- Required repository, contract, Go/race/real-PostgreSQL, Rust regression/audit, Web, Docker/browser and aggregate CI at the final implementation head; exact-main evidence after an authorized merge.

## Exclusions and next review

No durable incident open/resolve state, no notification delivery, no uptime percentages, no maintenance windows, no per-Monitor configurable thresholds, no aggregation, no ownership or remote access. R0-04 supplies the detection expectation (local detection, confirmation/recovery direction, no alert-delivery guarantee); the other R0 decisions remain as recorded for the Path-A local-evaluation scope.

Review the approach-1 decision (derive, no new persistence), the N/M draft values, the below-threshold status/reason choices, the inconclusive-reset rule and the retention-bounded lookback. Scope approval precedes detailed design; written design approval precedes an implementation plan; product execution then follows TDD -> branch/commit -> PR. Merging this document records a proposal unless product decisions are separately approved.

## Evidence pointers

- [Current handoff](../../devops/current-handoff.md), [R0 decision brief](../../roadmap/2026-10-18-r0-product-release-decision-brief.md), [post-R2 roadmap R3-1](../../roadmap/2026-11-13-post-r2-roadmap.md).
- `apps/api/internal/modules/monitoring/domain/availability.go`: the single-observation policy and freshness/precedence rules to extend.
- `apps/api/internal/modules/monitoring/domain/check_result.go`: the result-kind vocabulary.
- `apps/api/internal/modules/monitoring/adapters/postgres/monitor_history.go`: newest-first retained terminal page.
- `apps/api/internal/modules/monitoring/adapters/postgres/retention.go`: the newest-50 retained window bounding lookback.
- `apps/api/internal/modules/monitoring/adapters/postgres/latest_check_result.go`: the latest terminal evidence read.
- `contracts/openapi/public.yaml`: the availability reason enum to extend.