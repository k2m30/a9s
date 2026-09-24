// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"

	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	ebtypes "github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// checkEbCFN checks the CFN cache for a stack associated with this EB environment.
// Pattern C: match by stack name "awseb-{envID}-stack", or one under
// "awseb-{envID}" on a "-" boundary.
func checkEbCFN(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	envID := res.ID
	if eb, ok := assertStruct[ebtypes.EnvironmentDescription](res.RawStruct); ok {
		if eb.EnvironmentId != nil && *eb.EnvironmentId != "" {
			envID = *eb.EnvironmentId
		}
	}
	if envID == "" {
		return foundNone("cfn", "envID")
	}

	envIDPrefix := "awseb-" + envID
	expectedName := envIDPrefix + "-stack"

	cfnList, truncated, err := relatedResourcesFor(ctx, clients, cache, "cfn")
	if err != nil {
		return ReadFailed("cfn", err)
	}
	if cfnList == nil {
		return NotRead("cfn")
	}

	var ids []string
	for _, cfnRes := range cfnList {
		name := cfnRes.Fields["stack_name"]
		if cfnRes.ID == expectedName || cfnRes.Name == expectedName || nameUnder(name, envIDPrefix, "-") {
			ids = append(ids, cfnRes.ID)
		}
	}
	return relatedResultTrunc("cfn", ids, truncated)
}

// checkEbLogs checks the log groups cache for groups associated with this EB environment.
// Pattern C: match by log group prefix "/aws/elasticbeanstalk/{envName}/".
func checkEbLogs(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	envName := res.Name
	if eb, ok := assertStruct[ebtypes.EnvironmentDescription](res.RawStruct); ok {
		if eb.EnvironmentName != nil && *eb.EnvironmentName != "" {
			envName = *eb.EnvironmentName
		}
	}
	if envName == "" {
		return foundNone("logs", "envName")
	}

	logList, truncated, err := relatedResourcesFor(ctx, clients, cache, "logs")
	if err != nil {
		return ReadFailed("logs", err)
	}
	if logList == nil {
		return NotRead("logs")
	}

	return relatedResultTrunc("logs", logGroupsUnder(logList, "/aws/elasticbeanstalk/"+envName), truncated)
}

// checkEbASG checks the ASG cache for groups tagged with this EB environment name.
// Pattern C: match by "elasticbeanstalk:environment-name" tag on each ASG.
func checkEbASG(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	envName := res.Name
	if eb, ok := assertStruct[ebtypes.EnvironmentDescription](res.RawStruct); ok {
		if eb.EnvironmentName != nil && *eb.EnvironmentName != "" {
			envName = *eb.EnvironmentName
		}
	}
	if envName == "" {
		return foundNone("asg", "envName")
	}

	asgList, truncated, err := relatedResourcesFor(ctx, clients, cache, "asg")
	if err != nil {
		return ReadFailed("asg", err)
	}
	if asgList == nil {
		return NotRead("asg")
	}

	var ids []string
	for _, asgRes := range asgList {
		raw, ok := assertStruct[asgtypes.AutoScalingGroup](asgRes.RawStruct)
		if !ok {
			continue
		}
		for _, tag := range raw.Tags {
			if tag.Key != nil && *tag.Key == "elasticbeanstalk:environment-name" &&
				tag.Value != nil && *tag.Value == envName {
				ids = append(ids, asgRes.ID)
				break
			}
		}
	}
	return relatedResultTrunc("asg", ids, truncated)
}

// checkEbEC2 scans the EC2 instance cache for instances tagged with this EB
// environment name via the "elasticbeanstalk:environment-name" tag.
// Pattern C: tag-based cache scan.
func checkEbEC2(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	envName := res.Name
	if eb, ok := assertStruct[ebtypes.EnvironmentDescription](res.RawStruct); ok {
		if eb.EnvironmentName != nil && *eb.EnvironmentName != "" {
			envName = *eb.EnvironmentName
		}
	}
	if envName == "" {
		return foundNone("ec2", "envName")
	}

	ec2List, truncated, err := relatedResourcesFor(ctx, clients, cache, "ec2")
	if err != nil {
		return ReadFailed("ec2", err)
	}
	if ec2List == nil {
		return NotRead("ec2")
	}

	var ids []string
	for _, ec2Res := range ec2List {
		inst, ok := assertStruct[ec2types.Instance](ec2Res.RawStruct)
		if !ok {
			continue
		}
		if tagValue(inst.Tags, "elasticbeanstalk:environment-name") == envName {
			ids = append(ids, ec2Res.ID)
		}
	}
	return relatedResultTrunc("ec2", ids, truncated)
}

func checkEbAlarm(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	return alarmIDsByDimension(ctx, clients, cache, "eb", res)
}
