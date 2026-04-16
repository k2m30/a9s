package aws

import (
	"context"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// checkAMIEC2 checks the cache for EC2 instances launched from this AMI.
// Pattern C: matches ec2.Fields["image_id"] against the AMI's ID.
func checkAMIEC2(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	amiID := res.ID
	if amiID == "" {
		return resource.RelatedCheckResult{TargetType: "ec2", Count: 0}
	}

	ec2List, truncated, err := amiRelatedResources(ctx, clients, cache, "ec2")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "ec2", Count: -1, Err: err}
	}
	if ec2List == nil {
		return resource.RelatedCheckResult{TargetType: "ec2", Count: -1}
	}

	var ids []string
	for _, ec2Res := range ec2List {
		if ec2Res.Fields["image_id"] == amiID {
			ids = append(ids, ec2Res.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "ec2", Count: -1}
	}
	return relatedResult("ec2", ids)
}

// checkAMIEBSSnaps reads the backing snapshot IDs from the AMI's block device mappings.
// Pattern F: data is in RawStruct, no cache lookup needed.
func checkAMIEBSSnaps(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	img, ok := assertStruct[ec2types.Image](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "ebs-snap", Count: -1}
	}

	var ids []string
	for _, bdm := range img.BlockDeviceMappings {
		if bdm.Ebs != nil && bdm.Ebs.SnapshotId != nil && *bdm.Ebs.SnapshotId != "" {
			ids = append(ids, *bdm.Ebs.SnapshotId)
		}
	}
	if len(ids) == 0 {
		return resource.RelatedCheckResult{TargetType: "ebs-snap", Count: 0}
	}
	return relatedResult("ebs-snap", ids)
}

// checkAMIASG returns Count: 0 because Auto Scaling Groups referencing a specific
// AMI via launch templates or launch configs cannot be determined from the cache
// list alone — LaunchTemplate/MixedInstancesPolicy do not expose AMI IDs in the
// DescribeAutoScalingGroups summary.
func checkAMIASG(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "asg", Count: 0}
}

func checkAMICFN(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "cfn", Count: 0}
}

func checkAMING(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "ng", Count: 0}
}

// amiRelatedResources returns the cached resource list for the given target type,
// or fetches the first page via the registered paginated fetcher.
func amiRelatedResources(ctx context.Context, clients any, cache resource.ResourceCache, target string) ([]resource.Resource, bool, error) {
	resources, isTruncated, err := FetchRelatedTarget(ctx, clients, cache, target)
	if err != nil {
		if _, ok := clients.(*ServiceClients); !ok {
			return nil, false, nil
		}
	}
	return resources, isTruncated, err
}
