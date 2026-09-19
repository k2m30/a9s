// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// ecs_task_related_extra.go contains additional ECS task related-resource
// checkers required by docs/related-resources.md.
package aws

import (
	"context"
	"strings"

	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// checkECSTaskAlarm scans the alarm cache for alarms with a TaskDefinition or
// TaskArn dimension matching this task.
func checkECSTaskAlarm(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	taskID := res.ID
	if taskID == "" {
		return resource.ProvenZero("alarm", "taskID")
	}
	alarmList, truncated, err := relatedResourcesFor(ctx, clients, cache, "alarm")
	if err != nil {
		return resource.ErrorRelated("alarm", err)
	}
	if alarmList == nil {
		return resource.UnknownRelated("alarm")
	}
	var ids []string
	for _, alarmRes := range alarmList {
		alarm, ok := assertStruct[cwtypes.MetricAlarm](alarmRes.RawStruct)
		if !ok {
			continue
		}
		for _, d := range alarm.Dimensions {
			if d.Name == nil || d.Value == nil {
				continue
			}
			if (*d.Name == "TaskId" || *d.Name == "TaskArn") && strings.Contains(*d.Value, taskID) {
				ids = append(ids, alarmRes.ID)
				break
			}
		}
	}
	return relatedResultTrunc("alarm", ids, truncated)
}

// checkECSTaskCTEvents scans ct-events for events involving this task.
func checkECSTaskCTEvents(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	taskID := res.ID
	if taskID == "" {
		return resource.ProvenZero("ct-events", "taskID")
	}
	evList, truncated, err := relatedResourcesFor(ctx, clients, cache, "ct-events")
	if err != nil {
		return resource.ErrorRelated("ct-events", err)
	}
	if evList == nil {
		return resource.UnknownRelated("ct-events")
	}
	var ids []string
	for _, evRes := range evList {
		ev, ok := assertStruct[cloudtrailtypes.Event](evRes.RawStruct)
		if !ok {
			continue
		}
		for _, r := range ev.Resources {
			if r.ResourceName != nil && strings.Contains(*r.ResourceName, taskID) {
				ids = append(ids, evRes.ID)
				break
			}
		}
	}
	return relatedResultTrunc("ct-events", ids, truncated)
}

// checkECSTaskEC2 reports the EC2 instance an EC2-launch-type task runs on:
// the Ec2InstanceId of its container instance (ecs:DescribeContainerInstances).
// A Fargate task has no container instance → Count:0.
func checkECSTaskEC2(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	task, ok := assertStruct[ecstypes.Task](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("ec2")
	}
	if task.ContainerInstanceArn == nil || *task.ContainerInstanceArn == "" {
		return resource.ProvenZero("ec2", "task.ContainerInstanceArn")
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil {
		return resource.UnknownRelated("ec2")
	}
	api, ok := c.ECS.(ECSDescribeContainerInstancesAPI)
	if !ok {
		return resource.UnknownRelated("ec2")
	}
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*ecs.DescribeContainerInstancesOutput, error) {
		return api.DescribeContainerInstances(ctx, &ecs.DescribeContainerInstancesInput{
			Cluster:            task.ClusterArn,
			ContainerInstances: []string{*task.ContainerInstanceArn},
		})
	})
	if err != nil {
		return resource.ErrorRelated("ec2", err)
	}
	var ids []string
	for _, ci := range out.ContainerInstances {
		if ci.Ec2InstanceId != nil {
			ids = append(ids, *ci.Ec2InstanceId)
		}
	}
	ids, dropped := resolveRefs("ec2", ids, refContext(clients, cache, "ec2"))
	return relatedResultTrunc("ec2", ids, dropped || len(out.Failures) > 0)
}

// checkECSTaskECR reads the ECR repositories of the task's container image
// URIs. Pattern F — requires Containers[].Image to be populated in Task.
func checkECSTaskECR(_ context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	task, ok := assertStruct[ecstypes.Task](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("ecr")
	}
	var images []string
	for _, c := range task.Containers {
		if c.Image != nil && strings.Contains(*c.Image, ".dkr.ecr.") {
			images = append(images, *c.Image)
		}
	}
	return relatedRefs("ecr", images, refContext(clients, cache, "ecr"))
}

// checkECSTaskENI extracts ENI IDs from task.Attachments (awsvpc mode). Pattern F.
func checkECSTaskENI(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	task, ok := assertStruct[ecstypes.Task](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("eni")
	}
	var ids []string
	for _, att := range task.Attachments {
		if att.Type != nil && strings.EqualFold(*att.Type, "ElasticNetworkInterface") {
			for _, d := range att.Details {
				if d.Name != nil && *d.Name == "networkInterfaceId" && d.Value != nil && *d.Value != "" {
					ids = append(ids, *d.Value)
				}
			}
		}
	}
	if len(ids) == 0 {
		return resource.ProvenZero("eni", "ids")
	}
	return relatedResultTrunc("eni", ids, false)
}

