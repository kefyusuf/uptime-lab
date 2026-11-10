# Local Target-URL Edit Scope Reassessment

**Date:** 2026-11-10
**Status:** Proposed scope and design direction; awaiting user review
**Verified baseline:** `main@5c931a2`, after #59/#60/#64/#65/#68/#69/#70/#71/#72/#73/#74/#75/#76/#77/#78/#80/#81/#82/#83
**Authorization:** Documentation only; no runtime implementation, migration, merge or deployment.

## Purpose and before/after

A local operator who registers a Monitor with a wrong or changed target URL has exactly one remedy today: delete the Monitor (losing its identity and all recorded evidence) and register a new one. This is the last user-visible gap in the R2 management surface and the reason the landed history read cannot show a continuous evidence trail for a corrected target.

The proposed increment adds a durable **target-URL edit** on the existing Monitor identity: the operator corrects the target through a guarded detail-page control, the Monitor keeps its UUID, creation time, history, scheduling state and evidence, and future claims probe the new target. This is the one increment that deliberately changes the immutable-Monitor design — the identity (UUID) stays immutable while the target becomes a validated mutable field — so the scope surfaces that decision explicitly before any design work.

## The design tension (explicit)

The domain's `Monitor` is a closed immutable value: `NewMonitor(id, targetURL, createdAt)` validates once and never mutates. Editing the target means one of:

1. **A domain-owned target replacement** — the Monitor keeps its identity; the domain gains a validated `WithTargetURL` transformation returning a new immutable Monitor value. Read semantics, history and scheduling are unchanged; the edit is a single atomic `UPDATE ... RETURNING` like the landed pause/resume and deletion writes.
2. **A revision/replace model** — editing creates a new Monitor identity and retires the old one, preserving strict immutability. Costs a new identity link, a more complex history story and none of the user-visible simplicity.
3. **Keep the current state** — no edit; operators delete and re-register, losing evidence.

**Draft assumption requiring approval:** approach 1. The identity stays immutable; the target becomes the single mutable field. If identity-level immutability is preferred, revise scope before detailed design.

## Key semantics questions this scope must answer

- **In-flight claims probe the old target.** A claim snapshots the target at claim time (`ClaimedCheck.TargetURL`); already claimed work completes against the old target and its rows remain durable evidence of the monitor — the same already-claimed-completes rule pause/resume established. The design must state this as bounded, documented behavior.
- **History mixes targets.** The history read has no per-row target field; rows before and after an edit are indistinguishable in the page. Documented limitation, not a schema change.
- **Edit while paused is allowed** — scheduling state and target are independent.
- **Edit validation is identical to registration** (`TargetURL` syntax; policy rejection stays a probe-time fact, not an edit-time one — SSRF is execution-safety, not registration).
- **Concurrent edit vs claim vs delete vs pause/resume** serialize on the same Monitor row like every other mutation.

## Alternatives

| Approach | Benefit | Cost |
|---|---|---|
| Target edit on the same identity via a validated domain transformation (recommended) | Fixes the operator gap; identity, history and scheduling survive; one atomic row write | The immutable-Monitor design gains one mutable field; in-flight claims finish against the old target |
| Supersede-and-link model | Strict immutability with provenance | New identity semantics, history split across identities, heavier design |
| Restrict edit to paused Monitors only | Simpler in-flight story | Arbitrary product rule; pause would become a prerequisite for a typo fix |

## Proposed rules

1. A tenth public operation — `PATCH /monitors/{monitorId}` with a closed `{targetUrl}` body — replaces the target atomically: one explicit Read Committed transaction, one `UPDATE monitoring.monitors SET target_url = $2 WHERE id = $1 RETURNING ...` with not-found detection, commit before success. No existence preflight.
2. The new target passes the exact registration validation (`TargetURL` syntax); policy rejection remains a probe-time fact. Edit while paused is allowed.
3. Already claimed work completes against the target captured at claim time; its rows remain durable evidence under the Monitor. Documented limitation, mirrored on the pause/resume already-claimed rule.
4. History, creation time, scheduling state, pause/resume, deletion and retention semantics are unchanged; the history read gains no per-row target field (documented limitation).
5. Success is 200 with the updated closed Monitor shape (identity, new target, unchanged creation time); `no-store` on every matched response; unknown Monitor 404; invalid body/target 400/422; methods other than PATCH 405 with exact `Allow: GET, PATCH`; HEAD/OPTIONS unsupported.
6. Gateway: PATCH joins DELETE as a guarded mutation with exact Host/Origin, body-buffered forwarding, unfinished-body deadline and sanitized errors; the browser edit control follows the deletion control's two-step confirmation and uncertain-outcome pattern (edit failure is uncertain, no automatic retry, refresh resolves).
7. No new migration: `target_url` exists; no new result kinds; Rust and the internal contract are unchanged; the readiness set stays at four.
8. The domain gains the validated transformation; the closed Monitor shape's JSON contract changes only in the target value it returns, not in its schema.

## Acceptance evidence required later

- Contract RED/GREEN for the closed operation, unchanged existing schemas and generated parity.
- Domain RED/GREEN for the validated target transformation and unchanged immutable identity.
- Real PostgreSQL RED/GREEN: atomic edit with not-found detection, edit-vs-claim ordering (in-flight claims finish against the old target), edit-vs-pause/resume serialization, edit-vs-delete race, edit while paused, unchanged history/latest/availability semantics, retention unchanged.
- HTTP/gateway RED/GREEN for the PATCH mutation: Origin/body-buffering/deadline guards, grammar, no-store, Allow, sanitized errors, aliases without redirect.
- Web tests: two-step edit control, uncertain-outcome handling, independence from scheduling/deletion/history views, stale fencing.
- Composed Chromium on 4173/4817: edit a target, verify future claims probe the new target and old evidence rows remain, across an API restart.
- Required repository, contract, Go/race/real-PostgreSQL, Rust regression/audit, Web, Docker/browser and aggregate CI at the final implementation head; exact-main evidence after an authorized merge.

## Exclusions and next review

No history per-row target annotations, no revision model, no undelete, no bulk edits, no aggregation, no incidents/notifications/ownership/remote access, no configurable caps. R0-07 and the other eight R0 decisions remain OPEN.

Review the approach-1 decision (identity immutable, target mutable), the in-flight old-target rule, the history-mixing limitation and the PATCH operation shape. Scope approval precedes detailed design; written design approval precedes an implementation plan. Product execution then follows TDD -> branch/commit -> PR. Merging this document records a proposal unless product decisions are separately approved.

## Evidence pointers

- [Current handoff](../../devops/current-handoff.md), [R0 decision brief](../../roadmap/2026-10-18-r0-product-release-decision-brief.md).
- `apps/api/internal/modules/monitoring/domain/monitor.go`: the closed immutable Monitor.
- `apps/api/internal/modules/monitoring/adapters/postgres/monitor_scheduling.go`: the established atomic single-row write pattern.
- `apps/api/internal/modules/monitoring/adapters/postgres/check_execution.go`: the claim's target snapshot (in-flight old-target rule).
- `apps/web/src/features/delete-monitor/`: the two-step confirmation and uncertain-outcome pattern to mirror.
