package unit

// helpers_efs_test.go — shared EFS test helpers. Per project convention
// (helpers_color_test.go, helpers_demo_ec2_test.go, …) test helpers live
// in a dedicated helpers_*_test.go file rather than being scattered
// across the test files that consume them.

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/efs"
	efstypes "github.com/aws/aws-sdk-go-v2/service/efs/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo/fixtures"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// efsMountTargetFake implements EFSAPI for enrichment testing. It embeds
// the interface and overrides only DescribeMountTargets. The results map
// is keyed by FileSystemId (from the input) so the fake can serve
// different responses per resource.
type efsMountTargetFake struct {
	awsclient.EFSAPI
	// results maps FileSystemId → response. If absent the fake returns errByFS.
	results map[string][]efstypes.MountTargetDescription
	// errByFS maps FileSystemId → error; overrides results when set.
	errByFS map[string]error
}

func (f *efsMountTargetFake) DescribeMountTargets(
	_ context.Context,
	in *efs.DescribeMountTargetsInput,
	_ ...func(*efs.Options),
) (*efs.DescribeMountTargetsOutput, error) {
	fsID := ""
	if in != nil && in.FileSystemId != nil {
		fsID = *in.FileSystemId
	}
	if f.errByFS != nil {
		if err, ok := f.errByFS[fsID]; ok {
			return nil, err
		}
	}
	mts := f.results[fsID]
	return &efs.DescribeMountTargetsOutput{MountTargets: mts}, nil
}

// Compile-time check: efsMountTargetFake satisfies EFSAPI.
var _ awsclient.EFSAPI = (*efsMountTargetFake)(nil)

// efsMTFakeFromFixtures builds an efsMountTargetFake from the canonical
// fixture data. The fake serves results keyed by FileSystemId.
func efsMTFakeFromFixtures() *efsMountTargetFake {
	fix := fixtures.NewEFSFixtures()
	return &efsMountTargetFake{results: fix.MountTargets}
}

// efsResources returns a slice of EFS Resource stubs with the given IDs.
func efsResources(ids ...string) []resource.Resource {
	res := make([]resource.Resource, 0, len(ids))
	for _, id := range ids {
		res = append(res, resource.Resource{
			ID:   id,
			Name: "efs-" + id,
			Fields: map[string]string{
				"file_system_id":   id,
				"life_cycle_state": "available",
				"mount_targets":    "1",
			},
		})
	}
	return res
}

// availableMT returns an available MountTargetDescription for a given file system.
func availableMT(fsID, mtID string) efstypes.MountTargetDescription {
	return efstypes.MountTargetDescription{
		FileSystemId:   aws.String(fsID),
		MountTargetId:  aws.String(mtID),
		SubnetId:       aws.String("subnet-00000001"),
		LifeCycleState: efstypes.LifeCycleStateAvailable,
	}
}

// unavailableMT returns a MountTargetDescription with the given lifecycle state.
func unavailableMT(fsID, mtID string, state efstypes.LifeCycleState) efstypes.MountTargetDescription {
	return efstypes.MountTargetDescription{
		FileSystemId:   aws.String(fsID),
		MountTargetId:  aws.String(mtID),
		SubnetId:       aws.String("subnet-00000001"),
		LifeCycleState: state,
	}
}
