// ecs_task_related.go contains ECS task related-resource checker functions.
package aws

import (
	"context"
	"strings"

	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// checkECSTaskService returns the ECS service this task belongs to (Pattern F).
// For service-managed tasks, the Group field has the format "service:{service-name}".
func checkECSTaskService(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ecstypes.Task](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("ecs-svc")
	}
	if raw.Group == nil || !strings.HasPrefix(*raw.Group, "service:") {
		return resource.RelatedCheckResult{TargetType: "ecs-svc", Count: 0}
	}
	serviceName := strings.TrimPrefix(*raw.Group, "service:")
	if serviceName == "" {
		return resource.RelatedCheckResult{TargetType: "ecs-svc", Count: 0}
	}
	return relatedResult("ecs-svc", []string{serviceName})
}

// checkECSTaskCluster returns the ECS cluster this task belongs to (Pattern F).
// Extracts the cluster name from ClusterArn (last segment after "/"), falling
// back to Fields["cluster"] if ClusterArn is nil.
func checkECSTaskCluster(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ecstypes.Task](res.RawStruct)
	if ok && raw.ClusterArn != nil && *raw.ClusterArn != "" {
		clusterName := arnLastSegment(*raw.ClusterArn)
		if clusterName != "" {
			return relatedResult("ecs", []string{clusterName})
		}
	}
	// Fallback: use Fields["cluster"] set by the fetcher (stores full ClusterArn)
	clusterField := res.Fields["cluster"]
	if clusterField == "" {
		return resource.RelatedCheckResult{TargetType: "ecs", Count: 0}
	}
	clusterName := arnLastSegment(clusterField)
	if clusterName == "" {
		clusterName = clusterField
	}
	return relatedResult("ecs", []string{clusterName})
}

// arnLastSegment extracts the last segment after "/" from an ARN or any
// slash-delimited string. Returns the input unchanged if there is no "/".
func arnLastSegment(arn string) string {
	parts := strings.Split(arn, "/")
	return parts[len(parts)-1]
}

// checkECSTaskLogs searches the logs cache for log groups matching the task's
// task definition family name.
// Pattern N — convention: scan cache for log groups containing the task def family name.
func checkECSTaskLogs(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ecstypes.Task](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("logs")
	}
	taskDefARN := ""
	if raw.TaskDefinitionArn != nil {
		taskDefARN = *raw.TaskDefinitionArn
	}
	if taskDefARN == "" {
		return resource.RelatedCheckResult{TargetType: "logs", Count: 0}
	}
	// Extract task def family from ARN: arn:aws:ecs:region:account:task-definition/family:revision
	family := arnLastSegment(taskDefARN)
	// Remove revision suffix (e.g. "family:5" -> "family")
	if idx := strings.LastIndex(family, ":"); idx >= 0 {
		family = family[:idx]
	}
	if family == "" {
		return resource.RelatedCheckResult{TargetType: "logs", Count: 0}
	}

	logList, truncated, err := ecsTaskRelatedResources(ctx, clients, cache, "logs")
	if err != nil {
		return resource.ErrorRelated("logs", err)
	}
	if logList == nil {
		return resource.UnknownRelated("logs")
	}

	var ids []string
	for _, logRes := range logList {
		if strings.Contains(logRes.ID, family) {
			ids = append(ids, logRes.ID)
		}
	}
	return relatedResultTrunc("logs", ids, truncated)
}

// ecsTaskRelatedResources returns the resource list for target from cache or by fetching the first page.
func ecsTaskRelatedResources(ctx context.Context, clients any, cache resource.ResourceCache, target string) ([]resource.Resource, bool, error) {
	resources, isTruncated, err := FetchRelatedTarget(ctx, clients, cache, target)
	if err != nil {
		if _, ok := clients.(*ServiceClients); !ok {
			return nil, false, nil
		}
	}
	return resources, isTruncated, err
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
		return resource.RelatedCheckResult{TargetType: "role", Count: 0}
	}

	names := make(map[string]struct{}, len(arns))
	for _, arn := range arns {
		name := arn
		if idx := strings.LastIndex(arn, "/"); idx >= 0 && idx < len(arn)-1 {
			name = arn[idx+1:]
		}
		if name != "" {
			names[name] = struct{}{}
		}
	}
	if len(names) == 0 {
		return resource.RelatedCheckResult{TargetType: "role", Count: 0}
	}

	roleList, truncated, err := ecsTaskRelatedResources(ctx, clients, cache, "role")
	if err != nil {
		return resource.ErrorRelated("role", err)
	}
	if roleList == nil {
		return resource.UnknownRelated("role")
	}

	var ids []string
	for _, roleRes := range roleList {
		if _, match := names[roleRes.Name]; match {
			ids = append(ids, roleRes.ID)
			continue
		}
		if _, match := names[roleRes.Fields["role_name"]]; match {
			ids = append(ids, roleRes.ID)
		}
	}
	return relatedResultTrunc("role", ids, truncated)
}
