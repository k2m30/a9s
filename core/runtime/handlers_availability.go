// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package runtime

// handlers_availability.go — app-core availability + Wave-2 enrichment handlers.
//
// These are (c *Core) handlers reading session state via c.session; view
// updates are returned as []UIIntent and async probe dispatch as
// []TaskRequest.
//
// Handler dispatch:
//
//	handleAvailabilityCacheLoaded → seeds menu from disk cache, queues probes.
//	handleAvailabilityPrefetched  → applies sync prefetch counts (demo / no-cache).
//	handleAvailabilityChecked     → applies one Wave-1 probe result, fires next.
//	startEnrichment               → builds Wave-2 queue, returns probe tasks.
//	handleEnrichmentChecked       → applies one Wave-2 result, fires next.
//	unifiedIssueCount             → cross-wave de-duped issue count for S1 badge.

import (
	"fmt"
	"maps"
	"time"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/cache"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/logging"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/core/session"
)

// handleAvailabilityCacheLoaded applies cached entries to the main menu and
// starts background availability checks.
func (c *Core) handleAvailabilityCacheLoaded(msg messages.AvailabilityCacheLoaded) ([]UIIntent, []TaskRequest) {
	canonKey := func(k string) string {
		if td := resource.FindResourceType(k); td != nil {
			return td.ShortName
		}
		return k
	}
	canonIntMap := func(src map[string]int) map[string]int {
		dst := make(map[string]int, len(src))
		for k, v := range src {
			dst[canonKey(k)] = v
		}
		return dst
	}
	canonBoolMap := func(src map[string]bool) map[string]bool {
		dst := make(map[string]bool, len(src))
		for k, v := range src {
			dst[canonKey(k)] = v
		}
		return dst
	}
	entries := canonIntMap(msg.Entries)
	truncated := canonBoolMap(msg.Truncated)
	issueCounts := canonIntMap(msg.IssueCounts)
	issueTruncated := canonBoolMap(msg.IssueTruncated)
	issueKnown := canonBoolMap(msg.IssueKnown)

	var intents []UIIntent

	// Emit one PatchMenuAvailability intent per resource type with cached data.
	// Per cache contract C3: Origin="cache" — seeded from disk, not yet re-verified this
	// session (handleAvailabilityChecked flips it to "verified" once the
	// matching live probe result lands).
	for shortName, count := range entries {
		intents = append(intents, PatchMenuAvailability{
			ResourceType: shortName,
			Count:        count,
			Truncated:    truncated[shortName],
			Origin:       OriginCache,
		})
	}

	// Per C1: seed RowStore with the disk-cached rows for every
	// known type so a cold list-open renders real cells instantly
	// (Loading=false, Refreshing=true) instead of the empty Loading shell —
	// mirrors Count/Truncated already being applied to the menu above.
	//
	// Prefers real per-row data from the current pair's disk Store (all pages
	// C6 persisted, not just the first) when available. Falls back to
	// Count-many placeholder rows (ID-only, no Fields) when the Store has no
	// row data for a type that nonetheless carries a known count — this keeps
	// the seeded-list contract (Loading=false) intact even for callers that
	// deliver a counts-only AvailabilityCacheLoaded event without a populated
	// on-disk per-type file (e.g. a synthetic/legacy counts projection).
	//
	// task #17 wave 1 stage 2: this is now a store-only write — the
	// already-observed guard reads RowStore.ProbeOriginTypeNames' membership
	// test instead of the removed session.ProbeResources map.
	if len(entries) > 0 {
		alreadyObserved := make(map[string]struct{})
		for _, name := range c.session.RowStore.ProbeOriginTypeNames() {
			alreadyObserved[name] = struct{}{}
		}
		rowsByType := make(map[string][]cache.Row, len(entries))
		_ = c.ReadCacheStore(func(store *cache.Store) error {
			if store == nil {
				return nil
			}
			for shortName := range entries {
				if tf, ok := store.Type(shortName); ok && len(tf.Rows) > 0 {
					rowsByType[shortName] = tf.Rows
				}
			}
			return nil
		})
		for shortName, count := range entries {
			if count <= 0 {
				continue
			}
			if _, already := alreadyObserved[shortName]; already {
				continue
			}
			var realRows []resource.Resource
			if cr, ok := rowsByType[shortName]; ok {
				realRows = rowsFromCacheRows(shortName, cr)
			}
			// Dual-write (task #17 wave 1): only real per-type disk row data
			// feeds RowStore's rows-carrying Observe (OriginDisk — a later live
			// probe/fetch result is never regressed by a race-losing disk seed).
			// A type with no real disk rows (placeholder-only fallback) is a
			// counts-only observation (C6a): it must never fabricate Rows, so it
			// feeds ObserveCount instead of Observe.
			if len(realRows) > 0 {
				c.ObserveRows(shortName, realRows, &resource.PaginationMeta{IsTruncated: truncated[shortName]}, session.OriginDisk, false)
				// ObserveRows/Observe sets TotalCount=len(rows) unconditionally, which
				// silently drops a C6a count exceeding the seeded row page (e.g. a
				// 55-row disk count with only 50 real rows on disk) — the first list
				// frame would then show 50+ instead of 55+. ObserveCountRows'
				// counts-only write (C6a contract, rowstore.go's ObserveCount doc
				// comment) restores the wider count without touching Rows.
				if count > len(realRows) {
					c.ObserveCountRows(shortName, count)
				}
			} else {
				c.ObserveCountRows(shortName, count)
			}
		}
	}

	// Apply cached issue counts (T033).
	if len(issueKnown) > 0 {
		intents = append(intents, PatchMenuIssueBatch{
			Counts:    issueCounts,
			Truncated: issueTruncated,
			Known:     issueKnown,
		})
	}

	// Sweep-once-per-pair: a pair already swept this session (PairSwept)
	// skips rebuilding AvailQueue/firing probes and just reports the
	// progress intent as done. Disk-cache seeding above (menu counts +
	// RowStore) runs unconditionally either way.
	allNames := resource.AllShortNames()

	var tasks []TaskRequest
	switch {
	case c.session.PairSwept():
		c.session.AvailTotal = len(allNames)
		c.session.AvailChecked = c.session.AvailTotal
		intents = append(intents, PatchMenuCheckProgress{Checked: c.session.AvailChecked, Total: c.session.AvailTotal})
	case c.session.AvailTotal > 0:
		// A sweep for this pair is already under way: an earlier
		// AvailabilityCacheLoaded already built AvailQueue (messages.
		// AvailabilityCacheLoaded carries no Gen, so both the pre-connect
		// disk-cache seed and handleClientsReadySuccess's own unconditional
		// TaskKindLoadAvailCache dispatch land here for the same pair before
		// PairSwept goes true). Rebuilding AvailQueue/AvailChecked/AvailTotal
		// here would restart the sweep on top of probes already in flight,
		// double-dispatching every type and re-running this handler's
		// completion bookkeeping once per rebuild — report the current, real
		// progress instead. ClearPairSwept (manual full-menu refresh) resets
		// AvailTotal to 0 alongside the swept memo specifically so that
		// restart still reaches the fresh-start case below.
		intents = append(intents, PatchMenuCheckProgress{Checked: c.session.AvailChecked, Total: c.session.AvailTotal})
	default:
		c.session.AvailQueue = allNames
		c.session.AvailChecked = 0
		c.session.AvailTotal = len(allNames)
		c.session.ScanHealthLogged = make(map[string]bool)

		intents = append(intents, PatchMenuCheckProgress{Checked: 0, Total: c.session.AvailTotal})

		// The disk-cache load races ahead of the AWS connect on startup by
		// design (Init fires both concurrently via tea.Batch for
		// instant-paint, C1) — it routinely wins, since local disk I/O
		// finishes long before a network connect settles. Dispatching probe
		// tasks while Clients is still nil would run every one of them
		// against a nil transport and fail hard ("AWS clients not
		// initialized"), permanently losing that probe for the session
		// instead of actually checking availability (the four resource
		// types first in resource.AllShortNames() were observed failing
		// this way). Latch AvailSweepPending instead; the next successful
		// HandleClientsReady drains the first batch once a real transport
		// exists.
		if c.session.Clients != nil {
			tasks = append(tasks, c.fireNextAvailabilityProbes(4)...)
		} else {
			c.session.AvailSweepPending = true
		}
	}

	// Deferred -c navigation (D11): consume the one-shot -c navigation armed by
	// handleClientsReadySuccess on the live path, now that the
	// session.ProbeResources seed above has landed — dispatching it any
	// earlier (e.g. alongside TaskKindLoadAvailCache) would race the seed,
	// since the adapter runs task cmds concurrently via tea.Batch.
	if c.session.CommandArmed {
		tasks = append(tasks, emitNavigateForCommand(c.session.PendingCommand))
		c.session.CommandArmed = false
		c.session.PendingCommand = ""
	}

	return intents, tasks
}

