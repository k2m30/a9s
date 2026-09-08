// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package fakes

import (
	"context"
	"slices"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/efs"
	efstypes "github.com/aws/aws-sdk-go-v2/service/efs/types"

	"github.com/k2m30/a9s/v3/core/demo/fixtures"
)

// EFSFake implements aws.EFSAPI against fixture data loaded at construction time.
type EFSFake struct {
	fix *fixtures.EFSFixtures
}

// NewEFS constructs an EFSFake backed by fixture data from the fixtures package.
func NewEFS() *EFSFake {
	return &EFSFake{fix: fixtures.NewEFSFixtures()}
}

func (f *EFSFake) DescribeFileSystems(_ context.Context, _ *efs.DescribeFileSystemsInput, _ ...func(*efs.Options)) (*efs.DescribeFileSystemsOutput, error) {
	return &efs.DescribeFileSystemsOutput{FileSystems: f.fix.FileSystems}, nil
}

// DescribeMountTargets returns mount targets for the requested filesystem.
// Input.FileSystemId selects which filesystem's mount targets to return.
func (f *EFSFake) DescribeMountTargets(_ context.Context, in *efs.DescribeMountTargetsInput, _ ...func(*efs.Options)) (*efs.DescribeMountTargetsOutput, error) {
	if in == nil || in.FileSystemId == nil {
		return &efs.DescribeMountTargetsOutput{}, nil
	}
	id := aws.ToString(in.FileSystemId)
	if !f.hasFileSystem(id) {
		return nil, &efstypes.FileSystemNotFound{Message: notFoundMessage("File system", id)}
	}
	return &efs.DescribeMountTargetsOutput{MountTargets: f.fix.MountTargets[id]}, nil
}

// DescribeAccessPoints returns access points for the requested filesystem.
// Input.FileSystemId selects which filesystem's access points to return.
func (f *EFSFake) DescribeAccessPoints(_ context.Context, in *efs.DescribeAccessPointsInput, _ ...func(*efs.Options)) (*efs.DescribeAccessPointsOutput, error) {
	if in == nil || in.FileSystemId == nil {
		return &efs.DescribeAccessPointsOutput{}, nil
	}
	aps := f.fix.AccessPoints[aws.ToString(in.FileSystemId)]
	return &efs.DescribeAccessPointsOutput{AccessPoints: aps}, nil
}

// DescribeFileSystemPolicy returns the file system's resource policy. File
// systems absent from FileSystemPolicies have none, which AWS reports as
// PolicyNotFound — the healthy default.
func (f *EFSFake) DescribeFileSystemPolicy(_ context.Context, in *efs.DescribeFileSystemPolicyInput, _ ...func(*efs.Options)) (*efs.DescribeFileSystemPolicyOutput, error) {
	id := aws.ToString(in.FileSystemId)
	policy, ok := f.fix.FileSystemPolicies[id]
	if !ok {
		return nil, &efstypes.PolicyNotFound{Message: aws.String("No policy for " + id)}
	}
	return &efs.DescribeFileSystemPolicyOutput{
		FileSystemId: aws.String(id),
		Policy:       aws.String(policy),
	}, nil
}

// DescribeBackupPolicy returns the file system's AWS Backup policy. Only the
// file systems listed in BackupPolicyDisabled have it off; everything else
// reports ENABLED — the healthy default.
func (f *EFSFake) DescribeBackupPolicy(_ context.Context, in *efs.DescribeBackupPolicyInput, _ ...func(*efs.Options)) (*efs.DescribeBackupPolicyOutput, error) {
	id := aws.ToString(in.FileSystemId)
	if f.fix.BackupPolicyDisabled[id] {
		return &efs.DescribeBackupPolicyOutput{
			BackupPolicy: &efstypes.BackupPolicy{Status: efstypes.StatusDisabled},
		}, nil
	}
	return &efs.DescribeBackupPolicyOutput{
		BackupPolicy: &efstypes.BackupPolicy{Status: efstypes.StatusEnabled},
	}, nil
}

// hasFileSystem reports whether the fixtures register this file system. One
// with no mount targets still answers an empty list.
func (f *EFSFake) hasFileSystem(id string) bool {
	return slices.ContainsFunc(f.fix.FileSystems, func(fs efstypes.FileSystemDescription) bool {
		return aws.ToString(fs.FileSystemId) == id
	})
}
