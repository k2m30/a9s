package unit

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbtypes "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ---------------------------------------------------------------------------
// Local mock: ELBv2DescribeRulesAPI
// ---------------------------------------------------------------------------

type mockELBv2DescribeRulesClient struct {
	output *elbv2.DescribeRulesOutput
	err    error
}

func (m *mockELBv2DescribeRulesClient) DescribeRules(
	ctx context.Context,
	params *elbv2.DescribeRulesInput,
	optFns ...func(*elbv2.Options),
) (*elbv2.DescribeRulesOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.output, nil
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// TestFetchELBListenerRules_Basic verifies a forward rule with path-pattern
// condition returns correct ID, Name, Status, and Fields.
func TestFetchELBListenerRules_Basic(t *testing.T) {
	mock := &mockELBv2DescribeRulesClient{
		output: &elbv2.DescribeRulesOutput{
			Rules: []elbtypes.Rule{
				{
					RuleArn:  aws.String("arn:aws:elasticloadbalancing:us-east-1:123456789012:listener-rule/app/api-prod/abc123/def456/rule1"),
					Priority: aws.String("100"),
					Conditions: []elbtypes.RuleCondition{
						{
							Field: aws.String("path-pattern"),
							PathPatternConfig: &elbtypes.PathPatternConditionConfig{
								Values: []string{"/api/*"},
							},
						},
					},
					Actions: []elbtypes.Action{
						{
							Type:           elbtypes.ActionTypeEnumForward,
							TargetGroupArn: aws.String("arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/api-tg/abc123"),
						},
					},
					IsDefault: aws.Bool(false),
				},
			},
		},
	}

	parentCtx := map[string]string{
		"listener_arn":     "arn:aws:elasticloadbalancing:us-east-1:123456789012:listener/app/api-prod/abc123/def456",
		"listener_display": ":443 HTTPS",
	}

	resources, err := awsclient.FetchELBListenerRules(
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
	t.Run("ID_is_RuleArn", func(t *testing.T) {
		if r.ID != "arn:aws:elasticloadbalancing:us-east-1:123456789012:listener-rule/app/api-prod/abc123/def456/rule1" {
			t.Errorf("ID: expected rule ARN, got %q", r.ID)
		}
	})
	t.Run("priority_is_100", func(t *testing.T) {
		if r.Fields["priority"] != "100" {
			t.Errorf("Fields[priority]: expected %q, got %q", "100", r.Fields["priority"])
		}
	})
	t.Run("action_type_is_forward", func(t *testing.T) {
		if r.Fields["action_type"] != "forward" {
			t.Errorf("Fields[action_type]: expected %q, got %q", "forward", r.Fields["action_type"])
		}
	})
	t.Run("conditions_summary_contains_path", func(t *testing.T) {
		if !strings.Contains(r.Fields["conditions_summary"], "/api/*") {
			t.Errorf("Fields[conditions_summary]: expected to contain %q, got %q", "/api/*", r.Fields["conditions_summary"])
		}
	})
	t.Run("action_target_contains_tg", func(t *testing.T) {
		if r.Fields["action_target"] == "" {
			t.Error("Fields[action_target] should not be empty for forward action")
		}
	})
	t.Run("RawStruct_is_Rule", func(t *testing.T) {
		if r.RawStruct == nil {
			t.Fatal("RawStruct must not be nil")
		}
		rule, ok := r.RawStruct.(elbtypes.Rule)
		if !ok {
			t.Fatalf("RawStruct should be elbtypes.Rule, got %T", r.RawStruct)
		}
		if rule.RuleArn == nil {
			t.Error("RawStruct.RuleArn should not be nil")
		}
	})

	t.Run("required_fields_present", func(t *testing.T) {
		requiredFields := []string{"priority", "conditions_summary", "action_type", "action_target"}
		for _, key := range requiredFields {
			if _, ok := r.Fields[key]; !ok {
				t.Errorf("Fields missing key %q", key)
			}
		}
	})
}

// TestFetchELBListenerRules_MultipleConditions verifies that a rule with
// both path-pattern and host-header conditions joins them with AND.
func TestFetchELBListenerRules_MultipleConditions(t *testing.T) {
	mock := &mockELBv2DescribeRulesClient{
		output: &elbv2.DescribeRulesOutput{
			Rules: []elbtypes.Rule{
				{
					RuleArn:  aws.String("arn:rule/multi-cond"),
					Priority: aws.String("200"),
					Conditions: []elbtypes.RuleCondition{
						{
							Field: aws.String("path-pattern"),
							PathPatternConfig: &elbtypes.PathPatternConditionConfig{
								Values: []string{"/api/v2/*"},
							},
						},
						{
							Field: aws.String("host-header"),
							HostHeaderConfig: &elbtypes.HostHeaderConditionConfig{
								Values: []string{"api.example.com"},
							},
						},
					},
					Actions: []elbtypes.Action{
						{
							Type:           elbtypes.ActionTypeEnumForward,
							TargetGroupArn: aws.String("arn:tg/multi"),
						},
					},
					IsDefault: aws.Bool(false),
				},
			},
		},
	}

	parentCtx := map[string]string{"listener_arn": "arn:listener/multi"}
	resources, err := awsclient.FetchELBListenerRules(context.Background(), mock, parentCtx)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	summary := resources[0].Fields["conditions_summary"]
	if !strings.Contains(summary, "/api/v2/*") {
		t.Errorf("conditions_summary should contain path, got %q", summary)
	}
	if !strings.Contains(summary, "api.example.com") {
		t.Errorf("conditions_summary should contain host, got %q", summary)
	}
	// Should be joined with AND
	if !strings.Contains(summary, "AND") {
		t.Errorf("conditions_summary should join with AND, got %q", summary)
	}
}

// TestFetchELBListenerRules_DefaultRule verifies handling of the default rule
// (IsDefault=true, empty conditions, priority="default").
func TestFetchELBListenerRules_DefaultRule(t *testing.T) {
	mock := &mockELBv2DescribeRulesClient{
		output: &elbv2.DescribeRulesOutput{
			Rules: []elbtypes.Rule{
				{
					RuleArn:    aws.String("arn:rule/default"),
					Priority:   aws.String("default"),
					Conditions: []elbtypes.RuleCondition{},
					Actions: []elbtypes.Action{
						{
							Type:           elbtypes.ActionTypeEnumForward,
							TargetGroupArn: aws.String("arn:tg/default"),
						},
					},
					IsDefault: aws.Bool(true),
				},
			},
		},
	}

	parentCtx := map[string]string{"listener_arn": "arn:listener/default"}
	resources, err := awsclient.FetchELBListenerRules(context.Background(), mock, parentCtx)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	r := resources[0]
	if r.Fields["priority"] != "default" {
		t.Errorf("Fields[priority]: expected %q, got %q", "default", r.Fields["priority"])
	}
}

// TestFetchELBListenerRules_RedirectAction verifies redirect action parsing.
func TestFetchELBListenerRules_RedirectAction(t *testing.T) {
	mock := &mockELBv2DescribeRulesClient{
		output: &elbv2.DescribeRulesOutput{
			Rules: []elbtypes.Rule{
				{
					RuleArn:    aws.String("arn:rule/redirect"),
					Priority:   aws.String("50"),
					Conditions: []elbtypes.RuleCondition{},
					Actions: []elbtypes.Action{
						{
							Type: elbtypes.ActionTypeEnumRedirect,
							RedirectConfig: &elbtypes.RedirectActionConfig{
								Protocol:   aws.String("HTTPS"),
								Port:       aws.String("443"),
								StatusCode: elbtypes.RedirectActionStatusCodeEnumHttp301,
							},
						},
					},
					IsDefault: aws.Bool(false),
				},
			},
		},
	}

	parentCtx := map[string]string{"listener_arn": "arn:listener/redirect"}
	resources, err := awsclient.FetchELBListenerRules(context.Background(), mock, parentCtx)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	r := resources[0]
	if r.Fields["action_type"] != "redirect" {
		t.Errorf("Fields[action_type]: expected %q, got %q", "redirect", r.Fields["action_type"])
	}
	if r.Fields["action_target"] == "" {
		t.Error("Fields[action_target] should not be empty for redirect action")
	}
}

// TestFetchELBListenerRules_FixedResponseAction verifies fixed-response action parsing.
func TestFetchELBListenerRules_FixedResponseAction(t *testing.T) {
	mock := &mockELBv2DescribeRulesClient{
		output: &elbv2.DescribeRulesOutput{
			Rules: []elbtypes.Rule{
				{
					RuleArn:    aws.String("arn:rule/fixed"),
					Priority:   aws.String("10"),
					Conditions: []elbtypes.RuleCondition{},
					Actions: []elbtypes.Action{
						{
							Type: elbtypes.ActionTypeEnumFixedResponse,
							FixedResponseConfig: &elbtypes.FixedResponseActionConfig{
								StatusCode: aws.String("503"),
							},
						},
					},
					IsDefault: aws.Bool(false),
				},
			},
		},
	}

	parentCtx := map[string]string{"listener_arn": "arn:listener/fixed"}
	resources, err := awsclient.FetchELBListenerRules(context.Background(), mock, parentCtx)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	r := resources[0]
	if r.Fields["action_type"] != "fixed-response" {
		t.Errorf("Fields[action_type]: expected %q, got %q", "fixed-response", r.Fields["action_type"])
	}
	if !strings.Contains(r.Fields["action_target"], "503") {
		t.Errorf("Fields[action_target] should contain status code 503, got %q", r.Fields["action_target"])
	}
}

// TestFetchELBListenerRules_EmptyResponse verifies empty DescribeRules response.
func TestFetchELBListenerRules_EmptyResponse(t *testing.T) {
	mock := &mockELBv2DescribeRulesClient{
		output: &elbv2.DescribeRulesOutput{
			Rules: []elbtypes.Rule{},
		},
	}

	parentCtx := map[string]string{"listener_arn": "arn:listener/empty"}
	resources, err := awsclient.FetchELBListenerRules(context.Background(), mock, parentCtx)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(resources))
	}
}

