# Populated-Data EXPLAIN Evidence

**Recorded:** 2026-11-10
**Dataset:** 25 Monitors x 50 terminal rows each (newest kept per the retention cap), one pending row, total 1250 terminal rows + 1 pending.
**PostgreSQL:** 18.6 (docker `postgres:18.6-alpine3.24`), captured through the integration evidence suite's query copies over the migrated schema.

**Qualification boundary:** index *usage* at this exercised scale is qualified — the plans below are the recorded behavior of the exercised queries over this dataset. Production capacity, supported Monitor counts and response-time objectives remain unqualified; the plans are evidence, not an SLO.

## Asserted index usage

| Query | Asserted index | Recorded behavior |
|---|---|---|
| Inventory page | `monitors_inventory_order_idx` | Index Scan with incremental sort (created_at presorted; id tie-break sorted in-page) |
| History first page | `check_runs_latest_terminal_idx` | Bitmap Index Scan on the partial terminal index + top-N sort for the id tie-break |
| Latest terminal read | `check_runs_latest_terminal_idx` | Index Scan, no sort (single-row limit) |
| History anchored page | `check_runs_latest_terminal_idx` | Bitmap Index Scan with the keyset filter applied as a heap filter + top-N tie-break sort |
| Claim candidate | `check_runs_latest_terminal_idx` (via the lateral) | Index Only Scan per monitor inside the anti-join; monitors Seq Scan (25 rows) |
| Retention statement | recorded, not asserted | Delete plan over the window-function rank (no ANALYZE — nothing deleted during capture) |

The per-page id tie-break sort visible in the history plans is the acknowledged trade-off from the history design: the terminal index serves the monitor and time predicate; a composite index including `check_id` remains a future option if measurement demands it (the populated-data EXPLAIN minor closes with this evidence recorded).

## Recorded plans

### inventory
Limit  (cost=0.58..2.78 rows=21 width=76) (actual time=0.100..0.103 rows=21.00 loops=1)
  Buffers: shared hit=11
  ->  Incremental Sort  (cost=0.58..101.41 rows=960 width=76) (actual time=0.099..0.100 rows=21.00 loops=1)
        Sort Key: created_at DESC, ((id)::text) DESC
        Presorted Key: created_at
        Full-sort Groups: 1  Sort Method: quicksort  Average Memory: 27kB  Peak Memory: 27kB
        Buffers: shared hit=11
        ->  Index Scan using monitors_inventory_order_idx on monitors  (cost=0.15..74.55 rows=960 width=76) (actual time=0.020..0.025 rows=22.00 loops=1)
              Index Searches: 1
              Buffers: shared hit=2
Planning:
  Buffers: shared hit=120 read=3
Planning Time: 1.033 ms
Execution Time: 0.150 ms
### history-first
Limit  (cost=16.07..16.09 rows=5 width=84) (actual time=0.105..0.107 rows=21.00 loops=1)
  Buffers: shared hit=25
  ->  Sort  (cost=16.07..16.09 rows=5 width=84) (actual time=0.104..0.105 rows=21.00 loops=1)
        Sort Key: completed_at DESC, ((id)::text) DESC
        Sort Method: top-N heapsort  Memory: 27kB
        Buffers: shared hit=25
        ->  Bitmap Heap Scan on check_runs  (cost=4.31..16.02 rows=5 width=84) (actual time=0.042..0.060 rows=50.00 loops=1)
              Recheck Cond: ((monitor_id = '616ed53c-f8ee-466c-820e-75560be285ab'::uuid) AND (completed_at IS NOT NULL))
              Heap Blocks: exact=16
              Buffers: shared hit=19
              ->  Bitmap Index Scan on check_runs_latest_terminal_idx  (cost=0.00..4.31 rows=5 width=0) (actual time=0.025..0.026 rows=50.00 loops=1)
                    Index Cond: (monitor_id = '616ed53c-f8ee-466c-820e-75560be285ab'::uuid)
                    Index Searches: 1
                    Buffers: shared hit=3
Planning:
  Buffers: shared hit=135
Planning Time: 0.852 ms
Execution Time: 0.164 ms
### latest-terminal
Limit  (cost=0.28..5.10 rows=1 width=84) (actual time=0.033..0.034 rows=1.00 loops=1)
  Buffers: shared hit=3
  ->  Index Scan using check_runs_latest_terminal_idx on check_runs  (cost=0.28..24.39 rows=5 width=84) (actual time=0.032..0.033 rows=1.00 loops=1)
        Index Cond: (monitor_id = '616ed53c-f8ee-466c-820e-75560be285ab'::uuid)
        Index Searches: 1
        Buffers: shared hit=3
Planning:
  Buffers: shared hit=120
