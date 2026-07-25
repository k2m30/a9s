// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"regexp"
	"strings"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

var ebsSnapCreateImageRe = regexp.MustCompile(`Created by CreateImage\((i-[a-zA-Z0-9]+)\)`)

// checkEBSSnapAMI scans the AMI cache for AMIs whose block device mappings reference this snapshot (Pattern C).
func checkEBSSnapAMI(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	snapID := res.ID
	if snapID == "" {
		return resource.KnownRelated("ami", nil, false)
	}

	amiList, truncated, err := relatedResourcesFor(ctx, clients, cache, "ami")
	if err != nil {
		return resource.ErrorRelated("ami", err)
	}
	if amiList == nil {
		return resource.UnknownRelated("ami")
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
func checkEBSSnapEBS(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	volumeID := res.Fields["volume_id"]
	if volumeID == "" {
		return resource.KnownRelated("ebs", nil, false)
	}
	return relatedResult("ebs", []string{volumeID})
}

// checkEBSSnapEC2 parses the snapshot Description for "Created by CreateImage(i-xxx)" (Pattern F).
func checkEBSSnapEC2(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	description := res.Fields["description"]
	matches := ebsSnapCreateImageRe.FindStringSubmatch(description)
	if len(matches) < 2 {
		return resource.KnownRelated("ec2", nil, false)
	}
	return relatedResult("ec2", []string{matches[1]})
}

// checkEBSSnapKMS extracts the KMS key ID from RawStruct.KmsKeyId (Pattern F).
// Handles both full ARN format (arn:aws:kms:…/key-id) and bare key ID.
func checkEBSSnapKMS(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	snap, ok := assertStruct[ec2types.Snapshot](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("kms")
	}
	if snap.KmsKeyId == nil || *snap.KmsKeyId == "" {
		return resource.KnownRelated("kms", nil, false)
	}
	val := *snap.KmsKeyId
	keyID := val
	if idx := strings.LastIndex(val, "/"); idx >= 0 && idx < len(val)-1 {
		keyID = val[idx+1:]
	}
	if keyID == "" {
		return resource.KnownRelated("kms", nil, false)
	}
	return relatedResult("kms", []string{keyID})
}

// checkEBSSnapBackup scans this snapshot's own Description and Tags for the
// AWS Backup signature (Description prefix "Created by AWS Backup" or the
// auto-tag "aws:backup:source-resource", whose value is the source
// resource's ARN) per docs/resources/ebs-snap.md — no direct field on
// Snapshot points at a Backup plan. Once the signature is found, the source
// ARN is cross-referenced against the already-loaded backup cache's
// Fields["resources"] (the ARN list every plan's selections cover — already
// joined by the backup fetcher for sibling pivots) to resolve the owning
// plan(s). Zero extra calls.
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
		// No RawStruct, or no Backup signature in Description/Tags — the
		// parent's own fields definitively rule out coverage; not a
		// truncated-cache situation.
		return resource.KnownRelated("backup", nil, false)
	}
	if sourceARN == "" {
		// Backup-created signature confirmed via Description alone, but no
		// source-resource ARN to cross-reference against plan selections —
		// honestly unresolvable to a specific plan.
		return resource.UnknownRelated("backup")
	}

	backupList, truncated, err := relatedResourcesFor(ctx, clients, cache, "backup")
	if err != nil {
		return resource.ErrorRelated("backup", err)
	}
	if backupList == nil {
		return resource.UnknownRelated("backup")
	}

	var ids []string
	for _, planRes := range backupList {
		for arn := range strings.SplitSeq(planRes.Fields["resources"], ",") {
			if arn != "" && arn == sourceARN {
				ids = append(ids, planRes.ID)
				break
			}
		}
	}
	return relatedResultTrunc("backup", ids, truncated)
}
