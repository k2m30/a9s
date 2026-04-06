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
