// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"regexp"
	"slices"
	"strings"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

var ebsSnapCreateImageRe = regexp.MustCompile(`Created by CreateImage\((i-[a-zA-Z0-9]+)\)`)

// checkEBSSnapAMI scans the AMI cache for AMIs whose block device mappings reference this snapshot (Pattern C).
func checkEBSSnapAMI(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	snapID := res.ID
	if snapID == "" {
		return foundNone("ami", "snapID")
	}

	amiList, truncated, err := relatedResourcesFor(ctx, clients, cache, "ami")
	if err != nil {
		return ReadFailed("ami", err)
	}
	if amiList == nil {
		return NotRead("ami")
	}

	var ids []string
	for _, r := range amiList {
		img, ok := assertStruct[ec2types.Image](r.RawStruct)
		if !ok {
			continue
		}
		for _, bdm := range img.BlockDeviceMappings {
			if bdm.Ebs != nil && bdm.Ebs.SnapshotId != nil && *bdm.Ebs.SnapshotId == snapID {
				ids = append(ids, r.ID)
				break
			}
		}
	}
	return relatedResultTrunc("ami", ids, truncated)
}

// checkEBSSnapEBS reads the source volume ID from Fields["volume_id"] (Pattern F).
func checkEBSSnapEBS(_ context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	if !ebsSnapParentIsLocal(res.RawStruct) {
		return foundNone("ebs", "snap.VolumeId")
	}
	return relatedRefs("ebs", []string{res.Fields["volume_id"]}, refContext(clients, cache, "ebs"))
}

// checkEBSSnapEC2 parses the snapshot Description for "Created by
// CreateImage(i-xxx)" and resolves that instance against the loaded ec2 list:
// the instance is often terminated since the image was made.
func checkEBSSnapEC2(_ context.Context, _ any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	matches := ebsSnapCreateImageRe.FindStringSubmatch(res.Fields["description"])
	if len(matches) < 2 {
		return foundNone("ec2", "matches")
	}
	ec2List, truncated, loaded := cachedRelatedList(cache, "ec2")
	if !loaded {
		return NotRead("ec2")
	}
	var ids []string
	if slices.ContainsFunc(ec2List, func(r resource.Resource) bool { return r.ID == matches[1] }) {
		ids = []string{matches[1]}
	}
	return relatedAnswer("ec2", relatedRead{ids: ids, partial: truncated, atMostOne: true})
}

// checkEBSSnapKMS reads the KMS key from RawStruct.KmsKeyId (Pattern F).
func checkEBSSnapKMS(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	snap, ok := assertStruct[ec2types.Snapshot](res.RawStruct)
	if !ok {
		return NotRead("kms")
	}
	if snap.KmsKeyId == nil || *snap.KmsKeyId == "" {
		return foundNone("kms", "snap.KmsKeyId")
	}
	return kmsRelated(ctx, clients, cache, []string{*snap.KmsKeyId})
}

// checkEBSSnapBackup scans this snapshot's own Description and Tags for the
// AWS Backup signature (Description prefix "Created by AWS Backup" or the
// auto-tag "aws:backup:source-resource", whose value is the source
// resource's ARN) per docs/resources/ebs-snap.md — no direct field on
// Snapshot points at a Backup plan. Once the signature is found, the source
// ARN is evaluated against the already-loaded backup cache through
// BackupPlanCovers to resolve the owning plan(s). Zero extra calls.
func checkEBSSnapBackup(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	description := ""
	sourceARN := ""
	if snap, ok := assertStruct[ec2types.Snapshot](res.RawStruct); ok {
		if snap.Description != nil {
			description = *snap.Description
		}
		for _, tag := range snap.Tags {
			if tag.Key != nil && *tag.Key == "aws:backup:source-resource" && tag.Value != nil {
				sourceARN = *tag.Value
				break
			}
		}
	}
	isBackupCreated := strings.HasPrefix(description, "Created by AWS Backup") || sourceARN != ""
	if !isBackupCreated {
		// No Backup signature in Description/Tags — the parent's own fields
		// rule out coverage; not a truncated-cache situation.
		return unreadZero(res, foundNone("backup", "isBackupCreated"))
	}
	if sourceARN == "" {
		// Backup-created signature confirmed via Description alone, but no
		// source-resource ARN to cross-reference against plan selections —
		// honestly unresolvable to a specific plan.
		return NotRead("backup")
	}

	backupList, truncated, err := relatedResourcesFor(ctx, clients, cache, "backup")
	if err != nil {
		return ReadFailed("backup", err)
	}
	if backupList == nil {
		return NotRead("backup")
	}

	return unreadZeroScanned(res, len(backupList), backupPivot(backupList, truncated, backupTarget{arn: sourceARN, unread: "DescribeVolumes"}))
}
