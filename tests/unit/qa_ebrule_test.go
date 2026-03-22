package unit

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	ebtypes "github.com/aws/aws-sdk-go-v2/service/eventbridge/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
)

// ---------------------------------------------------------------------------
// EventBridge Rules fetcher tests
// ---------------------------------------------------------------------------

func TestFetchEventBridgeRules_ParsesMultiple(t *testing.T) {
	mock := &mockEventBridgeClient{
		output: &eventbridge.ListRulesOutput{
			Rules: []ebtypes.Rule{
				{
					Name:               aws.String("daily-backup"),
					Arn:                aws.String("arn:aws:events:us-east-1:123456789012:rule/daily-backup"),
					State:              ebtypes.RuleStateEnabled,
					Description:        aws.String("Daily backup trigger"),
					ScheduleExpression: aws.String("rate(1 day)"),
					EventBusName:       aws.String("default"),
				},
				{
					Name:         aws.String("ec2-state-change"),
					Arn:          aws.String("arn:aws:events:us-east-1:123456789012:rule/ec2-state-change"),
					State:        ebtypes.RuleStateDisabled,
					Description:  aws.String("EC2 state change notifications"),
					EventPattern: aws.String("{\"source\":[\"aws.ec2\"]}"),
					EventBusName: aws.String("default"),
				},
			},
		},
	}

	resources, err := awsclient.FetchEventBridgeRules(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}

	// Verify first rule
	r0 := resources[0]
	if r0.ID != "daily-backup" {
		t.Errorf("resource[0].ID: expected %q, got %q", "daily-backup", r0.ID)
	}
	if r0.Name != "daily-backup" {
		t.Errorf("resource[0].Name: expected %q, got %q", "daily-backup", r0.Name)
	}
	if r0.Status != "ENABLED" {
		t.Errorf("resource[0].Status: expected %q, got %q", "ENABLED", r0.Status)
	}

	// Verify required fields
	requiredFields := []string{"name", "state", "description", "event_bus", "schedule"}
	for i, r := range resources {
		for _, key := range requiredFields {
			if _, ok := r.Fields[key]; !ok {
				t.Errorf("resource[%d].Fields missing key %q", i, key)
			}
		}
	}

	if r0.Fields["schedule"] != "rate(1 day)" {
		t.Errorf("resource[0].Fields[\"schedule\"]: expected %q, got %q", "rate(1 day)", r0.Fields["schedule"])
	}
	if r0.Fields["event_bus"] != "default" {
		t.Errorf("resource[0].Fields[\"event_bus\"]: expected %q, got %q", "default", r0.Fields["event_bus"])
	}

	// Verify second rule (disabled, event pattern instead of schedule)
	r1 := resources[1]
	if r1.Status != "DISABLED" {
		t.Errorf("resource[1].Status: expected %q, got %q", "DISABLED", r1.Status)
	}
	if r1.Fields["schedule"] != "" {
		t.Errorf("resource[1].Fields[\"schedule\"]: expected empty, got %q", r1.Fields["schedule"])
	}
}

func TestFetchEventBridgeRules_RawStructPopulated(t *testing.T) {
	mock := &mockEventBridgeClient{
		output: &eventbridge.ListRulesOutput{
			Rules: []ebtypes.Rule{
				{
					Name:         aws.String("raw-rule"),
					Arn:          aws.String("arn:aws:events:us-east-1:123456789012:rule/raw-rule"),
					State:        ebtypes.RuleStateEnabled,
					EventBusName: aws.String("default"),
				},
			},
		},
	}

	resources, err := awsclient.FetchEventBridgeRules(context.Background(), mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	r := resources[0]
	if r.RawStruct == nil {
		t.Fatal("RawStruct must not be nil")
	}
	rule, ok := r.RawStruct.(ebtypes.Rule)
	if !ok {
		t.Fatalf("RawStruct should be ebtypes.Rule, got %T", r.RawStruct)
	}
	if rule.Name == nil || *rule.Name != "raw-rule" {
		t.Errorf("RawStruct.Name: expected %q", "raw-rule")
	}
}

func TestFetchEventBridgeRules_ErrorResponse(t *testing.T) {
	mock := &mockEventBridgeClient{
		err: fmt.Errorf("AWS API error: access denied"),
	}

	resources, err := awsclient.FetchEventBridgeRules(context.Background(), mock)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if resources != nil {
		t.Errorf("expected nil resources on error, got %d", len(resources))
	}
}

func TestFetchEventBridgeRules_EmptyResponse(t *testing.T) {
	mock := &mockEventBridgeClient{
		output: &eventbridge.ListRulesOutput{
			Rules: []ebtypes.Rule{},
		},
	}

	resources, err := awsclient.FetchEventBridgeRules(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(resources))
	}
}
