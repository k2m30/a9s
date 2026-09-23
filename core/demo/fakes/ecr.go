// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package fakes

import (
	"context"
	"slices"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	ecrtypes "github.com/aws/aws-sdk-go-v2/service/ecr/types"

	"github.com/k2m30/a9s/v3/core/demo/fixtures"
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

// DescribeImages answers the way Basic Scanning does: without the scan
// summary and scan status, which DescribeImageScanFindings serves.
func (f *ECRFake) DescribeImages(_ context.Context, input *ecr.DescribeImagesInput, _ ...func(*ecr.Options)) (*ecr.DescribeImagesOutput, error) {
	var repoName string
	if input != nil && input.RepositoryName != nil {
		repoName = *input.RepositoryName
	}
	images := slices.Clone(f.fix.Images[repoName])
	for i := range images {
		images[i].ImageScanFindingsSummary = nil
		images[i].ImageScanStatus = nil
	}
	return &ecr.DescribeImagesOutput{ImageDetails: images}, nil
}

// DescribeImageScanFindings serves the fixture image's scan summary. An image
// the fixtures give no summary was never scanned and answers
// ScanNotFoundException, as ECR does.
func (f *ECRFake) DescribeImageScanFindings(_ context.Context, input *ecr.DescribeImageScanFindingsInput, _ ...func(*ecr.Options)) (*ecr.DescribeImageScanFindingsOutput, error) {
	repoName := aws.ToString(input.RepositoryName)
	var digest string
	if input.ImageId != nil {
		digest = aws.ToString(input.ImageId.ImageDigest)
	}
	for _, img := range f.fix.Images[repoName] {
		if aws.ToString(img.ImageDigest) != digest || img.ImageScanFindingsSummary == nil {
			continue
		}
		return &ecr.DescribeImageScanFindingsOutput{
			RepositoryName:  input.RepositoryName,
			ImageId:         input.ImageId,
			ImageScanStatus: &ecrtypes.ImageScanStatus{Status: ecrtypes.ScanStatusComplete},
			ImageScanFindings: &ecrtypes.ImageScanFindings{
				FindingSeverityCounts:        img.ImageScanFindingsSummary.FindingSeverityCounts,
				ImageScanCompletedAt:         img.ImageScanFindingsSummary.ImageScanCompletedAt,
				VulnerabilitySourceUpdatedAt: img.ImageScanFindingsSummary.VulnerabilitySourceUpdatedAt,
			},
		}, nil
	}
	return nil, &ecrtypes.ScanNotFoundException{Message: aws.String("Image scan does not exist for the image with digest " + digest)}
}

// ListImages returns image identifiers for the requested repository from fixture data.
func (f *ECRFake) ListImages(_ context.Context, input *ecr.ListImagesInput, _ ...func(*ecr.Options)) (*ecr.ListImagesOutput, error) {
	var repoName string
	if input != nil && input.RepositoryName != nil {
		repoName = *input.RepositoryName
	}
	if !f.hasRepository(repoName) {
		return nil, &ecrtypes.RepositoryNotFoundException{
			Message: notFoundMessage("The repository with name", repoName),
		}
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

// GetRepositoryPolicy returns fixture policy JSON for the requested
// repository. checkECRRole reduces the policy's Principal.AWS role ARNs
// to bare RoleName before returning (core/aws/ecr_related_extra.go), so
// the fixture policy's role resolves cleanly via FetchRolesByIDs.
func (f *ECRFake) GetRepositoryPolicy(_ context.Context, input *ecr.GetRepositoryPolicyInput, _ ...func(*ecr.Options)) (*ecr.GetRepositoryPolicyOutput, error) {
	var repoName string
	if input != nil && input.RepositoryName != nil {
		repoName = *input.RepositoryName
	}
	policyText, ok := f.fix.Policies[repoName]
	if !ok {
		return nil, &ecrtypes.RepositoryPolicyNotFoundException{Message: aws.String("no policy configured for this repository")}
	}
	return &ecr.GetRepositoryPolicyOutput{
		RepositoryName: input.RepositoryName,
		PolicyText:     &policyText,
	}, nil
}

// ListTagsForResource returns fixture tags for the given repository ARN,
// backing the ecr:cfn related-panel pivot (checkECRCFN).
func (f *ECRFake) ListTagsForResource(_ context.Context, input *ecr.ListTagsForResourceInput, _ ...func(*ecr.Options)) (*ecr.ListTagsForResourceOutput, error) {
	var arn string
	if input != nil && input.ResourceArn != nil {
		arn = *input.ResourceArn
	}
	return &ecr.ListTagsForResourceOutput{Tags: f.fix.Tags[arn]}, nil
}

// GetLifecyclePolicy returns the fixture lifecycle policy for the requested
// repository, or LifecyclePolicyNotFoundException when the repository has
// none — the absence is what EnrichECRRepository reports, so the not-found
// error is a normal answer here rather than a failure.
func (f *ECRFake) GetLifecyclePolicy(_ context.Context, input *ecr.GetLifecyclePolicyInput, _ ...func(*ecr.Options)) (*ecr.GetLifecyclePolicyOutput, error) {
	var repoName string
	if input != nil && input.RepositoryName != nil {
		repoName = *input.RepositoryName
	}
	policyText, ok := f.fix.LifecyclePolicies[repoName]
	if !ok {
		return nil, &ecrtypes.LifecyclePolicyNotFoundException{Message: aws.String("no lifecycle policy configured for this repository")}
	}
	return &ecr.GetLifecyclePolicyOutput{
		RepositoryName:      input.RepositoryName,
		LifecyclePolicyText: &policyText,
	}, nil
}

// hasRepository reports whether the fixtures register this repository. An
// empty repository still answers an empty image list.
func (f *ECRFake) hasRepository(name string) bool {
	return slices.ContainsFunc(f.fix.Repositories, func(r ecrtypes.Repository) bool {
		return aws.ToString(r.RepositoryName) == name
	})
}
