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
func NoOpIssueEnricher(_ context.Context, _ *ServiceClients, _ []resource.Resource) (IssueEnricherResult, error) {
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
