// ecs_services_related_batch4.go contains ECS service related-resource checkers
// for eb-rule, ecr, secrets, and sfn targets (batch 4 of the related-panel).
package aws

import (
	"context"
	"encoding/json"
	"slices"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	eventbridgetypes "github.com/aws/aws-sdk-go-v2/service/eventbridge/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// checkECSSvcEbRule is a reverse-scan checker for the ecs-svc→eb-rule relationship.
// Pattern C+reverse: iterate cache["eb-rule"]; for each rule whose EventPattern
// has source ["aws.ecs"] and detail.clusterArn / detail.group matching this service,
// add the rule name. NeedsTargetCache: true.
func checkECSSvcEbRule(_ context.Context, _ any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	svcName := res.ID
	if svcName == "" {
		return resource.RelatedCheckResult{TargetType: "eb-rule", Count: 0}
	}
	clusterName := res.Fields["cluster"]

	entry, ok := cache["eb-rule"]
	if !ok {
		return resource.RelatedCheckResult{TargetType: "eb-rule"}
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
	result := relatedResult("eb-rule", ids)
	result.Approximate = entry.IsTruncated
	return result
}

// ecsSvcEbRuleMatches returns true if the EventPattern JSON has source ["aws.ecs"]
// and references the service by name or group ("service:{svcName}") or cluster name.
func ecsSvcEbRuleMatches(pattern, svcName, clusterName string) bool {
	var p map[string]json.RawMessage
	if err := json.Unmarshal([]byte(pattern), &p); err != nil {
		return false
	}

	// Check source includes "aws.ecs"
	if src, ok := p["source"]; ok {
		var sources []string
		if err := json.Unmarshal(src, &sources); err != nil || !slices.Contains(sources, "aws.ecs") {
			return false
		}
	} else {
		return false
	}

	// Check detail for service/cluster name match.
	// If a filter key is present but doesn't match, return false.
	hasFilter := false
	if detail, ok := p["detail"]; ok {
		var d map[string]json.RawMessage
		if err := json.Unmarshal(detail, &d); err == nil {
			// Check group field ("service:{svcName}")
			if grp, ok := d["group"]; ok {
				hasFilter = true
				var groups []string
				if err := json.Unmarshal(grp, &groups); err == nil {
					for _, g := range groups {
						if g == "service:"+svcName || g == svcName {
							return true
						}
					}
				}
			}
			// Check clusterArn field
			if carn, ok := d["clusterArn"]; ok {
				hasFilter = true
				var carns []string
				if err := json.Unmarshal(carn, &carns); err == nil {
					for _, c := range carns {
						if clusterName != "" && strings.Contains(c, clusterName) {
							return true
						}
					}
				}
			}
		}
	}
	if hasFilter {
		// A filter existed but didn't match — not related.
		return false
	}
	// Source matches aws.ecs with no narrowing filter — treat as broad match.
	return true
}

// checkECSSvcECR resolves ECR repositories used by this ECS service.
// Pattern A: calls ecs:DescribeTaskDefinition for the service's current task
// definition and extracts ECR repository names from ContainerDefinitions[].Image.
// NeedsTargetCache: false.
func checkECSSvcECR(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ecstypes.Service](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "ecr", Count: -1}
	}
	if raw.TaskDefinition == nil || *raw.TaskDefinition == "" {
		return resource.RelatedCheckResult{TargetType: "ecr", Count: 0}
	}
	taskDefARN := *raw.TaskDefinition

	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.ECS == nil {
		return resource.RelatedCheckResult{TargetType: "ecr", Count: -1}
	}
	api, ok := c.ECS.(ECSDescribeTaskDefinitionAPI)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "ecr", Count: -1}
	}

	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*ecs.DescribeTaskDefinitionOutput, error) {
		return api.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{
			TaskDefinition: &taskDefARN,
		})
	})
	if err != nil || out.TaskDefinition == nil {
		return resource.RelatedCheckResult{TargetType: "ecr", Count: -1, Err: err}
	}

	seen := make(map[string]struct{})
	for _, c := range out.TaskDefinition.ContainerDefinitions {
		if c.Image == nil || *c.Image == "" {
			continue
		}
		img := *c.Image
		// ECR image URI: {account}.dkr.ecr.{region}.amazonaws.com/{repo}[:{tag}|@{digest}]
		if !strings.Contains(img, ".dkr.ecr.") {
			continue
		}
		_, repo, hasSep := strings.Cut(img, "/")
		if !hasSep {
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

// checkECSSvcSecrets resolves Secrets Manager secrets referenced by this ECS service.
// Pattern A: calls ecs:DescribeTaskDefinition and inspects
// ContainerDefinitions[].Secrets[].ValueFrom for secretsmanager ARNs, plus
// ContainerDefinitions[].RepositoryCredentials.CredentialsParameter.
// NeedsTargetCache: false.
func checkECSSvcSecrets(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ecstypes.Service](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "secrets", Count: -1}
	}
	if raw.TaskDefinition == nil || *raw.TaskDefinition == "" {
		return resource.RelatedCheckResult{TargetType: "secrets", Count: 0}
	}
	taskDefARN := *raw.TaskDefinition

	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.ECS == nil {
		return resource.RelatedCheckResult{TargetType: "secrets", Count: -1}
	}
	api, ok := c.ECS.(ECSDescribeTaskDefinitionAPI)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "secrets", Count: -1}
	}

	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*ecs.DescribeTaskDefinitionOutput, error) {
		return api.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{
			TaskDefinition: &taskDefARN,
		})
	})
	if err != nil || out.TaskDefinition == nil {
		return resource.RelatedCheckResult{TargetType: "secrets", Count: -1, Err: err}
	}

	seen := make(map[string]struct{})
	for _, cd := range out.TaskDefinition.ContainerDefinitions {
		// Secrets[].ValueFrom — secretsmanager ARNs
		for _, s := range cd.Secrets {
			if s.ValueFrom == nil || *s.ValueFrom == "" {
				continue
			}
			v := *s.ValueFrom
			if strings.HasPrefix(v, "arn:aws:secretsmanager:") {
				seen[v] = struct{}{}
			}
		}
		// RepositoryCredentials.CredentialsParameter — may be a Secrets Manager ARN
		if cd.RepositoryCredentials != nil && cd.RepositoryCredentials.CredentialsParameter != nil {
			cp := *cd.RepositoryCredentials.CredentialsParameter
			if strings.HasPrefix(cp, "arn:aws:secretsmanager:") {
				seen[cp] = struct{}{}
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

// checkECSSvcSFN is a reverse-scan checker for the ecs-svc→sfn relationship.
// Pattern C+reverse: iterate cache["sfn"]; for each state machine call
// sfnDescribe and parse the ASL definition for states with
// Resource "arn:aws:states:::ecs:runTask*" whose Parameters.TaskDefinition
// matches the task definition family of this service.
// NeedsTargetCache: true.
func checkECSSvcSFN(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ecstypes.Service](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "sfn", Count: -1}
	}
	if raw.TaskDefinition == nil || *raw.TaskDefinition == "" {
		return resource.RelatedCheckResult{TargetType: "sfn", Count: 0}
	}

	// Extract task def family from ARN: arn:aws:ecs:region:account:task-definition/family:revision
	taskDefARN := *raw.TaskDefinition
	taskDefFamily := arnLastSegment(taskDefARN)
	if idx := strings.LastIndex(taskDefFamily, ":"); idx >= 0 {
		taskDefFamily = taskDefFamily[:idx]
	}
	if taskDefFamily == "" {
		return resource.RelatedCheckResult{TargetType: "sfn", Count: 0}
	}

	entry, ok := cache["sfn"]
	if !ok {
		return resource.RelatedCheckResult{TargetType: "sfn"}
	}

	var ids []string
	for _, sfnRes := range entry.Resources {
		sfnARN := sfnRes.Fields["arn"]
		if sfnARN == "" {
			continue
		}
		sm := sfnDescribe(ctx, clients, sfnARN)
		if sm == nil || sm.Definition == nil || *sm.Definition == "" {
			continue
		}
		if sfnASLHasECSFamily(*sm.Definition, taskDefFamily) {
			ids = append(ids, sfnRes.ID)
		}
	}
	result := relatedResult("sfn", ids)
	result.Approximate = entry.IsTruncated
	return result
}

// sfnASLHasECSFamily walks an ASL definition JSON and returns true if any Task state
// has Resource starting with "arn:aws:states:::ecs:runTask" and
// Parameters.TaskDefinition containing taskDefFamily.
func sfnASLHasECSFamily(definition, taskDefFamily string) bool {
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
		// Check if this node is an ECS runTask state
		if res, ok := m["Resource"].(string); ok {
			if strings.HasPrefix(res, "arn:aws:states:::ecs:runTask") {
				// Check Parameters.TaskDefinition
				if params, ok := m["Parameters"].(map[string]any); ok {
					if td, ok := params["TaskDefinition"].(string); ok {
						if strings.Contains(td, taskDefFamily) {
							found = true
							return
						}
					}
					// Also check "TaskDefinition.$" (reference)
					if td, ok := params["TaskDefinition.$"].(string); ok {
						if strings.Contains(td, taskDefFamily) {
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
