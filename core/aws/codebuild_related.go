// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// codebuild_related.go contains CodeBuild project related-resource checker functions
// that supplement cb_related.go. Kept separate to avoid exceeding 400 LOC in the
// primary file.
package aws

import (
	"context"

	cbtypes "github.com/aws/aws-sdk-go-v2/service/codebuild/types"
	cptypes "github.com/aws/aws-sdk-go-v2/service/codepipeline/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// checkCbPipeline is a reverse-scan checker for the codebuild→pipeline relationship.
// Pattern C+reverse: iterate cache["pipeline"]; for each pipeline call
// codepipeline:GetPipeline and scan Stages[].Actions[] where
// ActionTypeId.Provider == "CodeBuild" AND Configuration["ProjectName"] == parent name.
// NeedsTargetCache: true.
func checkCbPipeline(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	project, ok := assertStruct[cbtypes.Project](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("pipeline")
	}
	projectName := ""
	if project.Name != nil {
		projectName = *project.Name
	}
	if projectName == "" {
		projectName = res.ID
	}
	if projectName == "" {
		return resource.KnownRelated("pipeline", nil, false)
	}

	entry, ok := cache["pipeline"]
	if !ok {
		// cache not yet populated — unknown, not a definitive 0
		return resource.UnknownRelated("pipeline")
	}

	// If there are pipelines to check but no CodePipeline client to call GetPipeline,
	// we cannot determine the relationship — return -1 (unknown).
	if len(entry.Resources) > 0 {
		c, cok := clients.(*ServiceClients)
		if !cok || c == nil || c.CodePipeline == nil {
			return resource.UnknownRelated("pipeline")
		}
	}

	var ids []string
	attempted, failed := 0, 0
	for _, pipelineRes := range entry.Resources {
		pipelineName := pipelineRes.ID
		if pipelineName == "" {
			continue
		}
		attempted++
		p, err := pipelineGetDeclaration(ctx, clients, pipelineName)
		if err != nil {
			failed++
			continue
		}
		if cbPipelineHasProject(p.Stages, projectName) {
			ids = append(ids, pipelineName)
		}
	}
	// Every lookup in the loop failed (throttled, denied, deleted mid-scan):
	// nothing was actually resolved, so this is not a proven zero.
	if attempted > 0 && failed == attempted {
		return resource.UnknownRelated("pipeline")
	}
	return relatedResultTrunc("pipeline", ids, entry.IsTruncated || failed > 0)
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
