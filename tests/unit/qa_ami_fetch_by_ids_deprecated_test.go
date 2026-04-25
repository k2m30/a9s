package unit

// qa_ami_fetch_by_ids_deprecated_test.go — Regression pin for FetchAMIsByIDs
// including deprecated AMIs (Group I).
//
// Contract (internal/aws/ami.go:96-99):
//   FetchAMIsByIDs must pass IncludeDeprecated=true in the DescribeImages call.
//   Without this flag, deprecated AMIs silently vanish from batch lookups — a
//   drill from a related-panel pivot (ec2→ami, asg→ami) that references a
//   deprecated AMI would land on an empty list.
//
// This test is a REGRESSION PIN — the fix was already present at the commit
// this test was written against (1ca32ee). It guards against future regressions
// where IncludeDeprecated is accidentally removed.

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
)

// TestFetchAMIsByIDs_IncludeDeprecated verifies that FetchAMIsByIDs passes
// IncludeDeprecated=true to DescribeImages. This ensures deprecated AMIs
// referenced by related-panel pivots (ec2→ami, asg→ami) are not silently
// dropped from batch drill results.
//
// Fails if IncludeDeprecated is removed or set to false in the production call.
func TestFetchAMIsByIDs_IncludeDeprecated(t *testing.T) {
	captured := &capturingDescribeImagesClient{
		output: &ec2.DescribeImagesOutput{
			Images: []ec2types.Image{
				{
					ImageId:      aws.String("ami-0deprecated1111111"),
					Name:         aws.String("deprecated-ami"),
					State:        ec2types.ImageStateAvailable,
					Architecture: ec2types.ArchitectureValuesX8664,
				},
			},
		},
	}

	resources, err := awsclient.FetchAMIsByIDs(context.Background(), captured, []string{"ami-0deprecated1111111"})
	if err != nil {
		t.Fatalf("FetchAMIsByIDs: unexpected error: %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("FetchAMIsByIDs: expected 1 resource, got %d", len(resources))
	}

	if len(captured.inputs) == 0 {
		t.Fatal("FetchAMIsByIDs: no DescribeImages call captured")
	}
	input := captured.inputs[0]

	// CONTRACT ASSERTION: IncludeDeprecated must be non-nil and true.
	// This test fails if the flag is accidentally removed or set to false.
	if input.IncludeDeprecated == nil {
		t.Error("FetchAMIsByIDs: DescribeImages called without IncludeDeprecated flag — " +
			"deprecated AMIs will silently vanish from batch drill results (regression)")
	} else if !*input.IncludeDeprecated {
		t.Errorf("FetchAMIsByIDs: IncludeDeprecated = false, want true — " +
			"deprecated AMIs referenced by related-panel pivots must be included")
	}
}

// TestFetchAMIsByIDs_DeprecatedAMIReturnedInResults verifies end-to-end that
// a deprecated AMI ID passed to FetchAMIsByIDs is resolved and returned as a
// resource. Without IncludeDeprecated=true this would fail silently (empty
// results + missing-ID error).
func TestFetchAMIsByIDs_DeprecatedAMIReturnedInResults(t *testing.T) {
	const deprecatedID = "ami-0deprecated2222222"

	mock := &capturingDescribeImagesClient{
		output: &ec2.DescribeImagesOutput{
			Images: []ec2types.Image{
				{
					ImageId:      aws.String(deprecatedID),
					Name:         aws.String("old-baked-ami"),
					State:        ec2types.ImageStateAvailable,
					Architecture: ec2types.ArchitectureValuesArm64,
				},
			},
		},
	}

	resources, err := awsclient.FetchAMIsByIDs(context.Background(), mock, []string{deprecatedID})
	if err != nil {
		t.Fatalf("FetchAMIsByIDs: unexpected error for deprecated AMI: %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("FetchAMIsByIDs: deprecated AMI not in results — got %d resources; " +
			"want 1 (IncludeDeprecated must be true)", len(resources))
	}
	if resources[0].ID != deprecatedID {
		t.Errorf("FetchAMIsByIDs: result ID = %q, want %q", resources[0].ID, deprecatedID)
	}
}
