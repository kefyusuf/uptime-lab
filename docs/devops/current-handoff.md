# Current Project Handoff

**Recorded:** 2026-10-04
**Verified landed base:** `f17d2de10622f0ff9cf33489bf633005e7863672` (local Web implementation #51)
**Active follow-up branch:** `test/web-inbound-deadlines`
**Reviewed implementation commit:** `02f95b3bcae1623808389a68b51ba4ea6585e7a8` (Tasks1–7 complete)
**Implementation PR:** [#51](https://github.com/kefyusuf/uptime-lab/pull/51)

## Completed state

- Current Availability implementation #48 is merged; Go serves four public operations.
- Industry roadmap #45 is merged at `873c1a3`.
- Local Web design #49 is approved and merged at the base above.
- Exact-base main CI run [37023576947](https://github.com/kefyusuf/uptime-lab/actions/runs/37023576947) passed.
- The approved Native workflow now implements React registration/reopen/refresh, contract decoding, bounded same-origin gateway, non-root compiled runtime, loopback Compose override and staged real browser acceptance. Canonical Compose publishes no ports.
- Task1–6 commits: `1dace19`, `4ab7c3e`, `0c080b7`, `adb06c5`, `d3e46c3`, `d10bf21`. Pinned local Node evidence:142 app tests,49 topology cases,19 smoke orchestration cases,22 resolved Compose/durable evidence cases,5 browser orchestration/collision cases and10 architecture cases passed. Typecheck, lint, formatting and build passed; installation audit reported zero vulnerabilities.

## Next gate and artifact

PR51 is merged. Exact-main CI run [37215662225](https://github.com/kefyusuf/uptime-lab/actions/runs/37215662225) passed all10 jobs/gate at the landed base, including real default/custom-port Chromium, canonical Docker smoke, Go/Rust and contracts. Fresh implementation review found no Critical/Important defects. The active follow-up addresses its P3 coverage item with two permanent actual-socket tests; production code remains unchanged. Disabling header/body deadlines made both tests fail at their1000ms watchdog; restoring the deadlines passed144 app tests plus typecheck/lint/format/boundary checks. Before merging the follow-up, verify its actual-head CI and obtain user approval. No production deployment is authorized. See [Web ownership](../frontend/local-monitoring-web.md) and [verification](../testing/local-monitoring-web.md).

The user requires TDD, meaningful commits on a short-lived branch, a reviewable PR, and concise Turkish before/after explanations after completed actions. Code/docs/product copy remain English. Preserve user changes; persistent memory/configuration writes and global tooling installation are not authorized.

## Resume verification

Run `git status --short`, `git branch --show-current`, `git log -1 --format='%H %s'`, `git merge-base --is-ancestor f17d2de10622f0ff9cf33489bf633005e7863672 HEAD`, and inspect the active PR's exact-head checks. The recorded implementation commit is historical review evidence, not the current branch head. Refresh remote state and preserve user-owned changes.

This host lacks Docker. Task-local Node24.21.0/npm11.19.1 supplies application parity; Linux CI supplies real image/Compose/Chromium evidence. Fake CLI results prove orchestration only. The reviewer independently verified incomplete-header closure, incomplete-body408 closure, partial-upstream504 and interrupted-upstream502 under pinned Node; current bounded behavior passed.

## Implementation rulings

- Inbound creation-body deadline returns408 and closes the connection. The design pins the deadline without that status; unusual slow clients may receive conservative uncertain-creation feedback.
- Root build context requires `apps/web/Dockerfile.dockerignore`. Incorrect patterns could enlarge build context; the final image copies compiled outputs only.
- Remote authentication/TLS/multi-user release gates remain outside this approved local-only slice. Cost if this boundary is mistaken: premature public exposure; no remote deployment is performed.
- Go/Rust domain source is unchanged; review covers integration and existing runtime CI covers the domain. Cost if mistaken: an existing domain defect could remain outside this patch review.
- Asset replacement races require a mutable runtime filesystem, excluded by the verified read-only compiled image. Cost if that condition changes: containment assumptions must be revalidated.
- Docker/Chromium behavior and ignore-rule execution require actual Linux CI, not local source review. That evidence passed at the reviewed commit and must pass at the final head; accepting fixture evidence would risk unverified runtime integration.

## Inbound Deadline Coverage Follow-Up

- The prior P3 item now has two permanent socket tests in `server.test.ts` and `gateway.test.ts`. An incomplete header must close without reaching upstream; an unfinished JSON POST must return408/no-store, close the connection and forward zero requests. A1000ms watchdog destroys the test socket on failure; shortened100ms deadlines keep checks fast without fake timers. Mutation RED proves disabled timers are detected. The follow-up awaits PR CI and merge.
