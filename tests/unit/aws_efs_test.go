package unit

import (
	"context"
	"sort"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/efs"
	efstypes "github.com/aws/aws-sdk-go-v2/service/efs/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo/fakes"
	"github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/resource"
)

func buildEFSResourcesFromFake() ([]resource.Resource, error) {
	fake := fakes.NewEFS()
	return collectAllPages(func(token string) (resource.FetchResult, error) {
		return awsclient.FetchEFSFileSystemsPage(context.Background(), fake, token)
	})
}

func efsResourceByID(resources []resource.Resource, id string) (resource.Resource, bool) {
	for _, r := range resources {
		if r.ID == id {
			return r, true
		}
	}
	return resource.Resource{}, false
}

func TestFetchEFSFileSystems_HealthyBaseline_Silence(t *testing.T) {
	resources, err := buildEFSResourcesFromFake()
	if err != nil {
		t.Fatalf("FetchEFSFileSystems error: %v", err)
	}

	r, ok := efsResourceByID(resources, fixtures.ProdEFSID)
	if !ok {
		t.Fatalf("graph-root fixture %q not found in fetcher output", fixtures.ProdEFSID)
	}

	if statusField := r.Fields["status"]; statusField != "" {
		t.Errorf("Fields[\"status\"] = %q, want %q (healthy rows must have blank status field)", statusField, "")
	}
	if len(r.Findings) != 0 {
		t.Errorf("Findings = %v, want nil/empty (healthy rows have no Wave-1 findings)", r.Findings)
	}
}

func TestFetchEFSFileSystems_Creating_Warning(t *testing.T) {
	const fsID = "fs-0warncreating0001"

	resources, err := buildEFSResourcesFromFake()
	if err != nil {
		t.Fatalf("FetchEFSFileSystems error: %v", err)
	}

	r, ok := efsResourceByID(resources, fsID)
	if !ok {
		t.Fatalf("fixture %q not found in fetcher output", fsID)
	}

	if r.Fields["status"] != "creating" {
		t.Errorf("Fields[\"status\"] = %q, want %q", r.Fields["status"], "creating")
	}
	if len(r.Findings) != 1 || r.Findings[0].Phrase != "creating" {
		t.Errorf("Findings = %v, want one finding with Phrase %q", r.Findings, "creating")
	}
}

func TestFetchEFSFileSystems_Updating_Warning(t *testing.T) {
	const fsID = "fs-0warnupdating0001"

	resources, err := buildEFSResourcesFromFake()
	if err != nil {
		t.Fatalf("FetchEFSFileSystems error: %v", err)
	}

	r, ok := efsResourceByID(resources, fsID)
	if !ok {
		t.Fatalf("fixture %q not found in fetcher output", fsID)
	}

	if r.Fields["status"] != "updating" {
		t.Errorf("Fields[\"status\"] = %q, want %q", r.Fields["status"], "updating")
	}
	if len(r.Findings) != 1 || r.Findings[0].Phrase != "updating" {
		t.Errorf("Findings = %v, want one finding with Phrase %q", r.Findings, "updating")
	}
}

func TestFetchEFSFileSystems_Deleting_Warning(t *testing.T) {
	const fsID = "fs-0warndeleting0001"

	resources, err := buildEFSResourcesFromFake()
	if err != nil {
		t.Fatalf("FetchEFSFileSystems error: %v", err)
	}

	r, ok := efsResourceByID(resources, fsID)
	if !ok {
		t.Fatalf("fixture %q not found in fetcher output", fsID)
	}

	if r.Fields["status"] != "deleting" {
		t.Errorf("Fields[\"status\"] = %q, want %q", r.Fields["status"], "deleting")
	}
	if len(r.Findings) != 1 || r.Findings[0].Phrase != "deleting" {
		t.Errorf("Findings = %v, want one finding with Phrase %q", r.Findings, "deleting")
	}
}

func TestFetchEFSFileSystems_Error_Broken(t *testing.T) {
	const fsID = "fs-0brokenerror00001"

	resources, err := buildEFSResourcesFromFake()
	if err != nil {
		t.Fatalf("FetchEFSFileSystems error: %v", err)
	}

	r, ok := efsResourceByID(resources, fsID)
	if !ok {
		t.Fatalf("fixture %q not found in fetcher output", fsID)
	}

	if r.Fields["status"] != "error" {
		t.Errorf("Fields[\"status\"] = %q, want %q", r.Fields["status"], "error")
	}
	if len(r.Findings) != 1 || r.Findings[0].Phrase != "error" {
		t.Errorf("Findings = %v, want one finding with Phrase %q", r.Findings, "error")
	}
}

