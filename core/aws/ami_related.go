// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"

	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// checkAMIEC2 checks the cache for EC2 instances launched from this AMI.
// Pattern C: matches ec2.Fields["image_id"] against the AMI's ID.
func checkAMIEC2(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	amiID := res.ID
	if amiID == "" {
		return resource.KnownRelated("ec2", nil, false)
	}

	ec2List, truncated, err := relatedResourcesFor(ctx, clients, cache, "ec2")
	if err != nil {
		return resource.ErrorRelated("ec2", err)
	}
	if ec2List == nil {
		return resource.UnknownRelated("ec2")
	}

	var ids []string
	for _, ec2Res := range ec2List {
		if ec2Res.Fields["image_id"] == amiID {
			ids = append(ids, ec2Res.ID)
		}
	}
	return relatedResultTrunc("ec2", ids, truncated)
}

// checkAMIEBSSnaps reads the backing snapshot IDs from the AMI's block device mappings.
// Pattern F: data is in RawStruct, no cache lookup needed.
func checkAMIEBSSnaps(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	img, ok := assertStruct[ec2types.Image](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("ebs-snap")
	}

	var ids []string
	for _, bdm := range img.BlockDeviceMappings {
		if bdm.Ebs != nil && bdm.Ebs.SnapshotId != nil && *bdm.Ebs.SnapshotId != "" {
			ids = append(ids, *bdm.Ebs.SnapshotId)
		}
	}
	if len(ids) == 0 {
		return resource.KnownRelated("ebs-snap", nil, false)
	}
	return relatedResult("ebs-snap", ids)
}

// checkAMIASG scans the asg cache for Auto Scaling Groups whose currently
// running instances launched from this AMI. The AutoScalingGroup struct does
// NOT embed an ImageId (it references a LaunchTemplate or LaunchConfiguration,
// which would require a separate ec2:DescribeLaunchTemplateVersions or
// autoscaling:DescribeLaunchConfigurations call to resolve). Instead we go via
// the EC2 cache: for each ASG, walk Instances[].InstanceId and look them up in
// the ec2 cache; if any instance's image_id matches this AMI, that ASG is
// related. This is cache-only and correct for the running-fleet view.
func checkAMIASG(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	amiID := res.ID
	if amiID == "" {
		return resource.KnownRelated("asg", nil, false)
	}

	asgList, asgTruncated, err := relatedResourcesFor(ctx, clients, cache, "asg")
	if err != nil {
		return resource.ErrorRelated("asg", err)
	}
	if asgList == nil {
		return resource.UnknownRelated("asg")
	}
	ec2List, ec2Truncated, err := relatedResourcesFor(ctx, clients, cache, "ec2")
	if err != nil {
		return resource.ErrorRelated("asg", err)
	}
	// ec2List may be nil when no ec2 cache entry is present (secondary lookup).
	// Continue with an empty map — results will be based on asg cache alone.

	// Build map: instanceID -> image_id from the EC2 cache.
	ec2Image := make(map[string]string, len(ec2List))
	for _, ec2Res := range ec2List {
		if id := ec2Res.Fields["image_id"]; id != "" {
			ec2Image[ec2Res.ID] = id
		}
	}

	var ids []string
	for _, asgRes := range asgList {
		asg, ok := assertStruct[asgtypes.AutoScalingGroup](asgRes.RawStruct)
		if !ok {
			continue
		}
		for _, inst := range asg.Instances {
			if inst.InstanceId == nil {
				continue
			}
			if ec2Image[*inst.InstanceId] == amiID {
				ids = append(ids, asgRes.ID)
				break
			}
		}
	}
	return relatedResultTrunc("asg", ids, (asgTruncated || ec2Truncated))
}

// amiRelatedResources returns the resource list for target from cache or by
// fetching the first page via the registered paginated fetcher.
func amiRelatedResources(ctx context.Context, clients any, cache resource.ResourceCache, target string) ([]resource.Resource, bool, error) {
	return relatedResourcesFor(ctx, clients, cache, target)
}
