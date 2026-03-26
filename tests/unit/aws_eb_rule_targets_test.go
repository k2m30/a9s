package unit

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	ebtypes "github.com/aws/aws-sdk-go-v2/service/eventbridge/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ---------------------------------------------------------------------------
// EventBridge Rule Targets fetcher tests (child of EventBridge Rules)
// ---------------------------------------------------------------------------

// TestFetchEventBridgeRuleTargets_Basic verifies parsing of 2 targets
// (Lambda + SQS), checking ID, Name, all Fields, and RawStruct.
func TestFetchEventBridgeRuleTargets_Basic(t *testing.T) {
	mock := &mockEventBridgeListTargetsClient{
		output: &eventbridge.ListTargetsByRuleOutput{
			Targets: []ebtypes.Target{
				{
					Id:      aws.String("lambda-target-1"),
					Arn:     aws.String("arn:aws:lambda:us-east-1:123456789012:function:data-pipeline-daily"),
					RoleArn: aws.String("arn:aws:iam::123456789012:role/EventBridgeLambdaRole"),
				},
				{
					Id:      aws.String("sqs-target-2"),
					Arn:     aws.String("arn:aws:sqs:us-east-1:123456789012:processing-queue"),
					RoleArn: aws.String("arn:aws:iam::123456789012:role/EventBridgeSQSRole"),
					Input:   aws.String(`{"source":"eventbridge"}`),
				},
			},
		},
	}

	parentCtx := map[string]string{
		"rule_name": "daily-backup",
		"event_bus": "default",
	}

	resources, err := awsclient.FetchEventBridgeRuleTargets(
		context.Background(),
		mock,
		parentCtx,
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}

	// Verify first target (Lambda)
	r0 := resources[0]
	t.Run("ID_is_target_id", func(t *testing.T) {
		if r0.ID != "lambda-target-1" {
			t.Errorf("resource[0].ID: expected %q, got %q", "lambda-target-1", r0.ID)
		}
	})

	t.Run("Name_is_target_id", func(t *testing.T) {
		if r0.Name != "lambda-target-1" {
			t.Errorf("resource[0].Name: expected %q, got %q", "lambda-target-1", r0.Name)
		}
	})

	t.Run("Fields_target_id", func(t *testing.T) {
		if r0.Fields["target_id"] != "lambda-target-1" {
			t.Errorf("Fields[target_id]: expected %q, got %q", "lambda-target-1", r0.Fields["target_id"])
		}
	})

	t.Run("Fields_target_arn", func(t *testing.T) {
		expected := "arn:aws:lambda:us-east-1:123456789012:function:data-pipeline-daily"
		if r0.Fields["target_arn"] != expected {
			t.Errorf("Fields[target_arn]: expected %q, got %q", expected, r0.Fields["target_arn"])
		}
	})

	t.Run("Fields_role_arn", func(t *testing.T) {
		expected := "arn:aws:iam::123456789012:role/EventBridgeLambdaRole"
		if r0.Fields["role_arn"] != expected {
			t.Errorf("Fields[role_arn]: expected %q, got %q", expected, r0.Fields["role_arn"])
		}
	})

	t.Run("Fields_resource_type_name_lambda", func(t *testing.T) {
		if r0.Fields["resource_type_name"] != "Lambda: data-pipeline-daily" {
			t.Errorf("Fields[resource_type_name]: expected %q, got %q", "Lambda: data-pipeline-daily", r0.Fields["resource_type_name"])
		}
	})

	t.Run("Fields_input_summary_lambda_no_input", func(t *testing.T) {
		// Lambda target has no Input/InputPath/InputTransformer, so em-dash
		if r0.Fields["input_summary"] != "\u2014" {
			t.Errorf("Fields[input_summary]: expected em-dash, got %q", r0.Fields["input_summary"])
		}
	})

	// Verify second target (SQS)
	r1 := resources[1]
	t.Run("SQS_resource_type_name", func(t *testing.T) {
		if r1.Fields["resource_type_name"] != "SQS: processing-queue" {
			t.Errorf("Fields[resource_type_name]: expected %q, got %q", "SQS: processing-queue", r1.Fields["resource_type_name"])
		}
	})

	t.Run("SQS_input_summary_constant", func(t *testing.T) {
		expected := `{"source":"eventbridge"}`
		if r1.Fields["input_summary"] != expected {
			t.Errorf("Fields[input_summary]: expected %q, got %q", expected, r1.Fields["input_summary"])
		}
	})

	t.Run("RawStruct_is_Target", func(t *testing.T) {
		if r0.RawStruct == nil {
			t.Fatal("RawStruct must not be nil")
		}
		raw, ok := r0.RawStruct.(ebtypes.Target)
		if !ok {
			t.Fatalf("RawStruct should be ebtypes.Target, got %T", r0.RawStruct)
		}
		if raw.Id == nil || *raw.Id != "lambda-target-1" {
			t.Error("RawStruct.Id not preserved correctly")
		}
	})

	// Verify required fields are present
	t.Run("required_fields_present", func(t *testing.T) {
		requiredFields := []string{"target_id", "target_arn", "role_arn", "resource_type_name", "input_summary"}
		for i, r := range resources {
			for _, key := range requiredFields {
				if _, ok := r.Fields[key]; !ok {
					t.Errorf("resource[%d].Fields missing key %q", i, key)
				}
			}
		}
	})
}

