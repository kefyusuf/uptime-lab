# Local Monitoring Web

The React/TypeScript client in `apps/web` owns presentation and browser interaction. Go owns registration rules, durable state and availability policy; Rust owns bounded execution. Web consumes exactly the four operations in `contracts/openapi/public.yaml` through its same-origin `/api` gateway.

`app -> pages -> features -> entities -> shared` is enforced by `scripts/ci/check-web-architecture.mjs`. Features/entities expose entrypoints, generated wire types stay in the monitor adapter, and browser imports cannot reach Node or `server`. The separate Node runtime uses only standard-library modules and serves Vite manifest assets with CSP and no inline scripts.

The registration page preserves target text, prevents duplicate pending submissions and navigates only after a valid `201`. Rejected input can be corrected. An uncertain creation explicitly warns that the Monitor may have been created and that retry can duplicate it; it never retries automatically. Reopening uses a Monitor ID or saved `/monitors/{id}` URL, with native back/forward/reload support. No browser inventory is persisted.

The detail page verifies Monitor first, then reads availability and raw execution independently. It preserves `204` versus `unknown/no_result`, separate CheckIDs, absent versus zero duration, textual reasons and UTC timestamps. `Snapshot; refresh to reassess` explains that no browser clock/polling changes the verdict. Refresh clears old cards; current Monitor errors hide them. Cancellation/generation checks prevent older responses from replacing current state.

Web proxies only POST collection and GET Monitor/latest-result/availability to fixed `http://api:8080`. It checks exact Host/Origin, JSON creation, raw paths and methods; bounds headers (16KiB), request bodies (64KiB), responses (256KiB), inbound headers (5s), inbound requests (10s), upstream work (10s) and browser waiting (12s). Internal routes, redirects, environment proxies and forwarded credentials are blocked. API responses use no-store. Asset health is independent of Go readiness. Logs contain bounded method/category/status/duration fields, never targets or bodies.

The container runs as `10001:10001` with read-only storage and compiled assets/server only. Canonical Compose publishes no port. The explicit local override publishes Web on loopback only; it is not a remote-release contract. See [local development](../devops/local-development.md) and [verification](../testing/local-monitoring-web.md).
