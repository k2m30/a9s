package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	gluetypes "github.com/aws/aws-sdk-go-v2/service/glue/types"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/resource"
)

func roleCheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("role") {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("role related checker for %s is nil", target)
			}
			return def.Checker
		}
	}
	t.Fatalf("role related checker for %s not found", target)
	return nil
}

// --- Lambda checker (reverse-lookup: search lambda cache, ARN last segment) ---

func TestRelated_Role_Lambda_Found(t *testing.T) {
	source := resource.Resource{
		ID:   "acme-lambda-execution",
		Name: "acme-lambda-execution",
		Fields: map[string]string{
			"role_name": "acme-lambda-execution",
		},
		RawStruct: iamtypes.Role{
			RoleName: aws.String("acme-lambda-execution"),
			Arn:      aws.String("arn:aws:iam::123456789012:role/service-role/acme-lambda-execution"),
		},
	}

	lambdaRes := resource.Resource{
		ID:   "my-function",
		Name: "my-function",
		RawStruct: lambdatypes.FunctionConfiguration{
			FunctionName: aws.String("my-function"),
			Role:         aws.String("arn:aws:iam::123456789012:role/service-role/acme-lambda-execution"),
		},
	}
	cache := resource.ResourceCache{
		"lambda": resource.ResourceCacheEntry{Resources: []resource.Resource{lambdaRes}},
	}

	checker := roleCheckerByTarget(t, "lambda")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_Role_Lambda_NotFound(t *testing.T) {
	source := resource.Resource{
		ID:   "acme-other-role",
		Name: "acme-other-role",
		Fields: map[string]string{
			"role_name": "acme-other-role",
		},
		RawStruct: iamtypes.Role{
			RoleName: aws.String("acme-other-role"),
			Arn:      aws.String("arn:aws:iam::123456789012:role/acme-other-role"),
		},
	}

	lambdaRes := resource.Resource{
		ID:   "my-function",
		Name: "my-function",
		RawStruct: lambdatypes.FunctionConfiguration{
			FunctionName: aws.String("my-function"),
			Role:         aws.String("arn:aws:iam::123456789012:role/service-role/acme-lambda-execution"),
		},
	}
	cache := resource.ResourceCache{
		"lambda": resource.ResourceCacheEntry{Resources: []resource.Resource{lambdaRes}},
	}

	checker := roleCheckerByTarget(t, "lambda")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_Role_Lambda_CacheMissNoClients(t *testing.T) {
	source := resource.Resource{
		ID:   "acme-lambda-execution",
		Name: "acme-lambda-execution",
		Fields: map[string]string{
			"role_name": "acme-lambda-execution",
		},
	}

	checker := roleCheckerByTarget(t, "lambda")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (unknown)", result.Count)
	}
}

func TestRelated_Role_Lambda_EmptyID(t *testing.T) {
	source := resource.Resource{
		ID:   "",
		Name: "",
		Fields: map[string]string{
			"role_name": "",
		},
	}

	lambdaRes := resource.Resource{
		ID:   "my-function",
		Name: "my-function",
		RawStruct: lambdatypes.FunctionConfiguration{
			FunctionName: aws.String("my-function"),
			Role:         aws.String("arn:aws:iam::123456789012:role/service-role/acme-lambda-execution"),
		},
	}
	cache := resource.ResourceCache{
		"lambda": resource.ResourceCacheEntry{Resources: []resource.Resource{lambdaRes}},
	}

	checker := roleCheckerByTarget(t, "lambda")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 for empty ID", result.Count)
	}
}

// --- Glue checker (reverse-lookup: search glue cache, direct name match) ---

