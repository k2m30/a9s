// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package catalog

import (
	"strings"

	"github.com/k2m30/a9s/v3/core/domain"
)

// ResourceTypeDef is the declarative definition of one a9s resource type.
// It is the single source of truth: identity, display, fetchers, enrichers,
// related-panel definitions, and finding codes all live here.
//
// Boundary rule: ResourceTypeDef references types from core/domain only.
// Fetcher function signatures use `any` for the clients parameter (the
// concrete *aws.ServiceClients type lives in core/aws, which must NOT
// be imported from core/catalog).
//
// Wave2 carries the Wave 2 issue-enricher for this type. The concrete type is
// core/aws.IssueEnricher (a struct with Fn + Priority), stored as `any`
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
	// TitleOmitsID suppresses the resource ID in the detail frame title for
	// types whose ID is an opaque synthetic key (e.g. a 56-digit CloudWatch
	// event id). When true the detail title renders "detail -- <Name>"
	// instead of "detail -- <ID> (<Name>)".
	TitleOmitsID bool
	// CostExplorerServiceName is the exact Cost Explorer SERVICE dimension
	// value (e.g. "Amazon Elastic Compute Cloud - Compute") this type's
	// billed usage is reported under. Empty means Cost Explorer's per-service
	// cost grid has no RESOURCE_ID drill-down mapped to this type — its rows
	// get an "unsupported" note instead of a detail-view jump.
	CostExplorerServiceName string
	// ConsoleURL returns the AWS console page for r, or "" when the type has
	// no console page or required inputs are missing. region is the active
	// session region; accountID may be "" before the identity fetch lands.
	ConsoleURL func(r domain.Resource, region, accountID string) string

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
	// HumanizeFields names the fields of this type whose value is an AWS
	// constant an operator should not have to read, so every surface routes
	// them through domain.HumanizeStatusPhrase. Entries are matched
	// case-insensitively against a Fields key, a RawStruct path or a detail
	// row's label, because one fact is reached by all three spellings.
	//
	// The declaration is on the TYPE and not on a column: it says something
	// about the FACT, so a field no column happens to show owes the same
	// wording on the detail as one that has a column, and a field with a
	// column cannot read one way there and another way in the detail. A
	// second declaration for the second surface is a fact in two places to
	// disagree.
	//
	// Only enum-shaped values belong here. A name AWS assigned and a person
	// types back — an access key, a role id, an API operation name — must
	// stay verbatim or it stops being the thing they can search for.
	HumanizeFields []string

	// ─── Behavior ──────────────────────────────────────────────────────────

	// Fetcher is the Wave 1 paginated fetcher for this resource type.
	Fetcher domain.PaginatedFetcher
	// AvailabilityFetcher is an optional, cheaper alternative to Fetcher used
	// ONLY by the availability/count probe (core/runtime/probes.go's
	// Core.ProbeResourceAvailability). nil means the probe falls back to
	// Fetcher unchanged. Exists for types whose real list content is
	// materially more expensive to resolve than a mere availability/count
	// signal needs (e.g. "policy": managed policies alone suffice for a
	// count badge, without IAM's expensive per-group inline-policy sweep the
	// real list-open path also resolves).
	AvailabilityFetcher domain.PaginatedFetcher
	// Wave2 is the Wave 2 issue-enricher. nil means no Wave 2 signal.
	// Concrete type is aws.IssueEnricher (value type); stored as any to
	// avoid import cycle. A zero IssueEnricher with Fn == nil behaves
	// identically to a nil any — both bypass Wave 2 dispatch via the
	// AllWave2 filter in core/aws/wave2.go.
	Wave2 any
	// Project is an optional custom DetailProjector. When nil,
	// projection.GenericWithConfig (or GenericWithConfigAndNavProvider) is
	// used as the fallback projector.
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
	// FieldKeys lists the valid Resource.Fields keys produced by the Wave 1
	// fetcher. Populated by aws.Install(); zero value if no Wave 1 surface.
	FieldKeys []string
	// FieldAliases maps source field keys to alias keys copied into the
	// resource's Fields by ApplyFieldAliases. Populated by aws.Install().
	FieldAliases map[string]string
	// FetchByIDs fetches a specific set of resource instances by ID, bypassing
	// any pagination. Populated by aws.Install(); zero value if no Wave 1 surface.
	FetchByIDs domain.FetchByIDsFunc
	// FilteredFetcher returns a single page of resources filtered server-side.
	// Populated by aws.Install(); zero value if no server-side filter is supported.
	FilteredFetcher domain.FilteredPaginatedFetcher
	// IssueEnricherFieldKeys lists the Resource.Fields keys that the Wave 2
	// issue enricher writes via IssueEnricherResult.FieldUpdates. Populated by
	// aws.Install(); zero value if no Wave 2 surface.
	IssueEnricherFieldKeys []string
	// ChildFetcher is the paginated child-resource fetcher for child types.
	// Only meaningful on child-type entries (set via catalog.SetChildTypes).
	// Populated by aws.Install(); zero value on top-level type entries.
	ChildFetcher domain.PaginatedChildFetcher

	// ─── Cross-cutting ─────────────────────────────────────────────────────

	// CloudTrailKey specifies how to build the CloudTrail LookupEvents filter.
	// Format: "LookupAttr:ValueSource" (e.g., "ResourceName:ID").
	// LookupAttr is normally a CloudTrail LookupAttributeKey (e.g. "Username",
	// "ResourceName") sent as a server-side filter. When LookupAttr instead
	// starts with "_localfield." (e.g. "_localfield.role_name"), the filter is
	// not sent to CloudTrail at all — it is checked locally, after the page is
	// fetched, against the built event's Resource.Fields[<FieldsKey>] using the
	// same value source. Use the "_localfield." form when the value CloudTrail
	// should match on is not exposed as a queryable LookupAttribute (e.g. an
	// assumed-role session's CloudTrail Username is the session name, not the
	// role name — the role name only exists in a parsed event field).
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

	// ─── Color & Augmentation ──────────────────────────────────────────────

	// Color classifies the row's health. REQUIRED for all registered types.
	// Findings-first: derives the color from the resource's own Findings via
	// colorFromAnyFinding (worst severity wins). Raw structural fields are a
	// fallback consulted only when the resource carries no Finding at all.
	Color func(domain.Resource) domain.Color

	// Augment is an optional post-projector hook that injects additional sections
	// after the main projector has run (e.g. EC2 status checks). When nil, no
	// augmentation is applied. Pure function.
	Augment domain.Augmenter

	// ─── Findings ──────────────────────────────────────────────────────────

	// Findings is the declarative table of finding codes for this type.
	Findings []FindingDef
}

