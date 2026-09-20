// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// ecs_task_related.go contains ECS task related-resource checker functions.
package aws

import (
	"context"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// checkECSTaskService returns the ECS service this task belongs to (Pattern F).
// For service-managed tasks, the Group field has the format "service:{service-name}".
func checkECSTaskService(_ context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ecstypes.Task](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("ecs-svc")
	}
	if raw.Group == nil || !strings.HasPrefix(*raw.Group, "service:") {
		return resource.ProvenZero("ecs-svc", "raw.Group")
	}
	return relatedRefs("ecs-svc", []string{*raw.Group}, refContext(clients, cache, "ecs-svc"))
}

// checkECSTaskCluster returns the ECS cluster this task belongs to (Pattern F):
// ClusterArn, falling back to Fields["cluster"] (the fetcher stores the full
// ClusterArn there) when the RawStruct carries none.
func checkECSTaskCluster(_ context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	cluster := res.Fields["cluster"]
	if raw, ok := assertStruct[ecstypes.Task](res.RawStruct); ok && raw.ClusterArn != nil && *raw.ClusterArn != "" {
		cluster = *raw.ClusterArn
	}
	return relatedRefs("ecs", []string{cluster}, refContext(clients, cache, "ecs"))
}

// checkECSTaskLogs reports the log groups the containers of the task's
// task definition write to.
func checkECSTaskLogs(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ecstypes.Task](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("logs")
	}
	taskDefARN := aws.ToString(raw.TaskDefinitionArn)
	if taskDefARN == "" {
		return resource.ProvenZero("logs", "raw.TaskDefinitionArn")
	}
	return ecsTaskDefLogGroups(ctx, clients, cache, taskDefARN)
}

// checkECSTaskRole returns the IAM role(s) associated with this ECS task:
// the task role (application-level) and the execution role (pull/log). The
// ecstypes.Task struct returned by DescribeTasks does NOT include these ARNs
// directly — they live on the TaskDefinition. The fetcher's
// ecsJoinTaskDefinition join pre-populates Fields["task_role"] and
// Fields["execution_role"] via DescribeTaskDefinition. This checker
// cross-references the already-loaded role cache by ARN suffix/name per
// docs/resources/ecs-task.md so the returned IDs resolve to real role rows
// (0, 1, or 2 roles).
func checkECSTaskRole(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	var arns []string
	if v := strings.TrimSpace(res.Fields["task_role"]); v != "" {
		arns = append(arns, v)
	}
	if v := strings.TrimSpace(res.Fields["execution_role"]); v != "" {
		arns = append(arns, v)
	}
	if len(arns) == 0 {
		return resource.ProvenZero("role", "arns")
	}

	roleList, _, err := relatedResourcesFor(ctx, clients, cache, "role")
	if err != nil {
		return resource.ErrorRelated("role", err)
	}
	if roleList == nil {
		return resource.UnknownRelated("role")
	}
	ids, lowerBound := listedRefs("role", arns, refContext(clients, cache, "role"), roleList)
	return relatedResultTrunc("role", ids, lowerBound)
}
