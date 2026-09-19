// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// mwaa_related.go contains MWAA environment related-resource checker
// functions. Every checker here is zero-extra-API-call: the fetcher's
// GetEnvironment pass already carries every field these checkers read
// from the RawStruct, plus the alarm sibling-cache scan which may fetch
// the alarm list's first page on cache miss. See docs/resources/mwaa.md.
package aws

import (
	"context"

	mwaatypes "github.com/aws/aws-sdk-go-v2/service/mwaa/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// checkMWAAAlarms cross-references the already-loaded alarm cache for
// CloudWatch alarms in the AWS/MWAA namespace whose EnvironmentName
// dimension matches this environment's name. Environment carries no ARN
// field usable for a forward lookup, so this is a workflow pivot (cache
// scan).
func checkMWAAAlarms(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	return alarmIDsByDimension(ctx, clients, cache, "AWS/MWAA", "EnvironmentName", res.ID)
}

// checkMWAAKMS reads Environment.KmsKey directly.
func checkMWAAKMS(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	env, ok := assertStruct[mwaatypes.Environment](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("kms")
	}
	if env.KmsKey == nil || *env.KmsKey == "" {
		return resource.KnownRelated("kms", nil, false)
	}
	return kmsRelated(ctx, clients, cache, []string{kmsRefFromField(*env.KmsKey, res.Type)})
}

// checkMWAALogs reads the five LoggingConfiguration CloudWatchLogGroupArn
// fields directly.
func checkMWAALogs(_ context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	env, ok := assertStruct[mwaatypes.Environment](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("logs")
	}
	if env.LoggingConfiguration == nil {
		return resource.KnownRelated("logs", nil, false)
	}
	var ids []string
	for _, m := range []*mwaatypes.ModuleLoggingConfiguration{
		env.LoggingConfiguration.DagProcessingLogs,
		env.LoggingConfiguration.SchedulerLogs,
		env.LoggingConfiguration.WebserverLogs,
		env.LoggingConfiguration.WorkerLogs,
		env.LoggingConfiguration.TaskLogs,
	} {
		if m != nil && m.CloudWatchLogGroupArn != nil {
			ids = append(ids, *m.CloudWatchLogGroupArn)
		}
	}
	return relatedRefs("logs", ids, refContext(clients, cache, "logs"))
}

// checkMWAARole returns the role in ExecutionRoleArn.
func checkMWAARole(_ context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	env, ok := assertStruct[mwaatypes.Environment](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("role")
	}
	if env.ExecutionRoleArn == nil || *env.ExecutionRoleArn == "" {
		return resource.KnownRelated("role", nil, false)
	}
	return relatedRefs("role", []string{*env.ExecutionRoleArn}, refContext(clients, cache, "role"))
}

// checkMWAAS3 extracts the bare bucket name from SourceBucketArn.
func checkMWAAS3(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	env, ok := assertStruct[mwaatypes.Environment](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("s3")
	}
	if env.SourceBucketArn == nil || *env.SourceBucketArn == "" {
		return resource.KnownRelated("s3", nil, false)
	}
	a, ok := ARNForService(*env.SourceBucketArn, "s3")
	if !ok || a.Resource == "" {
		return resource.KnownRelated("s3", nil, false)
	}
	return relatedResult("s3", []string{a.Resource})
}

// checkMWAASG reads NetworkConfiguration.SecurityGroupIds directly.
func checkMWAASG(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	env, ok := assertStruct[mwaatypes.Environment](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("sg")
	}
	if env.NetworkConfiguration == nil || len(env.NetworkConfiguration.SecurityGroupIds) == 0 {
		return resource.KnownRelated("sg", nil, false)
	}
	return relatedResult("sg", env.NetworkConfiguration.SecurityGroupIds)
}

// checkMWAASubnet reads NetworkConfiguration.SubnetIds directly.
func checkMWAASubnet(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	env, ok := assertStruct[mwaatypes.Environment](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("subnet")
	}
	if env.NetworkConfiguration == nil || len(env.NetworkConfiguration.SubnetIds) == 0 {
		return resource.KnownRelated("subnet", nil, false)
	}
	return relatedResult("subnet", env.NetworkConfiguration.SubnetIds)
}
