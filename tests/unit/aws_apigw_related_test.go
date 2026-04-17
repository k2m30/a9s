package unit_test

import (
	"context"
	"testing"

	apigwtypes "github.com/aws/aws-sdk-go-v2/service/apigatewayv2/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
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
// checkApigwKMS tests — Count: 0 is definitive (apigatewayv2 exposes no
// customer-managed CMK anywhere in the service model).
// ---------------------------------------------------------------------------

// TestRelated_Apigw_KMS_DefinitiveZero: real apigatewayv2types.Api RawStruct
// → Count: 0 (definitive, not unknown).
func TestRelated_Apigw_KMS_DefinitiveZero(t *testing.T) {
	apiID := "abc123"
	name := "my-api"
	res := resource.Resource{
		ID:     apiID,
		Name:   name,
		Fields: map[string]string{"api_id": apiID, "name": name},
		RawStruct: apigwtypes.Api{
			ApiId:        &apiID,
			Name:         &name,
			ProtocolType: apigwtypes.ProtocolTypeHttp,
		},
	}
	checker := apigwCheckerByTarget(t, "kms")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (apigatewayv2 has no KMS relationship)", result.Count)
	}
	if result.TargetType != "kms" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "kms")
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
	if len(result.ResourceIDs) != 0 {
		t.Errorf("ResourceIDs = %v, want empty", result.ResourceIDs)
	}
}

// TestRelated_Apigw_KMS_EmptyInput: empty API id → Count: 0.
func TestRelated_Apigw_KMS_EmptyInput(t *testing.T) {
	res := resource.Resource{ID: "", Fields: map[string]string{}}
	checker := apigwCheckerByTarget(t, "kms")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty API id)", result.Count)
	}
	if result.TargetType != "kms" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "kms")
	}
}

// TestRelated_Apigw_KMS_WrongRawStructType: RawStruct is not apigatewayv2types.Api
// → Count: 0 (still definitive; mis-typed input cannot yield KMS info).
func TestRelated_Apigw_KMS_WrongRawStructType(t *testing.T) {
	res := resource.Resource{
		ID:        "abc123",
		Fields:    map[string]string{"api_id": "abc123"},
		RawStruct: "not-an-api-struct",
	}
	checker := apigwCheckerByTarget(t, "kms")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (wrong RawStruct type)", result.Count)
	}
	if result.TargetType != "kms" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "kms")
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}