// fireNextAvailabilityProbes pops up to n resource types off AvailQueue and
// returns one TaskKindProbeAvailability TaskRequest per type. Callers MUST
// hold c.session.Clients != nil before calling — this only shrinks the
// queue and builds tasks, it does not itself check readiness (see
// handleAvailabilityCacheLoaded's AvailSweepPending gate and
// handleClientsReadySuccess's drain for the two call sites and their
// respective readiness checks).
func (c *Core) fireNextAvailabilityProbes(n int) []TaskRequest {
	var tasks []TaskRequest
	for i := 0; i < n && len(c.session.AvailQueue) > 0; i++ {
		shortName := c.session.AvailQueue[0]
		c.session.AvailQueue = c.session.AvailQueue[1:]
		tasks = append(tasks, TaskRequest{Key: TaskKey{Kind: TaskKindProbeAvailability, Scope: shortName}})
	}
	return tasks
}

// handleAvailabilityPrefetched applies synchronously-prefetched counts to the
// main menu.  Used in no-cache + pre-supplied-clients mode so counts appear
// immediately without background probes.
func (c *Core) handleAvailabilityPrefetched(msg messages.AvailabilityPrefetched) ([]UIIntent, []TaskRequest) {
	var intents []UIIntent

	for shortName, count := range msg.Entries {
		intents = append(intents, PatchMenuAvailability{
			ResourceType: shortName,
			Count:        count,
			Truncated:    msg.Truncated[shortName],
			// Per cache contract C3: a prefetch is a synchronous LIVE count (demo /
			// no-cache mode), not a disk-cache seed — origin is "verified"
			// from the moment it lands, no separate probe confirms it.
			Origin: OriginVerified,
		})
	}
	// T034: wire issue counts from prefetch.
	for shortName, count := range msg.IssueCounts {
		intents = append(intents, PatchMenu{
			ResourceType: shortName,
			Issues:       count,
			Truncated:    msg.IssueTruncated[shortName],
		})
	}
	intents = append(intents, PatchMenuCheckProgress{Checked: 0, Total: 0}) // signal "done"

	// T034: retain prefetch resources for Wave-2 enrichment. Fetcher-emitted
	// rows already carry Findings; no re-derive needed (W1.4b.3 dropped the
	// legacy Status/Issues bridge). task #17 wave 1 stage 3: store-only —
	// the former ResourceCache map write is gone, ObserveRows below is this
	// write's only destination. A synchronous prefetch is a live, full
	// result — OriginFetch, wholesale replace.
	if msg.Resources != nil {
		for rt, resources := range msg.Resources {
			pageMeta := msg.Pagination[rt]
			if pageMeta == nil {
				pageMeta = &resource.PaginationMeta{IsTruncated: msg.Truncated[rt]}
			}
			c.ObserveRows(rt, resources, pageMeta, session.OriginFetch, false)
		}
	}

	enrichIntents, enrichTasks := c.startEnrichment()
	intents = append(intents, enrichIntents...)

	if msg.PrefetchErr != nil {
		err := msg.PrefetchErr
		intents = append(intents, FlashIntent{
			Text:    "availability: " + err.Error(),
			IsError: true,
		})
	}
	// Partial failures (rows present) go to the `!` error log only — the
	// rows already render with their degraded-state findings; a blocking
	// banner would double-shout what the list is honestly showing.
	if msg.PrefetchSoftErr != nil {
		intents = append(intents, AppendErrorHistoryIntent{
			Time:    time.Now(),
			Message: "availability: " + msg.PrefetchSoftErr.Error(),
		})
	}

	return intents, enrichTasks
}

