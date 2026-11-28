// Pins what an enricher records once its input exceeds EnrichmentCap (50):
// the rows past the cap are marked not inspected. The menu/list issue badge
// reads those marks against the type's catalog severities, so the marks make
// the count a lower bound only for a type that declares a "!" (SevBroken)
// Wave-2 finding; a "~"-only type's count stays exact.
package unit

import (
	"context"
	"fmt"
	"testing"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/domain"
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
// "~" findings (IssueCount is always 0), so its EnrichmentCap never
// lower-bounds the issue count.
func tildeOnlyEnricherCases() []tildeOnlyEnricherCase {
	return []tildeOnlyEnricherCase{
		{"athena", awsclient.EnrichAthenaWorkGroup},
		{"eb", awsclient.EnrichEBEnvironmentHealth},
		{"ecs", awsclient.EnrichECSClusters},
		{"elb", awsclient.EnrichELBAttributes},
		{"logs", awsclient.EnrichLogsMetricFilters},
		{"vpc", awsclient.EnrichVPCFlowLogs},
		{"waf", awsclient.EnrichWAFLogging},
	}
}

// tildeOnlyOverCapResources returns EnrichmentCap+1 (51) minimal synthetic
// resources — one more than every enricher under test's per-call cap. None
// of the enrichers need real Fields[] data to reach their cap: the cap runs
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

// declaresBrokenWave2 reports whether shortName's catalog entry declares a
// "!" Wave-2 finding: the severity the badge's lower bound reads.
func declaresBrokenWave2(t *testing.T, shortName string) bool {
	t.Helper()
	for _, td := range catalog.All() {
		if td.ShortName != shortName {
			continue
		}
		for _, f := range td.Findings {
			if f.Source == "wave2" && f.Severity == domain.SevBroken {
				return true
			}
		}
		return false
	}
	t.Fatalf("%s is not in the catalog", shortName)
	return false
}

// TestTildeOnlyEnrichers_OverEnrichmentCap_TruncatedFalse pins: the type of a
// "~"-only enricher declares no "!" Wave-2 finding, so the rows it leaves
// past EnrichmentCap never lower-bound the issue count the menu/list badge
// renders.
func TestTildeOnlyEnrichers_OverEnrichmentCap_TruncatedFalse(t *testing.T) {
	for _, tc := range tildeOnlyEnricherCases() {
		t.Run(tc.name, func(t *testing.T) {
			if declaresBrokenWave2(t, tc.name) {
				t.Errorf("%s declares a \"!\" Wave-2 finding, so its marks lower-bound the issue count", tc.name)
			}
		})
	}
}

// brokenEmittingEnricherCases lists enrichers that emit at least one "!"
// finding: their per-resource cap bounds the issue count.
func brokenEmittingEnricherCases() []tildeOnlyEnricherCase {
	return []tildeOnlyEnricherCase{
		{"apigw", awsclient.EnrichAPIGatewayStage},
		{"cf", awsclient.EnrichCloudFrontDistribution},
		{"msk", awsclient.EnrichMSKCluster},
		{"sns", awsclient.EnrichSNSSubscriptions},
		{"sqs", awsclient.EnrichSQSAttributes},
		{"r53", awsclient.EnrichRoute53Zone},
		{"iam-group", awsclient.EnrichIAMGroup},
		{"role", awsclient.EnrichIAMRoleLastUsed},
	}
}

// TestBrokenEmittingEnrichers_OverEnrichmentCap_Truncated is the mirror of
// the test above. An enricher that can emit a "!" finding and inspects only
// the first EnrichmentCap resources has genuinely not seen the rest: it marks
// them, and its type declares a "!" Wave-2 finding, so the badge renders its
// count as a lower bound rather than as a total.
func TestBrokenEmittingEnrichers_OverEnrichmentCap_Truncated(t *testing.T) {
	clients := demo.NewServiceClients()
	resources := tildeOnlyOverCapResources()
	if len(resources) <= awsclient.EnrichmentCap {
		t.Fatalf("test setup: len(resources) = %d, want > EnrichmentCap (%d)", len(resources), awsclient.EnrichmentCap)
	}

	for _, tc := range brokenEmittingEnricherCases() {
		t.Run(tc.name, func(t *testing.T) {
			result, _ := tc.fn(context.Background(), clients, resources, nil)
			if len(result.TruncatedIDs) == 0 {
				t.Errorf("%s: no row marked not inspected for %d resources (cap=%d)", tc.name, len(resources), awsclient.EnrichmentCap)
			}
			if !declaresBrokenWave2(t, tc.name) {
				t.Errorf("%s declares no \"!\" Wave-2 finding, so its marks cannot lower-bound the issue count", tc.name)
			}
		})
	}
}
