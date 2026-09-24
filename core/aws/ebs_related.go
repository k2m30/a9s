// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"

	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// checkEBSEC2 returns the EC2 instances this volume is attached to (Pattern F).
func checkEBSEC2(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	attachedTo := res.Fields["attached_to"]
	if attachedTo == "" {
		return foundNone("ec2", "attachedTo")
	}
	return relatedResultTrunc("ec2", splitCSV(attachedTo), false)
}

// checkEBSSnap searches the ebs-snap cache for snapshots of this volume (Pattern C).
func checkEBSSnap(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	volID := res.ID
	if volID == "" {
		return foundNone("ebs-snap", "volID")
	}

	snapList, truncated, err := relatedResourcesFor(ctx, clients, cache, "ebs-snap")
	if err != nil {
		return ReadFailed("ebs-snap", err)
	}
	if snapList == nil {
		return NotRead("ebs-snap")
	}

	var ids []string
	for _, r := range snapList {
		if r.Fields["volume_id"] == volID && ebsSnapParentIsLocal(r.RawStruct) {
			ids = append(ids, r.ID)
		}
	}
	return relatedResultTrunc("ebs-snap", ids, truncated)
}

// checkEBSKMS returns the KMS key used to encrypt this volume (Pattern F).
func checkEBSKMS(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	vol, ok := assertStruct[ec2types.Volume](res.RawStruct)
	if !ok {
		return NotRead("kms")
	}
	if vol.KmsKeyId == nil || *vol.KmsKeyId == "" {
		return foundNone("kms", "vol.KmsKeyId")
	}
	return kmsRelated(ctx, clients, cache, []string{*vol.KmsKeyId})
}

// checkEBSAlarm searches the alarm cache for alarms with a VolumeId dimension
// matching this volume.
func checkEBSAlarm(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	return alarmIDsByDimension(ctx, clients, cache, "ebs", res)
}

// checkEBSCFN matches the volume's aws:cloudformation:stack-name tag to a
// CFN stack in the cache. Pattern C — Volume.Tags is populated from
// DescribeVolumes.
func checkEBSCFN(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	vol, ok := assertStruct[ec2types.Volume](res.RawStruct)
	if !ok {
		return NotRead("cfn")
	}
	stackName := ""
	for _, tag := range vol.Tags {
		if tag.Key != nil && *tag.Key == "aws:cloudformation:stack-name" && tag.Value != nil {
			stackName = *tag.Value
			break
		}
	}
	if stackName == "" {
		return foundNone("cfn", "stackName")
	}

	cfnList, truncated, err := relatedResourcesFor(ctx, clients, cache, "cfn")
	if err != nil {
		return ReadFailed("cfn", err)
	}
	if cfnList == nil {
		return NotRead("cfn")
	}

	var ids []string
	for _, cfnRes := range cfnList {
		if cfnRes.ID == stackName || cfnRes.Name == stackName || cfnRes.Fields["stack_name"] == stackName {
			ids = append(ids, cfnRes.ID)
			continue
		}
		rawCFN, cfnOk := assertStruct[cfntypes.Stack](cfnRes.RawStruct)
		if cfnOk && rawCFN.StackName != nil && *rawCFN.StackName == stackName {
			ids = append(ids, cfnRes.ID)
		}
	}
	return relatedResultTrunc("cfn", ids, truncated)
}

// checkEBSBackup scans the backup cache for backup plans that cover this
// volume through BackupPlanCovers, with the ARN the coverage join builds and
// the volume's own tags. Zero extra calls — a pure cross-reference of the
// already-loaded backup cache, per docs/resources/ebs.md.
func checkEBSBackup(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	volID := res.ID
	if volID == "" {
		return unreadZero(res, foundNone("backup", "volID"))
	}
	tags, tagsKnown := ebsVolumeTags(res)

	backupList, truncated, err := relatedResourcesFor(ctx, clients, cache, "backup")
	if err != nil {
		return ReadFailed("backup", err)
	}
	if backupList == nil {
		return NotRead("backup")
	}

	target := backupTarget{arn: sessionEC2ARN(ctx, clients, "volume", volID), tags: tags}
	if !tagsKnown {
		target.unread = "DescribeVolumes"
	}
	return unreadZeroScanned(res, len(backupList), backupPivot(backupList, truncated, target))
}
