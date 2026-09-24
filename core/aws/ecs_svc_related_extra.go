// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// ecs_svc_related_extra.go contains ECS service related-resource checkers
// spilled out of ecs_svc_related.go — includes eb-rule/ecr/secrets/sfn and
// any other overflow targets required by docs/related-resources.md.
package aws

import (
	"cmp"
	"context"
	"encoding/json"
	"slices"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	eventbridgetypes "github.com/aws/aws-sdk-go-v2/service/eventbridge/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// checkECSSvcTasks scans the ecs-task cache for tasks belonging to this
// service. A task names its service by the bare name in Group, which is the
// name of a service in the task's own cluster — another cluster's service of
// that name runs none of these tasks. A row that cannot say which cluster it
// is in holds every task of the name, since nothing says otherwise.
func checkECSSvcTasks(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	svcName := ecsSvcName(res)
	if svcName == "" {
		return foundNone("ecs-task", "svcName")
	}
	cluster := res.Fields["cluster"]
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
		ref, ofService := ecsSvcRefFromTask(tRes, task)
		if !ofService {
			continue
		}
		if ref == ecsSvcID(cluster, svcName) || (cluster == "" && aws.ToString(task.Group) == "service:"+svcName) {
			ids = append(ids, tRes.ID)
		}
	}
	return relatedResultTrunc("ecs-task", ids, truncated)
}

// checkECSSvcSubnet extracts subnet IDs from Service's
// NetworkConfiguration.AwsvpcConfiguration.Subnets. Pattern F.
func checkECSSvcSubnet(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ecstypes.Service](res.RawStruct)
	if !ok {
		return NotRead("subnet")
	}
	if raw.NetworkConfiguration == nil || raw.NetworkConfiguration.AwsvpcConfiguration == nil {
		return foundNone("subnet", "raw.NetworkConfiguration.AwsvpcConfiguration")
	}
	var ids []string
	for _, s := range raw.NetworkConfiguration.AwsvpcConfiguration.Subnets {
		if s != "" {
			ids = append(ids, s)
		}
	}
	return relatedResultTrunc("subnet", ids, false)
}

// checkECSSvcVPC derives the VPC from the service's subnets. Pattern C:
// look up each subnet in the subnet cache and collect VpcIds.
func checkECSSvcVPC(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ecstypes.Service](res.RawStruct)
	if !ok {
		return NotRead("vpc")
	}
	if raw.NetworkConfiguration == nil || raw.NetworkConfiguration.AwsvpcConfiguration == nil {
		return foundNone("vpc", "raw.NetworkConfiguration.AwsvpcConfiguration")
	}
	subnetIDs := raw.NetworkConfiguration.AwsvpcConfiguration.Subnets
	if len(subnetIDs) == 0 {
		return foundNone("vpc", "subnetIDs")
	}
	subnetList, truncated, err := relatedResourcesFor(ctx, clients, cache, "subnet")
	if err != nil {
		return ReadFailed("vpc", err)
	}
	if subnetList == nil {
		return NotRead("vpc")
	}
	wanted := make(map[string]struct{}, len(subnetIDs))
	for _, s := range subnetIDs {
		wanted[s] = struct{}{}
	}
	vpcSet := make(map[string]struct{})
	for _, sRes := range subnetList {
		if _, ok := wanted[sRes.ID]; !ok {
			continue
		}
		if v := sRes.Fields["vpc_id"]; v != "" {
			vpcSet[v] = struct{}{}
		}
	}
	var ids []string
	for v := range vpcSet {
		ids = append(ids, v)
	}
	return relatedResultTrunc("vpc", ids, truncated)
}