// handleAvailabilityChecked processes a single resource type's probe result.
func (c *Core) handleAvailabilityChecked(msg messages.AvailabilityChecked) ([]UIIntent, []TaskRequest) {
	c.session.AvailChecked++

	// #462: record this probe's scan status regardless of outcome — a
	// hard-failed probe still needs to be visible via Core.ScanStatus.
	outcome, errClass := availabilityOutcome(c, msg.ResourceType, len(msg.Resources) > 0, msg.Truncated, msg.Err)
	// This probe IS the new Wave-1 baseline: aggregate == baseline.
	c.setProbeStatus(msg.ResourceType, outcome, msg.Duration, errClass, time.Now(), outcome, msg.Duration, errClass)

	var intents []UIIntent
	var tasks []TaskRequest

	// The row says why it has no fresh answer. A partial result is not a
	// refusal — its rows are on screen — so only a hard failure marks the row,
	// and any other outcome clears a mark an earlier sweep left.
	cause := ""
	if outcome == ProbeFailed {
		cause = errClass
	}
	intents = append(intents, PatchMenuProbeCause{ResourceType: msg.ResourceType, Cause: cause})

	// Update menu availability on full success or partial-success.
	if msg.Err == nil || len(msg.Resources) > 0 {
		intents = append(intents, PatchMenuAvailability{
			ResourceType: msg.ResourceType,
			Count:        msg.Count,
			Truncated:    msg.Truncated,
			// Per cache contract C3: a live AvailabilityChecked result confirms this type
			// this session — origin flips from "cache" (or unset) to
			// "verified" regardless of whether the exactness guard in
			// applyIntents' PatchMenuAvailability case ends up keeping the
			// prior Count/Truncated.
			Origin: OriginVerified,
		})
		// T032: wire issue counts from probe.
		intents = append(intents, PatchMenu{
			ResourceType: msg.ResourceType,
			Issues:       msg.Issues,
			Truncated:    msg.Truncated,
		})

		// T032: retain probe resources for Wave-2 enrichment. Fetcher-emitted
		// rows already carry Findings; no re-derive needed (W1.4b.3 dropped
		// the legacy Status/Issues bridge).
		canonType := msg.ResourceType
		if td := resource.FindResourceType(msg.ResourceType); td != nil {
			canonType = td.ShortName
		}
		// C6b/D17: this bare Wave-1 probe result carries no Wave-2 data of its
		// own — if a prior Wave-2 enrichment pass this session already wrote
		// Findings/Fields onto the type's previous in-memory rows, a plain
		// overwrite here would blank them until the next enrichment sweep
		// completes (the visible mid-session blink D17 describes). Carry
		// forward the previous rows' Wave-2-sourced Findings and this type's
		// registered enricher Fields keys per matching resource ID first.
		// task #17 wave 1 stage 2: the "previous rows" read is now RowStore's
		// own current snapshot for canonType (replaces the removed
		// session.ProbeResources[canonType] read) — the Wave-2-carried result
		// IS this probe's final row state, and ObserveRows below is its only
		// write destination now.
		previous, _ := c.ProbeResources(canonType)
		freshResources := carryWave2ForResources(previous, msg.Resources, issueEnricherFieldKeysFor(canonType))
		c.ObserveRows(canonType, freshResources, &resource.PaginationMeta{IsTruncated: msg.Truncated}, session.OriginProbe, false)
	}

	// Surface probe failures. Partial success (rows arrived alongside a
	// composite per-item error — the E5 contract, e.g. a role that can list
	// but not describe some resources) records into the `!` error log
	// without a blocking banner: the rows are already on screen carrying
	// their degraded-state findings. A region gap (the service endpoint's
	// DNS does not resolve — the service is not offered in this region)
	// logs a plain-language one-liner instead of transport jargon. Any
	// other row-less failure banners as before.
	//
	// #462: exactly one scan-health entry per type per sweep.
	// c.session.ScanHealthLogged (cleared at sweep start and by Rotate())
	// guards against a type's AvailabilityChecked message being delivered
	// more than once within the same sweep — a pre-existing double-delivery
	// path elsewhere in dispatch — re-adding the entry or re-flashing the
	// banner on the redundant delivery. The default case's FlashIntent is
	// the ONLY source of that case's entry (the adapter re-emits it as
	// messages.Flash, which routes through HandleFlash and appends the
	// history entry there — see runtime_adapter.go's applyIntents), so
	// gating that emission on the guard is sufficient to gate the entry too.
	//
	// The new "probe <type>: <outcome>: <detail>" shape applies only to the
	// default (row-less hard failure) case below. The partial-success-with-
	// rows case keeps msg.Err's own composite per-item text verbatim (e.g.
	// "partial: throttled on 1 of 3 IDs") — classifyProbeErr's single-code
	// classification cannot represent a composite E5 error, and the
	// composite text is the more useful diagnostic for that case.
	if msg.Err != nil && !c.session.ScanHealthLogged[msg.ResourceType] {
		if c.session.ScanHealthLogged == nil {
			c.session.ScanHealthLogged = make(map[string]bool)
		}
		c.session.ScanHealthLogged[msg.ResourceType] = true
		logging.L().Warn("scan probe failed", "type", msg.ResourceType, "outcome", string(outcome), "detail", errClass)

		switch {
		case len(msg.Resources) > 0:
			intents = append(intents, AppendErrorHistoryIntent{
				Time:    time.Now(),
				Message: fmt.Sprintf("availability %s: %v", msg.ResourceType, msg.Err),
			})
		case awsclient.IsEndpointNotFound(msg.Err):
			_, region := c.session.CurrentPair()
			intents = append(intents, AppendErrorHistoryIntent{
				Time:    time.Now(),
				Message: fmt.Sprintf("%s: service not available in region %s", msg.ResourceType, region),
			})
		default:
			intents = append(intents, FlashIntent{
				Text:    fmt.Sprintf("probe %s: %s: %s", msg.ResourceType, outcome, errClass),
				IsError: true,
			})
		}
	}

	intents = append(intents, PatchMenuCheckProgress{
		Checked: c.session.AvailChecked,
		Total:   c.session.AvailTotal,
	})

	// If queue has more items, fire next probe.
	if len(c.session.AvailQueue) > 0 {
		next := c.session.AvailQueue[0]
		c.session.AvailQueue = c.session.AvailQueue[1:]
		tasks = append(tasks, TaskRequest{Key: TaskKey{Kind: TaskKindProbeAvailability, Scope: next}})
		return intents, tasks
	}

	// Queue is drained but other probes may still be in flight.
	if c.session.AvailChecked < c.session.AvailTotal {
		return intents, tasks
	}

	// Completion latch: once this pair's sweep has already been marked
	// swept, a further AvailabilityChecked delivery reaching this point (a
	// redelivered/duplicate probe result pushing AvailChecked to or past
	// AvailTotal again) must not re-run completion. MarkPairSwept itself is
	// idempotent, but startEnrichment below rebuilds EnrichQueue and bumps
	// EnrichmentGen unconditionally, discarding whatever Wave-2 probes the
	// first completion already dispatched.
	if c.session.PairSwept() {
		return intents, tasks
	}

	// All checks done — clear progress indicator and save cache.
	intents = append(intents, PatchMenuCheckProgress{Checked: 0, Total: 0}) // 0,0 = done
	intents = append(intents, ClearFlash{})

	// The current profile/region pair's Wave-1 sweep has now genuinely
	// completed (not just interrupted by a mid-sweep pair switch) — memoize
	// it so a later revisit of this same pair skips the redundant sweep
	// (see Session.PairSwept's doc comment). Guarded on a non-empty sweep so
	// a stray zero-total call can never trivially mark a pair swept.
	if c.session.AvailTotal > 0 {
		c.session.MarkPairSwept()
	}

	// Dispatch-time payload freeze (restated on RowStore): snapshot RowStore NOW, before a later
	// mutation (e.g. a subsequent handleEnrichmentChecked's
	// applyEnrichment/FieldUpdates AmendRows fold for a type already in this
	// snapshot) could change it out from under an already-dispatched save —
	// snapshotRowStoreForSave's SnapshotAll call captures a defensive,
	// by-construction-isolated copy at THIS instant (see SaveCachePayload's
	// doc comment for why dispatch-time capture is required here). This is
	// the Wave-1 sweep-completion save, not the Wave-2-completion save, so
	// wave2Complete=false (C6b): the executor carries forward on-disk Wave-2
	// data these bare rows themselves lack instead of letting them clobber it.
	tasks = append(tasks, TaskRequest{
		Key:     TaskKey{Kind: TaskKindSaveCache},
		Payload: c.snapshotRowStoreForSave(false),
	})

	enrichIntents, enrichTasks := c.startEnrichment()
	intents = append(intents, enrichIntents...)
	tasks = append(tasks, enrichTasks...)

	return intents, tasks
}

