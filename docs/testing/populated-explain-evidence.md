# Populated-Data EXPLAIN Evidence

**Recorded:** 2026-11-10
**Capture method:** the committed integration evidence suite (`TestExplainEvidenceOverPopulatedDataset`) run against the migrated schema inside the pinned CI-equivalent `golang:1.27.3-alpine3.24` container; the plans below are its `EXPLAIN (ANALYZE, FORMAT TEXT)` output verbatim, extracted from the suite log. Bound parameters render in PostgreSQL's parameterized form.
**Dataset:** 25 Monitors x 50 terminal rows each (newest kept per the retention cap), one pending row — 1250 terminal rows + 1 pending.
**PostgreSQL:** 18.6 (docker `postgres:18.6-alpine3.24`).

**Qualification boundary:** index *usage* at this exercised scale is qualified — the plans below are the recorded behavior of the exercised queries over this dataset. Production capacity, supported Monitor counts and response-time objectives remain unqualified; the plans are evidence, not an SLO.

## Asserted index usage

| Query | Asserted index | Recorded behavior |
|---|---|---|
| Inventory page | `monitors_inventory_order_idx` | Index Scan with incremental sort (created_at presorted; id tie-break sorted in-page) |
| History first page | `check_runs_latest_terminal_idx` | Bitmap Index Scan on the partial terminal index + top-N sort for the id tie-break |
| Latest terminal read | `check_runs_latest_terminal_idx` (via the lateral) | Index Scan per monitor inside the lateral; the three-key lateral tie-break is served by the index for the time key |
| History anchored page | `check_runs_latest_terminal_idx` | Bitmap Index Scan with the keyset filter applied as a heap filter + top-N tie-break sort |
| Claim candidate | `check_runs_latest_terminal_idx` (via the lateral) | Index Only Scan per monitor inside the anti-join; monitors Seq Scan (25 rows) — the candidate is asserted through the lateral's index usage |
| Retention statement | recorded, not asserted | Delete plan over the window-function rank (no ANALYZE — nothing deleted during capture) |

The per-page id tie-break sort visible in the history plans is the acknowledged trade-off from the history design: the terminal index serves the monitor and time predicate; a composite index including `check_id` remains a future option if measurement demands it.

## Recorded plans (verbatim suite output)

### inventory

```text
    monitor_explain_evidence_integration_test.go:194: Limit  (cost=0.58..2.78 rows=21 width=76) (actual time=0.101..0.103 rows=21.00 loops=1)
          Buffers: shared hit=8
          ->  Incremental Sort  (cost=0.58..101.41 rows=960 width=76) (actual time=0.100..0.101 rows=21.00 loops=1)
                Sort Key: created_at DESC, ((id)::text) DESC
                Presorted Key: created_at
                Full-sort Groups: 1  Sort Method: quicksort  Average Memory: 28kB  Peak Memory: 28kB
                Buffers: shared hit=8
                ->  Index Scan using monitors_inventory_order_idx on monitors  (cost=0.15..74.55 rows=960 width=76) (actual time=0.015..0.021 rows=25.00 loops=1)
                      Index Searches: 1
                      Buffers: shared hit=2
        Planning:
          Buffers: shared hit=40
        Planning Time: 0.315 ms
        Execution Time: 0.130 ms
```

### history-first

```text
    monitor_explain_evidence_integration_test.go:194: Limit  (cost=16.08..16.09 rows=5 width=80) (actual time=0.055..0.057 rows=21.00 loops=1)
          Buffers: shared hit=3
          ->  Sort  (cost=16.08..16.09 rows=5 width=80) (actual time=0.055..0.055 rows=21.00 loops=1)
                Sort Key: completed_at DESC, ((id)::text) DESC
                Sort Method: top-N heapsort  Memory: 27kB
                Buffers: shared hit=3
                ->  Bitmap Heap Scan on check_runs  (cost=4.32..16.02 rows=5 width=80) (actual time=0.037..0.042 rows=50.00 loops=1)
                      Recheck Cond: ((monitor_id = '018f22d3-1d6a-7cc0-a37b-46fc3faf6000'::uuid) AND (completed_at IS NOT NULL))
                      Heap Blocks: exact=1
                      Buffers: shared hit=3
                      ->  Bitmap Index Scan on check_runs_latest_terminal_idx  (cost=0.00..4.31 rows=5 width=0) (actual time=0.021..0.021 rows=50.00 loops=1)
                            Index Cond: (monitor_id = '018f22d3-1d6a-7cc0-a37b-46fc3faf6000'::uuid)
                            Index Searches: 1
                            Buffers: shared hit=2
        Planning:
          Buffers: shared hit=9
        Planning Time: 0.171 ms
        Execution Time: 0.088 ms
```

### history-anchored

