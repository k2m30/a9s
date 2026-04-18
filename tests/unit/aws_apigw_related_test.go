package unit_test

import (
	"context"
	"testing"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// TestRelated_APIGW_Registered verifies all 3 related defs are registered with correct checker presence.
func TestRelated_APIGW_Registered(t *testing.T) {
	defs := resource.GetRelated("apigw")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for apigw")
	}

	type expectation struct {
		displayName string
		hasChecker  bool
	}
	expected := map[string]expectation{
		"lambda": {"Lambda Functions", true},
		"logs":   {"Log Groups", true},
		"waf":    {"WAF Web ACLs", true},
	}
	for target, want := range expected {
		found := false
		for _, def := range defs {
			if def.TargetType == target {
				found = true
				if want.hasChecker && def.Checker == nil {
					t.Errorf("apigw %q: Checker should not be nil", target)
				}
				if !want.hasChecker && def.Checker != nil {
					t.Errorf("apigw %q: Checker should be nil (stub)", target)
				}
				if def.DisplayName != want.displayName {
					t.Errorf("apigw %q: DisplayName = %q, want %q", target, def.DisplayName, want.displayName)
				}
				break
			}
		}
		if !found {
			t.Errorf("expected related def for target %q not found", target)
		}
	}
}

// apigwCheckerByTarget returns the RelatedChecker for the given target type registered
// under "apigw". It fails the test immediately if the checker is nil or not found.
func apigwCheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("apigw") {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("apigw related checker for %s is nil", target)
			}
			return def.Checker
		}
	}
	t.Fatalf("apigw related checker for %s not found", target)
	return nil
}

// --- checkApigwLogs tests (Pattern N — naming convention) ---

