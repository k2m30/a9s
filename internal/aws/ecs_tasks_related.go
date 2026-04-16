// ecs_tasks_related.go contains ECS task related-resource checker functions.
package aws

import (
	"context"
	"strings"

	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterRelated("ecs-task", []resource.RelatedDef{
		{TargetType: "ecs-svc", DisplayName: "ECS Services", Checker: checkECSTaskService},
		{TargetType: "ecs", DisplayName: "ECS Clusters", Checker: checkECSTaskCluster},
		{TargetType: "logs", DisplayName: "Log Groups", Checker: checkECSTaskLogs, NeedsTargetCache: true},
		{TargetType: "sg", DisplayName: "Security Groups", Checker: checkECSTaskSG},
		{TargetType: "role", DisplayName: "IAM Role", Checker: checkECSTaskRole},
		{TargetType: "kms", DisplayName: "KMS Key", Checker: checkECSTaskKMS},
	})

	// ecstypes.Task: ClusterArn (parent cluster for this task execution)
	resource.RegisterNavigableFields("ecs-task", []resource.NavigableField{
		{FieldPath: "ClusterArn", TargetType: "ecs"},
	})
}

// checkECSTaskService returns the ECS service this task belongs to (Pattern F).
// For service-managed tasks, the Group field has the format "service:{service-name}".
func checkECSTaskService(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ecstypes.Task](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "ecs-svc", Count: -1}
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
		return resource.RelatedCheckResult{TargetType: "logs", Count: -1}
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
		return resource.RelatedCheckResult{TargetType: "logs", Count: -1, Err: err}
	}
	if logList == nil {
		return resource.RelatedCheckResult{TargetType: "logs", Count: -1}
	}

	var ids []string
	for _, logRes := range logList {
		if strings.Contains(logRes.ID, family) {
			ids = append(ids, logRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "logs", Count: -1}
	}
	return relatedResult("logs", ids)
}

// checkECSTaskSG returns Count: 0 because the ECS Task struct does not carry
// security group IDs directly — they are set at the task definition level and
// resolved by ECS at launch time. The running task does not surface awsvpc
// SecurityGroups in the DescribeTasks/ListTasks response payload.
func checkECSTaskSG(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "sg", Count: 0}
}

// checkECSTaskRole returns Count: 0 because the ECS Task struct does not expose
// a TaskRoleArn directly — the task role is on the task definition, not on the
// running task in the ListTasks/DescribeTasks response.
func checkECSTaskRole(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "role", Count: 0}
}

// checkECSTaskKMS is a stub. The ECS Task struct does not carry a KMS key ID
// directly — KMS references are on the task definition, not the running task.
func checkECSTaskKMS(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "kms", Count: 0}
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
