package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	eventbridgetypes "github.com/aws/aws-sdk-go-v2/service/eventbridge/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ebRuleCheckerByTarget returns the RelatedChecker for the given target type
// registered under "eb-rule". Fails immediately if not found or nil.
func ebRuleCheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("eb-rule") {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("eb-rule related checker for %s is nil", target)
			}
			return def.Checker
		}
	}
	t.Fatalf("eb-rule related checker for %s not found", target)
	return nil
}

// ebRuleRes returns a canonical source resource for the eb-rule type.
func ebRuleRes(ruleName string, roleARN string) resource.Resource {
	var raw eventbridgetypes.Rule
	raw.Name = aws.String(ruleName)
	if roleARN != "" {
		raw.RoleArn = aws.String(roleARN)
	}
	return resource.Resource{
		ID:        ruleName,
		Name:      ruleName,
		Fields:    map[string]string{},
		RawStruct: raw,
	}
}

// ---------------------------------------------------------------------------
// Registered checkers
// ---------------------------------------------------------------------------

func TestRelated_EbRule_Registered(t *testing.T) {
	defs := resource.GetRelated("eb-rule")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for eb-rule")
	}
	expected := map[string]string{
		"role":    "IAM Role",
		"kinesis": "Kinesis (targets)",
		"lambda":  "Lambda (targets)",
		"logs":    "Log Groups (targets)",
		"sfn":     "Step Functions (targets)",
		"sns":     "SNS (targets)",
		"sqs":     "SQS (targets)",
	}
	for target, wantDisplay := range expected {
		found := false
		for _, def := range defs {
			if def.TargetType == target {
				found = true
				if def.Checker == nil {
					t.Errorf("eb-rule %q: Checker should not be nil", target)
				}
				if def.DisplayName != wantDisplay {
					t.Errorf("eb-rule %q: DisplayName = %q, want %q", target, def.DisplayName, wantDisplay)
				}
				break
			}
		}
		if !found {
			t.Errorf("expected related def for target %q not found", target)
		}
	}
}

// ---------------------------------------------------------------------------
// NavigableFields
// ---------------------------------------------------------------------------

func TestNavigableFields_EbRule_RoleArn(t *testing.T) {
	fields := resource.GetNavigableFields("eb-rule")
	found := false
	for _, f := range fields {
		if f.FieldPath == "RoleArn" && f.TargetType == "role" {
			found = true
			break
		}
	}
	if !found {
		t.Error("eb-rule NavigableField RoleArn→role not registered")
	}
}

// ---------------------------------------------------------------------------
// checkEbRuleRole — Pattern F (RawStruct eventbridgetypes.Rule.RoleArn)
// ---------------------------------------------------------------------------

func TestRelated_EbRule_Role_Match(t *testing.T) {
	res := ebRuleRes("my-rule", "arn:aws:iam::123456789012:role/EventBridgeDeployRole")
	checker := ebRuleCheckerByTarget(t, "role")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) == 0 || result.ResourceIDs[0] != "EventBridgeDeployRole" {
		t.Errorf("ResourceIDs = %v, want [EventBridgeDeployRole]", result.ResourceIDs)
	}
}

func TestRelated_EbRule_Role_NoRole(t *testing.T) {
	res := ebRuleRes("my-rule", "") // RoleArn not set
	checker := ebRuleCheckerByTarget(t, "role")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no RoleArn)", result.Count)
	}
}

func TestRelated_EbRule_Role_WrongRawStruct(t *testing.T) {
	res := resource.Resource{
		ID:        "my-rule",
		Fields:    map[string]string{},
		RawStruct: "not-a-rule",
	}
	checker := ebRuleCheckerByTarget(t, "role")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (wrong RawStruct type)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkEbRuleKinesis — ebRuleTargetsByService, service="kinesis"
// ---------------------------------------------------------------------------

func TestRelated_EbRule_Kinesis_Match(t *testing.T) {
	res := resource.Resource{ID: "my-rule", Fields: map[string]string{}}
	clients := &awsclient.ServiceClients{
		EventBridge: &fakeEventBridgeCR{
			targets: []eventbridgetypes.Target{
				{Arn: aws.String("arn:aws:kinesis:us-east-1:123456789012:stream/my-stream")},
			},
		},
	}
	checker := ebRuleCheckerByTarget(t, "kinesis")
	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) == 0 || result.ResourceIDs[0] != "my-stream" {
		t.Errorf("ResourceIDs = %v, want [my-stream]", result.ResourceIDs)
	}
}

func TestRelated_EbRule_Kinesis_NoMatch(t *testing.T) {
	res := resource.Resource{ID: "my-rule", Fields: map[string]string{}}
	clients := &awsclient.ServiceClients{
		EventBridge: &fakeEventBridgeCR{
			targets: []eventbridgetypes.Target{
				{Arn: aws.String("arn:aws:lambda:us-east-1:123456789012:function:my-func")},
			},
		},
	}
	checker := ebRuleCheckerByTarget(t, "kinesis")
	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no kinesis target)", result.Count)
	}
}

func TestRelated_EbRule_Kinesis_NilClients(t *testing.T) {
	res := resource.Resource{ID: "my-rule", Fields: map[string]string{}}
	checker := ebRuleCheckerByTarget(t, "kinesis")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (nil clients)", result.Count)
	}
}

func TestRelated_EbRule_Kinesis_EmptyID(t *testing.T) {
	res := resource.Resource{ID: "", Fields: map[string]string{}}
	checker := ebRuleCheckerByTarget(t, "kinesis")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty rule ID)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkEbRuleLambda — service="lambda", strips :version suffix