// enrichDispatchWindow caps the number of TaskKindProbeEnrich tasks
// startEnrichment dispatches in its initial batch; the remainder stays queued
// in session.EnrichQueue and is drained one type per EnrichmentChecked
// completion (see the refill branch in handleEnrichmentChecked). This is
// deliberately its own constant rather than a reference to
// runtime.MaxConcurrentProbes (core/runtime/related.go) — that constant caps
// the unrelated related-checker fan-out and the two are allowed to diverge.
const enrichDispatchWindow = 4

// startEnrichment builds the enrichment queue and returns the initial batch of
// probe tasks.  Called from handleAvailabilityPrefetched and
// handleAvailabilityChecked once wave-1 resources are retained.
func (c *Core) startEnrichment() ([]UIIntent, []TaskRequest) {
	c.session.EnrichQueue = c.BuildEnrichQueue()
	if len(c.session.EnrichQueue) == 0 {
		return nil, nil
	}
	c.session.EnrichmentGen++
	c.session.EnrichChecked = 0
	c.session.EnrichTotal = len(c.session.EnrichQueue)
	// Rebuilt fresh (not appended) every sweep start: a new sweep must not
	// inherit a prior, still-in-flight sweep's membership/pending state (the
	// per-type EnrichmentGen bump below already makes an old sweep's own
	// completions stale via the TypeGen guard in handleEnrichmentChecked, but
	// this set must independently start clean too — see startEnrichment's
	// package doc for the dispatch table this participates in).
	c.session.EnrichSweepMembers = make(map[string]bool, enrichDispatchWindow)
	c.session.EnrichListOpenPending = make(map[string]bool)

	var intents []UIIntent
	intents = append(intents, PatchMenuEnrichProgress{Checked: 0, Total: c.session.EnrichTotal})

	var tasks []TaskRequest
	for len(c.session.EnrichQueue) > 0 && len(tasks) < enrichDispatchWindow {
		name := c.session.EnrichQueue[0]
		c.session.EnrichQueue = c.session.EnrichQueue[1:]

		// Rerun-start bookkeeping: bump type gen and wipe the stale ran flag
		// so the eventual EnrichmentChecked result's staleness guard and
		// EnrichmentRan bookkeeping are correct for THIS run. Deliberately
		// does NOT strip the type's existing Wave-2 findings here (C1/C6b:
		// stale-until-replaced, not blank-until-replaced — mirrors the fix
		// already applied to internal/tui's Ctrl+R handleRefresh path, see
		// TestRerunStart_KeepsVisibleFindingsUntilReplaced). Merely queuing a
		// fresh enrichment probe must not blank a row's visible glyph before
		// the fresh result actually lands; applyEnrichment (called from
		// handleEnrichmentChecked once the result arrives) already
		// strips-then-reapplies Wave-2 findings from the fresh map, which
		// naturally clears a genuinely-healed row and replaces a still-broken
		// one — no separate eager clear is needed to reach that end state.
		c.session.EnrichmentTypeGenBump(name)
		delete(c.session.EnrichmentRan, name)
		c.session.EnrichSweepMembers[name] = true

		tasks = append(tasks, TaskRequest{Key: TaskKey{Kind: TaskKindProbeEnrich, Scope: name}})
	}
	return intents, tasks
}

