package unit

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
)

func int64Ptr(v int64) *int64 { return &v }

// ---------------------------------------------------------------------------
// DynamoDB - Test FetchDynamoDBTables response parsing (Pattern C: list+describe)
// ---------------------------------------------------------------------------

func TestFetchDynamoDBTables_ParsesMultipleTables(t *testing.T) {
	createdAt := time.Date(2025, 1, 10, 8, 0, 0, 0, time.UTC)

	listMock := &mockDDBListTablesClient{
		output: &dynamodb.ListTablesOutput{
			TableNames: []string{"orders-prod", "users-prod"},
		},
	}

	describeMock := &mockDDBDescribeTableClient{
		outputs: map[string]*dynamodb.DescribeTableOutput{
			"orders-prod": {
				Table: &ddbtypes.TableDescription{
					TableName:      aws.String("orders-prod"),
					TableStatus:    ddbtypes.TableStatusActive,
					ItemCount:      int64Ptr(15000),
					TableSizeBytes: int64Ptr(5242880),
					BillingModeSummary: &ddbtypes.BillingModeSummary{
						BillingMode: ddbtypes.BillingModePayPerRequest,
					},
					TableArn:         aws.String("arn:aws:dynamodb:us-east-1:123456789012:table/orders-prod"),
					TableId:          aws.String("abc123-table-id"),
					CreationDateTime: &createdAt,
				},
			},
			"users-prod": {
				Table: &ddbtypes.TableDescription{
					TableName:      aws.String("users-prod"),
					TableStatus:    ddbtypes.TableStatusActive,
					ItemCount:      int64Ptr(3200),
					TableSizeBytes: int64Ptr(1048576),
					BillingModeSummary: &ddbtypes.BillingModeSummary{
						BillingMode: ddbtypes.BillingModeProvisioned,
					},
					TableArn: aws.String("arn:aws:dynamodb:us-east-1:123456789012:table/users-prod"),
					TableId:  aws.String("def456-table-id"),
				},
			},
		},
	}

	resources, err := awsclient.FetchDynamoDBTables(context.Background(), listMock, describeMock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}

	// Verify required fields exist
	requiredFields := []string{"table_name", "status", "item_count", "size_bytes", "billing_mode"}
	for i, r := range resources {
		for _, key := range requiredFields {
			if _, ok := r.Fields[key]; !ok {
				t.Errorf("resource[%d].Fields missing key %q", i, key)
			}
		}
	}

	// Verify first table
	r0 := resources[0]
	if r0.ID != "orders-prod" {
		t.Errorf("resource[0].ID: expected %q, got %q", "orders-prod", r0.ID)
	}
	if r0.Name != "orders-prod" {
		t.Errorf("resource[0].Name: expected %q, got %q", "orders-prod", r0.Name)
	}
	if r0.Status != "ACTIVE" {
		t.Errorf("resource[0].Status: expected %q, got %q", "ACTIVE", r0.Status)
	}
	if r0.Fields["table_name"] != "orders-prod" {
		t.Errorf("resource[0].Fields[\"table_name\"]: expected %q, got %q", "orders-prod", r0.Fields["table_name"])
	}
	if r0.Fields["status"] != "ACTIVE" {
		t.Errorf("resource[0].Fields[\"status\"]: expected %q, got %q", "ACTIVE", r0.Fields["status"])
	}
	if r0.Fields["item_count"] != "15000" {
		t.Errorf("resource[0].Fields[\"item_count\"]: expected %q, got %q", "15000", r0.Fields["item_count"])
	}
	if r0.Fields["size_bytes"] != "5242880" {
		t.Errorf("resource[0].Fields[\"size_bytes\"]: expected %q, got %q", "5242880", r0.Fields["size_bytes"])
	}
	if r0.Fields["billing_mode"] != "PAY_PER_REQUEST" {
		t.Errorf("resource[0].Fields[\"billing_mode\"]: expected %q, got %q", "PAY_PER_REQUEST", r0.Fields["billing_mode"])
	}

	// Verify second table
	r1 := resources[1]
	if r1.ID != "users-prod" {
		t.Errorf("resource[1].ID: expected %q, got %q", "users-prod", r1.ID)
	}
	if r1.Fields["item_count"] != "3200" {
		t.Errorf("resource[1].Fields[\"item_count\"]: expected %q, got %q", "3200", r1.Fields["item_count"])
	}
	if r1.Fields["billing_mode"] != "PROVISIONED" {
		t.Errorf("resource[1].Fields[\"billing_mode\"]: expected %q, got %q", "PROVISIONED", r1.Fields["billing_mode"])
	}
}

func TestFetchDynamoDBTables_ErrorResponse(t *testing.T) {
	listMock := &mockDDBListTablesClient{
		output: nil,
		err:    fmt.Errorf("AWS API error: access denied"),
	}
	describeMock := &mockDDBDescribeTableClient{}

	resources, err := awsclient.FetchDynamoDBTables(context.Background(), listMock, describeMock)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if resources != nil {
		t.Errorf("expected nil resources on error, got %d resources", len(resources))
	}
}

func TestFetchDynamoDBTables_EmptyResponse(t *testing.T) {
	listMock := &mockDDBListTablesClient{
		output: &dynamodb.ListTablesOutput{
			TableNames: []string{},
		},
	}
	describeMock := &mockDDBDescribeTableClient{
		outputs: map[string]*dynamodb.DescribeTableOutput{},
	}

	resources, err := awsclient.FetchDynamoDBTables(context.Background(), listMock, describeMock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(resources))
	}
}
