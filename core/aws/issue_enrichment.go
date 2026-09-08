// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// issue_enrichment.go owns Wave 2 issue-enrichment shared types and helpers:
// the IssueEnricher metadata struct, InFetcherWave2Sentinel, the result/func
// contracts, and truly-shared helpers used by more than one enricher file.
//
// Wave 2 enricher registrations live as the Wave2 field on each
// catalog.ResourceTypeDef literal in the catalog_<category>.go files. Read
// access goes through awsclient.Wave2EnricherFor / awsclient.AllWave2 in
// wave2.go.
package aws

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// IssueEnricher carries a Wave 2 IssueEnricherFunc plus scheduling metadata.
// Priority controls Wave 2 dispatch order: lower values run first.
// The default priority is 100; batchable (cheap) enrichers use 10.
//
// Distinct from resource.DetailEnricher (core/resource/enricher.go) which
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
// InFetcherWave2Sentinel, Priority: 100}` marks the type as Wave-2-covered in
// the catalog without doing real background work — the sentinel returns zero
// findings. No gate enforces the sentinel's presence: the doc-sync guards
// (`make check-catalogen` and tests/unit/docs_attention_signals_sync_test.go)
// track FindingDef declarations, not the Wave2 field, so this wiring holds by
// convention only.
//
// Resource types whose Wave 2 column is "None" in docs/attention-signals.md
// must omit the Wave2 field entirely; this sentinel is reserved for the
// in-fetcher case. Returns zero findings, zero issues, not truncated, never
// fails. Tests use it as a benign Fn fixture too.
func InFetcherWave2Sentinel(_ context.Context, _ *ServiceClients, _ []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	return IssueEnricherResult{
		Findings:         map[string][]domain.Finding{},
		AttentionDetails: map[string]map[domain.FindingCode]domain.AttentionDetail{},
		TruncatedIDs:     map[string]bool{},
		FieldUpdates:     map[string]map[string]string{},
		Truncated:        false,
	}, nil
}

// EnrichmentCap is the maximum number of per-resource API calls for non-batchable enrichers.
const EnrichmentCap = 50

// EnrichmentParallelism bounds concurrent per-resource API calls in Wave-2
// enrichers that fan out via aws.ForEachParallel. Kept well under typical AWS
// service-side throttling limits (RetryOnThrottle still wraps each call as a
// second line of defense against bursts); 8 gives a meaningful wall-clock
// speedup over a sequential loop without materially increasing the odds of
// tripping per-second rate limits on accounts with default quotas.
const EnrichmentParallelism = 8

// PerParentPageCap limits per-parent pagination walks in enrichers to avoid
// runaway enumeration on huge tenants. When hit, the emitted count is marked
// with a "+" suffix to signal truncated.
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

// Wave2EmissionObserver, when non-nil, receives every Finding setWave2Finding
// builds, before the same-code collapse below keeps only the first emission of
// a code. Nothing in the app installs one — it is the only seam from which the
// phrase/severity/detail a second emission of a (resourceID, code) carries is
// observable at all, since the collapse discards it.
//
// An observer must be safe for concurrent use: enrichers emit from
// ForEachParallel workers, and not all of them hold a lock across the call.
var Wave2EmissionObserver func(resourceID string, f domain.Finding)

