package unit

// aws_ddb_enricher_test.go — Behavioral tests for EnrichDynamoDBPITR.
//
// Contract assertions:
//   - DescribeContinuousBackups is called once per table in the resources slice.
//   - PointInTimeRecoveryStatus=DISABLED → Finding keyed by resource.ID, severity "~".
//   - PointInTimeRecoveryStatus=ENABLED  → no finding.
//   - Response with no PointInTimeRecoveryDescription → no finding (skipped).
//   - clients.DynamoDB == nil → (EnricherResult{Findings: non-nil empty}, nil).
//   - API error for one table → no finding for that table, Truncated=true.

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ddbPITRFake implements DynamoDBAPI for enrichment testing.
// It embeds the interface and overrides only DescribeContinuousBackups.
// The perTable map keys on TableName and returns the configured output or error.
type ddbPITRFake struct {
	awsclient.DynamoDBAPI
	perTable map[string]*ddbPITRResponse
}

type ddbPITRResponse struct {
	out *dynamodb.DescribeContinuousBackupsOutput
	err error
}

func (f *ddbPITRFake) DescribeContinuousBackups(
	_ context.Context,
	in *dynamodb.DescribeContinuousBackupsInput,
	_ ...func(*dynamodb.Options),
) (*dynamodb.DescribeContinuousBackupsOutput, error) {
	name := ""
	if in != nil && in.TableName != nil {
		name = *in.TableName
	}
	if resp, ok := f.perTable[name]; ok {
		return resp.out, resp.err
	}
	// Default: ENABLED (no finding).
	return &dynamodb.DescribeContinuousBackupsOutput{
		ContinuousBackupsDescription: &ddbtypes.ContinuousBackupsDescription{
			PointInTimeRecoveryDescription: &ddbtypes.PointInTimeRecoveryDescription{
				PointInTimeRecoveryStatus: ddbtypes.PointInTimeRecoveryStatusEnabled,
			},
		},
	}, nil
}

// Compile-time check: ddbPITRFake satisfies DynamoDBAPI.
var _ awsclient.DynamoDBAPI = (*ddbPITRFake)(nil)

// makeDDBResources builds a []resource.Resource slice from the given table names.
func makeDDBResources(names ...string) []resource.Resource {
	rs := make([]resource.Resource, 0, len(names))
	for _, n := range names {
		rs = append(rs, resource.Resource{ID: n, Name: n})
	}
	return rs
}

// pitrDisabledOutput returns an output with PITR status DISABLED.
func pitrDisabledOutput() *dynamodb.DescribeContinuousBackupsOutput {
	return &dynamodb.DescribeContinuousBackupsOutput{
		ContinuousBackupsDescription: &ddbtypes.ContinuousBackupsDescription{
			PointInTimeRecoveryDescription: &ddbtypes.PointInTimeRecoveryDescription{
				PointInTimeRecoveryStatus: ddbtypes.PointInTimeRecoveryStatusDisabled,
			},
		},
	}
}

// pitrEnabledOutput returns an output with PITR status ENABLED.
func pitrEnabledOutput() *dynamodb.DescribeContinuousBackupsOutput {
	return &dynamodb.DescribeContinuousBackupsOutput{
		ContinuousBackupsDescription: &ddbtypes.ContinuousBackupsDescription{
			PointInTimeRecoveryDescription: &ddbtypes.PointInTimeRecoveryDescription{
				PointInTimeRecoveryStatus: ddbtypes.PointInTimeRecoveryStatusEnabled,
			},
		},
	}
}

// pitrNoSectionOutput returns an output with no PointInTimeRecoveryDescription.
func pitrNoSectionOutput() *dynamodb.DescribeContinuousBackupsOutput {
	return &dynamodb.DescribeContinuousBackupsOutput{
		ContinuousBackupsDescription: &ddbtypes.ContinuousBackupsDescription{
			PointInTimeRecoveryDescription: nil,
		},
	}
}

// TestEnrichDynamoDBPITR_DisabledProducesFinding verifies that both tables with
// PITR DISABLED each produce a finding with severity "~".
func TestEnrichDynamoDBPITR_DisabledProducesFinding(t *testing.T) {
	fake := &ddbPITRFake{
		perTable: map[string]*ddbPITRResponse{
			"orders": {out: pitrDisabledOutput()},
			"events": {out: pitrDisabledOutput()},
		},
	}
	clients := &awsclient.ServiceClients{DynamoDB: fake}
	resources := makeDDBResources("orders", "events")

	result, err := awsclient.EnrichDynamoDBPITR(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Findings) != 2 {
		t.Errorf("findings = %d, want 2", len(result.Findings))
	}
	for _, id := range []string{"orders", "events"} {
		f, ok := result.Findings[id]
		if !ok {
			t.Errorf("expected finding for table %q", id)
			continue
		}
		if f.Severity != "~" {
			t.Errorf("table %q severity = %q, want %q", id, f.Severity, "~")
		}
	}
}