// refillEnrichSweep dispatches the next queued type to keep the sweep's
// dispatch window full, returning at most one TaskRequest.
//
// A popped queue entry that already has an outstanding list-open probe
// (session.EnrichListOpenPending, set by HandleResourcesLoaded's list-open
// branch) is silently absorbed into session.EnrichSweepMembers instead of
// being redispatched — that list-open probe's own eventual completion will
// drive this sweep's refill/EnrichChecked bookkeeping in its place, rather
// than this call physically dispatching the SAME resource type a second
// time (#462/#463 defect 2b: a list-open probe racing a queued sweep entry
// must never be dispatched twice). The loop keeps popping past any number
// of such already-covered entries until it finds one to actually dispatch,
// or the queue drains empty.
func (c *Core) refillEnrichSweep() []TaskRequest {
	for len(c.session.EnrichQueue) > 0 {
		next := c.session.EnrichQueue[0]
		c.session.EnrichQueue = c.session.EnrichQueue[1:]

		if c.session.EnrichListOpenPending[next] {
			delete(c.session.EnrichListOpenPending, next)
			c.session.EnrichSweepMembers[next] = true
			continue
		}

		c.session.EnrichmentTypeGenBump(next)
		delete(c.session.EnrichmentRan, next)
		c.session.EnrichSweepMembers[next] = true

		return []TaskRequest{{Key: TaskKey{Kind: TaskKindProbeEnrich, Scope: next}}}
	}
	return nil
}

// removeEnrichQueuedType removes name from the pending sweep queue, preserving
// the remaining order. It is used when an independently dispatched list-open
// enrichment completion has already covered a type that the active sweep had
// not reached yet.
func (c *Core) removeEnrichQueuedType(name string) bool {
	for i, queued := range c.session.EnrichQueue {
		if queued != name {
			continue
		}
		copy(c.session.EnrichQueue[i:], c.session.EnrichQueue[i+1:])
		c.session.EnrichQueue = c.session.EnrichQueue[:len(c.session.EnrichQueue)-1]
		return true
	}
	return false
}

func (c *Core) finishEnrichmentSweepIfDone(intents []UIIntent, tasks []TaskRequest, refillTasks []TaskRequest) ([]UIIntent, []TaskRequest) {
	// All enrichment done — clear progress, save cache. RowStore retains its
	// rows for the session (task #17 wave 1 stage 2 — no enrichment-completion
	// free; see RowStore.Amend's doc comment), so unlike the removed
	// session.ProbeResources/ProbeTruncated free this branch used to perform,
	// there is nothing to nil out here.
	//
	// Gated on len(refillTasks)==0 rather than re-reading EnrichQueue: the
	// slot-freeing step above (refillEnrichSweep) already drained the queue as
	// far as this completion's refill can take it — mirrors the pre-fix "if
	// queue still has entries, fire next and return early; else check all-done"
	// structure, just computed up front so a stale-payload completion can still
	// return its refill task (see the TypeGen guard in handleEnrichmentChecked).
	if len(refillTasks) == 0 && c.session.EnrichChecked >= c.session.EnrichTotal {
		intents = append(intents, PatchMenuEnrichProgress{Checked: 0, Total: 0})
		// Dispatch-time payload freeze (restated on RowStore): SnapshotAll captures a defensive,
		// by-construction-isolated copy (see RowStore.SnapshotAll's doc
		// comment) at THIS dispatch instant, immune to any later Amend the live
		// store still accepts for this type. On the normal completion path this
		// carries the final Wave-2-enriched findings applyEnrichment folded onto
		// retained rows. On a stale progress-only completion, it persists the
		// latest currently retained rows; any newer rerun completion will save
		// again when it lands.
		// wave2Complete=true (C6b): this save IS the fresh enrichment result
		// and must supersede any carried Wave-2 data wholesale so a
		// healed/resolved issue can clear.
		saveSnapshot := c.snapshotRowStoreForSave(true)
		tasks = append(tasks, TaskRequest{
			Key:     TaskKey{Kind: TaskKindSaveCache},
			Payload: saveSnapshot,
		})
	}
	return intents, tasks
}

