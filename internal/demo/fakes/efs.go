package fakes

import (
	"context"

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