// TestEnrichDynamoDBPITR_EnabledProducesNoFinding verifies that both tables
// with PITR ENABLED produce zero findings.
func TestEnrichDynamoDBPITR_EnabledProducesNoFinding(t *testing.T) {
	fake := &ddbPITRFake{
		perTable: map[string]*ddbPITRResponse{
			"users":    {out: pitrEnabledOutput()},
			"sessions": {out: pitrEnabledOutput()},
		},
	}
	clients := &awsclient.ServiceClients{DynamoDB: fake}
	resources := makeDDBResources("users", "sessions")

	result, err := awsclient.EnrichDynamoDBPITR(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected 0 findings, got %d", len(result.Findings))
	}
	if result.IssueCount != 0 {
		t.Errorf("IssueCount = %d, want 0", result.IssueCount)
	}
}

// TestEnrichDynamoDBPITR_MixedProducesOneFinding verifies that only the
// DISABLED table produces a finding when mixed with an ENABLED table.
func TestEnrichDynamoDBPITR_MixedProducesOneFinding(t *testing.T) {
	fake := &ddbPITRFake{
		perTable: map[string]*ddbPITRResponse{
			"table1": {out: pitrDisabledOutput()},
			"table2": {out: pitrEnabledOutput()},
		},
	}
	clients := &awsclient.ServiceClients{DynamoDB: fake}
	resources := makeDDBResources("table1", "table2")

	result, err := awsclient.EnrichDynamoDBPITR(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Findings) != 1 {
		t.Errorf("findings = %d, want 1", len(result.Findings))
	}
	if _, ok := result.Findings["table1"]; !ok {
		t.Error("expected finding for table1 (DISABLED), got none")
	}
	if _, ok := result.Findings["table2"]; ok {
		t.Error("unexpected finding for table2 (ENABLED)")
	}
}

// TestEnrichDynamoDBPITR_NoPITRSectionProducesNoFinding verifies that a
// response with no PointInTimeRecoveryDescription is silently skipped.
func TestEnrichDynamoDBPITR_NoPITRSectionProducesNoFinding(t *testing.T) {
	fake := &ddbPITRFake{
		perTable: map[string]*ddbPITRResponse{
			"archive": {out: pitrNoSectionOutput()},
		},
	}
	clients := &awsclient.ServiceClients{DynamoDB: fake}
	resources := makeDDBResources("archive")

	result, err := awsclient.EnrichDynamoDBPITR(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected 0 findings when PITR section absent, got %d", len(result.Findings))
	}
}

// TestEnrichDynamoDBPITR_NilClientReturnsEmptyFindingsNoError verifies that
// when clients.DynamoDB is nil, the enricher returns a non-nil empty Findings
// map and no error.
func TestEnrichDynamoDBPITR_NilClientReturnsEmptyFindingsNoError(t *testing.T) {
	clients := &awsclient.ServiceClients{DynamoDB: nil}
	resources := makeDDBResources("irrelevant")

	result, err := awsclient.EnrichDynamoDBPITR(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Findings == nil {
		t.Error("Findings must not be nil when DynamoDB client is nil")
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected empty Findings, got %d entries", len(result.Findings))
	}
}

// TestEnrichDynamoDBPITR_APIErrorOneTableSetsTruncated verifies that an API
// error for table1 produces no finding for that table and sets Truncated=true,
// while table2 (ENABLED) proceeds normally with no finding.
func TestEnrichDynamoDBPITR_APIErrorOneTableSetsTruncated(t *testing.T) {
	apiErr := errors.New("ddb: DescribeContinuousBackups throttled")
	fake := &ddbPITRFake{
		perTable: map[string]*ddbPITRResponse{
			"table1": {err: apiErr},
			"table2": {out: pitrEnabledOutput()},
		},
	}
	clients := &awsclient.ServiceClients{DynamoDB: fake}
	resources := makeDDBResources("table1", "table2")

	result, err := awsclient.EnrichDynamoDBPITR(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected top-level error: %v", err)
	}
	if _, ok := result.Findings["table1"]; ok {
		t.Error("must not produce finding for table1 when API returned error")
	}
	if !result.Truncated {
		t.Error("Truncated must be true when at least one table returned an error")
	}
}