// setWave2Finding writes a Wave-2 Finding + AttentionDetail pair into the
// IssueEnricherResult maps. Every enricher constructs its emission via this
// helper rather than re-implementing the glyph→Severity mapping and the
// AttentionDetail{Rows: …} packing in every file.
//
// The wording is the one the code's catalog.FindingDef declares; values fill
// its "<…>" slots left to right, so an enricher measuring a count passes the
// count and never a sentence. The S5 detail sentence comes from the same
// declaration.
//
// rows MAY be nil; the helper omits the AttentionDetail entry when empty so a
// nil-row finding does not surface an empty Attention section.
//
// Append-style: calling this a second time for the same resourceID with a
// DIFFERENT code (an enricher with two independently-evaluated conditions on
// the same resource, e.g. opensearch's update-forced + encryption-off)
// appends the new Finding to r.Findings[resourceID] rather than overwriting
// it — every independently-evaluated condition survives as its own Finding,
// with its own Phrase/Detail/Code, never demoted into another finding's
// supporting row. ApplyWave2ToRow (core/runtime/helpers.go) folds the whole
// slice onto domain.Resource.Findings, and colorFromAnyFinding/
// buildAttentionEntries/domain.StatusPhrase already reads the whole slice for
// worst-severity color, one Attention entry per issue-severity finding, and
// the stacked "<top> (+N)" list phrase respectively.
//
// Calling it again with the SAME code is one condition found on a second item
// of the same resource — a second stage of one API, a second container of one
// task. The resource states that condition once, so the Finding is not
// repeated, and the new rows are appended to the (resourceID, code) entry, the
// way addWave1Rows appends. Assigning instead would leave the reader told
// about the last offending item with nothing to say the others were inspected.
//
// rows, when non-empty, accumulate on this (resourceID, code) pair's
// AttentionDetail entry — keyed by the Finding's own Code, so a second
// independently-evaluated condition on the same resourceID (its own Code)
// records its own rows without disturbing the first condition's entry.
// ApplyWave2ToRow looks up each appended Finding's AttentionDetail by
// (resourceID, Code), so every condition keeps its own supporting rows.
//
// The caller is responsible for initialising r.Findings and (when emitting
// rows) r.AttentionDetails before calling this helper. The IssueEnricherResult
// godoc requires both reference fields be non-nil on a successful return.
func setWave2Finding(
	r *IssueEnricherResult,
	resourceID string,
	code domain.FindingCode,
	rows []domain.DetailRow,
	values ...string,
) {
	f := wave2Finding(code, values...)
	if Wave2EmissionObserver != nil {
		Wave2EmissionObserver(resourceID, f)
	}
	if !slices.ContainsFunc(r.Findings[resourceID], func(g domain.Finding) bool { return g.Code == code }) {
		r.Findings[resourceID] = append(r.Findings[resourceID], f)
	}

	if len(rows) > 0 {
		if r.AttentionDetails == nil {
			r.AttentionDetails = make(map[string]map[domain.FindingCode]domain.AttentionDetail)
		}
		if r.AttentionDetails[resourceID] == nil {
			r.AttentionDetails[resourceID] = make(map[domain.FindingCode]domain.AttentionDetail, 1)
		}
		ad := r.AttentionDetails[resourceID][code]
		ad.Rows = capRows(ad.Rows, rows)
		r.AttentionDetails[resourceID][code] = ad
	}
}

// wave2Finding builds a Wave-2 Finding carrying the phrase and detail its
// code declares. It is the Wave-2 counterpart of wave1Finding: the two are the
// only places a domain.Finding is constructed, so a wording, a sentence and a
// Source string each have one owner.
//
// Source is the bare provenance class. The type-qualified form the readers
// test for ("wave2:<short>") is stamped by runtime.ApplyWave2ToRow, which
// holds the registry entry the result is being merged under and is therefore
// the only thing that knows which type's enricher ran. values fill the
// declared phrase's "<…>" slots left to right, and the severity is the code's.
func wave2Finding(code domain.FindingCode, values ...string) domain.Finding {
	return domain.Finding{
		Code:     code,
		Phrase:   fillPhrase(catalog.Phrase(code), values...),
		Detail:   catalog.Detail(code),
		Severity: catalog.Severity(code),
		Source:   "wave2",
	}
}

