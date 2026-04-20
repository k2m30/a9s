package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/efs"
)

// EFSDescribeFileSystemsAPI defines the interface for the EFS DescribeFileSystems operation.
type EFSDescribeFileSystemsAPI interface {
	DescribeFileSystems(ctx context.Context, params *efs.DescribeFileSystemsInput, optFns ...func(*efs.Options)) (*efs.DescribeFileSystemsOutput, error)
}

// EFSDescribeMountTargetsAPI defines the interface for the EFS DescribeMountTargets operation.
type EFSDescribeMountTargetsAPI interface {
	DescribeMountTargets(ctx context.Context, params *efs.DescribeMountTargetsInput, optFns ...func(*efs.Options)) (*efs.DescribeMountTargetsOutput, error)
}

// EFSDescribeAccessPointsAPI defines the interface for the EFS
// DescribeAccessPoints operation.
type EFSDescribeAccessPointsAPI interface {
	DescribeAccessPoints(ctx context.Context, params *efs.DescribeAccessPointsInput, optFns ...func(*efs.Options)) (*efs.DescribeAccessPointsOutput, error)
}

// EFSAPI is the aggregate interface covering all EFS operations used by a9s fetchers.
// *efs.Client structurally satisfies this interface.
type EFSAPI interface {
	EFSDescribeFileSystemsAPI
	EFSDescribeMountTargetsAPI
	EFSDescribeAccessPointsAPI
}
