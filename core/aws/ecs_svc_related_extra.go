// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// ecs_svc_related_extra.go contains ECS service related-resource checkers
// spilled out of ecs_svc_related.go — includes eb-rule/ecr/secrets/sfn and
// any other overflow targets required by docs/related-resources.md.
package aws

import (
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
		return resource.ProvenZero("ecs-task", "svcName")
	}
	cluster := res.Fields["cluster"]
	taskList, truncated, err := relatedResourcesFor(ctx, clients, cache, "ecs-task")
	if err != nil {
		return resource.ErrorRelated("ecs-task", err)
	}
	if taskList == nil {
		return resource.UnknownRelated("ecs-task")
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
		return resource.UnknownRelated("subnet")
	}
	if raw.NetworkConfiguration == nil || raw.NetworkConfiguration.AwsvpcConfiguration == nil {
		return resource.ProvenZero("subnet", "raw.NetworkConfiguration.AwsvpcConfiguration")
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
		if res.RawStruct == nil {
			return resource.UnknownRelated("vpc")
		}
		return resource.KnownRelated("vpc", nil, false)
	}
	if raw.NetworkConfiguration == nil || raw.NetworkConfiguration.AwsvpcConfiguration == nil {
		return resource.ProvenZero("vpc", "raw.NetworkConfiguration.AwsvpcConfiguration")
	}
	subnetIDs := raw.NetworkConfiguration.AwsvpcConfiguration.Subnets
	if len(subnetIDs) == 0 {
		return resource.ProvenZero("vpc", "subnetIDs")
	}
	subnetList, truncated, err := relatedResourcesFor(ctx, clients, cache, "subnet")
	if err != nil {
		return resource.ErrorRelated("vpc", err)
	}
	if subnetList == nil {
		return resource.UnknownRelated("vpc")
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

// checkECSSvcEbRule is a reverse-scan checker for the ecs-svc→eb-rule relationship.
// Pattern C+reverse: iterate cache["eb-rule"]; for each rule whose EventPattern
// has source ["aws.ecs"] and detail.clusterArn / detail.group matching this service,
// add the rule name. NeedsTargetCache: true.
func checkECSSvcEbRule(_ context.Context, _ any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	svcName := ecsSvcName(res)
	if svcName == "" {
		return resource.ProvenZero("eb-rule", "svcName")
	}
	clusterName := res.Fields["cluster"]

	entry, ok := cache["eb-rule"]
	if !ok {
		return resource.UnknownRelated("eb-rule")
	}

	var ids []string
	for _, ruleRes := range entry.Resources {
		rule, ok := assertStruct[eventbridgetypes.Rule](ruleRes.RawStruct)
		if !ok {
			continue
		}
		if rule.EventPattern == nil || *rule.EventPattern == "" {
			continue
		}
		if ecsSvcEbRuleMatches(*rule.EventPattern, svcName, clusterName) {
			ids = append(ids, ruleRes.ID)
		}
	}
	return relatedResultTrunc("eb-rule", ids, entry.IsTruncated)
}

// ecsSvcEbRuleMatches returns true if the EventPattern JSON has source ["aws.ecs"]
// and references the service by name or group ("service:{svcName}") or cluster name.
func ecsSvcEbRuleMatches(pattern, svcName, clusterName string) bool {
	var p map[string]json.RawMessage
	if err := json.Unmarshal([]byte(pattern), &p); err != nil {
		return false
	}

	if src, ok := p["source"]; ok {
		var sources []string
		if err := json.Unmarshal(src, &sources); err != nil || !slices.Contains(sources, "aws.ecs") {
			return false
		}
	} else {
		return false
	}

	// Every filter the pattern names has to match: a rule that names both a
	// service group and a cluster fires for one cluster's service, and a
	// service name is the name of a service in a cluster.
	hasFilter := false
	if detail, ok := p["detail"]; ok {
		var d map[string]json.RawMessage
		if err := json.Unmarshal(detail, &d); err == nil {
			if grp, ok := d["group"]; ok {
				hasFilter = true
				var groups []string
				if err := json.Unmarshal(grp, &groups); err != nil {
					return false
				}
				if !slices.Contains(groups, "service:"+svcName) && !slices.Contains(groups, svcName) {
					return false
				}
			}
			if carn, ok := d["clusterArn"]; ok {
				hasFilter = true
				var carns []string
				if err := json.Unmarshal(carn, &carns); err != nil {
					return false
				}
				// A row that cannot say which cluster it is in is not held to
				// one, the way an alarm's qualifier leaves an unanswerable
				// question unanswered.
				named := clusterName == ""
				for _, c := range carns {
					named = named || lastSegment(c, "/") == clusterName
				}
				if !named {
					return false
				}
			}
		}
	}
	if hasFilter {
		return true
	}
	// Source matches aws.ecs with no narrowing filter — treat as broad match.
	return true
}

// checkECSSvcECR resolves ECR repositories used by this ECS service.
// Pattern A: calls ecs:DescribeTaskDefinition for the service's current task
// definition and reads the ECR repositories of ContainerDefinitions[].Image.
// NeedsTargetCache: false.
func checkECSSvcECR(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ecstypes.Service](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("ecr")
	}
	if raw.TaskDefinition == nil || *raw.TaskDefinition == "" {
		return resource.ProvenZero("ecr", "raw.TaskDefinition")
	}
	taskDefARN := *raw.TaskDefinition

	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.ECS == nil {
		return resource.UnknownRelated("ecr")
	}
	api, ok := c.ECS.(ECSDescribeTaskDefinitionAPI)
	if !ok {
		return resource.UnknownRelated("ecr")
	}

	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*ecs.DescribeTaskDefinitionOutput, error) {
		return api.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{
			TaskDefinition: &taskDefARN,
		})
	})
	// The related panel's error result carries the refusal to the pivot.
	// no finding: this arm already answers with it.
	if err != nil || out.TaskDefinition == nil {
		return resource.ErrorRelated("ecr", err)
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
		return resource.UnknownRelated("secrets")
	}
	if raw.TaskDefinition == nil || *raw.TaskDefinition == "" {
		return resource.ProvenZero("secrets", "raw.TaskDefinition")
	}
	taskDefARN := *raw.TaskDefinition

	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.ECS == nil {
		return resource.UnknownRelated("secrets")
	}
	api, ok := c.ECS.(ECSDescribeTaskDefinitionAPI)
	if !ok {
		return resource.UnknownRelated("secrets")
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
		return resource.ErrorRelated("secrets", err)
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

	return relatedRefs("secrets", refs, refContext(clients, cache, "secrets"))
}

