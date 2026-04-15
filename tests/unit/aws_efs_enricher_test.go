package unit

// aws_efs_enricher_test.go — Behavioral tests for EnrichEFSMountTargets.
//
// Contract assertions:
//   - DescribeMountTargets is called once per EFS resource (keyed by file system ID).
//   - All mount targets LifeCycleState=available → 0 findings.
//   - Any mount target LifeCycleState != available (e.g. "creating") → 1 finding for
//     that file system, severity "!".
//   - clients.EFS == nil → (EnricherResult{Findings: non-nil empty}, nil).
//   - API error for a resource → 0 findings for that resource, Truncated=true, no error returned.

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/efs"
	efstypes "github.com/aws/aws-sdk-go-v2/service/efs/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// efsMountTargetFake implements EFSAPI for enrichment testing.
// It embeds the interface and overrides only DescribeMountTargets.
// The results map is keyed by FileSystemId (from the input) so the fake
// can serve different responses per resource.
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

// TestEnrichEFSMountTargets_AllAvailableProducesNoFindings verifies that when all
// mount targets for both EFS resources are in the "available" state, no findings
// are produced and IssueCount is 0.
func TestEnrichEFSMountTargets_AllAvailableProducesNoFindings(t *testing.T) {
	fake := &efsMountTargetFake{
		results: map[string][]efstypes.MountTargetDescription{
			"fs-00000001": {availableMT("fs-00000001", "fsmt-a001")},
			"fs-00000002": {availableMT("fs-00000002", "fsmt-a002")},
		},
	}
	clients := &awsclient.ServiceClients{EFS: fake}
	resources := efsResources("fs-00000001", "fs-00000002")

	result, err := awsclient.EnrichEFSMountTargets(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Findings == nil {
		t.Fatal("Findings must not be nil")
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected 0 findings, got %d: %v", len(result.Findings), result.Findings)
	}
	if result.IssueCount != 0 {
		t.Errorf("IssueCount = %d, want 0", result.IssueCount)
	}
}

// TestEnrichEFSMountTargets_OneUnavailableMTProducesFindingSevBang verifies that
// when EFS-1 has a mount target in "creating" state, a finding with severity "!"
// is produced for EFS-1, and EFS-2 (all available) produces no finding.
func TestEnrichEFSMountTargets_OneUnavailableMTProducesFindingSevBang(t *testing.T) {
	fake := &efsMountTargetFake{
		results: map[string][]efstypes.MountTargetDescription{
			"fs-00000001": {
				availableMT("fs-00000001", "fsmt-b001"),
				unavailableMT("fs-00000001", "fsmt-b002", efstypes.LifeCycleStateCreating),
			},
			"fs-00000002": {availableMT("fs-00000002", "fsmt-b003")},
		},
	}
	clients := &awsclient.ServiceClients{EFS: fake}
	resources := efsResources("fs-00000001", "fs-00000002")

	result, err := awsclient.EnrichEFSMountTargets(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	f, ok := result.Findings["fs-00000001"]
	if !ok {
		t.Fatalf("expected finding keyed by %q", "fs-00000001")
	}
	if f.Severity != "!" {
		t.Errorf("severity = %q, want %q", f.Severity, "!")
	}
	if _, ok := result.Findings["fs-00000002"]; ok {
		t.Error("fs-00000002 must NOT appear in Findings — all its MTs are available")
	}
	if result.IssueCount != 1 {
		t.Errorf("IssueCount = %d, want 1", result.IssueCount)
	}
}

// TestEnrichEFSMountTargets_NilClientReturnsEmptyFindingsNoError verifies that when
// clients.EFS is nil the enricher returns a non-nil empty Findings map and no error.
func TestEnrichEFSMountTargets_NilClientReturnsEmptyFindingsNoError(t *testing.T) {
	clients := &awsclient.ServiceClients{EFS: nil}

	result, err := awsclient.EnrichEFSMountTargets(context.Background(), clients, efsResources("fs-00000001", "fs-00000002"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Findings == nil {
		t.Error("Findings must not be nil when EFS client is nil")
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected empty Findings, got %d entries", len(result.Findings))
	}
}

// TestEnrichEFSMountTargets_APIErrorSetsTruncatedNoError verifies that when the
// API call for EFS-1 returns an error, the enricher sets Truncated=true, produces
// 0 findings, and does not propagate the error.
func TestEnrichEFSMountTargets_APIErrorSetsTruncatedNoError(t *testing.T) {
	apiErr := errors.New("efs: DescribeMountTargets throttled")
	fake := &efsMountTargetFake{
		errByFS: map[string]error{
			"fs-00000001": apiErr,
		},
		results: map[string][]efstypes.MountTargetDescription{
			"fs-00000002": {availableMT("fs-00000002", "fsmt-c001")},
		},
	}
	clients := &awsclient.ServiceClients{EFS: fake}
	resources := efsResources("fs-00000001", "fs-00000002")

	result, err := awsclient.EnrichEFSMountTargets(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected 0 findings on API error, got %d", len(result.Findings))
	}
	if !result.Truncated {
		t.Error("Truncated must be true when an API call fails")
	}
}
