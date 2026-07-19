// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// mwaa_related.go contains MWAA environment related-resource checker
// functions. Every checker here is zero-extra-API-call: the fetcher's
// GetEnvironment pass already carries every field these checkers read, so
// each one is a Pattern F (RawStruct read) checker, plus the alarm
// sibling-cache scan which may fetch the alarm list's first page on cache
// miss. docs/resources/mwaa.md §2.
package aws

import (
	"context"
	"strings"

	mwaatypes "github.com/aws/aws-sdk-go-v2/service/mwaa/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// checkMWAAAlarms cross-references the already-loaded alarm cache for
// CloudWatch alarms in the AWS/MWAA namespace whose EnvironmentName
// dimension matches this environment's name. Environment carries no ARN
// field usable for a forward lookup, so this is a workflow pivot (cache
// scan), not a Pattern F read — docs/resources/mwaa.md §2 `alarm`.
func checkMWAAAlarms(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	return alarmIDsByDimension(ctx, clients, cache, "AWS/MWAA", "EnvironmentName", res.ID)
}

// checkMWAAKMS reads Environment.KmsKey directly (Pattern F).
func checkMWAAKMS(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	env, ok := assertStruct[mwaatypes.Environment](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("kms")
	}
	if env.KmsKey == nil || *env.KmsKey == "" {
		return resource.RelatedCheckResult{TargetType: "kms", Count: 0}
	}
	return relatedResult("kms", []string{kmsKeyIDFromField(*env.KmsKey, res.Type)})
}

// checkMWAALogs reads the five LoggingConfiguration CloudWatchLogGroupArn
// fields directly (Pattern F) and converts each to the bare log-group name
// the logs type indexes on (mwaaLogGroupNameFromARN, mwaa.go).
func checkMWAALogs(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	env, ok := assertStruct[mwaatypes.Environment](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("logs")
	}
	if env.LoggingConfiguration == nil {
		return resource.RelatedCheckResult{TargetType: "logs", Count: 0}
	}
	var ids []string
	for _, m := range []*mwaatypes.ModuleLoggingConfiguration{
		env.LoggingConfiguration.DagProcessingLogs,
		env.LoggingConfiguration.SchedulerLogs,
		env.LoggingConfiguration.WebserverLogs,
		env.LoggingConfiguration.WorkerLogs,
		env.LoggingConfiguration.TaskLogs,
	} {
		if name := mwaaLogGroup(m); name != "" {
			ids = append(ids, name)
		}
	}
	return relatedResult("logs", ids)
}

// checkMWAARole extracts the bare role name from ExecutionRoleArn
// (Pattern F).
func checkMWAARole(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	env, ok := assertStruct[mwaatypes.Environment](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("role")
	}
	if env.ExecutionRoleArn == nil || *env.ExecutionRoleArn == "" {
		return resource.RelatedCheckResult{TargetType: "role", Count: 0}
	}
	roleARN := *env.ExecutionRoleArn
	return relatedResult("role", []string{roleARN[strings.LastIndex(roleARN, "/")+1:]})
}

// checkMWAAS3 extracts the bare bucket name from SourceBucketArn
// (Pattern F).
func checkMWAAS3(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	env, ok := assertStruct[mwaatypes.Environment](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("s3")
	}
	if env.SourceBucketArn == nil || *env.SourceBucketArn == "" {
		return resource.RelatedCheckResult{TargetType: "s3", Count: 0}
	}
	bucket := strings.TrimPrefix(*env.SourceBucketArn, "arn:aws:s3:::")
	return relatedResult("s3", []string{bucket})
}

// checkMWAASG reads NetworkConfiguration.SecurityGroupIds directly
// (Pattern F).
func checkMWAASG(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	env, ok := assertStruct[mwaatypes.Environment](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("sg")
	}
	if env.NetworkConfiguration == nil || len(env.NetworkConfiguration.SecurityGroupIds) == 0 {
		return resource.RelatedCheckResult{TargetType: "sg", Count: 0}
	}
	return relatedResult("sg", env.NetworkConfiguration.SecurityGroupIds)
}

// checkMWAASubnet reads NetworkConfiguration.SubnetIds directly
// (Pattern F).
func checkMWAASubnet(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	env, ok := assertStruct[mwaatypes.Environment](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("subnet")
	}
	if env.NetworkConfiguration == nil || len(env.NetworkConfiguration.SubnetIds) == 0 {
		return resource.RelatedCheckResult{TargetType: "subnet", Count: 0}
	}
	return relatedResult("subnet", env.NetworkConfiguration.SubnetIds)
}
