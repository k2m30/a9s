package fixtures

import (
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	efstypes "github.com/aws/aws-sdk-go-v2/service/efs/types"
)

// EFSFixtures holds typed fixture data for EFS.
type EFSFixtures struct {
	FileSystems []efstypes.FileSystemDescription
	// MountTargets maps FileSystemId → []MountTargetDescription.
	MountTargets map[string][]efstypes.MountTargetDescription
	// AccessPoints maps FileSystemId → []AccessPointDescription.
	AccessPoints map[string][]efstypes.AccessPointDescription
}

// Exported stable IDs/ARNs for the EFS graph-root fixture.
// These are referenced by sibling fixture files so pivots resolve correctly.
const (
	// ProdEFSID is the FileSystemId of the graph-root EFS fixture.
	ProdEFSID = "fs-0prod1234abcd5678"

	// ProdEFSARN is the full ARN of the graph-root EFS filesystem.
	ProdEFSARN = "arn:aws:elasticfilesystem:us-east-1:123456789012:file-system/fs-0prod1234abcd5678"

	// ProdEFSKmsKeyID is the bare KMS key ID encrypting the graph-root EFS.
	ProdEFSKmsKeyID = "efs-prod-app-data-key"

	// ProdEFSKmsKeyARN is the full ARN of the EFS KMS key.
	ProdEFSKmsKeyARN = "arn:aws:kms:us-east-1:123456789012:key/efs-prod-app-data-key"

	// ProdEFSVpcID is the VPC hosting the graph-root EFS mount targets.
	ProdEFSVpcID = "vpc-0efs0prod0000001"

	// ProdEFSSubnetAID is the subnet in us-east-1a for MT-A.
	ProdEFSSubnetAID = "subnet-0efs0prod00000a"

	// ProdEFSSubnetBID is the subnet in us-east-1b for MT-B.
	ProdEFSSubnetBID = "subnet-0efs0prod00000b"

	// ProdEFSSubnetCID is the subnet in us-east-1c for MT-C.
	ProdEFSSubnetCID = "subnet-0efs0prod00000c"

	// ProdEFSSecurityGroupAID is the primary SG attached to EFS mount-target ENIs.
	ProdEFSSecurityGroupAID = "sg-0efs0prod000000a"

	// ProdEFSSecurityGroupBID is the secondary SG attached to EFS mount-target ENIs.
	ProdEFSSecurityGroupBID = "sg-0efs0prod000000b"

	// ProdEFSMountTargetAID is the MountTargetId in us-east-1a.
	ProdEFSMountTargetAID = "fsmt-0prod1234abcd5678a"

	// ProdEFSMountTargetBID is the MountTargetId in us-east-1b.
	ProdEFSMountTargetBID = "fsmt-0prod1234abcd5678b"

	// ProdEFSMountTargetCID is the MountTargetId in us-east-1c.
	ProdEFSMountTargetCID = "fsmt-0prod1234abcd5678c"

	// ProdEFSEniAID is the NetworkInterfaceId for MT-A.
	ProdEFSEniAID = "eni-0efs0prod00000a"

	// ProdEFSEniBID is the NetworkInterfaceId for MT-B.
	ProdEFSEniBID = "eni-0efs0prod00000b"

	// ProdEFSEniCID is the NetworkInterfaceId for MT-C.
	ProdEFSEniCID = "eni-0efs0prod00000c"

	// ProdEFSAccessPointAARN is the ARN for access-point AP-A.
	ProdEFSAccessPointAARN = "arn:aws:elasticfilesystem:us-east-1:123456789012:access-point/fsap-prod-app-a"

	// ProdEFSAccessPointBARN is the ARN for access-point AP-B.
	ProdEFSAccessPointBARN = "arn:aws:elasticfilesystem:us-east-1:123456789012:access-point/fsap-prod-app-b"

	// ProdEFSCFNStackName is the CloudFormation stack name tagging the graph-root EFS.
	ProdEFSCFNStackName = "acme-efs-app-data"

	// ProdEFSAlarmAID is the AlarmName for the first EFS CloudWatch alarm.
	ProdEFSAlarmAID = "prod-efs-burst-credit-low"

	// ProdEFSAlarmBID is the AlarmName for the second EFS CloudWatch alarm.
	ProdEFSAlarmBID = "prod-efs-percent-io-high"

	// ProdEFSLambdaAName is the name of the first Lambda function mounting this EFS.
	ProdEFSLambdaAName = "efs-data-processor"

	// ProdEFSLambdaBName is the name of the second Lambda function mounting this EFS.
	ProdEFSLambdaBName = "efs-report-generator"

	// ProdEFSBackupARecoveryARN is the ARN of the first recovery point for this EFS.
	ProdEFSBackupARecoveryARN = "arn:aws:backup:us-east-1:123456789012:recovery-point:rp-efs-prod-daily-20260416"

	// ProdEFSBackupBRecoveryARN is the ARN of the second recovery point for this EFS.
	ProdEFSBackupBRecoveryARN = "arn:aws:backup:us-east-1:123456789012:recovery-point:rp-efs-prod-daily-20260415"

	// updatingMTDownMountTargetBID is the MountTargetId for the creating MT in warn-efs-updating-mt-down.
	// Exported so QA tests can assert on it.
	UpdatedMTDownMountTargetBID = "fsmt-0warnupdmtdown001b"
)

func mustParseEFSTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}

// NewEFSFixtures constructs EFSFixtures from the canonical demo data.
func NewEFSFixtures() *EFSFixtures {
	return &EFSFixtures{
		FileSystems:  buildEFSFileSystems(),
		MountTargets: buildEFSMountTargets(),
		AccessPoints: buildEFSAccessPoints(),
	}
}

func buildEFSFileSystems() []efstypes.FileSystemDescription {
	return []efstypes.FileSystemDescription{
		// 1. prod-efs-app-data — graph-root, healthy.
		{
			FileSystemId:         aws.String(ProdEFSID),
			FileSystemArn:        aws.String(ProdEFSARN),
			Name:                 aws.String("prod-app-data"),
			LifeCycleState:       efstypes.LifeCycleStateAvailable,
			NumberOfMountTargets: 3,
			Encrypted:            aws.Bool(true),
			KmsKeyId:             aws.String(ProdEFSKmsKeyARN),
			PerformanceMode:      efstypes.PerformanceModeGeneralPurpose,
			ThroughputMode:       efstypes.ThroughputModeBursting,
			CreationTime:         aws.Time(mustParseEFSTime("2025-04-01T10:00:00+00:00")),
			CreationToken:        aws.String("prod-app-data"),
			OwnerId:              aws.String("123456789012"),
			SizeInBytes: &efstypes.FileSystemSize{
				Value:     1073741824,
				Timestamp: aws.Time(mustParseEFSTime("2026-04-23T00:00:00+00:00")),
			},
			Tags: []efstypes.Tag{
				{Key: aws.String("Name"), Value: aws.String("prod-app-data")},
				{Key: aws.String("Environment"), Value: aws.String("production")},
				{Key: aws.String("aws:cloudformation:stack-name"), Value: aws.String(ProdEFSCFNStackName)},
			},
		},

		// 2. warn-efs-creating — Wave-1 Warning: creating.
		{
			FileSystemId:         aws.String("fs-0warncreating0001"),
			FileSystemArn:        aws.String("arn:aws:elasticfilesystem:us-east-1:123456789012:file-system/fs-0warncreating0001"),
			Name:                 aws.String("provisioning-efs"),
			LifeCycleState:       efstypes.LifeCycleStateCreating,
			NumberOfMountTargets: 1,
			Encrypted:            aws.Bool(true),
			KmsKeyId:             aws.String("arn:aws:kms:us-east-1:123456789012:key/a1b2c3d4-5678-90ab-cdef-111111111111"),
			PerformanceMode:      efstypes.PerformanceModeGeneralPurpose,
			ThroughputMode:       efstypes.ThroughputModeBursting,
			CreationTime:         aws.Time(mustParseEFSTime("2026-04-23T08:00:00+00:00")),
			CreationToken:        aws.String("provisioning-efs"),
			OwnerId:              aws.String("123456789012"),
			SizeInBytes: &efstypes.FileSystemSize{Value: 0},
			Tags: []efstypes.Tag{
				{Key: aws.String("Name"), Value: aws.String("provisioning-efs")},
				{Key: aws.String("Environment"), Value: aws.String("staging")},
			},
		},

		// 3. warn-efs-updating — Wave-1 Warning: updating (MTs both available).
		{
			FileSystemId:         aws.String("fs-0warnupdating0001"),
			FileSystemArn:        aws.String("arn:aws:elasticfilesystem:us-east-1:123456789012:file-system/fs-0warnupdating0001"),
			Name:                 aws.String("updating-efs"),
			LifeCycleState:       efstypes.LifeCycleStateUpdating,
			NumberOfMountTargets: 2,
			Encrypted:            aws.Bool(true),
			KmsKeyId:             aws.String("arn:aws:kms:us-east-1:123456789012:key/a1b2c3d4-5678-90ab-cdef-111111111111"),
			PerformanceMode:      efstypes.PerformanceModeGeneralPurpose,
			ThroughputMode:       efstypes.ThroughputModeElastic,
			CreationTime:         aws.Time(mustParseEFSTime("2025-10-15T12:00:00+00:00")),
			CreationToken:        aws.String("updating-efs"),
			OwnerId:              aws.String("123456789012"),
			SizeInBytes: &efstypes.FileSystemSize{Value: 2147483648},
			Tags: []efstypes.Tag{
				{Key: aws.String("Name"), Value: aws.String("updating-efs")},
				{Key: aws.String("Environment"), Value: aws.String("prod")},
			},
		},

		// 4. warn-efs-deleting — Wave-1 Warning: deleting.
		{
			FileSystemId:         aws.String("fs-0warndeleting0001"),
			FileSystemArn:        aws.String("arn:aws:elasticfilesystem:us-east-1:123456789012:file-system/fs-0warndeleting0001"),
			Name:                 aws.String("decommission-efs"),
			LifeCycleState:       efstypes.LifeCycleStateDeleting,
			NumberOfMountTargets: 1,
			Encrypted:            aws.Bool(true),
			KmsKeyId:             aws.String("arn:aws:kms:us-east-1:123456789012:key/a1b2c3d4-5678-90ab-cdef-111111111111"),
			PerformanceMode:      efstypes.PerformanceModeGeneralPurpose,
			ThroughputMode:       efstypes.ThroughputModeBursting,
			CreationTime:         aws.Time(mustParseEFSTime("2024-06-01T08:00:00+00:00")),
			CreationToken:        aws.String("decommission-efs"),
			OwnerId:              aws.String("123456789012"),
			SizeInBytes: &efstypes.FileSystemSize{Value: 536870912},
			Tags: []efstypes.Tag{
				{Key: aws.String("Name"), Value: aws.String("decommission-efs")},
				{Key: aws.String("Environment"), Value: aws.String("prod")},
			},
		},

		// 5. broken-efs-error — Wave-1 Broken: error.
		{
			FileSystemId:         aws.String("fs-0brokenerror00001"),
			FileSystemArn:        aws.String("arn:aws:elasticfilesystem:us-east-1:123456789012:file-system/fs-0brokenerror00001"),
			Name:                 aws.String("failed-efs"),
			LifeCycleState:       efstypes.LifeCycleStateError,
			NumberOfMountTargets: 1,
			Encrypted:            aws.Bool(true),
			KmsKeyId:             aws.String("arn:aws:kms:us-east-1:123456789012:key/a1b2c3d4-5678-90ab-cdef-111111111111"),
			PerformanceMode:      efstypes.PerformanceModeGeneralPurpose,
			ThroughputMode:       efstypes.ThroughputModeBursting,
			CreationTime:         aws.Time(mustParseEFSTime("2025-12-20T15:30:00+00:00")),
			CreationToken:        aws.String("failed-efs"),
			OwnerId:              aws.String("123456789012"),
			SizeInBytes: &efstypes.FileSystemSize{Value: 1073741824},
			Tags: []efstypes.Tag{
				{Key: aws.String("Name"), Value: aws.String("failed-efs")},
				{Key: aws.String("Environment"), Value: aws.String("prod")},
			},
		},

		// 6. broken-efs-no-mount-targets — Wave-1 Broken: no mount targets.
		{
			FileSystemId:         aws.String("fs-0brokennomt000001"),
			FileSystemArn:        aws.String("arn:aws:elasticfilesystem:us-east-1:123456789012:file-system/fs-0brokennomt000001"),
			Name:                 aws.String("orphan-efs"),
			LifeCycleState:       efstypes.LifeCycleStateAvailable,
			NumberOfMountTargets: 0,
			Encrypted:            aws.Bool(true),
			KmsKeyId:             aws.String("arn:aws:kms:us-east-1:123456789012:key/a1b2c3d4-5678-90ab-cdef-111111111111"),
			PerformanceMode:      efstypes.PerformanceModeGeneralPurpose,
			ThroughputMode:       efstypes.ThroughputModeBursting,
			CreationTime:         aws.Time(mustParseEFSTime("2025-02-10T08:00:00+00:00")),
			CreationToken:        aws.String("orphan-efs"),
			OwnerId:              aws.String("123456789012"),
			SizeInBytes: &efstypes.FileSystemSize{Value: 1073741824},
			Tags: []efstypes.Tag{
				{Key: aws.String("Name"), Value: aws.String("orphan-efs")},
				{Key: aws.String("Environment"), Value: aws.String("prod")},
			},
		},

		// 7. warn-efs-multi — U7a: deleting + no mount targets (multi W1, Broken wins).
		{
			FileSystemId:         aws.String("fs-0warnmulti0000001"),
			FileSystemArn:        aws.String("arn:aws:elasticfilesystem:us-east-1:123456789012:file-system/fs-0warnmulti0000001"),
			Name:                 aws.String("deleting-nomount-efs"),
			LifeCycleState:       efstypes.LifeCycleStateDeleting,
			NumberOfMountTargets: 0,
			Encrypted:            aws.Bool(true),
			KmsKeyId:             aws.String("arn:aws:kms:us-east-1:123456789012:key/a1b2c3d4-5678-90ab-cdef-111111111111"),
			PerformanceMode:      efstypes.PerformanceModeGeneralPurpose,
			ThroughputMode:       efstypes.ThroughputModeBursting,
			CreationTime:         aws.Time(mustParseEFSTime("2024-03-15T10:00:00+00:00")),
			CreationToken:        aws.String("deleting-nomount-efs"),
			OwnerId:              aws.String("123456789012"),
			SizeInBytes: &efstypes.FileSystemSize{Value: 268435456},
			Tags: []efstypes.Tag{
				{Key: aws.String("Name"), Value: aws.String("deleting-nomount-efs")},
				{Key: aws.String("Environment"), Value: aws.String("prod")},
			},
		},

		// 8. warn-efs-updating-mt-down — U7b/U7c/U7e: W1 updating + W2 mount-target down.
		{
			FileSystemId:         aws.String("fs-0warnupdmtdown001"),
			FileSystemArn:        aws.String("arn:aws:elasticfilesystem:us-east-1:123456789012:file-system/fs-0warnupdmtdown001"),
			Name:                 aws.String("updating-mt-down-efs"),
			LifeCycleState:       efstypes.LifeCycleStateUpdating,
			NumberOfMountTargets: 2,
			Encrypted:            aws.Bool(true),
			KmsKeyId:             aws.String("arn:aws:kms:us-east-1:123456789012:key/a1b2c3d4-5678-90ab-cdef-111111111111"),
			PerformanceMode:      efstypes.PerformanceModeGeneralPurpose,
			ThroughputMode:       efstypes.ThroughputModeBursting,
			CreationTime:         aws.Time(mustParseEFSTime("2025-07-20T09:00:00+00:00")),
			CreationToken:        aws.String("updating-mt-down-efs"),
			OwnerId:              aws.String("123456789012"),
			SizeInBytes: &efstypes.FileSystemSize{Value: 2147483648},
			Tags: []efstypes.Tag{
				{Key: aws.String("Name"), Value: aws.String("updating-mt-down-efs")},
				{Key: aws.String("Environment"), Value: aws.String("prod")},
			},
		},

		// 9. healthy-efs-with-mt-down — W2 on Healthy: available FS but MT-B creating.
		{
			FileSystemId:         aws.String("fs-0healthymtdown001"),
			FileSystemArn:        aws.String("arn:aws:elasticfilesystem:us-east-1:123456789012:file-system/fs-0healthymtdown001"),
			Name:                 aws.String("healthy-mt-down-efs"),
			LifeCycleState:       efstypes.LifeCycleStateAvailable,
			NumberOfMountTargets: 2,
			Encrypted:            aws.Bool(true),
			KmsKeyId:             aws.String("arn:aws:kms:us-east-1:123456789012:key/a1b2c3d4-5678-90ab-cdef-111111111111"),
			PerformanceMode:      efstypes.PerformanceModeGeneralPurpose,
			ThroughputMode:       efstypes.ThroughputModeBursting,
			CreationTime:         aws.Time(mustParseEFSTime("2025-09-10T11:00:00+00:00")),
			CreationToken:        aws.String("healthy-mt-down-efs"),
			OwnerId:              aws.String("123456789012"),
			SizeInBytes: &efstypes.FileSystemSize{Value: 4294967296},
			Tags: []efstypes.Tag{
				{Key: aws.String("Name"), Value: aws.String("healthy-mt-down-efs")},
				{Key: aws.String("Environment"), Value: aws.String("prod")},
			},
		},
	}
}

