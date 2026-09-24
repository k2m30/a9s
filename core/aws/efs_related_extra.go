// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// efs_related_extra.go contains additional EFS related-resource checkers
// required by docs/related-resources.md.
package aws

import (
	"context"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	efstypes "github.com/aws/aws-sdk-go-v2/service/efs/types"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

func checkEFSAlarm(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	return alarmIDsByDimension(ctx, clients, cache, "efs", res)
}

// checkEFSENI scans eni cache for mount-target ENIs (description contains fs-id).
func checkEFSENI(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	fsID := res.ID
	if fsID == "" {
		return foundNone("eni", "fsID")
	}
	eniList, truncated, err := relatedResourcesFor(ctx, clients, cache, "eni")
	if err != nil {
		return ReadFailed("eni", err)
	}
	if eniList == nil {
		return NotRead("eni")
	}
	var ids []string
	for _, eniRes := range eniList {
		eni, ok := assertStruct[ec2types.NetworkInterface](eniRes.RawStruct)
		if !ok {
			continue
		}
		if eniMountsFileSystem(eni, fsID) {
			ids = append(ids, eniRes.ID)
		}
	}
	return relatedResultTrunc("eni", ids, truncated)
}

// checkEFSVPC derives the VPC via mount-target ENIs → subnet → VPC lookup.
func checkEFSVPC(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	fsID := res.ID
	if fsID == "" {
		return foundNone("vpc", "fsID")
	}
	eniList, truncated, err := relatedResourcesFor(ctx, clients, cache, "eni")
	if err != nil {
		return ReadFailed("vpc", err)
	}
	if eniList == nil {
		return NotRead("vpc")
	}
	vpcSet := make(map[string]struct{})
	for _, eniRes := range eniList {
		eni, ok := assertStruct[ec2types.NetworkInterface](eniRes.RawStruct)
		if !ok {
			continue
		}
		if !eniMountsFileSystem(eni, fsID) {
			continue
		}
		if eni.VpcId != nil && *eni.VpcId != "" {
			vpcSet[*eni.VpcId] = struct{}{}
		}
	}
	var ids []string
	for id := range vpcSet {
		ids = append(ids, id)
	}
	return relatedResultTrunc("vpc", ids, truncated)
}

// checkEFSBackup resolves AWS Backup PLANS that protect this EFS file system
// by reverse-scanning the backup cache through BackupPlanCovers, with the file
// system's own tags for the selections' tag clauses.
func checkEFSBackup(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	fs, ok := assertStruct[efstypes.FileSystemDescription](res.RawStruct)
	if !ok {
		return NotRead("backup")
	}
	if fs.FileSystemArn == nil || *fs.FileSystemArn == "" {
		return foundNone("backup", "fs.FileSystemArn")
	}
	fsARN := *fs.FileSystemArn

	plans, truncated, err := relatedResourcesFor(ctx, clients, cache, "backup")
	if err != nil {
		return ReadFailed("backup", err)
	}
	tags := make(map[string]string, len(fs.Tags))
	for _, t := range fs.Tags {
		if t.Key != nil && t.Value != nil {
			tags[*t.Key] = *t.Value
		}
	}
	return backupPivot(plans, truncated, backupTarget{arn: fsARN, tags: tags})
}

// keep lambdatypes imported (used by checkEFSLambda in efs_related.go).
var _ = lambdatypes.FunctionConfiguration{}
