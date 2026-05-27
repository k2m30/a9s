package unit

// aws_ddb_issue_enrichment_test.go — Wave 2 PITR enricher tests for ddb.
//
// Tests drive EnrichDynamoDBPITR and assert:
//   - PITR enabled  → no finding, no FieldUpdates["status"].
//   - PITR disabled on Healthy row → Findings[id].Severity == "~",
//     Findings[id].Summary == "PITR off". AS-140 (W1.2 of AS-1390): no
//     FieldUpdates["status"] is written — the merged display phrase is
//     computed at render time by phraseFromFindings(r.Findings).
//   - PITR disabled on non-Healthy row (e.g. "archived: kms key lost") →
//     same as above; no (+N) suffix is applied by this enricher.
//   - Summary/Rows contract (U11): Summary is "PITR off"; Row values must
//     NOT be substrings of Summary.
//   - Error path: DescribeContinuousBackups errors → table skipped,
//     Truncated=true, TruncatedIDs[id]=true.
//   - IssueCount == 0 for every case ("~" does not bump S1 badge).

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo/fixtures"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ---------------------------------------------------------------------------
// mock — DynamoDB DescribeContinuousBackups stub
// ---------------------------------------------------------------------------

// ddbContinuousBackupsFake implements DynamoDBDescribeContinuousBackupsAPI.
// It returns responses keyed by table name; errTables causes an error return
// for the named table (adversarial path).
type ddbContinuousBackupsFake struct {
	awsclient.DynamoDBAPI
	responses map[string]*dynamodb.DescribeContinuousBackupsOutput
	errTables map[string]bool
}

func (f *ddbContinuousBackupsFake) DescribeContinuousBackups(
	_ context.Context,
	in *dynamodb.DescribeContinuousBackupsInput,
	_ ...func(*dynamodb.Options),
) (*dynamodb.DescribeContinuousBackupsOutput, error) {
	if in == nil || in.TableName == nil {
		return nil, fmt.Errorf("nil input")
	}
	name := *in.TableName
	if f.errTables != nil && f.errTables[name] {
		return nil, fmt.Errorf("simulated DescribeContinuousBackups error for %s", name)
	}
	if resp, ok := f.responses[name]; ok {
		return resp, nil
	}
	// Default: PITR enabled (healthy baseline — tables not in map are not failing).
	return &dynamodb.DescribeContinuousBackupsOutput{
		ContinuousBackupsDescription: &ddbtypes.ContinuousBackupsDescription{
			PointInTimeRecoveryDescription: &ddbtypes.PointInTimeRecoveryDescription{
				PointInTimeRecoveryStatus: ddbtypes.PointInTimeRecoveryStatusEnabled,
			},
		},
	}, nil
}

// ddbPITREnabledOutput returns a DescribeContinuousBackupsOutput with PITR enabled.
func ddbPITREnabledOutput() *dynamodb.DescribeContinuousBackupsOutput {
	return &dynamodb.DescribeContinuousBackupsOutput{
		ContinuousBackupsDescription: &ddbtypes.ContinuousBackupsDescription{
			PointInTimeRecoveryDescription: &ddbtypes.PointInTimeRecoveryDescription{
				PointInTimeRecoveryStatus: ddbtypes.PointInTimeRecoveryStatusEnabled,
			},
		},
	}
}