func buildEFSMountTargets() map[string][]efstypes.MountTargetDescription {
	return map[string][]efstypes.MountTargetDescription{
		// 1. prod-efs-app-data — 3 MTs, all available.
		ProdEFSID: {
			{
				MountTargetId:      aws.String(ProdEFSMountTargetAID),
				FileSystemId:       aws.String(ProdEFSID),
				LifeCycleState:     efstypes.LifeCycleStateAvailable,
				SubnetId:           aws.String(ProdEFSSubnetAID),
				VpcId:              aws.String(ProdEFSVpcID),
				NetworkInterfaceId: aws.String(ProdEFSEniAID),
				AvailabilityZoneName: aws.String("us-east-1a"),
				OwnerId:            aws.String("123456789012"),
				IpAddress:          aws.String("10.20.1.10"),
			},
			{
				MountTargetId:      aws.String(ProdEFSMountTargetBID),
				FileSystemId:       aws.String(ProdEFSID),
				LifeCycleState:     efstypes.LifeCycleStateAvailable,
				SubnetId:           aws.String(ProdEFSSubnetBID),
				VpcId:              aws.String(ProdEFSVpcID),
				NetworkInterfaceId: aws.String(ProdEFSEniBID),
				AvailabilityZoneName: aws.String("us-east-1b"),
				OwnerId:            aws.String("123456789012"),
				IpAddress:          aws.String("10.20.2.10"),
			},
			{
				MountTargetId:      aws.String(ProdEFSMountTargetCID),
				FileSystemId:       aws.String(ProdEFSID),
				LifeCycleState:     efstypes.LifeCycleStateAvailable,
				SubnetId:           aws.String(ProdEFSSubnetCID),
				VpcId:              aws.String(ProdEFSVpcID),
				NetworkInterfaceId: aws.String(ProdEFSEniCID),
				AvailabilityZoneName: aws.String("us-east-1c"),
				OwnerId:            aws.String("123456789012"),
				IpAddress:          aws.String("10.20.3.10"),
			},
		},

		// 2. warn-efs-creating — 1 MT in creating state.
		"fs-0warncreating0001": {
			{
				MountTargetId:        aws.String("fsmt-0warncreating001a"),
				FileSystemId:         aws.String("fs-0warncreating0001"),
				LifeCycleState:       efstypes.LifeCycleStateCreating,
				SubnetId:             aws.String("subnet-0aaa111111111111a"),
				VpcId:                aws.String("vpc-0abc123def456789a"),
				AvailabilityZoneName: aws.String("us-east-1a"),
				OwnerId:              aws.String("123456789012"),
			},
		},

		// 3. warn-efs-updating — 2 MTs both available.
		"fs-0warnupdating0001": {
			{
				MountTargetId:        aws.String("fsmt-0warnupdating001a"),
				FileSystemId:         aws.String("fs-0warnupdating0001"),
				LifeCycleState:       efstypes.LifeCycleStateAvailable,
				SubnetId:             aws.String("subnet-0aaa111111111111a"),
				VpcId:                aws.String("vpc-0abc123def456789a"),
				AvailabilityZoneName: aws.String("us-east-1a"),
				OwnerId:              aws.String("123456789012"),
				IpAddress:            aws.String("10.0.1.50"),
			},
			{
				MountTargetId:        aws.String("fsmt-0warnupdating001b"),
				FileSystemId:         aws.String("fs-0warnupdating0001"),
				LifeCycleState:       efstypes.LifeCycleStateAvailable,
				SubnetId:             aws.String("subnet-0bbb222222222222b"),
				VpcId:                aws.String("vpc-0abc123def456789a"),
				AvailabilityZoneName: aws.String("us-east-1b"),
				OwnerId:              aws.String("123456789012"),
				IpAddress:            aws.String("10.0.2.50"),
			},
		},

		// 4. warn-efs-deleting — 1 MT in deleting state.
		"fs-0warndeleting0001": {
			{
				MountTargetId:        aws.String("fsmt-0warndeleting001a"),
				FileSystemId:         aws.String("fs-0warndeleting0001"),
				LifeCycleState:       efstypes.LifeCycleStateDeleting,
				SubnetId:             aws.String("subnet-0aaa111111111111a"),
				VpcId:                aws.String("vpc-0abc123def456789a"),
				AvailabilityZoneName: aws.String("us-east-1a"),
				OwnerId:              aws.String("123456789012"),
			},
		},

		// 5. broken-efs-error — 1 MT in error state.
		"fs-0brokenerror00001": {
			{
				MountTargetId:        aws.String("fsmt-0brokenerror001a"),
				FileSystemId:         aws.String("fs-0brokenerror00001"),
				LifeCycleState:       efstypes.LifeCycleStateError,
				SubnetId:             aws.String("subnet-0aaa111111111111a"),
				VpcId:                aws.String("vpc-0abc123def456789a"),
				AvailabilityZoneName: aws.String("us-east-1a"),
				OwnerId:              aws.String("123456789012"),
			},
		},

		// 6. broken-efs-no-mount-targets — empty list.
		"fs-0brokennomt000001": {},

		// 7. warn-efs-multi — empty list.
		"fs-0warnmulti0000001": {},

		// 8. warn-efs-updating-mt-down — MT-A available, MT-B creating.
		"fs-0warnupdmtdown001": {
			{
				MountTargetId:        aws.String("fsmt-0warnupdmtdown001a"),
				FileSystemId:         aws.String("fs-0warnupdmtdown001"),
				LifeCycleState:       efstypes.LifeCycleStateAvailable,
				SubnetId:             aws.String("subnet-0aaa111111111111a"),
				VpcId:                aws.String("vpc-0abc123def456789a"),
				AvailabilityZoneName: aws.String("us-east-1a"),
				OwnerId:              aws.String("123456789012"),
				IpAddress:            aws.String("10.0.1.60"),
			},
			{
				MountTargetId:        aws.String(UpdatedMTDownMountTargetBID),
				FileSystemId:         aws.String("fs-0warnupdmtdown001"),
				LifeCycleState:       efstypes.LifeCycleStateCreating,
				SubnetId:             aws.String("subnet-0bbb222222222222b"),
				VpcId:                aws.String("vpc-0abc123def456789a"),
				AvailabilityZoneName: aws.String("us-east-1b"),
				OwnerId:              aws.String("123456789012"),
			},
		},

		// 9. healthy-efs-with-mt-down — MT-A available, MT-B creating.
		"fs-0healthymtdown001": {
			{
				MountTargetId:        aws.String("fsmt-0healthymtdown001a"),
				FileSystemId:         aws.String("fs-0healthymtdown001"),
				LifeCycleState:       efstypes.LifeCycleStateAvailable,
				SubnetId:             aws.String("subnet-0aaa111111111111a"),
				VpcId:                aws.String("vpc-0abc123def456789a"),
				AvailabilityZoneName: aws.String("us-east-1a"),
				OwnerId:              aws.String("123456789012"),
				IpAddress:            aws.String("10.0.1.70"),
			},
			{
				MountTargetId:        aws.String("fsmt-0healthymtdown001b"),
				FileSystemId:         aws.String("fs-0healthymtdown001"),
				LifeCycleState:       efstypes.LifeCycleStateCreating,
				SubnetId:             aws.String("subnet-0bbb222222222222b"),
				VpcId:                aws.String("vpc-0abc123def456789a"),
				AvailabilityZoneName: aws.String("us-east-1b"),
				OwnerId:              aws.String("123456789012"),
			},
		},
	}
}