// handleEnrichmentChecked processes a single Wave-2 enrichment result.
func (c *Core) handleEnrichmentChecked(msg messages.EnrichmentChecked) ([]UIIntent, []TaskRequest) {
	originalType := msg.ResourceType
	if td := resource.FindResourceType(msg.ResourceType); td != nil {
		msg.ResourceType = td.ShortName
	}
	payloadStale := msg.TypeGen != 0 && msg.TypeGen != c.session.EnrichmentTypeGenGet(msg.ResourceType)

	// Sweep-slot freeing is split from payload staleness (#462/#463 defect
	// 1): a sweep member's slot frees up — and the queue refills — the
	// moment ITS completion arrives, even when the TypeGen guard below goes
	// on to discard the payload itself (a rerun bumped the type's gen while
	// its sweep probe was still in flight). Skipping this refill on a stale
	// completion is exactly the bug: every such rerun would strand the rest
	// of EnrichQueue. A non-member completion (e.g. a list-open probe,
	// #462/#463 defect 2) never steals a refill slot; when its own current
	// payload covers a type still sitting in EnrichQueue, it removes exactly
	// that queued entry and advances progress so the later refill cannot
	// dispatch the same type a second time.
	isSweepMember := c.session.EnrichSweepMembers[msg.ResourceType]
	var refillTasks []TaskRequest
	sweepProgressAdvanced := false
	if isSweepMember {
		delete(c.session.EnrichSweepMembers, msg.ResourceType)
		c.session.EnrichChecked++
		sweepProgressAdvanced = true
		refillTasks = c.refillEnrichSweep()
	} else {
		delete(c.session.EnrichListOpenPending, msg.ResourceType)
		if !payloadStale && c.removeEnrichQueuedType(msg.ResourceType) {
			c.session.EnrichChecked++
			sweepProgressAdvanced = true
		}
	}

	// Per-type generation guard — discards a stale PAYLOAD only; the sweep
	// slot/progress above has already been freed (and refilled) regardless of
	// this check. When a stale sweep-member completion is the last outstanding
	// item, still emit progress/all-done bookkeeping so the sweep cannot strand
	// the menu in "Enriching N/N-1".
	if payloadStale {
		var intents []UIIntent
		tasks := refillTasks
		if sweepProgressAdvanced {
			intents = append(intents, PatchMenuEnrichProgress{
				Checked: c.session.EnrichChecked,
				Total:   c.session.EnrichTotal,
			})
			return c.finishEnrichmentSweepIfDone(intents, tasks, refillTasks)
		}
		return nil, tasks
	}

	// #462/#463 defect 1: fold this Wave-2 result onto the type's Wave-1
	// BASELINE (AvailOutcome/AvailDuration/AvailErr), never onto the
	// previous record's aggregate — folding onto the aggregate would
	// re-accumulate Duration and keep a stale partial/Err across repeated
	// enrichment reruns (list-open / Ctrl+R ProbeEnrich without a fresh
	// AvailabilityChecked). Duration is baseline + THIS enrichment probe's
	// wall time only; Err is re-classified only when this probe itself
	// errored, otherwise the baseline's own classification is kept. The
	// baseline fields themselves pass through unchanged, so a later rerun
	// folds from the same Wave-1 observation, not from this rerun's result.
	prevStatus, _ := c.session.GetProbeStatus(msg.ResourceType)
	enrichErrClass := classifyProbeErr(msg.Err)
	aggErr := enrichErrClass
	if aggErr == "" {
		aggErr = prevStatus.AvailErr
	}
	c.setProbeStatus(
		msg.ResourceType,
		degradeForEnrichment(ProbeOutcome(prevStatus.AvailOutcome), msg.Err, msg.Truncated),
		prevStatus.AvailDuration+msg.Duration,
		aggErr,
		time.Now(),
		ProbeOutcome(prevStatus.AvailOutcome), prevStatus.AvailDuration, prevStatus.AvailErr,
	)

	var intents []UIIntent
	tasks := refillTasks

	// Surface enrichment failures as flash.
	if msg.Err != nil {
		intents = append(intents, FlashIntent{
			Text:    fmt.Sprintf("enrich %s: %v", originalType, msg.Err),
			IsError: true,
		})
	}

	// Update findings and menu issue count on success or partial success.
	{
		if c.session.EnrichmentRan == nil {
			c.session.EnrichmentRan = make(map[string]bool)
		}
		c.session.EnrichmentRan[msg.ResourceType] = true

		if c.session.EnrichmentTruncatedIDs == nil {
			c.session.EnrichmentTruncatedIDs = make(map[string]map[string]bool)
		}
		c.session.EnrichmentTruncatedIDs[msg.ResourceType] = msg.TruncatedIDs

		// allFindings is the full per-resource slice this call folds onto rows
		// and counts from — every independently-evaluated Wave-2 condition per
		// resource, keyed by Resource.ID. Nil when the enricher found nothing.
		allFindings := msg.Findings

		// applyEnrichment directly mutates r.Findings and r.AttentionDetails on
		// every cached row of this type, using allFindings — every
		// independently-evaluated Wave-2 condition per resource — so a
		// multi-condition resource keeps every Finding on its cached row.
		c.applyEnrichment(msg.ResourceType, allFindings, msg.AttentionDetails)

		// Merge FieldUpdates into RowStore (task #17 wave 1 stage 3 — the
		// former ResourceCache map leg is gone; a type's rows live in exactly
		// one RowStore entry, so AmendRows' copy-on-write fold below is now
		// this merge's only per-type-row destination).
		if len(msg.FieldUpdates) > 0 {
			c.AmendRows(msg.ResourceType, func(rows []resource.Resource) []resource.Resource {
				out := make([]resource.Resource, len(rows))
				for i, r := range rows {
					updates, ok := msg.FieldUpdates[r.ID]
					if !ok {
						out[i] = r
						continue
					}
					fields := make(map[string]string, len(r.Fields)+len(updates))
					maps.Copy(fields, r.Fields)
					maps.Copy(fields, updates)
					r.Fields = fields
					out[i] = r
				}
				return out
			})
		}

		// td is never nil here in practice: every TaskKindProbeEnrich task is
		// scoped to a ResourceType that already passed a FindResourceType != nil
		// check at dispatch time (HandleResourcesLoaded's list-open branch,
		// startEnrichment's queue, and this handler's own canonicalization
		// above), and unifiedIssueCount needs *td (ExcludeFromIssueBadge,
		// ResolveColor) to compute anything meaningful — there is no
		// findings-only fallback that would produce a comparable count. The nil
		// guard stays only to avoid a *td panic on a malformed/unresolvable
		// message; unified simply stays 0 in that unreached case.
		td := resource.FindResourceType(msg.ResourceType)
		var unified int
		if td != nil {
			// task #17 wave 1 stage 2: unifiedIssueCount's input is now
			// RowStore's own current rows for this type (replaces the removed
			// session.ProbeResources[msg.ResourceType] read) — this is the
			// SAME rows applyEnrichment/AmendRows above just folded findings
			// and FieldUpdates onto, so this aggregation is over the freshest
			// per-type row state this call produced.
			rows, _ := c.ProbeResources(msg.ResourceType)
			unified = unifiedIssueCount(rows, *td, allFindings)
		}

		// Truncation precedence (behavior-preserving with the deleted
		// app_handlers_availability.go:475-478 block):
		//   1. start from Wave-2 truncated signal,
		//   2. clear to false when no issues at all are observed,
		//   3. force true when Wave-1 saw a truncated availability scan —
		//      that lower-bound signal is authoritative even when the visible
		//      subset shows zero issues, so the badge must remain truncated.
		issueTruncated := msg.Truncated
		if unified == 0 && len(allFindings) == 0 {
			issueTruncated = false
		}
		// task #17 wave 1 stage 2: the removed session.ProbeTruncated map's
		// per-type truncation signal is now RowStore's own Pagination for this
		// type, set by this same probe cycle's ObserveRows/AmendRows write.
		if tr := c.session.RowStore.Snapshot(msg.ResourceType); tr.Pagination != nil && tr.Pagination.IsTruncated {
			issueTruncated = true
		}

		// Emit menu issue badge update.
		intents = append(intents, PatchMenu{
			ResourceType: msg.ResourceType,
			Issues:       unified,
			Truncated:    issueTruncated,
		})
		intents = append(intents, PatchMenuEnrichProgress{
			Checked: c.session.EnrichChecked,
			Total:   c.session.EnrichTotal,
		})

		// Emit resource-list enrichment patch (updates list badge + row markers).
		// Findings/AttentionDetails carry every independently-evaluated
		// condition per resource (feeds applyRowFindings/ApplyWave2ToRow so a
		// multi-condition resource's row fold keeps every finding's own
		// AttentionDetail).
		enrichPatch := &ListEnrichmentPatch{
			Findings:         allFindings,
			AttentionDetails: msg.AttentionDetails,
			TruncatedIDs:     msg.TruncatedIDs,
		}
		if len(msg.FieldUpdates) > 0 {
			enrichPatch.FieldUpdates = msg.FieldUpdates
		}
		intents = append(intents, PatchResourceList{
			ResourceType: msg.ResourceType,
			Issues:       &IssueBadgePatch{Count: unified, Truncated: issueTruncated},
			Enrichment:   enrichPatch,
		})

		// Emit detail-view patch for any open detail views of this type.
		// ResourceID empty = all detail views of this type; the adapter looks up
		// the findings for each view's specific resource ID from
		// EnrichmentFindings. allFindings carries every independently-evaluated
		// Wave-2 condition per resource, so an already-open detail's Attention
		// block shows every condition, not just the worst-severity one.
		intents = append(intents, PatchDetail{
			ResourceType:               msg.ResourceType,
			EnrichmentFindings:         allFindings,
			EnrichmentAttentionDetails: msg.AttentionDetails,
		})
	}

	return c.finishEnrichmentSweepIfDone(intents, tasks, refillTasks)
}

