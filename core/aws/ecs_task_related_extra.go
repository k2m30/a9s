// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// ecs_task_related_extra.go contains additional ECS task related-resource
// checkers required by docs/related-resources.md.
package aws

import (
	"context"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

func checkECSTaskAlarm(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	return alarmIDsByDimension(ctx, clients, cache, "ecs-task", res)
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
func checkECSTaskECR(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	task, ok := assertStruct[ecstypes.Task](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("ecr")
	}
	var images []string
	for _, c := range task.Containers {
		if image := aws.ToString(c.Image); image != "" {
			images = append(images, image)
		}
	}
	return ecrWorkloadRepos(ctx, clients, cache, images)
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
	if !taskDefJoined(res) {
		return resource.UnknownRelated("secrets")
	}
	joined := res.Fields["secret_arns"]
	if joined == "" {
		return resource.ProvenZero("secrets", "joined")
	}
	secretList, _, err := relatedResourcesFor(ctx, clients, cache, "secrets")
	if err != nil {
		return resource.ErrorRelated("secrets", err)
	}
	if secretList == nil {
		return resource.UnknownRelated("secrets")
	}

	ids, lowerBound := listedRefs("secrets", strings.Split(joined, ","), refContext(clients, cache, "secrets"), secretList)
	return relatedResultTrunc("secrets", ids, lowerBound)
}

// checkECSTaskSSM reads Fields["ssm_param_names"] (a comma-joined list of SSM
// parameter names emitted by the fetcher's ecsJoinTaskDefinition join —
// ContainerDefinitions[].Secrets[].ValueFrom filtered to the ssm ARN prefix
// or a bare "/"-prefixed parameter name) and cross-references the
// already-loaded ssm cache by name, per docs/resources/ecs-task.md.
func checkECSTaskSSM(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	if !taskDefJoined(res) {
		return resource.UnknownRelated("ssm")
	}
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
