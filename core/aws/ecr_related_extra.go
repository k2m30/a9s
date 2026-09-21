// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// ecr_related_extra.go — additional ECR related-resource checkers.
package aws

import (
	"cmp"
	"context"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	cptypes "github.com/aws/aws-sdk-go-v2/service/codepipeline/types"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	ecrtypes "github.com/aws/aws-sdk-go-v2/service/ecr/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

func checkECRECSTask(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	repoURI := ecrRepoURI(res)
	if repoURI == "" {
		return resource.ProvenZero("ecs-task", "repoURI")
	}
	taskList, truncated, err := relatedResourcesFor(ctx, clients, cache, "ecs-task")
	if err != nil {
		return resource.ErrorRelated("ecs-task", err)
	}
	if taskList == nil {
		return resource.UnknownRelated("ecs-task")
	}
	var ids []string
	for _, tRes := range taskList {
		// Fields["container_images"] is a comma-joined list of this task's
		// Containers[].Image values (populated directly from DescribeTasks —
		// no task-definition join required).
		for image := range strings.SplitSeq(tRes.Fields["container_images"], ",") {
			if imageRefersToRepo(image, repoURI) {
				ids = append(ids, tRes.ID)
				break
			}
		}
	}
	return relatedResultTrunc("ecs-task", ids, truncated)
}

// checkECRPipeline is a reverse-scan checker for the ecr→pipeline relationship.
// Pattern C+reverse: iterate cache["pipeline"]; for each pipeline call
// codepipeline:GetPipeline and scan actions where Provider == "ECR" AND
// Configuration["RepositoryName"] == parent repository name.
// NeedsTargetCache: true.
func checkECRPipeline(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	repo, ok := assertStruct[ecrtypes.Repository](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("pipeline")
	}
	repoName := ""
	if repo.RepositoryName != nil {
		repoName = *repo.RepositoryName
	}
	if repoName == "" {
		return resource.ProvenZero("pipeline", "repoName")
	}

	entry, ok := cache["pipeline"]
	if !ok {
		return resource.UnknownRelated("pipeline")
	}

	// If there are pipelines to check but no CodePipeline client to call
	// GetPipeline, we cannot determine the relationship at all.
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
		if ecrPipelineHasRepo(p.Stages, repoName) {
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

// checkECRRole resolves the IAM roles the ECR repository's resource-based
// policy grants. Pattern F+forward: calls ecr:GetRepositoryPolicy.
func checkECRRole(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	repo, ok := assertStruct[ecrtypes.Repository](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("role")
	}
	repoName := ""
	if repo.RepositoryName != nil {
		repoName = *repo.RepositoryName
	}
	if repoName == "" {
		return resource.ProvenZero("role", "repoName")
	}

	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.ECR == nil {
		return resource.UnknownRelated("role")
	}
	api, ok := c.ECR.(ECRGetRepositoryPolicyAPI)
	if !ok {
		return resource.UnknownRelated("role")
	}

	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*ecr.GetRepositoryPolicyOutput, error) {
		return api.GetRepositoryPolicy(ctx, &ecr.GetRepositoryPolicyInput{
			RepositoryName: &repoName,
		})
	})
	if err != nil {
		// RepositoryPolicyNotFoundException means no policy exists → 0
		if ErrCodeIs(err, "RepositoryPolicyNotFoundException") {
			return resource.ProvenZero("role", "the API answered that none is configured")
		}
		return resource.ErrorRelated("role", err)
	}
	if out.PolicyText == nil || *out.PolicyText == "" {
		return resource.ProvenZero("role", "out.PolicyText")
	}

	// repo.RegistryId is the owning account, the one whose roles are local.
	rc := refContext(clients, cache, "role")
	rc.AccountID = cmp.Or(aws.ToString(repo.RegistryId), rc.AccountID)
	refs, ok := grantedPrincipalRefs(*out.PolicyText, "role/")
	if !ok {
		return resource.UnknownRelated("role")
	}
	return relatedRefs("role", refs, rc)
}
