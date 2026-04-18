package fakes

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/codeartifact"
	codeartifacttypes "github.com/aws/aws-sdk-go-v2/service/codeartifact/types"

	"github.com/k2m30/a9s/v3/internal/demo/fixtures"
)

// CodeArtifactFake implements aws.CodeArtifactAPI against fixture data loaded at construction time.
type CodeArtifactFake struct {
	fix *fixtures.CodeArtifactFixtures
}

// NewCodeArtifact constructs a CodeArtifactFake backed by fixture data from the fixtures package.
func NewCodeArtifact() *CodeArtifactFake {
	return &CodeArtifactFake{fix: fixtures.NewCodeArtifactFixtures()}
}

func (f *CodeArtifactFake) ListRepositories(_ context.Context, _ *codeartifact.ListRepositoriesInput, _ ...func(*codeartifact.Options)) (*codeartifact.ListRepositoriesOutput, error) {
	return &codeartifact.ListRepositoriesOutput{Repositories: f.fix.Repositories}, nil
}

// GetRepositoryPermissionsPolicy is a stub satisfying CodeArtifactGetRepositoryPermissionsPolicyAPI.
// Demo mode returns no policy (nil Policy), simulating repositories without a permissions policy.
func (f *CodeArtifactFake) GetRepositoryPermissionsPolicy(_ context.Context, _ *codeartifact.GetRepositoryPermissionsPolicyInput, _ ...func(*codeartifact.Options)) (*codeartifact.GetRepositoryPermissionsPolicyOutput, error) {
	return &codeartifact.GetRepositoryPermissionsPolicyOutput{}, nil
}

// DescribeRepository returns an empty repository — demo mode does not model repository details.
func (f *CodeArtifactFake) DescribeRepository(_ context.Context, _ *codeartifact.DescribeRepositoryInput, _ ...func(*codeartifact.Options)) (*codeartifact.DescribeRepositoryOutput, error) {
	return &codeartifact.DescribeRepositoryOutput{}, nil
}

// GetDomainPermissionsPolicy is a no-op stub satisfying CodeArtifactGetDomainPermissionsPolicyAPI.
// Demo mode does not model CodeArtifact domain permissions policies.
func (f *CodeArtifactFake) GetDomainPermissionsPolicy(_ context.Context, _ *codeartifact.GetDomainPermissionsPolicyInput, _ ...func(*codeartifact.Options)) (*codeartifact.GetDomainPermissionsPolicyOutput, error) {
	return &codeartifact.GetDomainPermissionsPolicyOutput{}, nil
}

// DescribeDomain is a no-op stub satisfying CodeArtifactDescribeDomainAPI.
// Demo mode does not model CodeArtifact domain KMS encryption keys.
func (f *CodeArtifactFake) DescribeDomain(_ context.Context, _ *codeartifact.DescribeDomainInput, _ ...func(*codeartifact.Options)) (*codeartifact.DescribeDomainOutput, error) {
	return &codeartifact.DescribeDomainOutput{}, nil
}

// ListPackages returns stub package summaries for demo mode.
// acme-npm returns 12 npm packages; acme-pypi returns 8; acme-maven returns 5.
func (f *CodeArtifactFake) ListPackages(_ context.Context, input *codeartifact.ListPackagesInput, _ ...func(*codeartifact.Options)) (*codeartifact.ListPackagesOutput, error) {
	repoPackageCounts := map[string]int{
		"acme-npm":   12,
		"acme-pypi":  8,
		"acme-maven": 5,
	}
	repoName := ""
	if input.Repository != nil {
		repoName = *input.Repository
	}
	count := repoPackageCounts[repoName]
	packages := make([]codeartifacttypes.PackageSummary, count)
	for i := range packages {
		packages[i] = codeartifacttypes.PackageSummary{
			Package: aws.String("pkg"),
		}
	}
	return &codeartifact.ListPackagesOutput{Packages: packages}, nil
}
