// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// ec2_related_extra.go contains additional EC2 related-resource checkers
// required by docs/related-resources.md beyond the core set in ec2_related.go.
package aws

import (
	"context"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// checkEC2AMI returns the AMI this EC2 instance was launched from (Pattern F).
// Reads ImageId from the Instance RawStruct.
func checkEC2AMI(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ec2types.Instance](res.RawStruct)
	if !ok {
		return NotRead("ami")
	}
	if raw.ImageId == nil || *raw.ImageId == "" {
		return foundNone("ami", "raw.ImageId")
	}
	return relatedResultTrunc("ami", []string{*raw.ImageId}, false)
}

// checkEC2ENI extracts network interface IDs from the EC2 Instance's
// NetworkInterfaces slice (Pattern F — no cache needed).
func checkEC2ENI(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ec2types.Instance](res.RawStruct)
	if !ok {
		return NotRead("eni")
	}
	var ids []string
	for _, eni := range raw.NetworkInterfaces {
		if eni.NetworkInterfaceId != nil && *eni.NetworkInterfaceId != "" {
			ids = append(ids, *eni.NetworkInterfaceId)
		}
	}
	return relatedResultTrunc("eni", ids, false)
}

// checkEC2Subnet returns the subnet this EC2 instance runs in (Pattern F).
// Reads SubnetId from the Instance RawStruct.
func checkEC2Subnet(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ec2types.Instance](res.RawStruct)
	if !ok {
		return NotRead("subnet")
	}
	if raw.SubnetId == nil || *raw.SubnetId == "" {
		return foundNone("subnet", "raw.SubnetId")
	}
	return relatedResultTrunc("subnet", []string{*raw.SubnetId}, false)
}

// checkEC2KMS returns the KMS keys encrypting any EBS volumes attached to this
// instance. Pattern C: scans the ebs cache for volumes attached to this
// instance and collects their KmsKeyId values.
func checkEC2KMS(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	instanceID := res.ID
	if instanceID == "" {
		return keyMissing("kms", "instanceID")
	}

	ebsList, truncated, err := relatedResourcesFor(ctx, clients, cache, "ebs")
	if err != nil {
		return ReadFailed("kms", err)
	}
	if ebsList == nil {
		return NotRead("kms")
	}

	var refs []string
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
		if vol.KmsKeyId != nil {
			refs = append(refs, *vol.KmsKeyId)
		}
	}
	reads := kmsReads(ctx, clients, cache, "", refs)
	reads[""] = joinReads(reads[""], relatedRead{partial: truncated})
	return regionalAnswer(clients, "kms", reads)
}

// checkEC2Logs offers the log groups whose name carries this instance's id,
// the CloudWatch agent's convention (/aws/ec2/{instance-id}).
func checkEC2Logs(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	return logGroupsNaming(ctx, clients, cache, res.ID)
}

// checkEC2Backup scans the backup cache for backup plans that cover this
// instance through BackupPlanCovers, with the instance's ARN and its own tags,
// zero extra calls. Per docs/resources/ec2.md: "match by backup-plan selection
// tags present on Instance.Tags[] or by ARN".
func checkEC2Backup(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	instanceID := res.ID
	if instanceID == "" {
		return unreadZero(res, foundNone("backup", "instanceID"))
	}

	tags := map[string]string{}
	inst, tagsKnown := assertStruct[ec2types.Instance](res.RawStruct)
	for _, t := range inst.Tags {
		if t.Key != nil && t.Value != nil {
			tags[*t.Key] = *t.Value
		}
	}

	target := backupTarget{arn: sessionEC2ARN(ctx, clients, "instance", instanceID), tags: tags}
	if !tagsKnown {
		target.unread = "DescribeInstances"
	}

	backupList, truncated, err := relatedResourcesFor(ctx, clients, cache, "backup")
	if err != nil {
		return ReadFailed("backup", err)
	}
	if backupList == nil {
		return NotRead("backup")
	}

	return unreadZeroScanned(res, len(backupList), backupPivot(backupList, truncated, target))
}