// checkECSSvcEbRule counts the eb-rule rows whose event pattern matches an
// event ECS emits about this service, and the rules that run the service's
// task definition family on its cluster: a scheduled or event-driven task is a
// rule whose target is the cluster, with the task definition in its
// EcsParameters.
func checkECSSvcEbRule(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	svcName := ecsSvcName(res)
	if svcName == "" {
		return foundNone("eb-rule", "svcName")
	}
	raw, _ := assertStruct[ecstypes.Service](res.RawStruct)
	clusterARN := aws.ToString(raw.ClusterArn)
	family := taskDefFamily(cmp.Or(aws.ToString(raw.TaskDefinition), res.Fields["task_definition"]))

	ruleList, truncated, err := relatedResourcesFor(ctx, clients, cache, "eb-rule")
	if err != nil {
		return ReadFailed("eb-rule", err)
	}
	if ruleList == nil {
		return NotRead("eb-rule")
	}
	read := ebRulesMatching(ruleList, truncated, ecsServiceEvents(svcName, aws.ToString(raw.ServiceArn), clusterARN))
	runs, err := ebTargetRead(ctx, clients, clusterARN, func(targets []eventbridgetypes.Target) bool {
		return slices.ContainsFunc(targets, func(t eventbridgetypes.Target) bool {
			return t.EcsParameters != nil && aws.ToString(t.Arn) == clusterARN &&
				family != "" && taskDefFamily(aws.ToString(t.EcsParameters.TaskDefinitionArn)) == family
		})
	})
	if err != nil {
		runs = relatedRead{unread: true, failure: err}
	}
	return relatedAnswer("eb-rule", relatedRead{
		ids:     append(read.ids, runs.ids...),
		partial: read.partial || runs.partial,
		unread:  runs.unread,
		failure: runs.failure,
	})
}

// checkECSSvcECR resolves ECR repositories used by this ECS service.
// Pattern A: calls ecs:DescribeTaskDefinition for the service's current task
// definition and reads the ECR repositories of ContainerDefinitions[].Image.
// NeedsTargetCache: false.
func checkECSSvcECR(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ecstypes.Service](res.RawStruct)
	if !ok {
		return NotRead("ecr")
	}
	if raw.TaskDefinition == nil || *raw.TaskDefinition == "" {
		return foundNone("ecr", "raw.TaskDefinition")
	}
	taskDefARN := *raw.TaskDefinition

	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.ECS == nil {
		return NotRead("ecr")
	}
	api, ok := c.ECS.(ECSDescribeTaskDefinitionAPI)
	if !ok {
		return NotRead("ecr")
	}

	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*ecs.DescribeTaskDefinitionOutput, error) {
		return api.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{
			TaskDefinition: &taskDefARN,
		})
	})
	// The related panel's error result carries the refusal to the pivot.
	// no finding: this arm already answers with it.
	if err != nil || out.TaskDefinition == nil {
		return ReadFailed("ecr", err)
	}

	var images []string
	for _, cd := range out.TaskDefinition.ContainerDefinitions {
		if image := aws.ToString(cd.Image); image != "" {
			images = append(images, image)
		}
	}
	return ecrWorkloadRepos(ctx, clients, cache, images)
}

// checkECSSvcSecrets resolves Secrets Manager secrets referenced by this ECS service.
// Pattern A: calls ecs:DescribeTaskDefinition and inspects
// ContainerDefinitions[].Secrets[].ValueFrom for secretsmanager ARNs, plus
// ContainerDefinitions[].RepositoryCredentials.CredentialsParameter.
// NeedsTargetCache: false.
func checkECSSvcSecrets(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ecstypes.Service](res.RawStruct)
	if !ok {
		return NotRead("secrets")
	}
	if raw.TaskDefinition == nil || *raw.TaskDefinition == "" {
		return foundNone("secrets", "raw.TaskDefinition")
	}
	taskDefARN := *raw.TaskDefinition

	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.ECS == nil {
		return NotRead("secrets")
	}
	api, ok := c.ECS.(ECSDescribeTaskDefinitionAPI)
	if !ok {
		return NotRead("secrets")
	}

	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*ecs.DescribeTaskDefinitionOutput, error) {
		return api.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{
			TaskDefinition: &taskDefARN,
		})
	})
	// As in checkECSSvcECR above, the pivot is told the call refused rather
	// than shown a count.
	// no finding: this arm already answers with the error result.
	if err != nil || out.TaskDefinition == nil {
		return ReadFailed("secrets", err)
	}

	var refs []string
	for _, cd := range out.TaskDefinition.ContainerDefinitions {
		for _, s := range cd.Secrets {
			if s.ValueFrom == nil || *s.ValueFrom == "" {
				continue
			}
			v := *s.ValueFrom
			if isSecret(v) {
				refs = append(refs, v)
			}
		}
		// RepositoryCredentials.CredentialsParameter — may be a Secrets Manager ARN
		if cd.RepositoryCredentials != nil && cd.RepositoryCredentials.CredentialsParameter != nil {
			cp := *cd.RepositoryCredentials.CredentialsParameter
			if isSecret(cp) {
				refs = append(refs, cp)
			}
		}
	}

	return listedRelated(ctx, clients, cache, "secrets", refs, false)
}

