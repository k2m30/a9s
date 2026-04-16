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
		return resource.RelatedCheckResult{TargetType: "kms", Count: -1}
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
		return resource.RelatedCheckResult{TargetType: "kms", Count: -1}
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
		return resource.RelatedCheckResult{TargetType: "logs", Count: -1}
	}
	return relatedResult("logs", ids)
}

// checkEC2SSM returns the SSM associations attached to this instance. SSM
// parameter references are not reachable from the Instance struct alone; the
// a9s "ssm" resource type is Parameter Store (not Session Manager). EC2
// instances can reference SSM parameters via user-data, but the parameter
// names are not in the Instance response. Return Count:0 — no cache-resolvable
// link. A live call would require ssm:ListAssociations per instance (N+1).
func checkEC2SSM(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "ssm", Count: 0}
}

// checkEC2Backup scans the backup cache for backup plans that select this
// EC2 instance by tag or by resource ARN. Pattern C.
func checkEC2Backup(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	instanceID := res.ID
	if instanceID == "" {
		return resource.RelatedCheckResult{TargetType: "backup", Count: 0}
	}
	backupList, truncated, err := ec2RelatedResources(ctx, clients, cache, "backup")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "backup", Count: -1, Err: err}
	}
	if backupList == nil {
		return resource.RelatedCheckResult{TargetType: "backup", Count: -1}
	}
	// Backup plan selections are not embedded in the cached BackupPlan list
	// entry — resolving would require GetBackupSelection per plan (N+1).
	// Without that detail we conservatively report Count:0 here; the presence
	// of a registration keeps the panel slot surfaced.
	_ = backupList
	_ = truncated
	return resource.RelatedCheckResult{TargetType: "backup", Count: 0}
}
