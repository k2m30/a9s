// Package runtime — see orchestrator.go for the package overview.
//
// enrich.go owns the on-demand detail-view enrichment dispatch policy.
// PR-05a-h7 moves the entry point off internal/tui per the boundary
// contract in docs/refactor/05-boundary.md §"5a-extract": the receiver
// migrates from *Model to *Core, and the function no longer returns a
// tea.Cmd. Instead it emits a TaskRequest the adapter translates into
// platform-specific async work.
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
// PR-05a-h4-b (AS-962) moved DetailCtx construction off the adapter so
// internal/tui no longer touches awsclient.DetailEnrichmentCtx directly;
// the adapter reads the typed payload fields verbatim instead. The
// DetailCtx pointer is nil only when session.Clients is unset (test
// harnesses constructing a Core without a transport) — the adapter is
// responsible for tolerating that branch.
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
// Receiver migrated from *Model to *Core per docs/refactor/05-boundary.md.
// EnrichGen / PolicyDocCache reads stay adapter-side (read from
// c.Session()) until those fields migrate in the orchestrator/state-
// promotion PR.
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
