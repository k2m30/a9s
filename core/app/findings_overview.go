// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package app

import (
	"sort"
	"time"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// FindingsGroup is one FindingCode's aggregated view across every resource
// type that emitted it in the current session — the per-rule row the a9s-app
// desktop triage cockpit renders.
type FindingsGroup struct {
	Code      domain.FindingCode
	Phrase    string
	Severity  domain.Severity
	Count     int
	Types     []string
	SampleIDs []string
	Truncated bool

	// OldestFirstSeen is the earliest FirstSeen recorded (#463) across every
	// (type, row, code) pair contributing to this group, read from the
	// on-disk availability cache (Core.FindingFirstSeenForType). Zero when
	// unknown — no contributing pair has a cache entry yet (cold start,
	// NoCache/demo, or a pair whose cache predates #463 and hasn't been
	// re-saved since).
	OldestFirstSeen time.Time
	// NewSincePrev is the sum, across this group's contributing types, of
	// that type's most recent on-disk-save new-finding-pair count for this
	// Code (Core.NewFindingPairsSincePrev) — the (row, code) pairs that
	// appeared for the first time on the type's last cache save. Zero when
	// no contributing type has saved this session, or caching is disabled.
	NewSincePrev int
}

// FindingsTotals summarizes FindingsOverview's session-wide counters
// alongside the per-group breakdown.
type FindingsTotals struct {
	Open                  int
	Errors                int
	Warnings              int
	OpenGroups            int
	TypesScanned          int
	TypesTotal            int
	TypesWithoutRules     int
	Resources             int
	ResourcesWithFindings int
	Truncated             bool
}

// findingsGroupAccum is the mutable in-progress state for one FindingCode
// while findingsOverview walks every resource type's findings. seen dedupes
// by "type\x00id" so a resource already counted for this code — once via its
// Wave-1 row's folded Finding (Source "wave2:<short>"), once via the raw
// enrichmentStore entry that produced it — contributes to Count exactly once.
type findingsGroupAccum struct {
	types           map[string]bool
	seen            map[string]bool
	sampleLabels    []string
	count           int
	worstSeverity   domain.Severity
	worstPhrase     string
	truncated       bool
	oldestFirstSeen time.Time
}

// FindingsOverview aggregates every qualifying Finding (Severity >=
// domain.SevWarn) across every resource type into per-FindingCode groups
// plus session-wide totals — the cross-type rule breakdown neither the
// per-type resource list nor the main-menu badges provide on their own
// (issue #461).
func (c *Controller) FindingsOverview() ([]FindingsGroup, FindingsTotals) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.findingsOverview()
}

