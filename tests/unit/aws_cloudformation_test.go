package unit

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
)

// ---------------------------------------------------------------------------
// T070 - Test CloudFormation DescribeStacks response parsing
// ---------------------------------------------------------------------------

func TestFetchCloudFormationStacks_ParsesMultipleStacks(t *testing.T) {
	creationTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	lastUpdated := time.Date(2025, 3, 10, 14, 30, 0, 0, time.UTC)

	mock := &mockCFNDescribeStacksClient{
		output: &cloudformation.DescribeStacksOutput{
			Stacks: []cfntypes.Stack{
				{
					StackName:         aws.String("prod-infra-stack"),
					StackStatus:       cfntypes.StackStatusCreateComplete,
					CreationTime:      &creationTime,
					LastUpdatedTime:   &lastUpdated,
					Description:       aws.String("Production infrastructure stack"),
					StackId:           aws.String("arn:aws:cloudformation:us-east-1:123456789012:stack/prod-infra-stack/abc123"),
					StackStatusReason: nil,
					RoleARN:           aws.String("arn:aws:iam::123456789012:role/cfn-role"),
					DriftInformation: &cfntypes.StackDriftInformation{
						StackDriftStatus: cfntypes.StackDriftStatusInSync,
					},
				},
				{
					StackName:       aws.String("staging-app-stack"),
					StackStatus:     cfntypes.StackStatusUpdateComplete,
					CreationTime:    &creationTime,
					LastUpdatedTime: &lastUpdated,
					Description:     aws.String("Staging application stack"),
					StackId:         aws.String("arn:aws:cloudformation:us-east-1:123456789012:stack/staging-app-stack/def456"),
					DriftInformation: &cfntypes.StackDriftInformation{
						StackDriftStatus: cfntypes.StackDriftStatusInSync,
					},
				},
			},
		},
	}

	resources, err := awsclient.FetchCloudFormationStacks(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}

	// Verify required fields exist
	requiredFields := []string{"stack_name", "status", "creation_time", "last_updated", "description"}
	for i, r := range resources {
		for _, key := range requiredFields {
			if _, ok := r.Fields[key]; !ok {
				t.Errorf("resource[%d].Fields missing key %q", i, key)
			}
		}
	}

	// Verify first stack
	r0 := resources[0]
	if r0.ID != "prod-infra-stack" {
		t.Errorf("resource[0].ID: expected %q, got %q", "prod-infra-stack", r0.ID)
	}
	if r0.Name != "prod-infra-stack" {
		t.Errorf("resource[0].Name: expected %q, got %q", "prod-infra-stack", r0.Name)
	}
	if r0.Status != "CREATE_COMPLETE" {
		t.Errorf("resource[0].Status: expected %q, got %q", "CREATE_COMPLETE", r0.Status)
	}
	if r0.Fields["stack_name"] != "prod-infra-stack" {
		t.Errorf("resource[0].Fields[\"stack_name\"]: expected %q, got %q", "prod-infra-stack", r0.Fields["stack_name"])
	}
	if r0.Fields["status"] != "CREATE_COMPLETE" {
		t.Errorf("resource[0].Fields[\"status\"]: expected %q, got %q", "CREATE_COMPLETE", r0.Fields["status"])
	}
	if r0.Fields["description"] != "Production infrastructure stack" {
		t.Errorf("resource[0].Fields[\"description\"]: expected %q, got %q", "Production infrastructure stack", r0.Fields["description"])
	}
	if r0.Fields["creation_time"] == "" {
		t.Error("resource[0].Fields[\"creation_time\"] should not be empty")
	}
	if r0.Fields["last_updated"] == "" {
		t.Error("resource[0].Fields[\"last_updated\"] should not be empty")
	}

	// Verify second stack
	r1 := resources[1]
	if r1.ID != "staging-app-stack" {
		t.Errorf("resource[1].ID: expected %q, got %q", "staging-app-stack", r1.ID)
	}
	if r1.Status != "UPDATE_COMPLETE" {
		t.Errorf("resource[1].Status: expected %q, got %q", "UPDATE_COMPLETE", r1.Status)
	}
	if r1.Fields["stack_name"] != "staging-app-stack" {
		t.Errorf("resource[1].Fields[\"stack_name\"]: expected %q, got %q", "staging-app-stack", r1.Fields["stack_name"])
	}
	if r1.Fields["description"] != "Staging application stack" {
		t.Errorf("resource[1].Fields[\"description\"]: expected %q, got %q", "Staging application stack", r1.Fields["description"])
	}
}

func TestFetchCloudFormationStacks_ErrorResponse(t *testing.T) {
	mock := &mockCFNDescribeStacksClient{
		output: nil,
		err:    fmt.Errorf("AWS API error: access denied"),
	}

	resources, err := awsclient.FetchCloudFormationStacks(context.Background(), mock)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if resources != nil {
		t.Errorf("expected nil resources on error, got %d resources", len(resources))
	}
}

func TestFetchCloudFormationStacks_EmptyResponse(t *testing.T) {
	mock := &mockCFNDescribeStacksClient{
		output: &cloudformation.DescribeStacksOutput{
			Stacks: []cfntypes.Stack{},
		},
	}

	resources, err := awsclient.FetchCloudFormationStacks(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(resources))
	}
}
