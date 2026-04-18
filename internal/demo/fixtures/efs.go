package fixtures

import (
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	efstypes "github.com/aws/aws-sdk-go-v2/service/efs/types"
)

// EFSFixtures holds typed fixture data for EFS.
type EFSFixtures struct {
	FileSystems []efstypes.FileSystemDescription
}

func mustParseEFSTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}

// NewEFSFixtures constructs EFSFixtures from the canonical demo data.
func NewEFSFixtures() *EFSFixtures {
	return &EFSFixtures{
		FileSystems: []efstypes.FileSystemDescription{
			{
				FileSystemId:         aws.String("fs-0abc111111111111a"),
				FileSystemArn:        aws.String("arn:aws:elasticfilesystem:us-east-1:123456789012:file-system/fs-0abc111111111111a"),
				Name:                 aws.String("acme-shared-data"),
				LifeCycleState:       efstypes.LifeCycleStateAvailable,
				PerformanceMode:      efstypes.PerformanceModeGeneralPurpose,
				ThroughputMode:       efstypes.ThroughputModeElastic,
				Encrypted:            aws.Bool(true),
				KmsKeyId:             aws.String("a1b2c3d4-5678-90ab-cdef-111111111111"),
				NumberOfMountTargets: 3,
				CreationTime:         aws.Time(mustParseEFSTime("2025-04-01T10:00:00+00:00")),
				CreationToken:        aws.String("acme-shared-data"),
				OwnerId:              aws.String("123456789012"),
				SizeInBytes: &efstypes.FileSystemSize{
					Value: 10737418240,
				},
				Tags: []efstypes.Tag{
					{Key: aws.String("Name"), Value: aws.String("acme-shared-data")},
					{Key: aws.String("Environment"), Value: aws.String("prod")},
				},
			},
			{
				FileSystemId:         aws.String("fs-0def222222222222b"),
				FileSystemArn:        aws.String("arn:aws:elasticfilesystem:us-east-1:123456789012:file-system/fs-0def222222222222b"),
				Name:                 aws.String("ml-training-storage"),
				LifeCycleState:       efstypes.LifeCycleStateAvailable,
				PerformanceMode:      efstypes.PerformanceModeMaxIo,
				ThroughputMode:       efstypes.ThroughputModeBursting,
				Encrypted:            aws.Bool(true),
				KmsKeyId:             aws.String("a1b2c3d4-5678-90ab-cdef-111111111111"),
				NumberOfMountTargets: 2,
				CreationTime:         aws.Time(mustParseEFSTime("2025-08-15T14:30:00+00:00")),
				CreationToken:        aws.String("ml-training-storage"),
				OwnerId:              aws.String("123456789012"),
				SizeInBytes: &efstypes.FileSystemSize{
					Value: 53687091200,
				},
				Tags: []efstypes.Tag{
					{Key: aws.String("Name"), Value: aws.String("ml-training-storage")},
					{Key: aws.String("Environment"), Value: aws.String("prod")},
				},
			},
			{
				FileSystemId:         aws.String("fs-0ghi333333333333c"),
				FileSystemArn:        aws.String("arn:aws:elasticfilesystem:us-east-1:123456789012:file-system/fs-0ghi333333333333c"),
				Name:                 aws.String("staging-efs"),
				LifeCycleState:       efstypes.LifeCycleStateCreating,
				PerformanceMode:      efstypes.PerformanceModeGeneralPurpose,
				ThroughputMode:       efstypes.ThroughputModeBursting,
				Encrypted:            aws.Bool(true),
				KmsKeyId:             aws.String("a1b2c3d4-5678-90ab-cdef-111111111111"),
				NumberOfMountTargets: 0,
				CreationTime:         aws.Time(mustParseEFSTime("2026-03-21T09:00:00+00:00")),
				CreationToken:        aws.String("staging-efs"),
				OwnerId:              aws.String("123456789012"),
				SizeInBytes: &efstypes.FileSystemSize{
					Value: 0,
				},
				Tags: []efstypes.Tag{
					{Key: aws.String("Name"), Value: aws.String("staging-efs")},
					{Key: aws.String("Environment"), Value: aws.String("staging")},
				},
			},
			// Issue: LifeCycleState=available, NumberOfMountTargets=0 → Broken (orphaned file system)
			{
				FileSystemId:         aws.String("fs-0jkl444444444444d"),
				FileSystemArn:        aws.String("arn:aws:elasticfilesystem:us-east-1:123456789012:file-system/fs-0jkl444444444444d"),
				Name:                 aws.String("efs-orphan"),
				LifeCycleState:       efstypes.LifeCycleStateAvailable,
				PerformanceMode:      efstypes.PerformanceModeGeneralPurpose,
				ThroughputMode:       efstypes.ThroughputModeBursting,
				Encrypted:            aws.Bool(true),
				KmsKeyId:             aws.String("a1b2c3d4-5678-90ab-cdef-111111111111"),
				NumberOfMountTargets: 0,
				CreationTime:         aws.Time(mustParseEFSTime("2025-02-10T08:00:00+00:00")),
				CreationToken:        aws.String("efs-orphan"),
				OwnerId:              aws.String("123456789012"),
				SizeInBytes: &efstypes.FileSystemSize{
					Value: 1073741824,
				},
				Tags: []efstypes.Tag{
					{Key: aws.String("Name"), Value: aws.String("efs-orphan")},
					{Key: aws.String("Environment"), Value: aws.String("prod")},
				},
			},
		},
	}
}
