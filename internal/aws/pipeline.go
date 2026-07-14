package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/codepipeline"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// FetchCodePipelinesPage fetches a single page of CodePipeline pipelines.
// No client context is available here to resolve the real account, so
// Fields["arn"] is left empty. Use FetchCodePipelinesPageWithClients (the
// production catalog path) to construct the ARN from the session's real
// resolved region/account.
func FetchCodePipelinesPage(ctx context.Context, api CodePipelineListPipelinesAPI, continuationToken string) (resource.FetchResult, error) {
	return fetchCodePipelinesPage(ctx, api, GetDefaultRegion("", ""), "", continuationToken)
}

// FetchCodePipelinesPageWithClients fetches a single page of CodePipeline
// pipelines and constructs Fields["arn"] for each pipeline
// (arn:aws:codepipeline:<region>:<account>:<name>) using the session's
// resolved region/account. Account resolution is best-effort — on failure
// Fields["arn"] is left empty rather than constructed from a wrong account.
func FetchCodePipelinesPageWithClients(ctx context.Context, c *ServiceClients, continuationToken string) (resource.FetchResult, error) {
	account := accountIDFromClients(ctx, c, c.IdentityStore())
	region := c.Region
	if region == "" {
		region = GetDefaultRegion("", "")
	}
	return fetchCodePipelinesPage(ctx, c.CodePipeline, region, account, continuationToken)
}

// fetchCodePipelinesPage is the shared implementation. When region and
// account are both non-empty, Fields["arn"] is constructed as
// arn:aws:codepipeline:<region>:<account>:<name> — CodePipeline ARNs have no
// "pipeline/" resource-type segment, unlike most other services; otherwise
// it is "".
func fetchCodePipelinesPage(ctx context.Context, api CodePipelineListPipelinesAPI, region, account, continuationToken string) (resource.FetchResult, error) {
	input := &codepipeline.ListPipelinesInput{
		MaxResults: aws.Int32(DefaultPageSize),
	}
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

		arn := ""
		if region != "" && account != "" && name != "" {
			arn = "arn:aws:codepipeline:" + region + ":" + account + ":" + name
		}

		r := resource.Resource{
			ID:   name,
			Name: name,
			Fields: map[string]string{
				"name":          name,
				"pipeline_type": pipelineType,
				"created":       created,
				"updated":       updated,
				"version":       version,
				"arn":           arn,
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
