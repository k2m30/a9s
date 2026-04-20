// detail_enrichment.go defines the composite context passed to on-demand
// detail enrichers. It separates pure AWS transport (*ServiceClients) from
// session-scoped caches (e.g. *PolicyDocumentCache) so cache ownership lives
// with the session runtime rather than hanging off transport objects.
package aws

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
	// nil; other enrichers ignore it. Callers construct one per session and
	// rotate it on profile/region switch via resetForSessionSwitch.
	PolicyDocs *PolicyDocumentCache
}
