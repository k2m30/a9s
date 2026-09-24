// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// ecs_related_extra.go contains additional ECS cluster related-
// resource checkers required by docs/related-resources.md beyond what is
// already registered in ecs.go.
package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// checkECSASG scans the asg cache for Auto Scaling Groups tagged with this
// ECS cluster's capacity provider (Pattern C). ECS cluster capacity providers
// reference ASG ARNs, but the Cluster struct exposes them only by name; the
// reverse link from ASG→cluster surfaces through the
// AmazonECSManaged tag that ECS adds to ASGs it manages.
func checkECSASG(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	clusterName := res.ID
	if clusterName == "" {
		return foundNone("asg", "clusterName")
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
	return relatedResultTrunc("asg", ids, truncated)
}

// checkECSEC2 scans the ec2 cache for instances running this ECS cluster's
// container instances (tagged "ecs:cluster-name").
func checkECSEC2(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	clusterName := res.ID
	if clusterName == "" {
		return foundNone("ec2", "clusterName")
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
	return relatedResultTrunc("ec2", ids, truncated)
}

// checkECSTasks scans the ecs-task cache for tasks whose ClusterArn refers
// to this cluster.
func checkECSTasks(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	clusterName := res.ID
	if clusterName == "" {
		return foundNone("ecs-task", "clusterName")
	}
	taskList, truncated, err := relatedResourcesFor(ctx, clients, cache, "ecs-task")
	if err != nil {
		return ReadFailed("ecs-task", err)
	}
	if taskList == nil {
		return NotRead("ecs-task")
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
		if name, _ := ecsRefToID(*task.ClusterArn, domain.RefContext{}); clusterName != "" && name == clusterName {
			ids = append(ids, tRes.ID)
		}
	}
	return relatedResultTrunc("ecs-task", ids, truncated)
}

// checkECSLogs reports the log group this cluster's ecs exec session
// transcripts are written to, the one log group a Cluster names:
// Configuration.ExecuteCommandConfiguration.LogConfiguration.CloudWatchLogGroupName.
// A cluster that sends its sessions nowhere names none.
func checkECSLogs(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	cluster, ok := assertStruct[ecstypes.Cluster](res.RawStruct)
	if !ok {
		return NotRead("logs")
	}
	group := ""
	if cfg := cluster.Configuration; cfg != nil && cfg.ExecuteCommandConfiguration != nil && cfg.ExecuteCommandConfiguration.LogConfiguration != nil {
		group = aws.ToString(cfg.ExecuteCommandConfiguration.LogConfiguration.CloudWatchLogGroupName)
	}
	if group == "" {
		return foundNone("logs", "the cluster's exec-command log configuration")
	}
	logList, _, err := relatedResourcesFor(ctx, clients, cache, "logs")
	if err != nil {
		return ReadFailed("logs", err)
	}
	if logList == nil {
		return NotRead("logs")
	}
	ids, lowerBound := listedRefs("logs", []string{group}, refContext(clients, cache, "logs"), logList)
	return relatedResultTrunc("logs", ids, lowerBound)
}
