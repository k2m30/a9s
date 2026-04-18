// ecs_tasks_related_extra.go contains additional ECS task related-resource
// checkers required by docs/related-resources.md.
package aws

import (
	"context"
	"strings"

	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// checkECSTaskAlarm scans the alarm cache for alarms with a TaskDefinition or
// TaskArn dimension matching this task.
func checkECSTaskAlarm(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	taskID := res.ID
	if taskID == "" {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: 0}
	}
	alarmList, truncated, err := ecsTaskRelatedResources(ctx, clients, cache, "alarm")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: -1, Err: err}
	}
	if alarmList == nil {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: -1}
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
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: -1}
	}
	return relatedResult("alarm", ids)
}

// checkECSTaskCTEvents scans ct-events for events involving this task.
func checkECSTaskCTEvents(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	taskID := res.ID
	if taskID == "" {
		return resource.RelatedCheckResult{TargetType: "ct-events", Count: 0}
	}
	evList, truncated, err := ecsTaskRelatedResources(ctx, clients, cache, "ct-events")
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
			if r.ResourceName != nil && strings.Contains(*r.ResourceName, taskID) {
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

// checkECSTaskEC2 extracts container-instance EC2 IDs from task.ContainerInstanceArn.
// For Fargate tasks this is absent → Count:0.
func checkECSTaskEC2(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	task, ok := assertStruct[ecstypes.Task](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "ec2", Count: -1}
	}
	if task.ContainerInstanceArn == nil || *task.ContainerInstanceArn == "" {
		return resource.RelatedCheckResult{TargetType: "ec2", Count: 0}
	}
	// ContainerInstanceArn: arn:aws:ecs:region:account:container-instance/cluster/uuid
	// The backing EC2 instance ID is not in this ARN — it's on the container
	// instance metadata. Return the container-instance UUID as a surfaced link.
	arn := *task.ContainerInstanceArn
	parts := strings.Split(arn, "/")
	name := parts[len(parts)-1]
	if name == "" {
		return resource.RelatedCheckResult{TargetType: "ec2", Count: 0}
	}
	return relatedResult("ec2", []string{name})
}

// checkECSTaskECR extracts ECR repository names from the task's container
// image URIs. Pattern F — requires Containers[].Image to be populated in Task.
func checkECSTaskECR(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	task, ok := assertStruct[ecstypes.Task](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "ecr", Count: -1}
	}
	seen := make(map[string]struct{})
	for _, c := range task.Containers {
		if c.Image == nil || *c.Image == "" {
			continue
		}
		img := *c.Image
		// ECR image URI: {account}.dkr.ecr.{region}.amazonaws.com/{repo}:tag
		if !strings.Contains(img, ".dkr.ecr.") {
			continue
		}
		_, repo, ok := strings.Cut(img, "/")
		if !ok {
			continue
		}
		if before, _, hasSep := strings.Cut(repo, ":"); hasSep {
			repo = before
		}
		if before, _, hasSep := strings.Cut(repo, "@"); hasSep {
			repo = before
		}
		if repo != "" {
			seen[repo] = struct{}{}
		}
	}
	var ids []string
	for id := range seen {
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return resource.RelatedCheckResult{TargetType: "ecr", Count: 0}
	}
	return relatedResult("ecr", ids)
}

// checkECSTaskENI extracts ENI IDs from task.Attachments (awsvpc mode). Pattern F.
func checkECSTaskENI(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	task, ok := assertStruct[ecstypes.Task](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "eni", Count: -1}
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
		return resource.RelatedCheckResult{TargetType: "eni", Count: 0}
	}
	return relatedResult("eni", ids)
}

// checkECSTaskSecrets scans ContainerDefinitions[].Secrets[].ValueFrom for secretsmanager ARNs.
// Pattern F — no AWS call needed; all data is in RawStruct (ecstypes.TaskDefinition).
// NOTE: The registration in ecs_tasks_related.go passes ecstypes.Task, but the Secrets
// field lives on TaskDefinition. The fetcher stores the TaskDefinition in the Resource
// RawStruct (res.RawStruct may be *ecstypes.TaskDefinition). Try both types.
func checkECSTaskSecrets(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	var containers []ecstypes.ContainerDefinition
	if td, ok := assertStruct[ecstypes.TaskDefinition](res.RawStruct); ok {
		containers = td.ContainerDefinitions
	} else if task, ok := assertStruct[ecstypes.Task](res.RawStruct); ok {
		// ecstypes.Task does not carry ContainerDefinitions — return 0.
		_ = task
		return resource.RelatedCheckResult{TargetType: "secrets", Count: 0}
	} else {
		return resource.RelatedCheckResult{TargetType: "secrets", Count: -1}
	}

	seen := make(map[string]struct{})
	for _, c := range containers {
		for _, s := range c.Secrets {
			if s.ValueFrom == nil || *s.ValueFrom == "" {
				continue
			}
			if strings.HasPrefix(*s.ValueFrom, "arn:aws:secretsmanager:") {
				seen[*s.ValueFrom] = struct{}{}
			}
		}
	}
	var ids []string
	for id := range seen {
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return resource.RelatedCheckResult{TargetType: "secrets", Count: 0}
	}
	return relatedResult("secrets", ids)
}

// checkECSTaskSSM scans ContainerDefinitions[].Secrets[].ValueFrom for SSM parameter ARNs.
// Pattern F — no AWS call needed; all data is in RawStruct.
// Returns SSM parameter names extracted from the ARN suffix after "/".
func checkECSTaskSSM(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	var containers []ecstypes.ContainerDefinition
	if td, ok := assertStruct[ecstypes.TaskDefinition](res.RawStruct); ok {
		containers = td.ContainerDefinitions
	} else if task, ok := assertStruct[ecstypes.Task](res.RawStruct); ok {
		_ = task
		return resource.RelatedCheckResult{TargetType: "ssm", Count: 0}
	} else {
		return resource.RelatedCheckResult{TargetType: "ssm", Count: -1}
	}

	// SSM parameter ARN form: arn:aws:ssm:<region>:<account>:parameter/<name>
	const ssmPrefix = "arn:aws:ssm:"
	const paramPart = ":parameter/"
	seen := make(map[string]struct{})
	for _, c := range containers {
		for _, s := range c.Secrets {
			if s.ValueFrom == nil || *s.ValueFrom == "" {
				continue
			}
			v := *s.ValueFrom
			if !strings.HasPrefix(v, ssmPrefix) {
				continue
			}
			if _, after, found := strings.Cut(v, paramPart); found && after != "" {
				seen[after] = struct{}{}
			}
		}
	}
	var ids []string
	for id := range seen {
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return resource.RelatedCheckResult{TargetType: "ssm", Count: 0}
	}
	return relatedResult("ssm", ids)
}

// checkECSTaskSG extracts security group IDs from task.Attachments (awsvpc). Pattern F.
func checkECSTaskSG(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	task, ok := assertStruct[ecstypes.Task](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "sg", Count: -1}
	}
	// SGs are on the service's NetworkConfiguration, not the Task. The Task
	// Attachments.Details don't carry SG IDs directly — they live in the
	// parent service/run-task call. Return Count:0 here.
	_ = task
	return resource.RelatedCheckResult{TargetType: "sg", Count: 0}
}

// checkECSTaskSubnet extracts subnet IDs from task.Attachments (awsvpc). Pattern F.
func checkECSTaskSubnet(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	task, ok := assertStruct[ecstypes.Task](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "subnet", Count: -1}
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
		return resource.RelatedCheckResult{TargetType: "subnet", Count: 0}
	}
	return relatedResult("subnet", ids)
}
