// qa_enrich_tilde_no_issue_truncation_test.go — pins the Truncated=false
// invariant for "~"-only (informational) Wave-2 issue enrichers once the
// input exceeds EnrichmentCap (50).
//
// Live bug this guards against: the menu/list issue badge renders an exact
// resource total alongside a spurious "+" (e.g. "CloudWatch Log Groups (96)
// issues:61+") even though all 96 rows loaded successfully. Root cause: the
// 18 enrichers below never emit a "!" (SevBroken) finding — IssueCount is
// always 0 — yet they still compute Truncated as len(resources) >
// EnrichmentCap, tying the aggregate Truncated flag (read by the badge as
// "this count is a lower bound") to their own per-resource API-call cap
// instead of to the issue count. EnrichSESAccount
// (core/aws/ses_issue_enrichment.go) already gets this right by setting
// Truncated = false unconditionally; the 18 enrichers here must match it.
//
// Each subtest drives one enricher with EnrichmentCap+1 (51) synthetic
// resources and asserts result.Truncated == false. Finding/IssueCount
// content is deliberately not asserted here — this file pins only the
// aggregate Truncated flag.
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
//
// ddb and dbi are deliberately absent: their enrichers now also emit "!"
// findings (an open resource policy, a deprecated engine version), so a capped
// walk really can hide an issue and Truncated is the correct answer for them.
func tildeOnlyEnricherCases() []tildeOnlyEnricherCase {
	return []tildeOnlyEnricherCase{
		{"apigw", awsclient.EnrichAPIGatewayStage},
		{"athena", awsclient.EnrichAthenaWorkGroup},
		{"cf", awsclient.EnrichCloudFrontDistribution},
		{"eb", awsclient.EnrichEBEnvironmentHealth},
		{"ecs", awsclient.EnrichECSClusters},
		{"elb", awsclient.EnrichELBAttributes},
		{"iam-group", awsclient.EnrichIAMGroup},
		{"iam-role", awsclient.EnrichIAMRoleLastUsed},
		// kms is deliberately absent: EnrichKMSRotation now also reports an
		// open key policy, an issue-severity finding, so its cap genuinely
		// lower-bounds the issue count and Truncated must be allowed to rise.
		{"logs", awsclient.EnrichLogsMetricFilters},
		{"msk", awsclient.EnrichMSKCluster},
		{"r53", awsclient.EnrichRoute53Zone},
		{"sns", awsclient.EnrichSNSSubscriptions},
		{"sqs", awsclient.EnrichSQSAttributes},
		{"vpc", awsclient.EnrichVPCFlowLogs},
		{"waf", awsclient.EnrichWAFLogging},
	}
}

// tildeOnlyOverCapResources returns EnrichmentCap+1 (51) minimal synthetic
// resources — one more than every enricher under test's per-call cap. None
// of the 18 enrichers need real Fields[] data to reach their
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
// 18 "~"-only enrichers may report Truncated=true purely because the input
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
