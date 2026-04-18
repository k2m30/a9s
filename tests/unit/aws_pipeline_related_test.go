package unit_test

import (
	"context"
	"testing"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// --- Navigable Fields ---

func TestNavigableFields_Pipeline_None(t *testing.T) {
	fields := resource.GetNavigableFields("pipeline")
	if len(fields) != 0 {
		t.Errorf("expected no navigable fields for pipeline, got %d: %v", len(fields), fields)
	}
}

// pipelineCheckerByTarget returns the RelatedChecker for the given target
// type registered under "pipeline". Fails immediately if not found or nil.
func pipelineCheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("pipeline") {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("pipeline related checker for %s is nil", target)
			}
			return def.Checker
		}
	}
	t.Fatalf("pipeline related checker for %s not found", target)
	return nil
}

// ---------------------------------------------------------------------------
// checkPipelineEbRule — Pattern C: ListRuleNamesByTarget on pipeline ARN
// ---------------------------------------------------------------------------

// TestRelated_Pipeline_EbRule_Match verifies that when the fake EventBridge
// returns 3 rule names, Count=3 and ResourceIDs has all 3 names.
func TestRelated_Pipeline_EbRule_Match(t *testing.T) {
	src := resource.Resource{
		ID:   "my-pipeline",
		Name: "my-pipeline",
		Fields: map[string]string{
			"arn": "arn:aws:codepipeline:us-east-1:123456789012:my-pipeline",
		},
	}
	clients := &awsclient.ServiceClients{
		EventBridge: &fakeEventBridgeUS1{
			ruleNames: []string{"rule-deploy", "rule-notify", "rule-rollback"},
		},
	}
	checker := pipelineCheckerByTarget(t, "eb-rule")
	result := checker(context.Background(), clients, src, resource.ResourceCache{})

	if result.Count != 3 {
		t.Errorf("Count = %d, want 3", result.Count)
	}
	if len(result.ResourceIDs) != 3 {
		t.Errorf("ResourceIDs = %v, want 3 entries", result.ResourceIDs)
	}
}

// TestRelated_Pipeline_EbRule_Empty verifies that a pipeline with no ARN
// field returns Count=0.
func TestRelated_Pipeline_EbRule_Empty(t *testing.T) {
	src := resource.Resource{
		ID:     "my-pipeline",
		Name:   "my-pipeline",
		Fields: map[string]string{},
	}
	checker := pipelineCheckerByTarget(t, "eb-rule")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty ARN field)", result.Count)
	}
}

// TestRelated_Pipeline_EbRule_WrongRawStruct verifies that nil clients with
// a valid ARN field returns Count=-1 (no EventBridge client available).
func TestRelated_Pipeline_EbRule_WrongRawStruct(t *testing.T) {
	src := resource.Resource{
		ID:   "my-pipeline",
		Name: "my-pipeline",
		Fields: map[string]string{
			"arn": "arn:aws:codepipeline:us-east-1:123456789012:my-pipeline",
		},
		RawStruct: "not-a-pipeline",
	}
	checker := pipelineCheckerByTarget(t, "eb-rule")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (nil clients)", result.Count)
	}
}
