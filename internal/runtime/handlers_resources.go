// handlers_resources.go — PR-05a-h4-b (AS-962) Update()-switch extraction.
//
// Ports five inline-mutation branches off internal/tui/app.go's Update()
// switch into platform-agnostic (c *Core) Handle* methods:
//
//	HandleResourcesLoaded     — ResourceCache write-through + Wave-2 probe
//	                            re-dispatch on enrichment-rerun token match.
//	HandleEnrichDetailResult  — detail-view enrichment error surface.
//	HandleRelatedCheckResult  — RelatedCache append; CachedPages and
//	                            LazyAddedResources merge; flash on errors.
//	HandleIdentityLoaded      — session.Identity write + domain-mirror emit
//	                            (renderer reads the mirror, not awsclient).
//	HandleIdentityError       — fetch-flag clear; adapter handles view note.
//
// Plus two utility methods used by the adapter after h4-b:
//
//	(*Core).AllRegions        — call-through to awsclient.AllRegions so the
//	                            adapter can drop its internal/aws import in h4-c.
//	(*Core).ResetRuleSets     — session.RuleSets swap + Clients rewire (used
//	                            by SES refresh paths in handleRefresh).
//
// Companion file: handlers.go owns the h3 + h4-a ports. Splitting the
// five h4-b methods into a sibling file mirrors the existing
// handlers_availability.go / handlers_navigate.go / handlers_related.go
// organisation: each shell-level concern lives in its own file so a Core
// handler grep stays narrow.
package runtime

import (
	"fmt"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/session"
)

// ResourcesLoadedEvent is the adapter-translated form of
// messages.ResourcesLoaded. Adapters compute no view-state-dependent
// fields when populating it — every field maps 1:1 to the source message
// — so the Core handler is a pure function of session + event.
type ResourcesLoadedEvent struct {
	ResourceType string
	Resources    []resource.Resource
	Pagination   *resource.PaginationMeta
	Append       bool
	TypeGen      domain.Gen
	Err          error
}

// HandleResourcesLoaded owns the session-state portion of the post-fetch
// processing for a top-level resource list. It does NOT touch the view
// stack — the adapter still routes the message through updateActiveView
// for view-side processing (ResourceListModel.Update absorbs Resources
// and writes the rich entry back via cacheTopLevelResourceList).
//
// What this method does:
//
//   - Emits ClearFlash so any active "Refreshing..." flash dismisses.
//   - When the loaded type is not yet cached (and the message is not an
//     Append page), emits PatchResourceCache so cross-view navigation
//     (e.g. related-navigate to a not-yet-visited type) finds an entry.
//     The !alreadyCached guard mirrors the original case body — the
//     view-side cacheTopLevelResourceList write wins when both run.
//   - On a paginated partial-success (Err non-nil with Resources present)
//     emits a FlashIntent so the `!` log records the failure.
//   - On enrichment-rerun match (TypeGen non-zero AND matches the per-type
//     gen captured at Ctrl+R dispatch), seeds Session.ProbeResources +
//     ProbeTruncated and emits a TaskKindProbeEnrich task. Stale rerun
//     tokens are silently dropped.
func (c *Core) HandleResourcesLoaded(ev ResourcesLoadedEvent) ([]UIIntent, []TaskRequest) {
	intents := []UIIntent{ClearFlash{}}

	if ev.ResourceType != "" && !ev.Append {
		if _, alreadyCached := c.session.ResourceCache[ev.ResourceType]; !alreadyCached {
			intents = append(intents, PatchResourceCache{
				ResourceType: ev.ResourceType,
				Entry: &session.ResourceCacheEntry{
					Resources: ev.Resources,
				},
			})
		}
	}

	if ev.Err != nil {
		intents = append(intents, FlashIntent{
			Text:    "fetch " + ev.ResourceType + ": " + ev.Err.Error(),
			IsError: true,
		})
	}

	var tasks []TaskRequest
	if ev.TypeGen != 0 && ev.TypeGen == c.session.EnrichmentTypeGen[ev.ResourceType] {
		if c.session.ProbeResources == nil {
			c.session.ProbeResources = make(map[string][]resource.Resource)
		}
		c.session.ProbeResources[ev.ResourceType] = ev.Resources
		if c.session.ProbeTruncated == nil {
			c.session.ProbeTruncated = make(map[string]bool)
		}
		c.session.ProbeTruncated[ev.ResourceType] = ev.Pagination != nil && ev.Pagination.IsTruncated
		tasks = append(tasks, TaskRequest{
			Key: TaskKey{Kind: TaskKindProbeEnrich, Scope: ev.ResourceType},
		})
	}

	return intents, tasks
}

// EnrichDetailResultEvent is the adapter-translated form of
// messages.EnrichDetailResult restricted to the fields Core inspects.
// EnrichedRes and ResourceID stay adapter-side because the renderer
// passes the result through updateActiveView for the detail view; the
// Core handler only decides whether to emit an error flash.
type EnrichDetailResultEvent struct {
	ResourceType string
	Err          error
}

