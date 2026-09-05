package unit

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/efs"
	efstypes "github.com/aws/aws-sdk-go-v2/service/efs/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
)

// d1EFSSinglePageMock serves one page of file systems to the efs fetcher.
type d1EFSSinglePageMock struct {
	output *efs.DescribeFileSystemsOutput
}

func (m *d1EFSSinglePageMock) DescribeFileSystems(
	_ context.Context, _ *efs.DescribeFileSystemsInput, _ ...func(*efs.Options),
) (*efs.DescribeFileSystemsOutput, error) {
	return m.output, nil
}

// d1EFSBaseline is a healthy file system: available, encrypted, three mount
// targets. Mount targets are what makes it reachable, so a baseline with none
// would carry a Broken finding before any case set its own state.
func d1EFSBaseline() efstypes.FileSystemDescription {
	return efstypes.FileSystemDescription{
		FileSystemId:         aws.String("fs-0prod1234abcd5678"),
		Name:                 aws.String("prod-app-data"),
		LifeCycleState:       efstypes.LifeCycleStateAvailable,
		NumberOfMountTargets: 3,
		Encrypted:            aws.Bool(true),
		PerformanceMode:      efstypes.PerformanceModeGeneralPurpose,
		ThroughputMode:       efstypes.ThroughputModeElastic,
	}
}

// TestEfsColor pins the lifecycle state → colour mapping for EFS file systems.
//
// "mount target down" is not here: it is a wave-2 phrase produced by
// EnrichEFSMountTargets from the mount-target API, which the fetcher never
// calls. prowler_w2_efs covers it. The "(+N)" rows are gone with the phrase
// matching that needed them.
func TestEfsColor(t *testing.T) {
	cases := []struct {
		name   string
		state  efstypes.LifeCycleState
		mutate func(*efstypes.FileSystemDescription)
		want   resource.Color
	}{
		{name: "available", state: efstypes.LifeCycleStateAvailable, want: resource.ColorHealthy},

		{name: "creating", state: efstypes.LifeCycleStateCreating, want: resource.ColorWarning},
		{name: "updating", state: efstypes.LifeCycleStateUpdating, want: resource.ColorWarning},
		{name: "deleting", state: efstypes.LifeCycleStateDeleting, want: resource.ColorWarning},

		{name: "error", state: efstypes.LifeCycleStateError, want: resource.ColorBroken},
		{
			name:   "no_mount_targets",
			state:  efstypes.LifeCycleStateAvailable,
			mutate: func(fs *efstypes.FileSystemDescription) { fs.NumberOfMountTargets = 0 },
			want:   resource.ColorBroken,
		},
		{
			name:   "not_encrypted",
			state:  efstypes.LifeCycleStateAvailable,
			mutate: func(fs *efstypes.FileSystemDescription) { fs.Encrypted = aws.Bool(false) },
			want:   resource.ColorWarning,
		},
		{
			name:   "no_mount_targets_outranks_updating",
			state:  efstypes.LifeCycleStateUpdating,
			mutate: func(fs *efstypes.FileSystemDescription) { fs.NumberOfMountTargets = 0 },
			want:   resource.ColorBroken,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fs := d1EFSBaseline()
			fs.LifeCycleState = tc.state
			if tc.mutate != nil {
				tc.mutate(&fs)
			}
			mock := &d1EFSSinglePageMock{
				output: &efs.DescribeFileSystemsOutput{
					FileSystems: []efstypes.FileSystemDescription{fs},
				},
			}
			page, err := awsclient.FetchEFSFileSystemsPage(context.Background(), mock, "")
			if err != nil {
				t.Fatalf("FetchEFSFileSystemsPage: %v", err)
			}
			if len(page.Resources) != 1 {
				t.Fatalf("expected 1 resource, got %d", len(page.Resources))
			}
			d1AssertColor(t, page.Resources[0], tc.want)
		})
	}
}