// checkECSTaskSecrets reads Fields["secret_arns"] (a comma-joined list of
// Secrets Manager ARNs emitted by the fetcher's ecsJoinTaskDefinition join —
// ContainerDefinitions[].Secrets[].ValueFrom and
// ContainerDefinitions[].RepositoryCredentials.CredentialsParameter, filtered
// to the secretsmanager ARN prefix) and cross-references the already-loaded
// secrets cache, per docs/resources/ecs-task.md.
func checkECSTaskSecrets(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	joined := res.Fields["secret_arns"]
	if joined == "" {
		return resource.ProvenZero("secrets", "joined")
	}
	secretList, truncated, err := relatedResourcesFor(ctx, clients, cache, "secrets")
	if err != nil {
		return resource.ErrorRelated("secrets", err)
	}
	if secretList == nil {
		return resource.UnknownRelated("secrets")
	}

	ids, dropped := listedRefs("secrets", strings.Split(joined, ","), refContext(clients, cache, "secrets"), secretList)
	return relatedResultTrunc("secrets", ids, truncated || dropped)
}

// checkECSTaskSSM reads Fields["ssm_param_names"] (a comma-joined list of SSM
// parameter names emitted by the fetcher's ecsJoinTaskDefinition join —
// ContainerDefinitions[].Secrets[].ValueFrom filtered to the ssm ARN prefix
// or a bare "/"-prefixed parameter name) and cross-references the
// already-loaded ssm cache by name, per docs/resources/ecs-task.md.
func checkECSTaskSSM(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	joined := res.Fields["ssm_param_names"]
	if joined == "" {
		return resource.ProvenZero("ssm", "joined")
	}
	nameSet := make(map[string]struct{})
	for name := range strings.SplitSeq(joined, ",") {
		if name != "" {
			nameSet[name] = struct{}{}
		}
	}
	if len(nameSet) == 0 {
		return resource.ProvenZero("ssm", "nameSet")
	}

	ssmList, truncated, err := relatedResourcesFor(ctx, clients, cache, "ssm")
	if err != nil {
		return resource.ErrorRelated("ssm", err)
	}
	if ssmList == nil {
		return resource.UnknownRelated("ssm")
	}

	var ids []string
	for _, pRes := range ssmList {
		if _, match := nameSet[pRes.ID]; match {
			ids = append(ids, pRes.ID)
			continue
		}
		if _, match := nameSet[pRes.Name]; match {
			ids = append(ids, pRes.ID)
		}
	}
	return relatedResultTrunc("ssm", ids, truncated)
}

// checkECSTaskSG chains Task -> ENI -> SG per docs/resources/ecs-task.md:
// derive the task's ENI id from task.Attachments (awsvpc mode), cross-
// reference the already-loaded eni cache to read Fields["security_groups"]
// on that ENI row, then cross-reference the already-loaded sg cache by ID.
// No extra API call beyond what the eni cache already consumed.
func checkECSTaskSG(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	task, ok := assertStruct[ecstypes.Task](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("sg")
	}
	var eniIDs []string
	for _, att := range task.Attachments {
		if att.Type != nil && strings.EqualFold(*att.Type, "ElasticNetworkInterface") {
			for _, d := range att.Details {
				if d.Name != nil && *d.Name == "networkInterfaceId" && d.Value != nil && *d.Value != "" {
					eniIDs = append(eniIDs, *d.Value)
				}
			}
		}
	}
	if len(eniIDs) == 0 {
		return resource.ProvenZero("sg", "eniIDs")
	}

	eniList, eniTruncated, err := relatedResourcesFor(ctx, clients, cache, "eni")
	if err != nil {
		return resource.ErrorRelated("sg", err)
	}
	if eniList == nil {
		return resource.UnknownRelated("sg")
	}

	eniIDSet := make(map[string]struct{}, len(eniIDs))
	for _, id := range eniIDs {
		eniIDSet[id] = struct{}{}
	}
	sgIDSet := make(map[string]struct{})
	for _, eniRes := range eniList {
		if _, match := eniIDSet[eniRes.ID]; !match {
			continue
		}
		for sgID := range strings.SplitSeq(eniRes.Fields["security_groups"], ",") {
			if sgID != "" {
				sgIDSet[sgID] = struct{}{}
			}
		}
	}
	if len(sgIDSet) == 0 {
		if eniTruncated {
			return resource.UnknownRelated("sg")
		}
		return resource.ProvenZero("sg", "sgIDSet")
	}

	sgList, sgTruncated, err := relatedResourcesFor(ctx, clients, cache, "sg")
	if err != nil {
		return resource.ErrorRelated("sg", err)
	}
	if sgList == nil {
		return resource.UnknownRelated("sg")
	}

	var ids []string
	for _, sgRes := range sgList {
		if _, match := sgIDSet[sgRes.ID]; match {
			ids = append(ids, sgRes.ID)
		}
	}
	return relatedResultTrunc("sg", ids, sgTruncated || eniTruncated)
}

// checkECSTaskSubnet extracts subnet IDs from task.Attachments (awsvpc). Pattern F.
func checkECSTaskSubnet(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	task, ok := assertStruct[ecstypes.Task](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("subnet")
	}
	seen := make(map[string]struct{})
	for _, att := range task.Attachments {
		if att.Type != nil && strings.EqualFold(*att.Type, "ElasticNetworkInterface") {
			for _, d := range att.Details {
				if d.Name != nil && *d.Name == "subnetId" && d.Value != nil && *d.Value != "" {
					seen[*d.Value] = struct{}{}
				}
			}
		}
	}
	var ids []string
	for id := range seen {
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return resource.ProvenZero("subnet", "ids")
	}
	return relatedResultTrunc("subnet", ids, false)
}
