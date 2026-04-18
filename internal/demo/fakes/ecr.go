package fakes

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/ecr"
	ecrtypes "github.com/aws/aws-sdk-go-v2/service/ecr/types"

	"github.com/k2m30/a9s/v3/internal/demo/fixtures"
)

// ECRFake implements aws.ECRAPI against fixture data loaded at construction time.
type ECRFake struct {
	fix *fixtures.ECRFixtures
}

// NewECR constructs an ECRFake backed by fixture data from the fixtures package.
func NewECR() *ECRFake {
	return &ECRFake{fix: fixtures.NewECRFixtures()}
}

func (f *ECRFake) DescribeRepositories(_ context.Context, _ *ecr.DescribeRepositoriesInput, _ ...func(*ecr.Options)) (*ecr.DescribeRepositoriesOutput, error) {
	return &ecr.DescribeRepositoriesOutput{Repositories: f.fix.Repositories}, nil
}

func (f *ECRFake) DescribeImages(_ context.Context, input *ecr.DescribeImagesInput, _ ...func(*ecr.Options)) (*ecr.DescribeImagesOutput, error) {
	var repoName string
	if input != nil && input.RepositoryName != nil {
		repoName = *input.RepositoryName
	}
	return &ecr.DescribeImagesOutput{ImageDetails: f.fix.Images[repoName]}, nil
}

// DescribeImageScanFindings is a stub for Wave 2 enrichment in demo mode.
// Returns an empty response (no scan findings) for all repositories.
func (f *ECRFake) DescribeImageScanFindings(_ context.Context, _ *ecr.DescribeImageScanFindingsInput, _ ...func(*ecr.Options)) (*ecr.DescribeImageScanFindingsOutput, error) {
	return &ecr.DescribeImageScanFindingsOutput{}, nil
}

// ListImages returns image identifiers for the requested repository from fixture data.
// Satisfies ECRListImagesAPI for Wave 2 enrichment in demo mode.
func (f *ECRFake) ListImages(_ context.Context, input *ecr.ListImagesInput, _ ...func(*ecr.Options)) (*ecr.ListImagesOutput, error) {
	var repoName string
	if input != nil && input.RepositoryName != nil {
		repoName = *input.RepositoryName
	}
	images := f.fix.Images[repoName]
	ids := make([]ecrtypes.ImageIdentifier, 0, len(images))
	for _, img := range images {
		if img.ImageDigest != nil {
			ids = append(ids, ecrtypes.ImageIdentifier{ImageDigest: img.ImageDigest})
		}
	}
	return &ecr.ListImagesOutput{ImageIds: ids}, nil
}

// GetRepositoryPolicy is a no-op stub satisfying ECRGetRepositoryPolicyAPI.
// Demo mode does not model ECR repository policies.
func (f *ECRFake) GetRepositoryPolicy(_ context.Context, _ *ecr.GetRepositoryPolicyInput, _ ...func(*ecr.Options)) (*ecr.GetRepositoryPolicyOutput, error) {
	return &ecr.GetRepositoryPolicyOutput{}, nil
}
