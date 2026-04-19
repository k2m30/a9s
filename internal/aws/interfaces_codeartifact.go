package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/codeartifact"
)

// CodeArtifactListRepositoriesAPI defines the interface for the CodeArtifact ListRepositories operation.
type CodeArtifactListRepositoriesAPI interface {
	ListRepositories(ctx context.Context, params *codeartifact.ListRepositoriesInput, optFns ...func(*codeartifact.Options)) (*codeartifact.ListRepositoriesOutput, error)
}

// CodeArtifactGetRepositoryPermissionsPolicyAPI defines the interface for the CodeArtifact
// GetRepositoryPermissionsPolicy operation. Used by EnrichCodeArtifactRepository (Wave 2 enrichment).
type CodeArtifactGetRepositoryPermissionsPolicyAPI interface {
	GetRepositoryPermissionsPolicy(ctx context.Context, params *codeartifact.GetRepositoryPermissionsPolicyInput, optFns ...func(*codeartifact.Options)) (*codeartifact.GetRepositoryPermissionsPolicyOutput, error)
}

// CodeArtifactGetDomainPermissionsPolicyAPI defines the interface for the CodeArtifact GetDomainPermissionsPolicy operation.
type CodeArtifactGetDomainPermissionsPolicyAPI interface {
	GetDomainPermissionsPolicy(ctx context.Context, params *codeartifact.GetDomainPermissionsPolicyInput, optFns ...func(*codeartifact.Options)) (*codeartifact.GetDomainPermissionsPolicyOutput, error)
}

// CodeArtifactDescribeDomainAPI defines the interface for the CodeArtifact DescribeDomain operation.
// Used by checkCodeartifactKMS to resolve the KMS encryption key for the repository's domain.
type CodeArtifactDescribeDomainAPI interface {
	DescribeDomain(ctx context.Context, params *codeartifact.DescribeDomainInput, optFns ...func(*codeartifact.Options)) (*codeartifact.DescribeDomainOutput, error)
}

// CodeArtifactListPackagesAPI defines the interface for the CodeArtifact ListPackages operation.
// Used by EnrichCodeArtifactRepository to count packages per repository.
type CodeArtifactListPackagesAPI interface {
	ListPackages(ctx context.Context, params *codeartifact.ListPackagesInput, optFns ...func(*codeartifact.Options)) (*codeartifact.ListPackagesOutput, error)
}

// CodeArtifactDescribeRepositoryAPI for codeartifact→* enrichers.
type CodeArtifactDescribeRepositoryAPI interface {
	DescribeRepository(ctx context.Context, params *codeartifact.DescribeRepositoryInput, optFns ...func(*codeartifact.Options)) (*codeartifact.DescribeRepositoryOutput, error)
}

// CodeArtifactAPI is the aggregate interface covering all CodeArtifact operations used by a9s fetchers.
// *codeartifact.Client structurally satisfies this interface.
type CodeArtifactAPI interface {
	CodeArtifactListRepositoriesAPI
	CodeArtifactGetRepositoryPermissionsPolicyAPI // Wave 2 enrichment
	CodeArtifactDescribeRepositoryAPI
	CodeArtifactGetDomainPermissionsPolicyAPI
	CodeArtifactDescribeDomainAPI
}
