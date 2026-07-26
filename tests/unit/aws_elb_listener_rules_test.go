package unit

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbtypes "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
)

type mockELBv2DescribeRulesClient struct {
	output *elbv2.DescribeRulesOutput
	err    error
}

func (m *mockELBv2DescribeRulesClient) DescribeRules(ctx context.Context, params *elbv2.DescribeRulesInput, optFns ...func(*elbv2.Options)) (*elbv2.DescribeRulesOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.output, nil
}

func TestFetchELBListenerRules_Basic(t *testing.T) {
	mock := &mockELBv2DescribeRulesClient{output: &elbv2.DescribeRulesOutput{Rules: []elbtypes.Rule{{RuleArn: aws.String("arn:aws:elasticloadbalancing:us-east-1:123456789012:listener-rule/app/api-prod/abc123/def456/rule1"), Priority: aws.String("100"), Conditions: []elbtypes.RuleCondition{{Field: aws.String("path-pattern"), PathPatternConfig: &elbtypes.PathPatternConditionConfig{Values: []string{"/api/*"}}}}, Actions: []elbtypes.Action{{Type: elbtypes.ActionTypeEnumForward, TargetGroupArn: aws.String("arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/api-tg/abc123")}}, IsDefault: aws.Bool(false)}}}}
	result, err := awsclient.FetchELBListenerRules(context.Background(), mock, map[string]string{"listener_arn": "arn:aws:elasticloadbalancing:us-east-1:123456789012:listener/app/api-prod/abc123/def456", "listener_display": ":443 HTTPS"}, "")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	rs := result.Resources
	if len(rs) != 1 {
		t.Fatalf("expected 1, got %d", len(rs))
	}
	r := rs[0]
	if r.ID != "arn:aws:elasticloadbalancing:us-east-1:123456789012:listener-rule/app/api-prod/abc123/def456/rule1" {
		t.Errorf("ID: got %q", r.ID)
	}
	if r.Fields["priority"] != "100" {
		t.Errorf("priority: got %q", r.Fields["priority"])
	}
	if r.Fields["action_type"] != "forward" {
		t.Errorf("action_type: got %q", r.Fields["action_type"])
	}
	if !strings.Contains(r.Fields["conditions_summary"], "/api/*") {
		t.Errorf("conditions_summary: got %q", r.Fields["conditions_summary"])
	}
	if r.Fields["action_target"] == "" {
		t.Error("action_target empty")
	}
	if r.RawStruct == nil {
		t.Fatal("RawStruct nil")
	}
	if _, ok := r.RawStruct.(elbtypes.Rule); !ok {
		t.Fatalf("RawStruct type: %T", r.RawStruct)
	}
	for _, k := range []string{"priority", "conditions_summary", "action_type", "action_target"} {
		if _, ok := r.Fields[k]; !ok {
			t.Errorf("missing %q", k)
		}
	}
}

func TestFetchELBListenerRules_MultipleConditions(t *testing.T) {
	mock := &mockELBv2DescribeRulesClient{output: &elbv2.DescribeRulesOutput{Rules: []elbtypes.Rule{{RuleArn: aws.String("arn:rule/multi-cond"), Priority: aws.String("200"), Conditions: []elbtypes.RuleCondition{{Field: aws.String("path-pattern"), PathPatternConfig: &elbtypes.PathPatternConditionConfig{Values: []string{"/api/v2/*"}}}, {Field: aws.String("host-header"), HostHeaderConfig: &elbtypes.HostHeaderConditionConfig{Values: []string{"api.example.com"}}}}, Actions: []elbtypes.Action{{Type: elbtypes.ActionTypeEnumForward, TargetGroupArn: aws.String("arn:tg/multi")}}, IsDefault: aws.Bool(false)}}}}
	result, err := awsclient.FetchELBListenerRules(context.Background(), mock, map[string]string{"listener_arn": "arn:listener/multi"}, "")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	rs := result.Resources
	if len(rs) != 1 {
		t.Fatalf("expected 1, got %d", len(rs))
	}
	s := rs[0].Fields["conditions_summary"]
	if !strings.Contains(s, "/api/v2/*") || !strings.Contains(s, "api.example.com") || !strings.Contains(s, "AND") {
		t.Errorf("conditions_summary: got %q", s)
	}
}

func TestFetchELBListenerRules_DefaultRule(t *testing.T) {
	mock := &mockELBv2DescribeRulesClient{output: &elbv2.DescribeRulesOutput{Rules: []elbtypes.Rule{{RuleArn: aws.String("arn:rule/default"), Priority: aws.String("default"), Actions: []elbtypes.Action{{Type: elbtypes.ActionTypeEnumForward, TargetGroupArn: aws.String("arn:tg/default")}}, IsDefault: aws.Bool(true)}}}}
	result, err := awsclient.FetchELBListenerRules(context.Background(), mock, map[string]string{"listener_arn": "arn:listener/default"}, "")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if rs := result.Resources; len(rs) != 1 || rs[0].Fields["priority"] != "default" {
		t.Errorf("unexpected: %v", result.Resources)
	}
}

