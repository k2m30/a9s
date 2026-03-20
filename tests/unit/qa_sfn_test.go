package unit

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sfn"
	sfntypes "github.com/aws/aws-sdk-go-v2/service/sfn/types"

	awsclient "github.com/k2m30/a9s/internal/aws"
)

// ---------------------------------------------------------------------------
// Step Functions (SFN) fetcher tests
// ---------------------------------------------------------------------------

func TestFetchStepFunctions_ParsesMultiple(t *testing.T) {
	now := time.Now()
	mock := &mockSFNClient{
		output: &sfn.ListStateMachinesOutput{
			StateMachines: []sfntypes.StateMachineListItem{
				{
					Name:            aws.String("order-processing"),
					StateMachineArn: aws.String("arn:aws:states:us-east-1:123456789012:stateMachine:order-processing"),
					Type:            sfntypes.StateMachineTypeStandard,
					CreationDate:    &now,
				},
				{
					Name:            aws.String("quick-task"),
					StateMachineArn: aws.String("arn:aws:states:us-east-1:123456789012:stateMachine:quick-task"),
					Type:            sfntypes.StateMachineTypeExpress,
					CreationDate:    &now,
				},
			},
		},
	}

	resources, err := awsclient.FetchStepFunctions(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}

	// Verify first state machine
	r0 := resources[0]
	if r0.ID != "order-processing" {
		t.Errorf("resource[0].ID: expected %q, got %q", "order-processing", r0.ID)
	}
	if r0.Name != "order-processing" {
		t.Errorf("resource[0].Name: expected %q, got %q", "order-processing", r0.Name)
	}

	// Verify required fields
	requiredFields := []string{"name", "arn", "type", "creation_date"}
	for i, r := range resources {
		for _, key := range requiredFields {
			if _, ok := r.Fields[key]; !ok {
				t.Errorf("resource[%d].Fields missing key %q", i, key)
			}
		}
	}

	if r0.Fields["type"] != "STANDARD" {
		t.Errorf("resource[0].Fields[\"type\"]: expected %q, got %q", "STANDARD", r0.Fields["type"])
	}
	if r0.Fields["arn"] != "arn:aws:states:us-east-1:123456789012:stateMachine:order-processing" {
		t.Errorf("resource[0].Fields[\"arn\"]: expected ARN, got %q", r0.Fields["arn"])
	}

	// Verify second state machine (express type)
	r1 := resources[1]
	if r1.Fields["type"] != "EXPRESS" {
		t.Errorf("resource[1].Fields[\"type\"]: expected %q, got %q", "EXPRESS", r1.Fields["type"])
	}
}

func TestFetchStepFunctions_RawStructPopulated(t *testing.T) {
	now := time.Now()
	mock := &mockSFNClient{
		output: &sfn.ListStateMachinesOutput{
			StateMachines: []sfntypes.StateMachineListItem{
				{
					Name:            aws.String("raw-sm"),
					StateMachineArn: aws.String("arn:aws:states:us-east-1:123456789012:stateMachine:raw-sm"),
					Type:            sfntypes.StateMachineTypeStandard,
					CreationDate:    &now,
				},
			},
		},
	}

	resources, err := awsclient.FetchStepFunctions(context.Background(), mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	r := resources[0]
	if r.RawStruct == nil {
		t.Fatal("RawStruct must not be nil")
	}
	sm, ok := r.RawStruct.(sfntypes.StateMachineListItem)
	if !ok {
		t.Fatalf("RawStruct should be sfntypes.StateMachineListItem, got %T", r.RawStruct)
	}
	if sm.Name == nil || *sm.Name != "raw-sm" {
		t.Errorf("RawStruct.Name: expected %q", "raw-sm")
	}
}

func TestFetchStepFunctions_ErrorResponse(t *testing.T) {
	mock := &mockSFNClient{
		err: fmt.Errorf("AWS API error: access denied"),
	}

	resources, err := awsclient.FetchStepFunctions(context.Background(), mock)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if resources != nil {
		t.Errorf("expected nil resources on error, got %d", len(resources))
	}
}

func TestFetchStepFunctions_EmptyResponse(t *testing.T) {
	mock := &mockSFNClient{
		output: &sfn.ListStateMachinesOutput{
			StateMachines: []sfntypes.StateMachineListItem{},
		},
	}

	resources, err := awsclient.FetchStepFunctions(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(resources))
	}
}
