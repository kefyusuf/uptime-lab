# Local History Retention Design

**Date:** 2026-11-07
**Status:** Written design for review; implementation not authorized
**Baseline:** `main@24b030df`; scope proposal merged through [PR #75](https://github.com/kefyusuf/uptime-lab/pull/75)
**Stage decision:** The user's approved continuation after the scope proposal authorizes this written design. It does not approve an implementation plan, merge or deployment.

## 1. Intent and boundaries

Bound the retained terminal evidence per Monitor: Go removes terminal CheckRuns beyond the newest 50 per Monitor inside the existing claim transaction, so the durable window never grows past one full history page. No retention policy decision is made (R0-07 stays OPEN), no configuration is added, no background reconciler or scheduler is introduced, and no read or mutation semantics outside the removed rows change. Rust and the internal Checker contract remain unchanged; no migration is added and the readiness set stays at four.

## 2. Retention semantics

Retention keeps the newest 50 terminal rows per Monitor, ordered by `completed_at DESC, id DESC`, and permanently removes terminal rows ranking below 50 within their Monitor's partition. Pending rows are never ranked and never removed — the ranking and the removal both filter `completed_at IS NOT NULL`. The cap is a repository-owned constant (50) asserted by the checker and mirrored in the contract description; it is not configurable at runtime.

Removed rows vanish from the bounded history read under its existing keyset predicate (positions, not snapshots), and the latest-result read is unaffected unless the removed row was itself the latest terminal row — in which case the previous terminal row becomes the latest, and the availability assessment follows the existing freshness policy against that row. Paused Monitors' windows do not advance while no claims occur; retention runs only inside claims, which paused Monitors never trigger for themselves. Deletion of a Monitor still removes all of its rows atomically and is unaffected.

## 3. Enforcement point and statement

The claim transaction already opens with a global reconciliation phase (expired pending rows terminalized) before the candidate selection. Retention adds one statement to that phase, after the expired-pending reconciliation:

```sql
DELETE FROM monitoring.check_runs
WHERE completed_at IS NOT NULL
  AND id IN (
    SELECT id FROM (
      SELECT id,
             row_number() OVER (PARTITION BY monitor_id
                                ORDER BY completed_at DESC, id DESC) AS rank
      FROM monitoring.check_runs
      WHERE completed_at IS NOT NULL
    ) ranked
    WHERE rank > 50
  )
```

The statement is global (all Monitors) because the reconciliation phase is already global; claims are the only writer that periodically touches every Monitor's evidence, and reusing them keeps a single Go-owned deletion path. The window function ranks terminal rows per Monitor newest-first; rows ranking above the cap are removed. Pending rows are excluded by the inner filter, giving pending-row immunity structurally rather than by exception handling. No locks are taken beyond the rows actually deleted; the statement cannot conflict with the claim's later `FOR UPDATE OF monitor SKIP LOCKED` candidate scan, which runs after this statement in the same transaction and takes only Monitor-row locks. No reverse lock path is introduced.

## 4. Consistency and interaction rules

| Ordering | Required outcome |
|---|---|
| Claim with retention runs while a completion is in flight | Completion's transaction holds its CheckRun row lock; retention skips that row (row lock) and removes only unlocked over-cap rows; the completion commits normally |
| Retention removes a row a history read anchored on | The anchored read's keyset predicate returns strictly older rows or an empty continuation — no error (matching the history design's corrected removed-anchor semantics) |
| Retention removes the latest terminal row of a Monitor | Impossible: the newest row per Monitor ranks 1 and is never removed |
| A Monitor's row count is at or below 50 | Retention removes nothing |
| Deletion racing a claim with retention | Unchanged from the landed deletion semantics: the delete's serialized transaction and the claim's Monitor-row locking resolve as landed; retention adds no new lock path |
| History read during retention | Positions, not snapshots: rows removed between reads yield strictly older rows or empty continuations |

The in-flight-completion interaction deserves emphasis: a completing worker holds its CheckRun row lock (`FOR UPDATE` in `CompleteCheck`) while retention attempts to delete an over-cap row set that may include that row. PostgreSQL resolves the conflict by blocking retention's delete on the locked row; once the completion commits, the row is terminal and retention removes it if it still ranks over the cap — a benign race the integration suite must exercise deterministically with parked locks, not sleeps.

## 5. Public contract and browser

No new operation, schema or fixture. The contract's history operation description gains one explicit sentence: the retained window keeps the newest 50 terminal rows per Monitor, so older rows disappear over time. Generated types regenerate without structural change. The browser history section, scheduling and deletion controls are unchanged: positions, not snapshots.

## 6. Required acceptance and evidence limits

- Real PostgreSQL RED/GREEN: a Monitor with 55 terminal rows converges to exactly 50 newest after one claim; rows 51–55 are removed; pending rows survive; a second Monitor's rows are untouched; equal-completion ties rank by `check_id DESC`; the latest terminal row is never removed; the history read pages correctly over the retained window at the 20/50 boundaries.
- Mutation checks: dropping the `rank > 50` filter (no removal), removing the pending immunity (`completed_at IS NOT NULL` filters) and dropping the `PARTITION BY monitor_id` (global cap instead of per-Monitor) must each reproduce specific failures.
- Deterministic in-flight-completion evidence: a parked CheckRun row lock (test-held transaction + `pg_locks` observation) with retention and completion racing, resolved as landed without deadlock or lost completion.
- Claim/completion/pause-resume/deletion/history regression suites remain green; HTTP/gateway regression for the history read unchanged.
- Required repository, Go/race/real-PostgreSQL, Rust regression/audit, Web, Docker/browser and aggregate CI at the final implementation head; exact-main evidence after an authorized merge. The populated-data EXPLAIN minor remains separate.

## 7. Review and next stage

Review this written design, especially the global-per-claim retention scope, the window-function statement, pending-row immunity, the in-flight-completion race and the explicit R0-07 non-decision. Only after design approval may the implementation plan be produced; plan review precedes Native TDD execution. No deployment, merge or external-service change follows automatically from these approvals.