func TestRelated_Role_Glue_Found(t *testing.T) {
	source := resource.Resource{
		ID:   "acme-glue-role",
		Name: "acme-glue-role",
		Fields: map[string]string{
			"role_name": "acme-glue-role",
		},
		RawStruct: iamtypes.Role{
			RoleName: aws.String("acme-glue-role"),
			Arn:      aws.String("arn:aws:iam::123456789012:role/acme-glue-role"),
		},
	}

	glueRes := resource.Resource{
		ID:   "my-etl-job",
		Name: "my-etl-job",
		RawStruct: gluetypes.Job{
			Name: aws.String("my-etl-job"),
			Role: aws.String("acme-glue-role"),
		},
	}
	cache := resource.ResourceCache{
		"glue": resource.ResourceCacheEntry{Resources: []resource.Resource{glueRes}},
	}

	checker := roleCheckerByTarget(t, "glue")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_Role_Glue_NotFound(t *testing.T) {
	source := resource.Resource{
		ID:   "other-role",
		Name: "other-role",
		Fields: map[string]string{
			"role_name": "other-role",
		},
		RawStruct: iamtypes.Role{
			RoleName: aws.String("other-role"),
			Arn:      aws.String("arn:aws:iam::123456789012:role/other-role"),
		},
	}

	glueRes := resource.Resource{
		ID:   "my-etl-job",
		Name: "my-etl-job",
		RawStruct: gluetypes.Job{
			Name: aws.String("my-etl-job"),
			Role: aws.String("acme-glue-role"),
		},
	}
	cache := resource.ResourceCache{
		"glue": resource.ResourceCacheEntry{Resources: []resource.Resource{glueRes}},
	}

	checker := roleCheckerByTarget(t, "glue")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_Role_Glue_CacheMissNoClients(t *testing.T) {
	source := resource.Resource{
		ID:   "acme-glue-role",
		Name: "acme-glue-role",
		Fields: map[string]string{
			"role_name": "acme-glue-role",
		},
	}

	checker := roleCheckerByTarget(t, "glue")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (unknown)", result.Count)
	}
}

// --- Node Group checker (reverse-lookup: search ng cache, ARN last segment) ---

func TestRelated_Role_NG_Found(t *testing.T) {
	source := resource.Resource{
		ID:   "eks-node-role",
		Name: "eks-node-role",
		Fields: map[string]string{
			"role_name": "eks-node-role",
		},
		RawStruct: iamtypes.Role{
			RoleName: aws.String("eks-node-role"),
			Arn:      aws.String("arn:aws:iam::123456789012:role/eks-node-role"),
		},
	}

	ngRes := resource.Resource{
		ID:   "general-pool",
		Name: "general-pool",
		RawStruct: ekstypes.Nodegroup{
			NodegroupName: aws.String("general-pool"),
			NodeRole:      aws.String("arn:aws:iam::123456789012:role/eks-node-role"),
		},
	}
	cache := resource.ResourceCache{
		"ng": resource.ResourceCacheEntry{Resources: []resource.Resource{ngRes}},
	}

	checker := roleCheckerByTarget(t, "ng")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_Role_NG_NotFound(t *testing.T) {
	source := resource.Resource{
		ID:   "other-role",
		Name: "other-role",
		Fields: map[string]string{
			"role_name": "other-role",
		},
		RawStruct: iamtypes.Role{
			RoleName: aws.String("other-role"),
			Arn:      aws.String("arn:aws:iam::123456789012:role/other-role"),
		},
	}

	ngRes := resource.Resource{
		ID:   "general-pool",
		Name: "general-pool",
		RawStruct: ekstypes.Nodegroup{
			NodegroupName: aws.String("general-pool"),
			NodeRole:      aws.String("arn:aws:iam::123456789012:role/eks-node-role"),
		},
	}
	cache := resource.ResourceCache{
		"ng": resource.ResourceCacheEntry{Resources: []resource.Resource{ngRes}},
	}

	checker := roleCheckerByTarget(t, "ng")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_Role_NG_CacheMissNoClients(t *testing.T) {
	source := resource.Resource{
		ID:   "eks-node-role",
		Name: "eks-node-role",
		Fields: map[string]string{
			"role_name": "eks-node-role",
		},
	}

	checker := roleCheckerByTarget(t, "ng")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (unknown)", result.Count)
	}
}

// --- Policy stub test ---

func TestRelated_Role_Policy_CheckerIsNil(t *testing.T) {
	for _, def := range resource.GetRelated("role") {
		if def.TargetType == "policy" {
			if def.Checker != nil {
				t.Errorf("role related checker for policy should be nil (stub), got non-nil")
			}
			return
		}
	}
	t.Fatal("role related def for policy not found")
}

// --- Demo checker ---

func TestRelatedDemo_Role_Registered(t *testing.T) {
	_ = demo.GetResources // ensure demo package is initialized
	checker := resource.GetRelatedDemo("role")
	if checker == nil {
		t.Fatal("no demo checker registered for role")
	}

	results := checker(resource.Resource{ID: "acme-lambda-execution"})
	if len(results) == 0 {
		t.Fatal("demo checker returned no results")
	}

	// Verify all 4 expected target types are present.
	wantTargets := map[string]bool{
		"lambda": false,
		"glue":   false,
		"ng":     false,
		"policy": false,
	}
	for _, r := range results {
		if r.TargetType == "" {
			t.Error("demo result has empty TargetType")
		}
		if _, ok := wantTargets[r.TargetType]; ok {
			wantTargets[r.TargetType] = true
		}
	}
	for target, found := range wantTargets {
		if !found {
			t.Errorf("demo checker missing result for target %q", target)
		}
	}

	// Verify at least one result has Count > 0 (lambda should have Count >= 1).
	hasPositiveCount := false
	for _, r := range results {
		if r.Count > 0 {
			hasPositiveCount = true
			break
		}
	}
	if !hasPositiveCount {
		t.Error("expected at least one demo result with Count > 0 (lambda should match)")
	}
}
