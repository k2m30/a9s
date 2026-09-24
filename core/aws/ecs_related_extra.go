// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// ecs_related_extra.go contains additional ECS cluster related-
// resource checkers required by docs/related-resources.md beyond what is
// already registered in ecs.go.
package aws

import (
	"context"
	"slices"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// checkECSASG reports the Auto Scaling groups of this cluster's capacity
// providers: ecs:DescribeCapacityProviders over the cluster's
// CapacityProviders names each one's
// AutoScalingGroupProvider.AutoScalingGroupArn, "the Amazon Resource Name
// (ARN) that identifies the Auto Scaling group, or the Auto Scaling group
// name". FARGATE and FARGATE_SPOT carry no group.
// https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_AutoScalingGroupProvider.html
func checkECSASG(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	cluster, ok := assertStruct[ecstypes.Cluster](res.RawStruct)
	if !ok {
		return NotRead("asg")
	}
	if len(cluster.CapacityProviders) == 0 {
		return foundNone("asg", "cluster.CapacityProviders")
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil {
		return NotRead("asg")
	}
	api, ok := c.ECS.(ECSDescribeCapacityProvidersAPI)
	if !ok {
		return NotRead("asg")
	}
	failed := false
	providers, complete, err := PageAll(ctx, PerParentPageCap, func(ctx context.Context, token *string) ([]ecstypes.CapacityProvider, *string, error) {
		out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*ecs.DescribeCapacityProvidersOutput, error) {
			return api.DescribeCapacityProviders(ctx, &ecs.DescribeCapacityProvidersInput{CapacityProviders: cluster.CapacityProviders, NextToken: token})
		})
		if err != nil {
			return nil, nil, err
		}
		failed = failed || len(out.Failures) > 0
		return out.CapacityProviders, out.NextToken, nil
	})
	if err != nil {
		return ReadFailed("asg", err)
	}
	var refs []string
	for _, p := range providers {
		if p.AutoScalingGroupProvider != nil {
			refs = append(refs, aws.ToString(p.AutoScalingGroupProvider.AutoScalingGroupArn))
		}
	}
	return listedRelated(ctx, clients, cache, "asg", refs, failed || !complete)
}

// checkECSEC2 reports the EC2 instances registered with this cluster as
// container instances: ecs:ListContainerInstances, then
// ecs:DescribeContainerInstances (up to 100 per call) for each one's
// ec2InstanceId — "for Amazon EC2 instances, this value is the Amazon EC2
// instance ID. For external instances, this value is the AWS Systems Manager
// managed instance ID", which is no EC2 instance.
// https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_ContainerInstance.html
// https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_DescribeContainerInstances.html
func checkECSEC2(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	clusterRef := res.ID
	if cluster, ok := assertStruct[ecstypes.Cluster](res.RawStruct); ok && cluster.ClusterArn != nil {
		clusterRef = *cluster.ClusterArn
	}
	if clusterRef == "" {
		return keyMissing("ec2", "clusterName")
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil {
		return NotRead("ec2")
	}
	lister, ok := c.ECS.(ECSListContainerInstancesAPI)
	if !ok {
		return NotRead("ec2")
	}
	describer, ok := c.ECS.(ECSDescribeContainerInstancesAPI)
	if !ok {
		return NotRead("ec2")
	}
	arns, complete, err := PageAll(ctx, PerParentPageCap, func(ctx context.Context, token *string) ([]string, *string, error) {
		out, err := lister.ListContainerInstances(ctx, &ecs.ListContainerInstancesInput{Cluster: &clusterRef, NextToken: token})
		if err != nil {
			return nil, nil, err
		}
		return out.ContainerInstanceArns, out.NextToken, nil
	})
	if err != nil {
		return ReadFailed("ec2", err)
	}
	var ids []string
	for batch := range slices.Chunk(arns, containerInstancesPerDescribe) {
		out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*ecs.DescribeContainerInstancesOutput, error) {
			return describer.DescribeContainerInstances(ctx, &ecs.DescribeContainerInstancesInput{Cluster: &clusterRef, ContainerInstances: batch})
		})
		if err != nil {
			return ReadFailed("ec2", err)
		}
		complete = complete && len(out.Failures) == 0
		for _, ci := range out.ContainerInstances {
			if id := aws.ToString(ci.Ec2InstanceId); strings.HasPrefix(id, "i-") && ecsContainerInstanceMember(ci) {
				ids = append(ids, id)
			}
		}
	}
	return listedRelated(ctx, clients, cache, "ec2", ids, !complete)
}

// ecsContainerInstanceMember is the one predicate over ContainerInstance.Status
// for cluster membership: a REGISTRATION_FAILED instance never joined, a
// DEREGISTERING one "is terminated" and leaving, an INACTIVE one has left
// (https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_ContainerInstance.html).
func ecsContainerInstanceMember(ci ecstypes.ContainerInstance) bool {
	switch aws.ToString(ci.Status) {
	case "REGISTRATION_FAILED", "DEREGISTERING", "INACTIVE":
		return false
	}
	return true
}

// containerInstancesPerDescribe is DescribeContainerInstances' limit: "a list
// of up to 100 container instance IDs or full Amazon Resource Name (ARN)
// entries".
const containerInstancesPerDescribe = 100

// checkECSTasks scans the ecs-task cache for tasks whose ClusterArn refers
// to this cluster.
func checkECSTasks(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	clusterName := res.ID
	if clusterName == "" {
		return keyMissing("ecs-task", "clusterName")
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
	// Only OVERRIDE sends exec sessions to logConfiguration: NONE logs
	// nothing and DEFAULT uses the task definition's awslogs
	// (https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_ExecuteCommandConfiguration.html).
	group := ""
	if cfg := cluster.Configuration; cfg != nil && cfg.ExecuteCommandConfiguration != nil && cfg.ExecuteCommandConfiguration.LogConfiguration != nil &&
		cfg.ExecuteCommandConfiguration.Logging == ecstypes.ExecuteCommandLoggingOverride {
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