func buildEFSAccessPoints() map[string][]efstypes.AccessPointDescription {
	return map[string][]efstypes.AccessPointDescription{
		// prod-efs-app-data has two access points, referenced by Lambda FileSystemConfigs.
		ProdEFSID: {
			{
				AccessPointId:  aws.String("fsap-prod-app-a"),
				AccessPointArn: aws.String(ProdEFSAccessPointAARN),
				FileSystemId:   aws.String(ProdEFSID),
				LifeCycleState: efstypes.LifeCycleStateAvailable,
				Name:           aws.String("prod-app-data-ap-a"),
				OwnerId:        aws.String("123456789012"),
				Tags: []efstypes.Tag{
					{Key: aws.String("Name"), Value: aws.String("prod-app-data-ap-a")},
				},
			},
			{
				AccessPointId:  aws.String("fsap-prod-app-b"),
				AccessPointArn: aws.String(ProdEFSAccessPointBARN),
				FileSystemId:   aws.String(ProdEFSID),
				LifeCycleState: efstypes.LifeCycleStateAvailable,
				Name:           aws.String("prod-app-data-ap-b"),
				OwnerId:        aws.String("123456789012"),
				Tags: []efstypes.Tag{
					{Key: aws.String("Name"), Value: aws.String("prod-app-data-ap-b")},
				},
			},
		},
	}
}
