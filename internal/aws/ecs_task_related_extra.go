// ecs_task_related_extra.go contains additional ECS task related-resource
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
		return resource.RelatedCheckResult{TargetType: "ct-events", Count: 0}
	}
	evList, truncated, err := ecsTaskRelatedResources(ctx, clients, cache, "ct-events")
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

// checkECSTaskEC2 extracts container-instance EC2 IDs from task.ContainerInstanceArn.
// For Fargate tasks this is absent → Count:0.
func checkECSTaskEC2(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	task, ok := assertStruct[ecstypes.Task](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("ec2")
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
		return resource.UnknownRelated("ecr")
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
		return resource.RelatedCheckResult{TargetType: "eni", Count: 0}
	}
	return relatedResult("eni", ids)
}

// checkECSTaskSecrets reads Fields["secret_arns"] (a comma-joined list of
// Secrets Manager ARNs emitted by the fetcher's ecsJoinTaskDefinition join —
// ContainerDefinitions[].Secrets[].ValueFrom and
// ContainerDefinitions[].RepositoryCredentials.CredentialsParameter, filtered
// to the secretsmanager ARN prefix) and cross-references the already-loaded
// secrets cache by ARN, per docs/resources/ecs-task.md.
func checkECSTaskSecrets(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	joined := res.Fields["secret_arns"]
	if joined == "" {
		return resource.RelatedCheckResult{TargetType: "secrets", Count: 0}
	}
	arnSet := make(map[string]struct{})
	for arn := range strings.SplitSeq(joined, ",") {
		if arn != "" {
			arnSet[arn] = struct{}{}
		}
	}
	if len(arnSet) == 0 {
		return resource.RelatedCheckResult{TargetType: "secrets", Count: 0}
	}

	secretList, truncated, err := ecsTaskRelatedResources(ctx, clients, cache, "secrets")
	if err != nil {
		return resource.ErrorRelated("secrets", err)
	}
	if secretList == nil {
		return resource.UnknownRelated("secrets")
	}

	var ids []string
	for _, sRes := range secretList {
		if _, match := arnSet[sRes.ID]; match {
			ids = append(ids, sRes.ID)
			continue
		}
		if arn := sRes.Fields["arn"]; arn != "" {
			if _, match := arnSet[arn]; match {
				ids = append(ids, sRes.ID)
			}
		}
	}
	return relatedResultTrunc("secrets", ids, truncated)
}

// checkECSTaskSSM reads Fields["ssm_param_names"] (a comma-joined list of SSM
// parameter names emitted by the fetcher's ecsJoinTaskDefinition join —
// ContainerDefinitions[].Secrets[].ValueFrom filtered to the ssm ARN prefix
// or a bare "/"-prefixed parameter name) and cross-references the
// already-loaded ssm cache by name, per docs/resources/ecs-task.md.
func checkECSTaskSSM(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	joined := res.Fields["ssm_param_names"]
	if joined == "" {
		return resource.RelatedCheckResult{TargetType: "ssm", Count: 0}
	}
	nameSet := make(map[string]struct{})
	for name := range strings.SplitSeq(joined, ",") {
		if name != "" {
			nameSet[name] = struct{}{}
		}
	}
	if len(nameSet) == 0 {
		return resource.RelatedCheckResult{TargetType: "ssm", Count: 0}
	}

	ssmList, truncated, err := ecsTaskRelatedResources(ctx, clients, cache, "ssm")
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
		return resource.RelatedCheckResult{TargetType: "sg", Count: 0}
	}

	eniList, eniTruncated, err := ecsTaskRelatedResources(ctx, clients, cache, "eni")
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
		return resource.RelatedCheckResult{TargetType: "sg", Count: 0}
	}

	sgList, sgTruncated, err := ecsTaskRelatedResources(ctx, clients, cache, "sg")
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
	return relatedResultTrunc("sg", ids, sgTruncated)
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
		return resource.RelatedCheckResult{TargetType: "subnet", Count: 0}
	}
	return relatedResult("subnet", ids)
}
