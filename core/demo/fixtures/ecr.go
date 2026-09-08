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
	// LifecyclePolicies maps repository name to its lifecycle policy JSON
	// (for GetLifecyclePolicy). A repository absent from this map has no
	// lifecycle policy, which is what ECRNoLifecycle witnesses.
	LifecyclePolicies map[string]string
}

// Witness repositories for the ecr posture findings. Each names the ONE demo
// repository that carries its finding; every other repository is set to the
// healthy value for that condition.
const (
	// ECRPublicPolicy — repository policy grants a wildcard principal.
	ECRPublicPolicy = "acme/public-mirror"
	// ECRScanOnPushOff — scan on push is off.
	ECRScanOnPushOff = "acme/base-images"
	// ECRMutableTags — image tags can be overwritten.
	ECRMutableTags = "acme/frontend"
	// ECRNoLifecycle — no lifecycle policy is configured.
	ECRNoLifecycle = "acme/batch-processor"
	// ECRHighVulnerabilities — the latest image has high findings and no
	// critical one, which is the warning tier of the vulnerability signal.
	ECRHighVulnerabilities = "acme/reporting"
)

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
			// ECRMutableTags witness: a deployed tag can be moved to different
			// image content without any deployment.
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
			// ECRScanOnPushOff witness: images arrive unscanned.
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
			// ECRHighVulnerabilities witness: high findings, no critical.
			RepositoryName:             aws.String(ECRHighVulnerabilities),
			RepositoryUri:              aws.String("123456789012.dkr.ecr.us-east-1.amazonaws.com/" + ECRHighVulnerabilities),
			RepositoryArn:              aws.String("arn:aws:ecr:us-east-1:123456789012:repository/" + ECRHighVulnerabilities),
			RegistryId:                 aws.String("123456789012"),
			ImageTagMutability:         ecrtypes.ImageTagMutabilityImmutable,
			ImageScanningConfiguration: &ecrtypes.ImageScanningConfiguration{ScanOnPush: true},
			EncryptionConfiguration: &ecrtypes.EncryptionConfiguration{
				EncryptionType: ecrtypes.EncryptionTypeKms,
				KmsKey:         aws.String(prodKMSKeyID),
			},
			CreatedAt: aws.Time(mustParseECRTime("2025-07-04T09:00:00+00:00")),
		},
		{
			// ECRNoLifecycle witness: absent from the LifecyclePolicies map
			// below, so every image it has ever held is kept forever.
			RepositoryName:             aws.String("acme/batch-processor"),
			RepositoryUri:              aws.String("123456789012.dkr.ecr.us-east-1.amazonaws.com/acme/batch-processor"),
			RepositoryArn:              aws.String("arn:aws:ecr:us-east-1:123456789012:repository/acme/batch-processor"),
			RegistryId:                 aws.String("123456789012"),
			ImageTagMutability:         ecrtypes.ImageTagMutabilityImmutable,
			ImageScanningConfiguration: &ecrtypes.ImageScanningConfiguration{ScanOnPush: true},
			EncryptionConfiguration: &ecrtypes.EncryptionConfiguration{
				EncryptionType: ecrtypes.EncryptionTypeKms,
				KmsKey:         aws.String(prodKMSKeyID),
			},
			CreatedAt: aws.Time(mustParseECRTime("2025-06-20T12:00:00+00:00")),
		},
		// ECRPublicPolicy witness: the repository policy grants a wildcard
		// principal, so any AWS account can pull these images.
		{
			RepositoryName:             aws.String(ECRPublicPolicy),
			RepositoryUri:              aws.String("123456789012.dkr.ecr.us-east-1.amazonaws.com/" + ECRPublicPolicy),
			RepositoryArn:              aws.String("arn:aws:ecr:us-east-1:123456789012:repository/" + ECRPublicPolicy),
			RegistryId:                 aws.String("123456789012"),
			ImageTagMutability:         ecrtypes.ImageTagMutabilityImmutable,
			ImageScanningConfiguration: &ecrtypes.ImageScanningConfiguration{ScanOnPush: true},
			EncryptionConfiguration: &ecrtypes.EncryptionConfiguration{
				EncryptionType: ecrtypes.EncryptionTypeKms,
				KmsKey:         aws.String(prodKMSKeyID),
			},
			CreatedAt: aws.Time(mustParseECRTime("2025-08-14T09:30:00+00:00")),
		},
		// The healthy repository: immutable tags, scan on push, a lifecycle
		// policy, no resource policy and no vulnerable images. Demo mode needs
		// one row of every type in the healthy state, and every other
		// repository here is a witness for something.
		{
			RepositoryName:             aws.String("acme/internal-tools"),
			RepositoryUri:              aws.String("123456789012.dkr.ecr.us-east-1.amazonaws.com/acme/internal-tools"),
			RepositoryArn:              aws.String("arn:aws:ecr:us-east-1:123456789012:repository/acme/internal-tools"),
			RegistryId:                 aws.String("123456789012"),
			ImageTagMutability:         ecrtypes.ImageTagMutabilityImmutable,
			ImageScanningConfiguration: &ecrtypes.ImageScanningConfiguration{ScanOnPush: true},
			EncryptionConfiguration: &ecrtypes.EncryptionConfiguration{
				EncryptionType: ecrtypes.EncryptionTypeKms,
				KmsKey:         aws.String(prodKMSKeyID),
			},
			CreatedAt: aws.Time(mustParseECRTime("2025-02-05T11:15:00+00:00")),
		},
	}

	images := map[string][]ecrtypes.ImageDetail{
		ECRHighVulnerabilities: {
			// ECRHighVulnerabilities witness: highs only, so the repository
			// carries the warning tier of the vulnerability signal rather
			// than the broken one acme/api-service carries.
			{
				ImageTags:        []string{"v1.4.0", "latest"},
				ImageDigest:      aws.String("sha256:5c1f0a9b7e2d"),
				ImageSizeInBytes: aws.Int64(61_000_000),
				ImagePushedAt:    aws.Time(mustParseECRTime("2026-03-19T11:00:00+00:00")),
				RegistryId:       aws.String("123456789012"),
				RepositoryName:   aws.String(ECRHighVulnerabilities),
				ImageScanFindingsSummary: &ecrtypes.ImageScanFindingsSummary{
					FindingSeverityCounts: map[string]int32{
						string(ecrtypes.FindingSeverityHigh): 3,
					},
				},
			},
		},
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
		ECRPublicPolicy:    `{"Version":"2012-10-17","Statement":[{"Sid":"AnyonePull","Effect":"Allow","Principal":"*","Action":["ecr:BatchGetImage","ecr:GetDownloadUrlForLayer"]}]}`,
		"acme/api-service": `{"Version":"2012-10-17","Statement":[{"Sid":"AllowCIPull","Effect":"Allow","Principal":{"AWS":"arn:aws:iam::123456789012:role/` + APIServiceRepoPolicyRoleName + `"},"Action":["ecr:GetDownloadUrlForLayer","ecr:BatchGetImage","ecr:BatchCheckLayerAvailability"]}]}`,
	}

	// Every repository but ECRNoLifecycle expires its untagged layers.
	const expireUntagged = `{"rules":[{"rulePriority":1,"description":"expire untagged after 14 days","selection":{"tagStatus":"untagged","countType":"sinceImagePushed","countUnit":"days","countNumber":14},"action":{"type":"expire"}}]}`
	lifecyclePolicies := map[string]string{
		"acme/api-service":     expireUntagged,
		"acme/frontend":        expireUntagged,
		"acme/base-images":     expireUntagged,
		ECRPublicPolicy:        expireUntagged,
		"acme/internal-tools":  expireUntagged,
		ECRHighVulnerabilities: expireUntagged,
	}

	return &ECRFixtures{
		Repositories:      repos,
		Images:            images,
		Tags:              tags,
		Policies:          policies,
		LifecyclePolicies: lifecyclePolicies,
	}
})

func NewECRFixtures() *ECRFixtures {
	return sharedECRFixtures()
}

func init() {
	Register(Pin{ShortName: "ecr", Rows: 7, Issues: 2, CoverageGaps: []string{"dim"}})
}
