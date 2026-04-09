package fixtures

import (
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	ecrtypes "github.com/aws/aws-sdk-go-v2/service/ecr/types"
)

// ECRFixtures holds typed fixture data for ECR.
type ECRFixtures struct {
	Repositories []ecrtypes.Repository
	// Images maps repository name to its images (for DescribeImages).
	Images map[string][]ecrtypes.ImageDetail
}

const prodKMSKeyID = "a1b2c3d4-5678-90ab-cdef-111111111111"

func mustParseECRTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}

// NewECRFixtures constructs ECRFixtures from the canonical demo data.
func NewECRFixtures() *ECRFixtures {
	repos := []ecrtypes.Repository{
		{
			RepositoryName: aws.String("acme/api-service"),
			RepositoryUri:  aws.String("123456789012.dkr.ecr.us-east-1.amazonaws.com/acme/api-service"),
			RepositoryArn:  aws.String("arn:aws:ecr:us-east-1:123456789012:repository/acme/api-service"),
			RegistryId:     aws.String("123456789012"),
			ImageTagMutability: ecrtypes.ImageTagMutabilityImmutable,
			ImageScanningConfiguration: &ecrtypes.ImageScanningConfiguration{ScanOnPush: true},
			EncryptionConfiguration: &ecrtypes.EncryptionConfiguration{
				EncryptionType: ecrtypes.EncryptionTypeKms,
				KmsKey:         aws.String(prodKMSKeyID),
			},
			CreatedAt: aws.Time(mustParseECRTime("2025-03-01T10:00:00+00:00")),
		},
		{
			RepositoryName: aws.String("acme/frontend"),
			RepositoryUri:  aws.String("123456789012.dkr.ecr.us-east-1.amazonaws.com/acme/frontend"),
			RepositoryArn:  aws.String("arn:aws:ecr:us-east-1:123456789012:repository/acme/frontend"),
			RegistryId:     aws.String("123456789012"),
			ImageTagMutability: ecrtypes.ImageTagMutabilityMutable,
			ImageScanningConfiguration: &ecrtypes.ImageScanningConfiguration{ScanOnPush: true},
			EncryptionConfiguration: &ecrtypes.EncryptionConfiguration{
				EncryptionType: ecrtypes.EncryptionTypeKms,
				KmsKey:         aws.String(prodKMSKeyID),
			},
			CreatedAt: aws.Time(mustParseECRTime("2025-03-01T10:05:00+00:00")),
		},
		{
			RepositoryName: aws.String("acme/base-images"),
			RepositoryUri:  aws.String("123456789012.dkr.ecr.us-east-1.amazonaws.com/acme/base-images"),
			RepositoryArn:  aws.String("arn:aws:ecr:us-east-1:123456789012:repository/acme/base-images"),
			RegistryId:     aws.String("123456789012"),
			ImageTagMutability: ecrtypes.ImageTagMutabilityImmutable,
			ImageScanningConfiguration: &ecrtypes.ImageScanningConfiguration{ScanOnPush: false},
			EncryptionConfiguration: &ecrtypes.EncryptionConfiguration{
				EncryptionType: ecrtypes.EncryptionTypeKms,
				KmsKey:         aws.String(prodKMSKeyID),
			},
			CreatedAt: aws.Time(mustParseECRTime("2025-01-15T08:30:00+00:00")),
		},
		{
			RepositoryName: aws.String("acme/batch-processor"),
			RepositoryUri:  aws.String("123456789012.dkr.ecr.us-east-1.amazonaws.com/acme/batch-processor"),
			RepositoryArn:  aws.String("arn:aws:ecr:us-east-1:123456789012:repository/acme/batch-processor"),
			RegistryId:     aws.String("123456789012"),
			ImageTagMutability: ecrtypes.ImageTagMutabilityMutable,
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
			{
				ImageTags:      []string{"v2.5.1", "latest"},
				ImageDigest:    aws.String("sha256:abc123def456"),
				ImageSizeInBytes: aws.Int64(85_000_000),
				ImagePushedAt:  aws.Time(mustParseECRTime("2026-03-22T03:20:00+00:00")),
				RegistryId:     aws.String("123456789012"),
				RepositoryName: aws.String("acme/api-service"),
			},
			{
				ImageTags:      []string{"v2.5.0"},
				ImageDigest:    aws.String("sha256:def789abc012"),
				ImageSizeInBytes: aws.Int64(84_500_000),
				ImagePushedAt:  aws.Time(mustParseECRTime("2026-03-15T10:00:00+00:00")),
				RegistryId:     aws.String("123456789012"),
				RepositoryName: aws.String("acme/api-service"),
			},
		},
	}

	return &ECRFixtures{
		Repositories: repos,
		Images:       images,
	}
}
