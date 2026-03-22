package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/ecr"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("ecr", []string{"repository_name", "uri", "tag_mutability", "scan_on_push", "created_at"})
	resource.Register("ecr", func(ctx context.Context, clients interface{}) ([]resource.Resource, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return nil, fmt.Errorf("AWS clients not initialized")
		}
		return FetchECRRepositories(ctx, c.ECR)
	})
}

// FetchECRRepositories calls the ECR DescribeRepositories API and converts
// the response into a slice of generic Resource structs.
func FetchECRRepositories(ctx context.Context, api ECRDescribeRepositoriesAPI) ([]resource.Resource, error) {
	output, err := api.DescribeRepositories(ctx, &ecr.DescribeRepositoriesInput{})
	if err != nil {
		return nil, fmt.Errorf("fetching ECR repositories: %w", err)
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
			createdAt = repo.CreatedAt.Format("2006-01-02 15:04:05")
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
			RawStruct:  repo,
		}

		resources = append(resources, r)
	}

	return resources, nil
}
