# Current Project Handoff

**Recorded:** 2026-10-05
**Verified landed base:** `f50eca01fd74b6636113a7c8afd6126f8aeed879` (approved plan55 after scope53/design54)
**Active branch:** `feat/local-monitor-inventory`
**Managed worktree:** `C:\Users\yukonit\.codex\worktrees\local-monitor-inventory\uptime-lab`
**Implementation PR:** [#56](https://github.com/kefyusuf/uptime-lab/pull/56), draft until the review-fix head passes affected CI and the aggregate gate

## Current implementation

The user approved merging plan55 and executing all seven dependent tasks through Native TDD. All seven tasks are complete:532db58 contract,4c20401 cursor/use case,666a6d3 SQL RED,336ce4b retrieval/index,0a1fb9c HTTP,96dc3cb maximum-row budget,21a8c70 gateway,1793e25 UI and1386e98 durable Monitor/browser/index acceptance, followed by canonical documentation57e3ad7.

Actual PostgreSQL RED in CI37239817150 detected the absent inventory migration. SQL GREEN336ce4b passed CI37240193666 and isolated local Linux-container integration. Actual Chromium RED3b5d4d3 in CI37241929991 expected20 rows and received1 before the controlled21-row preparation. GREEN code1386e98 passed [CI37242623843](https://github.com/kefyusuf/uptime-lab/actions/runs/37242623843), including actual default/custom-port browser inventory journeys, canonical Docker smoke, Go/PostgreSQL/race/vulnerability, public contract and Web quality gates. This evidence certifies that code checkpoint, not later changed heads.

Go now serves five public operations on this branch. Inventory is local-only, default20/max50,128 raw query bytes and245760 JSON bytes, ordered by persisted microsecond creation time/UUID. A first oversized item returns fixed500 and blocks older traversal without truncation/skipping. Cursors provide no authorization, ownership or snapshot guarantee. Lifecycle/history, aggregation and remote release remain separate.

Local Task6 evidence:221 Web tests, generated parity, typecheck, lint, package-context format, boundaries, build and zero-vulnerability audit passed. Task7 evidence mutation tests12, fake browser orchestration6 and fake smoke20 passed; fake CLI results are never browser/SQL proof. Existing creation/detail/availability/Checker behavior remains covered.

## Final review and remaining finish gates

Documentation head57e3ad7 passed [CI37243527314](https://github.com/kefyusuf/uptime-lab/actions/runs/37243527314);83 relative document links, architecture/repository, immutable migration history, contract/generated parity and whitespace also passed. Fresh whole-branch review found no critical issue and two important defects: browser URL-policy mismatch and outer HTTP canonical redirects. Both were reproduced and fixed in one TDD pass; the Go suite/vet and227 Web tests/typecheck/lint/format passed. See [review evidence and deferred EXPLAIN minor](../testing/local-monitoring-web.md#independent-inventory-review).

Require the final review-fix head's affected CI jobs and aggregate gate, then finalize the same PR and request merge approval. Preserve final evidence/rulings in the PR before deleting only this plan's ignored scratch directory. Keep the managed implementation worktree for PR feedback. Do not deploy or merge without the corresponding authorization.

## Resume verification

Use the attached inventory worktree above. Run `git status --short`, `git branch --show-current`, `git log -1 --format='%H %s'`, `git merge-base --is-ancestor f50eca01fd74b6636113a7c8afd6126f8aeed879 HEAD`, and inspect PR56 plus exact-head checks. All task checkpoints are recorded above; the finalized PR holds final review, CI and execution rulings after scratch cleanup. Preserve user-owned changes. The prior local Web worktree is archived; primary main at `E:\projects\uptime-lab` is separate from this implementation checkout.

## Runtime and rulings

- Docker is now available. Task-local Node24.21.0/npm11.19.1 provides Web parity.
- Git Bash Docker path conversion prevented the original PostgreSQL wrapper invocation. An ignored native PowerShell equivalent uses the same pinned Linux Go/PostgreSQL images, read-only source, suite order, isolated project and cleanup. Linux CI independently passes the direct integration suites. No global tooling change or historical migration edit was made.
- Windows race requires CGO; actual Linux CI supplies race and pinned govulncheck evidence.
- The failed Docker mount conversion created an empty `apps/api;C` directory. Its creation time and empty contents were verified; that exact session artifact was removed. Existing topology rules were preserved.
- Historical specs/plans retain their approval-stage wording. Canonical runtime/testing documents describe implemented behavior; the approved oversized-item traversal limitation remains binding.
- No new dependencies, remote exposure, persistent configuration or memory writes were authorized.

## Historical local Web baseline

R1 local browser capability landed through PR51 atf17d2de, then permanent inbound deadline coverage through PR52 at75200ab. Exact-main CI37215662225 passed all10 jobs at the PR51 runtime; CI37221902477 passed applicable Web/repository checks plus real Docker/default/custom-port journeys at the follow-up runtime, while unchanged Go/Rust/contracts were skipped. These are historical checkpoint evidence.

Registration remains conservative for uncertain creation: it warns that the Monitor may exist and never retries automatically. Independent raw/availability CheckIDs and UTC snapshots remain separate. Permanent actual-socket deadline tests detect missing incomplete-header closure and unfinished-body408/no-store closure before upstream work. No pending R1 deadline task remains.

The user requires TDD, meaningful branch commits, a reviewable PR and concise Turkish before/after explanations. Code, docs and product copy remain English. See [Web ownership](../frontend/local-monitoring-web.md) and [verification](../testing/local-monitoring-web.md).
