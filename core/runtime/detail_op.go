// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package runtime

import (
	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/trace"
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
// Also returns the operation's COMPLETE workload as one slice — an
// EnrichDetailPayload task only when resource.GetDetailEnricher(resourceType)
// is registered, a RelatedCheckPayload task only when
// resource.GetRelated(resourceType) is non-empty — in the SAME call that
// mints the op ID. No public type here names "half a workload": earlier
// revisions returned the two as separate *TaskRequest out-params, which let
// a caller keep one and drop the other; a single opaque slice has no such
// seam. The sole caller is core/app's beginDetailWorkloadLocked builder,
// which may itself trim the related entry out (cache-replay suppression) but
// never receives it as a separately addressable value.
//
// Must be called while the caller's own serialization is already held:
// core/app's beginDetailWorkloadLocked calls this under Controller.mu.
func (c *Core) BeginDetailOperation(resourceType string, res resource.Resource, refresh bool) (op DetailOperation, tasks []TaskRequest) {
	id := c.session.DetailOpGen.Bump()
	// Live fetchers (e.g. S3, CloudTrail events) may leave res.Type empty;
	// the operation is the single identity downstream code (e.g. RunRelatedDef's
	// ct-events lazy-add exemption) trusts, so it must carry a real type even
	// when the caller's own resource value doesn't.
	if res.Type == "" {
		res.Type = resourceType
	}
	op = DetailOperation{
		ID:           id,
		ResourceType: resourceType,
		Resource:     res,
		Clients:      c.session.Clients,
		Refresh:      refresh,
	}

	if trace.Enabled() {
		trace.Emit(trace.Event{
			Kind:         trace.KindDetailOpBegin,
			OperationID:  uint64(op.ID),
			ResourceType: op.ResourceType,
			ResourceID:   op.Resource.ID,
			Refresh:      op.Refresh,
		})
	}

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
		tasks = append(tasks, TaskRequest{
			Key:     TaskKey{Kind: KindEnrichDetail, Scope: scope},
			Cache:   CacheNone,
			Payload: EnrichDetailPayload{Op: op, DetailCtx: dctx},
		})
	}

	if len(resource.GetRelated(op.ResourceType)) > 0 {
		tasks = append(tasks, TaskRequest{
			Key:     TaskKey{Kind: KindRelatedCheck, Scope: scope},
			Cache:   CacheNone,
			Payload: RelatedCheckPayload{Op: op},
		})
	}

	return op, tasks
}

// ActiveDetailOp returns the session's current detail-operation ID — the
// value a detail-scope GenStamped event (EnrichDetailResult,
// RelatedCheckResult, RelatedCheckBatch) must match to be accepted.
func (c *Core) ActiveDetailOp() domain.Gen {
	return c.session.DetailOpGen
}
