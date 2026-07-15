// Package fixtures provides EC2 Launch Template fixture data for the EC2 fake.
// Launch templates ride the EC2 service client (DescribeLaunchTemplates,
// DescribeLaunchTemplateVersions) — no dedicated client field, see
// internal/demo/client.go and internal/demo/fakes/ec2.go.
package fixtures

import (
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// LTFixtures holds all EC2 Launch Template domain objects served by the fake.
type LTFixtures struct {
	// LaunchTemplates is the full list returned by DescribeLaunchTemplates.
	// Carries identity/version-number/creator facts only — no LaunchTemplateData
	// (mirrors the real API: DescribeLaunchTemplates never returns template data).
	LaunchTemplates []ec2types.LaunchTemplate
	// DefaultVersions maps LaunchTemplateId to its "$Default" version — the
	// single extra per-template call that funds every related-panel pivot and
	// every Wave 2 signal (docs/resources/lt.md §1, §2, §3.2). $Default (not
	// $Latest) is what asg/ng/ec2 actually resolve at launch.
	DefaultVersions map[string]ec2types.LaunchTemplateVersion
	// DeniedIDs lists LaunchTemplateIds whose DescribeLaunchTemplateVersions
	// call the fake denies with AccessDeniedException. The row is kept with
	// its list fields (rich degradation) — see WarnLTDeniedID.
	DeniedIDs []string
}

// Exported stable IDs — sibling fixture files (asg.go, ec2.go, eks.go) and the
// EC2 fake reference launch templates by these symbols, not by string literal.
const (
	// ProdWebLTID is the graph-root fixture: IMDSv2 required, encrypted EBS,
	// current AMI, two security groups. Referenced by two ASG fixtures
	// (asg.go) and tagged onto two EC2 instances (ec2.go).
	ProdWebLTID = "lt-0prodweb1111111a"
	// EKSNodeLTID matches the pre-existing literal already wired into
	// eks.go's "general-pool" nodegroup (Nodegroup.LaunchTemplate.Id) and the
	// EC2 fake's ImageId resolution for that nodegroup — reused verbatim so
	// TestFetchNodeGroups_ResolvesImageIDFromCustomLaunchTemplate-style image
	// resolution keeps working unchanged.
	EKSNodeLTID = "lt-0eks111111111111a"
	// SSMAmiLTID witnesses the no-pivot/no-finding ssm skip: ImageId is a
	// resolve:ssm: reference, not an ami- id.
	SSMAmiLTID = "lt-0ssmami11111111a"
	// WarnLTIMDSv1ID sets HttpTokens=optional explicitly.
	WarnLTIMDSv1ID = "lt-0warnimdsv11111a"
	// WarnLTIMDSv1DefaultID leaves MetadataOptions nil entirely — the
	// unset-defaults-to-optional trap witness (docs/resources/lt.md §3.2).
	WarnLTIMDSv1DefaultID = "lt-0warnimdsvdef111a"
	// WarnLTUnencryptedID sets one BlockDeviceMappings[].Ebs.Encrypted=false explicitly.
	WarnLTUnencryptedID = "lt-0warnunencrypt1a"
	// WarnLTMultiID stacks IMDSv1 + unencrypted on one template.
	WarnLTMultiID = "lt-0warnmulti111111a"
	// WarnLTDeniedID is listed but its DescribeLaunchTemplateVersions call is denied.
	WarnLTDeniedID = "lt-0warndenied11111a"
	// WarnLTDeprecatedAMIID references the existing deprecated ami fixture
	// (ec2.go's ami-0deprecated0ubuntu1, DeprecationTime in the past).
	WarnLTDeprecatedAMIID = "lt-0warndeprecated1a"
)

// ltPrimaryKMSKeyID mirrors the same bare key ID reused across every other
// fixture file in this package (see kms.go's primary CMK, KeyId
// "a1b2c3d4-5678-90ab-cdef-111111111111") — house convention is a local
// unexported const per file rather than one shared export.
const ltPrimaryKMSKeyID = "a1b2c3d4-5678-90ab-cdef-111111111111"

const ltCreatedBy = "arn:aws:iam::123456789012:user/acme-platform-admin"

// NewLTFixtures builds and returns a fully-populated LTFixtures struct.
var sharedLTFixtures = sync.OnceValue(func() *LTFixtures {
	return &LTFixtures{
		LaunchTemplates: buildLaunchTemplates(),
		DefaultVersions: buildLTDefaultVersions(),
		DeniedIDs:       []string{WarnLTDeniedID},
	}
})

func NewLTFixtures() *LTFixtures {
	return sharedLTFixtures()
}

func ltHealthyMetadataOptions() *ec2types.LaunchTemplateInstanceMetadataOptions {
	return &ec2types.LaunchTemplateInstanceMetadataOptions{
		HttpEndpoint:            ec2types.LaunchTemplateInstanceMetadataEndpointStateEnabled,
		HttpTokens:              ec2types.LaunchTemplateHttpTokensStateRequired,
		HttpPutResponseHopLimit: aws.Int32(2),
	}
}

func ltEncryptedRootVolume(kmsKeyID string) []ec2types.LaunchTemplateBlockDeviceMapping {
	return []ec2types.LaunchTemplateBlockDeviceMapping{
		{
			DeviceName: aws.String("/dev/xvda"),
			Ebs: &ec2types.LaunchTemplateEbsBlockDevice{
				DeleteOnTermination: aws.Bool(true),
				Encrypted:           aws.Bool(true),
				KmsKeyId:            aws.String(kmsKeyID),
				VolumeSize:          aws.Int32(50),
				VolumeType:          ec2types.VolumeTypeGp3,
			},
		},
	}
}

// ltUnencryptedRootVolume mirrors ltEncryptedRootVolume with
// Encrypted=false explicit and no KmsKeyId — the ltCodeUnencrypted witness.
func ltUnencryptedRootVolume() []ec2types.LaunchTemplateBlockDeviceMapping {
	return []ec2types.LaunchTemplateBlockDeviceMapping{
		{
			DeviceName: aws.String("/dev/xvda"),
			Ebs: &ec2types.LaunchTemplateEbsBlockDevice{
				DeleteOnTermination: aws.Bool(true),
				Encrypted:           aws.Bool(false),
				VolumeSize:          aws.Int32(30),
				VolumeType:          ec2types.VolumeTypeGp3,
			},
		},
	}
}

// buildLaunchTemplates returns the DescribeLaunchTemplates list shape: identity
// plus DefaultVersionNumber/LatestVersionNumber/CreatedBy/CreateTime/Tags only
// — no LaunchTemplateData (docs/resources/lt.md §6 citation on list-shape).
func buildLaunchTemplates() []ec2types.LaunchTemplate {
	entry := func(id, name string, defaultVer, latestVer int64, created string, env string) ec2types.LaunchTemplate {
		return ec2types.LaunchTemplate{
			LaunchTemplateId:     aws.String(id),
			LaunchTemplateName:   aws.String(name),
			DefaultVersionNumber: aws.Int64(defaultVer),
			LatestVersionNumber:  aws.Int64(latestVer),
			CreatedBy:            aws.String(ltCreatedBy),
			CreateTime:           aws.Time(mustTime(created)),
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String(name)},
				{Key: aws.String("Environment"), Value: aws.String(env)},
			},
		}
	}
	return []ec2types.LaunchTemplate{
		entry(ProdWebLTID, "prod-web-lt", 4, 4, "2025-02-01T09:00:00Z", "prod"),
		entry(EKSNodeLTID, "eks-node-lt", 1, 1, "2025-03-05T12:00:00Z", "prod"),
		entry(SSMAmiLTID, "ssm-ami-lt", 1, 1, "2025-04-10T08:00:00Z", "prod"),
		entry(WarnLTIMDSv1ID, "warn-lt-imdsv1", 1, 1, "2025-05-01T08:00:00Z", "staging"),
		entry(WarnLTIMDSv1DefaultID, "warn-lt-imdsv1-def", 1, 1, "2025-05-02T08:00:00Z", "staging"),
		entry(WarnLTUnencryptedID, "warn-lt-unencrypted", 1, 1, "2025-05-03T08:00:00Z", "staging"),
		entry(WarnLTMultiID, "warn-lt-multi", 1, 1, "2025-05-04T08:00:00Z", "staging"),
		entry(WarnLTDeniedID, "warn-lt-denied", 2, 2, "2025-05-05T08:00:00Z", "staging"),
		entry(WarnLTDeprecatedAMIID, "warn-lt-deprecated-ami", 1, 1, "2025-05-06T08:00:00Z", "staging"),
	}
}