func TestFetchELBListenerRules_RedirectAction(t *testing.T) {
	mock := &mockELBv2DescribeRulesClient{output: &elbv2.DescribeRulesOutput{Rules: []elbtypes.Rule{{RuleArn: aws.String("arn:rule/redirect"), Priority: aws.String("50"), Actions: []elbtypes.Action{{Type: elbtypes.ActionTypeEnumRedirect, RedirectConfig: &elbtypes.RedirectActionConfig{Protocol: aws.String("HTTPS"), Port: aws.String("443"), StatusCode: elbtypes.RedirectActionStatusCodeEnumHttp301}}}, IsDefault: aws.Bool(false)}}}}
	result, err := awsclient.FetchELBListenerRules(context.Background(), mock, map[string]string{"listener_arn": "arn:listener/redirect"}, "")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	rs := result.Resources
	if len(rs) != 1 {
		t.Fatalf("expected 1, got %d", len(rs))
	}
	if rs[0].Fields["action_type"] != "redirect" || rs[0].Fields["action_target"] == "" {
		t.Errorf("redirect action: type=%q target=%q", rs[0].Fields["action_type"], rs[0].Fields["action_target"])
	}
}

func TestFetchELBListenerRules_FixedResponseAction(t *testing.T) {
	mock := &mockELBv2DescribeRulesClient{output: &elbv2.DescribeRulesOutput{Rules: []elbtypes.Rule{{RuleArn: aws.String("arn:rule/fixed"), Priority: aws.String("10"), Actions: []elbtypes.Action{{Type: elbtypes.ActionTypeEnumFixedResponse, FixedResponseConfig: &elbtypes.FixedResponseActionConfig{StatusCode: aws.String("503")}}}, IsDefault: aws.Bool(false)}}}}
	result, err := awsclient.FetchELBListenerRules(context.Background(), mock, map[string]string{"listener_arn": "arn:listener/fixed"}, "")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	rs := result.Resources
	if len(rs) != 1 {
		t.Fatalf("expected 1, got %d", len(rs))
	}
	if rs[0].Fields["action_type"] != "fixed-response" || !strings.Contains(rs[0].Fields["action_target"], "503") {
		t.Errorf("fixed-response: type=%q target=%q", rs[0].Fields["action_type"], rs[0].Fields["action_target"])
	}
}

func TestFetchELBListenerRules_EmptyResponse(t *testing.T) {
	result, err := awsclient.FetchELBListenerRules(context.Background(), &mockELBv2DescribeRulesClient{output: &elbv2.DescribeRulesOutput{Rules: []elbtypes.Rule{}}}, map[string]string{"listener_arn": "arn:listener/empty"}, "")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(result.Resources) != 0 {
		t.Errorf("expected 0, got %d", len(result.Resources))
	}
}

func TestFetchELBListenerRules_APIError(t *testing.T) {
	_, err := awsclient.FetchELBListenerRules(context.Background(), &mockELBv2DescribeRulesClient{err: fmt.Errorf("listener not found")}, map[string]string{"listener_arn": "arn:listener/err"}, "")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestFetchELBListenerRules_NilFields(t *testing.T) {
	result, err := awsclient.FetchELBListenerRules(context.Background(), &mockELBv2DescribeRulesClient{output: &elbv2.DescribeRulesOutput{Rules: []elbtypes.Rule{{RuleArn: aws.String("arn:rule/nil-fields")}}}}, map[string]string{"listener_arn": "arn:listener/nil"}, "")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1, got %d", len(result.Resources))
	}
}

// markerCapturingELBv2DescribeRulesMock wraps a fixed DescribeRules response
// and records the Marker it was called with, so a test can assert that
// continuationToken round-trips through Marker.
type markerCapturingELBv2DescribeRulesMock struct {
	output         *elbv2.DescribeRulesOutput
	err            error
	capturedMarker *string
}

func (m *markerCapturingELBv2DescribeRulesMock) DescribeRules(_ context.Context, params *elbv2.DescribeRulesInput, _ ...func(*elbv2.Options)) (*elbv2.DescribeRulesOutput, error) {
	m.capturedMarker = params.Marker
	if m.err != nil {
		return nil, m.err
	}
	return m.output, nil
}

// TestFetchELBListenerRules_MarkerForwarded pins that continuationToken is
// forwarded as DescribeRules' Marker. DescribeRules DOES paginate via
// Marker/NextMarker like other ELBv2 List/Describe calls — the function's
// own doc comment used to (wrongly) claim there was no pagination from AWS.
func TestFetchELBListenerRules_MarkerForwarded(t *testing.T) {
	mock := &markerCapturingELBv2DescribeRulesMock{output: &elbv2.DescribeRulesOutput{Rules: []elbtypes.Rule{}}}
	_, err := awsclient.FetchELBListenerRules(context.Background(), mock, map[string]string{"listener_arn": "arn:listener/marker"}, "page2-marker-token")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if mock.capturedMarker == nil {
		t.Fatal("expected continuationToken to be forwarded as Marker, got nil")
	}
	if *mock.capturedMarker != "page2-marker-token" {
		t.Errorf("Marker: expected %q, got %q", "page2-marker-token", *mock.capturedMarker)
	}
}

