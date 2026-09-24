// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// mwaa_related.go contains MWAA environment related-resource checker
// functions. Every checker here is zero-extra-API-call: the fetcher's
// GetEnvironment pass already carries every field these checkers read
// from the RawStruct, plus the alarm sibling-cache scan which may fetch
// the alarm list's first page on cache miss. See docs/resources/mwaa.md.
package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	mwaatypes "github.com/aws/aws-sdk-go-v2/service/mwaa/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// checkMWAAAlarms cross-references the already-loaded alarm cache for
// CloudWatch alarms in the AWS/MWAA namespace whose EnvironmentName
// dimension matches this environment's name. Environment carries no ARN
// field usable for a forward lookup, so this is a workflow pivot (cache
// scan).
func checkMWAAAlarms(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	return alarmIDsByDimension(ctx, clients, cache, "mwaa", res)
}

// checkMWAAKMS reads Environment.KmsKey directly.
func checkMWAAKMS(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	env, ok := assertStruct[mwaatypes.Environment](res.RawStruct)
	if !ok {
		return NotRead("kms")
	}
	if env.KmsKey == nil || *env.KmsKey == "" {
		return foundNone("kms", "env.KmsKey")
	}
	return kmsRelated(ctx, clients, cache, []string{kmsRefFromField(*env.KmsKey, res.Type)})
}

// checkMWAALogs reads the five LoggingConfiguration CloudWatchLogGroupArn
// fields directly, each while its module's Enabled says the log type "is
// enabled" (https://docs.aws.amazon.com/mwaa/latest/API/API_ModuleLoggingConfiguration.html).
func checkMWAALogs(_ context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	env, ok := assertStruct[mwaatypes.Environment](res.RawStruct)
	if !ok {
		return NotRead("logs")
	}
	if env.LoggingConfiguration == nil {
		return foundNone("logs", "env.LoggingConfiguration")
	}
	var ids []string
	for _, m := range []*mwaatypes.ModuleLoggingConfiguration{
		env.LoggingConfiguration.DagProcessingLogs,
		env.LoggingConfiguration.SchedulerLogs,
		env.LoggingConfiguration.WebserverLogs,
		env.LoggingConfiguration.WorkerLogs,
		env.LoggingConfiguration.TaskLogs,
	} {
		if m != nil && aws.ToBool(m.Enabled) && m.CloudWatchLogGroupArn != nil {
			ids = append(ids, *m.CloudWatchLogGroupArn)
		}
	}
	return relatedRefs("logs", ids, refContext(clients, cache, "logs"))
}

// checkMWAARole returns the role in ExecutionRoleArn.
func checkMWAARole(_ context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	env, ok := assertStruct[mwaatypes.Environment](res.RawStruct)
	if !ok {
		return NotRead("role")
	}
	if env.ExecutionRoleArn == nil || *env.ExecutionRoleArn == "" {
		return foundNone("role", "env.ExecutionRoleArn")
	}
	return relatedRefs("role", []string{*env.ExecutionRoleArn}, refContext(clients, cache, "role"))
}

// checkMWAAS3 extracts the bare bucket name from SourceBucketArn.
func checkMWAAS3(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	env, ok := assertStruct[mwaatypes.Environment](res.RawStruct)
	if !ok {
		return NotRead("s3")
	}
	if env.SourceBucketArn == nil || *env.SourceBucketArn == "" {
		return foundNone("s3", "env.SourceBucketArn")
	}
	a, ok := ARNForService(*env.SourceBucketArn, "s3")
	if !ok || a.Resource == "" {
		return foundNone("s3", "a.Resource")
	}
	return relatedResultTrunc("s3", []string{a.Resource}, false)
}

// checkMWAASG reads NetworkConfiguration.SecurityGroupIds directly.
func checkMWAASG(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	env, ok := assertStruct[mwaatypes.Environment](res.RawStruct)
	if !ok {
		return NotRead("sg")
	}
	if env.NetworkConfiguration == nil || len(env.NetworkConfiguration.SecurityGroupIds) == 0 {
		return foundNone("sg", "env.NetworkConfiguration.SecurityGroupIds")
	}
	return relatedResultTrunc("sg", env.NetworkConfiguration.SecurityGroupIds, false)
}

// checkMWAASubnet reads NetworkConfiguration.SubnetIds directly.
func checkMWAASubnet(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	env, ok := assertStruct[mwaatypes.Environment](res.RawStruct)
	if !ok {
		return NotRead("subnet")
	}
	if env.NetworkConfiguration == nil || len(env.NetworkConfiguration.SubnetIds) == 0 {
		return foundNone("subnet", "env.NetworkConfiguration.SubnetIds")
	}
	return relatedResultTrunc("subnet", env.NetworkConfiguration.SubnetIds, false)
}