// HandleEnrichDetailResult emits a single FlashIntent on enrichment
// failure. The success path is a no-op from Core's perspective — the
// adapter shim derives wave-1 findings and routes the enriched resource
// through updateActiveView so the detail view rebuilds its field list.
func (c *Core) HandleEnrichDetailResult(ev EnrichDetailResultEvent) ([]UIIntent, []TaskRequest) {
	if ev.Err != nil {
		return []UIIntent{FlashIntent{
			Text:    "enrich failed: " + ev.Err.Error(),
			IsError: true,
		}}, nil
	}
	return nil, nil
}

// RelatedCheckResultEvent is the adapter-translated form of
// messages.RelatedCheckResult. SourceResourceID is the canonical source
// resource ID the adapter resolved from messages.SourceResourceID with a
// fallback to the active detail view's resource. The fallback resolution
// is adapter-side because the active view is renderer state; the Core
// handler treats SourceResourceID as authoritative.
type RelatedCheckResultEvent struct {
	ResourceType       string
	SourceResourceID   string
	DefDisplayName     string
	Result             resource.RelatedCheckResult
	CachedPages        map[string]resource.ResourceCacheEntry
	LazyAddedResources map[string][]resource.Resource
	LazyAddError       error
}

// HandleRelatedCheckResult owns the session-state decisions for an
// async related-check result. The actual session writes are emitted as
// intents (PatchRelatedCache, PatchResourceCache, PatchLazyResourceCache)
// so applyIntents is the single locus of session-cache mutation; this
// keeps the handler-result graph diff-able and shapes h4-c's eventual
// "Session() accessor goes away" move into one place. The adapter shim
// still routes the message through updateActiveView after applying the
// returned intents so the detail view's right-column model receives
// the result.
//
// Field canonicalisation: CachedPages and LazyAddedResources keys may be
// aliases (e.g. "rds" instead of canonical "dbi"). Core resolves each
// key via resource.FindResourceType so ResourceCache and LazyResourceCache
// are always keyed by the canonical ShortName — matching the read paths
// that look up by ShortName.
func (c *Core) HandleRelatedCheckResult(ev RelatedCheckResultEvent) ([]UIIntent, []TaskRequest) {
	var intents []UIIntent

	if ev.SourceResourceID != "" {
		intents = append(intents, PatchRelatedCache{
			ResourceType:   ev.ResourceType,
			SourceID:       ev.SourceResourceID,
			DefDisplayName: ev.DefDisplayName,
			Result:         ev.Result,
		})
	}

	// CachedPages: insert each first-page result verbatim if neither
	// ResourceCache nor LazyResourceCache already holds an entry for the
	// canonical type. The skip-when-present guards prevent CachedPages
	// from evicting richer entries (top-level fetch result, prior
	// lazy-add merge). Decision is in Core; the write is done by the
	// adapter via PatchResourceCache. addedInBatch preserves
	// first-write-wins semantics when two aliases in CachedPages
	// canonicalise to the same ShortName.
	addedInBatch := map[string]struct{}{}
	for aliasName, entry := range ev.CachedPages {
		shortName := canonShortName(aliasName)
		if _, dup := addedInBatch[shortName]; dup {
			continue
		}
		if _, exists := c.session.ResourceCache[shortName]; exists {
			continue
		}
		if _, lazyExists := c.session.LazyResourceCache[shortName]; lazyExists {
			continue
		}
		c.deriveFindingsForType(shortName, entry.Resources)
		pagination := entry.Pagination
		if pagination == nil && entry.IsTruncated {
			pagination = &resource.PaginationMeta{IsTruncated: true}
		}
		intents = append(intents, PatchResourceCache{
			ResourceType: shortName,
			Entry: &session.ResourceCacheEntry{
				Resources:  entry.Resources,
				Pagination: pagination,
			},
		})
		addedInBatch[shortName] = struct{}{}
	}

	// LazyAddedResources: append-dedup merge into LazyResourceCache.
	// Core computes the merged slices and emits a single
	// PatchLazyResourceCache carrying the full Adds map.
	var lazyAdds map[string][]resource.Resource
	for aliasName, extra := range ev.LazyAddedResources {
		if len(extra) == 0 {
			continue
		}
		shortName := canonShortName(aliasName)
		c.deriveFindingsForType(shortName, extra)
		existing := c.session.LazyResourceCache[shortName]
		known := make(map[string]struct{}, len(existing))
		for _, r := range existing {
			known[r.ID] = struct{}{}
		}
		merged := existing
		for _, r := range extra {
			if _, dup := known[r.ID]; dup {
				continue
			}
			known[r.ID] = struct{}{}
			merged = append(merged, r)
		}
		if lazyAdds == nil {
			lazyAdds = make(map[string][]resource.Resource)
		}
		lazyAdds[shortName] = merged
	}
	if lazyAdds != nil {
		intents = append(intents, PatchLazyResourceCache{Adds: lazyAdds})
	}

	if ev.LazyAddError != nil {
		intents = append(intents, FlashIntent{
			Text:    fmt.Sprintf("related-fetch: %v", ev.LazyAddError),
			IsError: true,
		})
	}
	if ev.Result.Err != nil {
		intents = append(intents, FlashIntent{
			Text:    fmt.Sprintf("related %s: %v", ev.Result.TargetType, ev.Result.Err),
			IsError: true,
		})
	}
	return intents, nil
}

