package unit

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

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
	return &dynamodb.DescribeContinuousBackupsOutput{
		ContinuousBackupsDescription: &ddbtypes.ContinuousBackupsDescription{
			PointInTimeRecoveryDescription: &ddbtypes.PointInTimeRecoveryDescription{
				PointInTimeRecoveryStatus: ddbtypes.PointInTimeRecoveryStatusEnabled,
			},
		},
	}, nil
}

func ddbPITREnabledOutput() *dynamodb.DescribeContinuousBackupsOutput {
	return &dynamodb.DescribeContinuousBackupsOutput{
		ContinuousBackupsDescription: &ddbtypes.ContinuousBackupsDescription{
			PointInTimeRecoveryDescription: &ddbtypes.PointInTimeRecoveryDescription{
				PointInTimeRecoveryStatus: ddbtypes.PointInTimeRecoveryStatusEnabled,
			},
		},
	}
}

func ddbPITRDisabledOutput() *dynamodb.DescribeContinuousBackupsOutput {
	return &dynamodb.DescribeContinuousBackupsOutput{
		ContinuousBackupsDescription: &ddbtypes.ContinuousBackupsDescription{
			PointInTimeRecoveryDescription: &ddbtypes.PointInTimeRecoveryDescription{
				PointInTimeRecoveryStatus: ddbtypes.PointInTimeRecoveryStatusDisabled,
			},
		},
	}
}

func buildDDBEnricherClients(fake awsclient.DynamoDBAPI) *awsclient.ServiceClients {
	return &awsclient.ServiceClients{DynamoDB: fake}
}

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
}

func TestDDB_Enrich_PITRDisabled_HealthyRow(t *testing.T) {
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

	findings, ok := result.Findings[fixtures.AuditPITROffID]
	if !ok {
		t.Fatalf("expected finding for %q (PITR disabled); Findings keys = %v", fixtures.AuditPITROffID, findingKeysDDB(result.Findings))
	}
	finding := findings[0]
	if finding.Severity != domain.SevWarn {
		t.Errorf("Severity = %v, want SevWarn", finding.Severity)
	}
	if finding.Phrase != "point-in-time recovery disabled" {
		t.Errorf("Phrase = %q, want %q", finding.Phrase, "point-in-time recovery disabled")
	}

	// FieldUpdates must be empty for this resource — the merged
	// display phrase is computed at render time by phraseFromFindings.
	if updates, ok := result.FieldUpdates[fixtures.AuditPITROffID]; ok && len(updates) != 0 {
		t.Errorf("AS-140: expected empty FieldUpdates for %q (status overlay removed); got %v", fixtures.AuditPITROffID, updates)
	}

}

func TestDDB_Enrich_PITRDisabled_NonHealthyRow(t *testing.T) {
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

	findings, ok := result.Findings[fixtures.LegacyArchivedID]
	if !ok {
		t.Fatalf("expected finding for %q (ARCHIVED + PITR off); Findings keys = %v", fixtures.LegacyArchivedID, findingKeysDDB(result.Findings))
	}
	finding := findings[0]
	if finding.Severity != domain.SevWarn {
		t.Errorf("Severity = %v, want SevWarn", finding.Severity)
	}
	if finding.Phrase != "point-in-time recovery disabled" {
		t.Errorf("Phrase = %q, want %q", finding.Phrase, "point-in-time recovery disabled")
	}

	// FieldUpdates must be empty for this resource — the merged
	// Wave-1 + Wave-2 display phrase is computed at render time by
	// phraseFromFindings(r.Findings).
	if updates, ok := result.FieldUpdates[fixtures.LegacyArchivedID]; ok && len(updates) != 0 {
		t.Errorf("AS-140: expected empty FieldUpdates for %q (status overlay removed); got %v", fixtures.LegacyArchivedID, updates)
	}

}

// Summary and Rows are distinct channels: no Row value repeats inside the
// Summary.
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

	findings, ok := result.Findings[fixtures.AuditPITROffID]
	if !ok {
		t.Fatalf("expected finding for %q", fixtures.AuditPITROffID)
	}
	finding := findings[0]

	if finding.Phrase != "point-in-time recovery disabled" {
		t.Errorf("Phrase = %q, want exactly %q", finding.Phrase, "point-in-time recovery disabled")
	}
	for _, row := range result.AttentionDetails[fixtures.AuditPITROffID][finding.Code].Rows {
		if row.Value != "" && strings.Contains(finding.Phrase, row.Value) {
			t.Errorf("Phrase %q embeds Row[%q].Value %q — Phrase and Rows must be distinct channels (U11)", finding.Phrase, row.Label, row.Value)
		}
	}
}

// ddb is a "~"-only enricher (IssueCount always 0), so a per-row coverage gap
// never lower-bounds the issue badge.
func TestDDB_Enrich_ErrorPath_TruncatedIDNotBadge(t *testing.T) {
	errorTableID := fixtures.AuditPITROffID
	fake := &ddbContinuousBackupsFake{
		errTables: map[string]bool{errorTableID: true},
	}
	clients := buildDDBEnricherClients(fake)
	resources := []resource.Resource{makeDDBResource(errorTableID, "")}

	result, err := awsclient.EnrichDynamoDBPITR(context.Background(), clients, resources, nil)
	// The failed call is recorded on the error; a nil error here is a silent
	// skip, the row marked uninspected with the reason dropped on the floor.
	if err == nil {
		t.Fatal("a failed DescribeContinuousBackups returned no error — the reason never reaches the log")
	}
	if !strings.Contains(err.Error(), errorTableID) {
		t.Errorf("failure line %q does not name the table it could not read", err)
	}

	if result.Truncated {
		t.Errorf("Truncated = true, want false: ddb is a \"~\"-only enricher, so a sub-call error marks the row via TruncatedIDs, never the aggregate issue badge")
	}
	if _, marked := result.TruncatedIDs[errorTableID]; !marked {
		t.Errorf("TruncatedIDs[%q] = false, want true (error on this table)", errorTableID)
	}
	if _, ok := result.Findings[errorTableID]; ok {
		t.Errorf("unexpected finding for %q when DescribeContinuousBackups errored", errorTableID)
	}
}

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

	if _, ok := result.Findings[id]; !ok {
		t.Errorf("expected PITR-off Finding for %q even when row carries stacked Wave-1 phrase", id)
	}
	if updates, ok := result.FieldUpdates[id]; ok && len(updates) != 0 {
		t.Errorf("AS-140: expected empty FieldUpdates for %q (status overlay removed); got %v", id, updates)
	}
}

func TestDDB_Enrich_NilDynamoDBClient(t *testing.T) {
	clients := &awsclient.ServiceClients{DynamoDB: nil}
	result, err := awsclient.EnrichDynamoDBPITR(context.Background(), clients, nil, nil)
	if err != nil {
		t.Fatalf("EnrichDynamoDBPITR error with nil client: %v", err)
	}
	if result.Findings == nil {
		t.Error("Findings must not be nil even when DynamoDB client is nil")
	}
	if len(result.Findings) != 0 {
		t.Errorf("len(Findings) = %d, want 0 (nil DynamoDB client, no resources)", len(result.Findings))
	}
}

func findingKeysDDB(m map[string][]domain.Finding) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// Suppress unused import warning for aws package.
var _ = aws.String
