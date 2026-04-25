package tui

// app_handlers_availability.go — main-menu availability + Wave-2 enrichment
// message handlers. Split from app_handlers_navigate.go to keep both files
// under the 500-line budget.
//
// Each handler here drives a stage of the menu-discovery pipeline whose probe
// helpers live in app_probes.go:
//
//   handleAvailabilityCacheLoaded → seeds menu from disk, fires probes.
//   handleAvailabilityPrefetched  → applies sync prefetch in demo / no-cache mode.
//   handleAvailabilityChecked     → applies one probe result, fires next.
//   startEnrichment               → builds Wave-2 queue, dispatches probes.
//   handleEnrichmentChecked       → applies one Wave-2 result, fires next.
//   unifiedIssueCount             → cross-wave de-duped issue count for the S1 badge.

import (
	"fmt"
	"maps"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// handleAvailabilityCacheLoaded applies cached entries to the main menu
// and starts background availability checks.
func (m Model) handleAvailabilityCacheLoaded(msg messages.AvailabilityCacheLoadedMsg) (tea.Model, tea.Cmd) {
	// Canonicalize any alias keys (e.g. "rds" → "dbi") so the menu's filter and
	// issue maps share the same key space as the ResourceTypeDef lookup.
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

	// Apply cached entries to the main menu
	if menu, ok := m.stack[0].(*views.MainMenuModel); ok {
		for shortName, count := range entries {
			menu.SetAvailability(shortName, count)
		}
		for shortName, trunc := range truncated {
			menu.SetTruncated(shortName, trunc)
		}
		// Apply cached issue counts (T033).
		if len(issueKnown) > 0 {
			menu.SetIssuesFromCache(issueCounts, issueTruncated, issueKnown)
		}
	}

	// Build queue of all resource types to check in background
	allNames := resource.AllShortNames()
	m.availQueue = allNames
	m.availChecked = 0
	m.availTotal = len(allNames)

	// Update menu progress
	if menu, ok := m.stack[0].(*views.MainMenuModel); ok {
		menu.SetCheckProgress(0, m.availTotal)
	}

	// Fire first batch of concurrent probes (up to 4)
	var cmds []tea.Cmd
	for i := 0; i < 4 && len(m.availQueue) > 0; i++ {
		shortName := m.availQueue[0]
		m.availQueue = m.availQueue[1:]
		cmds = append(cmds, m.probeResourceAvailability(shortName, m.availabilityGen))
	}

	return m, tea.Batch(cmds...)
}

// handleAvailabilityPrefetched applies synchronously-prefetched counts to the
// main menu. Used in no-cache + pre-supplied-clients mode so counts appear
// immediately without background probes.
func (m Model) handleAvailabilityPrefetched(msg messages.AvailabilityPrefetchedMsg) (tea.Model, tea.Cmd) {
	// Gen guard: drop stale results produced before a profile/region switch.
	// Gen=0 is the zero value (pre-guard dispatch) — accepted unconditionally
	// to preserve backwards-compatible test injection.
	if msg.Gen != 0 && msg.Gen != m.availabilityGen {
		return m, nil
	}
	if menu, ok := m.stack[0].(*views.MainMenuModel); ok {
		for shortName, count := range msg.Entries {
			menu.SetAvailability(shortName, count)
		}
		for shortName, trunc := range msg.Truncated {
			menu.SetTruncated(shortName, trunc)
		}
		// T034: wire issue counts from prefetch.
		for shortName, count := range msg.IssueCounts {
			trunc := msg.IssueTruncated[shortName]
			menu.SetIssues(shortName, count, trunc)
		}
		menu.SetCheckProgress(0, 0) // signal "done"
	}
	// T034: retain prefetch resources for Wave 2 enrichment (--no-cache live AWS).
	if msg.Resources != nil {
		if m.probeResources == nil {
			m.probeResources = make(map[string][]resource.Resource, len(msg.Resources))
		}
		maps.Copy(m.probeResources, msg.Resources)
		// Seed resourceCache from the prefetch too. Without this, Wave 2
		// FieldUpdates that the enricher merges into the cache entry
		// (handleEnrichmentChecked tail) have no entry to land on when the
		// user hasn't opened the list yet — the enrichment runs before any
		// OpenList, so FieldUpdates would otherwise die in probeResources
		// and vanish on the first fetch. Seeding here keeps the cache
		// entry alive so FieldUpdates survive across navigation.
		if m.resourceCache == nil {
			m.resourceCache = make(map[string]*resourceCacheEntry, len(msg.Resources))
		}
		for rt, resources := range msg.Resources {
			if _, exists := m.resourceCache[rt]; exists {
				continue
			}
			m.resourceCache[rt] = &resourceCacheEntry{
				resources: resources,
			}
		}
	}
	// Start Wave 2 enrichment. Demo mode's typed fakes implement the enricher
	// APIs (DescribePendingMaintenanceActions, etc.), so enrichment runs the
	// same production code path against fixture data — this is what gives the
	// demo its `~` glyphs, `(+N)` suffix, and "maintenance scheduled" status.
	enrichCmd := m.startEnrichment()

	// Surface aggregated per-type prefetch failures to the error log so
	// operators see permission / throttle issues instead of silently missing
	// resource types in the availability counts.
	var flashCmd tea.Cmd
	if msg.PrefetchErr != nil {
		err := msg.PrefetchErr
		flashCmd = func() tea.Msg {
			return messages.FlashMsg{
				Text:    "availability: " + err.Error(),
				IsError: true,
			}
		}
	}

	if enrichCmd != nil && flashCmd != nil {
		return m, tea.Batch(flashCmd, enrichCmd)
	}
	if enrichCmd != nil {
		return m, enrichCmd
	}
	return m, flashCmd
}

// handleAvailabilityChecked processes a single resource type's probe result.
func (m Model) handleAvailabilityChecked(msg messages.AvailabilityCheckedMsg) (tea.Model, tea.Cmd) {
	// Ignore stale results from a previous generation (profile/region switch)
	if msg.Gen != m.availabilityGen {
		return m, nil
	}

	m.availChecked++

	// Update menu availability on full success (Err==nil) or partial-success
	// (Err!=nil but Resources non-empty). Pure failures (Err!=nil, Resources
	// empty) leave the menu entry as "unknown" — don't grey out.
	if msg.Err == nil || len(msg.Resources) > 0 {
		if menu, ok := m.stack[0].(*views.MainMenuModel); ok {
			menu.SetAvailability(msg.ResourceType, msg.Count)
			menu.SetTruncated(msg.ResourceType, msg.Truncated)
			// T032: wire issue counts from probe.
			menu.SetIssues(msg.ResourceType, msg.Issues, msg.Truncated)
		}
		// T032: retain probe resources for Wave 2 enrichment.
		if m.probeResources == nil {
			m.probeResources = make(map[string][]resource.Resource)
		}
		m.probeResources[msg.ResourceType] = msg.Resources
	}

	// Surface partial-success failures as flash errors so operators see them.
	var flashCmd tea.Cmd
	if msg.Err != nil {
		err := msg.Err
		rt := msg.ResourceType
		flashCmd = func() tea.Msg {
			return messages.FlashMsg{
				Text:    fmt.Sprintf("availability %s: %v", rt, err),
				IsError: true,
			}
		}
	}

	// Update progress on menu
	if menu, ok := m.stack[0].(*views.MainMenuModel); ok {
		menu.SetCheckProgress(m.availChecked, m.availTotal)
	}

	// If queue has more items, fire next probe
	if len(m.availQueue) > 0 {
		next := m.availQueue[0]
		m.availQueue = m.availQueue[1:]
		cmd := m.probeResourceAvailability(next, m.availabilityGen)
		return m, tea.Batch(flashCmd, cmd)
	}

	// Queue is drained but other probes may still be in flight.
	// Only finalize when ALL probes have returned.
	if m.availChecked < m.availTotal {
		return m, flashCmd
	}

	// All checks done — clear progress indicator, flash, and save cache
	if menu, ok := m.stack[0].(*views.MainMenuModel); ok {
		menu.SetCheckProgress(0, 0) // 0,0 signals "done"
	}
	m.flash.active = false

	// Save cache to disk
	saveCmd := m.saveAvailabilityCache()

	// Start Wave 2 enrichment. Demo mode's typed fakes implement the enricher
	// APIs, so enrichment runs identically against fixture data.
	enrichCmd := m.startEnrichment()
	return m, tea.Batch(flashCmd, saveCmd, enrichCmd)
}

// startEnrichment builds the enrichment queue and fires the first batch of probes.
// For each type dispatched, it bumps enrichmentTypeGen, clears any existing
// findings and ran flag (clear-on-rerun-start), then captures the new gen into
// the probeEnrichment call.
func (m *Model) startEnrichment() tea.Cmd {
	m.enrichQueue = m.buildEnrichQueue()
	if len(m.enrichQueue) == 0 {
		return nil
	}
	m.enrichmentGen++
	m.enrichChecked = 0
	m.enrichTotal = len(m.enrichQueue)

	if menu, ok := m.stack[0].(*views.MainMenuModel); ok {
		menu.SetEnrichProgress(0, m.enrichTotal)
	}

	// Dispatch all enrichers at once so priority ordering is observable in a
	// single cmd tree. Each probe is independent; results arrive as individual
	// EnrichmentCheckedMsg values. handleEnrichmentChecked drains any residual
	// queue entries added by future requeue operations.
	var cmds []tea.Cmd
	for len(m.enrichQueue) > 0 {
		name := m.enrichQueue[0]
		m.enrichQueue = m.enrichQueue[1:]
		// Clear-on-rerun-start: bump type gen, wipe stale findings and ran flag.
		m.enrichmentTypeGen[name]++
		delete(m.enrichmentFindings, name)
		delete(m.enrichmentRan, name)
		cmds = append(cmds, m.probeEnrichment(name, m.enrichmentGen))
	}
	return tea.Batch(cmds...)
}

// handleEnrichmentChecked processes a single Wave 2 enrichment result.
func (m Model) handleEnrichmentChecked(msg messages.EnrichmentCheckedMsg) (tea.Model, tea.Cmd) {
	// Capture original type name before normalization so flash messages retain
	// the alias used by the caller (e.g. "rds" stays "rds" in the flash text).
	originalType := msg.ResourceType
	// Normalize to the canonical ShortName so alias-keyed messages (e.g. "rds" for
	// the "dbi" type) match the ResourceType() returned by views in the stack.
	if td := resource.FindResourceType(msg.ResourceType); td != nil {
		msg.ResourceType = td.ShortName
	}
	// Session-wide generation guard — drop stale messages from prior profile/region.
	// Gen=0 is the documented test-injection bypass: accepted regardless of enrichmentGen.
	if msg.Gen != 0 && msg.Gen != m.enrichmentGen {
		return m, nil
	}
	// Per-type generation guard — drop stale probes superseded by a newer rerun.
	// TypeGen=0 is the symmetric test-injection bypass (production always dispatches
	// with TypeGen≥1 because startEnrichment bumps enrichmentTypeGen[name] before
	// capturing typeGen in probeEnrichment).
	if msg.TypeGen != 0 && msg.TypeGen != m.enrichmentTypeGen[msg.ResourceType] {
		return m, nil
	}

	m.enrichChecked++

	// Surface enrichment failures as a flash error so operators see them in the
	// error log (! key). A failed enrichment does not stall the pipeline — the
	// queue continues to drain below.
	var flashCmd tea.Cmd
	if msg.Err != nil {
		err := msg.Err
		rt := originalType
		flashCmd = func() tea.Msg {
			return messages.FlashMsg{
				Text:    fmt.Sprintf("enrich %s: %v", rt, err),
				IsError: true,
			}
		}
	}

	// Update findings and menu issue count on success or partial success.
	// Looping over empty maps is a no-op, so this block is safe to run even
	// when Err != nil — the flash above already records the failure.
	{
		// Persist findings and mark enrichment as ran for this type.
		// Guard against nil maps: these are initialized in handleSessionStart
		// but may be nil in early or test-injected model states.
		if m.enrichmentFindings == nil {
			m.enrichmentFindings = make(map[string]map[string]resource.EnrichmentFinding)
		}
		m.enrichmentFindings[msg.ResourceType] = msg.Findings
		if m.enrichmentRan == nil {
			m.enrichmentRan = make(map[string]bool)
		}
		m.enrichmentRan[msg.ResourceType] = true
		// Always replace, including with empty/nil maps — a successful rerun
		// MUST clear prior "?" row markers. Using `if len > 0` would leave
		// stale markers from a previous attempt.
		if m.enrichmentTruncatedIDs == nil {
			m.enrichmentTruncatedIDs = make(map[string]map[string]bool)
		}
		m.enrichmentTruncatedIDs[msg.ResourceType] = msg.TruncatedIDs

		// Merge FieldUpdates into probeResources so the cached rows carry
		// Wave-2-derived fields. These are then visible to list columns that
		// reference the updated keys.
		if len(msg.FieldUpdates) > 0 {
			if m.probeResources == nil {
				m.probeResources = make(map[string][]resource.Resource)
			}
			slice := m.probeResources[msg.ResourceType]
			for i := range slice {
				if updates, ok := msg.FieldUpdates[slice[i].ID]; ok {
					if slice[i].Fields == nil {
						slice[i].Fields = make(map[string]string, len(updates))
					}
					maps.Copy(slice[i].Fields, updates)
				}
			}
			m.probeResources[msg.ResourceType] = slice
			// Persist FieldUpdates into resourceCache so that navigating away
			// and back restores the Wave-2-derived fields (e.g. last_build,
			// dlq, rotation_enabled) instead of rendering them blank.
			if entry, ok := m.resourceCache[msg.ResourceType]; ok {
				for i := range entry.resources {
					if updates, ok := msg.FieldUpdates[entry.resources[i].ID]; ok {
						if entry.resources[i].Fields == nil {
							entry.resources[i].Fields = make(map[string]string, len(updates))
						}
						maps.Copy(entry.resources[i].Fields, updates)
					}
				}
			}
			// Also propagate into any active ResourceListModel for this type.
			for _, v := range m.stack {
				if rl, ok := v.(*views.ResourceListModel); ok && rl.ResourceType() == msg.ResourceType {
					rl.ApplyFieldUpdates(msg.FieldUpdates)
				}
			}
		}

		// Merge IssueAppends into probeResources and resourceCache.
		// IssueAppends carries Wave-1 cross-ref phrases (e.g. "orphan: source DB deleted")
		// that could not be computed at fetch time because they require sibling cache access.
		// These are appended to Resource.Issues after fetcher-set phrases.
		if len(msg.IssueAppends) > 0 {
			if m.probeResources == nil {
				m.probeResources = make(map[string][]resource.Resource)
			}
			slice := m.probeResources[msg.ResourceType]
			for i := range slice {
				if phrases, ok := msg.IssueAppends[slice[i].ID]; ok {
					slice[i].Issues = append(slice[i].Issues, phrases...)
				}
			}
			m.probeResources[msg.ResourceType] = slice
			if entry, ok := m.resourceCache[msg.ResourceType]; ok {
				for i := range entry.resources {
					if phrases, ok := msg.IssueAppends[entry.resources[i].ID]; ok {
						entry.resources[i].Issues = append(entry.resources[i].Issues, phrases...)
					}
				}
			}
		}

		if menu, ok := m.stack[0].(*views.MainMenuModel); ok {
			// Wave 2 is authoritative: compute distinct-instance count across both waves.
			td := resource.FindResourceType(msg.ResourceType)
			var unified int
			if td != nil {
				unified = unifiedIssueCount(m.probeResources[msg.ResourceType], *td, msg.Findings)
			} else {
				unified = msg.Issues
			}
			// Wave 2 truncation with no findings and no wave-1 issues means a
			// sub-call errored but no actual issue was seen. Truncation signals
			// count completeness, not hidden issues — if Wave 2 had seen one, it
			// would have produced a Finding. Don't promote into the attention filter.
			issueTruncated := msg.Truncated
			if unified == 0 && len(msg.Findings) == 0 {
				issueTruncated = false
			}
			// If resource count is already a lower bound (Wave 1 truncated), the
			// issue count is also a lower bound — preserve that signal even when
			// Wave 2 itself did not truncate.
			if menu.GetTruncated()[msg.ResourceType] {
				issueTruncated = true
			}
			menu.SetIssues(msg.ResourceType, unified, issueTruncated)
			menu.SetEnrichProgress(m.enrichChecked, m.enrichTotal)

			// Live-update ALL ResourceListModel views in the stack showing this type.
			for _, v := range m.stack {
				if rl, ok := v.(*views.ResourceListModel); ok && rl.ResourceType() == msg.ResourceType {
					rl.SetEnrichmentState(unified, issueTruncated, msg.Findings)
					rl.SetTruncatedIDs(msg.TruncatedIDs)
				}
			}
		}

		// Live-update ALL DetailModel views in the stack for this resource type.
		// Iterating the full stack ensures stacked (non-active) detail views are also
		// updated when the user has navigated to a second detail view or another screen
		// and enrichment completes while that secondary view is active.
		for _, v := range m.stack {
			if d, ok := v.(*views.DetailModel); ok && d.ResourceType() == msg.ResourceType {
				if f, exists := msg.Findings[d.ResourceID()]; exists {
					d.SetEnrichmentFinding(&f)
				} else {
					d.SetEnrichmentFinding(nil)
				}
			}
		}
	} // end anonymous block (partial-success safe: empty maps are no-ops)

	// Fire next from queue — bump per-type gen before each dispatch.
	if len(m.enrichQueue) > 0 {
		next := m.enrichQueue[0]
		m.enrichQueue = m.enrichQueue[1:]
		// Clear-on-rerun-start for the next type.
		m.enrichmentTypeGen[next]++
		delete(m.enrichmentFindings, next)
		delete(m.enrichmentRan, next)
		cmd := m.probeEnrichment(next, m.enrichmentGen)
		return m, tea.Batch(flashCmd, cmd)
	}

	// All enrichment done — clear progress, free retained resources, save cache
	if m.enrichChecked >= m.enrichTotal {
		if menu, ok := m.stack[0].(*views.MainMenuModel); ok {
			menu.SetEnrichProgress(0, 0)
		}
		m.probeResources = nil
		// Save cache with enrichment-updated issue counts.
		cmd := m.saveAvailabilityCache()
		return m, tea.Batch(flashCmd, cmd)
	}
	return m, flashCmd
}

// unifiedIssueCount returns the distinct count of resource IDs with ≥1 issue
// across both Wave-1 (IsIssue() status color) and Wave-2 (enrichment findings).
// Two findings on the same instance count as one.
//
// Only `!`-severity findings contribute to the S1 badge. `~`-severity findings
// are informational and must not bump the count.
//
// Invariant: result ≤ len(wave1Resources). Findings keyed by IDs not present
// in wave1Resources are skipped (orphans) — they would otherwise inflate the
// badge above the visible row count, e.g. an enricher dispatched for cluster
// type writing instance-keyed findings.
func unifiedIssueCount(wave1Resources []resource.Resource, td resource.ResourceTypeDef, findings map[string]resource.EnrichmentFinding) int {
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
		if finding.Severity != "!" {
			continue
		}
		if _, ok := knownIDs[id]; ok {
			ids[id] = struct{}{}
		}
	}
	return len(ids)
}
