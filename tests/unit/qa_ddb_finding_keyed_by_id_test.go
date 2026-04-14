package unit

// qa_ddb_finding_keyed_by_id_test.go — Regression: EnrichDynamoDBStatus keys findings by r.ID.
//
// Bug: EnrichDynamoDBStatus was writing findings[r.Name] — r.ID was ignored.
// Fix: findings are now keyed by r.ID (falling back to r.Name only when ID is empty).
//
// This test fails if the fix is reverted: findings would be keyed by r.Name
// and the r.ID key would be absent.

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// TestEnrichDynamoDBStatus_FindingKeyedByID_NotByName verifies findings are keyed by
// r.ID when r.ID != r.Name. Regresses if the enricher reverts to findings[r.Name].
func TestEnrichDynamoDBStatus_FindingKeyedByID_NotByName(t *testing.T) {
	const tableName = "orders-table"
	const tableARN = "arn:aws:dynamodb:us-east-1:123456789012:table/orders-table"

	fake := &ddbStatusFake{
		tables: map[string]*dynamodb.DescribeTableOutput{
			tableName: {
				Table: &dbtypes.TableDescription{
					TableStatus: dbtypes.TableStatusDeleting, // non-ACTIVE → finding emitted
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{DynamoDB: fake}
	// r.ID is an ARN-like string; r.Name is the human-readable table name.
	resources := []resource.Resource{{ID: tableARN, Name: tableName}}

	result, err := awsclient.EnrichDynamoDBStatus(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Finding must be keyed by r.ID (the ARN), not r.Name (the table name).
	if _, ok := result.Findings[tableARN]; !ok {
		t.Errorf("finding must be keyed by r.ID=%q — was the fix to EnrichDynamoDBStatus reverted?", tableARN)
	}
	if _, ok := result.Findings[tableName]; ok {
		t.Errorf("finding must NOT be keyed by r.Name=%q — enricher should use r.ID as the key", tableName)
	}
}
