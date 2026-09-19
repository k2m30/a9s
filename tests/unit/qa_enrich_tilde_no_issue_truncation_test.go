// Pins the Truncated=false
// invariant for "~"-only (informational) Wave-2 issue enrichers once the
// input exceeds EnrichmentCap (50).
//
// The menu/list issue badge reads the aggregate Truncated flag as "this count
// is a lower bound" and renders a "+" after it. An enricher that never emits a
// "!" (SevBroken) finding has IssueCount 0, so its per-resource API-call cap
// says nothing about the issue count and must not set Truncated.
package unit

import (
	"context"
	"fmt"
	"testing"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/resource"
)

// tildeOnlyIssueEnricherFunc matches awsclient.IssueEnricherFunc
// (core/aws/issue_enrichment.go), the shared contract every case below
// implements.
type tildeOnlyIssueEnricherFunc func(context.Context, *awsclient.ServiceClients, []resource.Resource, resource.ResourceCache) (awsclient.IssueEnricherResult, error)

// tildeOnlyEnricherCase names one "~"-only enricher under test.
type tildeOnlyEnricherCase struct {
	name string
	fn   tildeOnlyIssueEnricherFunc
}

// tildeOnlyEnricherCases lists every enricher that emits only informational
// "~" findings (IssueCount is always 0) and must therefore never let its own
// EnrichmentCap lower-bound the aggregate Truncated flag.
func tildeOnlyEnricherCases() []tildeOnlyEnricherCase {
	return []tildeOnlyEnricherCase{
		{"athena", awsclient.EnrichAthenaWorkGroup},
		{"eb", awsclient.EnrichEBEnvironmentHealth},
		{"ecs", awsclient.EnrichECSClusters},
		{"elb", awsclient.EnrichELBAttributes},
		{"iam-group", awsclient.EnrichIAMGroup},
		{"iam-role", awsclient.EnrichIAMRoleLastUsed},
		{"logs", awsclient.EnrichLogsMetricFilters},
		{"vpc", awsclient.EnrichVPCFlowLogs},
		{"waf", awsclient.EnrichWAFLogging},
	}
}

// tildeOnlyOverCapResources returns EnrichmentCap+1 (51) minimal synthetic
// resources — one more than every enricher under test's per-call cap. None
// of the enrichers need real Fields[] data to reach their
// `len(resources) > EnrichmentCap` truncation line: that comparison runs
// before any per-resource API call, so a bare ID (used directly by most
// enrichers, or as the fallback when a type-specific Fields[] key is empty)
// is sufficient to drive the invariant under test.
func tildeOnlyOverCapResources() []resource.Resource {
	n := awsclient.EnrichmentCap + 1
	out := make([]resource.Resource, n)
	for i := range out {
		id := fmt.Sprintf("tilde-cap-%03d", i)
		out[i] = resource.Resource{ID: id, Name: id}
	}
	return out
}

// TestTildeOnlyEnrichers_OverEnrichmentCap_TruncatedFalse pins: none of the
// "~"-only enrichers may report Truncated=true purely because the input
// exceeds EnrichmentCap. Their per-resource cap limits informational
// coverage only — it must never lower-bound the ("!"-only) issue count the
// menu/list badge renders.
func TestTildeOnlyEnrichers_OverEnrichmentCap_TruncatedFalse(t *testing.T) {
	clients := demo.NewServiceClients()
	resources := tildeOnlyOverCapResources()
	if len(resources) <= awsclient.EnrichmentCap {
		t.Fatalf("test setup: len(resources) = %d, want > EnrichmentCap (%d)", len(resources), awsclient.EnrichmentCap)
	}

	for _, tc := range tildeOnlyEnricherCases() {
		t.Run(tc.name, func(t *testing.T) {
			result, _ := tc.fn(context.Background(), clients, resources, nil)
			if result.Truncated {
				t.Errorf("%s: Truncated = true for %d resources (cap=%d); want false — \"~\"-only enrichers must never let EnrichmentCap lower-bound the issue count",
					tc.name, len(resources), awsclient.EnrichmentCap)
			}
		})
	}
}

// brokenEmittingEnricherCases lists enrichers that emit at least one "!"
// finding: their per-resource cap bounds the issue count, so the aggregate
// flag must say so.
func brokenEmittingEnricherCases() []tildeOnlyEnricherCase {
	return []tildeOnlyEnricherCase{
		{"apigw", awsclient.EnrichAPIGatewayStage},
		{"cf", awsclient.EnrichCloudFrontDistribution},
		{"msk", awsclient.EnrichMSKCluster},
		{"sns", awsclient.EnrichSNSSubscriptions},
		{"sqs", awsclient.EnrichSQSAttributes},
		{"r53", awsclient.EnrichRoute53Zone},
	}
}

// TestBrokenEmittingEnrichers_OverEnrichmentCap_Truncated is the mirror of
// the test above. An enricher that can emit a "!" finding and inspects only
// the first EnrichmentCap resources has genuinely not seen the rest, so the
// badge must render its count as a lower bound rather than as a total. A
// hard-coded Truncated = false there under-reports the issue badge, and the
// operator reads "no issues" for a queue nobody looked at.
func TestBrokenEmittingEnrichers_OverEnrichmentCap_Truncated(t *testing.T) {
	clients := demo.NewServiceClients()
	resources := tildeOnlyOverCapResources()
	if len(resources) <= awsclient.EnrichmentCap {
		t.Fatalf("test setup: len(resources) = %d, want > EnrichmentCap (%d)", len(resources), awsclient.EnrichmentCap)
	}

	for _, tc := range brokenEmittingEnricherCases() {
		t.Run(tc.name, func(t *testing.T) {
			result, _ := tc.fn(context.Background(), clients, resources, nil)
			if !result.Truncated {
				t.Errorf("%s: Truncated = false for %d resources (cap=%d); want true — this enricher emits a \"!\" finding, so everything past the cap is unexamined and the badge count is a lower bound",
					tc.name, len(resources), awsclient.EnrichmentCap)
			}
		})
	}
}
