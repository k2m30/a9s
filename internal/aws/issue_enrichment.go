// issue_enrichment.go owns Wave 2 issue-enrichment shared types and helpers:
// the IssueEnricher metadata struct, InFetcherWave2Sentinel, the result/func
// contracts, and truly-shared helpers used by more than one enricher file.
//
// AS-795n removed the legacy package-init IssueEnricherRegistry map and the
// registerIssueEnricher helper. Wave 2 enricher registrations now live as
// the Wave2 field on each catalog.ResourceTypeDef literal in the
// catalog_<category>.go files. Read access goes through
// awsclient.Wave2EnricherFor / awsclient.AllWave2 in wave2.go.
package aws

import (
	"context"
	"strings"

	"github.com/k2m30/a9s/v3/internal/domain"
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

// InFetcherWave2Sentinel is the explicit "Wave 2 done in the fetcher" sentinel.
// Used by catalog entries (currently eks, ng, trail) whose Wave 2 signal in
// docs/attention-signals.md is non-None but is populated synchronously by the
// fetcher (e.g. EKS DescribeCluster, EKS Node Group DescribeNodegroup,
// CloudTrail GetTrailStatus per-resource). Setting `Wave2: IssueEnricher{Fn:
// InFetcherWave2Sentinel, Priority: 100}` keeps TestAttentionSignalsDoc happy
// (it sees a non-nil Wave2 wiring) without scheduling a redundant background
// enrichment pass — the sentinel returns zero findings.
//
// Resource types whose Wave 2 column is "None" in docs/attention-signals.md
// must omit the Wave2 field entirely; this sentinel is reserved for the
// in-fetcher case. Returns zero findings, zero issues, not truncated, never
// fails. Tests use it as a benign Fn fixture too.
//
// Renamed in AS-731 to make the in-fetcher contract explicit; the prior
// name read as "no-op" and prompted the question "why is a no-op enricher
// in the catalog at all?" during review. The rename also satisfies the
// AS-731 zero-hit grep gate for the prior name in internal/.
func InFetcherWave2Sentinel(_ context.Context, _ *ServiceClients, _ []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	return IssueEnricherResult{
		Findings:         map[string]domain.Finding{},
		AttentionDetails: map[string]domain.AttentionDetail{},
		TruncatedIDs:     map[string]bool{},
		FieldUpdates:     map[string]map[string]string{},
		IssueCount:       0,
		Truncated:        false,
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

// setWave2Finding writes a Wave-2 Finding + AttentionDetail pair into the
// IssueEnricherResult maps. AS-1395 helper that keeps per-file migration
// uniform: every enricher constructs its emission via this helper rather than
// re-implementing the glyph→Severity mapping and the AttentionDetail{Rows: …}
// packing in every file.
//
// severityGlyph: "!" → SevBroken, "~" → SevWarn, otherwise → SevDim. Matches
// the legacy EnrichmentFinding.Severity glyph contract used by per-enricher
// docstrings — view code now consumes domain.Severity directly.
//
// rows MAY be nil; the helper omits the AttentionDetail entry when empty so a
// nil-row finding does not surface an empty Attention section.
//
// shortName stamps Source = "wave2:<shortName>" on the emitted Finding. It is
// the resource short name the enricher serves (e.g. "acm", "dbi", "tg").
//
// The caller is responsible for initialising r.Findings and (when emitting
// rows) r.AttentionDetails before calling this helper. The IssueEnricherResult
// godoc requires both reference fields be non-nil on a successful return.
func setWave2Finding(
	r *IssueEnricherResult,
	resourceID string,
	code domain.FindingCode,
	phrase string,
	severityGlyph string,
	shortName string,
	rows []domain.DetailRow,
) {
	r.Findings[resourceID] = domain.Finding{
		Code:     code,
		Phrase:   phrase,
		Severity: glyphToSeverity(severityGlyph),
		Source:   "wave2:" + shortName,
	}
	if len(rows) > 0 {
		if r.AttentionDetails == nil {
			r.AttentionDetails = make(map[string]domain.AttentionDetail)
		}
		r.AttentionDetails[resourceID] = domain.AttentionDetail{Rows: rows}
	}
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
//   - Findings: map from Resource.ID → domain.Finding. The enricher emits ≤1
//     Finding per Resource.ID. May contain entries for resources NOT in the
//     input slice (account-wide enrichers). Enrichers that receive API
//     identifiers in a different form (e.g., ARNs) MUST normalize to
//     Resource.ID before writing to Findings.
//
//   - AttentionDetails: per-resource supporting rows for the Wave-2 Finding,
//     keyed by Resource.ID. Only entries with non-empty rows are emitted; the
//     fold layer (runtime.Core.applyEnrichment) re-keys to FindingCode against
//     the matching Finding when writing onto r.AttentionDetails.
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
	Findings     map[string]domain.Finding
	// AttentionDetails carries per-resource supporting rows for the Wave-2
	// Finding emitted in Findings. Keyed by Resource.ID until the fold layer
	// (runtime.Core.applyEnrichment) flips it to FindingCode against the
	// matching r.Findings entry. Enrichers MAY omit this when no rows accompany
	// the finding.
	AttentionDetails map[string]domain.AttentionDetail
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
// The TUI dispatcher (internal/tui/probe_adapter.go probeEnrichment tea.Cmd wrapper)
// invokes (*Core).ProbeEnrichment, which builds the cache once at dispatch time via
// (*Core).BuildResourceCacheSnapshot in internal/runtime/probes.go and passes the
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
