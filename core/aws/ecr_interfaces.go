// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

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

// ECRListTagsForResourceAPI defines the interface for the ECR ListTagsForResource operation.
// Used by checkECRCFN to read the aws:cloudformation:stack-name tag directly
// from the repository (ECR does not embed tags in DescribeRepositories).
type ECRListTagsForResourceAPI interface {
	ListTagsForResource(ctx context.Context, params *ecr.ListTagsForResourceInput, optFns ...func(*ecr.Options)) (*ecr.ListTagsForResourceOutput, error)
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

// ECRGetLifecyclePolicyAPI defines the interface for the ECR
// GetLifecyclePolicy operation. Used by the Wave 2 EnrichECRRepository
// enricher: the absence of a policy (LifecyclePolicyNotFoundException) is
// itself the finding, so there is no way to read this from
// DescribeRepositories.
//
// Deliberately NOT part of the aggregate ECRAPI: the enricher type-asserts
// for it the way it already does for ECRDescribeImagesAPI, so a caller
// wired with a client that predates this call degrades to "no lifecycle
// finding" instead of failing to construct. *ecr.Client satisfies it.
type ECRGetLifecyclePolicyAPI interface {
	GetLifecyclePolicy(ctx context.Context, params *ecr.GetLifecyclePolicyInput, optFns ...func(*ecr.Options)) (*ecr.GetLifecyclePolicyOutput, error)
}

// ECRAPI is the aggregate interface covering all ECR operations used by a9s fetchers.
// *ecr.Client structurally satisfies this interface.
type ECRAPI interface {
	ECRDescribeRepositoriesAPI
	ECRDescribeImagesAPI
	ECRDescribeImageScanFindingsAPI // Wave 2 enrichment
	ECRGetRepositoryPolicyAPI       // related-panel: ecr→role, Wave 2 public-policy
	ECRListTagsForResourceAPI       // related-panel: ecr→cfn
}