func TestFetchEFSFileSystems_NoMountTargets_Broken(t *testing.T) {
	const fsID = "fs-0brokennomt000001"

	resources, err := buildEFSResourcesFromFake()
	if err != nil {
		t.Fatalf("FetchEFSFileSystems error: %v", err)
	}

	r, ok := efsResourceByID(resources, fsID)
	if !ok {
		t.Fatalf("fixture %q not found in fetcher output", fsID)
	}

	if r.Fields["status"] != "no mount targets" {
		t.Errorf("Fields[\"status\"] = %q, want %q", r.Fields["status"], "no mount targets")
	}
	if len(r.Findings) != 1 || r.Findings[0].Phrase != "no mount targets" {
		t.Errorf("Findings = %v, want one finding with Phrase %q", r.Findings, "no mount targets")
	}
}

func TestFetchEFSFileSystems_MultiW1_NomountsPlusDeleting(t *testing.T) {
	const fsID = "fs-0warnmulti0000001"

	resources, err := buildEFSResourcesFromFake()
	if err != nil {
		t.Fatalf("FetchEFSFileSystems error: %v", err)
	}

	r, ok := efsResourceByID(resources, fsID)
	if !ok {
		t.Fatalf("fixture %q not found in fetcher output", fsID)
	}

	wantStatus := "no mount targets (+1)"
	if statusField := r.Fields["status"]; statusField != wantStatus {
		t.Errorf("Fields[\"status\"] = %q, want %q (top phrase + (+N) suffix)", statusField, wantStatus)
	}
	wantPhrases := []string{"no mount targets", "deleting"}
	if len(r.Findings) != len(wantPhrases) {
		t.Errorf("Findings = %v, want %d findings", r.Findings, len(wantPhrases))
	} else {
		for i, want := range wantPhrases {
			if r.Findings[i].Phrase != want {
				t.Errorf("Findings[%d].Phrase = %q, want %q", i, r.Findings[i].Phrase, want)
			}
		}
	}
}

func TestFetchEFSFileSystems_PopulatesResourceIssues(t *testing.T) {
	resources, err := buildEFSResourcesFromFake()
	if err != nil {
		t.Fatalf("FetchEFSFileSystems error: %v", err)
	}

	byID := make(map[string]resource.Resource, len(resources))
	for _, r := range resources {
		byID[r.ID] = r
	}

	cases := []struct {
		name       string
		fsID       string
		wantIssues []string // nil means empty/nil is expected
	}{
		{
			name:       "healthy baseline (prod-efs-app-data)",
			fsID:       fixtures.ProdEFSID,
			wantIssues: nil,
		},
		{
			name:       "warn-efs-creating (single W1)",
			fsID:       "fs-0warncreating0001",
			wantIssues: []string{"creating"},
		},
		{
			name:       "warn-efs-updating (single W1)",
			fsID:       "fs-0warnupdating0001",
			wantIssues: []string{"updating"},
		},
		{
			name:       "warn-efs-deleting (single W1)",
			fsID:       "fs-0warndeleting0001",
			wantIssues: []string{"deleting"},
		},
		{
			name:       "broken-efs-error (single W1)",
			fsID:       "fs-0brokenerror00001",
			wantIssues: []string{"error"},
		},
		{
			name:       "broken-efs-no-mount-targets (single W1)",
			fsID:       "fs-0brokennomt000001",
			wantIssues: []string{"no mount targets"},
		},
		{
			name:       "warn-efs-multi (multi-W1: Broken first, Warning second)",
			fsID:       "fs-0warnmulti0000001",
			wantIssues: []string{"no mount targets", "deleting"},
		},
		{
			name:       "warn-efs-updating-mt-down (W1 only; W2 in EnrichmentFinding, not Issues)",
			fsID:       "fs-0warnupdmtdown001",
			wantIssues: []string{"updating"},
		},
		{
			name:       "healthy-efs-with-mt-down (W2-only: Issues must be empty)",
			fsID:       "fs-0healthymtdown001",
			wantIssues: nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, ok := byID[tc.fsID]
			if !ok {
				t.Fatalf("fixture %q not found in fetcher output (got IDs: %v)", tc.fsID, sortedKeys(byID))
			}
			if tc.wantIssues == nil {
				if len(r.Findings) != 0 {
					t.Errorf("Findings = %v, want nil/empty", r.Findings)
				}
				return
			}
			if len(r.Findings) != len(tc.wantIssues) {
				t.Errorf("Findings len = %d, want %d; Findings = %v, want phrases %v", len(r.Findings), len(tc.wantIssues), r.Findings, tc.wantIssues)
				return
			}
			for i, want := range tc.wantIssues {
				if r.Findings[i].Phrase != want {
					t.Errorf("Findings[%d].Phrase = %q, want %q", i, r.Findings[i].Phrase, want)
				}
			}
		})
	}
}

