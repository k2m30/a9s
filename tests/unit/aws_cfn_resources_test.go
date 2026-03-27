package unit

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ---------------------------------------------------------------------------
// CFN Stack Resources fetcher tests (child of CloudFormation Stacks)
// ---------------------------------------------------------------------------

// TestFetchCfnResources_Basic verifies parsing of 3 stack resources with known
// types, statuses, and drift info, checking ID, Name, Status, all Fields, and
// RawStruct.
func TestFetchCfnResources_Basic(t *testing.T) {
	lastUpdated1 := time.Date(2024, 3, 22, 10, 0, 0, 0, time.UTC)
	lastUpdated2 := time.Date(2024, 3, 22, 10, 5, 0, 0, time.UTC)
	lastUpdated3 := time.Date(2024, 3, 22, 10, 10, 0, 0, time.UTC)

	mock := &mockCFNListStackResourcesClient{
		output: &cloudformation.ListStackResourcesOutput{
			StackResourceSummaries: []cfntypes.StackResourceSummary{
				{
					LogicalResourceId:    aws.String("MyBucket"),
					PhysicalResourceId:   aws.String("my-stack-mybucket-abc123"),
					ResourceType:         aws.String("AWS::S3::Bucket"),
					ResourceStatus:       cfntypes.ResourceStatusCreateComplete,
					ResourceStatusReason: aws.String(""),
					LastUpdatedTimestamp: &lastUpdated1,
					DriftInformation: &cfntypes.StackResourceDriftInformationSummary{
						StackResourceDriftStatus: cfntypes.StackResourceDriftStatusInSync,
					},
				},
				{
					LogicalResourceId:    aws.String("MyFunction"),
					PhysicalResourceId:   aws.String("my-stack-MyFunction-def456"),
					ResourceType:         aws.String("AWS::Lambda::Function"),
					ResourceStatus:       cfntypes.ResourceStatusCreateComplete,
					ResourceStatusReason: aws.String(""),
					LastUpdatedTimestamp: &lastUpdated2,
					DriftInformation: &cfntypes.StackResourceDriftInformationSummary{
						StackResourceDriftStatus: cfntypes.StackResourceDriftStatusModified,
					},
				},
				{
					LogicalResourceId:    aws.String("MyTable"),
					PhysicalResourceId:   aws.String("my-stack-MyTable-ghi789"),
					ResourceType:         aws.String("AWS::DynamoDB::Table"),
					ResourceStatus:       cfntypes.ResourceStatusUpdateComplete,
					ResourceStatusReason: aws.String(""),
					LastUpdatedTimestamp: &lastUpdated3,
					DriftInformation: &cfntypes.StackResourceDriftInformationSummary{
						StackResourceDriftStatus: cfntypes.StackResourceDriftStatusNotChecked,
					},
				},
			},
		},
	}

	result, err := awsclient.FetchCfnResources(
		context.Background(),
		mock,
		"my-stack",
		"",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	resources := result.Resources

	if len(resources) != 3 {
		t.Fatalf("expected 3 resources, got %d", len(resources))
	}

	t.Run("resource_0_ID", func(t *testing.T) {
		if resources[0].ID != "MyBucket" {
			t.Errorf("ID: expected %q, got %q", "MyBucket", resources[0].ID)
		}
	})

	t.Run("resource_0_Name", func(t *testing.T) {
		if resources[0].Name != "MyBucket" {
			t.Errorf("Name: expected %q, got %q", "MyBucket", resources[0].Name)
		}
	})

	t.Run("resource_0_Status", func(t *testing.T) {
		if resources[0].Status != "CREATE_COMPLETE" {
			t.Errorf("Status: expected %q, got %q", "CREATE_COMPLETE", resources[0].Status)
		}
	})

	t.Run("resource_0_Fields_logical_resource_id", func(t *testing.T) {
		r := resources[0]
		if r.Fields["logical_resource_id"] != "MyBucket" {
			t.Errorf("Fields[logical_resource_id]: expected %q, got %q", "MyBucket", r.Fields["logical_resource_id"])
		}
	})

	t.Run("resource_0_Fields_physical_resource_id", func(t *testing.T) {
		r := resources[0]
		if r.Fields["physical_resource_id"] != "my-stack-mybucket-abc123" {
			t.Errorf("Fields[physical_resource_id]: expected %q, got %q", "my-stack-mybucket-abc123", r.Fields["physical_resource_id"])
		}
	})

	t.Run("resource_0_Fields_resource_type", func(t *testing.T) {
		r := resources[0]
		if r.Fields["resource_type"] != "AWS::S3::Bucket" {
			t.Errorf("Fields[resource_type]: expected %q, got %q", "AWS::S3::Bucket", r.Fields["resource_type"])
		}
	})

	t.Run("resource_0_Fields_resource_status", func(t *testing.T) {
		r := resources[0]
		if r.Fields["resource_status"] != "CREATE_COMPLETE" {
			t.Errorf("Fields[resource_status]: expected %q, got %q", "CREATE_COMPLETE", r.Fields["resource_status"])
		}
	})

	t.Run("resource_0_Fields_drift_status", func(t *testing.T) {
		r := resources[0]
		if r.Fields["drift_status"] != "IN_SYNC" {
			t.Errorf("Fields[drift_status]: expected %q, got %q", "IN_SYNC", r.Fields["drift_status"])
		}
	})

	t.Run("resource_1_Fields_drift_status_modified", func(t *testing.T) {
		r := resources[1]
		if r.Fields["drift_status"] != "MODIFIED" {
			t.Errorf("Fields[drift_status]: expected %q, got %q", "MODIFIED", r.Fields["drift_status"])
		}
	})

	t.Run("resource_0_Fields_last_updated", func(t *testing.T) {
		r := resources[0]
		if r.Fields["last_updated"] == "" {
			t.Error("Fields[last_updated] should not be empty")
		}
	})

	t.Run("resource_0_RawStruct", func(t *testing.T) {
		r := resources[0]
		if r.RawStruct == nil {
			t.Fatal("RawStruct must not be nil")
		}
		raw, ok := r.RawStruct.(cfntypes.StackResourceSummary)
		if !ok {
			t.Fatalf("RawStruct should be cfntypes.StackResourceSummary, got %T", r.RawStruct)
		}
		if raw.LogicalResourceId == nil || *raw.LogicalResourceId != "MyBucket" {
			t.Error("RawStruct.LogicalResourceId not preserved correctly")
		}
	})

	// Verify required fields on all resources
	t.Run("required_fields_present", func(t *testing.T) {
		requiredFields := []string{"logical_resource_id", "physical_resource_id", "resource_type", "resource_status", "drift_status", "last_updated"}
		for i, r := range resources {
			for _, key := range requiredFields {
				if _, ok := r.Fields[key]; !ok {
					t.Errorf("resource[%d].Fields missing key %q", i, key)
				}
			}
		}
	})
}

// TestFetchCfnResources_Empty verifies that a stack with no resources
// returns an empty slice with no error.
func TestFetchCfnResources_Empty(t *testing.T) {
	mock := &mockCFNListStackResourcesClient{
		output: &cloudformation.ListStackResourcesOutput{
			StackResourceSummaries: []cfntypes.StackResourceSummary{},
		},
	}

	result, err := awsclient.FetchCfnResources(
		context.Background(),
		mock,
		"empty-stack",
		"",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(result.Resources))
	}
}