// findingsOverview is the lock-free implementation of FindingsOverview.
// Callers must hold c.mu (at least read).
//
// Wave-1 source: ForEachResourceCache, not ForEachProbeResources — the
// latter excludes OriginFetch rows (a resource list the user currently has
// open), which would silently drop that type's findings from the cockpit
// while it's on screen. ForEachResourceCache also includes OriginDisk-seeded
// types the availability sweep hasn't reconfirmed yet, matching what the
// menu badges already show at cold start.
//
// Wave-2 source: c.enrichmentStore, the carry that survives a Wave-1
// refetch without flicker. A Wave-1 row may already carry a folded copy of
// the same finding; the per-(type,id) dedupe in findingsGroupAccum.seen
// absorbs that overlap without double-counting.
func (c *Controller) findingsOverview() ([]FindingsGroup, FindingsTotals) {
	groups := make(map[domain.FindingCode]*findingsGroupAccum)
	resourceWorst := make(map[string]domain.Severity)
	resourceLabel := make(map[string]map[string]string)

	var totals FindingsTotals

	// firstSeenByType lazily loads Core.FindingFirstSeenForType(rt) at most
	// once per canonical type this pass touches (#463) — contribute() is
	// called once per (type, resource, finding), so caching here is what
	// keeps this the single aggregation pass rather than adding a second
	// per-row walk.
	firstSeenByType := make(map[string]map[string]map[domain.FindingCode]time.Time)
	firstSeenFor := func(rt, id string, code domain.FindingCode) (time.Time, bool) {
		byRow, ok := firstSeenByType[rt]
		if !ok {
			byRow = c.core.FindingFirstSeenForType(rt)
			firstSeenByType[rt] = byRow
		}
		codes, ok := byRow[id]
		if !ok {
			return time.Time{}, false
		}
		t, ok := codes[code]
		return t, ok
	}

	contribute := func(rt, id, label string, f domain.Finding, typeTruncated bool) {
		if f.Severity < domain.SevWarn {
			return
		}
		// Wave-1 rows are written under the canonical ShortName
		// (handleAvailabilityChecked's canonType, core/runtime/
		// handlers_availability.go:322-340), but Controller.enrichmentStore
		// is keyed by ApplyEnrichmentState's raw caller-supplied typeName —
		// which may be an alias (e.g. "rds" for "dbi") — since
		// applyEnrichmentState never canonicalizes the store key itself
		// (list_filter.go:318, only its menu-badge sync path canonicalizes).
		// Normalize here, the single place both passes' contributions meet,
		// so the same resource under two different type strings doesn't
		// double-count.
		rt = c.canonicalResourceType(rt)
		resKey := rt + "\x00" + id
		if cur, ok := resourceWorst[resKey]; !ok || f.Severity > cur {
			resourceWorst[resKey] = f.Severity
		}

		acc, ok := groups[f.Code]
		if !ok {
			acc = &findingsGroupAccum{types: make(map[string]bool), seen: make(map[string]bool)}
			groups[f.Code] = acc
		}
		acc.types[rt] = true
		if typeTruncated {
			acc.truncated = true
		}
		if t, ok := firstSeenFor(rt, id, f.Code); ok {
			if acc.oldestFirstSeen.IsZero() || t.Before(acc.oldestFirstSeen) {
				acc.oldestFirstSeen = t
			}
		}
		if f.Severity > acc.worstSeverity {
			acc.worstSeverity = f.Severity
			acc.worstPhrase = f.Phrase
		}
		if !acc.seen[resKey] {
			acc.seen[resKey] = true
			acc.count++
			if label == "" {
				label = id
			}
			acc.sampleLabels = append(acc.sampleLabels, label)
		}
	}

	c.core.ForEachResourceCache(func(rt string, entry *domain.ListViewCacheEntry) {
		// Mirrors rowStoreResourcesAndTruncated's C5 rule (core/runtime/
		// handlers_availability.go): a nil Pagination is never exact.
		typeTruncated := entry.Pagination == nil || entry.Pagination.IsTruncated
		canonRt := c.canonicalResourceType(rt)
		labels := resourceLabel[canonRt]
		if labels == nil {
			labels = make(map[string]string, len(entry.Resources))
			resourceLabel[canonRt] = labels
		}
		for _, r := range entry.Resources {
			label := r.Name
			if label == "" {
				label = r.ID
			}
			labels[r.ID] = label
			for _, f := range r.Findings {
				contribute(rt, r.ID, label, f, typeTruncated)
			}
		}
	})

	// enrichmentStore/enrichmentTruncated/EnrichmentTruncatedIDs are all
	// keyed by ApplyEnrichmentState's raw typeName (list_filter.go:318-320),
	// so the truncation lookup below stays on the raw rt — only the label
	// lookup needs the canonical key, to match Wave-1's resourceLabel map.
	for rt, byID := range c.enrichmentStore {
		typeTruncated := c.enrichmentTruncated[rt] || len(c.core.EnrichmentTruncatedIDs(rt)) > 0
		labels := resourceLabel[c.canonicalResourceType(rt)]
		for id, findings := range byID {
			label := ""
			if labels != nil {
				label = labels[id]
			}
			for _, f := range findings {
				contribute(rt, id, label, f, typeTruncated)
			}
		}
	}

	for _, worst := range resourceWorst {
		totals.Open++
		totals.ResourcesWithFindings++
		if worst == domain.SevBroken {
			totals.Errors++
		} else {
			totals.Warnings++
		}
	}

	// TypesScanned/TypesTotal/Resources mirror buildMenuBody's own entry
	// walk (menuAllItems + menuActiveKey) rather than re-deriving type
	// visibility — the costs pseudo-entry is spliced in there for
	// cursor/filter purposes only and never carries availability data.
	ms := c.rootMenuState()
	for _, item := range menuAllItems() {
		if item.ShortName == CostsMenuShortName {
			continue
		}
		totals.TypesTotal++
		if len(item.Findings) == 0 && !c.core.HasIssueEnricher(item.ShortName) {
			totals.TypesWithoutRules++
		}
		if ms == nil {
			continue
		}
		key := menuActiveKey(ms, item)
		if avail, known := ms.Availability[key]; known {
			totals.TypesScanned++
			totals.Resources += avail
		}
		if ms.Truncated != nil && ms.Truncated[key] {
			// A truncated availability count makes Resources a floor, not
			// an exact total — the cockpit's totals inherit that caveat.
			totals.Truncated = true
		}
	}

	// newPairsByType is fetched once (not per group) — NewSincePrev only ever
	// sums pre-recorded per-type-per-code counts, never re-walks rows, so a
	// single fetch here keeps this the aggregation pass's single lookup
	// rather than one per group.
	newPairsByType := c.core.NewFindingPairsSincePrev()

	result := make([]FindingsGroup, 0, len(groups))
	for code, acc := range groups {
		types := make([]string, 0, len(acc.types))
		for t := range acc.types {
			types = append(types, t)
		}
		sort.Strings(types)

		newSincePrev := 0
		for _, t := range types {
			newSincePrev += newPairsByType[t][code]
		}

		phrase := acc.worstPhrase
		for _, t := range types {
			td := c.findingsTypeDef(t)
			if td == nil {
				continue
			}
			found := false
			for _, fd := range td.Findings {
				if fd.Code == code {
					phrase = fd.Phrase
					found = true
					break
				}
			}
			if found {
				break
			}
		}

		sort.Strings(acc.sampleLabels)
		samples := acc.sampleLabels
		if len(samples) > 5 {
			samples = samples[:5]
		}

		if acc.truncated {
			totals.Truncated = true
		}

		result = append(result, FindingsGroup{
			Code:            code,
			Phrase:          phrase,
			Severity:        acc.worstSeverity,
			Count:           acc.count,
			Types:           types,
			SampleIDs:       samples,
			Truncated:       acc.truncated,
			OldestFirstSeen: acc.oldestFirstSeen,
			NewSincePrev:    newSincePrev,
		})
	}
	totals.OpenGroups = len(result)

	sort.Slice(result, func(i, j int) bool {
		if result[i].Severity != result[j].Severity {
			return result[i].Severity > result[j].Severity
		}
		if result[i].Count != result[j].Count {
			return result[i].Count > result[j].Count
		}
		return result[i].Code < result[j].Code
	})

	return result, totals
}

// canonicalResourceType resolves rt to the catalog's canonical ShortName,
// matching handleAvailabilityChecked's own canonType resolution
// (core/runtime/handlers_availability.go:322-324) — the single normalization
// point findingsOverview uses to reconcile Wave-1's canonically-keyed
// RowStore rows with Wave-2's alias-keyed enrichmentStore before deciding
// per-resource identity. rt with no catalog match (e.g. a test's ad hoc
// type) passes through unchanged.
func (c *Controller) canonicalResourceType(rt string) string {
	if td := resource.FindResourceType(rt); td != nil {
		return td.ShortName
	}
	return rt
}

// findingsTypeDef resolves a resource type's ResourceTypeDef the same way
// every other Controller read of c.fallbackTypeDefs does (e.g.
// materializeListFieldsForType in list_body.go): a registered fallback
// (test typeDefs, web child-view typeDefs) wins over the catalog lookup, so
// a type that only exists as a fallback still resolves its Findings table.
func (c *Controller) findingsTypeDef(shortName string) *resource.ResourceTypeDef {
	if ftd, ok := c.fallbackTypeDefs[shortName]; ok {
		v := ftd
		return &v
	}
	return resource.FindResourceType(shortName)
}
