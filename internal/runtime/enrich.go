// Package runtime — see orchestrator.go for the package overview.
//
// enrich.go owns the on-demand detail-view enrichment dispatch policy. The
// receiver is *Core (not *Model) and it returns a TaskRequest rather than a
// tea.Cmd — the adapter translates that into platform-specific async work.
package runtime

import (
	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

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

// EnrichDetailPayload is the typed TaskPayload variant for
// KindEnrichDetail. It carries the structured fields the adapter needs
// to execute the dispatch — the resource type (selects the registered
// detail enricher), the resource itself (the enricher's input), the
// DetailEnrichmentCtx the runtime constructs from session-owned
// Clients/PolicyDocCache, and the EnrichGen captured at dispatch so
// the adapter can stamp the result for stale-rejection on receipt.
//
// DetailCtx construction lives on the runtime, not the adapter, so
// internal/tui never touches awsclient.DetailEnrichmentCtx directly; the
// adapter reads the typed payload fields verbatim. The DetailCtx pointer is
// nil only when session.Clients is unset (test harnesses constructing a Core
// without a transport) — the adapter is responsible for tolerating that
// branch.
type EnrichDetailPayload struct {
	ResourceType string
	Resource     resource.Resource
	DetailCtx    *awsclient.DetailEnrichmentCtx
	Generation   domain.Gen
}

// isTaskPayload satisfies the TaskPayload marker interface.
func (EnrichDetailPayload) isTaskPayload() {}

// HandleEnrichDetail decides whether to dispatch detail enrichment for
// the given event. Today's contract: only resource types with a
// registered detail enricher trigger a TaskRequest; everything else is
// a no-op. No UIIntent is emitted at dispatch time — the adapter posts
// the result back as a normal event.
//
// The runtime is the single source of truth for the dispatch decision:
// when a TaskRequest is returned, the adapter is invariant-guaranteed
// that an enricher is registered for Payload.ResourceType. The adapter
// does not re-check, ensuring SSOT for the policy gate.
//
// EnrichGen / PolicyDocCache reads stay adapter-side (read from c.Session()).
//
// Sibling per-handler PRs expose their own entry points in the same
// pattern. Once enough handlers have migrated, Core.HandleEvent will
// aggregate them into a single type-switch.
func (c *Core) HandleEnrichDetail(ev EnrichDetailEvent) ([]UIIntent, []TaskRequest) {
	if resource.GetDetailEnricher(ev.ResourceType) == nil {
		return nil, nil
	}
	var dctx *awsclient.DetailEnrichmentCtx
	if c.session.Clients != nil || c.session.PolicyDocCache != nil {
		dctx = &awsclient.DetailEnrichmentCtx{
			Clients:    c.session.Clients,
			PolicyDocs: c.session.PolicyDocCache,
		}
	}
	return nil, []TaskRequest{{
		Key:   TaskKey{Kind: KindEnrichDetail, Scope: ev.ResourceType + "/" + ev.Resource.ID},
		Cache: CacheNone,
		Payload: EnrichDetailPayload{
			ResourceType: ev.ResourceType,
			Resource:     ev.Resource,
			DetailCtx:    dctx,
			Generation:   c.session.EnrichGen,
		},
	}}
}