```text
    monitor_explain_evidence_integration_test.go:194: Limit  (cost=10.23..10.24 rows=2 width=80) (actual time=0.026..0.027 rows=21.00 loops=1)
          Buffers: shared hit=3
          ->  Sort  (cost=10.23..10.24 rows=2 width=80) (actual time=0.025..0.026 rows=21.00 loops=1)
                Sort Key: completed_at DESC, ((id)::text) DESC
                Sort Method: top-N heapsort  Memory: 27kB
                Buffers: shared hit=3
                ->  Bitmap Heap Scan on check_runs  (cost=4.30..10.22 rows=2 width=80) (actual time=0.009..0.014 rows=50.00 loops=1)
                      Recheck Cond: ((monitor_id = '018f22d3-1d6a-7cc0-a37b-46fc3faf6000'::uuid) AND (completed_at IS NOT NULL))
                      Filter: (ROW(completed_at, id) < ROW('2026-11-10 10:30:00+00'::timestamp with time zone, '7d9b2eb0-52bb-4dd7-8934-c5ce30d5c3e7'::uuid))
                      Heap Blocks: exact=1
                      Buffers: shared hit=3
                      ->  Bitmap Index Scan on check_runs_latest_terminal_idx  (cost=0.00..4.30 rows=2 width=0) (actual time=0.003..0.003 rows=50.00 loops=1)
                            Index Cond: ((monitor_id = '018f22d3-1d6a-7cc0-a37b-46fc3faf6000'::uuid) AND (completed_at <= '2026-11-10 10:30:00+00'::timestamp with time zone))
                            Index Searches: 1
                            Buffers: shared hit=2
        Planning:
          Buffers: shared hit=12
        Planning Time: 0.100 ms
        Execution Time: 0.097 ms
```

### latest-terminal

```text
    monitor_explain_evidence_integration_test.go:194: Nested Loop Left Join  (cost=5.26..17.19 rows=1 width=80) (actual time=0.045..0.046 rows=1.00 loops=1)
          Buffers: shared hit=5
          ->  Index Only Scan using monitors_pkey on monitors monitor  (cost=0.15..8.17 rows=1 width=16) (actual time=0.023..0.024 rows=1.00 loops=1)
                Index Cond: (id = '018f22d3-1d6a-7cc0-a37b-46fc3faf6000'::uuid)
                Heap Fetches: 1
                Index Searches: 1
                Buffers: shared hit=2
          ->  Limit  (cost=5.11..9.00 rows=1 width=72) (actual time=0.020..0.020 rows=1.00 loops=1)
                Buffers: shared hit=3
                ->  Incremental Sort  (cost=5.11..24.59 rows=5 width=72) (actual time=0.020..0.020 rows=1.00 loops=1)
                      Sort Key: run.completed_at DESC, run.issued_at DESC, run.id DESC
                      Presorted Key: run.completed_at
                      Full-sort Groups: 1  Sort Method: quicksort  Average Memory: 25kB  Peak Memory: 25kB
                      Buffers: shared hit=3
                      ->  Index Scan using check_runs_latest_terminal_idx on check_runs run  (cost=0.28..24.37 rows=5 width=72) (actual time=0.009..0.009 rows=2.00 loops=1)
                            Index Cond: (monitor_id = monitor.id)
                            Index Searches: 1
                            Buffers: shared hit=3
        Planning:
          Buffers: shared hit=10
        Planning Time: 0.166 ms
        Execution Time: 0.083 ms
```

### claim-candidate

