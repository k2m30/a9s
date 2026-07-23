// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// Package runtime — see orchestrator.go for the package overview.
//
// enrich.go owns the KindEnrichDetail task family. Dispatch is decided by
// Core.DetailOperationTasks (detail_op.go) — a resource type either has a
// registered detail enricher or it doesn't; there is no separate policy
// gate here anymore.
package runtime

import (
	awsclient "github.com/k2m30/a9s/v3/core/aws"
)

// KindEnrichDetail is the TaskKind the runtime emits to ask the adapter
// to run the on-demand detail enricher for a single resource. Adapters
// look up the enricher via resource.GetDetailEnricher and post the
// result back through the normal event channel.
const KindEnrichDetail TaskKind = "enrich-detail"

// EnrichDetailPayload is the typed TaskPayload variant for KindEnrichDetail.
// Op carries the resource type/resource/AWS clients/refresh flag every
// enrich dispatch needs (see DetailOperation's doc comment); DetailCtx is
// the DetailEnrichmentCtx Core.DetailOperationTasks built from op.Clients +
// session.PolicyDocCache + session.DetailDocCache, with SkipCache/OpID
// already stamped from op. The DetailCtx pointer is nil only when both
// op.Clients and the session's document caches are unset (test harnesses
// constructing a Core without a transport) — the adapter is responsible for
// tolerating that branch.
type EnrichDetailPayload struct {
	Op        DetailOperation
	DetailCtx *awsclient.DetailEnrichmentCtx
}

// isTaskPayload satisfies the TaskPayload marker interface.
func (EnrichDetailPayload) isTaskPayload() {}
