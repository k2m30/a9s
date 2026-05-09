package catalog

import "github.com/k2m30/a9s/v3/internal/domain"

// ResourceTypeDef is the declarative definition of one a9s resource type.
// It is the single source of truth: identity, display, fetchers, enrichers,
// related-panel definitions, and finding codes all live here.
//
// Boundary rule: ResourceTypeDef references types from internal/domain only.
// Fetcher function signatures use `any` for the clients parameter (the
// concrete *aws.ServiceClients type lives in internal/aws, which must NOT
// be imported from internal/catalog).
//
// Wave2 carries the Wave 2 issue-enricher for this type. The concrete type is
// internal/aws.IssueEnricher (a struct with Fn + Priority), stored as `any`
// here to avoid an import cycle. Per-category PRs (04b–04m) will cast to the
// concrete type when populating catalog entries.
type ResourceTypeDef struct {
	// ─── Identity ──────────────────────────────────────────────────────────

	// Name is the human-readable display name (e.g., "EC2 Instances").
	Name string
	// ShortName is the colon-command alias used as registry key (e.g., "ec2").
	ShortName string
	// Aliases are alternative command names for this resource type.
	Aliases []string
	// Category groups resource types in the main menu (e.g., "COMPUTE").
	Category string
	// ListTitle overrides ShortName for list-view frame titles.
	// When empty, ShortName is used.
	ListTitle string

	// ─── Display ───────────────────────────────────────────────────────────

	// Columns defines the table columns for the list view.
	Columns []domain.Column
	// LifecycleKey names the Resource.Fields key holding lifecycle state
	// (e.g. "running", "stopped"). Defaults to "state" when empty.
	LifecycleKey string
	// IdentityKey optionally names the column key used to position the
	// enrichment-finding row marker. When empty, the row-marker resolver
	// uses a cascade (see internal/tui documentation).
	IdentityKey string
	// CellDecorators optionally transforms cell values per column key before
	// render. Key = column key; value = decorator func.
	CellDecorators map[string]func(domain.Resource, string) string
	// CopyField overrides which field CopyContent copies. When non-empty,
	// the resource list copies Fields[CopyField] instead of the default ID.
	CopyField string

	// ─── Behavior ──────────────────────────────────────────────────────────

	// Fetcher is the Wave 1 paginated fetcher for this resource type.
	Fetcher domain.PaginatedFetcher
	// Wave2 is the Wave 2 issue-enricher. nil means no Wave 2 signal.
	// Concrete type is *aws.IssueEnricher; stored as any to avoid import cycle.
	// Replaces NoOpIssueEnricher — nil is the explicit "no Wave 2" signal.
	Wave2 any
	// Project is an optional custom DetailProjector. When nil,
	// projection.Generic is used as the fallback projector.
	Project domain.DetailProjector
	// Related defines the right-column related-resource panel for this type.
	Related []domain.RelatedDef
	// Navigable associates detail-view field paths with target resource types.
	Navigable []domain.NavigableField
	// Children defines child views that can be drilled into from the list view.
	Children []domain.ChildViewDef
	// Reveal is the fetcher for secret/reveal values (e.g. Secrets Manager).
	// nil means no reveal support.
	Reveal domain.RevealFetcher
	// DetailEnrich is an optional on-demand detail enricher (e.g. policy fetch).
	// nil means no detail enrichment beyond the base fetcher.
	DetailEnrich domain.DetailEnricher

	// ─── Cross-cutting ─────────────────────────────────────────────────────

	// Capabilities declares which cross-cutting capabilities this type supports.
	// Handlers for each capability live outside internal/catalog.
	Capabilities []domain.CapabilityID
	// CloudTrailKey specifies how to build the CloudTrail LookupEvents filter.
	// Format: "LookupAttr:ValueSource" (e.g., "ResourceName:ID").
	// Empty string means no CloudTrail support.
	CloudTrailKey string
	// ExcludeFromIssueBadge, when true, excludes this type from the main-menu
	// badge count while still coloring rows and honoring ctrl+z.
	ExcludeFromIssueBadge bool
	// StubCreator optionally creates a minimal stub Resource for the given ID
	// when the target is not yet in the resource cache.
	StubCreator func(string) domain.Resource
	// RelatedContextFromIDs extracts the ParentContext for a child-view
	// navigation triggered from the related panel.
	RelatedContextFromIDs func([]string) map[string]string

	// ─── Findings ──────────────────────────────────────────────────────────

	// Findings is the declarative table of finding codes for this type.
	// Graduated from Phase 03's per-enricher constants.
	Findings []FindingDef
}

// FindingDef is a declarative entry in a resource type's findings table.
// It maps a finding code to its display phrase, severity, and provenance.
type FindingDef struct {
	// Code is the machine-readable finding code (e.g., "ec2.impaired").
	Code domain.FindingCode
	// Phrase is the human-readable §4 display phrase (e.g., "impaired").
	Phrase string
	// Severity classifies the finding for coloring and badge counting.
	Severity domain.Severity
	// Source is the provenance class: "wave1" (emitted by the fetcher)
	// or "wave2" (emitted by the Wave 2 enricher).
	Source string
}
