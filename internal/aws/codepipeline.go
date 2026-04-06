package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/codepipeline"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("pipeline", []string{"name", "pipeline_type", "version", "created", "updated"})

	resource.RegisterPaginated("pipeline", func(ctx context.Context, clients interface{}, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}
		return FetchCodePipelinesPage(ctx, c.CodePipeline, continuationToken)
	})

	resource.RegisterRelated("pipeline", []resource.RelatedDef{
		{TargetType: "cb", DisplayName: "CodeBuild Projects", Checker: nil, NeedsTargetCache: false},
		{TargetType: "role", DisplayName: "IAM Roles", Checker: nil, NeedsTargetCache: false},
	})
}

// FetchCodePipelines calls the CodePipeline ListPipelines API and converts
// the response into a slice of generic Resource structs.
func FetchCodePipelines(ctx context.Context, api CodePipelineListPipelinesAPI) ([]resource.Resource, error) {
	var all []resource.Resource
	token := ""
	for {
		result, err := FetchCodePipelinesPage(ctx, api, token)
		if err != nil {
			return nil, err
		}
		all = append(all, result.Resources...)
		if result.Pagination == nil || !result.Pagination.IsTruncated {
			break
		}
		token = result.Pagination.NextToken
	}
	return all, nil
}

// FetchCodePipelinesPage fetches a single page of CodePipeline pipelines.
func FetchCodePipelinesPage(ctx context.Context, api CodePipelineListPipelinesAPI, continuationToken string) (resource.FetchResult, error) {
	input := &codepipeline.ListPipelinesInput{}
	if continuationToken != "" {
		input.NextToken = &continuationToken
	}

	output, err := api.ListPipelines(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching CodePipeline pipelines: %w", err)
	}

	var resources []resource.Resource

	for _, pl := range output.Pipelines {
		name := ""
		if pl.Name != nil {
			name = *pl.Name
		}

		pipelineType := string(pl.PipelineType)

		created := ""
		if pl.Created != nil {
			created = pl.Created.Format("2006-01-02 15:04")
		}

		updated := ""
		if pl.Updated != nil {
			updated = pl.Updated.Format("2006-01-02 15:04")
		}

		version := ""
		if pl.Version != nil {
			version = fmt.Sprintf("%d", *pl.Version)
		}

		r := resource.Resource{
			ID:     name,
			Name:   name,
			Status: "",
			Fields: map[string]string{
				"name":          name,
				"pipeline_type": pipelineType,
				"created":       created,
				"updated":       updated,
				"version":       version,
			},
			RawStruct: pl,
		}

		resources = append(resources, r)
	}

	nextToken := ""
	isTruncated := false
	if output.NextToken != nil {
		nextToken = *output.NextToken
		isTruncated = true
	}

	totalHint := len(resources)
	if isTruncated {
		totalHint = -1
	}

	return resource.FetchResult{
		Resources: resources,
		Pagination: &resource.PaginationMeta{
			IsTruncated: isTruncated,
			NextToken:   nextToken,
			PageSize:    len(resources),
			TotalHint:   totalHint,
		},
	}, nil
}
