package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

func lambdaCheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("lambda") {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("lambda related checker for %s is nil", target)
			}
			return def.Checker
		}
	}
	t.Fatalf("lambda related checker for %s not found", target)
	return nil
}

// --- Navigable Field Registration ---

func TestNavigableFields_Lambda_Registered(t *testing.T) {
	nav := resource.IsFieldNavigable("lambda", "Role")
	if nav == nil {
		t.Fatal("expected navigable field Role not found for lambda")
	}
	if nav.TargetType != "role" {
		t.Errorf("Role TargetType = %q, want %q", nav.TargetType, "role")
	}
}

// --- IAM Role checker (Pattern C — cache, name extracted from ARN) ---

func TestRelated_Lambda_Role_Found(t *testing.T) {
	const roleARN = "arn:aws:iam::123456789012:role/my-lambda-role"
	const roleName = "my-lambda-role"

	roleRes := resource.Resource{
		ID:   roleName,
		Name: roleName,
	}
	cache := resource.ResourceCache{
		"role": resource.ResourceCacheEntry{Resources: []resource.Resource{roleRes}},
	}
	source := resource.Resource{
		ID:   "my-function",
		Name: "my-function",
		Fields: map[string]string{
			"role": roleARN,
		},
		RawStruct: lambdatypes.FunctionConfiguration{
			FunctionName: aws.String("my-function"),
			Role:         aws.String(roleARN),
		},
	}

	checker := lambdaCheckerByTarget(t, "role")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != roleName {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, roleName)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_Lambda_Role_NotFound(t *testing.T) {
	const roleARN = "arn:aws:iam::123456789012:role/my-lambda-role"

	roleRes := resource.Resource{
		ID:   "DifferentRole",
		Name: "DifferentRole",
	}
	cache := resource.ResourceCache{
		"role": resource.ResourceCacheEntry{Resources: []resource.Resource{roleRes}},
	}
	source := resource.Resource{
		ID:   "my-function",
		Name: "my-function",
		Fields: map[string]string{
			"role": roleARN,
		},
		RawStruct: lambdatypes.FunctionConfiguration{
			FunctionName: aws.String("my-function"),
			Role:         aws.String(roleARN),
		},
	}

	checker := lambdaCheckerByTarget(t, "role")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_Lambda_Role_CacheMissNoClients(t *testing.T) {
	source := resource.Resource{
		ID:   "my-function",
		Name: "my-function",
		Fields: map[string]string{
			"role": "arn:aws:iam::123456789012:role/my-lambda-role",
		},
		RawStruct: lambdatypes.FunctionConfiguration{
			FunctionName: aws.String("my-function"),
			Role:         aws.String("arn:aws:iam::123456789012:role/my-lambda-role"),
		},
	}

	checker := lambdaCheckerByTarget(t, "role")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (unknown)", result.Count)
	}
}

// --- CloudWatch Alarms checker (Pattern C — cache, FunctionName dimension) ---

func TestRelated_Lambda_Alarms_Found(t *testing.T) {
	const fnName = "my-function"

	alarmRes := resource.Resource{
		ID: "lambda-error-alarm",
		RawStruct: cwtypes.MetricAlarm{
			AlarmName: aws.String("lambda-error-alarm"),
			Dimensions: []cwtypes.Dimension{
				{Name: aws.String("FunctionName"), Value: aws.String(fnName)},
			},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{alarmRes}},
	}
	source := resource.Resource{
		ID:   fnName,
		Name: fnName,
		Fields: map[string]string{
			"function_name": fnName,
		},
		RawStruct: lambdatypes.FunctionConfiguration{
			FunctionName: aws.String(fnName),
			Role:         aws.String("arn:aws:iam::123456789012:role/my-lambda-role"),
		},
	}

	checker := lambdaCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "lambda-error-alarm" {
		t.Errorf("ResourceIDs = %v, want [lambda-error-alarm]", result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_Lambda_Alarms_NotFound(t *testing.T) {
	const fnName = "my-function"

	alarmRes := resource.Resource{
		ID: "other-function-alarm",
		RawStruct: cwtypes.MetricAlarm{
			AlarmName: aws.String("other-function-alarm"),
			Dimensions: []cwtypes.Dimension{
				{Name: aws.String("FunctionName"), Value: aws.String("different-function")},
			},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{alarmRes}},
	}
	source := resource.Resource{
		ID:   fnName,
		Name: fnName,
		Fields: map[string]string{
			"function_name": fnName,
		},
		RawStruct: lambdatypes.FunctionConfiguration{
			FunctionName: aws.String(fnName),
			Role:         aws.String("arn:aws:iam::123456789012:role/my-lambda-role"),
		},
	}

	checker := lambdaCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_Lambda_Alarms_CacheMissNoClients(t *testing.T) {
	source := resource.Resource{
		ID:   "my-function",
		Name: "my-function",
		Fields: map[string]string{
			"function_name": "my-function",
		},
		RawStruct: lambdatypes.FunctionConfiguration{
			FunctionName: aws.String("my-function"),
			Role:         aws.String("arn:aws:iam::123456789012:role/my-lambda-role"),
		},
	}

	checker := lambdaCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (unknown)", result.Count)
	}
}

// --- Stub checkers (nil Checker) ---

func TestRelated_Lambda_SQS_IsStub(t *testing.T) {
	defs := resource.GetRelated("lambda")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for lambda")
	}
	for _, def := range defs {
		if def.TargetType == "sqs" {
			if def.Checker == nil {
				t.Errorf("lambda sqs Checker should not be nil")
			}
			return
		}
	}
	t.Error("expected related def for target sqs not found for lambda")
}

func TestRelated_Lambda_CFN_IsStub(t *testing.T) {
	defs := resource.GetRelated("lambda")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for lambda")
	}
	for _, def := range defs {
		if def.TargetType == "cfn" {
			if def.Checker == nil {
				t.Errorf("lambda cfn Checker should not be nil")
			}
			return
		}
	}
	t.Error("expected related def for target cfn not found for lambda")
}
