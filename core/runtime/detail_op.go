// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package runtime

import (
	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// DetailOperation is the single identity for one detail-view lifecycle: the
// user opening a resource's detail (or YAML/JSON) view, or explicitly
// refreshing an already-open one. Every question a detail-scoped background
// task or its result needs answered has exactly one answer — this
// operation:
//
//   - Which AWS transport to call: op.Clients, captured once at
//     construction, never re-read from the session afterward.
//   - Which coalescing namespace concurrent AWS calls share: op.ID, wrapped
//     onto every call's context via awsclient.WithDetailOp — the enricher and
//     every related checker begun together share one in-flight call per
//     underlying API key; a later operation (fresh open or refresh) can never
//     join an earlier one's calls, because its ID is different.
//   - Whether a given result is still wanted: a result is accepted only when
//     its OperationID equals the session's current DetailOpGen (see
//     messages.AspectDetailOp) — the operation that produced it is still the
//     active one.
//
// Constructed exactly once per open/refresh via Core.BeginDetailOperation,
// copied by value into every task payload it spawns (EnrichDetailPayload,
// RelatedCheckPayload), and never mutated afterward.
type DetailOperation struct {
	ID           domain.Gen
	ResourceType string
	Resource     resource.Resource
	Clients      *awsclient.ServiceClients
	Refresh      bool
}

// BeginDetailOperation starts a new DetailOperation for resourceType/res,
// bumping session.DetailOpGen so the returned ID becomes both this
// operation's identity and the session's new active one — no earlier
// operation's in-flight enrich/related-check results can be accepted after
// this call, and no earlier operation's in-flight AWS calls can be joined by
// calls this operation makes (see DetailOperation's doc comment).
//
// Must be called while the caller's own serialization is already held:
// core/app's call sites (navigate.go, actions_list.go, actions_nav.go) call
// this under Controller.mu; internal/tui's direct call sites rely on Bubble
// Tea's single Update() goroutine, the same discipline every other
// session-mutating Core method in this package already relies on.
func (c *Core) BeginDetailOperation(resourceType string, res resource.Resource, refresh bool) DetailOperation {
	id := c.session.DetailOpGen.Bump()
	return DetailOperation{
		ID:           id,
		ResourceType: resourceType,
		Resource:     res,
		Clients:      c.session.Clients,
		Refresh:      refresh,
	}
}

// ActiveDetailOp returns the session's current detail-operation ID — the
// value a detail-scope GenStamped event (EnrichDetailResult,
// RelatedCheckResult, RelatedCheckBatch) must match to be accepted.
func (c *Core) ActiveDetailOp() domain.Gen {
	return c.session.DetailOpGen
}

// DetailOperationTasks returns the enrich-detail and related-check
// TaskRequests for op, gated by catalog registration: enrichTask is
// non-nil only when resource.GetDetailEnricher(op.ResourceType) is
// registered; relatedTask is non-nil only when
// resource.GetRelated(op.ResourceType) is non-empty — mirroring the
// dispatch gate the former per-navigation HandleEnrichDetail/
// HandleRelatedCheckStarted calls used.
//
// Callers that intend to replay a cached related result instead of
// dispatching a fresh fan-out (Controller.ReplayRelatedCache) must drop
// relatedTask themselves — cache-hit replay is a caching concern, not an
// operation concern, so it stays at the call site.
func (c *Core) DetailOperationTasks(op DetailOperation) (enrichTask, relatedTask *TaskRequest) {
	scope := op.ResourceType + "/" + op.Resource.ID

	if resource.GetDetailEnricher(op.ResourceType) != nil {
		var dctx *awsclient.DetailEnrichmentCtx
		if op.Clients != nil || c.session.PolicyDocCache != nil || c.session.DetailDocCache != nil {
			dctx = &awsclient.DetailEnrichmentCtx{
				Clients:    op.Clients,
				PolicyDocs: c.session.PolicyDocCache,
				DetailDocs: c.session.DetailDocCache,
				SkipCache:  op.Refresh,
				OpID:       op.ID,
			}
		}
		enrichTask = &TaskRequest{
			Key:     TaskKey{Kind: KindEnrichDetail, Scope: scope},
			Cache:   CacheNone,
			Payload: EnrichDetailPayload{Op: op, DetailCtx: dctx},
		}
	}

	if len(resource.GetRelated(op.ResourceType)) > 0 {
		relatedTask = &TaskRequest{
			Key:     TaskKey{Kind: KindRelatedCheck, Scope: scope},
			Cache:   CacheNone,
			Payload: RelatedCheckPayload{Op: op},
		}
	}

	return enrichTask, relatedTask
}
