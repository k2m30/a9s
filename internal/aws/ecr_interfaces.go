package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/ecr"
)

// ECRDescribeRepositoriesAPI defines the interface for the ECR DescribeRepositories operation.
type ECRDescribeRepositoriesAPI interface {
	DescribeRepositories(ctx context.Context, params *ecr.DescribeRepositoriesInput, optFns ...func(*ecr.Options)) (*ecr.DescribeRepositoriesOutput, error)
}

// ECRGetRepositoryPolicyAPI defines the interface for the ECR GetRepositoryPolicy operation.
// Used by checkECRRole to extract IAM roles from the repository's resource-based policy.
type ECRGetRepositoryPolicyAPI interface {
	GetRepositoryPolicy(ctx context.Context, params *ecr.GetRepositoryPolicyInput, optFns ...func(*ecr.Options)) (*ecr.GetRepositoryPolicyOutput, error)
}

// ECRDescribeImagesAPI defines the interface for the ECR DescribeImages operation.
type ECRDescribeImagesAPI interface {
	DescribeImages(ctx context.Context, params *ecr.DescribeImagesInput, optFns ...func(*ecr.Options)) (*ecr.DescribeImagesOutput, error)
}

// ECRDescribeImageScanFindingsAPI defines the interface for the ECR DescribeImageScanFindings operation.
// Used by the Wave 2 EnrichECRRepository enricher.
type ECRDescribeImageScanFindingsAPI interface {
	DescribeImageScanFindings(ctx context.Context, params *ecr.DescribeImageScanFindingsInput, optFns ...func(*ecr.Options)) (*ecr.DescribeImageScanFindingsOutput, error)
}

// ECRListImagesAPI defines the interface for the ECR ListImages operation.
// Used by the Wave 2 EnrichECRRepository enricher to enumerate image IDs per repository
// before calling DescribeImageScanFindings on each image.
type ECRListImagesAPI interface {
	ListImages(ctx context.Context, params *ecr.ListImagesInput, optFns ...func(*ecr.Options)) (*ecr.ListImagesOutput, error)
}

// ECRAPI is the aggregate interface covering all ECR operations used by a9s fetchers.
// *ecr.Client structurally satisfies this interface.
type ECRAPI interface {
	ECRDescribeRepositoriesAPI
	ECRDescribeImagesAPI
	ECRDescribeImageScanFindingsAPI // Wave 2 enrichment
	ECRGetRepositoryPolicyAPI       // related-panel: ecr→role
}