// TestFetchEventBridgeRuleTargets_Empty verifies that a rule with no targets
// returns an empty slice with no error.
func TestFetchEventBridgeRuleTargets_Empty(t *testing.T) {
	mock := &mockEventBridgeListTargetsClient{
		output: &eventbridge.ListTargetsByRuleOutput{
			Targets: []ebtypes.Target{},
		},
	}

	parentCtx := map[string]string{
		"rule_name": "empty-rule",
		"event_bus": "default",
	}

	resources, err := awsclient.FetchEventBridgeRuleTargets(
		context.Background(),
		mock,
		parentCtx,
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(resources))
	}
}

// TestFetchEventBridgeRuleTargets_APIError verifies that API errors are propagated.
func TestFetchEventBridgeRuleTargets_APIError(t *testing.T) {
	mock := &mockEventBridgeListTargetsClient{
		err: fmt.Errorf("AWS API error: access denied"),
	}

	parentCtx := map[string]string{
		"rule_name": "error-rule",
		"event_bus": "default",
	}

	resources, err := awsclient.FetchEventBridgeRuleTargets(
		context.Background(),
		mock,
		parentCtx,
	)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if resources != nil {
		t.Errorf("expected nil resources on error, got %d", len(resources))
	}
}

// TestFetchEventBridgeRuleTargets_NilFields verifies that nil optional fields
// (Id, Arn, RoleArn are *string) do not cause a panic and produce empty strings.
func TestFetchEventBridgeRuleTargets_NilFields(t *testing.T) {
	mock := &mockEventBridgeListTargetsClient{
		output: &eventbridge.ListTargetsByRuleOutput{
			Targets: []ebtypes.Target{
				{
					// All *string fields nil
				},
			},
		},
	}

	parentCtx := map[string]string{
		"rule_name": "nil-fields-rule",
		"event_bus": "default",
	}

	// Should not panic
	resources, err := awsclient.FetchEventBridgeRuleTargets(
		context.Background(),
		mock,
		parentCtx,
	)
	if err != nil {
		t.Fatalf("expected no error for nil fields, got %v", err)
	}

	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	r := resources[0]

	t.Run("nil_Id_empty_string", func(t *testing.T) {
		if r.ID != "" {
			t.Errorf("ID should be empty for nil Id, got %q", r.ID)
		}
	})

	t.Run("nil_Arn_empty_fields", func(t *testing.T) {
		if r.Fields["target_arn"] != "" {
			t.Errorf("Fields[target_arn] should be empty for nil Arn, got %q", r.Fields["target_arn"])
		}
	})

	t.Run("nil_RoleArn_empty_fields", func(t *testing.T) {
		if r.Fields["role_arn"] != "" {
			t.Errorf("Fields[role_arn] should be empty for nil RoleArn, got %q", r.Fields["role_arn"])
		}
	})

	t.Run("input_summary_em_dash_for_no_input", func(t *testing.T) {
		if r.Fields["input_summary"] != "\u2014" {
			t.Errorf("Fields[input_summary]: expected em-dash, got %q", r.Fields["input_summary"])
		}
	})
}

