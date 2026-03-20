package aws

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/codeartifact"

	"github.com/k2m30/a9s/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("codeartifact", []string{"repo_name", "domain_name", "description", "domain_owner"})
	resource.Register("codeartifact", func(ctx context.Context, clients interface{}) ([]resource.Resource, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return nil, fmt.Errorf("AWS clients not initialized")
		}
		return FetchCodeArtifactRepos(ctx, c.CodeArtifact)
	})
}

// FetchCodeArtifactRepos calls the CodeArtifact ListRepositories API and converts the
// response into a slice of generic Resource structs.
func FetchCodeArtifactRepos(ctx context.Context, api CodeArtifactListRepositoriesAPI) ([]resource.Resource, error) {
	output, err := api.ListRepositories(ctx, &codeartifact.ListRepositoriesInput{})
	if err != nil {
		return nil, err
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
			createdTime = repo.CreatedTime.Format("2006-01-02 15:04:05")
		}

		// Build RawJSON
		rawJSON := ""
		if jsonBytes, err := json.MarshalIndent(repo, "", "  "); err == nil {
			rawJSON = string(jsonBytes)
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
			RawJSON:   rawJSON,
			RawStruct: repo,
		}

		resources = append(resources, r)
	}

	return resources, nil
}
