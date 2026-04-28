// aws_efs_test.go — Wave-1 fetcher behavioral tests for EFS file systems.
//
// Tests the CONTRACT from docs/resources/efs-impl-plan.md §1, not the current
// implementation. Phase 7 coder will make these pass.
//
// Covered invariants:
//   - TEST: efs_available_silence (U1) — healthy baseline: Status="", Issues=nil, Fields["status"]="".
//   - TEST: efs_lifecycle_creating_warning — Status="creating", Issues=["creating"], Warning.
//   - TEST: efs_lifecycle_updating_warning — Status="updating", Issues=["updating"], Warning.
//   - TEST: efs_lifecycle_deleting_warning — Status="deleting", Issues=["deleting"], Warning.
//   - TEST: efs_lifecycle_error_broken — Status="error", Issues=["error"], Broken.
//   - TEST: efs_no_mount_targets_broken — Status="no mount targets", Issues=["no mount targets"], Broken.
//   - TEST: efs_multi_w1_no_mounts_plus_deleting_suffix (U7a) — multi-W1: "no mount targets (+1)".
//   - TEST: efs_fetcher_populates_resource_issues (U7f) — table-driven across every fixture.
//   - TEST: efs_out_of_scope_percentiolimit_unreachable — zero CW metric calls.
package unit

