# Current Project Handoff

**Updated:** 2026-10-15
**Verified landed baseline:** `main@a4dae6f224d1bb2af0402bf6030c2b8f9ddac565`
**Active feature implementation PR:** none; documentation PR #59 and feature PR #60 are both merged.
**Continuation checkout:** resolve the current local worktree and clean Git state before any future change; the historical worktree path below is not a current instruction.
**Current scope:** reconcile post-merge documentation and assess open R0 release decisions only. No new runtime feature, release, production migration or deployment is authorized.

## Current landed state and release boundary

- [Documentation PR #59](https://github.com/kefyusuf/uptime-lab/pull/59) was squash-merged to `main@c6103cb61d69cbf7a26d3d3f8420dd8002daef27`; [push CI #38594288750](https://github.com/kefyusuf/uptime-lab/actions/runs/38594288750) passed.
- [Pause/resume implementation PR #60](https://github.com/kefyusuf/uptime-lab/pull/60) was independently reviewed and squash-merged from exact head `e15e64ce5c3f2c8678933abc4ab14a13c4ec65e5` to [`main@a4dae6f224d1bb2af0402bf6030c2b8f9ddac565`](https://github.com/kefyusuf/uptime-lab/commit/a4dae6f224d1bb2af0402bf6030c2b8f9ddac565). [Post-merge push CI #38607266725](https://github.com/kefyusuf/uptime-lab/actions/runs/38607266725) passed policy, repository, changes, Go/PostgreSQL, public-contract, real Docker local-dev, Web/Chromium, and aggregate `CI / gate`; unchanged Checker/internal-contract jobs were skipped.
- Local scheduling GET/PUT, durable PostgreSQL pause state, claim serialization, guarded browser controls, API/Checker restart and real SQL/Chromium evidence are landed. Pausing prevents **new** claims after commit; already-claimed checks may complete. Resuming preserves the existing completion-based cadence. This is local-development acceptance, **not** a deployed-system certification.
- Migration `00004` is repository-owned SQL; source inclusion and local CI migration evidence are **not** approval to run production migration. No release tag, external deployment or production migration was performed by the #59/#60 landing.

## Open R0 decisions — not approved

The [industry benchmark and launch roadmap](../roadmap/2026-10-02-industry-benchmark-and-launch-roadmap.md) remains a proposal. Before any protected monitoring beta, define and approve: intended operator/audience and access model; supported targets and workload/admission cap; detection and notification expectations; hosting region and budget; retention and sensitive target URL policy; backup/restore objectives (RPO/RTO), named operator and escalation. A first notification channel and confirmed incident semantics remain R3/R4 design work. None has a chosen value here.

**Current release judgment:** local implementation landed / **external monitoring beta NO-GO**. CI, loopback access and read-only schema readiness do not qualify authentication, quotas, HTTPS ingress, independent notification, production backup/restore or rollback. Do not start R3/R4/R5 implementation without an explicit scoped decision.

## Historical local scheduling continuation (recorded 2026-10-08)

The entries below preserve the original branch-stage plan, worktree setup, evidence checkpoints and rulings. References to open #59/#60 PRs, pending reviews or an active feature worktree describe their 2026-10-08 state and must not be followed as current instructions.

[Feature PR 60](https://github.com/kefyusuf/uptime-lab/pull/60) stacks on open documentation PR 59 (`c05b4e7`). Tasks 1–6 committed: 7aa83db contract, a36ebec application/domain, 5aa1190 SQL serialization/races, cb9ff9a strict HTTP/composition, 239dc86 gateway, 69bc753 independent detail controls. Task 7 at fe9aca8 adds durable browser/restart acceptance and canonical documentation. All seven implementation tasks passed their completion commands. The ignored plan ledger in this worktree records historical task boundaries, rulings and test logs; preserve the unmerged worktree.

Native full Go/vet, real Linux PostgreSQL/race integration and 301 Web tests passed. Task 7 working-tree actual Chromium journeys passed 4173 and 4817 with API/Checker restart timestamps, no new paused CheckRun IDs over real claim opportunities and one resumed claim. These local observations do not substitute for final exact-head CI or independent review. Fresh whole-branch review and final affected CI/aggregate gate remain finish requirements until the PR records them.

Initial draft CI 37726542597 at 54fa8a6 passed Go, public-contract, Web and canonical Docker smoke but policy/gate failed because the draft PR title used an unapproved `monitoring` scope. The title was corrected to `api`; CI 37727675818 then caught the same scope in two earlier commit subjects. Only those subjects were reworded on this task's own draft branch. The resulting fe9aca8 tree equals 5843bd4 exactly; historical checkpoint hashes remain valid test provenance. Repository scopes also override the historical Task 7 suggested commit scope: use `test(api)`, without expanding allowed scopes. Final exact-head CI must pass the corrected range.

Runtime rulings: Windows Git Bash converted the container migration path during the first local attempt; that setup failure is not feature RED. Ignored task-only Docker/npm wrappers keep pinned host tools and suppress path conversion only for Docker calls. The scheduling acceptance helper is a small orchestration seam; it pauses only other task-fixture Monitors and waits for the real 60-second cadence, preserving displayed terminal timestamps. There is no production test hook. Task stacks remove their own containers/networks/volumes/local images; baseline images, preexisting resources and ignored module/build caches are retained.

## Earlier landed state and decision (historical)

Inventory [PR #56](https://github.com/kefyusuf/uptime-lab/pull/56) landed at `fcd844116667b34ca5c8d126882193abbff34bac`. Final feature head `730763e66c4d00ac1557e1516b9383e51938217f` passed [CI37244127913](https://github.com/kefyusuf/uptime-lab/actions/runs/37244127913); landed main passed [CI37428521056](https://github.com/kefyusuf/uptime-lab/actions/runs/37428521056). The managed inventory worktree was archived recoverably; plan scratch was removed after preserving evidence/rulings in PR #56.

Dependabot policy [PR #58](https://github.com/kefyusuf/uptime-lab/pull/58) landed at `9ed656e486c06ec3806b7968b3e97b4967523702`: `deps`/`deps-dev` scopes and explicit `build` plus scope configuration, with four RED cases followed by 17 GREEN cases. [PR #57](https://github.com/kefyusuf/uptime-lab/pull/57) landed at the recorded baseline: pinned upload-artifact 7.0.1 and commit-range checks that exclude generated merge messages while validating ordinary commits, with real merge regressions and 19 GREEN cases.

The recorded main passed all ten jobs and the aggregate gate in [CI37432170970](https://github.com/kefyusuf/uptime-lab/actions/runs/37432170970), including Go/race/real PostgreSQL, Rust/audit, contracts, Web and actual Docker/browser journeys on 4173/4817. The failure-only artifact upload step was not exercised. This is development evidence, not deployed-environment certification. No PR was open when this documentation branch started.

Review the [pause/resume scope proposal](../superpowers/specs/2026-10-08-local-monitor-pause-resume-scope-reassessment.md). Proposed semantics: stop new claims, allow already claimed work to complete, preserve existing resume cadence and result evidence. Scope approval precedes detailed design, written design approval precedes an implementation plan, and product execution follows TDD -> branch/commit -> PR. These rules remain proposals. R0 audience, workload, operator, budget and recovery decisions remain unresolved.

Subsequent user continuations authorized the [written pause/resume design](../superpowers/specs/2026-10-08-local-monitor-pause-resume-design.md), the [TDD implementation plan](../superpowers/plans/2026-10-08-local-monitor-pause-resume.md) and then Native execution. PR #59 contains scope/design/plan documentation; feature PR60 is stacked on it. Earlier artifact approval-stage wording is historical. Actual PostgreSQL snapshot-interleaving evidence is implemented and qualified independently of browser timing.

## Historical inventory implementation evidence

The user approved merging plan55 and executing all seven dependent tasks through Native TDD. All seven tasks are complete:532db58 contract,4c20401 cursor/use case,666a6d3 SQL RED,336ce4b retrieval/index,0a1fb9c HTTP,96dc3cb maximum-row budget,21a8c70 gateway,1793e25 UI and1386e98 durable Monitor/browser/index acceptance, followed by canonical documentation57e3ad7.

Actual PostgreSQL RED in CI37239817150 detected the absent inventory migration. SQL GREEN336ce4b passed CI37240193666 and isolated local Linux-container integration. Actual Chromium RED3b5d4d3 in CI37241929991 expected20 rows and received1 before the controlled21-row preparation. GREEN code1386e98 passed [CI37242623843](https://github.com/kefyusuf/uptime-lab/actions/runs/37242623843), including actual default/custom-port browser inventory journeys, canonical Docker smoke, Go/PostgreSQL/race/vulnerability, public contract and Web quality gates. This evidence certifies that code checkpoint, not later changed heads.

Landed main serves five public operations. Inventory is local-only, default20/max50,128 raw query bytes and245760 JSON bytes, ordered by persisted microsecond creation time/UUID. A first oversized item returns fixed500 and blocks older traversal without truncation/skipping. Cursors provide no authorization, ownership or snapshot guarantee. Lifecycle/history, aggregation and remote release remain separate.

Local Task6 evidence:221 Web tests, generated parity, typecheck, lint, package-context format, boundaries, build and zero-vulnerability audit passed. Task7 evidence mutation tests12, fake browser orchestration6 and fake smoke20 passed; fake CLI results are never browser/SQL proof. Existing creation/detail/availability/Checker behavior remains covered.

## Inventory review and retained limitation

Documentation head57e3ad7 passed [CI37243527314](https://github.com/kefyusuf/uptime-lab/actions/runs/37243527314);83 relative document links, architecture/repository, immutable migration history, contract/generated parity and whitespace also passed. Fresh whole-branch review found no critical issue and two important defects: browser URL-policy mismatch and outer HTTP canonical redirects. Both were reproduced and fixed in one TDD pass; the Go suite/vet and227 Web tests/typecheck/lint/format passed. See [review evidence and deferred EXPLAIN minor](../testing/local-monitoring-web.md#independent-inventory-review).

Those finish gates completed before the approved merge of PR #56. The deferred minor is representative populated-data JSON EXPLAIN evidence: existing index checks do not qualify performance, capacity or a supported Monitor count. Lifecycle/history and remote release remain separate.

## Historical worktree verification (not a current command)

Use the active unmerged feature worktree above; the primary checkout remains the documentation branch. Inspect `git status --short --branch`, `git log -1 --format='%H %s'`, worktree inventory, current PRs and exact-head checks. Verify that the recorded landed baseline remains an ancestor. Preserve user-owned changes and check the plan ledger before repeating work. PR #56 holds historical inventory review/CI rulings; do not resume from its archived worktree.

## Runtime and rulings

- Docker and task-local Node24.21.0/npm11.19.1 were available for the historical runtime evidence. Recheck local tooling before future runtime work. This documentation task created or removed no Docker resources; all preexisting resources were retained untouched.
- Git Bash Docker path conversion prevented the original PostgreSQL wrapper invocation. An ignored native PowerShell equivalent uses the same pinned Linux Go/PostgreSQL images, read-only source, suite order, isolated project and cleanup. Linux CI independently passes the direct integration suites. No global tooling change or historical migration edit was made.
- Windows race requires CGO; actual Linux CI supplies race and pinned govulncheck evidence.
- The failed Docker mount conversion created an empty `apps/api;C` directory. Its creation time and empty contents were verified; that exact session artifact was removed. Existing topology rules were preserved.
- Historical specs/plans retain their approval-stage wording. Canonical runtime/testing documents describe implemented behavior; the approved oversized-item traversal limitation remains binding.
- No new dependencies, remote exposure, persistent configuration or memory writes were authorized.

## Historical local Web baseline

R1 local browser capability landed through PR51 atf17d2de, then permanent inbound deadline coverage through PR52 at75200ab. Exact-main CI37215662225 passed all10 jobs at the PR51 runtime; CI37221902477 passed applicable Web/repository checks plus real Docker/default/custom-port journeys at the follow-up runtime, while unchanged Go/Rust/contracts were skipped. These are historical checkpoint evidence.

Registration remains conservative for uncertain creation: it warns that the Monitor may exist and never retries automatically. Independent raw/availability CheckIDs and UTC snapshots remain separate. Permanent actual-socket deadline tests detect missing incomplete-header closure and unfinished-body408/no-store closure before upstream work. No pending R1 deadline task remains.

The user requires TDD, meaningful branch commits, a reviewable PR and concise Turkish before/after explanations. Code, docs and product copy remain English. See [Web ownership](../frontend/local-monitoring-web.md) and [verification](../testing/local-monitoring-web.md).