Planning Time: 1.179 ms
Execution Time: 0.081 ms
### history-anchored
Limit  (cost=10.24..10.25 rows=2 width=84) (actual time=0.231..0.233 rows=21.00 loops=1)
  Buffers: shared hit=25
  ->  Sort  (cost=10.24..10.25 rows=2 width=84) (actual time=0.230..0.231 rows=21.00 loops=1)
        Sort Key: completed_at DESC, ((id)::text) DESC
        Sort Method: top-N heapsort  Memory: 27kB
        Buffers: shared hit=25
        ->  Bitmap Heap Scan on check_runs  (cost=4.30..10.23 rows=2 width=84) (actual time=0.131..0.153 rows=50.00 loops=1)
              Recheck Cond: ((monitor_id = '616ed53c-f8ee-466c-820e-75560be285ab'::uuid) AND (completed_at IS NOT NULL))
              Filter: (ROW(completed_at, id) < ROW((now() - '01:35:00'::interval), '7d9b2eb0-52bb-4dd7-8934-c5ce30d5c675'::uuid))
              Heap Blocks: exact=16
              Buffers: shared hit=19
              ->  Bitmap Index Scan on check_runs_latest_terminal_idx  (cost=0.00..4.30 rows=2 width=0) (actual time=0.114..0.114 rows=50.00 loops=1)
                    Index Cond: ((monitor_id = '616ed53c-f8ee-466c-820e-75560be285ab'::uuid) AND (completed_at <= (now() - '01:35:00'::interval)))
                    Index Searches: 1
                    Buffers: shared hit=3
Planning:
  Buffers: shared hit=146
Planning Time: 1.941 ms
Execution Time: 0.423 ms
### claim-candidate
Limit  (cost=2517.05..2517.07 rows=1 width=132) (actual time=0.340..0.341 rows=1.00 loops=1)
  Buffers: shared hit=99
  ->  LockRows  (cost=2517.05..2519.07 rows=161 width=132) (actual time=0.339..0.340 rows=1.00 loops=1)
        Buffers: shared hit=99
        ->  Sort  (cost=2517.05..2517.46 rows=161 width=132) (actual time=0.322..0.323 rows=1.00 loops=1)
              Sort Key: (COALESCE(terminal.latest_completed_at, monitor.created_at)), monitor.id
              Sort Method: quicksort  Memory: 28kB
              Buffers: shared hit=98
              ->  Nested Loop Anti Join  (cost=0.28..2516.25 rows=161 width=132) (actual time=0.165..0.256 rows=25.00 loops=1)
                    Join Filter: (pending.monitor_id = monitor.id)
                    Buffers: shared hit=92
                    ->  Nested Loop Left Join  (cost=0.28..2477.20 rows=162 width=102) (actual time=0.074..0.158 rows=25.00 loops=1)
                          Filter: ((terminal.latest_completed_at IS NULL) OR (terminal.latest_completed_at <= (now() - '00:01:00'::interval)))
                          Buffers: shared hit=76
                          ->  Seq Scan on monitors monitor  (cost=0.00..19.60 rows=480 width=62) (actual time=0.009..0.012 rows=25.00 loops=1)
                                Filter: (NOT paused)
                                Buffers: shared hit=1
                          ->  Subquery Scan on terminal  (cost=0.28..5.10 rows=1 width=40) (actual time=0.005..0.005 rows=1.00 loops=25)
                                Buffers: shared hit=75
                                ->  Limit  (cost=0.28..5.09 rows=1 width=8) (actual time=0.005..0.005 rows=1.00 loops=25)
                                      Buffers: shared hit=75
                                      ->  Index Only Scan using check_runs_latest_terminal_idx on check_runs run  (cost=0.28..24.36 rows=5 width=8) (actual time=0.004..0.004 rows=1.00 loops=25)
                                            Index Cond: (monitor_id = monitor.id)
                                            Heap Fetches: 25
                                            Index Searches: 25
                                            Buffers: shared hit=75
                    ->  Materialize  (cost=0.00..26.10 rows=5 width=22) (actual time=0.004..0.004 rows=0.00 loops=25)
                          Storage: Memory  Maximum Storage: 17kB
                          Buffers: shared hit=16
                          ->  Seq Scan on check_runs pending  (cost=0.00..26.08 rows=5 width=22) (actual time=0.080..0.080 rows=0.00 loops=1)
                                Filter: (completed_at IS NULL)
                                Rows Removed by Filter: 1250
                                Buffers: shared hit=16
Planning:
  Buffers: shared hit=178
Planning Time: 1.545 ms
Execution Time: 0.475 ms
### retention (EXPLAIN only, no ANALYZE)
Delete on check_runs  (cost=115.94..148.34 rows=0 width=0)
  ->  Hash Join  (cost=115.94..148.34 rows=332 width=46)
        Hash Cond: (check_runs.id = ranked.id)
        ->  Seq Scan on check_runs  (cost=0.00..26.08 rows=1003 width=22)
              Filter: (completed_at IS NOT NULL)
        ->  Hash  (cost=113.76..113.76 rows=174 width=56)
              ->  HashAggregate  (cost=112.02..113.76 rows=174 width=56)
                    Group Key: ranked.id
                    ->  Subquery Scan on ranked  (cost=76.10..111.19 rows=334 width=56)
                          Filter: (ranked.rank > 50)
                          ->  WindowAgg  (cost=76.10..98.65 rows=1003 width=48)
                                Window: w1 AS (PARTITION BY check_runs_1.monitor_id ORDER BY check_runs_1.completed_at, check_runs_1.id ROWS UNBOUNDED PRECEDING)
                                ->  Sort  (cost=76.08..78.59 rows=1003 width=40)
                                      Sort Key: check_runs_1.monitor_id, check_runs_1.completed_at DESC, check_runs_1.id DESC
                                      ->  Seq Scan on check_runs check_runs_1  (cost=0.00..26.08 rows=1003 width=40)
                                            Filter: (completed_at IS NOT NULL)
