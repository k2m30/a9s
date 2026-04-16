package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/codeartifact"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("codeartifact", []string{"repo_name", "domain_name", "description", "domain_owner"})

	resource.RegisterPaginated("codeartifact", func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}
		return FetchCodeArtifactReposPage(ctx, c.CodeArtifact, continuationToken)
	})

	resource.RegisterRelated("codeartifact", []resource.RelatedDef{
		{TargetType: "cb", DisplayName: "CodeBuild Projects", Checker: checkCodeArtifactCB},
		{TargetType: "role", DisplayName: "IAM Role", Checker: checkCodeArtifactRole},
		{TargetType: "kms", DisplayName: "KMS Key", Checker: checkCodeArtifactKMS},
		{TargetType: "acm", DisplayName: "ACM Certificates", Checker: checkCodeArtifactACM},
		{TargetType: "kinesis", DisplayName: "Kinesis Streams", Checker: checkCodeArtifactKinesis},
		{TargetType: "lambda", DisplayName: "Lambda Functions", Checker: checkCodeArtifactLambda},
		{TargetType: "logs", DisplayName: "Log Groups", Checker: checkCodeArtifactLogs},
		{TargetType: "r53", DisplayName: "Route 53 Records", Checker: checkCodeArtifactR53},
		{TargetType: "waf", DisplayName: "WAF Web ACLs", Checker: checkCodeArtifactWAF},
	})
}

// FetchCodeArtifactRepos calls the CodeArtifact ListRepositories API and converts the
// response into a slice of generic Resource structs.
func FetchCodeArtifactRepos(ctx context.Context, api CodeArtifactListRepositoriesAPI) ([]resource.Resource, error) {
	var all []resource.Resource
	token := ""
	for {
		result, err := FetchCodeArtifactReposPage(ctx, api, token)
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

// FetchCodeArtifactReposPage fetches a single page of CodeArtifact repositories.
func FetchCodeArtifactReposPage(ctx context.Context, api CodeArtifactListRepositoriesAPI, continuationToken string) (resource.FetchResult, error) {
	input := &codeartifact.ListRepositoriesInput{
		MaxResults: aws.Int32(DefaultPageSize),
	}
	if continuationToken != "" {
		input.NextToken = &continuationToken
	}

	output, err := api.ListRepositories(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching CodeArtifact repositories: %w", err)
	}

	var resources []resource.Resource

	for _, repo := range output.Repositories {
		repoName := ""
		if repo.Name != nil {
			repoName = *repo.Name
		}

		domainName := ""
		if repo.DomainName != nil {
			domainName = *repo.DomainName
		}

		domainOwner := ""
		if repo.DomainOwner != nil {
			domainOwner = *repo.DomainOwner
		}

		arn := ""
		if repo.Arn != nil {
			arn = *repo.Arn
		}

		description := ""
		if repo.Description != nil {
			description = *repo.Description
		}

		adminAccount := ""
		if repo.AdministratorAccount != nil {
			adminAccount = *repo.AdministratorAccount
		}

		createdTime := ""
		if repo.CreatedTime != nil {
			createdTime = repo.CreatedTime.Format("2006-01-02 15:04")
		}

		r := resource.Resource{
			ID:     repoName,
			Name:   repoName,
			Status: "",
			Fields: map[string]string{
				"repo_name":     repoName,
				"domain_name":   domainName,
				"domain_owner":  domainOwner,
				"arn":           arn,
				"description":   description,
				"admin_account": adminAccount,
				"created_time":  createdTime,
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
