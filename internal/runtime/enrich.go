// Package runtime — see orchestrator.go for the package overview.
//
// enrich.go owns the on-demand detail-view enrichment dispatch policy.
// PR-05a-h7 moves the entry point off internal/tui per the boundary
// contract in docs/refactor/05-boundary.md §"5a-extract": the receiver
// migrates from *Model to *Core, and the function no longer returns a
// tea.Cmd. Instead it emits a TaskRequest the adapter translates into
// platform-specific async work.
package runtime

import "github.com/k2m30/a9s/v3/internal/resource"

// KindEnrichDetail is the TaskKind the runtime emits to ask the adapter
// to run the on-demand detail enricher for a single resource. Adapters
// look up the enricher via resource.GetDetailEnricher and post the
// result back through the normal event channel (e.g. a Bubble Tea
// EnrichDetailResultMsg with the captured EnrichGen for stale-result
// rejection).
const KindEnrichDetail TaskKind = "enrich-detail"

// EnrichDetailEvent is the runtime-side event the adapter forwards when
// the user opens (or refreshes) a detail view that should be enriched
// on demand. Adapters translate from their native message type before
// calling HandleEnrichDetail.
type EnrichDetailEvent struct {
	ResourceType string
	Resource     resource.Resource
}

// HandleEnrichDetail decides whether to dispatch detail enrichment for
// the given event. Today's contract: only resource types with a
// registered detail enricher trigger a TaskRequest; everything else is
// a no-op. No UIIntent is emitted at dispatch time — the adapter posts
// the result back as a normal event.
//
// Receiver migrated from *Model to *Core per docs/refactor/05-boundary.md.
// The previous m.EnrichGen / m.PolicyDocCache accesses are adapter-owned
// state that the adapter reads directly from c.Session() when building
// the platform-specific async closure.
//
// Sibling per-handler PRs expose their own entry points in the same
// pattern. Once enough handlers have migrated, Core.HandleEvent will
// aggregate them into a single type-switch; until then each handler
// entry point keeps its migration step localized and independently
// reviewable.
func (c *Core) HandleEnrichDetail(ev EnrichDetailEvent) ([]UIIntent, []TaskRequest) {
	if resource.GetDetailEnricher(ev.ResourceType) == nil {
		return nil, nil
	}
	return nil, []TaskRequest{{
		Key:   TaskKey{Kind: KindEnrichDetail, Scope: ev.ResourceType + "/" + ev.Resource.ID},
		Cache: CacheNone,
	}}
}