// MarkSkipped records one item from a failed batch call: it sets
// result.TruncatedIDs[id] so the row renders "?" instead of vanishing, and
// records the failure for Finish to fold into the caller's composite error.
// Every "!"-severity Wave 2 enricher that iterates batched AWS calls
// (DescribeTasks, DescribeServices, …) MUST call this once per item in a
// failed batch — recording only the aggregate Truncated flag drops the
// per-row signal the list view needs to distinguish "not inspected" from
// "inspected and healthy".
func MarkSkipped(result *IssueEnricherResult, id string, failures *[]Failure, err error) {
	result.TruncatedIDs[id] = true
	*failures = append(*failures, FailedCall(id, err))
}

// MarkUnusable records an item the service answered for without the field the
// enricher needs — a summary the response omitted, a name the batch did not
// come back with. There is no error to classify, so a9s states the cause
// itself; the row is uninspected either way.
func MarkUnusable(result *IssueEnricherResult, id string, failures *[]Failure, cause string) {
	result.TruncatedIDs[id] = true
	*failures = append(*failures, UnusableAnswer(id, cause))
}

// SetTruncated raises result.Truncated when cut is true and never lowers it.
//
// One enricher discovers a cut answer in several independent places — a failed
// batch, a work list trimmed at EnrichmentCap, a walk stopped at the page cap,
// a second pass over the same rows — and each of them runs whether or not the
// others did. A bare assignment composes with none of them: the last pass to
// finish cleanly erases what an earlier one found. This is the only writer
// that raises the flag; MarkInformationalOnly is the only one that lowers it.
func SetTruncated(result *IssueEnricherResult, cut bool) {
	if cut {
		result.Truncated = true
	}
}

// MarkInformationalOnly declares that an enricher's whole answer is "~":
// capping or cutting its walk hides no issue, only informational coverage, so
// the issue count it contributes to is not a lower bound and the flag every
// cap along the way raised is dropped. Call it last — a later pass that emits
// "!" would have nothing to raise the flag with afterwards.
//
// Per-resource truncation is unaffected: TruncatedIDs still says which rows
// were not looked at.
func MarkInformationalOnly(result *IssueEnricherResult) {
	result.Truncated = false
}

// markAllUninspected records that a single account-wide call answered for
// every row and failed, so none of them was inspected. Without it the rows
// render as inspected-and-healthy, which is the one thing a failed check must
// never claim. Enrichers whose calls are per-item use MarkSkipped instead —
// only the row whose own call failed is uninspected there.
func markAllUninspected(result *IssueEnricherResult, resources []resource.Resource) {
	SetTruncated(result, true)
	for _, r := range resources {
		if r.ID != "" {
			result.TruncatedIDs[r.ID] = true
		}
	}
}

// resourceIDsOf is capAtEnrichmentCap's idsOf for the common case: a work list
// of resources, where inspecting one item decides exactly its own row.
func resourceIDsOf(r resource.Resource) []string { return []string{r.ID} }

// capAtEnrichmentCap trims a per-item work list to EnrichmentCap and records
// every row the dropped items would have answered for as uninspected. A cap is
// a limit on what a9s looked at, so the rows past it are "?" and never clean.
//
// idsOf maps one work item to the resource IDs its inspection decides, which is
// the item's own ID for a list of resources and every task on a definition for
// a list grouped by a shared key.
func capAtEnrichmentCap[T any](result *IssueEnricherResult, items []T, idsOf func(T) []string) []T {
	if len(items) <= EnrichmentCap {
		return items
	}
	for _, item := range items[EnrichmentCap:] {
		for _, id := range idsOf(item) {
			if id != "" {
				result.TruncatedIDs[id] = true
			}
		}
	}
	SetTruncated(result, true)
	return items[:EnrichmentCap]
}

// Finish folds a Wave 2 enricher's accumulated per-batch failures into result
// and returns the composite error via AggregateFailures. It raises Truncated
// through SetTruncated, so a call with zero failures composes safely with a
// flag another pass raised earlier in the same enricher (EnrichmentCap, the
// page cap, a per-parent cap).
func Finish(result *IssueEnricherResult, failures []Failure, total int, op string) error {
	SetTruncated(result, len(failures) > 0)
	return AggregateFailures(op, failures, total)
}

