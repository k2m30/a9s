// issue_enrichment.go owns Wave 2 issue-enrichment infrastructure: the
// IssueEnricher registry, the registerIssueEnricher helper, NoOpIssueEnricher,
// shared caps, and truly-shared helpers used by more than one enricher file.
package aws

import (
	"context"
	"fmt"
	"strings"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// IssueEnricher carries a Wave 2 IssueEnricherFunc plus scheduling metadata.
// Priority controls Wave 2 dispatch order: lower values run first.
// The default priority is 100; batchable (cheap) enrichers use 10.
//
// Distinct from resource.DetailEnricher (internal/resource/enricher.go) which
// is the on-demand detail-view enricher contract.
type IssueEnricher struct {
	Fn       IssueEnricherFunc
	Priority int // lower runs first; default 100
}

// IssueEnricherRegistry maps resource short names to their Wave 2 Enricher metadata.
// Each entry carries the enricher function (Fn) and its dispatch priority
// (Priority: lower runs first; 10 = batchable/cheap, 100 = default).
//
// Priority is the single source of truth for enrichment ordering.
// buildEnrichQueue in internal/tui/app_fetchers.go sorts by Priority then
// alphabetically within each tier — no hardcoded ordering list needed.
//
// Every registered resource type per docs/attention-signals.md either:
//   - has a real Wave 2 enricher registered here (Wave 2 column non-empty), or
//   - is registered with NoOpIssueEnricher (Wave 2 column is "None" in the doc).
var IssueEnricherRegistry = map[string]IssueEnricher{}

// NoOpIssueEnricher is registered for resource types whose Wave 2 column in
// docs/attention-signals.md is "None". It makes the "no Wave 2 signal"
// classification explicit in the registry rather than implicit-by-absence.
// Returns zero findings, zero issues, not truncated — never fails.
func NoOpIssueEnricher(_ context.Context, _ *ServiceClients, _ []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	return IssueEnricherResult{
		Findings:     map[string]resource.EnrichmentFinding{},
		TruncatedIDs: map[string]bool{},
		IssueCount:   0,
		Truncated:    false,
	}, nil
}

// EnrichmentCap is the maximum number of per-resource API calls for non-batchable enrichers.
const EnrichmentCap = 50

// PerParentPageCap limits per-parent pagination walks in enrichers to avoid
// runaway enumeration on huge tenants. When hit, the emitted count is marked
// with a "+" suffix to signal approximate.
const PerParentPageCap = 10

// isInstanceARN returns true when the RDS ARN targets a DB instance
// (resource-type segment = "db"), not a cluster, snapshot, or other resource.
// ARN format: arn:aws:rds:region:account:resource-type:id
func isInstanceARN(arn string) bool {
	parts := strings.Split(arn, ":")
	return len(parts) >= 7 && parts[5] == "db"
}

// formatDate formats a *time.Time as "2006-01-02" or returns "" for nil.
func formatDate(t interface{ Format(string) string }) string {
	if t == nil {
		return ""
	}
	return t.Format("2006-01-02")
}

// registerIssueEnricher registers a Wave 2 enricher function for the given short name
// with the given priority. Panics on: empty shortName, nil fn, duplicate registration,
// or non-positive priority.
func registerIssueEnricher(shortName string, fn IssueEnricherFunc, priority int) {
	if shortName == "" {
		panic("registerIssueEnricher: short name must not be empty")
	}
	if fn == nil {
		panic(fmt.Sprintf("registerIssueEnricher: nil fn for short name %q", shortName))
	}
	if _, exists := IssueEnricherRegistry[shortName]; exists {
		panic(fmt.Sprintf("registerIssueEnricher: duplicate registration for short name %q", shortName))
	}
	if priority <= 0 {
		panic(fmt.Sprintf("registerIssueEnricher: priority must be positive, got %d for short name %q", priority, shortName))
	}
	IssueEnricherRegistry[shortName] = IssueEnricher{Fn: fn, Priority: priority}
}

// IssueEnricherResult is the typed return value of a Wave 2 issue enricher.
//
//   - IssueCount: number of resources classified issue-worthy for the menu badge
//     (severity "!" findings; "~" informational do NOT count).
//
//   - Truncated: GLOBAL signal — true when ANY part of the enricher's walk was
//     cut short (EnrichmentCap hit, page cap hit, or API errors skipped records).
//     Kept for back-compat and banner aggregation. Prefer TruncatedIDs for
//     per-resource resolution.
//
//   - TruncatedIDs: per-resource truncation. Key = Resource.ID that could not be
//     fully inspected (API error on that resource, page cap hit during a
//     per-parent paginated walk, etc.). The UI renders "?" on just that row
//     instead of a global banner. An ID appearing here MUST NOT also appear in
//     Findings unless the partial data was still usable.
//
//   - Findings: map from Resource.ID → EnrichmentFinding. May contain entries
//     for resources NOT in the input slice (account-wide enrichers). Enrichers
//     that receive API identifiers in a different form (e.g., ARNs) MUST
//     normalize to Resource.ID before writing to Findings.
//
//   - FieldUpdates: map from Resource.ID → (fieldKey → value). Same normalization
//     rule applies.
//
// MAY have empty maps but MUST NOT be nil for any reference field on
// success — initialize each with `make(...)` before returning.
type IssueEnricherResult struct {
	IssueCount   int
	Truncated    bool
	TruncatedIDs map[string]bool
	Findings     map[string]resource.EnrichmentFinding
	// FieldUpdates carries per-resource Fields[] mutations the enricher wants
	// merged into the cached row. Keyed by resource ID, then by field key.
	// Used by list columns and Color funcs that need access to Wave-2-derived
	// data without subscribing to the Findings stream separately.
	// MUST NOT be nil if the enricher writes any updates; use
	// make(map[string]map[string]string).
	FieldUpdates map[string]map[string]string
}

// IssueEnricherFunc is a pluggable function that makes additional API calls
// for a resource type and returns a typed IssueEnricherResult. The resources
// slice contains retained first-page resources from Wave 1 probes. The cache
// parameter provides sibling-type ResourceCache entries for cross-ref enrichers
// (e.g. dbi-snap reads cache["dbi"] to detect orphan/past-retention signals).
// Non-cross-ref enrichers ignore the cache via `_ resource.ResourceCache`.
// This is the Wave 2 issue-enrichment contract; distinct from on-demand DetailEnricher
// (internal/resource/enricher.go) which enriches a single resource for detail views.
//
// Cache invariant — read-only shallow snapshot:
// The dispatcher (internal/tui/app_probes.go probeEnrichment) builds the cache
// once at dispatch time via m.buildResourceCacheSnapshot() and passes the
// resulting map by value. The map and its ResourceCacheEntry structs are
// freshly allocated, but the .Resources slice header is COPIED — its backing
// array is shared with the live m.resourceCache / m.probeResources / m.lazyResourceCache
// state. Enrichers MUST treat the cache as read-only:
//   - DO NOT append to cache[k].Resources (would mutate the shared backing array
//     when len < cap, surfacing as phantom rows in the live view).
//   - DO NOT mutate fields on cache[k].Resources[i] or cache[k].Resources[i].RawStruct
//     (those are pointers / interface values shared with the running app).
//   - DO read field values, lengths, and IsTruncated freely.
// Violations are not currently caught at compile time. Future contributors
// who need to derive a mutable view should append([]Resource{}, slice...) into
// a local slice first.
type IssueEnricherFunc func(ctx context.Context, clients *ServiceClients, resources []resource.Resource, cache resource.ResourceCache) (IssueEnricherResult, error)
