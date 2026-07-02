# a9s Cache — Requirements

Status: DRAFT for product-owner review.
Audience: a developer who has never seen the codebase, and an operator who will verify the behavior against a real AWS account.

This document defines TARGET behavior, implementation-blind. Where current behavior differs, current behavior is wrong.

Context: a9s is a read-only AWS viewer with two renderers (TUI and web) over one shared headless view-state; 66 resource types; each type has a menu entry (count + issue badge), a list screen, and a detail screen with a related-resources panel. Sessions are scoped to one AWS profile+region pair.

## 1. Goals

1. The operator never stares at an empty screen when a previous session already learned the answer.
2. Staleness is always visible: "cached, being verified" is distinguishable from "fresh this session" at a glance.
3. Knowledge is never destroyed by a session that didn't re-learn it.
4. Both renderers behave identically; renderers contain zero cache logic.
5. One mechanism for all 66 types; a rule that needs per-type code is not met.
6. No user intent is silently lost.

Non-goals: cross-profile sharing, multi-region aggregation, any write path to AWS, multi-page row caching, live propagation between concurrent instances.

## 2. Rules

**C1 — Show what you know, verify on sight.** At startup and on every screen entry, anything cached renders instantly, visibly marked stale, and is re-verified immediately in the background. There is no TTL — a DELIBERATE product decision: arbitrarily old rows may render as long as they are stale-marked and re-verification is already running, because a marked-and-verifying answer beats an empty screen. Approving this document approves that trade-off. A menu entry whose type was never probed shows a neutral placeholder — never `0`.

**C2 — Silent swap.** A completed fetch replaces cached content in place: no flicker, cursor follows the stable resource ID (or ordinal fallback), filter and sort re-apply, the stale mark drops in the same frame. A result older than a later invalidation (manual refresh, pair switch, newer fetch of the same content) is discarded.

**C3 — One staleness signal.** Each surface carries one boolean overlay in the view-state: the menu shows a single global "updating" indicator while the verification sweep runs and dims cache-origin numbers; a list or detail rendered from cache shows one refreshing marker (e.g. `⟳` in the frame title). Renderers read the flags; they never compute them.

**C4 — Never block the transport.** A web response returns in < 200 ms regardless of fetch state; the TUI event loop stays responsive during any fetch. Fetch results always arrive as asynchronous view-state updates. A never-cached list shows `Loading…` (frame, title and headers present) until its first fetch lands — the only case where a bare loading state is permitted. A fetch failure while cached content is on screen keeps the content, swaps the marker for an error marker, and logs once — nothing goes blank.

**C5 — Exact totals stick.** When paging reaches the last page, the type's total becomes exact: displayed everywhere as `N` (not `N+`), persisted, surviving restart. Exactness is only ever replaced by newer exactness: a full-depth fetch, or a first-page fetch that is itself complete (untruncated — the whole population fits one page), produces a new exact total; a TRUNCATED first-page fetch never downgrades a stored exact total to `N+` — it merely knows less, and the exact value stays (stale-marked) until the next exact observation corrects it.

**C6 — What is cached.**
Persisted, one file per profile+region, structured per the data contract below. Issue counts everywhere in this document follow the menu-badge aggregation: only `!`-severity findings count; warning (`~`) findings never bump a count (see docs/attention-signals.md, Visualization Surfaces).
Session-only, per resource: related-panel results and detail enrichments. A related-panel result stores everything its interactions need — target identifiers, fetch filters/scope, approximate/truncated markers, and per-row errors — so that activating a cached row navigates identically to a fresh one and never triggers the fan-out. Session caches live until `Ctrl+R` on their screen or a pair switch, and are exempt from C1's re-verify-on-sight: re-opening a detail renders the cached panel in < 100 ms and MUST NOT re-run the fan-out (that re-run is defect D6).

**C7 — Merge on save.** Saving reads the latest file, merges the session's knowledge PER FIELD GROUP (newest group saved-at wins), and writes via atomic rename. A session that only re-learned s3's count must not disturb s3's cached rows or findings, nor any other type's entry. Two concurrent instances may cost each other one save; the loss self-heals on the next view (C1). No locks.