// snapshotRowStoreForSave builds a TaskKindSaveCache payload from RowStore's
// current retained rows (task #17 wave 1 stage 2 — replaces the deleted
// snapshotProbeResourcesForSave, whose source was session.ProbeResources/
// ProbeTruncated). SnapshotAll(false) excludes Partial-only entries (C6
// scope: a sparse LazyResourceCache-role read must never poison a type's
// canonical persisted list) — mirrors snapshotProbeResourcesForSave's own
// scope, since the legacy session.ProbeResources map never carried Partial
// rows either.
//
// SnapshotAll's own defensive-copy guarantee (fresh slice + fresh Fields map
// per row, see RowStore.Snapshot's doc comment) is what gives this payload
// the dispatch-time-freeze property the deleted function's
// element-by-element copy used to provide by hand.
//
// wave2Complete is stamped onto the returned payload's Wave2Complete field
// as-is (C6b) — see SaveCachePayload's doc comment for what it drives at
// execute time.
func (c *Core) snapshotRowStoreForSave(wave2Complete bool) *SaveCachePayload {
	resources, truncated := c.rowStoreResourcesAndTruncated()
	if len(resources) == 0 {
		return nil
	}
	return &SaveCachePayload{Resources: resources, Truncated: truncated, Wave2Complete: wave2Complete}
}

