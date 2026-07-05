// ec2_related_extra.go contains additional EC2 related-resource checkers
// required by docs/related-resources.md beyond the core set in ec2_related.go.
package aws

import (
	"context"
	"strings"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// checkEC2AMI returns the AMI this EC2 instance was launched from (Pattern F).
// Reads ImageId from the Instance RawStruct.
func checkEC2AMI(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ec2types.Instance](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "ami", Count: -1}
	}
	if raw.ImageId == nil || *raw.ImageId == "" {
		return resource.RelatedCheckResult{TargetType: "ami", Count: 0}
	}
	return relatedResult("ami", []string{*raw.ImageId})
}

// checkEC2ENI extracts network interface IDs from the EC2 Instance's
// NetworkInterfaces slice (Pattern F — no cache needed).
func checkEC2ENI(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ec2types.Instance](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "eni", Count: -1}
	}
	var ids []string
	for _, eni := range raw.NetworkInterfaces {
		if eni.NetworkInterfaceId != nil && *eni.NetworkInterfaceId != "" {
			ids = append(ids, *eni.NetworkInterfaceId)
		}
	}
	return relatedResult("eni", ids)
}

// checkEC2Subnet returns the subnet this EC2 instance runs in (Pattern F).
// Reads SubnetId from the Instance RawStruct.
func checkEC2Subnet(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ec2types.Instance](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "subnet", Count: -1}
	}
	if raw.SubnetId == nil || *raw.SubnetId == "" {
		return resource.RelatedCheckResult{TargetType: "subnet", Count: 0}
	}
	return relatedResult("subnet", []string{*raw.SubnetId})
}

// checkEC2KMS returns the KMS keys encrypting any EBS volumes attached to this
// instance. Pattern C: scans the ebs cache for volumes attached to this
// instance and collects their KmsKeyId values.
func checkEC2KMS(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	instanceID := res.ID
	if instanceID == "" {
		return resource.RelatedCheckResult{TargetType: "kms", Count: 0}
	}

	ebsList, truncated, err := ec2RelatedResources(ctx, clients, cache, "ebs")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "kms", Count: -1, Err: err}
	}
	if ebsList == nil {
		return resource.RelatedCheckResult{TargetType: "kms", Count: 0}
	}

	keySet := make(map[string]struct{})
	for _, ebsRes := range ebsList {
		vol, ok := assertStruct[ec2types.Volume](ebsRes.RawStruct)
		if !ok {
			continue
		}
		attachedHere := false
		for _, att := range vol.Attachments {
			if att.InstanceId != nil && *att.InstanceId == instanceID {
				attachedHere = true
				break
			}
		}
		if !attachedHere {
			continue
		}
		if vol.KmsKeyId == nil || *vol.KmsKeyId == "" {
			continue
		}
		keyID := *vol.KmsKeyId
		if idx := strings.LastIndex(keyID, "/"); idx >= 0 && idx < len(keyID)-1 {
			keyID = keyID[idx+1:]
		}
		keySet[keyID] = struct{}{}
	}
	var ids []string
	for id := range keySet {
		ids = append(ids, id)
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("kms")
	}
	return relatedResult("kms", ids)
}

// checkEC2Logs searches the logs cache for log groups matching this EC2
// instance. Convention: CloudWatch Agent writes to /aws/ec2/{instance-id}.
// Pattern N — scan logs cache for groups containing the instance ID.
func checkEC2Logs(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	instanceID := res.ID
	if instanceID == "" {
		return resource.RelatedCheckResult{TargetType: "logs", Count: 0}
	}
	logList, truncated, err := ec2RelatedResources(ctx, clients, cache, "logs")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "logs", Count: -1, Err: err}
	}
	if logList == nil {
		return resource.RelatedCheckResult{TargetType: "logs", Count: -1}
	}

	var ids []string
	for _, logRes := range logList {
		if strings.Contains(logRes.ID, instanceID) {
			ids = append(ids, logRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("logs")
	}
	return relatedResult("logs", ids)
}

// checkEC2Backup scans the backup cache for backup plans that cover this
// instance, matching on two independent signals with zero extra calls: (1)
// the plan's selection tags (BackupSelection.ListOfTags, joined into
// Fields["selection_tags"] by the backup fetcher) against this instance's
// own tags, and (2) the plan's selection ARN list/wildcards
// (Fields["resources"]/Fields["not_resources"]) against this instance's ARN,
// via the same BackupPlanCoversARN helper checkS3Backup and other ARN-based
// backup pivots already use. Per docs/resources/ec2.md: "match by
// backup-plan selection tags present on Instance.Tags[] or by ARN".
func checkEC2Backup(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	instanceID := res.ID
	if instanceID == "" {
		return resource.RelatedCheckResult{TargetType: "backup", Count: 0}
	}

	tags := map[string]string{}
	if inst, ok := assertStruct[ec2types.Instance](res.RawStruct); ok {
		for _, t := range inst.Tags {
			if t.Key != nil && t.Value != nil {
				tags[*t.Key] = *t.Value
			}
		}
	}

	instanceARN := res.Fields["arn"]

	backupList, truncated, err := ec2RelatedResources(ctx, clients, cache, "backup")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "backup", Count: -1, Err: err}
	}
	if backupList == nil {
		return resource.RelatedCheckResult{TargetType: "backup", Count: -1}
	}

	var ids []string
	for _, planRes := range backupList {
		if backupSelectionTagsMatch(planRes.Fields["selection_tags"], tags) ||
			BackupPlanCoversARN(planRes.Fields["resources"], planRes.Fields["not_resources"], instanceARN) {
			ids = append(ids, planRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("backup")
	}
	return relatedResult("backup", ids)
}
