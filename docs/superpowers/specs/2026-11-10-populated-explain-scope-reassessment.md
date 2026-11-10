# Populated-Data EXPLAIN Evidence Scope Reassessment

**Date:** 2026-11-10
**Status:** Proposed scope and design direction; awaiting user review
**Verified baseline:** `main@352ee83`, after #59/#60/#64/#65/#68/#69/#70/#71/#72/#73/#74/#75/#76/#77/#78/#80
**Authorization:** Documentation only; no runtime implementation, migration, merge or deployment.

## Purpose and before/after

The inventory review that landed the bounded Monitor inventory retained one explicit limitation: existing index-existence checks qualify index *presence*, not query behavior over populated data. Every read that matters to a local operator — the bounded inventory page, the bounded history page, the latest-result read, the claim's candidate selection and its new retention statement — is served by indexes whose behavior over populated data has never been observed or recorded. The retention landing changed this question's shape: the durable dataset is now naturally bounded (newest 50 terminal rows per Monitor), so populated-data evidence finally has a well-defined, stable dataset to observe.

The proposed increment is an **evidence-only increment**: it adds no product behavior, no new operation, no schema change and no migration. It produces, records and pins query-plan evidence over a representative populated dataset, so the repository can state exactly what the exercised indexes do at the exercised scale — and, just as importantly, what remains unqualified (production capacity, supported Monitor counts, response-time objectives).

## Alternatives

| Approach | Benefit | Cost |
|---|---|---|
| Integration-tagged EXPLAIN evidence suite over a retention-bounded dataset (recommended) | Deterministic, repeatable evidence at the cheapest meaningful layer; asserts index usage per exercised query; records plans for review | Evidence is scale- and machine-relative — it qualifies index *usage*, not production capacity |
| Manual docs-only EXPLAIN run | No test surface changes | Not repeatable; plans drift from the schema; the retained limitation would remain formally open |
| New composite indexes now | Removes the tie-break sort concern preemptively | Premature: no measured need; the design docs explicitly route capacity to evidence first |

**Draft assumption requiring approval:** use the first approach. If a new composite index is wanted preemptively, revise scope before detailed design.

## Proposed rules

1. One integration-tagged evidence test (`-tags=integration`) exercises the four exercised queries over a populated dataset built by fixture preparation: a bounded set of Monitors (e.g. 25) each with a retention-bounded terminal window (50 terminal rows per Monitor — the retention cap), plus one pending row and one over-cap-retention scenario, so the exercised dataset matches the durable reality retention created.
2. The suite runs `EXPLAIN (ANALYZE, FORMAT TEXT)` through the real pgx pool against the four exercised query shapes: the inventory candidate query, the history page query (first page and anchored continuation), the latest-terminal-result read, and the claim's candidate selection with its retention statement. No product code changes.
3. Each plan is asserted to use the expected index: the inventory page on the monitors ordering index, the history page on `check_runs_latest_terminal_idx`, the latest-result read on the same terminal index, and the claim's candidate scan on the monitors ordering index; the retention statement's plan is recorded (its cost model is not asserted — deletion volume is data-dependent).
4. The full captured plans are written to a committed evidence document under `docs/testing/` with the dataset shape, the PostgreSQL version and the recorded plans verbatim; the evidence document states explicitly that index usage at the exercised scale is qualified and that production capacity, supported Monitor counts and response-time objectives remain unqualified.
5. The evidence suite is deterministic and bounded: fixture preparation uses fixed UUIDs and fixed instants, the dataset is removed by the existing per-test reset, and the suite runs in the existing real-PostgreSQL CI job unchanged. No new CI job, no new dependencies.
6. The plans are evidence, not gates: if a future schema or query change alters a plan, the evidence document is updated through the same increment that made the change — the suite fails when the asserted index usage disappears, tying the evidence to the schema it describes.

## Architecture and contract direction

Evidence lives where the queries live: the suite joins the existing real-PostgreSQL integration suites (same package, same fixtures, same job), so the plans are captured against the migrated schema in the same environment the functional evidence already uses. The evidence document under `docs/testing/` is updated through the same commits as the suite; no contract, generated-type or browser surface changes.

## Acceptance evidence required later

- RED/GREEN: the suite fails when an asserted index is absent (mutation: drop the index on a scratch copy) and passes over the retention-bounded populated dataset.
- The committed evidence document contains the four recorded plans with dataset shape and PostgreSQL version, reviewed for the asserted index usage.
- Required repository, Go/race/real-PostgreSQL (the evidence suite joins the existing job), Rust regression/audit, Web, Docker/browser and aggregate CI at the final implementation head; exact-main evidence after an authorized merge.

## Exclusions and next review

No new indexes, no query changes, no capacity SLOs, no supported-Monitor-count claims, no target-URL edit, no retention policy change, no incidents/notifications/ownership/remote access. The nine R0 decisions remain OPEN; the recorded evidence may inform a future R0-adjacent capacity discussion without deciding one.

Review the dataset shape (retention-bounded windows), the four exercised queries, the asserted index usage per query, and the evidence-document placement. Scope approval precedes detailed design; written design approval precedes an implementation plan. Product execution then follows TDD -> branch/commit -> PR. Merging this document records a proposal unless product decisions are separately approved.

## Evidence pointers

- [Current handoff](../../devops/current-handoff.md), [R0 decision brief](../../roadmap/2026-10-18-r0-product-release-decision-brief.md).
- `apps/api/internal/modules/monitoring/adapters/postgres/monitor_inventory.go`, `monitor_history.go`, `check_execution.go`: the exercised queries.
- `apps/api/migrations/00002_create_monitoring_check_runs.sql`: the terminal index the history and latest-result reads rely on.
- `docs/testing/local-monitoring-web.md`: the retained-limitation record this increment closes.