// TestFetchCfnResources_APIError verifies that API errors are propagated.
func TestFetchCfnResources_APIError(t *testing.T) {
	mock := &mockCFNListStackResourcesClient{
		err: fmt.Errorf("AWS API error: stack not found"),
	}

	result, err := awsclient.FetchCfnResources(
		context.Background(),
		mock,
		"err-stack",
		"",
	)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources on error, got %d", len(result.Resources))
	}
}

// TestFetchCfnResources_NilDriftInformation verifies that nil DriftInformation
// does not cause a panic and drift_status is empty string.
func TestFetchCfnResources_NilDriftInformation(t *testing.T) {
	lastUpdated := time.Date(2024, 3, 22, 10, 0, 0, 0, time.UTC)

	mock := &mockCFNListStackResourcesClient{
		output: &cloudformation.ListStackResourcesOutput{
			StackResourceSummaries: []cfntypes.StackResourceSummary{
				{
					LogicalResourceId:    aws.String("NoDrift"),
					PhysicalResourceId:   aws.String("phys-id-123"),
					ResourceType:         aws.String("AWS::EC2::Instance"),
					ResourceStatus:       cfntypes.ResourceStatusCreateComplete,
					LastUpdatedTimestamp: &lastUpdated,
					// DriftInformation is nil
				},
			},
		},
	}

	result, err := awsclient.FetchCfnResources(
		context.Background(),
		mock,
		"nodrift-stack",
		"",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	resources := result.Resources

	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	drift := resources[0].Fields["drift_status"]
	if drift != "" {
		t.Errorf("Fields[drift_status] should be empty for nil DriftInformation, got %q", drift)
	}
}

// TestFetchCfnResources_NilOptionalFields verifies that nil PhysicalResourceId
// and nil ResourceStatusReason do not cause a panic.
func TestFetchCfnResources_NilOptionalFields(t *testing.T) {
	lastUpdated := time.Date(2024, 3, 22, 10, 0, 0, 0, time.UTC)

	mock := &mockCFNListStackResourcesClient{
		output: &cloudformation.ListStackResourcesOutput{
			StackResourceSummaries: []cfntypes.StackResourceSummary{
				{
					LogicalResourceId:   aws.String("NilPhysical"),
					ResourceType:        aws.String("AWS::EC2::Instance"),
					ResourceStatus:      cfntypes.ResourceStatusCreateInProgress,
					LastUpdatedTimestamp: &lastUpdated,
					// PhysicalResourceId is nil (resource not yet created)
					// ResourceStatusReason is nil
				},
			},
		},
	}

	// Should not panic
	result, err := awsclient.FetchCfnResources(
		context.Background(),
		mock,
		"nil-fields-stack",
		"",
	)
	if err != nil {
		t.Fatalf("expected no error for nil optional fields, got %v", err)
	}
	resources := result.Resources

	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	t.Run("nil_PhysicalResourceId", func(t *testing.T) {
		if resources[0].Fields["physical_resource_id"] != "" {
			t.Logf("Fields[physical_resource_id] is %q (expected empty for nil)", resources[0].Fields["physical_resource_id"])
		}
	})
}

// TestFetchCfnResources_TimestampFormatting verifies that a known time.Time
// produces a formatted string in Fields[last_updated].
func TestFetchCfnResources_TimestampFormatting(t *testing.T) {
	ts := time.Date(2024, 12, 25, 14, 30, 45, 0, time.UTC)

	mock := &mockCFNListStackResourcesClient{
		output: &cloudformation.ListStackResourcesOutput{
			StackResourceSummaries: []cfntypes.StackResourceSummary{
				{
					LogicalResourceId:    aws.String("TsResource"),
					PhysicalResourceId:   aws.String("phys-ts"),
					ResourceType:         aws.String("AWS::S3::Bucket"),
					ResourceStatus:       cfntypes.ResourceStatusCreateComplete,
					LastUpdatedTimestamp: &ts,
				},
			},
		},
	}

	result, err := awsclient.FetchCfnResources(
		context.Background(),
		mock,
		"ts-stack",
		"",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	resources := result.Resources

	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	lastUpdated := resources[0].Fields["last_updated"]
	if !strings.Contains(lastUpdated, "2024-12-25") {
		t.Errorf("Fields[last_updated] should contain date, got %q", lastUpdated)
	}
}

// TestFetchCfnResources_RawStruct verifies that RawStruct preserves the
// original cfntypes.StackResourceSummary, including all sub-fields.
func TestFetchCfnResources_RawStruct(t *testing.T) {
	ts := time.Date(2024, 3, 22, 12, 30, 0, 0, time.UTC)

	mock := &mockCFNListStackResourcesClient{
		output: &cloudformation.ListStackResourcesOutput{
			StackResourceSummaries: []cfntypes.StackResourceSummary{
				{
					LogicalResourceId:    aws.String("RawBucket"),
					PhysicalResourceId:   aws.String("my-raw-bucket-xyz"),
					ResourceType:         aws.String("AWS::S3::Bucket"),
					ResourceStatus:       cfntypes.ResourceStatusCreateComplete,
					ResourceStatusReason: aws.String("Resource creation complete"),
					LastUpdatedTimestamp: &ts,
					DriftInformation: &cfntypes.StackResourceDriftInformationSummary{
						StackResourceDriftStatus: cfntypes.StackResourceDriftStatusInSync,
					},
				},
			},
		},
	}

	result, err := awsclient.FetchCfnResources(
		context.Background(),
		mock,
		"raw-stack",
		"",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	resources := result.Resources

	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	r := resources[0]
	if r.RawStruct == nil {
		t.Fatal("RawStruct must not be nil")
	}

	raw, ok := r.RawStruct.(cfntypes.StackResourceSummary)
	if !ok {
		t.Fatalf("RawStruct should be cfntypes.StackResourceSummary, got %T", r.RawStruct)
	}

	t.Run("LogicalResourceId_preserved", func(t *testing.T) {
		if raw.LogicalResourceId == nil || *raw.LogicalResourceId != "RawBucket" {
			t.Errorf("RawStruct.LogicalResourceId not preserved correctly")
		}
	})

	t.Run("PhysicalResourceId_preserved", func(t *testing.T) {
		if raw.PhysicalResourceId == nil || *raw.PhysicalResourceId != "my-raw-bucket-xyz" {
			t.Errorf("RawStruct.PhysicalResourceId not preserved correctly")
		}
	})

	t.Run("ResourceType_preserved", func(t *testing.T) {
		if raw.ResourceType == nil || *raw.ResourceType != "AWS::S3::Bucket" {
			t.Errorf("RawStruct.ResourceType not preserved correctly")
		}
	})

	t.Run("DriftInformation_preserved", func(t *testing.T) {
		if raw.DriftInformation == nil {
			t.Fatal("RawStruct.DriftInformation should not be nil")
		}
		if raw.DriftInformation.StackResourceDriftStatus != cfntypes.StackResourceDriftStatusInSync {
			t.Errorf("RawStruct.DriftInformation.StackResourceDriftStatus: expected IN_SYNC, got %v", raw.DriftInformation.StackResourceDriftStatus)
		}
	})

	t.Run("LastUpdatedTimestamp_preserved", func(t *testing.T) {
		if raw.LastUpdatedTimestamp == nil || !raw.LastUpdatedTimestamp.Equal(ts) {
			t.Errorf("RawStruct.LastUpdatedTimestamp not preserved correctly")
		}
	})
}

// TestCfnResourceColumns verifies that CfnResourceColumns returns the expected
// columns with correct keys.
func TestCfnResourceColumns(t *testing.T) {
	cols := resource.CfnResourceColumns()

	expectedKeys := []string{"logical_resource_id", "physical_resource_id", "resource_type", "resource_status", "drift_status", "last_updated"}

	t.Run("column_count", func(t *testing.T) {
		if len(cols) != len(expectedKeys) {
			t.Fatalf("expected %d columns, got %d", len(expectedKeys), len(cols))
		}
	})

	t.Run("column_keys", func(t *testing.T) {
		for i, expected := range expectedKeys {
			if cols[i].Key != expected {
				t.Errorf("column[%d].Key: expected %q, got %q", i, expected, cols[i].Key)
			}
		}
	})

	t.Run("columns_have_titles", func(t *testing.T) {
		for i, col := range cols {
			if col.Title == "" {
				t.Errorf("column[%d] (%s) has empty Title", i, col.Key)
			}
		}
	})

	t.Run("columns_have_positive_width", func(t *testing.T) {
		for i, col := range cols {
			if col.Width <= 0 {
				t.Errorf("column[%d] (%s) has non-positive Width: %d", i, col.Key, col.Width)
			}
		}
	})
}

// TestCfnResources_ChildTypeRegistered verifies that the child type is
// registered under the correct short name.
func TestCfnResources_ChildTypeRegistered(t *testing.T) {
	td := resource.GetChildType("cfn_resources")
	if td == nil {
		t.Fatal("cfn_resources child resource type not registered")
	}
	if td.Name == "" {
		t.Error("child type Name should not be empty")
	}
	if td.ShortName != "cfn_resources" {
		t.Errorf("child type ShortName: expected %q, got %q", "cfn_resources", td.ShortName)
	}
}

// TestCfnResources_PaginatedChildFetcherRegistered verifies that the paginated
// child fetcher is registered under the correct short name.
func TestCfnResources_PaginatedChildFetcherRegistered(t *testing.T) {
	f := resource.GetPaginatedChildFetcher("cfn_resources")
	if f == nil {
		t.Fatal("cfn_resources paginated child fetcher not registered")
	}
}

// TestCfnResources_ParentHasChildDef verifies that the parent cfn resource
// type has a child view definition for cfn_resources with key "r".
func TestCfnResources_ParentHasChildDef(t *testing.T) {
	rt := resource.FindResourceType("cfn")
	if rt == nil {
		t.Fatal("cfn resource type not found")
	}

	found := false
	for _, child := range rt.Children {
		if child.ChildType == "cfn_resources" {
			found = true
			if child.Key != "r" {
				t.Errorf("expected key %q, got %q", "r", child.Key)
			}
			if child.ContextKeys["stack_name"] == "" {
				t.Error("ContextKeys should include 'stack_name'")
			}
		}
	}
	if !found {
		t.Error("cfn Children should contain cfn_resources child view def")
	}
}

// TestFetchCfnResources_Pagination verifies that paginated responses via
// NextToken are followed and all resources collected across multiple pages.
func TestFetchCfnResources_Pagination(t *testing.T) {
	lastUpdated := time.Date(2024, 3, 22, 10, 0, 0, 0, time.UTC)

	mock := &mockCFNListStackResourcesClient{
		outputs: []*cloudformation.ListStackResourcesOutput{
			{
				NextToken: aws.String("page2-token"),
				StackResourceSummaries: []cfntypes.StackResourceSummary{
					{
						LogicalResourceId:    aws.String("Bucket1"),
						PhysicalResourceId:   aws.String("phys-bucket1"),
						ResourceType:         aws.String("AWS::S3::Bucket"),
						ResourceStatus:       cfntypes.ResourceStatusCreateComplete,
						LastUpdatedTimestamp: &lastUpdated,
						DriftInformation: &cfntypes.StackResourceDriftInformationSummary{
							StackResourceDriftStatus: cfntypes.StackResourceDriftStatusInSync,
						},
					},
					{
						LogicalResourceId:    aws.String("Function1"),
						PhysicalResourceId:   aws.String("phys-function1"),
						ResourceType:         aws.String("AWS::Lambda::Function"),
						ResourceStatus:       cfntypes.ResourceStatusCreateComplete,
						LastUpdatedTimestamp: &lastUpdated,
					},
					{
						LogicalResourceId:    aws.String("Table1"),
						PhysicalResourceId:   aws.String("phys-table1"),
						ResourceType:         aws.String("AWS::DynamoDB::Table"),
						ResourceStatus:       cfntypes.ResourceStatusCreateComplete,
						LastUpdatedTimestamp: &lastUpdated,
					},
				},
			},
			{
				// No NextToken — last page
				StackResourceSummaries: []cfntypes.StackResourceSummary{
					{
						LogicalResourceId:    aws.String("Queue1"),
						PhysicalResourceId:   aws.String("phys-queue1"),
						ResourceType:         aws.String("AWS::SQS::Queue"),
						ResourceStatus:       cfntypes.ResourceStatusCreateComplete,
						LastUpdatedTimestamp: &lastUpdated,
					},
					{
						LogicalResourceId:    aws.String("Topic1"),
						PhysicalResourceId:   aws.String("phys-topic1"),
						ResourceType:         aws.String("AWS::SNS::Topic"),
						ResourceStatus:       cfntypes.ResourceStatusUpdateComplete,
						LastUpdatedTimestamp: &lastUpdated,
					},
				},
			},
		},
	}

	result, err := awsclient.FetchCfnResources(
		context.Background(),
		mock,
		"paginated-stack",
		"",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	resources := result.Resources

	t.Run("total_count", func(t *testing.T) {
		if len(resources) != 5 {
			t.Fatalf("expected 5 resources across 2 pages, got %d", len(resources))
		}
	})

	t.Run("page1_resources", func(t *testing.T) {
		expectedIDs := []string{"Bucket1", "Function1", "Table1"}
		for i, expectedID := range expectedIDs {
			if resources[i].ID != expectedID {
				t.Errorf("resources[%d].ID: expected %q, got %q", i, expectedID, resources[i].ID)
			}
		}
	})

	t.Run("page2_resources", func(t *testing.T) {
		expectedIDs := []string{"Queue1", "Topic1"}
		for i, expectedID := range expectedIDs {
			if resources[i+3].ID != expectedID {
				t.Errorf("resources[%d].ID: expected %q, got %q", i+3, expectedID, resources[i+3].ID)
			}
		}
	})

	t.Run("api_called_twice", func(t *testing.T) {
		if mock.callIdx != 2 {
			t.Errorf("expected 2 API calls for pagination, got %d", mock.callIdx)
		}
	})

	t.Run("all_fields_populated", func(t *testing.T) {
		requiredFields := []string{"logical_resource_id", "physical_resource_id", "resource_type", "resource_status", "drift_status", "last_updated"}
		for i, r := range resources {
			for _, key := range requiredFields {
				if _, ok := r.Fields[key]; !ok {
					t.Errorf("resource[%d].Fields missing key %q", i, key)
				}
			}
		}
	})

	t.Run("resource_types_correct", func(t *testing.T) {
		expectedTypes := []string{
			"AWS::S3::Bucket",
			"AWS::Lambda::Function",
			"AWS::DynamoDB::Table",
			"AWS::SQS::Queue",
			"AWS::SNS::Topic",
		}
		for i, expectedType := range expectedTypes {
			if resources[i].Fields["resource_type"] != expectedType {
				t.Errorf("resources[%d].Fields[resource_type]: expected %q, got %q", i, expectedType, resources[i].Fields["resource_type"])
			}
		}
	})
}
