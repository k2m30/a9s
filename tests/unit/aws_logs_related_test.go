package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/resource"
)

func logsCheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("logs") {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("logs related checker for %s is nil", target)
			}
			return def.Checker
		}
	}
	t.Fatalf("logs related checker for %s not found", target)
	return nil
}

// --- Lambda checker (Pattern C — cache, name parsed from /aws/lambda/{name}) ---

func TestRelated_Logs_Lambda_Found(t *testing.T) {
	const logGroupName = "/aws/lambda/my-function"
	const functionName = "my-function"

	lambdaRes := resource.Resource{
		ID:   functionName,
		Name: functionName,
	}
	cache := resource.ResourceCache{
		"lambda": resource.ResourceCacheEntry{Resources: []resource.Resource{lambdaRes}},
	}
	source := resource.Resource{
		ID:   logGroupName,
		Name: logGroupName,
		Fields: map[string]string{
			"log_group_name": logGroupName,
		},
	}

	checker := logsCheckerByTarget(t, "lambda")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != functionName {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, functionName)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_Logs_Lambda_NotLambdaGroup(t *testing.T) {
	const logGroupName = "/aws/rds/instance/mydb/error"

	lambdaRes := resource.Resource{
		ID:   "mydb",
		Name: "mydb",
	}
	cache := resource.ResourceCache{
		"lambda": resource.ResourceCacheEntry{Resources: []resource.Resource{lambdaRes}},
	}
	source := resource.Resource{
		ID:   logGroupName,
		Name: logGroupName,
		Fields: map[string]string{
			"log_group_name": logGroupName,
		},
	}

	checker := logsCheckerByTarget(t, "lambda")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (not a lambda log group)", result.Count)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_Logs_Lambda_CacheMissNoClients(t *testing.T) {
	source := resource.Resource{
		ID:   "/aws/lambda/my-function",
		Name: "/aws/lambda/my-function",
		Fields: map[string]string{
			"log_group_name": "/aws/lambda/my-function",
		},
	}

	checker := logsCheckerByTarget(t, "lambda")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (unknown/cache miss)", result.Count)
	}
}

// --- Alarms checker (Pattern C — cache, LogGroupName dimension) ---

func TestRelated_Logs_Alarms_Found(t *testing.T) {
	const logGroupName = "/aws/lambda/my-function"

	alarmRes := resource.Resource{
		ID: "log-group-error-alarm",
		RawStruct: cwtypes.MetricAlarm{
			AlarmName: aws.String("log-group-error-alarm"),
			Dimensions: []cwtypes.Dimension{
				{Name: aws.String("LogGroupName"), Value: aws.String(logGroupName)},
			},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{alarmRes}},
	}
	source := resource.Resource{
		ID:   logGroupName,
		Name: logGroupName,
		Fields: map[string]string{
			"log_group_name": logGroupName,
		},
	}

	checker := logsCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "log-group-error-alarm" {
		t.Errorf("ResourceIDs = %v, want [log-group-error-alarm]", result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_Logs_Alarms_NotFound(t *testing.T) {
	const logGroupName = "/aws/lambda/my-function"

	alarmRes := resource.Resource{
		ID: "other-log-group-alarm",
		RawStruct: cwtypes.MetricAlarm{
			AlarmName: aws.String("other-log-group-alarm"),
			Dimensions: []cwtypes.Dimension{
				{Name: aws.String("LogGroupName"), Value: aws.String("/aws/lambda/different-function")},
			},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{alarmRes}},
	}
	source := resource.Resource{
		ID:   logGroupName,
		Name: logGroupName,
		Fields: map[string]string{
			"log_group_name": logGroupName,
		},
	}

	checker := logsCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_Logs_Alarms_CacheMissNoClients(t *testing.T) {
	source := resource.Resource{
		ID:   "/aws/lambda/my-function",
		Name: "/aws/lambda/my-function",
		Fields: map[string]string{
			"log_group_name": "/aws/lambda/my-function",
		},
	}

	checker := logsCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (unknown/cache miss)", result.Count)
	}
}

// --- Demo Checker ---

func TestRelatedDemo_Logs_Registered(t *testing.T) {
	_ = demo.GetResources // ensure demo package is initialized
	checker := resource.GetRelatedDemo("logs")
	if checker == nil {
		t.Fatal("no demo checker registered for logs")
	}

	// Test with a lambda-prefixed log group (should produce lambda Count=1).
	lambdaLogGroup := resource.Resource{ID: "/aws/lambda/api-gateway-authorizer"}
	results := checker(lambdaLogGroup)
	if len(results) == 0 {
		t.Fatal("demo checker returned no results for lambda log group")
	}
	for _, r := range results {
		if r.TargetType == "" {
			t.Error("demo result has empty TargetType")
		}
	}

	// Verify all expected target types are present.
	wantTargets := map[string]bool{"lambda": false, "alarm": false}
	for _, r := range results {
		if _, ok := wantTargets[r.TargetType]; ok {
			wantTargets[r.TargetType] = true
		}
	}
	for target, found := range wantTargets {
		if !found {
			t.Errorf("demo checker missing result for target %q", target)
		}
	}

	// Lambda log group should have lambda Count > 0.
	for _, r := range results {
		if r.TargetType == "lambda" && r.Count < 1 {
			t.Errorf("demo checker lambda Count = %d for lambda log group, want >= 1", r.Count)
		}
	}

	// Test with a non-lambda log group (lambda Count should be 0).
	nonLambdaLogGroup := resource.Resource{ID: "/aws/rds/instance/prod-api-primary/postgresql"}
	nonLambdaResults := checker(nonLambdaLogGroup)
	for _, r := range nonLambdaResults {
		if r.TargetType == "lambda" && r.Count != 0 {
			t.Errorf("demo checker lambda Count = %d for non-lambda log group, want 0", r.Count)
		}
	}
}
