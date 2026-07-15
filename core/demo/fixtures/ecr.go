// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package fixtures

import (
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	ecrtypes "github.com/aws/aws-sdk-go-v2/service/ecr/types"
)

// ECRFixtures holds typed fixture data for ECR.
type ECRFixtures struct {
	Repositories []ecrtypes.Repository
	// Images maps repository name to its images (for DescribeImages).
	Images map[string][]ecrtypes.ImageDetail
	// Tags maps repository ARN to its tags (for ListTagsForResource) —
	// required for the ecr:cfn related-panel pivot (checkECRCFN).
	Tags map[string][]ecrtypes.Tag
	// Policies maps repository name to its resource-based policy JSON (for
	// GetRepositoryPolicy) — required for the ecr:role related-panel pivot
	// (checkECRRole).
	Policies map[string]string
}

// APIServiceRepoPolicyRoleName is the IAM role name granted pull access via
// the acme/api-service repository policy — matches the acme-ci-deploy-role
// fixture (iam.go), required for the ecr:role related-panel pivot
// (checkECRRole).
const APIServiceRepoPolicyRoleName = "acme-ci-deploy-role"

// BaseImagesRepoStackName is the aws:cloudformation:stack-name tag value on
// the acme/base-images ECR repository — matches the acme-monitoring stack
// (cfn.go), required for the ecr:cfn related-panel pivot (checkECRCFN).
const BaseImagesRepoStackName = "acme-monitoring"

const prodKMSKeyID = "a1b2c3d4-5678-90ab-cdef-111111111111"

func mustParseECRTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}

// NewECRFixtures constructs ECRFixtures from the canonical demo data.
var sharedECRFixtures = sync.OnceValue(func() *ECRFixtures {
	repos := []ecrtypes.Repository{
		{
			RepositoryName:             aws.String("acme/api-service"),
			RepositoryUri:              aws.String("123456789012.dkr.ecr.us-east-1.amazonaws.com/acme/api-service"),
			RepositoryArn:              aws.String("arn:aws:ecr:us-east-1:123456789012:repository/acme/api-service"),
			RegistryId:                 aws.String("123456789012"),
			ImageTagMutability:         ecrtypes.ImageTagMutabilityImmutable,
			ImageScanningConfiguration: &ecrtypes.ImageScanningConfiguration{ScanOnPush: true},
			EncryptionConfiguration: &ecrtypes.EncryptionConfiguration{
				EncryptionType: ecrtypes.EncryptionTypeKms,
				KmsKey:         aws.String(prodKMSKeyID),
			},
			CreatedAt: aws.Time(mustParseECRTime("2025-03-01T10:00:00+00:00")),
		},
		{
			RepositoryName:             aws.String("acme/frontend"),
			RepositoryUri:              aws.String("123456789012.dkr.ecr.us-east-1.amazonaws.com/acme/frontend"),
			RepositoryArn:              aws.String("arn:aws:ecr:us-east-1:123456789012:repository/acme/frontend"),
			RegistryId:                 aws.String("123456789012"),
			ImageTagMutability:         ecrtypes.ImageTagMutabilityMutable,
			ImageScanningConfiguration: &ecrtypes.ImageScanningConfiguration{ScanOnPush: true},
			EncryptionConfiguration: &ecrtypes.EncryptionConfiguration{
				EncryptionType: ecrtypes.EncryptionTypeKms,
				KmsKey:         aws.String(prodKMSKeyID),
			},
			CreatedAt: aws.Time(mustParseECRTime("2025-03-01T10:05:00+00:00")),
		},
		{
			RepositoryName:             aws.String("acme/base-images"),
			RepositoryUri:              aws.String("123456789012.dkr.ecr.us-east-1.amazonaws.com/acme/base-images"),
			RepositoryArn:              aws.String("arn:aws:ecr:us-east-1:123456789012:repository/acme/base-images"),
			RegistryId:                 aws.String("123456789012"),
			ImageTagMutability:         ecrtypes.ImageTagMutabilityImmutable,
			ImageScanningConfiguration: &ecrtypes.ImageScanningConfiguration{ScanOnPush: false},
			EncryptionConfiguration: &ecrtypes.EncryptionConfiguration{
				EncryptionType: ecrtypes.EncryptionTypeKms,
				KmsKey:         aws.String(prodKMSKeyID),
			},
			CreatedAt: aws.Time(mustParseECRTime("2025-01-15T08:30:00+00:00")),
		},
		{
			RepositoryName:             aws.String("acme/batch-processor"),
			RepositoryUri:              aws.String("123456789012.dkr.ecr.us-east-1.amazonaws.com/acme/batch-processor"),
			RepositoryArn:              aws.String("arn:aws:ecr:us-east-1:123456789012:repository/acme/batch-processor"),
			RegistryId:                 aws.String("123456789012"),
			ImageTagMutability:         ecrtypes.ImageTagMutabilityMutable,
			ImageScanningConfiguration: &ecrtypes.ImageScanningConfiguration{ScanOnPush: true},
			EncryptionConfiguration: &ecrtypes.EncryptionConfiguration{
				EncryptionType: ecrtypes.EncryptionTypeKms,
				KmsKey:         aws.String(prodKMSKeyID),
			},
			CreatedAt: aws.Time(mustParseECRTime("2025-06-20T12:00:00+00:00")),
		},
	}

	images := map[string][]ecrtypes.ImageDetail{
		"acme/api-service": {
			// Issue: CRITICAL vulnerability findings on the latest image —
			// required for EnrichECRRepository's Wave-2 "!" issue check.
			{
				ImageTags:        []string{"v2.5.1", "latest"},
				ImageDigest:      aws.String("sha256:abc123def456"),
				ImageSizeInBytes: aws.Int64(85_000_000),
				ImagePushedAt:    aws.Time(mustParseECRTime("2026-03-22T03:20:00+00:00")),
				RegistryId:       aws.String("123456789012"),
				RepositoryName:   aws.String("acme/api-service"),
				ImageScanFindingsSummary: &ecrtypes.ImageScanFindingsSummary{
					FindingSeverityCounts: map[string]int32{
						string(ecrtypes.FindingSeverityCritical): 2,
						string(ecrtypes.FindingSeverityHigh):     5,
					},
				},
			},
			{
				ImageTags:        []string{"v2.5.0"},
				ImageDigest:      aws.String("sha256:def789abc012"),
				ImageSizeInBytes: aws.Int64(84_500_000),
				ImagePushedAt:    aws.Time(mustParseECRTime("2026-03-15T10:00:00+00:00")),
				RegistryId:       aws.String("123456789012"),
				RepositoryName:   aws.String("acme/api-service"),
			},
		},
	}

	tags := map[string][]ecrtypes.Tag{
		"arn:aws:ecr:us-east-1:123456789012:repository/acme/base-images": {
			{Key: aws.String("aws:cloudformation:stack-name"), Value: aws.String(BaseImagesRepoStackName)},
		},
	}

	policies := map[string]string{
		"acme/api-service": `{"Version":"2012-10-17","Statement":[{"Sid":"AllowCIPull","Effect":"Allow","Principal":{"AWS":"arn:aws:iam::123456789012:role/` + APIServiceRepoPolicyRoleName + `"},"Action":["ecr:GetDownloadUrlForLayer","ecr:BatchGetImage","ecr:BatchCheckLayerAvailability"]}]}`,
	}

	return &ECRFixtures{
		Repositories: repos,
		Images:       images,
		Tags:         tags,
		Policies:     policies,
	}
})

func NewECRFixtures() *ECRFixtures {
	return sharedECRFixtures()
}