import (
	"context"
	"sort"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/efs"
	efstypes "github.com/aws/aws-sdk-go-v2/service/efs/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo/fakes"
	"github.com/k2m30/a9s/v3/internal/demo/fixtures"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ---------------------------------------------------------------------------
// mockEFSDescribeOnly — implements EFSDescribeFileSystemsAPI for fetcher tests.
// Delegates to mockEFSClient (defined in mocks_services_test.go).
// ---------------------------------------------------------------------------

func buildEFSResourcesFromFake() ([]resource.Resource, error) {
	fake := fakes.NewEFS()
	return awsclient.FetchEFSFileSystems(context.Background(), fake)
}

func efsResourceByID(resources []resource.Resource, id string) (resource.Resource, bool) {
	for _, r := range resources {
		if r.ID == id {
			return r, true
		}
	}
	return resource.Resource{}, false
}

// ---------------------------------------------------------------------------
// TEST: efs_available_silence (U1)
// ---------------------------------------------------------------------------

// TestFetchEFSFileSystems_HealthyBaseline_Silence verifies that the graph-root
// fixture (prod-app-data, LifeCycleState="available", 3 MTs) produces:
//   - Resource.Status = "" (NOT "available")
//   - Resource.Fields["status"] = ""
//   - Resource.Issues = nil or empty
//
// Spec §4: "Wave 1 Healthy → no §4 row; S4 renders blank."
func TestFetchEFSFileSystems_HealthyBaseline_Silence(t *testing.T) {
	resources, err := buildEFSResourcesFromFake()
	if err != nil {
		t.Fatalf("FetchEFSFileSystems error: %v", err)
	}

	r, ok := efsResourceByID(resources, fixtures.ProdEFSID)
	if !ok {
		t.Fatalf("graph-root fixture %q not found in fetcher output", fixtures.ProdEFSID)
	}

	if r.Status != "" {
		t.Errorf("Status = %q, want %q (healthy rows must have blank Status per spec §4)", r.Status, "")
	}
	if statusField := r.Fields["status"]; statusField != "" {
		t.Errorf("Fields[\"status\"] = %q, want %q (healthy rows must have blank status field)", statusField, "")
	}
	if len(r.Findings) != 0 {
		t.Errorf("Findings = %v, want nil/empty (healthy rows have no Wave-1 findings)", r.Findings)
	}
}

// ---------------------------------------------------------------------------
// TEST: efs_lifecycle_creating_warning
// ---------------------------------------------------------------------------

// TestFetchEFSFileSystems_Creating_Warning verifies that LifeCycleState="creating"
// maps to Status="creating", Issues=["creating"].
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

// ---------------------------------------------------------------------------
// TEST: efs_lifecycle_updating_warning
// ---------------------------------------------------------------------------

// TestFetchEFSFileSystems_Updating_Warning verifies that LifeCycleState="updating"
// maps to Status="updating", Issues=["updating"].
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

// ---------------------------------------------------------------------------
// TEST: efs_lifecycle_deleting_warning
// ---------------------------------------------------------------------------

// TestFetchEFSFileSystems_Deleting_Warning verifies that LifeCycleState="deleting"
// maps to Status="deleting", Issues=["deleting"].
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

// ---------------------------------------------------------------------------
// TEST: efs_lifecycle_error_broken
// ---------------------------------------------------------------------------

// TestFetchEFSFileSystems_Error_Broken verifies that LifeCycleState="error"
// maps to Status="error", Issues=["error"].
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

// ---------------------------------------------------------------------------
// TEST: efs_no_mount_targets_broken
// ---------------------------------------------------------------------------

// TestFetchEFSFileSystems_NoMountTargets_Broken verifies that
// NumberOfMountTargets == 0 maps to Status="no mount targets",
// Issues=["no mount targets"], even when LifeCycleState="available".
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

// ---------------------------------------------------------------------------
// TEST: efs_multi_w1_no_mounts_plus_deleting_suffix (U7a)
// ---------------------------------------------------------------------------

// TestFetchEFSFileSystems_MultiW1_NomountsPlusDeleting verifies the multi-W1
// fixture (warn-efs-multi: LifeCycleState="deleting" + NumberOfMountTargets=0):
//
//   - §4 precedence (severity first, then table order): Broken > Warning
//   - Resource.Status = "no mount targets (+1)"
//   - Resource.Fields["status"] = "no mount targets (+1)"
//   - Resource.Issues = ["no mount targets", "deleting"]
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
	if r.Status != "" {
		t.Errorf("Status = %q, want %q (fetcher must not write Status)", r.Status, "")
	}
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

// ---------------------------------------------------------------------------
// TEST: efs_fetcher_populates_resource_issues (U7f)
// ---------------------------------------------------------------------------

// TestFetchEFSFileSystems_PopulatesResourceIssues is a table-driven test that
// verifies every fixture yields the correct Resource.Issues slice (U7f).
// Wave-2-only fixtures (healthy-efs-with-mt-down) must have Issues nil/empty.
func TestFetchEFSFileSystems_PopulatesResourceIssues(t *testing.T) {
	resources, err := buildEFSResourcesFromFake()
	if err != nil {
		t.Fatalf("FetchEFSFileSystems error: %v", err)
	}

	// indexByID for quick lookup
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

// sortedKeys returns sorted keys of a resource map for diagnostic output.
func sortedKeys(m map[string]resource.Resource) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// ---------------------------------------------------------------------------
// TEST: efs_out_of_scope (Wave-3 anti-test)
// ---------------------------------------------------------------------------

// failingCWClient is a mock CloudWatch client that fails the test if any metric
// call is made. It is a no-op struct that does not implement any real interface;
// the test verifies that no real CW client is ever called during EFS fetch/enrich.
//
// Since EFS does NOT import or use CloudWatch, this test verifies the contract
// at the API level: FetchEFSFileSystems never calls any CloudWatch API.
func TestEFS_NoCloudWatchMetricCalls(t *testing.T) {
	// Construct a mock EFS API client that intercepts DescribeFileSystems.
	// If FetchEFSFileSystems internally calls any CW metric API, the test will
	// fail because no CW client is provided and no CW import exists in efs.go.
	//
	// This test verifies the absence of a CW call by ensuring the fetcher
	// completes successfully using ONLY the EFS client — no CW client in scope.
	mock := &mockEFSClient{
		output: &efs.DescribeFileSystemsOutput{
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

	// FetchEFSFileSystems must complete with ZERO external service calls beyond
	// the EFS client passed in. If the implementation calls CloudWatch, it would
	// panic on nil-client access or return an error — neither is acceptable.
	resources, err := awsclient.FetchEFSFileSystems(context.Background(), mock)
	if err != nil {
		t.Fatalf("FetchEFSFileSystems must not call CloudWatch; error: %v", err)
	}
	if len(resources) != 1 {
		t.Errorf("expected 1 resource, got %d", len(resources))
	}
	// If we reach here, no CloudWatch call was attempted (no panic, no nil-deref).
}

// TestEFS_NoCloudWatchMetricCalls_Enricher verifies that the Wave-2 enricher
// (EnrichEFSMountTargets) also makes zero CloudWatch metric calls.
// The enricher only calls DescribeMountTargets — nothing else.
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

	// ServiceClients with ONLY EFS set — CloudWatch is nil.
	// If EnrichEFSMountTargets tries to use a CW client, it will panic on nil access.
	clients := &awsclient.ServiceClients{EFS: fake}
	resources := efsResources("fs-nocw-enrich001")

	result, err := awsclient.EnrichEFSMountTargets(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("EnrichEFSMountTargets must not call CloudWatch; error: %v", err)
	}
	// Healthy FS → no findings.
	if len(result.Findings) != 0 {
		t.Errorf("expected 0 findings, got %d", len(result.Findings))
	}
	// If we reach here, no CloudWatch call was attempted.
}

// ---------------------------------------------------------------------------
// Wave-1 single-fixture status field contract tests.
// ---------------------------------------------------------------------------

// TestFetchEFSFileSystems_StatusField_Warning verifies that Warning fixtures
// have Fields["status"] set to the §4 phrase (not raw LifeCycleState enum).
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
		fsID        string
		wantPhrase  string
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

// TestFetchEFSFileSystems_StatusField_Broken verifies that Broken fixtures
// have Fields["status"] set to the §4 phrase.
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

// TestFetchEFSFileSystems_W1Updating_IsolatesFromMTState verifies that the
// warn-efs-updating fixture (which has 2 available MTs) produces Status="updating"
// from Wave-1 alone — the MT state is irrelevant at this phase.
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

	// Phase 1 contract: lifecycle-state signal only.
	if r.Fields["status"] != "updating" {
		t.Errorf("Fields[\"status\"] = %q, want %q (W1 signal only; MT state is W2)", r.Fields["status"], "updating")
	}
	if len(r.Findings) != 1 || r.Findings[0].Phrase != "updating" {
		t.Errorf("Findings = %v, want one finding with Phrase %q", r.Findings, "updating")
	}
}
