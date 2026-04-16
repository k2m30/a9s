// ami_related_extra.go contains additional AMI related-resource checkers
// required by docs/related-resources.md.
package aws

import (
	"context"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// checkAMICFN scans AMI tags for aws:cloudformation:stack-name and matches
// the cfn cache.
func checkAMICFN(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	img, ok := assertStruct[ec2types.Image](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: -1}
	}
	stackName := ""
	for _, t := range img.Tags {
		if t.Key != nil && *t.Key == "aws:cloudformation:stack-name" && t.Value != nil {
			stackName = *t.Value
			break
		}
	}
	if stackName == "" {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: 0}
	}
	cfnList, truncated, err := amiRelatedResources(ctx, clients, cache, "cfn")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: -1, Err: err}
	}
	if cfnList == nil {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: -1}
	}
	var ids []string
	for _, cfnRes := range cfnList {
		if cfnRes.ID == stackName || cfnRes.Name == stackName {
			ids = append(ids, cfnRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: -1}
	}
	return relatedResult("cfn", ids)
}

// checkAMIKMS extracts KMS key IDs from the AMI's block device mappings
// (where EBS.KmsKeyId is set).
func checkAMIKMS(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	img, ok := assertStruct[ec2types.Image](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "kms", Count: -1}
	}
	seen := make(map[string]struct{})
	for _, bdm := range img.BlockDeviceMappings {
		if bdm.Ebs == nil || bdm.Ebs.KmsKeyId == nil || *bdm.Ebs.KmsKeyId == "" {
			continue
		}
		seen[*bdm.Ebs.KmsKeyId] = struct{}{}
	}
	var ids []string
	for id := range seen {
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return resource.RelatedCheckResult{TargetType: "kms", Count: 0}
	}
	return relatedResult("kms", ids)
}

// checkAMING scans EKS node-group cache for node groups using this AMI.
// Nodegroup struct exposes AmiType (not ID) unless a custom launch template
// is used. Without custom-template resolution (which lives in launch
// template versions), we match NG entries whose Fields["image_id"] is
// populated by fetcher enrichment.
func checkAMING(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	amiID := res.ID
	if amiID == "" {
		return resource.RelatedCheckResult{TargetType: "ng", Count: 0}
	}
	ngList, truncated, err := amiRelatedResources(ctx, clients, cache, "ng")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "ng", Count: -1, Err: err}
	}
	if ngList == nil {
		return resource.RelatedCheckResult{TargetType: "ng", Count: -1}
	}
	var ids []string
	for _, ngRes := range ngList {
		if ngRes.Fields["image_id"] == amiID {
			ids = append(ids, ngRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "ng", Count: -1}
	}
	return relatedResult("ng", ids)
}
