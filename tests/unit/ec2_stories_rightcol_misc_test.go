package unit_test

// ec2_stories_rightcol_misc_test.go — shared EC2 resource fixture for the
// EC2 QA stories (Issue #119/#140) that still need a hand-built EC2 resource
// (rather than a demo fixture). The former DetailModel-backed helpers
// (ec2StoryDetail/deliverRelatedResult/pressDetailKey) were retired
// (round 5, specs/022-codebase-cleanup): all callers now drive the live
// Controller/RenderDetail seam via makePreviewEC2Detail/previewDetailView
// (left_column_preview_regressions_test.go) instead.

import (
	"github.com/k2m30/a9s/v3/core/resource"
)

// ec2StoryResource returns the synthetic EC2 instance shared by the EC2
// right-column QA stories (Issue #119/#140).
func ec2StoryResource() resource.Resource {
	return resource.Resource{
		ID:   "i-0a1b2c3d4e5f60001",
		Name: "web-prod-01",
		Fields: map[string]string{
			"InstanceId":   "i-0a1b2c3d4e5f60001",
			"VpcId":        "vpc-0abc123def456789a",
			"SubnetId":     "subnet-0aaa111111111111a",
			"ImageId":      "ami-0abc123def456789a",
			"InstanceType": "t3.large",
			"State":        "running",
		},
	}
}