// rowStoreResourcesAndTruncated converts RowStore.SnapshotAll(false) (task
// #17 wave 1 stage 2) into the (resources, truncated) map pair
// snapshotRowStoreForSave and the TaskKindSaveCache nil-Payload executor
// fallback both need — the shape TaskKindSaveCache's payload/live-read
// carried since before the row-store unification (SaveCachePayload.Resources/
// Truncated). Excludes Partial-only entries (SnapshotAll(false) already
// drops those) and every Rows-empty entry EXCEPT a live, exact rows-carrying
// observation of a genuine zero population (Origin Probe or Fetch, exact
// pagination) — that IS the type's population going to zero this session,
// not the absence of an observation, and must still reach the save. A
// counts-only ObserveCount write (C6a: count known, rows never observed)
// stays excluded even though it may leave Origin at its OriginDisk zero
// value or carry forward a prior rows-carrying Origin untouched —
// discriminated on Pagination (ObserveCount never sets it) together with
// Origin, not Origin alone, since TypeRows' zero-value Origin IS OriginDisk.
// A disk-seeded entry (Origin OriginDisk) stays excluded the same way it
// always has.
//
// A nil tr.Pagination reports truncated=true (C5: nil pagination is never
// exact — mirrors availabilityFromResourceCache's identical guard), never
// truncated=false. A bare Pagination-present check here previously let a
// nil-Pagination entry (the exact shape HandleResourcesLoaded's
// PatchResourceCache/SetResourceCache carries before any real page-boundary
// observation) read as exact, so a downstream Exact-count derivation could
// silently downgrade an already-deeper stored-exact total (the false-exact
// shape D14 describes).
func (c *Core) rowStoreResourcesAndTruncated() (map[string][]resource.Resource, map[string]bool) {
	all := c.session.RowStore.SnapshotAll(false)
	resources := make(map[string][]resource.Resource, len(all))
	truncated := make(map[string]bool, len(all))
	for shortName, tr := range all {
		if len(tr.Rows) == 0 {
			liveExactZeroPopulation := (tr.Origin == session.OriginProbe || tr.Origin == session.OriginFetch) &&
				tr.Pagination != nil && !tr.Pagination.IsTruncated
			if !liveExactZeroPopulation {
				continue
			}
		}
		resources[shortName] = tr.Rows
		// C5: nil Pagination means this entry's truncation state was never
		// observed — treat as unknown/conservatively truncated, mirroring
		// availabilityFromResourceCache's identical guard. A bare `!= nil &&
		// .IsTruncated` here would let an unobserved (nil-Pagination) entry
		// read as exact, letting a downstream Exact-count derivation silently
		// downgrade a genuinely deeper stored-exact total (D14).
		truncated[shortName] = tr.Pagination == nil || tr.Pagination.IsTruncated
	}
	return resources, truncated
}

// rowsFromCacheRows converts a per-type file's persisted Rows (ID/Name/Fields
// + Findings, C6: no RawStruct, no color/glyph/status) into the
// resource.Resource shape session.ProbeResources carries. Colors/glyphs/
// status are intentionally NOT reconstructed here — buildListBody derives
// them at render time from Fields + Findings via the same classification
// rules live data uses (C6).
//
// row.Fields/row.Findings are copied rather than assigned by reference:
// (*cache.Store).Type returns a TypeFile by value, but its Rows slice and
// each Row's Fields map / Findings slice still alias the Store's own backing
// data. Downstream mutators (ApplyWave2ToRow, the FieldUpdates maps.Copy in
// handleEnrichmentChecked) write into the seeded ProbeResources rows this
// function produces — an aliased Fields/Findings would let those writes
// corrupt the in-memory Store the next save reads from.
func rowsFromCacheRows(shortName string, rows []cache.Row) []resource.Resource {
	out := make([]resource.Resource, len(rows))
	for i, row := range rows {
		var fields map[string]string
		if row.Fields != nil {
			fields = make(map[string]string, len(row.Fields))
			maps.Copy(fields, row.Fields)
		}
		out[i] = resource.Resource{
			ID:       row.ID,
			Name:     row.Name,
			Type:     shortName,
			Fields:   fields,
			Findings: append([]domain.Finding(nil), row.Findings...),
		}
	}
	return out
}

// unifiedIssueCount returns the distinct count of resource IDs with ≥1
// SevBroken-equivalent issue: a Wave-1 structural/wave1-Finding color of
// ColorBroken/ColorWarning (IsIssue()), or ANY Wave-2 SevBroken finding in
// findings[id]'s slice — a resource counts once even when it carries more
// than one independently-evaluated Wave-2 condition. Never a SevWarn Wave-2
// finding — those are excluded from both halves of this single per-resource
// decision.
func unifiedIssueCount(wave1Resources []resource.Resource, td resource.ResourceTypeDef, findings map[string][]domain.Finding) int {
	if td.ExcludeFromIssueBadge {
		return 0
	}
	// Count per resource, not per ID: two DISTINCT resources that share an ID
	// (e.g. two ACM certs for one domain — ACM keys on the domain name) must
	// each bump the badge, matching the list title (listIssueCount counts per
	// row). A resource issue-colored by Wave-1 is counted and `continue`d so a
	// Wave-2 finding on the same resource never double-counts it.
	count := 0
	for _, r := range wave1Resources {
		if td.ResolveColor(Wave1Only(r)).IsIssue() {
			count++
			continue
		}
		// A Wave-2 "!"-severity finding bumps the badge; "~" (SevWarn) never does.
		for _, finding := range findings[r.ID] {
			if finding.Severity == domain.SevBroken {
				count++
				break
			}
		}
	}
	return count
}

// Wave1Only returns a copy of r with every Wave-2-sourced Finding
// (Finding.IsWave2Sourced()) stripped, so a Color func reading
// r.Findings (colorFromAnyFinding and friends) sees only wave1-sourced
// signal — never a merged Wave-2 SevWarn/SevBroken finding. Wave-2's own
// contribution to the badge is folded in separately from the live findings
// map, which is why this copy must not leak wave2 Findings into ResolveColor.
//
// Exported because both issue-count surfaces — the menu badge (unifiedIssueCount)
// and the list title (Controller.listIssueCount) — must strip Wave-2 the same
// way per docs/attention-signals.md S1 ("~ findings do not bump"), so the two
// counts cannot drift.
func Wave1Only(r resource.Resource) resource.Resource {
	if len(r.Findings) == 0 {
		return r
	}
	kept := make([]domain.Finding, 0, len(r.Findings))
	for _, f := range r.Findings {
		if !f.IsWave2Sourced() {
			kept = append(kept, f)
		}
	}
	r.Findings = kept
	return r
}
