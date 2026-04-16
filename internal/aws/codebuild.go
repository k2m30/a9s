package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/codebuild"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("cb", []string{"name", "source_type", "description", "last_modified"})

	resource.RegisterPaginated("cb", func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}
		return FetchCodeBuildProjectsPage(ctx, c.CodeBuild, c.CodeBuild, continuationToken)
	})

	resource.RegisterRelated("cb", []resource.RelatedDef{
		{TargetType: "logs", DisplayName: "Log Groups", Checker: checkCbLogs, NeedsTargetCache: true},
		{TargetType: "role", DisplayName: "IAM Roles", Checker: checkCbRole, NeedsTargetCache: true},
		{TargetType: "pipeline", DisplayName: "CodePipelines", Checker: checkCbPipeline},
		{TargetType: "sg", DisplayName: "Security Groups", Checker: checkCbSG},
		{TargetType: "vpc", DisplayName: "VPC", Checker: checkCbVPC},
	})

	// cbtypes.Project: ServiceRole, EncryptionKey (KMS), VpcConfig.{VpcId,Subnets,SecurityGroupIds}
	resource.RegisterNavigableFields("cb", []resource.NavigableField{
		{FieldPath: "ServiceRole", TargetType: "role"},
		{FieldPath: "EncryptionKey", TargetType: "kms"},
		{FieldPath: "VpcConfig.VpcId", TargetType: "vpc"},
		{FieldPath: "VpcConfig.Subnets", TargetType: "subnet"},
		{FieldPath: "VpcConfig.SecurityGroupIds", TargetType: "sg"},
	})
}

// FetchCodeBuildProjectsPage fetches one page of project names from ListProjects
// using the continuationToken, then calls BatchGetProjects for that page's names.
// IsTruncated reflects whether ListProjects has more pages beyond this one.
func FetchCodeBuildProjectsPage(
	ctx context.Context,
	listAPI CodeBuildListProjectsAPI,
	batchAPI CodeBuildBatchGetProjectsAPI,
	continuationToken string,
) (resource.FetchResult, error) {
	input := &codebuild.ListProjectsInput{}
	if continuationToken != "" {
		input.NextToken = &continuationToken
	}

	listOutput, err := listAPI.ListProjects(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("listing CodeBuild projects: %w", err)
	}

	if len(listOutput.Projects) == 0 {
		nextToken := ""
		isTruncated := false
		if listOutput.NextToken != nil {
			nextToken = *listOutput.NextToken
			isTruncated = true
		}
		return resource.FetchResult{
			Resources: []resource.Resource{},
			Pagination: &resource.PaginationMeta{
				IsTruncated: isTruncated,
				NextToken:   nextToken,
				PageSize:    0,
				TotalHint:   -1,
			},
		}, nil
	}

	batchOutput, err := batchAPI.BatchGetProjects(ctx, &codebuild.BatchGetProjectsInput{
		Names: listOutput.Projects,
	})
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("batch getting CodeBuild projects: %w", err)
	}

	var resources []resource.Resource

	for _, project := range batchOutput.Projects {
		name := ""
		if project.Name != nil {
			name = *project.Name
		}

		description := ""
		if project.Description != nil {
			description = *project.Description
		}

		sourceType := ""
		if project.Source != nil {
			sourceType = string(project.Source.Type)
		}

		lastModified := ""
		if project.LastModified != nil {
			lastModified = project.LastModified.Format("2006-01-02 15:04")
		}

		r := resource.Resource{
			ID:     name,
			Name:   name,
			Status: "",
			Fields: map[string]string{
				"name":          name,
				"source_type":   sourceType,
				"description":   description,
				"last_modified": lastModified,
			},
			RawStruct: project,
		}

		resources = append(resources, r)
	}

	nextToken := ""
	isTruncated := false
	if listOutput.NextToken != nil {
		nextToken = *listOutput.NextToken
		isTruncated = true
	}

	return resource.FetchResult{
		Resources: resources,
		Pagination: &resource.PaginationMeta{
			IsTruncated: isTruncated,
			NextToken:   nextToken,
			PageSize:    len(resources),
			TotalHint:   -1,
		},
	}, nil
}

// FetchCodeBuildProjects performs a two-step fetch:
// 1. ListProjects to get project names (paginated via NextToken)
// 2. BatchGetProjects to get full project details
func FetchCodeBuildProjects(
	ctx context.Context,
	listAPI CodeBuildListProjectsAPI,
	batchAPI CodeBuildBatchGetProjectsAPI,
) ([]resource.Resource, error) {
	var allResources []resource.Resource
	continuationToken := ""

	for {
		result, err := FetchCodeBuildProjectsPage(ctx, listAPI, batchAPI, continuationToken)
		if err != nil {
			return nil, err
		}

		allResources = append(allResources, result.Resources...)

		if result.Pagination == nil || !result.Pagination.IsTruncated {
			break
		}
		continuationToken = result.Pagination.NextToken
	}

	return allResources, nil
}