// checkECSSvcSFN is a reverse-scan checker for the ecs-svc→sfn relationship.
// Pattern C+reverse: iterate cache["sfn"]; for each state machine call
// sfnDescribe and parse the ASL definition for states with
// Resource "arn:aws:states:::ecs:runTask*" whose Parameters.TaskDefinition
// matches the task definition family of this service.
// NeedsTargetCache: true.
func checkECSSvcSFN(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	if res.RawStruct == nil {
		return unreadZero(res, resource.KnownRelated("sfn", nil, false))
	}
	raw, ok := assertStruct[ecstypes.Service](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("sfn")
	}
	if raw.TaskDefinition == nil || *raw.TaskDefinition == "" {
		return unreadZero(res, resource.ProvenZero("sfn", "raw.TaskDefinition"))
	}

	family := taskDefFamily(*raw.TaskDefinition)
	if family == "" {
		return unreadZero(res, resource.ProvenZero("sfn", "family"))
	}

	entry, ok := cache["sfn"]
	if !ok {
		return resource.UnknownRelated("sfn")
	}

	var ids []string
	var failures []Failure
	for _, sfnRes := range entry.Resources {
		sfnARN := sfnRes.Fields["arn"]
		if sfnARN == "" {
			continue
		}
		sm, err := sfnDescribe(ctx, clients, sfnARN)
		if err != nil {
			failures = append(failures, FailedCall(sfnRes.ID, err))
			continue
		}
		if sm == nil || sm.Definition == nil || *sm.Definition == "" {
			continue
		}
		if sfnASLHasECSFamily(*sm.Definition, family) {
			ids = append(ids, sfnRes.ID)
		}
	}
	if len(ids) == 0 && !entry.IsTruncated {
		// Every state machine refused its read and the cache page was
		// complete: nothing was established about any of them, which is a
		// fetch failure rather than a lower bound over what was read.
		if aggErr := AggregateFailures("ecs-svc-related: DescribeStateMachine", failures, len(entry.Resources)); aggErr != nil &&
			len(failures) == len(entry.Resources) {
			return resource.ErrorRelated("sfn", aggErr)
		}
	}
	// Some DescribeStateMachine calls may have failed: ids is a proven subset,
	// not necessarily exhaustive. Truncated (not Errored) keeps the row
	// actionable rather than discarding confirmed matches as a dead end.
	return unreadZero(res, relatedResultTrunc("sfn", ids, entry.IsTruncated || len(failures) > 0))
}

// sfnASLHasECSFamily walks an ASL definition JSON and returns true if any Task state
// has Resource starting with "arn:aws:states:::ecs:runTask" and
// Parameters.TaskDefinition containing family.
func sfnASLHasECSFamily(definition, family string) bool {
	var raw any
	if err := json.Unmarshal([]byte(definition), &raw); err != nil {
		return false
	}
	found := false
	var walk func(v any)
	walk = func(v any) {
		if found {
			return
		}
		m, ok := v.(map[string]any)
		if !ok {
			if arr, ok := v.([]any); ok {
				for _, item := range arr {
					walk(item)
				}
			}
			return
		}
		if res, ok := m["Resource"].(string); ok {
			if a, ok := ARNForService(res, "states"); ok && strings.HasPrefix(a.Resource, "ecs:runTask") {
				if params, ok := m["Parameters"].(map[string]any); ok {
					if td, ok := params["TaskDefinition"].(string); ok {
						if strings.Contains(td, family) {
							found = true
							return
						}
					}
					if td, ok := params["TaskDefinition.$"].(string); ok {
						if strings.Contains(td, family) {
							found = true
							return
						}
					}
				}
			}
		}
		for _, val := range m {
			walk(val)
		}
	}
	walk(raw)
	return found
}