// checkECSSvcSFN is a reverse-scan checker for the ecs-svc→sfn relationship.
// Pattern C+reverse: iterate cache["sfn"]; for each state machine call
// sfnDescribe and parse the ASL definition for states with
// Resource "arn:aws:states:::ecs:runTask*" whose Parameters.TaskDefinition
// matches the task definition family of this service.
// NeedsTargetCache: true.
func checkECSSvcSFN(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	if res.RawStruct == nil {
		return NotRead("sfn")
	}
	raw, ok := assertStruct[ecstypes.Service](res.RawStruct)
	if !ok {
		return NotRead("sfn")
	}
	if raw.TaskDefinition == nil || *raw.TaskDefinition == "" {
		return unreadZero(res, foundNone("sfn", "raw.TaskDefinition"))
	}

	family := taskDefFamily(*raw.TaskDefinition)
	if family == "" {
		return unreadZero(res, foundNone("sfn", "family"))
	}

	sfnList, truncated, err := relatedResourcesFor(ctx, clients, cache, "sfn")
	if err != nil {
		return ReadFailed("sfn", err)
	}
	if sfnList == nil {
		return NotRead("sfn")
	}

	var ids []string
	var reads rowReads
	for _, sfnRes := range sfnList {
		sfnARN := sfnRes.Fields["arn"]
		if sfnARN == "" {
			reads.missed()
			continue
		}
		sm, err := sfnDescribe(ctx, clients, sfnARN)
		if err != nil {
			reads.fail(sfnRes.ID, err)
			continue
		}
		reads.read++
		if sm == nil || sm.Definition == nil || *sm.Definition == "" {
			continue
		}
		runs, undecided := sfnASLRunsECSFamily(*sm.Definition, family)
		if runs {
			ids = append(ids, sfnRes.ID)
		}
		truncated = truncated || undecided && !runs
	}
	return unreadZero(res, reads.answer("sfn", "ecs-svc-related: DescribeStateMachine", ids, truncated))
}

// sfnASLRunsECSFamily walks an ASL definition for Task states whose Resource
// is the ecs:runTask integration and reports whether one runs a task
// definition of family, and whether one names its task definition by an
// expression this reading cannot evaluate. RunTask's TaskDefinition is
// "family", "family:revision" or the task definition's ARN; a JSONPath state
// passes it in Parameters ("TaskDefinition.$" for a path), a JSONata state in
// Arguments ("{% … %}" for an expression).
// https://docs.aws.amazon.com/step-functions/latest/dg/connect-ecs.html
// https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_RunTask.html
func sfnASLRunsECSFamily(definition, family string) (runs, undecided bool) {
	var raw any
	if err := json.Unmarshal([]byte(definition), &raw); err != nil {
		return false, true
	}
	var walk func(v any)
	walk = func(v any) {
		switch x := v.(type) {
		case []any:
			for _, item := range x {
				walk(item)
			}
		case map[string]any:
			if res, ok := x["Resource"].(string); ok {
				if a, ok := ARNForService(res, "states"); ok && strings.HasPrefix(a.Resource, "ecs:runTask") {
					for _, key := range []string{"Parameters", "Arguments"} {
						params, _ := x[key].(map[string]any)
						td, literal := params["TaskDefinition"].(string)
						_, path := params["TaskDefinition.$"]
						switch {
						case path || literal && strings.HasPrefix(strings.TrimSpace(td), "{%"):
							undecided = true
						case literal:
							runs = runs || taskDefFamily(td) == family
						}
					}
				}
			}
			for _, val := range x {
				walk(val)
			}
		}
	}
	walk(raw)
	return runs, undecided
}