// ---------------------------------------------------------------------------

func TestRelated_EbRule_Lambda_Match(t *testing.T) {
	res := resource.Resource{ID: "my-rule", Fields: map[string]string{}}
	clients := &awsclient.ServiceClients{
		EventBridge: &fakeEventBridgeCR{
			targets: []eventbridgetypes.Target{
				{Arn: aws.String("arn:aws:lambda:us-east-1:123456789012:function:process-events:3")},
			},
		},
	}
	checker := ebRuleCheckerByTarget(t, "lambda")
	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	// :version should be stripped
	if len(result.ResourceIDs) == 0 || result.ResourceIDs[0] != "process-events" {
		t.Errorf("ResourceIDs = %v, want [process-events]", result.ResourceIDs)
	}
}

func TestRelated_EbRule_Lambda_NoVersion(t *testing.T) {
	res := resource.Resource{ID: "my-rule", Fields: map[string]string{}}
	clients := &awsclient.ServiceClients{
		EventBridge: &fakeEventBridgeCR{
			targets: []eventbridgetypes.Target{
				{Arn: aws.String("arn:aws:lambda:us-east-1:123456789012:function:my-func")},
			},
		},
	}
	checker := ebRuleCheckerByTarget(t, "lambda")
	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) == 0 || result.ResourceIDs[0] != "my-func" {
		t.Errorf("ResourceIDs = %v, want [my-func]", result.ResourceIDs)
	}
}

// ---------------------------------------------------------------------------
// checkEbRuleLogs — service="logs", trims :* suffix
// ---------------------------------------------------------------------------

func TestRelated_EbRule_Logs_Match(t *testing.T) {
	res := resource.Resource{ID: "my-rule", Fields: map[string]string{}}
	clients := &awsclient.ServiceClients{
		EventBridge: &fakeEventBridgeCR{
			targets: []eventbridgetypes.Target{
				{Arn: aws.String("arn:aws:logs:us-east-1:123456789012:log-group:/aws/my-app:*")},
			},
		},
	}
	checker := ebRuleCheckerByTarget(t, "logs")
	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	// :* suffix should be stripped
	if len(result.ResourceIDs) == 0 || result.ResourceIDs[0] != "/aws/my-app" {
		t.Errorf("ResourceIDs = %v, want [/aws/my-app]", result.ResourceIDs)
	}
}

// ---------------------------------------------------------------------------
// checkEbRuleSFN — service="states", extracts :stateMachine: suffix
// ---------------------------------------------------------------------------

func TestRelated_EbRule_SFN_Match(t *testing.T) {
	res := resource.Resource{ID: "my-rule", Fields: map[string]string{}}
	clients := &awsclient.ServiceClients{
		EventBridge: &fakeEventBridgeCR{
			targets: []eventbridgetypes.Target{
				{Arn: aws.String("arn:aws:states:us-east-1:123456789012:stateMachine:my-state-machine")},
			},
		},
	}
	checker := ebRuleCheckerByTarget(t, "sfn")
	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) == 0 || result.ResourceIDs[0] != "my-state-machine" {
		t.Errorf("ResourceIDs = %v, want [my-state-machine]", result.ResourceIDs)
	}
}

// ---------------------------------------------------------------------------
// checkEbRuleSNS — service="sns", last ":" segment
// ---------------------------------------------------------------------------

func TestRelated_EbRule_SNS_Match(t *testing.T) {
	res := resource.Resource{ID: "my-rule", Fields: map[string]string{}}
	clients := &awsclient.ServiceClients{
		EventBridge: &fakeEventBridgeCR{
			targets: []eventbridgetypes.Target{
				{Arn: aws.String("arn:aws:sns:us-east-1:123456789012:my-alerts-topic")},
			},
		},
	}
	checker := ebRuleCheckerByTarget(t, "sns")
	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) == 0 || result.ResourceIDs[0] != "my-alerts-topic" {
		t.Errorf("ResourceIDs = %v, want [my-alerts-topic]", result.ResourceIDs)
	}
}

// ---------------------------------------------------------------------------
// checkEbRuleSQS — service="sqs", last ":" segment
// ---------------------------------------------------------------------------

func TestRelated_EbRule_SQS_Match(t *testing.T) {
	res := resource.Resource{ID: "my-rule", Fields: map[string]string{}}
	clients := &awsclient.ServiceClients{
		EventBridge: &fakeEventBridgeCR{
			targets: []eventbridgetypes.Target{
				{Arn: aws.String("arn:aws:sqs:us-east-1:123456789012:my-queue")},
			},
		},
	}
	checker := ebRuleCheckerByTarget(t, "sqs")
	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) == 0 || result.ResourceIDs[0] != "my-queue" {
		t.Errorf("ResourceIDs = %v, want [my-queue]", result.ResourceIDs)
	}
}

// TestRelated_EbRule_TargetService_NilClients verifies that all
// ebRuleTargetsByService-based checkers return Count=-1 when clients are nil.
func TestRelated_EbRule_TargetService_NilClients(t *testing.T) {
	res := resource.Resource{ID: "my-rule", Fields: map[string]string{}}
	for _, target := range []string{"kinesis", "lambda", "logs", "sfn", "sns", "sqs"} {
		checker := ebRuleCheckerByTarget(t, target)
		result := checker(context.Background(), nil, res, resource.ResourceCache{})
		if result.Count != -1 {
			t.Errorf("target=%s: Count = %d, want -1 (nil clients)", target, result.Count)
		}
	}
}
