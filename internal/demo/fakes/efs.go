package fakes

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/efs"

	"github.com/k2m30/a9s/v3/internal/demo/fixtures"
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
	mts := f.fix.MountTargets[aws.ToString(in.FileSystemId)]
	return &efs.DescribeMountTargetsOutput{MountTargets: mts}, nil
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
