# Local Monitoring Web

The React/TypeScript client in `apps/web` owns presentation and browser interaction. Go owns registration rules, durable state and availability policy; Rust owns bounded execution. Web consumes exactly the five operations in `contracts/openapi/public.yaml` through its same-origin `/api` gateway.

`app -> pages -> features -> entities -> shared` is enforced by `scripts/ci/check-web-architecture.mjs`. Features/entities expose entrypoints, generated wire types stay in the monitor adapter, and browser imports cannot reach Node or `server`. The separate Node runtime uses only standard-library modules and serves Vite manifest assets with CSP and no inline scripts.

The registration page preserves target text, prevents duplicate pending submissions and navigates only after a valid `201`. Rejected input can be corrected. An uncertain creation explicitly warns that the Monitor may have been created and that retry can duplicate it; it never retries automatically. Reopening uses a Monitor ID or saved `/monitors/{id}` URL, with native back/forward/reload support. No browser inventory is persisted.

The detail page verifies Monitor first, then reads availability and raw execution independently. It preserves `204` versus `unknown/no_result`, separate CheckIDs, absent versus zero duration, textual reasons and UTC timestamps. `Snapshot; refresh to reassess` explains that no browser clock/polling changes the verdict. Refresh clears old cards; current Monitor errors hide them. Cancellation/generation checks prevent older responses from replacing current state.

Web proxies POST collection and GET inventory/Monitor/latest-result/availability to fixed `http://api:8080`. Only collection GET accepts canonical limit/cursor once each and at most128 raw ASCII query bytes; POST/resource queries remain rejected. Exact Host/Origin and GET-body checks,16KiB headers,64KiB requests,256KiB responses and5/10/10s gateway deadlines remain in force; browser waiting stays12s. Internal routes, redirects, environment proxies and forwarded credentials are blocked. Responses use no-store. Logs exclude targets, cursors and raw queries.

The container runs as `10001:10001` with read-only storage and compiled assets/server only. Canonical Compose publishes no port. The explicit local override publishes Web on loopback only; it is not a remote-release contract. See [local development](../devops/local-development.md) and [verification](../testing/local-monitoring-web.md).

## Inventory on the start page

A newly mounted start page loads one20-row inventory page. Next replaces it; Refresh returns to the first page; Retry uses the attempted cursor. Pending reads clear old rows and abort/generation guards suppress stale completions. Exact targets wrap as escaped text without outbound links. ID, UTC creation time and native local detail links remain usable by keyboard and modified clicks. List rows make no health/detail requests.

First-page and cursor-page empty messages differ; a failed read remains an error. Only the closed approved oversized500 receives fixed client copy; arbitrary server details are hidden. That first-item error blocks older traversal without truncation or skipping. Creation and UUID reopen remain usable. No cursor history/localStorage, backward stack, polling, automatic POST retry or ownership boundary is introduced.
