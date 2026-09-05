// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

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

// EFSDescribeFileSystemPolicyAPI defines the interface for the EFS
// DescribeFileSystemPolicy operation. Used by EnrichEFSMountTargets to read
// the file system's resource policy.
type EFSDescribeFileSystemPolicyAPI interface {
	DescribeFileSystemPolicy(ctx context.Context, params *efs.DescribeFileSystemPolicyInput, optFns ...func(*efs.Options)) (*efs.DescribeFileSystemPolicyOutput, error)
}

// EFSDescribeBackupPolicyAPI defines the interface for the EFS
// DescribeBackupPolicy operation. Used by EnrichEFSMountTargets.
type EFSDescribeBackupPolicyAPI interface {
	DescribeBackupPolicy(ctx context.Context, params *efs.DescribeBackupPolicyInput, optFns ...func(*efs.Options)) (*efs.DescribeBackupPolicyOutput, error)
}

// EFSAPI is the aggregate interface covering all EFS operations used by a9s fetchers.
// *efs.Client structurally satisfies this interface.
type EFSAPI interface {
	EFSDescribeFileSystemsAPI
	EFSDescribeMountTargetsAPI
	EFSDescribeAccessPointsAPI
	// Wave 2 enrichment (EnrichEFSMountTargets).
	EFSDescribeFileSystemPolicyAPI
	EFSDescribeBackupPolicyAPI
}
