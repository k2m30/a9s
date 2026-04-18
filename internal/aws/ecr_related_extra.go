// ecr_related_extra.go — additional ECR related-resource checkers.
package aws

import (
	"context"
	"encoding/json"
	"strings"

	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"
	cptypes "github.com/aws/aws-sdk-go-v2/service/codepipeline/types"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	ecrtypes "github.com/aws/aws-sdk-go-v2/service/ecr/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func checkECRCTEvents(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	repoName := res.ID
	if repoName == "" {
		return resource.RelatedCheckResult{TargetType: "ct-events", Count: 0}
	}
	evList, truncated, err := ecrRelatedResources(ctx, clients, cache, "ct-events")
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
			if r.ResourceName != nil && strings.Contains(*r.ResourceName, repoName) {
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

func checkECRECSTask(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	repoName := res.ID
	if repoName == "" {
		return resource.RelatedCheckResult{TargetType: "ecs-task", Count: 0}
	}
	taskList, truncated, err := ecrRelatedResources(ctx, clients, cache, "ecs-task")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "ecs-task", Count: -1, Err: err}
	}
	if taskList == nil {
		return resource.RelatedCheckResult{TargetType: "ecs-task", Count: -1}
	}
	var ids []string
	for _, tRes := range taskList {
		// The task's Containers[].Image is only populated in the task struct
		// for running tasks, not in DescribeTasks responses for all tasks.
		// A weak substring match on any field that looks like an image URI.
		for _, v := range tRes.Fields {
			if strings.Contains(v, ".dkr.ecr.") && strings.Contains(v, "/"+repoName) {
				ids = append(ids, tRes.ID)
				break
			}
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "ecs-task", Count: -1}
	}
	result := relatedResult("ecs-task", ids)
	result.Approximate = truncated
	return result
}

// checkECRPipeline is a reverse-scan checker for the ecr→pipeline relationship.
// Pattern C+reverse: iterate cache["pipeline"]; for each pipeline call
// codepipeline:GetPipeline and scan actions where Provider == "ECR" AND
// Configuration["RepositoryName"] == parent repository name.
// NeedsTargetCache: true.
func checkECRPipeline(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	repo, ok := assertStruct[ecrtypes.Repository](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "pipeline", Count: -1}
	}
	repoName := ""
	if repo.RepositoryName != nil {
		repoName = *repo.RepositoryName
	}
	if repoName == "" {
		return resource.RelatedCheckResult{TargetType: "pipeline", Count: 0}
	}

	entry, ok := cache["pipeline"]
	if !ok {
		return resource.RelatedCheckResult{TargetType: "pipeline"}
	}

	var ids []string
	for _, pipelineRes := range entry.Resources {
		pipelineName := pipelineRes.ID
		if pipelineName == "" {
			continue
		}
		p := pipelineGetDeclaration(ctx, clients, pipelineName)
		if p == nil {
			continue
		}
		if ecrPipelineHasRepo(p.Stages, repoName) {
			ids = append(ids, pipelineName)
		}
	}
	result := relatedResult("pipeline", ids)
	result.Approximate = entry.IsTruncated
	return result
}

// ecrPipelineHasRepo returns true if any action in the given stages has
// Provider == "ECR" AND Configuration["RepositoryName"] == repoName.
func ecrPipelineHasRepo(stages []cptypes.StageDeclaration, repoName string) bool {
	for _, stg := range stages {
		for _, a := range stg.Actions {
			if actionProvider(a) == "ECR" && a.Configuration["RepositoryName"] == repoName {
				return true
			}
		}
	}
	return false
}

// checkECRRole resolves IAM roles from the ECR repository's resource-based policy.
// Pattern F+forward: calls ecr:GetRepositoryPolicy and parses Statement[].Principal.AWS
// for role ARNs matching arn:aws:iam::*:role/*.
func checkECRRole(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	repo, ok := assertStruct[ecrtypes.Repository](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "role", Count: -1}
	}
	repoName := ""
	if repo.RepositoryName != nil {
		repoName = *repo.RepositoryName
	}
	if repoName == "" {
		return resource.RelatedCheckResult{TargetType: "role", Count: 0}
	}

	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.ECR == nil {
		return resource.RelatedCheckResult{TargetType: "role", Count: -1}
	}
	api, ok := c.ECR.(ECRGetRepositoryPolicyAPI)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "role", Count: -1}
	}

	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*ecr.GetRepositoryPolicyOutput, error) {
		return api.GetRepositoryPolicy(ctx, &ecr.GetRepositoryPolicyInput{
			RepositoryName: &repoName,
		})
	})
	if err != nil {
		// RepositoryPolicyNotFoundException means no policy exists → 0
		if strings.Contains(err.Error(), "RepositoryPolicyNotFoundException") {
			return resource.RelatedCheckResult{TargetType: "role", Count: 0}
		}
		return resource.RelatedCheckResult{TargetType: "role", Count: -1, Err: err}
	}
	if out.PolicyText == nil || *out.PolicyText == "" {
		return resource.RelatedCheckResult{TargetType: "role", Count: 0}
	}

	roleARNs := ecrPolicyRoleARNs(*out.PolicyText)
	return relatedResult("role", roleARNs)
}

// ecrPolicyRoleARNs parses an IAM policy JSON document and returns all IAM role
// ARNs found in Statement[].Principal.AWS. Both string and []string Principal.AWS
// values are handled.
func ecrPolicyRoleARNs(policyText string) []string {
	var policy struct {
		Statement []struct {
			Principal json.RawMessage `json:"Principal"`
		} `json:"Statement"`
	}
	if err := json.Unmarshal([]byte(policyText), &policy); err != nil {
		return nil
	}

	seen := map[string]struct{}{}
	for _, stmt := range policy.Statement {
		if stmt.Principal == nil {
			continue
		}
		// Try as object {"AWS": ...}
		var principalObj map[string]json.RawMessage
		if err := json.Unmarshal(stmt.Principal, &principalObj); err == nil {
			if awsRaw, ok := principalObj["AWS"]; ok {
				addRoleARNs(awsRaw, seen)
			}
		}
	}
	ids := make([]string, 0, len(seen))
	for arn := range seen {
		ids = append(ids, arn)
	}
	return ids
}

// addRoleARNs extracts role ARNs from a JSON value that is either a string or
// []string and adds any matching arn:aws:iam::*:role/* entries to seen.
func addRoleARNs(raw json.RawMessage, seen map[string]struct{}) {
	// Try single string
	var single string
	if err := json.Unmarshal(raw, &single); err == nil {
		if isRoleARN(single) {
			seen[single] = struct{}{}
		}
		return
	}
	// Try array of strings
	var multi []string
	if err := json.Unmarshal(raw, &multi); err == nil {
		for _, s := range multi {
			if isRoleARN(s) {
				seen[s] = struct{}{}
			}
		}
	}
}

// isRoleARN returns true if s is an IAM role ARN (arn:aws:iam::*:role/*).
func isRoleARN(s string) bool {
	return strings.HasPrefix(s, "arn:") && strings.Contains(s, ":role/")
}

// ecrRelatedResources returns the resource list for target from cache or fetches
// the first page via the registered paginated fetcher.
func ecrRelatedResources(ctx context.Context, clients any, cache resource.ResourceCache, target string) ([]resource.Resource, bool, error) {
	resources, isTruncated, err := FetchRelatedTarget(ctx, clients, cache, target)
	if err != nil {
		if _, ok := clients.(*ServiceClients); !ok {
			return nil, false, nil
		}
	}
	return resources, isTruncated, err
}