// ddbPITRDisabledOutput returns a DescribeContinuousBackupsOutput with PITR disabled.
func ddbPITRDisabledOutput() *dynamodb.DescribeContinuousBackupsOutput {
	return &dynamodb.DescribeContinuousBackupsOutput{
		ContinuousBackupsDescription: &ddbtypes.ContinuousBackupsDescription{
			PointInTimeRecoveryDescription: &ddbtypes.PointInTimeRecoveryDescription{
				PointInTimeRecoveryStatus: ddbtypes.PointInTimeRecoveryStatusDisabled,
			},
		},
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// buildDDBEnricherClients wraps the fake as a ServiceClients.DynamoDB.
func buildDDBEnricherClients(fake awsclient.DynamoDBAPI) *awsclient.ServiceClients {
	return &awsclient.ServiceClients{DynamoDB: fake}
}

// makeDDBResource constructs a minimal Resource matching what FetchDynamoDBTablesPage
// would produce for the given table id and pre-enrichment status.
func makeDDBResource(id string, status string) resource.Resource {
	arn := "arn:aws:dynamodb:us-east-1:123456789012:table/" + id
	return resource.Resource{
		ID:   id,
		Name: id,
		Fields: map[string]string{
			"status": status,
			"arn":    arn,
		},
	}
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// TestDDB_Enrich_PITREnabled_NoFinding verifies that a Healthy table with PITR
// enabled produces no finding and no FieldUpdates["status"] entry.
func TestDDB_Enrich_PITREnabled_NoFinding(t *testing.T) {
	fake := &ddbContinuousBackupsFake{
		responses: map[string]*dynamodb.DescribeContinuousBackupsOutput{
			fixtures.OrdersProdID: ddbPITREnabledOutput(),
		},
	}
	clients := buildDDBEnricherClients(fake)
	resources := []resource.Resource{makeDDBResource(fixtures.OrdersProdID, "")}

	result, err := awsclient.EnrichDynamoDBPITR(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("EnrichDynamoDBPITR error: %v", err)
	}

	if _, ok := result.Findings[fixtures.OrdersProdID]; ok {
		t.Errorf("expected no finding for orders-prod (PITR enabled), got one")
	}
	if updates, ok := result.FieldUpdates[fixtures.OrdersProdID]; ok {
		if updates["status"] != "" {
			t.Errorf("FieldUpdates[orders-prod][status] = %q, want empty (no finding)", updates["status"])
		}
	}
	if result.IssueCount != 0 {
		t.Errorf("IssueCount = %d, want 0 (~ finding does not bump badge)", result.IssueCount)
	}
}

// TestDDB_Enrich_PITRDisabled_HealthyRow verifies a Healthy ACTIVE table with
// PITR disabled produces the correct EnrichmentFinding. AS-140 (W1.2 of
// AS-1390): no FieldUpdates["status"] is written — the display phrase is
// computed at render time by phraseFromFindings(r.Findings).
func TestDDB_Enrich_PITRDisabled_HealthyRow(t *testing.T) {
	fake := &ddbContinuousBackupsFake{
		responses: map[string]*dynamodb.DescribeContinuousBackupsOutput{
			fixtures.AuditPITROffID: ddbPITRDisabledOutput(),
		},
	}
	clients := buildDDBEnricherClients(fake)
	// audit-pitr-off is ACTIVE; fetcher produces Status="" (Healthy silence).
	resources := []resource.Resource{makeDDBResource(fixtures.AuditPITROffID, "")}

	result, err := awsclient.EnrichDynamoDBPITR(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("EnrichDynamoDBPITR error: %v", err)
	}

	finding, ok := result.Findings[fixtures.AuditPITROffID]
	if !ok {
		t.Fatalf("expected finding for %q (PITR disabled); Findings keys = %v", fixtures.AuditPITROffID, findingKeysDDB(result.Findings))
	}
	if finding.Severity != domain.SevWarn {
		t.Errorf("Severity = %v, want SevWarn", finding.Severity)
	}
	if finding.Phrase != "PITR off" {
		t.Errorf("Phrase = %q, want %q", finding.Phrase, "PITR off")
	}

	// AS-140: FieldUpdates must be empty for this resource — the merged
	// display phrase is now computed at render time by phraseFromFindings.
	if updates, ok := result.FieldUpdates[fixtures.AuditPITROffID]; ok && len(updates) != 0 {
		t.Errorf("AS-140: expected empty FieldUpdates for %q (status overlay removed); got %v", fixtures.AuditPITROffID, updates)
	}

	if result.IssueCount != 0 {
		t.Errorf("IssueCount = %d, want 0 (~ severity must not bump S1 badge)", result.IssueCount)
	}
}

// TestDDB_Enrich_PITRDisabled_NonHealthyRow verifies that a table already
// carrying "archived: kms key lost" (Wave-1 fetcher phrase) still gets the
// Wave-2 EnrichmentFinding emitted when PITR is disabled. AS-140 (W1.2 of
// AS-1390): no FieldUpdates["status"] is written and no (+N) suffix is
// applied; the merged display phrase is computed at render time by
// phraseFromFindings(r.Findings).
func TestDDB_Enrich_PITRDisabled_NonHealthyRow(t *testing.T) {
	// legacy-archived: fetcher sets Status="archived: kms key lost"
	existingStatus := "archived: kms key lost"
	fake := &ddbContinuousBackupsFake{
		responses: map[string]*dynamodb.DescribeContinuousBackupsOutput{
			fixtures.LegacyArchivedID: ddbPITRDisabledOutput(),
		},
	}
	clients := buildDDBEnricherClients(fake)
	resources := []resource.Resource{makeDDBResource(fixtures.LegacyArchivedID, existingStatus)}

	result, err := awsclient.EnrichDynamoDBPITR(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("EnrichDynamoDBPITR error: %v", err)
	}

	finding, ok := result.Findings[fixtures.LegacyArchivedID]
	if !ok {
		t.Fatalf("expected finding for %q (ARCHIVED + PITR off); Findings keys = %v", fixtures.LegacyArchivedID, findingKeysDDB(result.Findings))
	}
	if finding.Severity != domain.SevWarn {
		t.Errorf("Severity = %v, want SevWarn", finding.Severity)
	}
	if finding.Phrase != "PITR off" {
		t.Errorf("Phrase = %q, want %q", finding.Phrase, "PITR off")
	}

	// AS-140: FieldUpdates must be empty for this resource — the merged
	// Wave-1 + Wave-2 display phrase is computed at render time by
	// phraseFromFindings(r.Findings).
	if updates, ok := result.FieldUpdates[fixtures.LegacyArchivedID]; ok && len(updates) != 0 {
		t.Errorf("AS-140: expected empty FieldUpdates for %q (status overlay removed); got %v", fixtures.LegacyArchivedID, updates)
	}

	if result.IssueCount != 0 {
		t.Errorf("IssueCount = %d, want 0 (~ must not bump S1 badge)", result.IssueCount)
	}
}

// TestDDB_Enrich_SummaryNotRows_Contract (covers U11): Finding.Summary is
// "PITR off" and for every Row in the finding, the Row.Value must NOT be a
// substring of Summary. This pins the enrichment contract that Summary and
// Rows are distinct channels (no duplication).
func TestDDB_Enrich_SummaryNotRows_Contract(t *testing.T) {
	fake := &ddbContinuousBackupsFake{
		responses: map[string]*dynamodb.DescribeContinuousBackupsOutput{
			fixtures.AuditPITROffID: ddbPITRDisabledOutput(),
		},
	}
	clients := buildDDBEnricherClients(fake)
	resources := []resource.Resource{makeDDBResource(fixtures.AuditPITROffID, "")}

	result, err := awsclient.EnrichDynamoDBPITR(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("EnrichDynamoDBPITR error: %v", err)
	}

	finding, ok := result.Findings[fixtures.AuditPITROffID]
	if !ok {
		t.Fatalf("expected finding for %q", fixtures.AuditPITROffID)
	}

	if finding.Phrase != "PITR off" {
		t.Errorf("Phrase = %q, want exactly %q", finding.Phrase, "PITR off")
	}
	// U11: no Row value should appear in Phrase.
	for _, row := range result.AttentionDetails[fixtures.AuditPITROffID].Rows {
		if row.Value != "" && strings.Contains(finding.Phrase, row.Value) {
			t.Errorf("Phrase %q embeds Row[%q].Value %q — Phrase and Rows must be distinct channels (U11)", finding.Phrase, row.Label, row.Value)
		}
	}
}

// TestDDB_Enrich_ErrorPath_TruncatedID verifies that when DescribeContinuousBackups
// returns an error for a table, that table is skipped, Truncated=true, and
// TruncatedIDs[id]=true.
func TestDDB_Enrich_ErrorPath_TruncatedID(t *testing.T) {
	errorTableID := fixtures.AuditPITROffID
	fake := &ddbContinuousBackupsFake{
		errTables: map[string]bool{errorTableID: true},
	}
	clients := buildDDBEnricherClients(fake)
	resources := []resource.Resource{makeDDBResource(errorTableID, "")}

	result, err := awsclient.EnrichDynamoDBPITR(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("EnrichDynamoDBPITR error: %v", err)
	}

	if !result.Truncated {
		t.Errorf("Truncated = false, want true when a sub-call errors")
	}
	if !result.TruncatedIDs[errorTableID] {
		t.Errorf("TruncatedIDs[%q] = false, want true (error on this table)", errorTableID)
	}
	// The table with an error must NOT have a finding (partial data unusable).
	if _, ok := result.Findings[errorTableID]; ok {
		t.Errorf("unexpected finding for %q when DescribeContinuousBackups errored", errorTableID)
	}
}

// TestDDB_Enrich_PITRDisabled_NoFieldUpdates_WithStackedWave1 verifies AS-140
// (W1.2 of AS-1390): when the existing status already carries a stacked
// Wave-1 phrase (e.g. "kms key inaccessible (+2)"), the enricher must NOT
// write FieldUpdates["status"] — no suffix-bump arithmetic happens here.
// The merged display phrase is computed at render time by
// phraseFromFindings(r.Findings), which aggregates Wave-1 findings on the
// resource with this enricher's Wave-2 "PITR off" finding.
func TestDDB_Enrich_PITRDisabled_NoFieldUpdates_WithStackedWave1(t *testing.T) {
	id := "inline-bump-ddb-test"
	existingStatus := "kms key inaccessible (+2)"
	fake := &ddbContinuousBackupsFake{
		responses: map[string]*dynamodb.DescribeContinuousBackupsOutput{
			id: ddbPITRDisabledOutput(),
		},
	}
	clients := buildDDBEnricherClients(fake)
	resources := []resource.Resource{
		{
			ID:     id,
			Name:   id,
			Fields: map[string]string{"status": existingStatus},
		},
	}

	result, err := awsclient.EnrichDynamoDBPITR(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("EnrichDynamoDBPITR error: %v", err)
	}

	// The Wave-2 PITR finding must still be emitted.
	if _, ok := result.Findings[id]; !ok {
		t.Errorf("expected PITR-off Finding for %q even when row carries stacked Wave-1 phrase", id)
	}
	// AS-140: no FieldUpdates write — no suffix-bump arithmetic from this enricher.
	if updates, ok := result.FieldUpdates[id]; ok && len(updates) != 0 {
		t.Errorf("AS-140: expected empty FieldUpdates for %q (status overlay removed); got %v", id, updates)
	}
}

// TestDDB_Enrich_NilDynamoDBClient verifies nil DynamoDB client returns empty
// result gracefully without error.
func TestDDB_Enrich_NilDynamoDBClient(t *testing.T) {
	clients := &awsclient.ServiceClients{DynamoDB: nil}
	result, err := awsclient.EnrichDynamoDBPITR(context.Background(), clients, nil, nil)
	if err != nil {
		t.Fatalf("EnrichDynamoDBPITR error with nil client: %v", err)
	}
	if result.Findings == nil {
		t.Error("Findings must not be nil even when DynamoDB client is nil")
	}
	if result.IssueCount != 0 {
		t.Errorf("IssueCount = %d, want 0", result.IssueCount)
	}
}

// ---------------------------------------------------------------------------
// internal helpers
// ---------------------------------------------------------------------------

func findingKeysDDB(m map[string]domain.Finding) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// Suppress unused import warning for aws package.
var _ = aws.String