// ResolveColor classifies r using d.Color, defaulting to a generic
// status-based color when d.Color is nil, reading Fields["status"]. All
// registered types have non-nil Color (invariant #7); the fallback exists
// only for ad-hoc test doubles.
func (d ResourceTypeDef) ResolveColor(r domain.Resource) domain.Color {
	if d.Color == nil {
		return colorFallback(r.Fields["status"])
	}
	return d.Color(r)
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
	// Detail is the S5 operator sentence the finding renders in the
	// detail-view Attention section, and the sentence catalogen writes into
	// the docs. Empty means the finding renders its Phrase alone and the doc
	// cell reads "—". The emitter copies it from here; nothing else declares it.
	Detail string
}

// HumanizeFieldKey normalizes a field identifier to the FACT it names: case
// and underscores dropped, so "ClusterType", "cluster_type" and "Cluster Type"
// are one key.
//
// One fact is reached by more than one spelling — a list column reads it by
// its RawStruct path, a fetcher writes it in snake_case, a detail row labels
// it in words — and a declaration that answered only one of them would
// humanize the surface that happens to use that spelling and leave the others
// showing the constant. That is the same fact in two words, which is the
// shape the declaration exists to remove.
func HumanizeFieldKey(s string) string {
	return strings.ToLower(strings.NewReplacer("_", "", " ", "").Replace(s))
}

// HumanizedFields returns HumanizeFields as a lookup set keyed by
// HumanizeFieldKey, or nil when the type declares none. The list surface, the
// detail surface and any later reader ask here, so none of them can disagree
// about which fields the type wants in words.
func (d ResourceTypeDef) HumanizedFields() map[string]bool {
	if len(d.HumanizeFields) == 0 {
		return nil
	}
	out := make(map[string]bool, len(d.HumanizeFields))
	for _, f := range d.HumanizeFields {
		out[HumanizeFieldKey(f)] = true
	}
	return out
}

// Humanizes reports whether set — a type's HumanizedFields — names any of the
// spellings by which one field is reached. Every reader asks here rather than
// indexing the map itself, so no two of them can normalize a spelling
// differently and disagree about the same field.
func Humanizes(set map[string]bool, names ...string) bool {
	for _, n := range names {
		if n != "" && set[HumanizeFieldKey(n)] {
			return true
		}
	}
	return false
}
