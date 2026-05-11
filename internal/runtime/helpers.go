package runtime

// helpers.go — session-state helpers on Core used by the per-handler PRs.
//
// These methods operate only on c.session fields and platform-agnostic packages
// (resource, semantics/attention, aws/IssueEnricherRegistry).  They are the
// runtime equivalents of the same-named methods that still exist on tui.Model
// for non-migrated callers; both sets operate on the same *session.Session so
// mutations are visible to both.

import (
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/semantics/attention"
)

// applyEnrichment merges Wave-2 enrichment findings into every cached row of
// the given resource type by calling DeriveFindings with the provided findings
// map. Replaces prior wave-2 findings in place while preserving wave-1.  Walks
// ResourceCache, LazyResourceCache, and ProbeResources.
func (c *Core) applyEnrichment(resourceType string, findings map[string]resource.EnrichmentFinding) {
	canon := resourceType
	var td resource.ResourceTypeDef
	if t := resource.FindResourceType(resourceType); t != nil {
		canon = t.ShortName
		td = *t
	} else {
		td = resource.ResourceTypeDef{ShortName: canon}
	}

	apply := func(rows []resource.Resource) {
		for i := range rows {
			attention.DeriveFindings(&rows[i], td, findings)
		}
	}

	if entry, ok := c.session.ResourceCache[canon]; ok {
		apply(entry.Resources)
	}
	if rows, ok := c.session.LazyResourceCache[canon]; ok {
		apply(rows)
	}
	if rows, ok := c.session.ProbeResources[canon]; ok {
		apply(rows)
	}
}

// clearEnrichmentFor strips wave-2 findings from every cached row of the given
// resource type by re-deriving with nil enrichment.  Wave-1 findings are
// preserved.  Used by clear-on-rerun-start logic in handleEnrichmentChecked.
func (c *Core) clearEnrichmentFor(resourceType string) {
	canon := resourceType
	var td resource.ResourceTypeDef
	if t := resource.FindResourceType(resourceType); t != nil {
		canon = t.ShortName
		td = *t
	} else {
		td = resource.ResourceTypeDef{ShortName: canon}
	}

	clear := func(rows []resource.Resource) {
		for i := range rows {
			attention.DeriveFindings(&rows[i], td, nil)
		}
	}

	if entry, ok := c.session.ResourceCache[canon]; ok {
		clear(entry.Resources)
	}
	if rows, ok := c.session.LazyResourceCache[canon]; ok {
		clear(rows)
	}
	if rows, ok := c.session.ProbeResources[canon]; ok {
		clear(rows)
	}
}

// deriveFindingsForType re-derives wave-1 findings across rows in-place,
// preserving any wave-2 entries already present.
func (c *Core) deriveFindingsForType(short string, rows []resource.Resource) {
	if len(rows) == 0 {
		return
	}
	var td resource.ResourceTypeDef
	if t := resource.FindResourceType(short); t != nil {
		td = *t
	} else {
		td = resource.ResourceTypeDef{ShortName: short}
	}
	for i := range rows {
		attention.DeriveWave1Only(&rows[i], td)
	}
}

