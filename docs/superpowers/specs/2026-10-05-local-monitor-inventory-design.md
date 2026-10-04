# Local Monitor Inventory Slice Design

**Status:** Written design candidate; awaiting review before implementation planning
**Date:** 2026-10-05
**Base:** `main@449fdd130c6d9bc84e0cdec6e571bd98c001a783`
**Approved scope:** [Local Monitor Inventory](2026-10-04-local-monitor-inventory-scope-reassessment.md), merged through [#53](https://github.com/kefyusuf/uptime-lab/pull/53)

## 1. Intent and current truth

The user approved merging #53 and preparing this written design, with careful continuation toward the roadmap and TDD -> branch/commit -> PR delivery. The intended user remains a contributor using the opt-in local browser endpoint. Success is discovering durable Monitors without retaining their UUIDs, then opening the existing detail journey. It does not select a beta audience or authorize remote access.

Local registration, UUID reopen, manual detail refresh and independent Go availability/raw-result reads already work. Four public operations and two internal Checker operations exist. This design proposes a fifth public operation; no source, schema, contract or runtime change is made by this document.

The merged scope's [exact-main CI](https://github.com/kefyusuf/uptime-lab/actions/runs/37235062042) passed documentation/policy checks at the base above. Runtime jobs were skipped because that change was documentation only. Real runtime evidence remains the separately recorded #51/#52 runs; no inventory execution evidence exists yet.

## 2. Selected approach and ownership

| Approach | Benefit | Trade-off |
|---|---|---|
| Keyset collection read through Go and the existing local gateway — selected proposal | Durable discovery, deterministic ordering, bounded requests and rendering | Requires cursor/byte policy and one new immutable index migration |
| Offset/page-number API | Simple page numbers and arbitrary jumps | Deep offsets and insertion shifts; no need for arbitrary jumps in this journey |
| Browser-local recently created IDs | Avoids a contract extension | Cannot discover durable targets from other sessions; fails the approved intent |

Go owns input validation, ordering and durable retrieval. A narrow new inventory read port avoids expanding Create/ByID consumers and their test doubles. PostgreSQL implements this port; the Monitoring module wires it into a ListMonitors application use case and the public HTTP adapter. The Go HTTP adapter owns JSON representation and encoded-body budgeting. The use case and port bound candidate rows and raw target bytes without importing HTTP or serialization code into domain/application layers.

React consumes the closed public page through the Monitor entity adapter. The page composes an inventory feature alongside the existing creation and UUID reopen features. Follow `app -> pages -> widgets -> features -> entities -> shared`; introduce no empty layers. The Node gateway allows the single new collection read and precisely validated query forms. Rust, work claims/results, availability policy and CheckRun persistence remain unchanged.

## 3. Public operation and query grammar

Add `GET /monitors` with operationId `listMonitors`. The browser calls `/api/monitors`. Default limit is 20; valid limits are canonical decimal integers 1 through 50. There is no total count, filter, sort, offset or page-number parameter.

Only optional `limit` and `cursor` parameters are accepted, at most once each, in either order. Examples are no query, `?limit=20`, `?cursor=<token>`, and `?limit=20&cursor=<token>`. Missing values, duplicate/unknown keys, empty query markers, trailing separators, leading zeroes, signs, spaces, semicolons, plus signs and percent escapes return 400. Limit 0 or 51 is invalid. The entire raw collection query is at most 128 ASCII bytes. Go and gateway enforce the same lexical bounds independently; Go remains authoritative for cursor meaning. No generic query decoder may normalize invalid raw input into a permitted form.

Only the new collection GET accepts queries. Gateway collection POST and existing resource reads still reject them; preserve existing Go POST behavior rather than silently changing its query semantics. Preserve raw-path rejection before URL normalization: encoded/doubled separators, dot segments, backslashes, fragments, absolute-form requests and trailing slash are invalid. The Go collection GET also checks its original request target, not only a decoded URL.Path.

| Response | Meaning |
|---|---|
| 200 application/json | A closed inventory page, including a valid empty page |
| 400 application/problem+json | Invalid collection query or cursor; fixed sanitized detail |
| 500 application/problem+json | Persistence/mapping failure, or first eligible Monitor cannot fit the inventory page budget |
| 405 | Unsupported method on the exact collection, `Allow: GET, POST` |

Collection POST keeps its existing creation responses. HEAD is unsupported; no implicit GET execution. GET rejects request bodies (nonzero Content-Length or any Transfer-Encoding) before retrieval. All collection GET responses and collection 405 responses use `Cache-Control: no-store`. Gateway errors retain existing no-store behavior and transport errors (502/504). Internal/health/unknown paths remain inaccessible through the gateway.

The closed page envelope requires exactly `items` and `nextCursor`. Items reuse the existing Monitor shape (`id`, exact `targetUrl`, UTC `createdAt`); no status/result fields are added. `nextCursor` is either a token or null, never absent. At most the requested limit is returned, and byte limits may return fewer. An exhausted or empty result is `200 {"items":[],"nextCursor":null}`. A successful empty page never has a continuation token.

## 4. Ordering, precision and cursor

Order by `(created_at DESC, id DESC)`, using PostgreSQL's UUID comparison as the unique tie-breaker. Subsequent reads use the strict predicate `(created_at, id) < ($cursorTime, $cursorID)`. This is position-based traversal, not lookup of a cursor row. A missing/deleted/nonexistent anchor does not return 404; a valid anchor beyond the tail returns an empty page. Cursors may be syntactically valid positions never issued by the server. They grant no authority and are not signed or encrypted.

Cursor bytes are the following fixed ASCII form, encoded with unpadded base64url:

```text
1|YYYY-MM-DDTHH:MM:SS.ffffffZ|xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
```

The decoded payload is exactly 66 bytes and the token exactly 88 characters from `[A-Za-z0-9_-]`. Version must be `1`; timestamp must be a real finite UTC Gregorian instant with year 0001 through 9999, exactly six fractional digits and uppercase T/Z. UUID must be canonical lowercase hyphenated, nonzero. Reject unsupported versions, alternate spelling, impossible dates, padded/standard base64, excess bytes and noncanonical base64 pad bits. Validate by exact parse/format and base64 decode/re-encode equality. Zero Go time is invalid as an existing Monitor invariant. The fixed positional format has no JSON duplicate-key ambiguity.

Generate cursors from the persisted row returned by PostgreSQL, never the creation response's pre-persistence clock value. PostgreSQL timestamps resolve to microseconds; JavaScript Date resolves less precisely. Preserve cursor strings and creation-time text in the browser rather than converting them back into a cursor. PostgreSQL date/time precision is documented in [Date/Time Types](https://www.postgresql.org/docs/18/datatype-datetime.html). Valid persisted values outside the public date representation or corrupt domain fields fail closed as sanitized 500, without truncation or coercion.

Each page is one statement snapshot. No traversal-wide snapshot or registration watermark is promised. Normal newer inserts appear after a first-page refresh; inserts whose sort key is below the current anchor may appear on later pages, including backdated clock values or later commits with earlier creation times. Strict ordering prevents duplicate existing immutable rows during forward traversal. Refresh resets to the first page. Changing limit with a cursor is valid and does not change the anchor. No count or promise of an exhaustive historical snapshot is made.

## 5. Row and response-byte bounds

Keep the existing gateway response cap of 262144 bytes. The inventory HTTP adapter buffers and writes one compact UTF-8 JSON body of at most **245760 bytes (240 KiB)**, leaving 16 KiB of body headroom; headers are not part of this body budget. Use the exact production serializer and its HTML/Unicode escaping policy for measurements. Do not approximate with string character count or disable escaping as an unreviewed workaround.

Read at most `limit + 1` ordered candidates (maximum 51). Reserve a conservative 128 bytes for page framing and the possible 88-character next cursor; this reserve excludes item contents and inter-item commas. Charge each item's exact serialized bytes plus one comma byte for every item after the first. Add complete serialized items in order while that sum plus the reserve stays within the budget. Never split a target, skip a candidate, or expose a cursor from a lookahead/omitted row. When count or bytes stop the page and another candidate exists, `nextCursor` is generated from the last included Monitor. If all candidates fit and no lookahead exists, it is null. Final serialized size is checked before headers or any body bytes are written. A successful page with continuation always includes at least one item and advances its key.

An oversized candidate after a fitting prefix ends the successful page before that candidate; the next request starts there. If the first eligible item cannot fit alone with framing, return a fixed 500 Problem detail: `A registered monitor exceeds the inventory response limit.` No empty success, target text, invented continuation or silently skipped Monitor is returned. The browser shows that error and preserves manual UUID reopen. Older records cannot be reached through that blocked position until a later deliberately designed data remedy; this is an explicit limitation, not a claim of complete inventory traversal for every existing database. Registration validation and stored targets are not changed in this slice. Existing individual reads may independently exceed the unchanged gateway cap for such data.

The database projection returns `octet_length(target_url)` and only returns target text when raw length is at most 245760 bytes; larger values produce a typed oversized marker with ID/time, not a truncated value or domain Monitor with a fabricated target. Inspect row metadata before constructing normal domain values. For count lookahead, only key/existence matters; do not serialize or reject its oversized target prematurely. Bounded raw text can still expand under JSON escaping; the HTTP budget detects that case.

The inventory port returns bounded candidate records, including the explicit oversized marker. The adapter closes rows on cancellation/error, uses query parameters, and does not read CheckRuns. Raw target text transferred to Go is at most `51 * 245760` bytes (about 12 MiB); serialize one candidate at a time rather than an unbounded collection, with at most sixfold JSON string escaping expansion for that candidate plus fixed fields. This bounds Go-side transfer/buffering, not total database execution memory or arbitrary SQL access. It does not certify production throughput or global concurrency admission; those remain release gates.

## 6. Persistence and migration

Add `00003_add_monitor_inventory_order_index.sql` with a non-unique B-tree index on `monitoring.monitors (created_at DESC, id DESC)`. No table/column changes, data rewrite, ownership field or historical migration edits. Its down section removes only the new index, using the repository's normal migration conventions. Production rollback policy remains separate; no automatic downgrade is introduced.

First page and cursor page are separate parameterized SELECT shapes; avoid an optional-parameter OR predicate that hides the index range. Both use the same ORDER BY and LIMIT, with safe projection described above. PostgreSQL's [LIMIT guidance](https://www.postgresql.org/docs/18/queries-limit.html) requires deterministic ordering, and [B-tree ordering guidance](https://www.postgresql.org/docs/18/indexes-ordering.html) supports ordered bounded retrieval. These support this selected design; they do not prove the eventual query plan or supported Monitor capacity.

Test migration/index presence and read behavior against real PostgreSQL. Do not assert every tiny fixture must select an index; record a representative query-plan inspection alongside functional tests. Readiness requires the exact repository migration set. Use the existing local sequence: start API unready, explicitly execute its migration command, then verify readiness before browser traffic and Checker startup. Earlier API images can become unready against the new migration set; no rolling-upgrade compatibility is claimed. Canonical Docker smoke and schema expectations must be updated in the implementation, while historical migrations remain byte-identical.

## 7. Browser flow and cancellation

Keep `/` as the existing creation/UUID reopen page and add an inventory section there. No new client route or cursor in browser history/localStorage is needed. A newly mounted start page loads `limit=20`; one active inventory request and one displayed page of at most 20 items are allowed. Server-side maximum 50 supports direct API callers, not a new UI selector.

Show a semantic table/list with exact target text (escaped React text, wrapping long values), Monitor ID, UTC creation time and a local detail link. Do not make target URLs outbound links or fetch them. No availability calls, badges, total count, polling or CheckRun queries are triggered by list rows. Existing registration and direct UUID reopen stay usable while inventory is loading or failing.

- `Next page` appears only with a valid next cursor. It replaces the page rather than appending to an unlimited list; no backward cursor stack is kept.
- `Refresh list` resets the anchor and loads the first page. There is no previous-page control; refresh is the return path.
- Each new inventory request aborts the old request and increments a generation token. Unmount aborts too. Ignore responses/errors from obsolete generations even when abort races with completion.
- During refresh/navigation, clear old rows and mark loading; do not present stale rows as the requested page. Disable Next while pending. A failed read is an error with explicit Retry for the attempted anchor and Refresh from first page, never an empty list.
- Empty first page says `No monitors registered yet.` Empty cursor page says `No more monitors on this page. Refresh the list to start again.` There is no continuation in either case.
- Preserve malformed-response, bounded transport timeout and sanitized error behavior in the entity client. Validate closed envelope, row count, duplicates, UUIDs/timestamps and cursor syntax; do not reconstruct server ordering or classify availability in the UI. The cursor remains an opaque client value.
- Successful creation continues to navigate to the existing detail route. Returning to `/` mounts a fresh first-page read; it does not repeat POST or assume a newly created record belongs on an already displayed page. Uncertain creation keeps existing duplicate-risk feedback and performs no automatic retry.

Keep keyboard focus and announced loading/errors usable: headings, accessible table headers, button labels, aria-busy/live updates, and genuine local links with existing modified-click behavior. Retain skip-link, registration focus/error handling and detail manual-refresh semantics. Existing create/detail tests and real browser flow must continue to pass.

## 8. Gateway and trust boundary

Add GET to the exact `/api/monitors` method allowlist; match/validate the raw collection query before stripping `/api`. Forward only validated path/query text to the fixed API origin. No client upstream URL, reflected redirect, forwarded credentials, widened generic proxy or CORS policy is introduced. A cursor contains time/ID only; no target URL, secret or authorization scope.

Preserve Host/Origin/Fetch-Metadata checks, matching-origin requirement for POST, forbidden body rules for GET, 16 KiB headers, 64 KiB request bodies, 256 KiB upstream responses, 5/10/10-second header/inbound/upstream deadlines, browser timeout, upstream abort on disconnect, redirect rejection and response-header filtering. Query strings remain rejected on existing resource reads and creation. Collection GET must not make internal endpoints or arbitrary API query paths reachable.

Inventory reveals every stored Monitor and potentially sensitive target query text. The system has no account ownership or authentication. Retain canonical unpublished ports, opt-in loopback-only Web, the compiled read-only runtime and existing CSP. Local-machine processes/users can still call the service; this boundary does not isolate them. Log neither target text nor cursor/raw query; use existing sanitized route/status failures. Remote/shared-account access requires a separately reviewed authorization/ownership model before ordering and LIMIT, plus the roadmap's release safeguards.

## 9. Verification contract for the later TDD plan

| Boundary | Required meaningful evidence |
|---|---|
| Public contract | Closed pages, empty/tail pages, limit/cursor grammar, all error/media/cache/method cases; unchanged four-operation fixtures |
| Go application/ports | Validation precedes DB access, bounded candidates, cancellation, sanitized errors, no mutation/scheduling; explicit oversized marker handling |
| PostgreSQL/migrations | Equal creation timestamps and UUID order; strict boundaries; nonexisting anchors; microsecond roundtrip; inserted rows between pages; oversized projection; 51-row cap; index/migration/readiness; no CheckRun mutation |
| HTTP byte budgeting | Exact serializer at boundary/one byte beyond; HTML/Unicode expansion; fitting prefixes; first oversized item; oversized count-lookahead; no empty continuation, skipped record or partial write |
| Gateway | Both query orders and optional defaults; encoded/duplicate/unknown/oversized queries; bad raw paths; bodies/methods/origin/internal-route rejection; unchanged deadline/byte bounds and POST behavior |
| Entity/UI | Strict decoder; loading/empty/failure distinctions; bounded replacement; cancellation and stale responses; Retry/Refresh/Next behavior; exact target text; no per-row health fan-out or automatic POST retry; keyboard interaction |
| Composed acceptance | Real PostgreSQL durable inventory after browser reload with no retained IDs; register enough controlled records for more than one UI page, traverse/select by captured ID; retain staged raw204 versus unknown/no_result and independent CheckIDs; actual loopback4173/4817 and canonical Docker smoke |

The implementation plan must pin focused RED -> GREEN tests before each behavior change, immutable migration checks, affected Go/contracts/Web/architecture checks, real PostgreSQL acceptance, build/audit, and exact-head Linux composed CI. Fixture orchestration alone is not database/browser/runtime proof. Independent review and required CI precede a merge request. This host lacks Docker; do not install global tooling or claim local composed evidence.

## 10. Scope and next gate

No pause/resume, edit/delete, history, retention, status aggregation, incidents, notifications, cadence changes, accounts, remote ingress or multi-checker work is included. R2 remains partially addressed by the future inventory implementation; R0 release assumptions remain unresolved.

This written candidate requires user review. Approval permits a detailed TDD implementation plan and execution-method selection; it does not approve implementation in this documentation PR. The candidate adds no dependencies, contract/generated code, migration, product code or runtime configuration. Subsequent work must preserve the chosen local scope and existing availability/execution boundaries.
