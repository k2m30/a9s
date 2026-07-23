// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// detail_enrichment.go defines the composite context passed to on-demand
// detail enrichers. It separates pure AWS transport (*ServiceClients) from
// session-scoped caches (e.g. *PolicyDocumentCache) so cache ownership lives
// with the session runtime rather than hanging off transport objects.
package aws

import "github.com/k2m30/a9s/v3/core/domain"

// DetailEnrichmentCtx bundles the AWS service clients together with
// feature-specific session-scoped caches for on-demand detail enrichment.
//
// The detail enricher contract (resource.DetailEnricher) receives an opaque
// `any` for its "clients" argument. The TUI session runtime passes a pointer
// to this struct, and enrichers type-assert to *DetailEnrichmentCtx to access
// both the transport and any session caches they need.
//
// Why the split:
//   - *ServiceClients now carries only AWS transport objects (no session state).
//   - *PolicyDocumentCache (and any future feature-specific caches) are owned
//     by the session runtime, so their lifetime is explicitly tied to
//     profile/region rotations rather than implicit via ServiceClients
//     replacement.
type DetailEnrichmentCtx struct {
	// Clients holds the AWS transport objects. Enrichers that need an AWS
	// API return an "invalid detail-enrichment context" error when this is
	// nil rather than panicking.
	Clients *ServiceClients

	// PolicyDocs is the session-scoped IAM policy document cache. Enrichers
	// that rely on it (role_policies, policy) return an error when it is
	// nil; other enrichers ignore it. Callers construct one per session;
	// session.Session.Rotate() replaces it on profile/region switch.
	PolicyDocs *PolicyDocumentCache

	// DetailDocs is the session-scoped cache for on-demand detail documents
	// (CFN stack templates). Enrichers that rely on it (cfn) return an error
	// when it is nil; other enrichers ignore it. Callers construct one per
	// session; session.Session.Rotate() replaces it on profile/region switch.
	DetailDocs *DetailDocCache

	// SkipCache tells the engine (detail_enrich_engine.go) to bypass the
	// cache READ for this one enrichment call while still WRITING its fresh
	// result — set for an explicit refresh (detail Ctrl+R). Without this, a
	// cached enricher whose key is derived from the list row itself (cfn's
	// versioned "cfn:<id>:<lastUpdatedUnix>" key) would unwrap the CURRENT
	// StackEnriched RawStruct — which still carries the pre-refresh
	// LastUpdatedTime — and rebuild the SAME cache key, hitting the stale
	// entry despite the operator explicitly asking to refresh. Any future
	// cached enricher inherits the same guarantee for free.
	SkipCache bool

	// OpID is the core/runtime.DetailOperation.ID this enrichment call was
	// dispatched under. enrichDetail (detail_enrich_engine.go) wraps the
	// fetch-side ctx with WithDetailOp(ctx, OpID) so every coalescing
	// decorator (coalesce.go) keys its singleflight group per-operation —
	// see WithDetailOp's doc comment.
	OpID domain.Gen
}
