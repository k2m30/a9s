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
			Origin:       "cache",
		})
	}

	// Contract D / C1: seed session.ProbeResources with the disk-cached rows
	// for every known type so a cold list-open renders real cells instantly
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
	if len(entries) > 0 {
		store := c.EnsureCacheStore()
		if c.session.ProbeResources == nil {
			c.session.ProbeResources = make(map[string][]resource.Resource, len(entries))
		}
		if c.session.ProbeTruncated == nil {
			c.session.ProbeTruncated = make(map[string]bool, len(entries))
		}
		for shortName, count := range entries {
			if count <= 0 {
				continue
			}
			if _, already := c.session.ProbeResources[shortName]; already {
				continue
			}
			var rows []resource.Resource
			if store != nil {
				if tf, ok := store.Type(shortName); ok && len(tf.Rows) > 0 {
					rows = rowsFromCacheRows(shortName, tf.Rows)
				}
			}
			if len(rows) == 0 {
				rows = placeholderRows(shortName, count)
			}
			c.session.ProbeResources[shortName] = rows
			c.session.ProbeTruncated[shortName] = truncated[shortName]
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

	// Fire first batch of concurrent probes (up to 4).
	var tasks []TaskRequest
	for i := 0; i < 4 && len(c.session.AvailQueue) > 0; i++ {
		shortName := c.session.AvailQueue[0]
		c.session.AvailQueue = c.session.AvailQueue[1:]
		tasks = append(tasks, TaskRequest{Key: TaskKey{Kind: TaskKindProbeAvailability, Scope: shortName}})
	}

	return intents, tasks
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
			Origin: "verified",
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

	// T034: retain prefetch resources for Wave-2 enrichment.
	if msg.Resources != nil {
		if c.session.ProbeResources == nil {
			c.session.ProbeResources = make(map[string][]resource.Resource, len(msg.Resources))
		}
		// Fetcher-emitted rows already carry Findings; no re-derive needed
		// (W1.4b.3 dropped the legacy Status/Issues bridge).
		maps.Copy(c.session.ProbeResources, msg.Resources)

		if c.session.ProbeTruncated == nil {
			c.session.ProbeTruncated = make(map[string]bool, len(msg.Truncated))
		}
		maps.Copy(c.session.ProbeTruncated, msg.Truncated)

		if c.session.ResourceCache == nil {
			c.session.ResourceCache = make(map[string]*session.ResourceCacheEntry, len(msg.Resources))
		}
		for rt, resources := range msg.Resources {
			if _, exists := c.session.ResourceCache[rt]; exists {
				continue
			}
			pageMeta := msg.Pagination[rt]
			if pageMeta == nil {
				pageMeta = &resource.PaginationMeta{IsTruncated: msg.Truncated[rt]}
			}
			c.session.ResourceCache[rt] = &session.ResourceCacheEntry{
				Resources:  resources,
				Pagination: pageMeta,
			}
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
			Origin: "verified",
		})
		// T032: wire issue counts from probe.
		intents = append(intents, PatchMenu{
			ResourceType: msg.ResourceType,
			Issues:       msg.Issues,
			Truncated:    msg.Truncated,
		})

		// T032: retain probe resources for Wave-2 enrichment.
		if c.session.ProbeResources == nil {
			c.session.ProbeResources = make(map[string][]resource.Resource)
		}
		canonType := msg.ResourceType
		if td := resource.FindResourceType(msg.ResourceType); td != nil {
			canonType = td.ShortName
		}
		// Fetcher-emitted rows already carry Findings; no re-derive needed
		// (W1.4b.3 dropped the legacy Status/Issues bridge).
		c.session.ProbeResources[canonType] = msg.Resources
		if c.session.ProbeTruncated == nil {
			c.session.ProbeTruncated = make(map[string]bool)
		}
		c.session.ProbeTruncated[canonType] = msg.Truncated
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

	// DEF-7: snapshot ProbeResources/ProbeTruncated NOW, before startEnrichment
	// (below) can mutate them via clearEnrichmentFor's clear-on-rerun-start
	// step — see SaveCachePayload's doc comment for why dispatch-time capture
	// is required here.
	tasks = append(tasks, TaskRequest{
		Key:     TaskKey{Kind: TaskKindSaveCache},
		Payload: c.snapshotProbeResourcesForSave(),
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

		// Clear-on-rerun-start: bump type gen, wipe stale ran flag, strip wave-2.
		c.session.EnrichmentTypeGen[name]++
		delete(c.session.EnrichmentRan, name)
		c.clearEnrichmentFor(name)

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

		// applyEnrichment directly mutates r.Findings and r.AttentionDetails on
		// every cached row of this type.
		c.applyEnrichment(msg.ResourceType, msg.Findings, msg.AttentionDetails)

		// Merge FieldUpdates into ProbeResources and ResourceCache.
		if len(msg.FieldUpdates) > 0 {
			if c.session.ProbeResources == nil {
				c.session.ProbeResources = make(map[string][]resource.Resource)
			}
			slice := c.session.ProbeResources[msg.ResourceType]
			for i := range slice {
				if updates, ok := msg.FieldUpdates[slice[i].ID]; ok {
					if slice[i].Fields == nil {
						slice[i].Fields = make(map[string]string, len(updates))
					}
					maps.Copy(slice[i].Fields, updates)
				}
			}
			c.session.ProbeResources[msg.ResourceType] = slice

			if entry, ok := c.session.ResourceCache[msg.ResourceType]; ok {
				for i := range entry.Resources {
					if updates, ok := msg.FieldUpdates[entry.Resources[i].ID]; ok {
						if entry.Resources[i].Fields == nil {
							entry.Resources[i].Fields = make(map[string]string, len(updates))
						}
						maps.Copy(entry.Resources[i].Fields, updates)
					}
				}
			}
		}

		td := resource.FindResourceType(msg.ResourceType)
		var unified int
		if td != nil {
			unified = unifiedIssueCount(c.session.ProbeResources[msg.ResourceType], *td, msg.Findings)
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
		if unified == 0 && len(msg.Findings) == 0 {
			issueTruncated = false
		}
		if c.session.ProbeTruncated[msg.ResourceType] {
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
		enrichPatch := &ListEnrichmentPatch{
			Findings:         msg.Findings,
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
		// the finding for each view's specific resource ID from EnrichmentFindings.
		intents = append(intents, PatchDetail{
			ResourceType:               msg.ResourceType,
			EnrichmentFindings:         msg.Findings,
			EnrichmentAttentionDetails: msg.AttentionDetails,
		})
	}

	// Fire next from queue.
	if len(c.session.EnrichQueue) > 0 {
		next := c.session.EnrichQueue[0]
		c.session.EnrichQueue = c.session.EnrichQueue[1:]

		c.session.EnrichmentTypeGen[next]++
		delete(c.session.EnrichmentRan, next)
		c.clearEnrichmentFor(next)

		tasks = append(tasks, TaskRequest{Key: TaskKey{Kind: TaskKindProbeEnrich, Scope: next}})
		return intents, tasks
	}

	// All enrichment done — clear progress, free retained resources, save cache.
	if c.session.EnrichChecked >= c.session.EnrichTotal {
		intents = append(intents, PatchMenuEnrichProgress{Checked: 0, Total: 0})
		// DEF-7: snapshot BEFORE freeing ProbeResources/ProbeTruncated below —
		// this is the completion path that carries the FINAL Wave-2-enriched
		// findings (applyEnrichment above already mutated r.Findings on every
		// cached row of this type), so it must not be lost to the free.
		saveSnapshot := c.snapshotProbeResourcesForSave()
		c.session.ProbeResources = nil
		c.session.ProbeTruncated = nil
		tasks = append(tasks, TaskRequest{
			Key:     TaskKey{Kind: TaskKindSaveCache},
			Payload: saveSnapshot,
		})
	}

	return intents, tasks
}

// snapshotProbeResourcesForSave captures a deep-enough copy of
// c.session.ProbeResources/ProbeTruncated for a TaskKindSaveCache dispatch —
// DEF-7. Must be called BEFORE any subsequent same-call mutation of those
// maps (clearEnrichmentFor's clear-on-rerun-start step, or the
// free-on-enrichment-complete nil-out) so the eventual save sees the rows as
// they stood at dispatch time, not as they stand whenever the task executes.
//
// A shallow map copy is NOT sufficient here: clearEnrichmentFor mutates each
// resource.Resource IN PLACE (via ApplyWave2ToRow on &rows[i]) on the SAME
// backing array a shallow []resource.Resource slice copy would still alias —
// a plain maps.Copy of the outer map would still observe that later
// in-place strip. Each per-type slice (and each resource's Findings slice,
// the field clearEnrichmentFor mutates) is copied element-by-element so the
// snapshot is fully isolated from any later mutation of the live session
// state.
func (c *Core) snapshotProbeResourcesForSave() *SaveCachePayload {
	if len(c.session.ProbeResources) == 0 {
		return nil
	}
	resources := make(map[string][]resource.Resource, len(c.session.ProbeResources))
	for shortName, rows := range c.session.ProbeResources {
		cp := make([]resource.Resource, len(rows))
		for i, r := range rows {
			r.Findings = append([]domain.Finding(nil), r.Findings...)
			cp[i] = r
		}
		resources[shortName] = cp
	}
	truncated := make(map[string]bool, len(c.session.ProbeTruncated))
	maps.Copy(truncated, c.session.ProbeTruncated)
	return &SaveCachePayload{Resources: resources, Truncated: truncated}
}

// rowsFromCacheRows converts a per-type file's persisted Rows (ID/Name/Fields
// + Findings, C6: no RawStruct, no color/glyph/status) into the
// resource.Resource shape session.ProbeResources carries. Colors/glyphs/
// status are intentionally NOT reconstructed here — buildListBody derives
// them at render time from Fields + Findings via the same classification
// rules live data uses (C6).
func rowsFromCacheRows(shortName string, rows []cache.Row) []resource.Resource {
	out := make([]resource.Resource, len(rows))
	for i, row := range rows {
		out[i] = resource.Resource{
			ID:       row.ID,
			Name:     row.Name,
			Type:     shortName,
			Fields:   row.Fields,
			Findings: row.Findings,
		}
	}
	return out
}

// placeholderRows synthesizes count ID-only resource.Resource rows for a type
// whose cached count is known but whose per-row disk data is unavailable
// (e.g. a counts-only cache projection). Placeholder IDs are never shown to
// the operator as real identifiers by themselves — the immediate live fetch
// this seeding always accompanies (Refreshing=true) replaces them.
func placeholderRows(shortName string, count int) []resource.Resource {
	out := make([]resource.Resource, count)
	for i := range out {
		out[i] = resource.Resource{
			ID:   fmt.Sprintf("%s-cached-%d", shortName, i),
			Type: shortName,
		}
	}
	return out
}

// unifiedIssueCount returns the distinct count of resource IDs with ≥1 issue
// across both Wave-1 (IsIssue() status color) and Wave-2 (enrichment findings).
// Only SevBroken findings contribute to the S1 badge ("!"-glyph equivalent;
// SevWarn / "~" informational findings are excluded).
func unifiedIssueCount(wave1Resources []resource.Resource, td resource.ResourceTypeDef, findings map[string]domain.Finding) int {
	if td.ExcludeFromIssueBadge {
		return 0
	}
	knownIDs := make(map[string]struct{}, len(wave1Resources))
	for _, r := range wave1Resources {
		knownIDs[r.ID] = struct{}{}
	}
	ids := make(map[string]struct{})
	for _, r := range wave1Resources {
		if td.ResolveColor(r).IsIssue() {
			ids[r.ID] = struct{}{}
		}
	}
	for id, finding := range findings {
		if finding.Severity != domain.SevBroken {
			continue
		}
		if _, ok := knownIDs[id]; ok {
			ids[id] = struct{}{}
		}
	}
	return len(ids)
}