// buildLTDefaultVersions returns the "$Default" DescribeLaunchTemplateVersions
// response for every launch template except WarnLTDeniedID, whose call the
// fake denies (see LTFixtures.DeniedIDs / EC2Fake.DescribeLaunchTemplateVersions).
func buildLTDefaultVersions() map[string]ec2types.LaunchTemplateVersion {
	version := func(id string, verNum int64, data *ec2types.ResponseLaunchTemplateData, created string) ec2types.LaunchTemplateVersion {
		return ec2types.LaunchTemplateVersion{
			LaunchTemplateId:   aws.String(id),
			VersionNumber:      aws.Int64(verNum),
			DefaultVersion:     aws.Bool(true),
			CreatedBy:          aws.String(ltCreatedBy),
			CreateTime:         aws.Time(mustTime(created)),
			LaunchTemplateData: data,
		}
	}

	return map[string]ec2types.LaunchTemplateVersion{
		// prod-web-lt (GRAPH ROOT): healthy — IMDSv2 required, encrypted EBS
		// with a real KMS key, current AMI, two real security groups.
		ProdWebLTID: version(ProdWebLTID, 4, &ec2types.ResponseLaunchTemplateData{
			ImageId:             aws.String(fixtProdAMIID1),
			InstanceType:        ec2types.InstanceTypeM5Large,
			SecurityGroupIds:    []string{fixtProdWebALBSGID, fixtProdAPIInternalSGID},
			BlockDeviceMappings: ltEncryptedRootVolume(ltPrimaryKMSKeyID),
			MetadataOptions:     ltHealthyMetadataOptions(),
		}, "2026-06-01T10:00:00Z"),

		// eks-node-lt: NetworkInterfaces[] (Groups ∪ SubnetId) instead of the
		// top-level SecurityGroupIds/subnet fields — mutually exclusive by API
		// design, and this fixture is the NI-union witness. ImageId matches the
		// pre-existing EC2 fake response for this exact LT id.
		EKSNodeLTID: version(EKSNodeLTID, 1, &ec2types.ResponseLaunchTemplateData{
			ImageId:      aws.String("ami-0eks111111111111a"),
			InstanceType: ec2types.InstanceTypeM5Large,
			NetworkInterfaces: []ec2types.LaunchTemplateInstanceNetworkInterfaceSpecification{
				{
					DeviceIndex: aws.Int32(0),
					Groups:      []string{fixtProdAPIInternalSGID},
					SubnetId:    aws.String(fixtProdPublicSubnetA),
				},
			},
			MetadataOptions: ltHealthyMetadataOptions(),
		}, "2025-03-05T12:00:00Z"),

		// ssm-ami-lt: healthy; ImageId is a resolve:ssm: reference, a display
		// fact only — never a pivot (docs/resources/lt.md §2 ami bullet).
		SSMAmiLTID: version(SSMAmiLTID, 1, &ec2types.ResponseLaunchTemplateData{
			ImageId:             aws.String("resolve:ssm:/aws/service/ami-amazon-linux-latest/al2023-ami-kernel-default-x86_64"),
			InstanceType:        ec2types.InstanceTypeT3Medium,
			BlockDeviceMappings: ltEncryptedRootVolume(ltPrimaryKMSKeyID),
			MetadataOptions:     ltHealthyMetadataOptions(),
		}, "2025-04-10T08:00:00Z"),

		// warn-lt-imdsv1: HttpTokens=optional explicit.
		WarnLTIMDSv1ID: version(WarnLTIMDSv1ID, 1, &ec2types.ResponseLaunchTemplateData{
			ImageId:             aws.String(fixtProdAMIID1),
			InstanceType:        ec2types.InstanceTypeT3Medium,
			BlockDeviceMappings: ltEncryptedRootVolume(ltPrimaryKMSKeyID),
			MetadataOptions: &ec2types.LaunchTemplateInstanceMetadataOptions{
				HttpEndpoint:            ec2types.LaunchTemplateInstanceMetadataEndpointStateEnabled,
				HttpTokens:              ec2types.LaunchTemplateHttpTokensStateOptional,
				HttpPutResponseHopLimit: aws.Int32(1),
			},
		}, "2025-05-01T08:00:00Z"),

		// warn-lt-imdsv1-def: MetadataOptions entirely nil — unset defaults to
		// optional (SDK-confirmed); the same finding fires on absence alone.
		WarnLTIMDSv1DefaultID: version(WarnLTIMDSv1DefaultID, 1, &ec2types.ResponseLaunchTemplateData{
			ImageId:             aws.String(fixtProdAMIID1),
			InstanceType:        ec2types.InstanceTypeT3Medium,
			BlockDeviceMappings: ltEncryptedRootVolume(ltPrimaryKMSKeyID),
			MetadataOptions:     nil,
		}, "2025-05-02T08:00:00Z"),

		// warn-lt-unencrypted: one BlockDeviceMappings[].Ebs.Encrypted=false explicit.
		WarnLTUnencryptedID: version(WarnLTUnencryptedID, 1, &ec2types.ResponseLaunchTemplateData{
			ImageId:             aws.String(fixtProdAMIID1),
			InstanceType:        ec2types.InstanceTypeT3Medium,
			BlockDeviceMappings: ltUnencryptedRootVolume(),
			MetadataOptions:     ltHealthyMetadataOptions(),
		}, "2025-05-03T08:00:00Z"),

		// warn-lt-multi: IMDSv1 allowed + unencrypted volume stack on one
		// template → "IMDSv1 allowed (+1)" per §4 precedence.
		WarnLTMultiID: version(WarnLTMultiID, 1, &ec2types.ResponseLaunchTemplateData{
			ImageId:             aws.String(fixtProdAMIID1),
			InstanceType:        ec2types.InstanceTypeT3Medium,
			BlockDeviceMappings: ltUnencryptedRootVolume(),
			MetadataOptions: &ec2types.LaunchTemplateInstanceMetadataOptions{
				HttpEndpoint:            ec2types.LaunchTemplateInstanceMetadataEndpointStateEnabled,
				HttpTokens:              ec2types.LaunchTemplateHttpTokensStateOptional,
				HttpPutResponseHopLimit: aws.Int32(1),
			},
		}, "2025-05-04T08:00:00Z"),

		// warn-lt-deprecated-ami: healthy on IMDS + encryption axes, isolating
		// the deprecated-ami signal alone. ImageId matches ec2.go's
		// ami-0deprecated0ubuntu1 (DeprecationTime in the past).
		WarnLTDeprecatedAMIID: version(WarnLTDeprecatedAMIID, 1, &ec2types.ResponseLaunchTemplateData{
			ImageId:             aws.String("ami-0deprecated0ubuntu1"),
			InstanceType:        ec2types.InstanceTypeT3Medium,
			BlockDeviceMappings: ltEncryptedRootVolume(ltPrimaryKMSKeyID),
			MetadataOptions:     ltHealthyMetadataOptions(),
		}, "2025-05-06T08:00:00Z"),

		// WarnLTDeniedID intentionally absent — DescribeLaunchTemplateVersions
		// is denied for this id (see LTFixtures.DeniedIDs).
	}
}
