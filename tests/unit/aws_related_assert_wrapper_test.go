package unit

// aws_related_assert_wrapper_test.go — regression coverage for defect (B)
// from the independent Codex + CodeRabbit review: assertStruct[T]
// (core/aws/related_common.go:20) failed to resolve T when RawStruct held a
// detail enricher's wrapper instead of the raw SDK struct, so Ctrl+R after
// enrichment degraded related panels (every checker's assertStruct[T] call
// silently returned the zero value, ok=false).
//
// The fix (findEmbeddedStruct, core/aws/related_common.go) makes
// assertStruct search v's exported anonymous embedded struct fields
// (pointer-deref'd) for a T when the direct/pointer assertions miss.
//
// Chosen approach: a real related-checker path (checkEC2SG, registered as
// the "sg" RelatedDef for "ec2" — Pattern F, no clients/cache needed) rather
// than the engine-level GetDetailEnricher fallback, since this checker's
// plumbing is lightweight enough to call directly. Mirrors the existing
// checker-lookup helper ec2CheckerByTarget and the fixture/assertion style
// of TestEC2RelatedCheckers_NoUnknownCounts (aws_ec2_related_test.go).
//
// May be RED until the parallel assertStruct fix lands — that is the
// expected TDD state, not a bug in this test.

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
)

// TestRelatedChecker_AssertStructOnWrapper_EC2SG verifies that checkEC2SG
// (assertStruct[ec2types.Instance] internally) produces the identical
// count/witness (ResourceIDs) whether res.RawStruct holds the raw
// ec2types.Instance or the on-demand detail enricher's InstanceEnriched
// wrapper embedding it — the shape RawStruct actually holds after a Ctrl+R
// re-enrichment on a detail view.
func TestRelatedChecker_AssertStructOnWrapper_EC2SG(t *testing.T) {
	checker := ec2CheckerByTarget(t, "sg")

	raw := ec2types.Instance{
		InstanceId: aws.String("i-0abc123def4567890"),
		SecurityGroups: []ec2types.GroupIdentifier{
			{GroupId: aws.String("sg-0abc123"), GroupName: aws.String("web-sg")},
			{GroupId: aws.String("sg-0def456"), GroupName: aws.String("db-sg")},
		},
	}

	rawRes := resource.Resource{ID: "i-0abc123def4567890", RawStruct: raw}
	wantResult := checker(context.Background(), nil, rawRes, nil)
	if wantResult.Count() != 2 {
		t.Fatalf("sanity check failed: checker(raw) Count = %d, want 2 — fixture is wrong", wantResult.Count())
	}

	wrapped := awsclient.InstanceEnriched{Instance: raw, UserData: "#!/bin/bash\necho hi\n"}
	wrappedRes := resource.Resource{ID: "i-0abc123def4567890", RawStruct: wrapped}
	gotResult := checker(context.Background(), nil, wrappedRes, nil)

	if gotResult.Count() != wantResult.Count() {
		t.Errorf("checker(InstanceEnriched) Count = %d, want %d (identical to raw ec2types.Instance) — "+
			"assertStruct[ec2types.Instance] must resolve through the wrapper", gotResult.Count(), wantResult.Count())
	}
	if len(gotResult.ResourceIDs()) != len(wantResult.ResourceIDs()) {
		t.Fatalf("checker(InstanceEnriched) ResourceIDs = %v, want %v", gotResult.ResourceIDs(), wantResult.ResourceIDs())
	}
	for i, id := range wantResult.ResourceIDs() {
		if gotResult.ResourceIDs()[i] != id {
			t.Errorf("checker(InstanceEnriched) ResourceIDs[%d] = %q, want %q", i, gotResult.ResourceIDs()[i], id)
		}
	}
}
