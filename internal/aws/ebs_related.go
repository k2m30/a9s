package aws

import (
	"context"
	"strings"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// checkEBSEC2 returns the EC2 instance this volume is attached to (Pattern F).
func checkEBSEC2(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	instanceID := res.Fields["attached_to"]
	if instanceID == "" {
		return resource.RelatedCheckResult{TargetType: "ec2", Count: 0}
	}
	return relatedResult("ec2", []string{instanceID})
}

// checkEBSSnap searches the ebs-snap cache for snapshots of this volume (Pattern C).
func checkEBSSnap(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	volID := res.ID
	if volID == "" {
		return resource.RelatedCheckResult{TargetType: "ebs-snap", Count: 0}
	}

	snapList, truncated, err := ebsRelatedResources(ctx, clients, cache, "ebs-snap")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "ebs-snap", Count: -1, Err: err}
	}
	if snapList == nil {
		return resource.RelatedCheckResult{TargetType: "ebs-snap", Count: -1}
	}

	var ids []string
	for _, r := range snapList {
		if r.Fields["volume_id"] == volID {
			ids = append(ids, r.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "ebs-snap", Count: -1}
	}
	return relatedResult("ebs-snap", ids)
}

// checkEBSKMS returns the KMS key used to encrypt this volume (Pattern F).
func checkEBSKMS(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	vol, ok := assertStruct[ec2types.Volume](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "kms", Count: -1}
	}
	if vol.KmsKeyId == nil || *vol.KmsKeyId == "" {
		return resource.RelatedCheckResult{TargetType: "kms", Count: 0}
	}
	arn := *vol.KmsKeyId
	idx := strings.LastIndex(arn, "/")
	if idx < 0 || idx == len(arn)-1 {
		return resource.RelatedCheckResult{TargetType: "kms", Count: 0}
	}
	return relatedResult("kms", []string{arn[idx+1:]})
}

// ebsRelatedResources returns the resource list for target from cache or fetches
// the first page via the registered paginated fetcher.
func ebsRelatedResources(ctx context.Context, clients any, cache resource.ResourceCache, target string) ([]resource.Resource, bool, error) {
	resources, isTruncated, err := FetchRelatedTarget(ctx, clients, cache, target)
	if err != nil {
		if _, ok := clients.(*ServiceClients); !ok {
			return nil, false, nil
		}
	}
	return resources, isTruncated, err
}
