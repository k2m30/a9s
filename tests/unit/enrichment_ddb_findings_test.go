package unit

// enrichment_ddb_findings_test.go — Behavioral tests for EnrichDynamoDBStatus.
//
// Contract assertions (enricher-contract.md):
//   - Returns EnricherResult.Findings keyed by table name (r.Name).
//   - Severity "!" for all findings.
//   - Summary "table status: <status>" when table itself is non-ACTIVE.
//   - Summary "GSI <name> status: <status>" when a GSI is non-ACTIVE.
//   - IssueCount = len(Findings).
//   - Truncated = true when len(resources) > EnrichmentCap.
//   - ACTIVE tables with all-ACTIVE GSIs must NOT appear in Findings.
//   - Empty resources → non-nil empty Findings map.

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ddbStatusFake implements DynamoDBAPI subset for enrichment testing.
type ddbStatusFake struct {
	awsclient.DynamoDBAPI
	// tables maps table name → DescribeTableOutput
	tables map[string]*dynamodb.DescribeTableOutput
	err    error
}

func (f *ddbStatusFake) DescribeTable(
	_ context.Context,
	params *dynamodb.DescribeTableInput,
	_ ...func(*dynamodb.Options),
) (*dynamodb.DescribeTableOutput, error) {
	if f.err != nil {
		return nil, f.err
	}
	name := aws.ToString(params.TableName)
	if out, ok := f.tables[name]; ok {
		return out, nil
	}
	return &dynamodb.DescribeTableOutput{}, nil
}