// TestFetchEventBridgeRuleTargets_RawStruct verifies that RawStruct preserves
// the original ebtypes.Target, including all sub-fields.
func TestFetchEventBridgeRuleTargets_RawStruct(t *testing.T) {
	mock := &mockEventBridgeListTargetsClient{
		output: &eventbridge.ListTargetsByRuleOutput{
			Targets: []ebtypes.Target{
				{
					Id:      aws.String("raw-target"),
					Arn:     aws.String("arn:aws:lambda:us-east-1:123456789012:function:my-func"),
					RoleArn: aws.String("arn:aws:iam::123456789012:role/MyRole"),
					Input:   aws.String(`{"key":"value"}`),
				},
			},
		},
	}

	parentCtx := map[string]string{
		"rule_name": "raw-rule",
		"event_bus": "default",
	}

	resources, err := awsclient.FetchEventBridgeRuleTargets(
		context.Background(),
		mock,
		parentCtx,
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	r := resources[0]
	if r.RawStruct == nil {
		t.Fatal("RawStruct must not be nil")
	}

	raw, ok := r.RawStruct.(ebtypes.Target)
	if !ok {
		t.Fatalf("RawStruct should be ebtypes.Target, got %T", r.RawStruct)
	}

	t.Run("Id_preserved", func(t *testing.T) {
		if raw.Id == nil || *raw.Id != "raw-target" {
			t.Error("RawStruct.Id not preserved correctly")
		}
	})

	t.Run("Arn_preserved", func(t *testing.T) {
		if raw.Arn == nil || *raw.Arn != "arn:aws:lambda:us-east-1:123456789012:function:my-func" {
			t.Error("RawStruct.Arn not preserved correctly")
		}
	})

	t.Run("RoleArn_preserved", func(t *testing.T) {
		if raw.RoleArn == nil || *raw.RoleArn != "arn:aws:iam::123456789012:role/MyRole" {
			t.Error("RawStruct.RoleArn not preserved correctly")
		}
	})

	t.Run("Input_preserved", func(t *testing.T) {
		if raw.Input == nil || *raw.Input != `{"key":"value"}` {
			t.Error("RawStruct.Input not preserved correctly")
		}
	})
}

// ---------------------------------------------------------------------------
// ArnToResourceName helper tests
// ---------------------------------------------------------------------------

// TestArnToResourceName is a table-driven test covering all ARN parsing examples
// from the architect spec, plus edge cases.
func TestArnToResourceName(t *testing.T) {
	tests := []struct {
		name     string
		arn      string
		expected string
	}{
		{
			name:     "lambda_function",
			arn:      "arn:aws:lambda:us-east-1:123456789012:function:data-pipeline-daily",
			expected: "Lambda: data-pipeline-daily",
		},
		{
			name:     "sqs_queue",
			arn:      "arn:aws:sqs:us-east-1:123456789012:processing-queue",
			expected: "SQS: processing-queue",
		},
		{
			name:     "sfn_state_machine",
			arn:      "arn:aws:states:us-east-1:123456789012:stateMachine:order-workflow",
			expected: "SFN: order-workflow",
		},
		{
			name:     "ecs_cluster",
			arn:      "arn:aws:ecs:us-east-1:123456789012:cluster/prod",
			expected: "ECS: prod",
		},
		{
			name:     "sns_topic",
			arn:      "arn:aws:sns:us-east-1:123456789012:my-topic",
			expected: "SNS: my-topic",
		},
		{
			name:     "kinesis_stream",
			arn:      "arn:aws:kinesis:us-east-1:123456789012:stream/clicks",
			expected: "Kinesis: clicks",
		},
		{
			name:     "empty_arn",
			arn:      "",
			expected: "",
		},
		{
			name:     "not_an_arn",
			arn:      "not-an-arn",
			expected: "not-an-arn",
		},
		{
			name:     "unknown_service_arn",
			arn:      "arn:aws:logs:us-east-1:123456789012:log-group:/aws/lambda/my-func",
			expected: "logs: /aws/lambda/my-func",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := awsclient.ArnToResourceName(tt.arn)
			if result != tt.expected {
				t.Errorf("ArnToResourceName(%q): expected %q, got %q", tt.arn, tt.expected, result)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ComputeInputSummary helper tests
// ---------------------------------------------------------------------------

// TestComputeInputSummary is a table-driven test covering all input summary
// priority cases from the architect spec.
func TestComputeInputSummary(t *testing.T) {
	tests := []struct {
		name     string
		target   ebtypes.Target
		expected string
	}{
		{
			name:     "no_input_em_dash",
			target:   ebtypes.Target{},
			expected: "\u2014",
		},
		{
			name: "constant_input_short",
			target: ebtypes.Target{
				Input: aws.String(`{"source":"eventbridge"}`),
			},
			expected: `{"source":"eventbridge"}`,
		},
		{
			name: "constant_input_long_truncated",
			target: ebtypes.Target{
				Input: aws.String(`{"source":"eventbridge","detail-type":"Scheduled Event","resources":["arn:aws:events:us-east-1:123456789012:rule/my-rule"]}`),
			},
			expected: `{"source":"eventbridge","detail-ty` + "...",
		},
		{
			name: "input_path",
			target: ebtypes.Target{
				InputPath: aws.String("$.detail"),
			},
			expected: "$.detail",
		},
		{
			name: "input_transformer",
			target: ebtypes.Target{
				InputTransformer: &ebtypes.InputTransformer{
					InputTemplate: aws.String(`"<instance> is in state <state>"`),
				},
			},
			expected: "InputTransformer",
		},
		{
			name: "transformer_precedence_over_input",
			target: ebtypes.Target{
				InputTransformer: &ebtypes.InputTransformer{
					InputTemplate: aws.String(`"<instance>"`),
				},
				InputPath: aws.String("$.detail"),
				Input:     aws.String(`{"key":"value"}`),
			},
			expected: "InputTransformer",
		},
		{
			name: "input_path_precedence_over_input",
			target: ebtypes.Target{
				InputPath: aws.String("$.detail.state"),
				Input:     aws.String(`{"key":"value"}`),
			},
			expected: "$.detail.state",
		},
		{
			name: "empty_input_path_falls_through",
			target: ebtypes.Target{
				InputPath: aws.String(""),
				Input:     aws.String(`{"key":"value"}`),
			},
			expected: `{"key":"value"}`,
		},
		{
			name: "empty_input_falls_through",
			target: ebtypes.Target{
				Input: aws.String(""),
			},
			expected: "\u2014",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := awsclient.ComputeInputSummary(tt.target)
			if result != tt.expected {
				t.Errorf("ComputeInputSummary: expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Column definition tests
// ---------------------------------------------------------------------------

// TestEbRuleTargetColumns verifies that EbRuleTargetColumns returns the expected
// columns with correct keys, titles, widths, and sortability.
func TestEbRuleTargetColumns(t *testing.T) {
	cols := resource.EbRuleTargetColumns()

	expectedKeys := []string{"target_id", "target_arn", "resource_type_name", "input_summary"}

	t.Run("column_count", func(t *testing.T) {
		if len(cols) != 4 {
			t.Fatalf("expected 4 columns, got %d", len(cols))
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

	t.Run("expected_widths", func(t *testing.T) {
		expectedWidths := []int{20, 48, 28, 36}
		for i, expected := range expectedWidths {
			if cols[i].Width != expected {
				t.Errorf("column[%d] (%s).Width: expected %d, got %d", i, cols[i].Key, expected, cols[i].Width)
			}
		}
	})

	t.Run("input_summary_not_sortable", func(t *testing.T) {
		// input_summary is the 4th column (index 3)
		if cols[3].Sortable {
			t.Error("input_summary column should NOT be sortable")
		}
	})

	t.Run("sortable_columns", func(t *testing.T) {
		// target_id, target_arn, resource_type_name are sortable
		for _, idx := range []int{0, 1, 2} {
			if !cols[idx].Sortable {
				t.Errorf("column[%d] (%s) should be sortable", idx, cols[idx].Key)
			}
		}
	})
}

// ---------------------------------------------------------------------------
// Registration tests
// ---------------------------------------------------------------------------

// TestEbRuleTargets_ChildTypeRegistered verifies that the child type is
// registered under the correct short name.
func TestEbRuleTargets_ChildTypeRegistered(t *testing.T) {
	td := resource.GetChildType("eb_rule_targets")
	if td == nil {
		t.Fatal("eb_rule_targets child resource type not registered")
	}
	if td.Name == "" {
		t.Error("child type Name should not be empty")
	}
	if td.ShortName != "eb_rule_targets" {
		t.Errorf("child type ShortName: expected %q, got %q", "eb_rule_targets", td.ShortName)
	}
}

// TestEbRuleTargets_ChildFetcherRegistered verifies that the child fetcher is
// registered under the correct short name.
func TestEbRuleTargets_ChildFetcherRegistered(t *testing.T) {
	f := resource.GetChildFetcher("eb_rule_targets")
	if f == nil {
		t.Fatal("eb_rule_targets child fetcher not registered")
	}
}

// TestEbRuleTargets_ParentHasChildDef verifies that the parent eb-rule resource
// type has a child view definition for eb_rule_targets with key "enter".
func TestEbRuleTargets_ParentHasChildDef(t *testing.T) {
	rt := resource.FindResourceType("eb-rule")
	if rt == nil {
		t.Fatal("eb-rule resource type not found")
	}

	found := false
	for _, child := range rt.Children {
		if child.ChildType == "eb_rule_targets" {
			found = true
			if child.Key != "enter" {
				t.Errorf("expected key %q, got %q", "enter", child.Key)
			}
			if child.ContextKeys["rule_name"] != "ID" {
				t.Errorf("ContextKeys[rule_name]: expected %q, got %q", "ID", child.ContextKeys["rule_name"])
			}
			if child.ContextKeys["event_bus"] != "event_bus" {
				t.Errorf("ContextKeys[event_bus]: expected %q, got %q", "event_bus", child.ContextKeys["event_bus"])
			}
			if child.DisplayNameKey != "rule_name" {
				t.Errorf("DisplayNameKey: expected %q, got %q", "rule_name", child.DisplayNameKey)
			}
		}
	}
	if !found {
		t.Error("eb-rule Children should contain eb_rule_targets child view def")
	}
}

// TestEbRuleTargets_CopyField verifies that the eb_rule_targets child type
// has CopyField set to "target_arn".
func TestEbRuleTargets_CopyField(t *testing.T) {
	td := resource.GetChildType("eb_rule_targets")
	if td == nil {
		t.Fatal("eb_rule_targets child type not found")
	}
	if td.CopyField != "target_arn" {
		t.Errorf("CopyField: expected %q, got %q", "target_arn", td.CopyField)
	}
}

// ---------------------------------------------------------------------------
// ArnToResourceName edge case: unknown service uses raw service name
// ---------------------------------------------------------------------------

func TestArnToResourceName_UnknownService(t *testing.T) {
	result := awsclient.ArnToResourceName("arn:aws:codecommit:us-east-1:123456789012:my-repo")
	if !strings.Contains(result, "codecommit") {
		t.Errorf("expected unknown service to include service name, got %q", result)
	}
	if !strings.Contains(result, "my-repo") {
		t.Errorf("expected unknown service to include resource name, got %q", result)
	}
}
