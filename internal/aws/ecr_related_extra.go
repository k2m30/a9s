// ecr_related_extra.go — additional ECR related-resource checkers.
package aws

import (
	"context"
	"strings"

	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func checkECRCTEvents(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	repoName := res.ID
	if repoName == "" {
		return resource.RelatedCheckResult{TargetType: "ct-events", Count: 0}
	}
	evList, truncated, err := ecrRelatedResources(ctx, clients, cache, "ct-events")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "ct-events", Count: -1, Err: err}
	}
	if evList == nil {
		return resource.RelatedCheckResult{TargetType: "ct-events", Count: -1}
	}
	var ids []string
	for _, evRes := range evList {
		ev, ok := assertStruct[cloudtrailtypes.Event](evRes.RawStruct)
		if !ok {
			continue
		}
		for _, r := range ev.Resources {
			if r.ResourceName != nil && strings.Contains(*r.ResourceName, repoName) {
				ids = append(ids, evRes.ID)
				break
			}
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "ct-events", Count: -1}
	}
	return relatedResult("ct-events", ids)
}

func checkECREbRule(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	// EventBridge rules on ECR image-push events are resolvable only via
	// events:ListTargetsByRule per rule (N+1). Not cache-resolvable.
	return resource.RelatedCheckResult{TargetType: "eb-rule", Count: 0}
}

func checkECRECS(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	// ECS services pull images from repos, but the link is on the task
	// definition ContainerDefinitions.Image field — requires task def fetch.
	return resource.RelatedCheckResult{TargetType: "ecs", Count: 0}
}

func checkECRECSTask(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	repoName := res.ID
	if repoName == "" {
		return resource.RelatedCheckResult{TargetType: "ecs-task", Count: 0}
	}
	taskList, truncated, err := ecrRelatedResources(ctx, clients, cache, "ecs-task")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "ecs-task", Count: -1, Err: err}
	}
	if taskList == nil {
		return resource.RelatedCheckResult{TargetType: "ecs-task", Count: -1}
	}
	var ids []string
	for _, tRes := range taskList {
		// The task's Containers[].Image is only populated in the task struct
		// for running tasks, not in DescribeTasks responses for all tasks.
		// A weak substring match on any field that looks like an image URI.
		for _, v := range tRes.Fields {
			if strings.Contains(v, ".dkr.ecr.") && strings.Contains(v, "/"+repoName) {
				ids = append(ids, tRes.ID)
				break
			}
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "ecs-task", Count: -1}
	}
	return relatedResult("ecs-task", ids)
}

func checkECREKS(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	// EKS Pod → ECR link lives in k8s manifests, not the cluster struct.
	return resource.RelatedCheckResult{TargetType: "eks", Count: 0}
}

func checkECRPipeline(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	// CodePipeline stages reference ECR repos only in GetPipeline response
	// (ActionTypeId). Not in ListPipelines response.
	return resource.RelatedCheckResult{TargetType: "pipeline", Count: 0}
}

func checkECRRole(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	// Repository policies reference principals, but the repository list API
	// does not return them. GetRepositoryPolicy is a separate call.
	return resource.RelatedCheckResult{TargetType: "role", Count: 0}
}

