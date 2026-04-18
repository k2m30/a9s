// codebuild_related.go contains CodeBuild project related-resource checker functions
// that supplement cb_related.go. Kept separate to avoid exceeding 400 LOC in the
// primary file.
package aws

import (
	"context"

	cbtypes "github.com/aws/aws-sdk-go-v2/service/codebuild/types"
	cptypes "github.com/aws/aws-sdk-go-v2/service/codepipeline/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// checkCbPipeline is a reverse-scan checker for the codebuild→pipeline relationship.
// Pattern C+reverse: iterate cache["pipeline"]; for each pipeline call
// codepipeline:GetPipeline and scan Stages[].Actions[] where
// ActionTypeId.Provider == "CodeBuild" AND Configuration["ProjectName"] == parent name.
// NeedsTargetCache: true.
func checkCbPipeline(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	project, ok := assertStruct[cbtypes.Project](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "pipeline", Count: -1}
	}
	projectName := ""
	if project.Name != nil {
		projectName = *project.Name
	}
	if projectName == "" {
		projectName = res.ID
	}
	if projectName == "" {
		return resource.RelatedCheckResult{TargetType: "pipeline", Count: 0}
	}

	entry, ok := cache["pipeline"]
	if !ok {
		// cache not yet populated — honest 0, not -1
		return resource.RelatedCheckResult{TargetType: "pipeline"}
	}

	// If there are pipelines to check but no CodePipeline client to call GetPipeline,
	// we cannot determine the relationship — return -1 (unknown).
	if len(entry.Resources) > 0 {
		c, cok := clients.(*ServiceClients)
		if !cok || c == nil || c.CodePipeline == nil {
			return resource.RelatedCheckResult{TargetType: "pipeline", Count: -1}
		}
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
		if cbPipelineHasProject(p.Stages, projectName) {
			ids = append(ids, pipelineName)
		}
	}
	result := relatedResult("pipeline", ids)
	result.Approximate = entry.IsTruncated
	return result
}

// cbPipelineHasProject returns true if any action across the given stages has
// Provider == "CodeBuild" and Configuration["ProjectName"] == projectName.
func cbPipelineHasProject(stages []cptypes.StageDeclaration, projectName string) bool {
	for _, stg := range stages {
		for _, a := range stg.Actions {
			if actionProvider(a) == "CodeBuild" && a.Configuration["ProjectName"] == projectName {
				return true
			}
		}
	}
	return false
}
