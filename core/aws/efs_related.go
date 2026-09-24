// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// efs_related.go contains EFS related-resource checker functions.
package aws

import (
	"context"
	"slices"
	"strings"

	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/efs"
	efstypes "github.com/aws/aws-sdk-go-v2/service/efs/types"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// checkEFSKMS returns the KMS key used to encrypt this EFS file system (Pattern F).
//
// The checker emits the key blindly; the related-check orchestrator's
// lazy-add path (SetFetchByIDsForTest for "kms") fetches the key metadata on
// demand when the ID is not already in the customer-managed kms cache. That
// keeps this checker simple AND lets AWS-managed keys (aws/elasticfilesystem,
// etc.) drill into a real entry — both the count and the drill land on the
// same resource.
func checkEFSKMS(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	fs, ok := assertStruct[efstypes.FileSystemDescription](res.RawStruct)
	if !ok {
		return NotRead("kms")
	}
	if fs.KmsKeyId == nil || *fs.KmsKeyId == "" {
		return foundNone("kms", "fs.KmsKeyId")
	}
	return kmsRelated(ctx, clients, cache, []string{*fs.KmsKeyId})
}

// checkEFSCFN checks EFS file system tags for aws:cloudformation:stack-name
// and matches against the CFN stack cache (Pattern C — tag-based).
func checkEFSCFN(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	stackName := efsCFNStackName(res)
	if stackName == "" {
		return unreadZero(res, foundNone("cfn", "stackName"))
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
		raw, ok := assertStruct[cfntypes.Stack](cfnRes.RawStruct)
		if ok && raw.StackName != nil && *raw.StackName == stackName {
			ids = append(ids, cfnRes.ID)
		}
	}
	return unreadZeroScanned(res, len(cfnList), relatedResultTrunc("cfn", ids, truncated))
}

// efsCFNStackName extracts the aws:cloudformation:stack-name tag value from the
// EFS file system's Tags slice.
func efsCFNStackName(res resource.Resource) string {
	fs, ok := assertStruct[efstypes.FileSystemDescription](res.RawStruct)
	if !ok {
		return ""
	}
	for _, tag := range fs.Tags {
		if tag.Key != nil && *tag.Key == "aws:cloudformation:stack-name" && tag.Value != nil {
			return *tag.Value
		}
	}
	return ""
}

// checkEFSSG finds security groups for this EFS file system by scanning the ENI
// cache for mount-target ENIs whose Description contains the filesystem ID (Pattern C).
func checkEFSSG(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	fsID := res.ID
	if fsID == "" {
		return foundNone("sg", "fsID")
	}

	eniList, truncated, err := relatedResourcesFor(ctx, clients, cache, "eni")
	if err != nil {
		return ReadFailed("sg", err)
	}
	if eniList == nil {
		return NotRead("sg")
	}

	sgSet := make(map[string]struct{})
	for _, eniRes := range eniList {
		eni, ok := assertStruct[ec2types.NetworkInterface](eniRes.RawStruct)
		if !ok {
			continue
		}
		if !eniMountsFileSystem(eni, fsID) {
			continue
		}
		for _, sg := range eni.Groups {
			if sg.GroupId != nil && *sg.GroupId != "" {
				sgSet[*sg.GroupId] = struct{}{}
			}
		}
	}

	var ids []string
	for id := range sgSet {
		ids = append(ids, id)
	}
	return relatedResultTrunc("sg", ids, truncated)
}

// checkEFSSubnet finds subnets for this EFS file system by scanning the ENI
// cache for the mount-target ENIs whose Description names it (Pattern C).
func checkEFSSubnet(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	fsID := res.ID
	if fsID == "" {
		return foundNone("subnet", "fsID")
	}

	eniList, truncated, err := relatedResourcesFor(ctx, clients, cache, "eni")
	if err != nil {
		return ReadFailed("subnet", err)
	}
	if eniList == nil {
		return NotRead("subnet")
	}

	subnetSet := make(map[string]struct{})
	for _, eniRes := range eniList {
		eni, ok := assertStruct[ec2types.NetworkInterface](eniRes.RawStruct)
		if !ok {
			continue
		}
		if !eniMountsFileSystem(eni, fsID) {
			continue
		}
		if eni.SubnetId != nil && *eni.SubnetId != "" {
			subnetSet[*eni.SubnetId] = struct{}{}
		}
	}

	var ids []string
	for id := range subnetSet {
		ids = append(ids, id)
	}
	return relatedResultTrunc("subnet", ids, truncated)
}

// checkEFSLambda finds Lambda functions that mount this EFS file system via
// FileSystemConfigs. Lambda FileSystemConfigs carry EFS *access-point* ARNs,
// not filesystem ARNs, so the link requires resolving the filesystem's access
// points via efs:DescribeAccessPoints (Pattern A + C): collect this file
// system's access point ARNs, then scan the lambda cache for
// FunctionConfiguration.FileSystemConfigs entries whose Arn is in that set.
// Returns an unknown result when no live EFS client is available to list access points.
func checkEFSLambda(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	fsID := res.ID
	if fsID == "" {
		return foundNone("lambda", "fsID")
	}

	c, cok := clients.(*ServiceClients)
	if !cok || c == nil || c.EFS == nil {
		return NotRead("lambda")
	}

	aps, apsComplete, err := PageAll(ctx, PerParentPageCap, func(ctx context.Context, token *string) ([]efstypes.AccessPointDescription, *string, error) {
		out, err := c.EFS.DescribeAccessPoints(ctx, &efs.DescribeAccessPointsInput{
			FileSystemId: &fsID,
			NextToken:    token,
		})
		if err != nil {
			return nil, nil, err
		}
		return out.AccessPoints, out.NextToken, nil
	})
	if err != nil {
		return ReadFailed("lambda", err)
	}
	apARNs := make(map[string]struct{})
	for _, ap := range aps {
		if ap.AccessPointArn != nil && *ap.AccessPointArn != "" {
			apARNs[*ap.AccessPointArn] = struct{}{}
		}
	}
	if len(apARNs) == 0 {
		// No access points exist for this filesystem — no Lambda can mount it.
		return relatedResultTrunc("lambda", nil, !apsComplete)
	}

	lambdaList, truncated, err := relatedResourcesFor(ctx, clients, cache, "lambda")
	if err != nil {
		return ReadFailed("lambda", err)
	}
	if lambdaList == nil {
		return NotRead("lambda")
	}

	var ids []string
	for _, lRes := range lambdaList {
		fn, ok := assertStruct[lambdatypes.FunctionConfiguration](lRes.RawStruct)
		if !ok {
			continue
		}
		for _, cfg := range fn.FileSystemConfigs {
			if cfg.Arn == nil {
				continue
			}
			if _, matched := apARNs[*cfg.Arn]; matched {
				ids = append(ids, lRes.ID)
				break
			}
		}
	}
	return relatedResultTrunc("lambda", ids, truncated || !apsComplete)
}

// checkEFSECSTask is a reverse-scan checker for the efs→ecs-task relationship.
// Pattern C+reverse: iterate cache["ecs-task"]; for each task read
// Fields["efs_file_system_ids"] (comma-separated list of EFS file-system IDs
// joined by the ecs-task fetcher via DescribeTaskDefinition) and match against
// this filesystem's ID.
// NeedsTargetCache: true.
func checkEFSECSTask(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	fsID := res.ID
	if fsID == "" {
		return foundNone("ecs-task", "fsID")
	}

	ecsTaskList, truncated, err := relatedResourcesFor(ctx, clients, cache, "ecs-task")
	if err != nil {
		return ReadFailed("ecs-task", err)
	}
	if ecsTaskList == nil {
		return NotRead("ecs-task")
	}

	var ids []string
	joinIncomplete := false
	for _, tRes := range ecsTaskList {
		// A task whose DescribeTaskDefinition failed has incomplete
		// efs_file_system_ids; treat its contribution as unknown and mark
		// the overall result Truncated instead of a silently-wrong zero.
		if !taskDefJoined(tRes) {
			joinIncomplete = true
		}
		joined := tRes.Fields["efs_file_system_ids"]
		if joined == "" {
			continue
		}
		if slices.Contains(strings.Split(joined, ","), fsID) {
			ids = append(ids, tRes.ID)
		}
	}
	return relatedResultTrunc("ecs-task", ids, truncated || joinIncomplete)
}