// IssueEnricherResult is the typed return value of a Wave 2 issue enricher.
//
//   - Truncated: true when the ISSUE count (severity "!") is a lower bound —
//     the enricher's walk was cut short (EnrichmentCap/page cap/API errors) AND
//     this enricher can emit "!" findings. Enrichers that emit only "~"
//     informational findings MUST leave Truncated false: capping their walk
//     hides no issues, only informational coverage. Use TruncatedIDs for
//     per-resource "?" coverage gaps regardless of severity.
//
//   - TruncatedIDs: per-resource truncation. Key = Resource.ID that could not be
//     fully inspected (API error on that resource, page cap hit during a
//     per-parent paginated walk, etc.). The UI renders "?" on just that row
//     instead of a global banner. An ID appearing here MUST NOT also appear in
//     Findings unless the partial data was still usable.
//
//   - Findings: map from Resource.ID → []domain.Finding. The enricher emits
//     one entry per independently-evaluated Wave-2 condition for the
//     resource (a single-condition enricher always emits a one-element
//     slice). May contain entries for resources NOT in the input slice
//     (account-wide enrichers). Enrichers that receive API identifiers in a
//     different form (e.g., ARNs) MUST normalize to Resource.ID before
//     writing to Findings.
//
//   - AttentionDetails: per-resource, per-Code supporting rows for the Wave-2
//     Finding(s), keyed by Resource.ID then by the owning Finding's Code.
//     Only entries with non-empty rows are emitted. Keying by Code (rather
//     than Resource.ID alone) lets each independently-evaluated condition on
//     the same resource carry its own supporting rows without one condition's
//     rows crowding out another's.
//
//   - FieldUpdates: map from Resource.ID → (fieldKey → value). Same normalization
//     rule applies.
//
// MAY have empty maps but MUST NOT be nil for any reference field on
// success — initialize each with `make(...)` before returning.
type IssueEnricherResult struct {
	Truncated    bool
	TruncatedIDs map[string]bool
	Findings     map[string][]domain.Finding
	// AttentionDetails carries per-resource, per-Code supporting rows for the
	// Wave-2 Finding(s) emitted in Findings. Keyed by Resource.ID then by the
	// owning Finding's Code — the target shape domain.Resource.AttentionDetails
	// already uses one layer down, threaded up through this result so every
	// independently-evaluated condition on a resource keeps its own rows.
	// Enrichers MAY omit an entry when no rows accompany a given finding.
	AttentionDetails map[string]map[domain.FindingCode]domain.AttentionDetail
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
// (core/resource/enricher.go) which enriches a single resource for detail views.
//
// Cache invariant — read-only shallow snapshot:
// The TUI dispatcher (internal/tui/probe_adapter.go probeEnrichment tea.Cmd wrapper)
// invokes (*Core).ProbeEnrichment, which builds the cache once at dispatch time via
// (*Core).BuildResourceCacheSnapshot in core/runtime/probes.go and passes the
// resulting map by value. The map and its ResourceCacheEntry structs are
// freshly allocated, but the .Resources slice header is COPIED — its backing
// array is shared with the live m.resourceCache / m.probeResources / m.lazyResourceCache
// state. Enrichers MUST treat the cache as read-only:
//   - DO NOT append to cache[k].Resources (would mutate the shared backing array
//     when len < cap, surfacing as phantom rows in the live view).
//   - DO NOT mutate fields on cache[k].Resources[i] or cache[k].Resources[i].RawStruct
//     (those are pointers / interface values shared with the running app).
//   - DO read field values, lengths, and IsTruncated freely.
//
// Violations are not currently caught at compile time. Future contributors
// who need to derive a mutable view should append([]Resource{}, slice...) into
// a local slice first.
type IssueEnricherFunc func(ctx context.Context, clients *ServiceClients, resources []resource.Resource, cache resource.ResourceCache) (IssueEnricherResult, error)

// FindingRowCap bounds the supporting rows one finding shows in the detail
// Attention section. A target group can report hundreds of unhealthy targets
// and a node group dozens of health issues; without a bound one noisy
// condition pushes the rest of the resource's posture off the section.
const FindingRowCap = 10

// overflowRowFormat renders the count of rows a capped finding did not show.
const overflowRowFormat = "… +%d more"

// capRows appends incoming to the rows a finding already carries, holding the
// result at FindingRowCap rows of content plus one closing "… +K more" row. It
// is the one place the bound is applied: both sinks call it and no builder
// caps on its own.
//
// The closing row carries no label. It is not a supporting row but the
// statement that there are more of them, and under the label above it — the
// backup detail's "State", the target group's "Unhealthy target" — it reads
// as one more member of the list, a job whose state is that text. It keeps
// the last kept row's tier, so it is coloured with the list it closes.
//
// A second call for the same (resource, code) — a second pipeline stage, a
// second container of one task — reads K back out of the closing row, which
// is where the count already lives, so the cap holds over the combined rows
// and K keeps counting everything not shown.
func capRows(kept, incoming []domain.DetailRow) []domain.DetailRow {
	hidden := 0
	if n := len(kept); n > 0 {
		// Only a value capRows itself wrote is a stored count. Sscanf stops at
		// the last verb and ignores whatever follows, so "… +3 more replicas"
		// would parse as 3 and swallow the row it belongs to; rendering the
		// parse back and requiring the whole value to match rejects that, and
		// requiring a positive count rejects "… +0 more" and "… +-2 more",
		// which round-trip but which no closing row is ever written for.
		var k int
		if _, err := fmt.Sscanf(kept[n-1].Value, overflowRowFormat, &k); err == nil &&
			k > 0 && fmt.Sprintf(overflowRowFormat, k) == kept[n-1].Value {
			hidden, kept = k, kept[:n-1]
		}
	}
	for _, row := range incoming {
		if len(kept) < FindingRowCap {
			kept = append(kept, row)
			continue
		}
		hidden++
	}
	if hidden == 0 {
		return kept
	}
	return append(kept, domain.DetailRow{
		Value: fmt.Sprintf(overflowRowFormat, hidden),
		Tier:  kept[len(kept)-1].Tier,
	})
}

// walkAccountPages runs an account-wide paginated walk bounded at
// EnrichmentCap pages and returns everything the walked pages carried.
//
// next reads one page for the given token and returns that page's items and
// the token of the page after it; a nil or empty token ends the walk, as does
// a page that fails. Between them they are the whole loop, because the bound
// is only half the rule and an enricher that writes the bound out itself
// writes the visible half only.
//
// The other half is the rows the walk never reached. A cap limits what a9s
// looked at, not what exists, so when the walk ends early every resource no
// walked page named is recorded as uninspected: its answer sat on a page
// nobody read, and such a row must not render as inspected-and-healthy. idOf
// maps one item to the resource ID it answers for, and returns "" for an item
// that answers for no row of this type.
//
// The aggregate Truncated flag stays with the caller: a walk that can hide
// only informational coverage lower-bounds no issue count.
func walkAccountPages[T any](
	result *IssueEnricherResult,
	resources []resource.Resource,
	idOf func(T) string,
	next func(token *string) ([]T, *string, error),
) (items []T, pages int, cut bool, err error) {
	seen := make(map[string]bool, len(resources))
	var token *string
	for pages < EnrichmentCap {
		var page []T
		page, token, err = next(token)
		pages++
		if err != nil {
			break
		}
		items = append(items, page...)
		for _, item := range page {
			if id := idOf(item); id != "" {
				seen[id] = true
			}
		}
		if token == nil || *token == "" {
			return items, pages, false, nil
		}
	}
	for _, r := range resources {
		if r.ID != "" && !seen[r.ID] {
			result.TruncatedIDs[r.ID] = true
		}
	}
	return items, pages, true, err
}