// TestFetchELBListenerRules_TruncatedByNextMarker pins that a real AWS-side
// NextMarker reports IsTruncated=true and round-trips as NextToken.
func TestFetchELBListenerRules_TruncatedByNextMarker(t *testing.T) {
	mock := &mockELBv2DescribeRulesClient{output: &elbv2.DescribeRulesOutput{
		Rules:      []elbtypes.Rule{{RuleArn: aws.String("arn:rule/page1-only"), Priority: aws.String("1")}},
		NextMarker: aws.String("next-marker-xyz"),
	}}
	result, err := awsclient.FetchELBListenerRules(context.Background(), mock, map[string]string{"listener_arn": "arn:listener/truncated"}, "")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination, got nil")
	}
	if !result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=true when AWS returns a NextMarker")
	}
	if result.Pagination.NextToken != "next-marker-xyz" {
		t.Errorf("NextToken: expected %q, got %q", "next-marker-xyz", result.Pagination.NextToken)
	}
}

// TestFetchELBListenerRules_TruncatedByMaxRulesCap pins the SECOND
// truncation cause: even when AWS reports no NextMarker (AWS itself has no
// more pages), the client-side maxRules=200 cap can still truncate the
// response converted to resources. Before the fix, IsTruncated was
// hardcoded false, so a listener with more than 200 rules silently reported
// an exact, complete rule count that was 200+ short of the truth.
func TestFetchELBListenerRules_TruncatedByMaxRulesCap(t *testing.T) {
	const maxRules = 200
	rules := make([]elbtypes.Rule, maxRules+1)
	for i := range rules {
		rules[i] = elbtypes.Rule{RuleArn: aws.String(fmt.Sprintf("arn:rule/cap-%d", i)), Priority: aws.String(fmt.Sprintf("%d", i))}
	}
	mock := &mockELBv2DescribeRulesClient{output: &elbv2.DescribeRulesOutput{Rules: rules}} // NextMarker nil: AWS itself has no more pages
	result, err := awsclient.FetchELBListenerRules(context.Background(), mock, map[string]string{"listener_arn": "arn:listener/capped"}, "")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination, got nil")
	}
	if len(result.Resources) != maxRules {
		t.Fatalf("expected %d resources (capped), got %d", maxRules, len(result.Resources))
	}
	if !result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=true from the local maxRules cap, even though AWS itself reported no NextMarker")
	}
}

func TestBuildConditionsSummary_PathPattern(t *testing.T) {
	s := awsclient.BuildConditionsSummary([]elbtypes.RuleCondition{{Field: aws.String("path-pattern"), PathPatternConfig: &elbtypes.PathPatternConditionConfig{Values: []string{"/api/*"}}}})
	if !strings.Contains(s, "/api/*") {
		t.Errorf("got %q", s)
	}
}

func TestBuildConditionsSummary_HostHeader(t *testing.T) {
	s := awsclient.BuildConditionsSummary([]elbtypes.RuleCondition{{Field: aws.String("host-header"), HostHeaderConfig: &elbtypes.HostHeaderConditionConfig{Values: []string{"example.com"}}}})
	if !strings.Contains(s, "example.com") {
		t.Errorf("got %q", s)
	}
}

func TestBuildConditionsSummary_SourceIP(t *testing.T) {
	s := awsclient.BuildConditionsSummary([]elbtypes.RuleCondition{{Field: aws.String("source-ip"), SourceIpConfig: &elbtypes.SourceIpConditionConfig{Values: []string{"10.0.0.0/8"}}}})
	if !strings.Contains(s, "10.0.0.0/8") {
		t.Errorf("got %q", s)
	}
}

func TestBuildConditionsSummary_Combined(t *testing.T) {
	s := awsclient.BuildConditionsSummary([]elbtypes.RuleCondition{{Field: aws.String("path-pattern"), PathPatternConfig: &elbtypes.PathPatternConditionConfig{Values: []string{"/api/*"}}}, {Field: aws.String("host-header"), HostHeaderConfig: &elbtypes.HostHeaderConditionConfig{Values: []string{"api.example.com"}}}})
	if !strings.Contains(s, "/api/*") || !strings.Contains(s, "api.example.com") || !strings.Contains(s, "AND") {
		t.Errorf("got %q", s)
	}
}

func TestBuildConditionsSummary_Empty(t *testing.T) {
	if s := awsclient.BuildConditionsSummary(nil); s != "" {
		t.Errorf("got %q", s)
	}
}

func TestELBListenerRuleColumns(t *testing.T) {
	cols := resource.ELBListenerRuleColumns()
	if len(cols) != 4 {
		t.Fatalf("got %d columns", len(cols))
	}
	for i, w := range []string{"priority", "conditions_summary", "action_type", "action_target"} {
		if cols[i].Key != w {
			t.Errorf("col %d: got %q", i, cols[i].Key)
		}
	}
}
