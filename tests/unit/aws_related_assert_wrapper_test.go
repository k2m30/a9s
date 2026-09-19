package unit

// After a Ctrl+R re-enrichment on a detail view, RawStruct holds the detail
// enricher's wrapper embedding the SDK struct. assertStruct[T] finds T through
// that embedding; otherwise every checker would read the zero value and the
// related panel would degrade.

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
)

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
