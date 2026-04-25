// aws_efs_issue_enrichment_test.go — Wave-2 enricher behavioral tests for EFS.
//
// Tests the CONTRACT from docs/resources/efs-impl-plan.md §1 Wave-2,
// U7b, U7c, U7e (detail content), U11. Phase 7 coder will make these pass.
//
// Covered invariants:
//   - TestEnrichEFSMountTargets_HealthyRowWithDown — healthy FS + MT-B creating:
//     Summary="mount target down", FieldUpdates[id]["status"]="mount target down",
//     Rows include {MountTarget,AZ,State,Degraded}, U11 (Summary ≠ Row values).
//   - TestEnrichEFSMountTargets_W1WarningPlusW2Bumps — W1 "updating" + W2 "mount target down":
//     FieldUpdates[id]["status"]="mount target down (+1)".
//   - TestEnrichEFSMountTargets_AllHealthyMounts_NoFinding — graph-root 3 MTs all available:
//     no finding produced for ProdEFSID.
//   - TestEnrichEFSMountTargets_SummaryDoesNotContainRowValues — U11 pin.
package unit

import (
	"context"
	"strings"
	"testing"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo/fixtures"
)

// Shared helper efsMTFakeFromFixtures lives in helpers_efs_test.go.

// ---------------------------------------------------------------------------
// TEST: TestEnrichEFSMountTargets_HealthyRowWithDown
//
// GIVEN: healthy-efs-with-mt-down fixture (available FS, MT-A available, MT-B creating).
// THEN:
//   - Enricher produces ONE finding for fs-0healthymtdown001.
//   - Severity = "!"
//   - Summary = "mount target down" (exact §4 phrase; ≤ 40 chars)
//   - FieldUpdates[id]["status"] = "mount target down" (no suffix — single finding)
//   - Rows contain: {Mount Target, AZ, State, Degraded}
//   - U11: Summary must NOT contain any Row Value as substring.
// ---------------------------------------------------------------------------

