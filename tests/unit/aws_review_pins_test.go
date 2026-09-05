// aws_review_pins_test.go — pinning tests added in response to code review.
// Each test pins a specific invariant that previously had no regression guard.
// They must fail before the corresponding fix lands (red-first) and pass after.
package unit

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"

	"github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/resource"
)

// ---------------------------------------------------------------------------
// #9 PIN — efs SetFieldKeysForTest must include every key the fetcher writes.
//
// The fetcher populates Fields["throughput_mode"] (efs.go:152) but the initial
// SetFieldKeysForTest list at efs.go:15 omitted it. This makes the key invisible
// to tooling that enumerates the registered keys (viewsgen, YAML merging).
// ---------------------------------------------------------------------------

func TestEFS_RegisterFieldKeys_IncludesThroughputMode(t *testing.T) {
	keys := resource.GetFieldKeys("efs")
	for _, k := range keys {
		if k == "throughput_mode" {
			return
		}
	}
	t.Fatalf("SetFieldKeysForTest(\"efs\") missing %q — fetcher writes Fields[%q] but it is not registered; keys=%v", "throughput_mode", "throughput_mode", keys)
}

// ---------------------------------------------------------------------------
// #10 PIN — EFS mount-target ENI Groups[].GroupName must match the GroupName
// on the SecurityGroup fixtures with the same GroupId.
//
// The ENI fixtures for ProdEFSSecurityGroupA/B were literals "efs-prod-app-data-sg-a/b"
// while buildSecurityGroups emitted GroupName="acme-efs-prod-sg-a/b". A name
// mismatch for the same GroupId is a self-inconsistent graph.
// ---------------------------------------------------------------------------

func TestEFS_FixtureENIGroupNamesMatchSecurityGroups(t *testing.T) {
	fix := fixtures.NewEC2Fixtures()

	// Build GroupId → GroupName map from SecurityGroup fixtures.
	sgNames := make(map[string]string)
	for _, sg := range fix.SecurityGroups {
		if sg.GroupId != nil && sg.GroupName != nil {
			sgNames[*sg.GroupId] = *sg.GroupName
		}
	}

	// For each ENI that references an EFS prod SG, its GroupName must match.
	checkIDs := map[string]bool{
		fixtures.ProdEFSSecurityGroupAID: true,
		fixtures.ProdEFSSecurityGroupBID: true,
	}

	for _, eni := range fix.NetworkInterfaces {
		for _, g := range eni.Groups {
			if g.GroupId == nil || g.GroupName == nil {
				continue
			}
			if !checkIDs[*g.GroupId] {
				continue
			}
			want, ok := sgNames[*g.GroupId]
			if !ok {
				continue
			}
			if *g.GroupName != want {
				t.Errorf("ENI %s references SG %s with GroupName=%q, but SecurityGroup fixture has GroupName=%q — fixtures are self-inconsistent",
					aws.ToString(eni.NetworkInterfaceId), *g.GroupId, *g.GroupName, want)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// #11 PIN — DELETED by AS-140.
//
// The original test pinned the EnrichEFSMountTargets suffix-bump idempotency
// invariant against the FieldUpdates["status"] write path. AS-140 removed
// that write entirely (the merged "mount target down (+N)" phrase is now
// computed at render time by phraseFromFindings(r.Findings) in
// internal/tui/views/table_render.go), making the bug structurally
// impossible: the enricher no longer computes or stores the merged phrase.
// This is parallel to the QA deletion of snapshot_cross_ref_internal_test.go
// (which pinned the now-deleted computeMergedStatus helper).
//
// The new structural invariant ("FieldUpdates is empty after enrichment") is
// pinned by the AS-140 assertions in tests/unit/aws_efs_issue_enrichment_test.go.
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// The Redshift phrase-probe pin is gone. It asserted that the Color func could
// classify a resource carrying nothing but Fields["status"], because the
// unified Attention renderer used to probe severity by handing the classifier a
// synthetic phrase. Severity now travels on the finding that produced the
// phrase, so there is no probe to answer and no phrase table to keep in step
// with the fetcher. The status to colour mapping it stood in for is pinned
// against the fetcher in qa_redshift_color_test.go.
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// P2 PIN — OpenSearch enricher must NOT emit a finding for a Deleted domain
// even when UpdateAvailable is true. A deleted domain's pending update is not
// actionable; emitting it would contaminate the unified S1 menu badge count.
// ---------------------------------------------------------------------------

// The deleted-domain guard moved to the fetcher with the checks it guarded, so
// TestOpenSearch_Enrich_DeletedDomain_SkipsFinding is deleted rather than
// inverted: TestOpenSearch_Fetch_DeletedPlusBackgroundBackgroundSuppressed
// (aws_opensearch_test.go) pins the same fact, anchor included, on the surface
// that now decides it.
