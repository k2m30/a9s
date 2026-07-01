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
	for shortName, count := range entries {
		intents = append(intents, PatchMenuAvailability{
			ResourceType: shortName,
			Count:        count,
			Truncated:    truncated[shortName],
		})
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

	tasks = append(tasks, TaskRequest{Key: TaskKey{Kind: TaskKindSaveCache}})

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
		c.session.ProbeResources = nil
		c.session.ProbeTruncated = nil
		tasks = append(tasks, TaskRequest{Key: TaskKey{Kind: TaskKindSaveCache}})
	}

	return intents, tasks
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