// TestFetchELBListenerRules_APIError verifies that API errors are propagated.
func TestFetchELBListenerRules_APIError(t *testing.T) {
	mock := &mockELBv2DescribeRulesClient{
		err: fmt.Errorf("AWS API error: listener not found"),
	}

	parentCtx := map[string]string{"listener_arn": "arn:listener/err"}
	_, err := awsclient.FetchELBListenerRules(context.Background(), mock, parentCtx)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
}

// TestFetchELBListenerRules_NilFields verifies that nil Priority, nil Conditions,
// nil Actions, and nil IsDefault do not cause a panic.
func TestFetchELBListenerRules_NilFields(t *testing.T) {
	mock := &mockELBv2DescribeRulesClient{
		output: &elbv2.DescribeRulesOutput{
			Rules: []elbtypes.Rule{
				{
					RuleArn: aws.String("arn:rule/nil-fields"),
					// Priority is nil
					// Conditions is nil
					// Actions is nil
					// IsDefault is nil
				},
			},
		},
	}

	parentCtx := map[string]string{"listener_arn": "arn:listener/nil"}
	resources, err := awsclient.FetchELBListenerRules(context.Background(), mock, parentCtx)
	if err != nil {
		t.Fatalf("expected no error for nil fields, got %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}
}

// ---------------------------------------------------------------------------
// BuildConditionsSummary helper tests
// ---------------------------------------------------------------------------

// TestBuildConditionsSummary_PathPattern verifies path-pattern condition summary.
func TestBuildConditionsSummary_PathPattern(t *testing.T) {
	conditions := []elbtypes.RuleCondition{
		{
			Field: aws.String("path-pattern"),
			PathPatternConfig: &elbtypes.PathPatternConditionConfig{
				Values: []string{"/api/*"},
			},
		},
	}
	summary := awsclient.BuildConditionsSummary(conditions)
	if !strings.Contains(summary, "/api/*") {
		t.Errorf("expected summary to contain %q, got %q", "/api/*", summary)
	}
}

// TestBuildConditionsSummary_HostHeader verifies host-header condition summary.
func TestBuildConditionsSummary_HostHeader(t *testing.T) {
	conditions := []elbtypes.RuleCondition{
		{
			Field: aws.String("host-header"),
			HostHeaderConfig: &elbtypes.HostHeaderConditionConfig{
				Values: []string{"example.com"},
			},
		},
	}
	summary := awsclient.BuildConditionsSummary(conditions)
	if !strings.Contains(summary, "example.com") {
		t.Errorf("expected summary to contain %q, got %q", "example.com", summary)
	}
}

// TestBuildConditionsSummary_SourceIP verifies source-ip condition summary.
func TestBuildConditionsSummary_SourceIP(t *testing.T) {
	conditions := []elbtypes.RuleCondition{
		{
			Field: aws.String("source-ip"),
			SourceIpConfig: &elbtypes.SourceIpConditionConfig{
				Values: []string{"10.0.0.0/8"},
			},
		},
	}
	summary := awsclient.BuildConditionsSummary(conditions)
	if !strings.Contains(summary, "10.0.0.0/8") {
		t.Errorf("expected summary to contain %q, got %q", "10.0.0.0/8", summary)
	}
}

// TestBuildConditionsSummary_Combined verifies multiple conditions joined with AND.
func TestBuildConditionsSummary_Combined(t *testing.T) {
	conditions := []elbtypes.RuleCondition{
		{
			Field: aws.String("path-pattern"),
			PathPatternConfig: &elbtypes.PathPatternConditionConfig{
				Values: []string{"/api/*"},
			},
		},
		{
			Field: aws.String("host-header"),
			HostHeaderConfig: &elbtypes.HostHeaderConditionConfig{
				Values: []string{"api.example.com"},
			},
		},
	}
	summary := awsclient.BuildConditionsSummary(conditions)
	if !strings.Contains(summary, "/api/*") {
		t.Errorf("expected summary to contain path, got %q", summary)
	}
	if !strings.Contains(summary, "api.example.com") {
		t.Errorf("expected summary to contain host, got %q", summary)
	}
	if !strings.Contains(summary, "AND") {
		t.Errorf("expected AND separator in combined conditions, got %q", summary)
	}
}

// TestBuildConditionsSummary_Empty verifies empty conditions return empty string.
func TestBuildConditionsSummary_Empty(t *testing.T) {
	summary := awsclient.BuildConditionsSummary([]elbtypes.RuleCondition{})
	if summary != "" {
		t.Errorf("expected empty summary for empty conditions, got %q", summary)
	}
}

// ---------------------------------------------------------------------------
// Column and registration tests
// ---------------------------------------------------------------------------

// TestELBListenerRuleColumns verifies the column count, keys, titles, and widths.
func TestELBListenerRuleColumns(t *testing.T) {
	cols := resource.ELBListenerRuleColumns()

	if len(cols) != 4 {
		t.Fatalf("ELBListenerRuleColumns() returned %d columns, expected 4", len(cols))
	}

	wantCols := []struct {
		key   string
		title string
		width int
	}{
		{"priority", "Priority", 10},
		{"conditions_summary", "Conditions", 36},
		{"action_type", "Action", 16},
		{"action_target", "Target", 32},
	}

	for i, want := range wantCols {
		if i >= len(cols) {
			t.Errorf("Missing column at index %d", i)
			continue
		}
		if cols[i].Key != want.key {
			t.Errorf("Column %d Key: expected %q, got %q", i, want.key, cols[i].Key)
		}
		if cols[i].Title != want.title {
			t.Errorf("Column %d Title: expected %q, got %q", i, want.title, cols[i].Title)
		}
		if cols[i].Width != want.width {
			t.Errorf("Column %d Width: expected %d, got %d", i, want.width, cols[i].Width)
		}
	}
}

// TestELBListenerRules_ChildFetcherRegistered verifies that the child fetcher
// is registered under the correct short name.
func TestELBListenerRules_ChildFetcherRegistered(t *testing.T) {
	f := resource.GetChildFetcher("elb_listener_rules")
	if f == nil {
		t.Fatal("elb_listener_rules child fetcher not registered")
	}
}

// TestELBListenerRules_ParentListenerHasChildDef verifies that the
// elb_listeners child type has a Children entry for elb_listener_rules.
func TestELBListenerRules_ParentListenerHasChildDef(t *testing.T) {
	listenerType := resource.GetChildType("elb_listeners")
	if listenerType == nil {
		t.Fatal("elb_listeners child type not registered")
	}

	found := false
	for _, child := range listenerType.Children {
		if child.ChildType == "elb_listener_rules" {
			found = true
			if child.Key != "enter" {
				t.Errorf("elb_listener_rules child def Key: expected %q, got %q", "enter", child.Key)
			}
			if child.ContextKeys["listener_arn"] != "ID" {
				t.Errorf("elb_listener_rules ContextKeys[listener_arn]: expected %q, got %q",
					"ID", child.ContextKeys["listener_arn"])
			}
			if child.DisplayNameKey != "listener_display" {
				t.Errorf("elb_listener_rules DisplayNameKey: expected %q, got %q",
					"listener_display", child.DisplayNameKey)
			}
			break
		}
	}
	if !found {
		t.Error("elb_listeners child type missing Children entry for elb_listener_rules")
	}
}

// TestELBListenerDisplayField verifies that elb_listeners RegisterFieldKeys
// includes "listener_display".
func TestELBListenerDisplayField(t *testing.T) {
	keys := resource.GetFieldKeys("elb_listeners")
	if keys == nil {
		t.Fatal("elb_listeners field keys not registered")
	}
	found := false
	for _, k := range keys {
		if k == "listener_display" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("elb_listeners field keys missing 'listener_display', got: %v", keys)
	}
}