// TestEnrichDynamoDBStatus_TableStatusNonActiveFinding verifies a non-ACTIVE table
// produces a finding keyed by table name with summary "table status: <status>".
func TestEnrichDynamoDBStatus_TableStatusNonActiveFinding(t *testing.T) {
	fake := &ddbStatusFake{
		tables: map[string]*dynamodb.DescribeTableOutput{
			"broken-table": {
				Table: &dbtypes.TableDescription{
					TableStatus:             dbtypes.TableStatusDeleting,
					GlobalSecondaryIndexes: nil,
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{DynamoDB: fake}
	resources := []resource.Resource{{ID: "broken-table", Name: "broken-table"}}

	result, err := awsclient.EnrichDynamoDBStatus(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	f, ok := result.Findings["broken-table"]
	if !ok {
		t.Fatal("expected finding keyed by table name")
	}
	if f.Severity != "!" {
		t.Errorf("severity = %q, want %q", f.Severity, "!")
	}
	if !strings.HasPrefix(f.Summary, "table status:") {
		t.Errorf("summary %q must start with %q", f.Summary, "table status:")
	}
	if !strings.Contains(f.Summary, "DELETING") {
		t.Errorf("summary %q must contain table status %q", f.Summary, "DELETING")
	}
}

// TestEnrichDynamoDBStatus_GSINonActiveFinding verifies a non-ACTIVE GSI produces
// a finding with summary "GSI <name> status: <status>".
func TestEnrichDynamoDBStatus_GSINonActiveFinding(t *testing.T) {
	fake := &ddbStatusFake{
		tables: map[string]*dynamodb.DescribeTableOutput{
			"gsi-table": {
				Table: &dbtypes.TableDescription{
					TableStatus: dbtypes.TableStatusActive,
					GlobalSecondaryIndexes: []dbtypes.GlobalSecondaryIndexDescription{
						{
							IndexName:   aws.String("my-gsi"),
							IndexStatus: dbtypes.IndexStatusCreating,
						},
					},
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{DynamoDB: fake}
	resources := []resource.Resource{{Name: "gsi-table"}}

	result, err := awsclient.EnrichDynamoDBStatus(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	f, ok := result.Findings["gsi-table"]
	if !ok {
		t.Fatal("expected finding for table with non-ACTIVE GSI")
	}
	if f.Severity != "!" {
		t.Errorf("severity = %q, want %q", f.Severity, "!")
	}
	if !strings.Contains(f.Summary, "GSI") {
		t.Errorf("summary %q must contain %q", f.Summary, "GSI")
	}
	if !strings.Contains(f.Summary, "my-gsi") {
		t.Errorf("summary %q must contain GSI name %q", f.Summary, "my-gsi")
	}
}

// TestEnrichDynamoDBStatus_ActiveTableExcluded verifies ACTIVE tables with ACTIVE GSIs
// do not appear in Findings.
func TestEnrichDynamoDBStatus_ActiveTableExcluded(t *testing.T) {
	fake := &ddbStatusFake{
		tables: map[string]*dynamodb.DescribeTableOutput{
			"ok-table": {
				Table: &dbtypes.TableDescription{
					TableStatus: dbtypes.TableStatusActive,
					GlobalSecondaryIndexes: []dbtypes.GlobalSecondaryIndexDescription{
						{IndexName: aws.String("ok-gsi"), IndexStatus: dbtypes.IndexStatusActive},
					},
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{DynamoDB: fake}
	resources := []resource.Resource{{Name: "ok-table"}}

	result, err := awsclient.EnrichDynamoDBStatus(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := result.Findings["ok-table"]; ok {
		t.Error("ACTIVE table with all-ACTIVE GSIs must NOT appear in Findings")
	}
	if result.IssueCount != 0 {
		t.Errorf("IssueCount = %d, want 0", result.IssueCount)
	}
}

// TestEnrichDynamoDBStatus_IssueCountEqualsFindings verifies IssueCount = len(Findings).
func TestEnrichDynamoDBStatus_IssueCountEqualsFindings(t *testing.T) {
	fake := &ddbStatusFake{
		tables: map[string]*dynamodb.DescribeTableOutput{
			"bad-table-1": {Table: &dbtypes.TableDescription{TableStatus: dbtypes.TableStatusDeleting}},
			"bad-table-2": {Table: &dbtypes.TableDescription{TableStatus: dbtypes.TableStatusUpdating}},
			"ok-table":    {Table: &dbtypes.TableDescription{TableStatus: dbtypes.TableStatusActive}},
		},
	}
	clients := &awsclient.ServiceClients{DynamoDB: fake}
	resources := []resource.Resource{
		{Name: "bad-table-1"},
		{Name: "bad-table-2"},
		{Name: "ok-table"},
	}

	result, err := awsclient.EnrichDynamoDBStatus(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IssueCount != 2 {
		t.Errorf("IssueCount = %d, want 2", result.IssueCount)
	}
	if result.IssueCount != len(result.Findings) {
		t.Errorf("IssueCount (%d) != len(Findings) (%d)", result.IssueCount, len(result.Findings))
	}
}

// TestEnrichDynamoDBStatus_TruncatedWhenResourcesExceedCap verifies Truncated=true.
func TestEnrichDynamoDBStatus_TruncatedWhenResourcesExceedCap(t *testing.T) {
	count := awsclient.EnrichmentCap + 1
	resources := make([]resource.Resource, count)
	tables := make(map[string]*dynamodb.DescribeTableOutput, count)
	for i := range count {
		name := fmt.Sprintf("table-%03d", i)
		resources[i] = resource.Resource{Name: name}
		tables[name] = &dynamodb.DescribeTableOutput{
			Table: &dbtypes.TableDescription{TableStatus: dbtypes.TableStatusActive},
		}
	}
	fake := &ddbStatusFake{tables: tables}
	clients := &awsclient.ServiceClients{DynamoDB: fake}

	result, err := awsclient.EnrichDynamoDBStatus(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Truncated {
		t.Errorf("Truncated must be true when len(resources)=%d > EnrichmentCap=%d",
			count, awsclient.EnrichmentCap)
	}
}

// TestEnrichDynamoDBStatus_EmptyResourcesReturnsEmptyFindings verifies nil/empty
// resources returns non-nil empty Findings.
func TestEnrichDynamoDBStatus_EmptyResourcesReturnsEmptyFindings(t *testing.T) {
	fake := &ddbStatusFake{tables: map[string]*dynamodb.DescribeTableOutput{}}
	clients := &awsclient.ServiceClients{DynamoDB: fake}

	result, err := awsclient.EnrichDynamoDBStatus(context.Background(), clients, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Findings == nil {
		t.Error("Findings must not be nil on empty resources")
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected empty Findings, got %d entries", len(result.Findings))
	}
}
