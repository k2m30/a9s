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
// Iterate cache["pipeline"]; for each pipeline call
// codepipeline:GetPipeline and scan Stages[].Actions[] where
// ActionTypeId.Provider == "CodeBuild" AND Configuration["ProjectName"] == parent name.
func checkCbPipeline(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	project, ok := assertStruct[cbtypes.Project](res.RawStruct)
	if !ok {
		return NotRead("pipeline")
	}
	projectName := ""
	if project.Name != nil {
		projectName = *project.Name
	}
	if projectName == "" {
		projectName = res.ID
	}
	if projectName == "" {
		return keyMissing("pipeline", "projectName")
	}

	pipelineList, truncated, err := relatedResourcesFor(ctx, clients, cache, "pipeline")
	if err != nil {
		return ReadFailed("pipeline", err)
	}
	if pipelineList == nil {
		return NotRead("pipeline")
	}

	// If there are pipelines to check but no CodePipeline client to call GetPipeline,
	// we cannot determine the relationship — return -1 (unknown).
	if len(pipelineList) > 0 {
		c, cok := clients.(*ServiceClients)
		if !cok || c == nil || c.CodePipeline == nil {
			return NotRead("pipeline")
		}
	}

	var ids []string
	var reads rowReads
	for _, pipelineRes := range pipelineList {
		pipelineName := pipelineRes.ID
		if pipelineName == "" {
			reads.missed()
			continue
		}
		p, err := pipelineGetDeclaration(ctx, clients, pipelineName)
		if err != nil {
			reads.fail(pipelineName, err)
			continue
		}
		reads.read++
		if cbPipelineHasProject(p.Stages, projectName) {
			ids = append(ids, pipelineName)
		}
	}
	return reads.answer("pipeline", "cb-related: GetPipeline", ids, truncated)
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