func TestRelated_APIGW_Logs_MatchByExecutionLogPattern(t *testing.T) {
	logRes := resource.Resource{
		ID:     "API-Gateway-Execution-Logs_abc123/prod",
		Fields: map[string]string{},
	}
	cache := resource.ResourceCache{
		"logs": resource.ResourceCacheEntry{Resources: []resource.Resource{logRes}},
	}

	res := resource.Resource{
		ID:     "abc123",
		Fields: map[string]string{},
	}

	checker := apigwCheckerByTarget(t, "logs")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "API-Gateway-Execution-Logs_abc123/prod" {
		t.Errorf("ResourceIDs = %v, want [API-Gateway-Execution-Logs_abc123/prod]", result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_APIGW_Logs_MatchByAccessLogPattern(t *testing.T) {
	logRes := resource.Resource{
		ID:     "/aws/apigateway/my-api",
		Fields: map[string]string{},
	}
	cache := resource.ResourceCache{
		"logs": resource.ResourceCacheEntry{Resources: []resource.Resource{logRes}},
	}

	res := resource.Resource{
		ID:     "some-id",
		Name:   "my-api",
		Fields: map[string]string{},
	}

	checker := apigwCheckerByTarget(t, "logs")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "/aws/apigateway/my-api" {
		t.Errorf("ResourceIDs = %v, want [/aws/apigateway/my-api]", result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_APIGW_Logs_NoMatch(t *testing.T) {
	logRes := resource.Resource{
		ID:     "/aws/apigateway/other-api",
		Fields: map[string]string{},
	}
	cache := resource.ResourceCache{
		"logs": resource.ResourceCacheEntry{Resources: []resource.Resource{logRes}},
	}

	res := resource.Resource{
		ID:     "xyz999",
		Name:   "my-api",
		Fields: map[string]string{},
	}

	checker := apigwCheckerByTarget(t, "logs")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_APIGW_Logs_NilCache(t *testing.T) {
	cache := resource.ResourceCache{}

	res := resource.Resource{
		ID:     "abc123",
		Fields: map[string]string{},
	}

	checker := apigwCheckerByTarget(t, "logs")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (empty cache, no clients)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkApigwLambda tests (requires GetIntegrations per API — outside budget)
// ---------------------------------------------------------------------------

// TestRelated_APIGW_Lambda_Unknown: valid API → Count: -1 (integrations via GetIntegrations).
func TestRelated_APIGW_Lambda_Unknown(t *testing.T) {
	res := resource.Resource{
		ID:     "api-abc123",
		Name:   "my-api",
		Fields: map[string]string{},
	}
	checker := apigwCheckerByTarget(t, "lambda")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (unknown: integration targets via GetIntegrations)", result.Count)
	}
	if result.TargetType != "lambda" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "lambda")
	}
}

// TestRelated_APIGW_Lambda_EmptyInput: empty API id → Count: 0.
func TestRelated_APIGW_Lambda_EmptyInput(t *testing.T) {
	res := resource.Resource{ID: "", Fields: map[string]string{}}
	checker := apigwCheckerByTarget(t, "lambda")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty API id)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkApigwWAF tests (requires ListResourcesForWebACL per Web ACL — outside budget)
// ---------------------------------------------------------------------------

// TestRelated_APIGW_WAF_Unknown: valid API → Count: -1 (Web ACL links resolved from WAF side).
func TestRelated_APIGW_WAF_Unknown(t *testing.T) {
	res := resource.Resource{
		ID:     "api-abc123",
		Name:   "my-api",
		Fields: map[string]string{},
	}
	checker := apigwCheckerByTarget(t, "waf")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (unknown: WAF associations require ListResourcesForWebACL)", result.Count)
	}
	if result.TargetType != "waf" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "waf")
	}
}

// TestRelated_APIGW_WAF_EmptyInput: empty API id → Count: 0.
func TestRelated_APIGW_WAF_EmptyInput(t *testing.T) {
	res := resource.Resource{ID: "", Fields: map[string]string{}}
	checker := apigwCheckerByTarget(t, "waf")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty API id)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkApigwKMS tests (Pattern C: GetIntegrations + GetFunction per Lambda).
// ---------------------------------------------------------------------------

// TestRelated_Apigw_KMS_Match verifies that an API with a Lambda integration
// whose FunctionConfiguration carries a KMSKeyArn yields Count=1.
func TestRelated_Apigw_KMS_Match(t *testing.T) {
	const keyARN = "arn:aws:kms:us-east-1:123456789012:key/a1b2c3d4-1234-5678-abcd-111111111111"
	const fnName = "my-function"

	res := resource.Resource{
		ID:     "abc123",
		Name:   "my-api",
		Fields: map[string]string{},
	}
	clients := &awsclient.ServiceClients{
		APIGatewayV2: newFakeAPIGWV2WithLambdaIntegration(fnName),
		Lambda:       newFakeLambdaWithKMSKey(keyARN),
	}
	checker := apigwCheckerByTarget(t, "kms")
	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 {
		t.Fatalf("ResourceIDs = %v, want 1 entry", result.ResourceIDs)
	}
	if result.ResourceIDs[0] != "a1b2c3d4-1234-5678-abcd-111111111111" {
		t.Errorf("ResourceIDs[0] = %q, want key UUID", result.ResourceIDs[0])
	}
}

// TestRelated_Apigw_KMS_EmptyInput verifies that an empty API ID returns Count=0
// without calling any API.
func TestRelated_Apigw_KMS_EmptyInput(t *testing.T) {
	res := resource.Resource{ID: "", Fields: map[string]string{}}
	checker := apigwCheckerByTarget(t, "kms")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty API ID)", result.Count)
	}
}

// TestRelated_Apigw_KMS_WrongRawStructType verifies that nil clients returns
// Count=-1 (GetIntegrations cannot proceed).
func TestRelated_Apigw_KMS_WrongRawStructType(t *testing.T) {
	res := resource.Resource{
		ID:     "abc123",
		Fields: map[string]string{},
	}
	checker := apigwCheckerByTarget(t, "kms")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (nil clients)", result.Count)
	}
}
