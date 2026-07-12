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

	"github.com/k2m30/a9s/v3/internal/cache"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/session"
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
	// DEF-6/C3: Origin="cache" — seeded from disk, not yet re-verified this
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

	// Contract D / C1: seed RowStore with the disk-cached rows for every
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

	// Build queue of all resource types to check in background.
	allNames := resource.AllShortNames()
	c.session.AvailQueue = allNames
	c.session.AvailChecked = 0
	c.session.AvailTotal = len(allNames)

	intents = append(intents, PatchMenuCheckProgress{Checked: 0, Total: c.session.AvailTotal})

	// The disk-cache load races ahead of the AWS connect on startup by
	// design (Init fires both concurrently via tea.Batch for instant-paint,
	// C1) — it routinely wins, since local disk I/O finishes long before a
	// network connect settles. Dispatching probe tasks while Clients is
	// still nil would run every one of them against a nil transport and
	// fail hard ("AWS clients not initialized"), permanently losing that
	// probe for the session instead of actually checking availability (the
	// four resource types first in resource.AllShortNames() were observed
	// failing this way). Latch AvailSweepPending instead; the next
	// successful HandleClientsReady drains the first batch once a real
	// transport exists.
	var tasks []TaskRequest
	if c.session.Clients != nil {
		tasks = append(tasks, c.fireNextAvailabilityProbes(4)...)
	} else {
		c.session.AvailSweepPending = true
	}

	// DEF-14/D11: consume the one-shot -c navigation armed by
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
			// DEF-6/C3: a prefetch is a synchronous LIVE count (demo /
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

	return intents, enrichTasks
}

// handleAvailabilityChecked processes a single resource type's probe result.
func (c *Core) handleAvailabilityChecked(msg messages.AvailabilityChecked) ([]UIIntent, []TaskRequest) {
	c.session.AvailChecked++

	var intents []UIIntent
	var tasks []TaskRequest

	// Update menu availability on full success or partial-success.
	if msg.Err == nil || len(msg.Resources) > 0 {
		intents = append(intents, PatchMenuAvailability{
			ResourceType: msg.ResourceType,
			Count:        msg.Count,
			Truncated:    msg.Truncated,
			// DEF-6/C3: a live AvailabilityChecked result confirms this type
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

	// Surface partial-success failures as flash errors.
	if msg.Err != nil {
		intents = append(intents, FlashIntent{
			Text:    fmt.Sprintf("availability %s: %v", msg.ResourceType, msg.Err),
			IsError: true,
		})
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

	// All checks done — clear progress indicator and save cache.
	intents = append(intents, PatchMenuCheckProgress{Checked: 0, Total: 0}) // 0,0 = done
	intents = append(intents, ClearFlash{})

	// DEF-7 (restated on RowStore): snapshot RowStore NOW, before a later
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

	var intents []UIIntent
	intents = append(intents, PatchMenuEnrichProgress{Checked: 0, Total: c.session.EnrichTotal})

	var tasks []TaskRequest
	for len(c.session.EnrichQueue) > 0 {
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
		c.session.EnrichmentTypeGen[name]++
		delete(c.session.EnrichmentRan, name)

		tasks = append(tasks, TaskRequest{Key: TaskKey{Kind: TaskKindProbeEnrich, Scope: name}})
	}
	return intents, tasks
}

// handleEnrichmentChecked processes a single Wave-2 enrichment result.
func (c *Core) handleEnrichmentChecked(msg messages.EnrichmentChecked) ([]UIIntent, []TaskRequest) {
	originalType := msg.ResourceType
	if td := resource.FindResourceType(msg.ResourceType); td != nil {
		msg.ResourceType = td.ShortName
	}

	// Per-type generation guard.
	if msg.TypeGen != 0 && msg.TypeGen != c.session.EnrichmentTypeGen[msg.ResourceType] {
		return nil, nil
	}

	c.session.EnrichChecked++

	var intents []UIIntent
	var tasks []TaskRequest

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
		} else {
			unified = msg.Issues
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

	// Fire next from queue. Mirrors startEnrichment's rerun-start bookkeeping
	// (C1/C6b: stale-until-replaced — no eager clearEnrichmentFor here either,
	// see startEnrichment's doc comment for why).
	if len(c.session.EnrichQueue) > 0 {
		next := c.session.EnrichQueue[0]
		c.session.EnrichQueue = c.session.EnrichQueue[1:]

		c.session.EnrichmentTypeGen[next]++
		delete(c.session.EnrichmentRan, next)

		tasks = append(tasks, TaskRequest{Key: TaskKey{Kind: TaskKindProbeEnrich, Scope: next}})
		return intents, tasks
	}

	// All enrichment done — clear progress, save cache. RowStore retains its
	// rows for the session (task #17 wave 1 stage 2 — no
	// enrichment-completion free; see RowStore.Amend's doc comment), so
	// unlike the removed session.ProbeResources/ProbeTruncated free this
	// branch used to perform, there is nothing to nil out here.
	if c.session.EnrichChecked >= c.session.EnrichTotal {
		intents = append(intents, PatchMenuEnrichProgress{Checked: 0, Total: 0})
		// DEF-7 (restated on RowStore): SnapshotAll captures a defensive,
		// by-construction-isolated copy (see RowStore.SnapshotAll's doc
		// comment) at THIS dispatch instant, immune to any later Amend the
		// live store still accepts for this type. This is the completion path
		// that carries the FINAL Wave-2-enriched findings (applyEnrichment
		// above already folded r.Findings/r.AttentionDetails onto every
		// retained row of this type via AmendRows), so it must not be lost to
		// a later mutation of the live store.
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
// the DEF-7 dispatch-time-freeze property the deleted function's
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
// Truncated). Excludes Partial-only entries and any Rows-empty entry (C6a:
// a counts-only ObserveCount write must never surface here as a
// zero-resource, zero-value-Truncated row-carrying entry).
//
// A nil tr.Pagination reports truncated=true (C5: nil pagination is never
// exact — mirrors availabilityFromResourceCache's identical guard), never
// truncated=false. A bare Pagination-present check here previously let a
// nil-Pagination entry (the exact shape HandleResourcesLoaded's
// PatchResourceCache/SetResourceCache carries before any real page-boundary
// observation) read as exact, so a downstream Exact-count derivation could
// silently downgrade an already-deeper stored-exact total (DEF-18).
func (c *Core) rowStoreResourcesAndTruncated() (map[string][]resource.Resource, map[string]bool) {
	all := c.session.RowStore.SnapshotAll(false)
	resources := make(map[string][]resource.Resource, len(all))
	truncated := make(map[string]bool, len(all))
	for shortName, tr := range all {
		if len(tr.Rows) == 0 {
			continue
		}
		resources[shortName] = tr.Rows
		// C5: nil Pagination means this entry's truncation state was never
		// observed — treat as unknown/conservatively truncated, mirroring
		// availabilityFromResourceCache's identical guard. A bare `!= nil &&
		// .IsTruncated` here would let an unobserved (nil-Pagination) entry
		// read as exact, letting a downstream Exact-count derivation silently
		// downgrade a genuinely deeper stored-exact total (DEF-18).
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