func sortedKeys(m map[string]resource.Resource) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// With no CloudWatch client in scope, any CloudWatch call panics or errors.
func TestEFS_NoCloudWatchMetricCalls(t *testing.T) {
	mock := &fakeEFSDescribeFileSystems{
		Output: &efs.DescribeFileSystemsOutput{
			FileSystems: []efstypes.FileSystemDescription{
				{
					FileSystemId:         aws.String("fs-nocw000000001"),
					FileSystemArn:        aws.String("arn:aws:elasticfilesystem:us-east-1:123456789012:file-system/fs-nocw000000001"),
					LifeCycleState:       efstypes.LifeCycleStateAvailable,
					NumberOfMountTargets: 1,
					PerformanceMode:      efstypes.PerformanceModeGeneralPurpose,
					ThroughputMode:       efstypes.ThroughputModeBursting,
					Encrypted:            aws.Bool(true),
					CreationToken:        aws.String("nocw-token"),
					OwnerId:              aws.String("123456789012"),
					SizeInBytes:          &efstypes.FileSystemSize{Value: 0},
					Tags:                 []efstypes.Tag{},
				},
			},
		},
	}

	resources, err := collectAllPages(func(token string) (resource.FetchResult, error) {
		return awsclient.FetchEFSFileSystemsPage(context.Background(), mock, token)
	})
	if err != nil {
		t.Fatalf("FetchEFSFileSystems must not call CloudWatch; error: %v", err)
	}
	if len(resources) != 1 {
		t.Errorf("expected 1 resource, got %d", len(resources))
	}
}

func TestEFS_NoCloudWatchMetricCalls_Enricher(t *testing.T) {
	fake := &efsMountTargetFake{
		results: map[string][]efstypes.MountTargetDescription{
			"fs-nocw-enrich001": {
				{
					MountTargetId:  aws.String("fsmt-nocw001a"),
					FileSystemId:   aws.String("fs-nocw-enrich001"),
					LifeCycleState: efstypes.LifeCycleStateAvailable,
					SubnetId:       aws.String("subnet-00000001"),
				},
			},
		},
	}

	// CloudWatch is nil, so any CloudWatch use by the enricher panics.
	clients := &awsclient.ServiceClients{EFS: fake}
	resources := efsResources("fs-nocw-enrich001")

	result, err := awsclient.EnrichEFSMountTargets(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("EnrichEFSMountTargets must not call CloudWatch; error: %v", err)
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected 0 findings, got %d", len(result.Findings))
	}
}

func TestFetchEFSFileSystems_StatusField_Warning(t *testing.T) {
	resources, err := buildEFSResourcesFromFake()
	if err != nil {
		t.Fatalf("FetchEFSFileSystems error: %v", err)
	}

	byID := make(map[string]resource.Resource, len(resources))
	for _, r := range resources {
		byID[r.ID] = r
	}

	warningCases := []struct {
		fsID       string
		wantPhrase string
	}{
		{"fs-0warncreating0001", "creating"},
		{"fs-0warnupdating0001", "updating"},
		{"fs-0warndeleting0001", "deleting"},
	}

	for _, tc := range warningCases {
		t.Run(tc.fsID, func(t *testing.T) {
			r, ok := byID[tc.fsID]
			if !ok {
				t.Fatalf("fixture %q not found", tc.fsID)
			}
			got := r.Fields["status"]
			if got != tc.wantPhrase {
				t.Errorf("Fields[\"status\"] = %q, want %q", got, tc.wantPhrase)
			}
			// status must not embed raw SDK enum suffix (e.g. "LifeCycleState.creating")
			if strings.Contains(got, ".") {
				t.Errorf("Fields[\"status\"] %q contains dot — must be plain phrase", got)
			}
		})
	}
}

func TestFetchEFSFileSystems_StatusField_Broken(t *testing.T) {
	resources, err := buildEFSResourcesFromFake()
	if err != nil {
		t.Fatalf("FetchEFSFileSystems error: %v", err)
	}

	byID := make(map[string]resource.Resource, len(resources))
	for _, r := range resources {
		byID[r.ID] = r
	}

	brokenCases := []struct {
		fsID       string
		wantPhrase string
	}{
		{"fs-0brokenerror00001", "error"},
		{"fs-0brokennomt000001", "no mount targets"},
	}

	for _, tc := range brokenCases {
		t.Run(tc.fsID, func(t *testing.T) {
			r, ok := byID[tc.fsID]
			if !ok {
				t.Fatalf("fixture %q not found", tc.fsID)
			}
			got := r.Fields["status"]
			if got != tc.wantPhrase {
				t.Errorf("Fields[\"status\"] = %q, want %q", got, tc.wantPhrase)
			}
		})
	}
}

func TestFetchEFSFileSystems_W1Updating_IsolatesFromMTState(t *testing.T) {
	const fsID = "fs-0warnupdating0001"

	resources, err := buildEFSResourcesFromFake()
	if err != nil {
		t.Fatalf("FetchEFSFileSystems error: %v", err)
	}

	r, ok := efsResourceByID(resources, fsID)
	if !ok {
		t.Fatalf("fixture %q not found", fsID)
	}

	if r.Fields["status"] != "updating" {
		t.Errorf("Fields[\"status\"] = %q, want %q (W1 signal only; MT state is W2)", r.Fields["status"], "updating")
	}
	if len(r.Findings) != 1 || r.Findings[0].Phrase != "updating" {
		t.Errorf("Findings = %v, want one finding with Phrase %q", r.Findings, "updating")
	}
}
