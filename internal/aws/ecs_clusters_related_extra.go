// ecs_clusters_related_extra.go contains additional ECS cluster related-
// resource checkers required by docs/related-resources.md beyond what is
// already registered in ecs_clusters.go.
package aws

import (
	"context"
	"strings"

	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// checkECSASG scans the asg cache for Auto Scaling Groups tagged with this
// ECS cluster's capacity provider (Pattern C). ECS cluster capacity providers
// reference ASG ARNs, but the Cluster struct exposes them only by name; the
// reverse link from ASG→cluster surfaces through the
// AmazonECSManaged tag that ECS adds to ASGs it manages.
func checkECSASG(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	clusterName := res.ID
	if clusterName == "" {
		return resource.RelatedCheckResult{TargetType: "asg", Count: 0}
	}
	asgList, truncated, err := ecsRelatedResources(ctx, clients, cache, "asg")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "asg", Count: -1, Err: err}
	}
	if asgList == nil {
		return resource.RelatedCheckResult{TargetType: "asg", Count: -1}
	}
	var ids []string
	for _, asgRes := range asgList {
		asg, ok := assertStruct[asgtypes.AutoScalingGroup](asgRes.RawStruct)
		if !ok {
			continue
		}
		for _, t := range asg.Tags {
			if t.Key != nil && *t.Key == "AmazonECSManaged" {
				// ASGs managed by this cluster's capacity provider
				ids = append(ids, asgRes.ID)
				break
			}
			if t.Key != nil && *t.Key == "ClusterName" && t.Value != nil && *t.Value == clusterName {
				ids = append(ids, asgRes.ID)
				break
			}
		}
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("asg")
	}
	return relatedResult("asg", ids)
}

// checkECSEC2 scans the ec2 cache for instances running this ECS cluster's
// container instances (tagged "ecs:cluster-name").
func checkECSEC2(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	clusterName := res.ID
	if clusterName == "" {
		return resource.RelatedCheckResult{TargetType: "ec2", Count: 0}
	}
	ec2List, truncated, err := ecsRelatedResources(ctx, clients, cache, "ec2")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "ec2", Count: -1, Err: err}
	}
	if ec2List == nil {
		return resource.RelatedCheckResult{TargetType: "ec2", Count: -1}
	}
	var ids []string
	for _, ec2Res := range ec2List {
		inst, ok := assertStruct[ec2types.Instance](ec2Res.RawStruct)
		if !ok {
			continue
		}
		for _, t := range inst.Tags {
			if t.Key == nil || t.Value == nil {
				continue
			}
			if (*t.Key == "aws:ecs:cluster-name" || *t.Key == "ClusterName") && *t.Value == clusterName {
				ids = append(ids, ec2Res.ID)
				break
			}
		}
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("ec2")
	}
	return relatedResult("ec2", ids)
}

// checkECSCTEvents scans the ct-events cache for events whose Resources or
// requestParameters reference this ECS cluster.
func checkECSCTEvents(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	clusterName := res.ID
	if clusterName == "" {
		return resource.RelatedCheckResult{TargetType: "ct-events", Count: 0}
	}
	evList, truncated, err := ecsRelatedResources(ctx, clients, cache, "ct-events")
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
			if r.ResourceName == nil {
				continue
			}
			if strings.Contains(*r.ResourceName, clusterName) {
				ids = append(ids, evRes.ID)
				break
			}
		}
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("ct-events")
	}
	return relatedResult("ct-events", ids)
}

// checkECSTasks scans the ecs-task cache for tasks whose ClusterArn refers
// to this cluster.
func checkECSTasks(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	clusterName := res.ID
	if clusterName == "" {
		return resource.RelatedCheckResult{TargetType: "ecs-task", Count: 0}
	}
	taskList, truncated, err := ecsRelatedResources(ctx, clients, cache, "ecs-task")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "ecs-task", Count: -1, Err: err}
	}
	if taskList == nil {
		return resource.RelatedCheckResult{TargetType: "ecs-task", Count: -1}
	}
	var ids []string
	for _, tRes := range taskList {
		task, ok := assertStruct[ecstypes.Task](tRes.RawStruct)
		if !ok {
			continue
		}
		if task.ClusterArn == nil {
			continue
		}
		if *task.ClusterArn == clusterName || strings.HasSuffix(*task.ClusterArn, "/"+clusterName) {
			ids = append(ids, tRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("ecs-task")
	}
	return relatedResult("ecs-task", ids)
}

// checkECSLogs scans the logs cache for log groups associated with this
// cluster's task definitions. ECS convention uses /ecs/{family}; with no
// concrete family we match any log group whose ID contains the cluster name
// as a substring (weak signal).
func checkECSLogs(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	clusterName := res.ID
	if clusterName == "" {
		return resource.RelatedCheckResult{TargetType: "logs", Count: 0}
	}
	logList, truncated, err := ecsRelatedResources(ctx, clients, cache, "logs")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "logs", Count: -1, Err: err}
	}
	if logList == nil {
		return resource.RelatedCheckResult{TargetType: "logs", Count: -1}
	}
	var ids []string
	for _, logRes := range logList {
		if strings.Contains(logRes.ID, clusterName) {
			ids = append(ids, logRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("logs")
	}
	return relatedResult("logs", ids)
}

