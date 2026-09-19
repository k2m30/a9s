package unit

import (
	"context"
	"strings"
	"testing"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/domain"
)

func TestEnrichEFSMountTargets_HealthyRowWithDown(t *testing.T) {
	const fsID = "fs-0healthymtdown001"

	fake := efsMTFakeFromFixtures()
	clients := &awsclient.ServiceClients{EFS: fake}

	res := efsResources(fsID)
	res[0].Fields["status"] = ""

	result, err := awsclient.EnrichEFSMountTargets(context.Background(), clients, res, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	findings, ok := result.Findings[fsID]
	if !ok {
		t.Fatalf("expected finding for %q, got none; all findings: %v", fsID, result.Findings)
	}
	finding := findings[0]

	if finding.Severity != domain.SevBroken {
		t.Errorf("Severity = %v, want SevBroken", finding.Severity)
	}

	if finding.Phrase != "mount target down" {
		t.Errorf("Phrase = %q, want %q", finding.Phrase, "mount target down")
	}

	if len(finding.Phrase) > 40 {
		t.Errorf("Phrase length %d > 40 chars: %q", len(finding.Phrase), finding.Phrase)
	}

	// FieldUpdates must be empty — the merged display phrase is
	// computed by phraseFromFindings(r.Findings) at render time.
	if updates, hasUpdates := result.FieldUpdates[fsID]; hasUpdates && len(updates) != 0 {
		t.Errorf("AS-140: expected empty FieldUpdates for %q (status overlay removed); got %v", fsID, updates)
	}

	wantLabels := []string{"Mount Target", "AZ", "State", "Degraded"}
	rowLabels := make(map[string]string, len(result.AttentionDetails[fsID][finding.Code].Rows))
	for _, row := range result.AttentionDetails[fsID][finding.Code].Rows {
		rowLabels[row.Label] = row.Value
	}
	for _, label := range wantLabels {
		if _, ok := rowLabels[label]; !ok {
			t.Errorf("Rows missing label %q; got rows: %v", label, result.AttentionDetails[fsID][finding.Code].Rows)
		}
	}

	if mtVal := rowLabels["Mount Target"]; !strings.Contains(mtVal, "fsmt-0healthymtdown001b") {
		t.Errorf("Rows[Mount Target] = %q, want it to contain %q", mtVal, "fsmt-0healthymtdown001b")
	}

	if azVal := rowLabels["AZ"]; azVal != "us-east-1b" {
		t.Errorf("Rows[AZ] = %q, want %q", azVal, "us-east-1b")
	}

	if stateVal := rowLabels["State"]; stateVal != "creating" {
		t.Errorf("Rows[State] = %q, want %q", stateVal, "creating")
	}

	if degVal := rowLabels["Degraded"]; degVal != "1/2" {
		t.Errorf("Rows[Degraded] = %q, want %q", degVal, "1/2")
	}

	for _, row := range result.AttentionDetails[fsID][finding.Code].Rows {
		if row.Value != "" && strings.Contains(finding.Phrase, row.Value) {
			t.Errorf("U11 violation: Phrase %q contains Row Value %q (label=%q)", finding.Phrase, row.Value, row.Label)
		}
	}
}

func TestEnrichEFSMountTargets_W1WarningPlusW2Bumps(t *testing.T) {
	const fsID = "fs-0warnupdmtdown001"

	fake := efsMTFakeFromFixtures()
	clients := &awsclient.ServiceClients{EFS: fake}

	res := efsResources(fsID)
	res[0].Fields["status"] = "updating"

	result, err := awsclient.EnrichEFSMountTargets(context.Background(), clients, res, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, ok := result.Findings[fsID]; !ok {
		t.Fatalf("expected finding for %q, got none", fsID)
	}

	// FieldUpdates must be empty. The W1+W2 stack merge to
	// "mount target down (+1)" happens at render time via
	// phraseFromFindings(r.Findings) on the unified findings slice.
	if updates, hasUpdates := result.FieldUpdates[fsID]; hasUpdates && len(updates) != 0 {
		t.Errorf("AS-140: expected empty FieldUpdates for %q (status overlay removed); got %v", fsID, updates)
	}

	finding := result.Findings[fsID][0]
	if finding.Phrase != "mount target down" {
		t.Errorf("Phrase = %q, want %q", finding.Phrase, "mount target down")
	}

	rowLabels := make(map[string]string, len(result.AttentionDetails[fsID][finding.Code].Rows))
	for _, row := range result.AttentionDetails[fsID][finding.Code].Rows {
		rowLabels[row.Label] = row.Value
	}
	if mtVal := rowLabels["Mount Target"]; !strings.Contains(mtVal, fixtures.UpdatingMTDownMountTargetBID) {
		t.Errorf("Rows[Mount Target] = %q, want it to contain %q", mtVal, fixtures.UpdatingMTDownMountTargetBID)
	}

	if degVal := rowLabels["Degraded"]; degVal != "1/2" {
		t.Errorf("Rows[Degraded] = %q, want %q", degVal, "1/2")
	}
}

func TestEnrichEFSMountTargets_AllHealthyMounts_NoFinding(t *testing.T) {
	fake := efsMTFakeFromFixtures()
	clients := &awsclient.ServiceClients{EFS: fake}

	res := efsResources(fixtures.ProdEFSID)
	res[0].Fields["status"] = ""

	result, err := awsclient.EnrichEFSMountTargets(context.Background(), clients, res, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, ok := result.Findings[fixtures.ProdEFSID]; ok {
		t.Errorf("unexpected finding for %q — all 3 MTs are available", fixtures.ProdEFSID)
	}
}

func TestEnrichEFSMountTargets_SummaryDoesNotContainRowValues(t *testing.T) {
	fake := efsMTFakeFromFixtures()
	clients := &awsclient.ServiceClients{EFS: fake}

	findingFSIDs := []string{
		"fs-0healthymtdown001",
		"fs-0warnupdmtdown001",
	}

	res := efsResources(findingFSIDs...)

	result, err := awsclient.EnrichEFSMountTargets(context.Background(), clients, res, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, fsID := range findingFSIDs {
		findings, ok := result.Findings[fsID]
		if !ok {
			t.Errorf("expected finding for %q, got none", fsID)
			continue
		}
		finding := findings[0]
		for _, row := range result.AttentionDetails[fsID][finding.Code].Rows {
			if row.Value == "" {
				continue
			}
			if strings.Contains(finding.Phrase, row.Value) {
				t.Errorf("U11 violation for %q: Phrase %q contains Row[%q].Value %q",
					fsID, finding.Phrase, row.Label, row.Value)
			}
		}
	}
}

func TestEnrichEFSMountTargets_FindingRowsStructure(t *testing.T) {
	const fsID = "fs-0healthymtdown001"

	fake := efsMTFakeFromFixtures()
	clients := &awsclient.ServiceClients{EFS: fake}

	res := efsResources(fsID)

	result, err := awsclient.EnrichEFSMountTargets(context.Background(), clients, res, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	findings, ok := result.Findings[fsID]
	if !ok {
		t.Fatalf("expected finding for %q", fsID)
	}
	finding := findings[0]

	for i, row := range result.AttentionDetails[fsID][finding.Code].Rows {
		if row.Label == "" {
			t.Errorf("Rows[%d].Label is empty", i)
		}
	}

	tierMap := make(map[string]string)
	for _, row := range result.AttentionDetails[fsID][finding.Code].Rows {
		tierMap[row.Label] = row.Tier
	}

	if tier := tierMap["Mount Target"]; tier != "!" {
		t.Errorf("Rows[Mount Target].Tier = %q, want %q", tier, "!")
	}
	if tier := tierMap["State"]; tier != "!" {
		t.Errorf("Rows[State].Tier = %q, want %q", tier, "!")
	}
}

func TestEnrichEFSMountTargets_FieldUpdates_EmptyAS140(t *testing.T) {
	fake := efsMTFakeFromFixtures()
	clients := &awsclient.ServiceClients{EFS: fake}

	res := efsResources("fs-0healthymtdown001")

	result, err := awsclient.EnrichEFSMountTargets(context.Background(), clients, res, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if updates, ok := result.FieldUpdates["fs-0healthymtdown001"]; ok && len(updates) != 0 {
		t.Errorf("AS-140: expected empty FieldUpdates for finding-producing FS; got %v", updates)
	}
}

// Two MT-down file systems enriched together each keep their own "!" finding
// through the concurrent per-resource walk.

func TestEnrichEFSMountTargets_BothMTDownFixtures_ProduceSevBrokenFindings(t *testing.T) {
	fake := efsMTFakeFromFixtures()
	clients := &awsclient.ServiceClients{EFS: fake}

	findingFSIDs := []string{
		"fs-0healthymtdown001",
		"fs-0warnupdmtdown001",
	}
	res := efsResources(findingFSIDs...)
	if len(res) < 2 {
		t.Fatalf("efsResources returned %d resources, expected %d (one per ID)", len(res), len(findingFSIDs))
	}
	res[1].Fields["status"] = "updating"

	result, err := awsclient.EnrichEFSMountTargets(context.Background(), clients, res, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Findings) != 2 {
		t.Errorf("len(Findings) = %d, want 2 (both MT-down fixtures)", len(result.Findings))
	}
	for _, fsID := range findingFSIDs {
		fs, ok := result.Findings[fsID]
		if !ok || len(fs) == 0 {
			t.Errorf("expected a finding keyed by %q", fsID)
			continue
		}
		if fs[0].Severity != domain.SevBroken {
			t.Errorf("Findings[%q][0].Severity = %v, want SevBroken (mount target down)", fsID, fs[0].Severity)
		}
	}
}

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

func TestEnrichEFSMountTargets_DetailContent_U7c(t *testing.T) {
	const fsID = "fs-0warnupdmtdown001"

	fake := efsMTFakeFromFixtures()
	clients := &awsclient.ServiceClients{EFS: fake}

	res := efsResources(fsID)
	res[0].Fields["status"] = "updating"

	result, err := awsclient.EnrichEFSMountTargets(context.Background(), clients, res, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	findings, ok := result.Findings[fsID]
	if !ok {
		t.Fatalf("expected finding for %q", fsID)
	}
	finding := findings[0]

	rowMap := make(map[string]string, len(result.AttentionDetails[fsID][finding.Code].Rows))
	for _, row := range result.AttentionDetails[fsID][finding.Code].Rows {
		rowMap[row.Label] = row.Value
	}

	if azVal := rowMap["AZ"]; azVal != "us-east-1b" {
		t.Errorf("Rows[AZ] = %q, want %q (S5 must show the degraded AZ)", azVal, "us-east-1b")
	}

	if stateVal := rowMap["State"]; stateVal != "creating" {
		t.Errorf("Rows[State] = %q, want %q (S5 must show the MT's lifecycle state)", stateVal, "creating")
	}

	if degVal := rowMap["Degraded"]; degVal != "1/2" {
		t.Errorf("Rows[Degraded] = %q, want %q", degVal, "1/2")
	}
}
