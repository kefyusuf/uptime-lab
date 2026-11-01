# Local Check-Run History Design

**Date:** 2026-11-01
**Status:** Written design for review; implementation not authorized
**Baseline:** `main@b4f0154f`; scope proposal at `c30439a` in [PR #71](https://github.com/kefyusuf/uptime-lab/pull/71)
**Stage decision:** The user's continuation after the scope proposal authorizes this written design. It does not approve an implementation plan, merge or deployment.

## 1. Intent and boundaries

Expose the retained terminal CheckRun window of one Monitor as a bounded, keyset-paginated read and a detail-page history section, preserving the Go/Rust ownership model, the closed evidence views and the bounded-response discipline. This increment introduces no retention policy, target-URL edit, aggregation or uptime math, incidents, notifications, ownership, audit trail or remote exposure. Rust and the internal Checker contract remain unchanged; no migration is added, so the readiness migration set stays at four.

## 2. Read semantics

History contains terminal CheckRuns only — pending rows stay invisible, consistent with `latest-result`. Rows appear in `completed_at DESC, check_id DESC` order and carry the existing result vocabulary (`http_response`, classified failures, `worker_timeout`) with their optional `httpStatus`/`durationMs` fields; no verdict, availability or uptime derivation is performed. A known Monitor without terminal history is a successful empty page with a null cursor; an unknown Monitor is 404. Pages are positions, not snapshots: concurrent completions may shift row contents between reads, and a cursor anchored on evidence that deletion later removed yields an empty continuation rather than an error. The read never samples a clock and never mutates state.

## 3. Domain, application and persistence

A dedicated inward port `MonitorHistoryRepository` exposes `Page(ctx, monitorID, anchor, limit)`, where `anchor` is a validated decoded position (completed instant + CheckRun UUID) or nil for the first page, and `limit` is the validated page size. The application use case `GetMonitorHistory` receives the validated MonitorID and raw query values, decodes the cursor before any storage call, delegates once, and reuses the existing application not-found/persistence translation and context cancellation mapping. No clock is sampled and no Checker path is invoked.

The cursor mirrors the inventory encoding with its own version: raw `"2|" + completed_at microseconds (2006-01-02T15:04:05.000000Z) + "|" + checkId` — 66 bytes, 88 base64url characters, strict round-trip decode, canonical re-encode equality and structural position checks identical to `inventory_cursor.go`. The separate version prefix rejects cross-resource cursors as malformed rather than silently accepting a foreign position.

The adapter runs one explicit Read Committed transaction containing an existence probe for the Monitor and the page query, so a concurrent deletion yields either a consistent page or a 404 — never rows for a vanished Monitor. The page query uses the row-value keyset predicate `WHERE monitor_id = $1 AND completed_at IS NOT NULL AND (completed_at, id) < ($2, $3) ORDER BY completed_at DESC, id DESC LIMIT $4` for anchored reads and the same query without the comparison for the first page; the identifier binds as binary `pgtype.UUID`/`pgtype.Timestamptz`-compatible values, matching the deletion adapter's binding discipline. The existing `check_runs_latest_terminal_idx (monitor_id, completed_at DESC) WHERE completed_at IS NOT NULL` serves the predicate; no new index or migration is added and no query performance claim is made.

## 4. Pagination position and consistency

The cursor is a strict ordering position with the same contract as the inventory cursor: no ownership, no authorization boundary, no cross-page snapshot guarantee. Concurrent completions between reads may shift or duplicate rows across pages; the design documents this rather than preventing it. Because the anchor is compared with row values under Read Committed inside one transaction, a Monitor deleted between the existence probe and the page query resolves through the shared snapshot as either a page or a 404 — both atomic.

## 5. Public HTTP contract

Add `getMonitorHistory` on the new sub-resource `/monitors/{monitorId}/history`: success is 200 JSON `{"items":[...],"nextCursor":string|null}`, with page items referencing the existing `LatestCheckResult` result-kind vocabulary through a closed `MonitorHistoryEntry` schema (`checkId` uuid, `resultKind`, optional `httpStatus`/`durationMs`, `completedAt`) and a closed `MonitorHistoryPage` schema (`items` ≤ 50, `nextCursor` null-or-88-char) carrying the same 245760-byte compact JSON budget as the inventory via the existing extension convention. Error statuses are 400 (malformed UUID, limit, cursor or raw path), 404 (unknown Monitor), 405 (`Allow: GET`) and 500 (sanitized persistence failure); every matched response uses `Cache-Control: no-store`.

Query grammar mirrors the inventory exactly: optional canonical `limit` (1..50, default 20) and optional 88-character `cursor`, each at most once in either order, no other keys, empty values, percent escapes, semicolons, whitespace or query bodies; the raw query is at most 128 ASCII bytes. Raw-path canonicality, encoded-segment and alias rejection without redirect follow the sub-resource precedents. `contracts/openapi/public.yaml` gains the operation, schemas and fixtures (empty, single-row, full page, continuation); no existing schema changes and Web types regenerate through the existing generator. Contract tests cover schema closure and the transport grammar separately.

## 6. Gateway boundary

The gateway maps `GET /api/monitors/{monitorId}/history` with the same strict query grammar the inventory enforces today, reusing `inventory-query.ts`'s validator for the shared limit/cursor shape rather than widening accepted input; the sub-resource pattern joins `latest-result`/`availability` in the route mapping, the raw path stays canonical (aliases, escapes and fragments rejected before forwarding), no new mutation class is introduced and GET requires no Origin. Existing header/response caps, deadlines and sanitized error behavior are unchanged; internal routes stay closed.

## 7. Browser direction

`entities/monitor` gains the history decoder and a `getHistory(id, cursor, signal)` client method returning `ReadResult<MonitorHistoryPage>`; a `features/monitor-history` hook and section compose only while the Monitor is known, styled after the evidence cards. One read fires on Monitor success plus explicit Refresh and Next-page traversal mirroring the inventory start-page behaviour; loading/empty/error states are announced accessibly with the same empty-page copy conventions. The section stays independent of `useMonitorReads`, the scheduling control and the deletion control — no fan-out, no polling, no optimistic insertion — and a deleted Monitor's section disappears with the detail view through the existing not-found tolerance. Stale responses after navigation or refresh are fenced the way the inventory hook fences them.

## 8. Required acceptance and evidence limits

- Contract RED/GREEN for the closed operation, cursor/page schemas, budget, unchanged existing schemas and generated parity.
- Real PostgreSQL RED/GREEN for terminal-only paging at the exact 20/50 boundaries, ordering ties on equal completion instants, continuation keys naming their last included row, unknown-Monitor 404 vs empty-page distinction, cursor grammar rejection (version, length, canonical re-encode, foreign resource), concurrent-completion position shift behavior, and deletion racing a read (page-or-404 atomicity). Coordinated transactions and bounded synchronization; no arbitrary sleeps as race proof.
- HTTP/gateway RED/GREEN for method/no-store/Allow behavior, query grammar, aliases without redirect and unchanged existing operations.
- Web tests for loading/empty/error states, one in-flight read, stale-response fencing and independent evidence views.
- Actual composed Chromium on 4173/4817: register, claim through the real Checker, page through the real history across an API restart, verify durable row linkage; transaction evidence proves ordering — browser timing alone does not.
- Required repository, contract/generated, Go/race/real-PostgreSQL, Rust regression/audit, Web, Docker/browser and aggregate CI at the final implementation head; exact-main evidence after an authorized merge. The populated-data EXPLAIN evidence remains a separate capacity-evidence minor.

## 9. Review and next stage

Review this written design, especially the terminal-only rule, the versioned-cursor separation from the inventory cursor, the existence-probe-plus-page transaction, the budget mirroring and the detail-only section scope. Only after design approval may the implementation plan be produced; plan review precedes Native TDD execution. No deployment, merge or external-service change follows automatically from these approvals.