// IdentityLoadedEvent is the adapter-translated form of
// messages.IdentityLoaded. Identity is the awsclient-typed pointer
// (carried as `any` on the message itself to keep the messages package
// renderer-neutral). Core type-asserts it and converts to the domain
// mirror before emitting SetIdentityIntent.
type IdentityLoadedEvent struct {
	Identity any
}

// HandleIdentityLoaded writes the resolved caller identity into session
// state, clears IdentityFetching, and emits SetIdentityIntent paired with
// HeaderInvalidateIntent so the next View() picks up the new badge / role.
// A nil or wrong-typed Identity clears IdentityFetching but emits no
// further intents — the adapter sees the same observable result as the
// pre-h4-b inline assertion (which silently dropped non-matching types).
func (c *Core) HandleIdentityLoaded(ev IdentityLoadedEvent) ([]UIIntent, []TaskRequest) {
	c.session.IdentityFetching = false
	awsID, ok := ev.Identity.(*awsclient.CallerIdentity)
	if !ok || awsID == nil {
		return nil, nil
	}
	c.session.Identity = awsID
	return []UIIntent{
		SetIdentityIntent{Identity: domainCallerIdentityFrom(awsID)},
		HeaderInvalidateIntent{},
	}, nil
}

// IdentityErrorEvent is the adapter-translated form of
// messages.IdentityError. Err is the renderer-visible message string
// shown on the IdentityModel; HandleIdentityError clears
// IdentityFetching so the header drops the spinner and lets the
// adapter render the error directly on the view.
type IdentityErrorEvent struct {
	Err string
}

// HandleIdentityError clears IdentityFetching. The adapter shim
// additionally calls IdentityModel.SetError when an identity view is
// active — that branch needs renderer-side view-stack inspection so it
// stays out of Core.
func (c *Core) HandleIdentityError(ev IdentityErrorEvent) ([]UIIntent, []TaskRequest) {
	c.session.IdentityFetching = false
	_ = ev.Err // not surfaced as a flash today; reserved for future hook
	return nil, nil
}

// AllRegions returns the commercial-partition region catalogue. Exposed
// on Core so the adapter does not need to import internal/aws directly
// for the region selector — h4-c uses this to drop the awsclient import
// from runtime_adapter_navigate.go.
func (c *Core) AllRegions() []awsclient.AWSRegion {
	return awsclient.AllRegions()
}

// ResetRuleSets swaps Session.RuleSets to a fresh store and rewires the
// retained ServiceClients transport so any in-flight blocked
// DescribeActiveReceiptRuleSet calls write to the orphaned old store on
// completion. Called by the SES refresh paths in handleRefresh (detail
// view and resource list view, when ResourceType == "ses"). Exposed on
// Core so the adapter does not need to import internal/session for the
// NewRuleSetStore call — h4-c uses this to shrink the session import.
func (c *Core) ResetRuleSets() {
	c.session.RuleSets = session.NewRuleSetStore()
	if c.session.Clients != nil {
		c.session.Clients.SetRuleSets(c.session.RuleSets)
	}
}

// canonShortName resolves an alias to the canonical ShortName when
// resource.FindResourceType returns a hit; otherwise it returns the
// input verbatim (matches the original case-body behaviour for unknown
// keys — they pass through unchanged).
func canonShortName(alias string) string {
	if td := resource.FindResourceType(alias); td != nil {
		return td.ShortName
	}
	return alias
}

// domainCallerIdentityFrom converts an *awsclient.CallerIdentity to the
// renderer-shaped *domain.CallerIdentity mirror. UserID is intentionally
// dropped — only ARN-parser internals use it and the adapter never
// reads it.
func domainCallerIdentityFrom(id *awsclient.CallerIdentity) *domain.CallerIdentity {
	if id == nil {
		return nil
	}
	return &domain.CallerIdentity{
		AccountID:     id.AccountID,
		AccountAlias:  id.AccountAlias,
		Arn:           id.Arn,
		RoleName:      id.RoleName,
		UserName:      id.UserName,
		SessionName:   id.SessionName,
		IdentityName:  id.IdentityName,
		IsAssumedRole: id.IsAssumedRole,
	}
}
