package fakes

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/codeartifact"
	codeartifacttypes "github.com/aws/aws-sdk-go-v2/service/codeartifact/types"

	"github.com/k2m30/a9s/v3/core/demo/fixtures"
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

// GetRepositoryPermissionsPolicy returns the fixture-registered policy for
// the requested repository (see CodeArtifactFixtures.PermissionsPolicies).
// Repositories with no registered policy return ResourceNotFoundException,
// matching real AWS behavior for a repository with no permissions policy set —
// required for EnrichCodeArtifactRepository's Wave-2 issue checks.
func (f *CodeArtifactFake) GetRepositoryPermissionsPolicy(_ context.Context, input *codeartifact.GetRepositoryPermissionsPolicyInput, _ ...func(*codeartifact.Options)) (*codeartifact.GetRepositoryPermissionsPolicyOutput, error) {
	var repoName string
	if input != nil && input.Repository != nil {
		repoName = *input.Repository
	}
	policy, ok := f.fix.PermissionsPolicies[repoName]
	if !ok {
		return nil, &codeartifacttypes.ResourceNotFoundException{
			Message: aws.String("policy does not exist for repository " + repoName),
		}
	}
	return &codeartifact.GetRepositoryPermissionsPolicyOutput{Policy: policy}, nil
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

// DescribeDomain returns the fixture-registered domain description for the
// requested domain name (see CodeArtifactFixtures.Domains).
func (f *CodeArtifactFake) DescribeDomain(_ context.Context, input *codeartifact.DescribeDomainInput, _ ...func(*codeartifact.Options)) (*codeartifact.DescribeDomainOutput, error) {
	var domainName string
	if input != nil && input.Domain != nil {
		domainName = *input.Domain
	}
	domain, ok := f.fix.Domains[domainName]
	if !ok {
		return &codeartifact.DescribeDomainOutput{}, nil
	}
	return &codeartifact.DescribeDomainOutput{Domain: &domain}, nil
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