```text
    monitor_explain_evidence_integration_test.go:194: Limit  (cost=2489.32..2489.33 rows=1 width=132) (actual time=0.250..0.251 rows=1.00 loops=1)
          Buffers: shared hit=76
          ->  LockRows  (cost=2489.32..2491.33 rows=161 width=132) (actual time=0.250..0.251 rows=1.00 loops=1)
                Buffers: shared hit=76
                ->  Sort  (cost=2489.32..2489.72 rows=161 width=132) (actual time=0.246..0.247 rows=1.00 loops=1)
                      Sort Key: (COALESCE(terminal.latest_completed_at, monitor.created_at)), monitor.id
                      Sort Method: quicksort  Memory: 28kB
                      Buffers: shared hit=75
                      ->  Nested Loop Left Join  (cost=16.18..2488.51 rows=161 width=132) (actual time=0.138..0.225 rows=24.00 loops=1)
                            Filter: ((terminal.latest_completed_at IS NULL) OR (terminal.latest_completed_at <= '2026-11-10 11:59:00+00'::timestamp with time zone))
                            Buffers: shared hit=75
                            ->  Hash Anti Join  (cost=15.90..41.54 rows=478 width=68) (actual time=0.103..0.109 rows=24.00 loops=1)
                                  Hash Cond: (monitor.id = pending.monitor_id)
                                  Buffers: shared hit=3
                                  ->  Seq Scan on monitors monitor  (cost=0.00..19.60 rows=480 width=62) (actual time=0.022..0.025 rows=25.00 loops=1)
                                        Filter: (NOT paused)
                                        Buffers: shared hit=1
                                  ->  Hash  (cost=15.84..15.84 rows=5 width=22) (actual time=0.020..0.020 rows=1.00 loops=1)
                                        Buckets: 1024  Batches: 1  Memory Usage: 9kB
                                        Buffers: shared hit=2
                                        ->  Bitmap Heap Scan on check_runs pending  (cost=4.17..15.84 rows=5 width=22) (actual time=0.006..0.006 rows=1.00 loops=1)
                                              Recheck Cond: (completed_at IS NULL)
                                              Heap Blocks: exact=1
                                              Buffers: shared hit=2
                                              ->  Bitmap Index Scan on check_runs_one_pending_per_monitor_idx  (cost=0.00..4.17 rows=5 width=0) (actual time=0.002..0.002 rows=1.00 loops=1)
                                                    Index Searches: 1
                                                    Buffers: shared hit=1
                            ->  Subquery Scan on terminal  (cost=0.28..5.11 rows=1 width=40) (actual time=0.004..0.004 rows=1.00 loops=24)
                                  Buffers: shared hit=72
                                  ->  Limit  (cost=0.28..5.10 rows=1 width=8) (actual time=0.004..0.004 rows=1.00 loops=24)
                                        Buffers: shared hit=72
                                        ->  Index Only Scan using check_runs_latest_terminal_idx on check_runs run  (cost=0.28..24.37 rows=5 width=8) (actual time=0.004..0.004 rows=1.00 loops=24)
                                              Index Cond: (monitor_id = monitor.id)
                                              Heap Fetches: 24
                                              Index Searches: 24
                                              Buffers: shared hit=72
        Planning:
          Buffers: shared hit=7
        Planning Time: 0.202 ms
        Execution Time: 0.383 ms
```

### retention

```text
    monitor_explain_evidence_integration_test.go:194: Delete on check_runs  (cost=119.25..152.19 rows=0 width=0) (actual time=0.638..0.639 rows=0.00 loops=1)
          Buffers: shared hit=18
          ->  Hash Join  (cost=119.25..152.19 rows=343 width=46) (actual time=0.638..0.638 rows=0.00 loops=1)
                Hash Cond: (check_runs.id = ranked.id)
                Buffers: shared hit=18
                ->  Seq Scan on check_runs  (cost=0.00..26.40 rows=1035 width=22) (actual time=0.005..0.005 rows=1.00 loops=1)
                      Filter: (completed_at IS NOT NULL)
                      Buffers: shared hit=2
                ->  Hash  (cost=117.07..117.07 rows=175 width=56) (actual time=0.627..0.628 rows=0.00 loops=1)
                      Buckets: 1024  Batches: 1  Memory Usage: 8kB
                      Buffers: shared hit=16
                      ->  HashAggregate  (cost=115.32..117.07 rows=175 width=56) (actual time=0.627..0.627 rows=0.00 loops=1)
                            Group Key: ranked.id
                            Batches: 1  Memory Usage: 32kB
                            Buffers: shared hit=16
                            ->  Subquery Scan on ranked  (cost=78.25..114.45 rows=345 width=56) (actual time=0.625..0.626 rows=0.00 loops=1)
                                  Filter: (ranked.rank > '50'::bigint)
                                  Rows Removed by Filter: 1250
                                  Buffers: shared hit=16
                                  ->  WindowAgg  (cost=78.25..101.52 rows=1035 width=48) (actual time=0.386..0.587 rows=1250.00 loops=1)
                                        Window: w1 AS (PARTITION BY check_runs_1.monitor_id ORDER BY check_runs_1.completed_at, check_runs_1.id ROWS UNBOUNDED PRECEDING)
                                        Storage: Memory  Maximum Storage: 17kB
                                        Buffers: shared hit=16
                                        ->  Sort  (cost=78.23..80.82 rows=1035 width=40) (actual time=0.377..0.408 rows=1250.00 loops=1)
                                              Sort Key: check_runs_1.monitor_id, check_runs_1.completed_at DESC, check_runs_1.id DESC
                                              Sort Method: quicksort  Memory: 117kB
                                              Buffers: shared hit=16
                                              ->  Seq Scan on check_runs check_runs_1  (cost=0.00..26.40 rows=1035 width=40) (actual time=0.028..0.131 rows=1250.00 loops=1)
                                                    Filter: (completed_at IS NOT NULL)
                                                    Rows Removed by Filter: 1
                                                    Buffers: shared hit=16
        Planning:
          Buffers: shared hit=4
        Planning Time: 0.124 ms
        Execution Time: 0.840 ms
```