func TestEnrichEFSMountTargets_HealthyRowWithDown(t *testing.T) {
	const fsID = "fs-0healthymtdown001"

	fake := efsMTFakeFromFixtures()
	clients := &awsclient.ServiceClients{EFS: fake}

	// Simulate a healthy fetcher result (Status="", Issues=[]) for this FS.
	// Only the Wave-2 enricher fires — the FS itself is available but MT-B is creating.
	res := efsResources(fsID)
	res[0].Status = ""
	res[0].Issues = nil

	result, err := awsclient.EnrichEFSMountTargets(context.Background(), clients, res, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Must have exactly one finding keyed by fsID.
	finding, ok := result.Findings[fsID]
	if !ok {
		t.Fatalf("expected finding for %q, got none; all findings: %v", fsID, result.Findings)
	}

	// Severity must be "!".
	if finding.Severity != "!" {
		t.Errorf("Severity = %q, want %q", finding.Severity, "!")
	}

	// Summary must be exactly "mount target down" (§4 "list text" phrase).
	if finding.Summary != "mount target down" {
		t.Errorf("Summary = %q, want %q", finding.Summary, "mount target down")
	}

	// Summary must be ≤ 40 chars.
	if len(finding.Summary) > 40 {
		t.Errorf("Summary length %d > 40 chars: %q", len(finding.Summary), finding.Summary)
	}

	// FieldUpdates must write status = "mount target down" (no suffix — single W2 finding).
	updates, hasUpdates := result.FieldUpdates[fsID]
	if !hasUpdates {
		t.Fatalf("FieldUpdates missing entry for %q", fsID)
	}
	if updates["status"] != "mount target down" {
		t.Errorf("FieldUpdates[%q][\"status\"] = %q, want %q", fsID, updates["status"], "mount target down")
	}

	// Rows must contain the four expected labels.
	wantLabels := []string{"Mount Target", "AZ", "State", "Degraded"}
	rowLabels := make(map[string]string, len(finding.Rows))
	for _, row := range finding.Rows {
		rowLabels[row.Label] = row.Value
	}
	for _, label := range wantLabels {
		if _, ok := rowLabels[label]; !ok {
			t.Errorf("Rows missing label %q; got rows: %v", label, finding.Rows)
		}
	}

	// The down MT-B must appear in the Mount Target row.
	if mtVal := rowLabels["Mount Target"]; !strings.Contains(mtVal, "fsmt-0healthymtdown001b") {
		t.Errorf("Rows[Mount Target] = %q, want it to contain %q", mtVal, "fsmt-0healthymtdown001b")
	}

	// AZ row must show us-east-1b (the down MT's zone).
	if azVal := rowLabels["AZ"]; azVal != "us-east-1b" {
		t.Errorf("Rows[AZ] = %q, want %q", azVal, "us-east-1b")
	}

	// State row must show "creating" (the down MT's LifeCycleState).
	if stateVal := rowLabels["State"]; stateVal != "creating" {
		t.Errorf("Rows[State] = %q, want %q", stateVal, "creating")
	}

	// Degraded row must be "1/2" (1 of 2 MTs unavailable).
	if degVal := rowLabels["Degraded"]; degVal != "1/2" {
		t.Errorf("Rows[Degraded] = %q, want %q", degVal, "1/2")
	}

	// U11: Summary must NOT contain any Row Value as substring.
	for _, row := range finding.Rows {
		if row.Value != "" && strings.Contains(finding.Summary, row.Value) {
			t.Errorf("U11 violation: Summary %q contains Row Value %q (label=%q)", finding.Summary, row.Value, row.Label)
		}
	}
}

// ---------------------------------------------------------------------------
// TEST: TestEnrichEFSMountTargets_W1WarningPlusW2Bumps (U7b)
//
// GIVEN: warn-efs-updating-mt-down fixture (LifeCycleState="updating", MT-A avail, MT-B creating).
// THEN:
//   - Fetcher sets Status="updating", Issues=["updating"].
//   - Enricher: W2 Broken > W1 Warning in severity.
//   - FieldUpdates[id]["status"] = "mount target down (+1)"
//     (+1 hidden = the W1 "updating" phrase).
//   - Resource.Issues (Wave-1 only) stays = ["updating"].
// ---------------------------------------------------------------------------

func TestEnrichEFSMountTargets_W1WarningPlusW2Bumps(t *testing.T) {
	const fsID = "fs-0warnupdmtdown001"

	fake := efsMTFakeFromFixtures()
	clients := &awsclient.ServiceClients{EFS: fake}

	// Resource represents the fetcher output for this W1-Warning fixture.
	res := efsResources(fsID)
	res[0].Status = "updating"
	res[0].Issues = []string{"updating"}

	result, err := awsclient.EnrichEFSMountTargets(context.Background(), clients, res, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Must have a finding.
	if _, ok := result.Findings[fsID]; !ok {
		t.Fatalf("expected finding for %q, got none", fsID)
	}

	// FieldUpdates[id]["status"] must be "mount target down (+1)":
	//   - W2 Broken phrase wins over W1 Warning.
	//   - "+1" counts the hidden W1 Warning ("updating").
	updates, hasUpdates := result.FieldUpdates[fsID]
	if !hasUpdates {
		t.Fatalf("FieldUpdates missing entry for %q", fsID)
	}
	wantStatus := "mount target down (+1)"
	if updates["status"] != wantStatus {
		t.Errorf("FieldUpdates[%q][\"status\"] = %q, want %q", fsID, updates["status"], wantStatus)
	}

	// The finding's Summary is still "mount target down" (the bare W2 phrase).
	finding := result.Findings[fsID]
	if finding.Summary != "mount target down" {
		t.Errorf("Summary = %q, want %q", finding.Summary, "mount target down")
	}

	// The down MT-B must appear in Rows[Mount Target].
	rowLabels := make(map[string]string, len(finding.Rows))
	for _, row := range finding.Rows {
		rowLabels[row.Label] = row.Value
	}
	if mtVal := rowLabels["Mount Target"]; !strings.Contains(mtVal, fixtures.UpdatingMTDownMountTargetBID) {
		t.Errorf("Rows[Mount Target] = %q, want it to contain %q", mtVal, fixtures.UpdatingMTDownMountTargetBID)
	}

	// Degraded row: 1 of 2 MTs unavailable.
	if degVal := rowLabels["Degraded"]; degVal != "1/2" {
		t.Errorf("Rows[Degraded] = %q, want %q", degVal, "1/2")
	}
}

// ---------------------------------------------------------------------------
// TEST: TestEnrichEFSMountTargets_AllHealthyMounts_NoFinding
//
// GIVEN: graph-root fixture prod-efs-app-data (3 MTs, all available).
// THEN: no finding produced for ProdEFSID.
// ---------------------------------------------------------------------------

func TestEnrichEFSMountTargets_AllHealthyMounts_NoFinding(t *testing.T) {
	fake := efsMTFakeFromFixtures()
	clients := &awsclient.ServiceClients{EFS: fake}

	res := efsResources(fixtures.ProdEFSID)
	res[0].Status = ""
	res[0].Issues = nil

	result, err := awsclient.EnrichEFSMountTargets(context.Background(), clients, res, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, ok := result.Findings[fixtures.ProdEFSID]; ok {
		t.Errorf("unexpected finding for %q — all 3 MTs are available", fixtures.ProdEFSID)
	}
	if result.IssueCount != 0 {
		t.Errorf("IssueCount = %d, want 0 (all healthy)", result.IssueCount)
	}
}

// ---------------------------------------------------------------------------
// TEST: TestEnrichEFSMountTargets_SummaryDoesNotContainRowValues (U11 pin)
//
// Runs the enricher against BOTH fixtures that produce a finding and verifies
// that Summary never contains any Row Value as a substring.
// ---------------------------------------------------------------------------

func TestEnrichEFSMountTargets_SummaryDoesNotContainRowValues(t *testing.T) {
	fake := efsMTFakeFromFixtures()
	clients := &awsclient.ServiceClients{EFS: fake}

	// Both finding-producing fixtures.
	findingFSIDs := []string{
		"fs-0healthymtdown001",  // W2 on Healthy
		"fs-0warnupdmtdown001",  // W1 Warning + W2
	}

	res := efsResources(findingFSIDs...)

	result, err := awsclient.EnrichEFSMountTargets(context.Background(), clients, res, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, fsID := range findingFSIDs {
		finding, ok := result.Findings[fsID]
		if !ok {
			t.Errorf("expected finding for %q, got none", fsID)
			continue
		}
		for _, row := range finding.Rows {
			if row.Value == "" {
				continue
			}
			if strings.Contains(finding.Summary, row.Value) {
				t.Errorf("U11 violation for %q: Summary %q contains Row[%q].Value %q",
					fsID, finding.Summary, row.Label, row.Value)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// TEST: TestEnrichEFSMountTargets_FindingRowsStructure
//
// Verifies that every row in the finding has non-empty Label and non-empty Value
// for the finding-producing fixtures. Also verifies the Tier on specific rows.
// ---------------------------------------------------------------------------

func TestEnrichEFSMountTargets_FindingRowsStructure(t *testing.T) {
	const fsID = "fs-0healthymtdown001"

	fake := efsMTFakeFromFixtures()
	clients := &awsclient.ServiceClients{EFS: fake}

	res := efsResources(fsID)

	result, err := awsclient.EnrichEFSMountTargets(context.Background(), clients, res, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	finding, ok := result.Findings[fsID]
	if !ok {
		t.Fatalf("expected finding for %q", fsID)
	}

	// Every row must have a non-empty Label.
	for i, row := range finding.Rows {
		if row.Label == "" {
			t.Errorf("Rows[%d].Label is empty", i)
		}
	}

	// "Mount Target" row must have Tier="!".
	// "State" row must have Tier="!".
	// Other rows (AZ, Degraded) may have Tier="" (neutral context).
	tierMap := make(map[string]string)
	for _, row := range finding.Rows {
		tierMap[row.Label] = row.Tier
	}

	if tier := tierMap["Mount Target"]; tier != "!" {
		t.Errorf("Rows[Mount Target].Tier = %q, want %q", tier, "!")
	}
	if tier := tierMap["State"]; tier != "!" {
		t.Errorf("Rows[State].Tier = %q, want %q", tier, "!")
	}
}

// ---------------------------------------------------------------------------
// TEST: TestEnrichEFSMountTargets_FieldUpdates_NotNil
//
// Verifies that FieldUpdates is never nil in the result (contract: MUST NOT be
// nil if the enricher writes any updates).
// ---------------------------------------------------------------------------

func TestEnrichEFSMountTargets_FieldUpdates_NotNil(t *testing.T) {
	fake := efsMTFakeFromFixtures()
	clients := &awsclient.ServiceClients{EFS: fake}

	// Run with a finding-producing fixture.
	res := efsResources("fs-0healthymtdown001")

	result, err := awsclient.EnrichEFSMountTargets(context.Background(), clients, res, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.FieldUpdates == nil {
		t.Error("FieldUpdates must not be nil when enricher produces findings")
	}
}

// ---------------------------------------------------------------------------
// TEST: TestEnrichEFSMountTargets_IssueCount
//
// Verifies IssueCount reflects "!" findings: two fixtures produce findings
// (healthy-mt-down and updating-mt-down). Non-finding fixtures produce 0.
// ---------------------------------------------------------------------------

func TestEnrichEFSMountTargets_IssueCount(t *testing.T) {
	fake := efsMTFakeFromFixtures()
	clients := &awsclient.ServiceClients{EFS: fake}

	// Two finding-producing fixtures.
	findingFSIDs := []string{
		"fs-0healthymtdown001",
		"fs-0warnupdmtdown001",
	}
	res := efsResources(findingFSIDs...)
	if len(res) < 2 {
		t.Fatalf("efsResources returned %d resources, expected %d (one per ID)", len(res), len(findingFSIDs))
	}
	res[1].Status = "updating"
	res[1].Issues = []string{"updating"}

	result, err := awsclient.EnrichEFSMountTargets(context.Background(), clients, res, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.IssueCount != 2 {
		t.Errorf("IssueCount = %d, want 2 (both MT-down fixtures produce ! findings)", result.IssueCount)
	}
}

// ---------------------------------------------------------------------------
// TEST: TestEnrichEFSMountTargets_HealthyFSProducesNoFieldUpdates
//
// Verifies that the graph-root (all MTs available) produces no FieldUpdates
// entry — the status field must not be written for healthy resources.
// ---------------------------------------------------------------------------

func TestEnrichEFSMountTargets_HealthyFSProducesNoFieldUpdates(t *testing.T) {
	fake := efsMTFakeFromFixtures()
	clients := &awsclient.ServiceClients{EFS: fake}

	res := efsResources(fixtures.ProdEFSID)

	result, err := awsclient.EnrichEFSMountTargets(context.Background(), clients, res, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if updates, ok := result.FieldUpdates[fixtures.ProdEFSID]; ok && len(updates) > 0 {
		t.Errorf("FieldUpdates[%q] = %v, want none (all MTs healthy)", fixtures.ProdEFSID, updates)
	}
}

// ---------------------------------------------------------------------------
// Inline test for W2 enricher Rows content — U7c (S5 surface).
// Verifies that the finding Rows include the AZ and State values that would
// render in the Attention (S5) detail section.
// ---------------------------------------------------------------------------

// TestEnrichEFSMountTargets_DetailContent_U7c verifies that the W2 finding on
// warn-efs-updating-mt-down carries the rows that detail view (S5) would render.
func TestEnrichEFSMountTargets_DetailContent_U7c(t *testing.T) {
	const fsID = "fs-0warnupdmtdown001"

	fake := efsMTFakeFromFixtures()
	clients := &awsclient.ServiceClients{EFS: fake}

	res := efsResources(fsID)
	res[0].Status = "updating"
	res[0].Issues = []string{"updating"}

	result, err := awsclient.EnrichEFSMountTargets(context.Background(), clients, res, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	finding, ok := result.Findings[fsID]
	if !ok {
		t.Fatalf("expected finding for %q", fsID)
	}

	rowMap := make(map[string]string, len(finding.Rows))
	for _, row := range finding.Rows {
		rowMap[row.Label] = row.Value
	}

	// S5 must contain AZ row (us-east-1b — the down MT's AZ).
	if azVal := rowMap["AZ"]; azVal != "us-east-1b" {
		t.Errorf("Rows[AZ] = %q, want %q (S5 must show the degraded AZ)", azVal, "us-east-1b")
	}

	// S5 must contain State row ("creating" — the non-available lifecycle state).
	if stateVal := rowMap["State"]; stateVal != "creating" {
		t.Errorf("Rows[State] = %q, want %q (S5 must show the MT's lifecycle state)", stateVal, "creating")
	}

	// S5 must contain Degraded counter.
	if degVal := rowMap["Degraded"]; degVal != "1/2" {
		t.Errorf("Rows[Degraded] = %q, want %q", degVal, "1/2")
	}
}