**Data contract.** The persisted file is: a single integer schema version, then per type three independently-timestamped field groups — `count {value, exact, savedAt}`, `issues {value, truncated, savedAt}`, `rows {items with their findings, savedAt}`. Merge-on-save (C7) and staleness are decided per group by its own savedAt; the UI still surfaces a single per-surface staleness overlay (C3) — per-group timestamps exist for correct merging, not for extra UI states. An unreadable or wrong-version file means "no cache" (normal cold start, one log line, replaced on the next save).

**C8 — Manual refresh.** `Ctrl+R` re-verifies the current surface (menu: the availability sweep; list: that type's rows and findings; detail: fan-out and enrichment). Cached content stays visible under the marker while the refetch runs; other types are untouched.

**C9 — Pair isolation.** Everything is keyed by profile+region. Switching drops the old pair's in-memory state atomically, loads the new pair's file per C1, and discards in-flight results of the old pair. No frame mixes two pairs.

**C10 — Pre-connect intent.** A navigation issued before the AWS connection is ready renders cached content immediately (C1) and replays automatically once connected; only the LAST navigation is replayed, and a pair switch clears it.

Lifecycle, all content kinds: `none → cached (stale) → fresh`; "refreshing" is an overlay flag, not a state; invalidation (C8/C9) moves fresh back to cached-or-none.

## 3. Observed defects this rules out

| # | Defect (observed live) | Ruled out by |
|---|---|---|
| D1 | Web starts with an empty menu while a cache exists; the TUI shows cached counts | C1 + goal 4 |
| D2 | No visible "updating" indication during background verification | C3 |
| D3 | Opening a never-cached list blocks the web request ~11 s with no render | C4 |
| D4 | A command before connection readiness silently loses the fetch | C10 |
| D5 | Persisting an exact total overwrote the file, destroying other types' knowledge | C7 |
| D6 | Re-opening a detail re-ran the full related fan-out (~18 s) in one UI | C6 |

## 4. S3 pilot acceptance

Run in BOTH renderers against a real account with ≥ 55 buckets (first page truncates at 50), starting with no cache file for the pair.

1. **Cold start.** Menu renders immediately; s3 shows a neutral placeholder (never `0`); global updating indicator visible; the entry becomes `s3(50+)` as verification lands; the indicator ends with the sweep.
2. **Open s3 cold.** Frame renders < 200 ms (`Loading…`); rows appear when the fetch lands; the web request never hangs for the fetch.
3. **Page to the end.** `m` until paging stops → title `s3(55)`, exact, no `+`.
4. **Back to menu.** `s3 (55)` — exact survives navigation, no refetch triggered by the navigation itself.
5. **Warm restart.** First frame < 100 ms shows `s3 (55)` dimmed + updating indicator; verification re-confirms; never `0`, never an unexplained `50+` mid-sweep.
6. **Other-type knowledge survives.** A different type's issue badge noted before the restart still renders from cache after it (merge-on-save, C7).
7. **Open s3 warm.** Cached rows with their glyphs and status texts render < 100 ms; `⟳` marker; silent swap on fetch completion; cursor and scroll survive.
8. **Manual refresh.** `Ctrl+R` in the list: rows stay visible, marker returns, findings re-verify, other menu entries untouched.
9. **Detail + panel.** Open a bucket's detail: fan-out runs once, panel populates. Leave and re-open the SAME bucket: panel renders < 100 ms from cache, no re-fan-out, no multi-second stall; `Ctrl+R` re-runs it.
10. **Pre-connect intent.** Restart and immediately press `Enter` on s3: cached rows show at once with the marker; fresh rows arrive with no further input.
11. **Failure visibility.** With rows on screen, break credentials/network, `Ctrl+R`: rows remain, error marker + one log entry, nothing blanks.
12. **Corrupt file.** Truncate the cache file; start: normal cold start, one log entry, next save writes a healthy file.

The pilot passes when all 12 steps hold in both renderers.

## 5. Extension to all 66 types

Nothing per-type: the mechanism is driven entirely by the type registry, so extension is verification, not implementation. Verify by re-running steps 1, 5, 7 and 9 on a sample covering the structural classes (paginated type, zero-resource type, issue-badge-excluded type, child-list type, related-heavy type), plus an automated all-types sweep asserting: cached render marked stale, silent swap, merge-on-save leaves siblings untouched.
