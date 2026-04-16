package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecr"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("ecr", []string{"repository_name", "uri", "tag_mutability", "scan_on_push", "created_at"})

	resource.RegisterPaginated("ecr", func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}
		return FetchECRRepositoriesPage(ctx, c.ECR, continuationToken)
	})

	resource.RegisterRelated("ecr", []resource.RelatedDef{
		{TargetType: "lambda", DisplayName: "Lambda Functions", Checker: checkECRLambda, NeedsTargetCache: true},
		{TargetType: "cb", DisplayName: "CodeBuild Projects", Checker: checkECRCodeBuild, NeedsTargetCache: true},
		{TargetType: "cfn", DisplayName: "CloudFormation Stacks", Checker: checkECRCFN, NeedsTargetCache: true},
		{TargetType: "kms", DisplayName: "KMS Key", Checker: checkECRKMS},
	})

	resource.RegisterNavigableFields("ecr", []resource.NavigableField{
		{FieldPath: "EncryptionConfiguration.KmsKey", TargetType: "kms"},
	})
}

// FetchECRRepositories calls the ECR DescribeRepositories API and converts
// the response into a slice of generic Resource structs.
func FetchECRRepositories(ctx context.Context, api ECRDescribeRepositoriesAPI) ([]resource.Resource, error) {
	var all []resource.Resource
	token := ""
	for {
		result, err := FetchECRRepositoriesPage(ctx, api, token)
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

// FetchECRRepositoriesPage fetches a single page of ECR repositories.
func FetchECRRepositoriesPage(ctx context.Context, api ECRDescribeRepositoriesAPI, continuationToken string) (resource.FetchResult, error) {
	input := &ecr.DescribeRepositoriesInput{
		MaxResults: aws.Int32(DefaultPageSize),
	}
	if continuationToken != "" {
		input.NextToken = &continuationToken
	}

	output, err := api.DescribeRepositories(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching ECR repositories: %w", err)
	}

	var resources []resource.Resource

	for _, repo := range output.Repositories {
		repoName := ""
		if repo.RepositoryName != nil {
			repoName = *repo.RepositoryName
		}

		uri := ""
		if repo.RepositoryUri != nil {
			uri = *repo.RepositoryUri
		}

		tagMutability := string(repo.ImageTagMutability)

		scanOnPush := "false"
		if repo.ImageScanningConfiguration != nil && repo.ImageScanningConfiguration.ScanOnPush {
			scanOnPush = "true"
		}

		createdAt := ""
		if repo.CreatedAt != nil {
			createdAt = repo.CreatedAt.Format("2006-01-02 15:04")
		}

		r := resource.Resource{
			ID:     repoName,
			Name:   repoName,
			Status: "",
			Fields: map[string]string{
				"repository_name": repoName,
				"uri":             uri,
				"tag_mutability":  tagMutability,
				"scan_on_push":    scanOnPush,
				"created_at":      createdAt,
			},
			RawStruct: repo,
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
