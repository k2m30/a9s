package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	cbtypes "github.com/aws/aws-sdk-go-v2/service/codebuild/types"
	ecrtypes "github.com/aws/aws-sdk-go-v2/service/ecr/types"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/fieldpath"
	"github.com/k2m30/a9s/v3/internal/resource"
)

func ecrCheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("ecr") {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("ecr related checker for %s is nil", target)
			}
			return def.Checker
		}
	}
	t.Fatalf("ecr related checker for %s not found", target)
	return nil
}

// --- Navigable Field Registration ---

func TestNavigableFields_ECR_Registered(t *testing.T) {
	fields := resource.GetNavigableFields("ecr")
	if len(fields) == 0 {
		t.Fatal("no navigable fields registered for ecr")
	}

	expected := map[string]string{
		"EncryptionConfiguration.KmsKey": "kms",
	}
	for path, targetType := range expected {
		nav := resource.IsFieldNavigable("ecr", path)
		if nav == nil {
			t.Errorf("expected navigable field %q not found", path)
			continue
		}
		if nav.TargetType != targetType {
			t.Errorf("field %q: TargetType = %q, want %q", path, nav.TargetType, targetType)
		}
	}
}

func TestNavigableFields_ECR_FieldPathsResolve(t *testing.T) {
	resources, ok := demo.GetResources("ecr")
	if !ok {
		t.Fatal("no demo fixture registered for ecr — fixtures_cicd.go must register it")
	}
	if len(resources) == 0 {
		t.Fatal("demo fixture returned no resources for ecr")
	}

	fields := resource.GetNavigableFields("ecr")
	if len(fields) == 0 {
		t.Fatal("no navigable fields registered for ecr")
	}

	for _, nav := range fields {
		found := false
		for _, r := range resources {
			items := fieldpath.ExtractFieldList(r.RawStruct, r.Fields, []string{nav.FieldPath}, nil)
			for _, item := range items {
				if item.Value != "" && item.Value != "-" {
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		if !found {
			t.Skipf("NavigableField.FieldPath %q resolved to empty/missing value in all demo fixtures — no demo ECR repo with KMS encryption configured", nav.FieldPath)
		}
	}
}

// --- Demo Checker ---

func TestRelatedDemo_ECR_Registered(t *testing.T) {
	_ = demo.GetResources
	checker := resource.GetRelatedDemo("ecr")
	if checker == nil {
		t.Fatal("no demo checker registered for ecr")
	}

	results := checker(resource.Resource{ID: "acme/api-service"})
	if len(results) != 3 {
		t.Fatalf("demo checker returned %d results, want 3", len(results))
	}
	for _, r := range results {
		if r.TargetType == "" {
			t.Error("demo result has empty TargetType")
		}
	}
}

// --- Lambda checker (Pattern C — cache-based, heuristic PackageType=Image) ---

func TestRelated_ECR_Lambda_Found(t *testing.T) {
	repoURI := "123456789012.dkr.ecr.us-east-1.amazonaws.com/acme/api-service"
	lambdaRes := resource.Resource{
		ID: "my-image-fn",
		Fields: map[string]string{
			"package_type": "Image",
		},
		RawStruct: lambdatypes.FunctionConfiguration{
			PackageType: lambdatypes.PackageTypeImage,
		},
	}
	cache := resource.ResourceCache{
		"lambda": resource.ResourceCacheEntry{Resources: []resource.Resource{lambdaRes}},
	}
	source := resource.Resource{
		ID:     "acme/api-service",
		Fields: map[string]string{"uri": repoURI},
	}

	checker := ecrCheckerByTarget(t, "lambda")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "my-image-fn" {
		t.Errorf("ResourceIDs = %v, want [my-image-fn]", result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_ECR_Lambda_NotFound(t *testing.T) {
	repoURI := "123456789012.dkr.ecr.us-east-1.amazonaws.com/acme/api-service"
	lambdaRes := resource.Resource{
		ID: "my-zip-fn",
		Fields: map[string]string{
			"package_type": "Zip",
		},
		RawStruct: lambdatypes.FunctionConfiguration{
			PackageType: lambdatypes.PackageTypeZip,
		},
	}
	cache := resource.ResourceCache{
		"lambda": resource.ResourceCacheEntry{Resources: []resource.Resource{lambdaRes}},
	}
	source := resource.Resource{
		ID:     "acme/api-service",
		Fields: map[string]string{"uri": repoURI},
	}

	checker := ecrCheckerByTarget(t, "lambda")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_ECR_Lambda_CacheMissNoClients(t *testing.T) {
	source := resource.Resource{
		ID:     "acme/api-service",
		Fields: map[string]string{"uri": "123456789012.dkr.ecr.us-east-1.amazonaws.com/acme/api-service"},
	}

	checker := ecrCheckerByTarget(t, "lambda")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (unknown)", result.Count)
	}
}

func TestRelated_ECR_Lambda_EmptyURI(t *testing.T) {
	lambdaRes := resource.Resource{
		ID: "my-image-fn",
		Fields: map[string]string{
			"package_type": "Image",
		},
		RawStruct: lambdatypes.FunctionConfiguration{
			PackageType: lambdatypes.PackageTypeImage,
		},
	}
	cache := resource.ResourceCache{
		"lambda": resource.ResourceCacheEntry{Resources: []resource.Resource{lambdaRes}},
	}
	source := resource.Resource{
		ID:     "acme/api-service",
		Fields: map[string]string{"uri": ""},
	}

	checker := ecrCheckerByTarget(t, "lambda")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 for empty URI", result.Count)
	}
}

// --- CodeBuild checker (Pattern C — cache-based, image URI contains repo URI) ---

func TestRelated_ECR_CB_Found(t *testing.T) {
	repoURI := "123456789012.dkr.ecr.us-east-1.amazonaws.com/acme/api-service"
	cbRes := resource.Resource{
		ID: "my-build-project",
		RawStruct: cbtypes.Project{
			Environment: &cbtypes.ProjectEnvironment{
				Image: aws.String("123456789012.dkr.ecr.us-east-1.amazonaws.com/acme/api-service:latest"),
			},
		},
	}
	cache := resource.ResourceCache{
		"cb": resource.ResourceCacheEntry{Resources: []resource.Resource{cbRes}},
	}
	source := resource.Resource{
		ID:     "acme/api-service",
		Fields: map[string]string{"uri": repoURI},
	}

	checker := ecrCheckerByTarget(t, "cb")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "my-build-project" {
		t.Errorf("ResourceIDs = %v, want [my-build-project]", result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_ECR_CB_NotFound(t *testing.T) {
	repoURI := "123456789012.dkr.ecr.us-east-1.amazonaws.com/acme/api-service"
	cbRes := resource.Resource{
		ID: "my-build-project",
		RawStruct: cbtypes.Project{
			Environment: &cbtypes.ProjectEnvironment{
				Image: aws.String("aws/codebuild/standard:5.0"),
			},
		},
	}
	cache := resource.ResourceCache{
		"cb": resource.ResourceCacheEntry{Resources: []resource.Resource{cbRes}},
	}
	source := resource.Resource{
		ID:     "acme/api-service",
		Fields: map[string]string{"uri": repoURI},
	}

	checker := ecrCheckerByTarget(t, "cb")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_ECR_CB_CacheMissNoClients(t *testing.T) {
	source := resource.Resource{
		ID:     "acme/api-service",
		Fields: map[string]string{"uri": "123456789012.dkr.ecr.us-east-1.amazonaws.com/acme/api-service"},
	}

	checker := ecrCheckerByTarget(t, "cb")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (unknown)", result.Count)
	}
}

func TestRelated_ECR_CB_EmptyURI(t *testing.T) {
	cbRes := resource.Resource{
		ID: "my-build-project",
		RawStruct: cbtypes.Project{
			Environment: &cbtypes.ProjectEnvironment{
				Image: aws.String("123456789012.dkr.ecr.us-east-1.amazonaws.com/acme/api-service:latest"),
			},
		},
	}
	cache := resource.ResourceCache{
		"cb": resource.ResourceCacheEntry{Resources: []resource.Resource{cbRes}},
	}
	source := resource.Resource{
		ID:     "acme/api-service",
		Fields: map[string]string{"uri": ""},
	}

	checker := ecrCheckerByTarget(t, "cb")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 for empty URI", result.Count)
	}
}

// --- CloudFormation checker (Pattern C — cache-based, CFN tag on repo) ---

func TestRelated_ECR_CFN_Found(t *testing.T) {
	cfnRes := resource.Resource{
		ID: "my-stack",
		RawStruct: cfntypes.Stack{
			StackName: aws.String("my-stack"),
		},
	}
	cache := resource.ResourceCache{
		"cfn": resource.ResourceCacheEntry{Resources: []resource.Resource{cfnRes}},
	}
	// ECR DescribeRepositories does not embed tags; the cfn_stack_name field
	// is populated by tag enrichment into Fields["cfn_stack_name"].
	source := resource.Resource{
		ID: "acme/api-service",
		Fields: map[string]string{
			"uri":           "123456789012.dkr.ecr.us-east-1.amazonaws.com/acme/api-service",
			"cfn_stack_name": "my-stack",
		},
		RawStruct: ecrtypes.Repository{
			RepositoryName: aws.String("acme/api-service"),
		},
	}

	checker := ecrCheckerByTarget(t, "cfn")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "my-stack" {
		t.Errorf("ResourceIDs = %v, want [my-stack]", result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_ECR_CFN_NotFound(t *testing.T) {
	cfnRes := resource.Resource{
		ID: "different-stack",
		RawStruct: cfntypes.Stack{
			StackName: aws.String("different-stack"),
		},
	}
	cache := resource.ResourceCache{
		"cfn": resource.ResourceCacheEntry{Resources: []resource.Resource{cfnRes}},
	}
	source := resource.Resource{
		ID: "acme/api-service",
		Fields: map[string]string{
			"uri":           "123456789012.dkr.ecr.us-east-1.amazonaws.com/acme/api-service",
			"cfn_stack_name": "my-stack",
		},
		RawStruct: ecrtypes.Repository{
			RepositoryName: aws.String("acme/api-service"),
		},
	}

	checker := ecrCheckerByTarget(t, "cfn")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_ECR_CFN_CacheMissNoClients(t *testing.T) {
	source := resource.Resource{
		ID: "acme/api-service",
		Fields: map[string]string{
			"uri":           "123456789012.dkr.ecr.us-east-1.amazonaws.com/acme/api-service",
			"cfn_stack_name": "my-stack",
		},
		RawStruct: ecrtypes.Repository{
			RepositoryName: aws.String("acme/api-service"),
		},
	}

	checker := ecrCheckerByTarget(t, "cfn")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (unknown)", result.Count)
	}
}

func TestRelated_ECR_CFN_NoCFNTag(t *testing.T) {
	cfnRes := resource.Resource{
		ID: "my-stack",
		RawStruct: cfntypes.Stack{
			StackName: aws.String("my-stack"),
		},
	}
	cache := resource.ResourceCache{
		"cfn": resource.ResourceCacheEntry{Resources: []resource.Resource{cfnRes}},
	}
	// No cfn_stack_name in Fields — repo was not created by CFN.
	source := resource.Resource{
		ID: "acme/api-service",
		Fields: map[string]string{
			"uri": "123456789012.dkr.ecr.us-east-1.amazonaws.com/acme/api-service",
		},
		RawStruct: ecrtypes.Repository{
			RepositoryName: aws.String("acme/api-service"),
		},
	}

	checker := ecrCheckerByTarget(t, "cfn")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 for no CFN tag", result.Count)
	}
}
