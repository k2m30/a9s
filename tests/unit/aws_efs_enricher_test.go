package unit

import (
	"context"
	"errors"
	"strings"
	"testing"

	efstypes "github.com/aws/aws-sdk-go-v2/service/efs/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
)

func TestEnrichEFSMountTargets_AllAvailableProducesNoFindings(t *testing.T) {
	fake := &efsMountTargetFake{
		results: map[string][]efstypes.MountTargetDescription{
			"fs-00000001": {availableMT("fs-00000001", "fsmt-a001")},
			"fs-00000002": {availableMT("fs-00000002", "fsmt-a002")},
		},
	}
	clients := &awsclient.ServiceClients{EFS: fake}
	resources := efsResources("fs-00000001", "fs-00000002")

	result, err := awsclient.EnrichEFSMountTargets(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Findings == nil {
		t.Fatal("Findings must not be nil")
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected 0 findings, got %d: %v", len(result.Findings), result.Findings)
	}
}

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

	result, err := awsclient.EnrichEFSMountTargets(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fs, ok := result.Findings["fs-00000001"]
	if !ok {
		t.Fatalf("expected finding keyed by %q", "fs-00000001")
	}
	f := fs[0]
	if f.Severity != domain.SevBroken {
		t.Errorf("severity = %v, want %v", f.Severity, "!")
	}
	if _, ok := result.Findings["fs-00000002"]; ok {
		t.Error("fs-00000002 must NOT appear in Findings — all its MTs are available")
	}
}

func TestEnrichEFSMountTargets_NilClientReturnsEmptyFindingsNoError(t *testing.T) {
	clients := &awsclient.ServiceClients{EFS: nil}

	result, err := awsclient.EnrichEFSMountTargets(context.Background(), clients, efsResources("fs-00000001", "fs-00000002"), nil)
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

func TestEnrichEFSMountTargets_APIErrorSetsTruncatedAndSurfacesError(t *testing.T) {
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

	result, err := awsclient.EnrichEFSMountTargets(context.Background(), clients, resources, nil)
	if err == nil {
		t.Fatal("enricher must surface a composite error when an API call fails")
	}
	// The aggregate names the call, not the type: the type comes from the
	// registry key at the surface, and a type in the label would render it
	// twice ("enrich efs: efs: DescribeMountTargets ...").
	if errStr := err.Error(); !strings.Contains(errStr, "DescribeMountTargets") {
		t.Errorf("composite error must name the call, %q, got: %q", "DescribeMountTargets", errStr)
	}
	if errStr := err.Error(); !strings.Contains(errStr, "fs-00000001") {
		t.Errorf("composite error must contain the failing file system ID \"fs-00000001\", got: %q", errStr)
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected 0 findings on API error, got %d", len(result.Findings))
	}
	if !result.Truncated {
		t.Error("Truncated must be true when an API call fails")
	}
}
