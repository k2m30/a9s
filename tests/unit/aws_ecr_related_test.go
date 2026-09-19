package unit_test

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	cbtypes "github.com/aws/aws-sdk-go-v2/service/codebuild/types"
	cptypes "github.com/aws/aws-sdk-go-v2/service/codepipeline/types"
	ecrtypes "github.com/aws/aws-sdk-go-v2/service/ecr/types"
	eventbridgetypes "github.com/aws/aws-sdk-go-v2/service/eventbridge/types"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"

	_ "github.com/k2m30/a9s/v3/core/aws"
	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// fakeCFNDescribeStacksErrors implements awsclient.CFNAPI and always fails
// DescribeStacks — used to exercise the "cfn cache miss, fetch fails" path
// without crashing on an unset CloudFormation client (which would nil-panic
// inside FetchCloudFormationStacksPage).
type fakeCFNDescribeStacksErrors struct{}

func (f *fakeCFNDescribeStacksErrors) DescribeStacks(_ context.Context, _ *cloudformation.DescribeStacksInput, _ ...func(*cloudformation.Options)) (*cloudformation.DescribeStacksOutput, error) {
	return nil, errors.New("simulated DescribeStacks failure")
}

func (f *fakeCFNDescribeStacksErrors) DescribeStackEvents(_ context.Context, _ *cloudformation.DescribeStackEventsInput, _ ...func(*cloudformation.Options)) (*cloudformation.DescribeStackEventsOutput, error) {
	return &cloudformation.DescribeStackEventsOutput{}, nil
}

func (f *fakeCFNDescribeStacksErrors) ListStackResources(_ context.Context, _ *cloudformation.ListStackResourcesInput, _ ...func(*cloudformation.Options)) (*cloudformation.ListStackResourcesOutput, error) {
	return &cloudformation.ListStackResourcesOutput{}, nil
}

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

func TestNavigableFields_ECR_Registered(t *testing.T) {
	fields := resource.GetNavigableFields("ecr")
	if len(fields) == 0 {
		t.Fatal("no navigable fields registered for ecr")
	}

	expected := map[string]string{
		"EncryptionConfiguration.KmsKey": "kms",
	}
	for path, targetType := range expected {
		nav := resource.IsFieldNavigableForTest("ecr", path)
		if nav == nil {
			t.Errorf("expected navigable field %q not found", path)
			continue
		}
		if nav.TargetType != targetType {
			t.Errorf("field %q: TargetType = %q, want %q", path, nav.TargetType, targetType)
		}
	}
}

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

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "my-image-fn" {
		t.Errorf("ResourceIDs = %v, want [my-image-fn]", result.ResourceIDs())
	}
	if result.Err() != nil {
		t.Errorf("unexpected error: %v", result.Err())
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

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0", result.Count())
	}
}

func TestRelated_ECR_Lambda_CacheMissNoClients(t *testing.T) {
	source := resource.Resource{
		ID:     "acme/api-service",
		Fields: map[string]string{"uri": "123456789012.dkr.ecr.us-east-1.amazonaws.com/acme/api-service"},
	}

	checker := ecrCheckerByTarget(t, "lambda")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (unknown)", result.Count())
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

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 for empty URI", result.Count())
	}
}

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

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "my-build-project" {
		t.Errorf("ResourceIDs = %v, want [my-build-project]", result.ResourceIDs())
	}
	if result.Err() != nil {
		t.Errorf("unexpected error: %v", result.Err())
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

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0", result.Count())
	}
}

func TestRelated_ECR_CB_CacheMissNoClients(t *testing.T) {
	source := resource.Resource{
		ID:     "acme/api-service",
		Fields: map[string]string{"uri": "123456789012.dkr.ecr.us-east-1.amazonaws.com/acme/api-service"},
	}

	checker := ecrCheckerByTarget(t, "cb")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (unknown)", result.Count())
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

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 for empty URI", result.Count())
	}
}

// checkECRCFN reads the aws:cloudformation:stack-name tag through
// ecr:ListTagsForResource on the RepositoryArn.
func TestRelated_ECR_CFN_Found(t *testing.T) {
	const repoArn = "arn:aws:ecr:us-east-1:123456789012:repository/acme/api-service"
	cfnRes := resource.Resource{
		ID: "my-stack",
		RawStruct: cfntypes.Stack{
			StackName: aws.String("my-stack"),
		},
	}
	cache := resource.ResourceCache{
		"cfn": resource.ResourceCacheEntry{Resources: []resource.Resource{cfnRes}},
	}
	source := resource.Resource{
		ID: "acme/api-service",
		Fields: map[string]string{
			"uri": "123456789012.dkr.ecr.us-east-1.amazonaws.com/acme/api-service",
		},
		RawStruct: ecrtypes.Repository{
			RepositoryName: aws.String("acme/api-service"),
			RepositoryArn:  aws.String(repoArn),
		},
	}
	fake := &fakeECRListTagsForResource{
		repoArn: repoArn,
		tags:    map[string]string{"aws:cloudformation:stack-name": "my-stack"},
	}
	clients := &awsclient.ServiceClients{ECR: fake}

	checker := ecrCheckerByTarget(t, "cfn")
	result := checker(context.Background(), clients, source, cache)

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "my-stack" {
		t.Errorf("ResourceIDs = %v, want [my-stack]", result.ResourceIDs())
	}
	if result.Err() != nil {
		t.Errorf("unexpected error: %v", result.Err())
	}
	if fake.calls == 0 {
		t.Error("ListTagsForResource was never called — checker must fetch tags per open repo")
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
			"uri":            "123456789012.dkr.ecr.us-east-1.amazonaws.com/acme/api-service",
			"cfn_stack_name": "my-stack",
		},
		RawStruct: ecrtypes.Repository{
			RepositoryName: aws.String("acme/api-service"),
		},
	}

	checker := ecrCheckerByTarget(t, "cfn")
	result := checker(context.Background(), nil, source, cache)

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0", result.Count())
	}
}

// A failed cfn page fetch over an empty cache is unknown, not zero.
func TestRelated_ECR_CFN_CacheMissNoClients(t *testing.T) {
	const repoArn = "arn:aws:ecr:us-east-1:123456789012:repository/acme/api-service"
	source := resource.Resource{
		ID: "acme/api-service",
		Fields: map[string]string{
			"uri": "123456789012.dkr.ecr.us-east-1.amazonaws.com/acme/api-service",
		},
		RawStruct: ecrtypes.Repository{
			RepositoryName: aws.String("acme/api-service"),
			RepositoryArn:  aws.String(repoArn),
		},
	}
	fakeECR := &fakeECRListTagsForResource{
		repoArn: repoArn,
		tags:    map[string]string{"aws:cloudformation:stack-name": "my-stack"},
	}
	clients := &awsclient.ServiceClients{ECR: fakeECR, CloudFormation: &fakeCFNDescribeStacksErrors{}}

	checker := ecrCheckerByTarget(t, "cfn")
	result := checker(context.Background(), clients, source, resource.ResourceCache{})

	if result.State() != domain.RelatedError {
		t.Errorf("State = %v, want RelatedError (CFN page-fetch failed)", result.State())
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

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 for no CFN tag", result.Count())
	}
}

func ecrEbRuleResource(ruleName, repoName string) resource.Resource {
	pattern := `{"source":["aws.ecr"],"detail":{"repository-name":["` + repoName + `"]}}`
	return resource.Resource{
		ID:   ruleName,
		Name: ruleName,
		RawStruct: eventbridgetypes.Rule{
			Name:         aws.String(ruleName),
			EventPattern: aws.String(pattern),
		},
	}
}

func TestRelated_ECR_EbRule_Match(t *testing.T) {
	const repoName = "acme/api-service"
	const ruleName = "ecr-push-rule"

	cache := resource.ResourceCache{
		"eb-rule": resource.ResourceCacheEntry{
			Resources: []resource.Resource{ecrEbRuleResource(ruleName, repoName)},
		},
	}
	source := resource.Resource{
		ID:   repoName,
		Name: repoName,
		Fields: map[string]string{
			"uri": "123456789012.dkr.ecr.us-east-1.amazonaws.com/" + repoName,
		},
		RawStruct: ecrtypes.Repository{
			RepositoryName: aws.String(repoName),
		},
	}

	checker := ecrCheckerByTarget(t, "eb-rule")
	result := checker(context.Background(), nil, source, cache)

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != ruleName {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs(), ruleName)
	}
	if result.Err() != nil {
		t.Errorf("unexpected error: %v", result.Err())
	}
}

func TestRelated_ECR_EbRule_Empty(t *testing.T) {
	cache := resource.ResourceCache{
		"eb-rule": resource.ResourceCacheEntry{
			Resources: []resource.Resource{ecrEbRuleResource("other-rule", "other/repo")},
		},
	}
	source := resource.Resource{
		ID:   "acme/api-service",
		Name: "acme/api-service",
		Fields: map[string]string{
			"uri": "123456789012.dkr.ecr.us-east-1.amazonaws.com/acme/api-service",
		},
		RawStruct: ecrtypes.Repository{
			RepositoryName: aws.String("acme/api-service"),
		},
	}

	checker := ecrCheckerByTarget(t, "eb-rule")
	result := checker(context.Background(), nil, source, cache)

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (rule references different repo)", result.Count())
	}
}

func TestRelated_ECR_EbRule_WrongRawStruct(t *testing.T) {
	source := resource.Resource{
		ID:        "acme/api-service",
		RawStruct: "not-a-repository",
	}

	checker := ecrCheckerByTarget(t, "eb-rule")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (wrong RawStruct type)", result.Count())
	}
}

func TestRelated_ECR_Pipeline_Match(t *testing.T) {
	const repoName = "acme/api-service"
	const pipelineName = "deploy-pipeline"

	fakeCp := newFakeCodePipelineWithDeclarations(map[string]*cptypes.PipelineDeclaration{
		pipelineName: pipelineDeclarationWithECRSourceAction(pipelineName, repoName),
	})
	clients := &awsclient.ServiceClients{CodePipeline: fakeCp}

	cache := resource.ResourceCache{
		"pipeline": resource.ResourceCacheEntry{
			Resources: []resource.Resource{{ID: pipelineName, Name: pipelineName}},
		},
	}
	source := resource.Resource{
		ID:   repoName,
		Name: repoName,
		Fields: map[string]string{
			"uri": "123456789012.dkr.ecr.us-east-1.amazonaws.com/" + repoName,
		},
		RawStruct: ecrtypes.Repository{
			RepositoryName: aws.String(repoName),
		},
	}

	checker := ecrCheckerByTarget(t, "pipeline")
	result := checker(context.Background(), clients, source, cache)

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != pipelineName {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs(), pipelineName)
	}
	if result.Err() != nil {
		t.Errorf("unexpected error: %v", result.Err())
	}
}

func TestRelated_ECR_Pipeline_Empty(t *testing.T) {
	const repoName = "acme/api-service"
	const pipelineName = "unrelated-pipeline"

	fakeCp := newFakeCodePipelineWithDeclarations(map[string]*cptypes.PipelineDeclaration{
		pipelineName: pipelineDeclarationEmpty(pipelineName),
	})
	clients := &awsclient.ServiceClients{CodePipeline: fakeCp}

	cache := resource.ResourceCache{
		"pipeline": resource.ResourceCacheEntry{
			Resources: []resource.Resource{{ID: pipelineName, Name: pipelineName}},
		},
	}
	source := resource.Resource{
		ID:   repoName,
		Name: repoName,
		Fields: map[string]string{
			"uri": "123456789012.dkr.ecr.us-east-1.amazonaws.com/" + repoName,
		},
		RawStruct: ecrtypes.Repository{
			RepositoryName: aws.String(repoName),
		},
	}

	checker := ecrCheckerByTarget(t, "pipeline")
	result := checker(context.Background(), clients, source, cache)

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (no pipeline references this repo)", result.Count())
	}
}

// Survivors of a partly failed reverse scan are a lower bound: 2 matches with
// 2 failures render "(2+)".
func TestRelated_ECR_Pipeline_PartialFailure_RendersInflatedTruncated(t *testing.T) {
	const repoName = "acme/api-service"

	fakeCp := newFakeCodePipelineWithDeclarations(map[string]*cptypes.PipelineDeclaration{
		"matching-pipeline-1": pipelineDeclarationWithECRSourceAction("matching-pipeline-1", repoName),
		"matching-pipeline-2": pipelineDeclarationWithECRSourceAction("matching-pipeline-2", repoName),
		// The failing-pipeline names have no declaration, so GetPipeline returns no
		// Pipeline and pipelineGetDeclaration reports an error.
	})
	clients := &awsclient.ServiceClients{CodePipeline: fakeCp}

	cache := resource.ResourceCache{
		"pipeline": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{ID: "matching-pipeline-1", Name: "matching-pipeline-1"},
				{ID: "matching-pipeline-2", Name: "matching-pipeline-2"},
				{ID: "failing-pipeline-1", Name: "failing-pipeline-1"},
				{ID: "failing-pipeline-2", Name: "failing-pipeline-2"},
			},
		},
	}
	source := resource.Resource{
		ID:        repoName,
		Name:      repoName,
		RawStruct: ecrtypes.Repository{RepositoryName: aws.String(repoName)},
	}

	checker := ecrCheckerByTarget(t, "pipeline")
	result := checker(context.Background(), clients, source, cache)

	if result.Count() != 2 {
		t.Fatalf("Count = %d, want 2 (the two survivors)", result.Count())
	}
	if !result.Truncated() {
		t.Fatal("Truncated = false, want true (2 of 4 lookups failed)")
	}
	display := resource.FormatRelatedCount(result.State(), result.Count(), result.Truncated())
	if display != "(2+)" {
		t.Errorf("rendered display = %q, want %q — a partial-failure count must never look exact", display, "(2+)")
	}
}

func TestRelated_ECR_Pipeline_AllFail_UnknownRelated(t *testing.T) {
	const repoName = "acme/api-service"

	clients := &awsclient.ServiceClients{
		CodePipeline: newFakeCodePipelineWithDeclarations(nil), // every lookup misses -> fails
	}

	cache := resource.ResourceCache{
		"pipeline": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{ID: "failing-pipeline-1", Name: "failing-pipeline-1"},
				{ID: "failing-pipeline-2", Name: "failing-pipeline-2"},
			},
		},
	}
	source := resource.Resource{
		ID:        repoName,
		Name:      repoName,
		RawStruct: ecrtypes.Repository{RepositoryName: aws.String(repoName)},
	}

	checker := ecrCheckerByTarget(t, "pipeline")
	result := checker(context.Background(), clients, source, cache)

	if result.State() != domain.RelatedUnknown {
		t.Fatalf("State = %v, want RelatedUnknown (every lookup failed)", result.State())
	}
	display := resource.FormatRelatedCount(result.State(), result.Count(), result.Truncated())
	if display != "" {
		t.Errorf("rendered display = %q, want %q — an all-failed scan must never render a confident zero", display, "")
	}
}

// Without a CodePipeline client no GetPipeline call succeeds, so the checker
// cannot claim a confident 0.
func TestRelated_ECR_Pipeline_NilCodePipelineClient_NotConfidentZeroWithoutASingleCall(t *testing.T) {
	const repoName = "acme/api-service"

	clients := &awsclient.ServiceClients{CodePipeline: nil}

	cache := resource.ResourceCache{
		"pipeline": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{ID: "some-pipeline-1", Name: "some-pipeline-1"},
				{ID: "some-pipeline-2", Name: "some-pipeline-2"},
			},
		},
	}
	source := resource.Resource{
		ID:        repoName,
		Name:      repoName,
		RawStruct: ecrtypes.Repository{RepositoryName: aws.String(repoName)},
	}

	checker := ecrCheckerByTarget(t, "pipeline")
	result := checker(context.Background(), clients, source, cache)

	if result.State() != domain.RelatedUnknown {
		t.Fatalf("State = %v, want RelatedUnknown (no CodePipeline client — nothing was ever checked)", result.State())
	}
	display := resource.FormatRelatedCount(result.State(), result.Count(), result.Truncated())
	if display != "" {
		t.Errorf("rendered display = %q, want %q — must not claim a definitive zero-pipelines-use-this-repo without a single successful call", display, "")
	}
}

func TestRelated_ECR_Pipeline_WrongRawStruct(t *testing.T) {
	source := resource.Resource{
		ID:        "acme/api-service",
		RawStruct: "not-a-repository",
	}

	checker := ecrCheckerByTarget(t, "pipeline")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (wrong RawStruct type)", result.Count())
	}
}

func ecrPolicyWithRoles(roleARNs ...string) string {
	arns := ""
	for i, arn := range roleARNs {
		if i > 0 {
			arns += ","
		}
		arns += `"` + arn + `"`
	}
	return `{"Statement":[{"Effect":"Allow","Principal":{"AWS":[` + arns + `]},"Action":["ecr:GetDownloadUrlForLayer"]}]}`
}

// Role IDs are bare RoleNames (via arnRoleName) because the role cache's
// FetchByIDs keys on RoleName.
func TestRelated_ECR_Role_Match(t *testing.T) {
	const repoName = "acme/api-service"
	const role1 = "arn:aws:iam::123456789012:role/deploy-role"
	const role2 = "arn:aws:iam::123456789012:role/ci-role"
	const role1Name = "deploy-role"
	const role2Name = "ci-role"

	fakeECR := newFakeECRWithRepositoryPolicy(ecrPolicyWithRoles(role1, role2))
	clients := &awsclient.ServiceClients{ECR: fakeECR}

	source := resource.Resource{
		ID:   repoName,
		Name: repoName,
		Fields: map[string]string{
			"uri": "123456789012.dkr.ecr.us-east-1.amazonaws.com/" + repoName,
		},
		RawStruct: ecrtypes.Repository{
			RepositoryName: aws.String(repoName),
		},
	}

	checker := ecrCheckerByTarget(t, "role")
	result := checker(context.Background(), clients, source, nil)

	if result.Count() != 2 {
		t.Errorf("Count = %d, want 2", result.Count())
	}
	seen := map[string]bool{}
	for _, id := range result.ResourceIDs() {
		seen[id] = true
	}
	if !seen[role1Name] {
		t.Errorf("ResourceIDs missing %q; got %v", role1Name, result.ResourceIDs())
	}
	if !seen[role2Name] {
		t.Errorf("ResourceIDs missing %q; got %v", role2Name, result.ResourceIDs())
	}
	if result.Err() != nil {
		t.Errorf("unexpected error: %v", result.Err())
	}
}

func TestRelated_ECR_Role_Empty(t *testing.T) {
	fakeECR := newFakeECRWithNoPolicyError()
	clients := &awsclient.ServiceClients{ECR: fakeECR}

	source := resource.Resource{
		ID:   "acme/api-service",
		Name: "acme/api-service",
		Fields: map[string]string{
			"uri": "123456789012.dkr.ecr.us-east-1.amazonaws.com/acme/api-service",
		},
		RawStruct: ecrtypes.Repository{
			RepositoryName: aws.String("acme/api-service"),
		},
	}

	checker := ecrCheckerByTarget(t, "role")
	result := checker(context.Background(), clients, source, nil)

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (no policy → no roles)", result.Count())
	}
}

// An untyped error whose message merely contains
// "RepositoryPolicyNotFoundException" is a RelatedError, not "no policy":
// checkECRRole classifies via ClassifyAWSError's errors.As(smithy.APIError).
func TestRelated_ECR_Role_UnrelatedErrorContainingExceptionNameIsNotSwallowed(t *testing.T) {
	fakeECR := &fakeECRForRole{
		getPolicyErr: errors.New("throttled while calling ecr:GetRepositoryPolicy (an unrelated log line happens to mention RepositoryPolicyNotFoundException)"),
	}
	clients := &awsclient.ServiceClients{ECR: fakeECR}

	source := resource.Resource{
		ID:   "acme/api-service",
		Name: "acme/api-service",
		Fields: map[string]string{
			"uri": "123456789012.dkr.ecr.us-east-1.amazonaws.com/acme/api-service",
		},
		RawStruct: ecrtypes.Repository{
			RepositoryName: aws.String("acme/api-service"),
		},
	}

	checker := ecrCheckerByTarget(t, "role")
	result := checker(context.Background(), clients, source, nil)

	if result.State() != domain.RelatedError {
		t.Errorf("State = %v, want RelatedError (an unrelated error must not become a confident zero)", result.State())
	}
	if result.Err() == nil {
		t.Error("expected Err() to be non-nil")
	}
}

func TestRelated_ECR_Role_WrongRawStruct(t *testing.T) {
	source := resource.Resource{
		ID:        "acme/api-service",
		RawStruct: "not-a-repository",
	}

	checker := ecrCheckerByTarget(t, "role")
	result := checker(context.Background(), nil, source, nil)

	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (wrong RawStruct type)", result.Count())
	}
}

func TestRelated_ECR_Role_NoClient(t *testing.T) {
	source := resource.Resource{
		ID:   "acme/api-service",
		Name: "acme/api-service",
		Fields: map[string]string{
			"uri": "123456789012.dkr.ecr.us-east-1.amazonaws.com/acme/api-service",
		},
		RawStruct: ecrtypes.Repository{
			RepositoryName: aws.String("acme/api-service"),
		},
	}

	checker := ecrCheckerByTarget(t, "role")
	result := checker(context.Background(), nil, source, nil)

	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (no client)", result.Count())
	}
}

// Fields["container_images"] is the comma-joined Containers[].Image list from
// DescribeTasks; checkECRECSTask matches ".dkr.ecr." plus "/repoName".
func ecrECSTaskResource(taskFamily, imageURI string) resource.Resource {
	return resource.Resource{
		ID:   taskFamily + ":1",
		Name: taskFamily + ":1",
		Fields: map[string]string{
			"container_images": imageURI,
		},
	}
}

func TestRelated_ECR_ECSTask_Match(t *testing.T) {
	const repoName = "acme/api-service"
	const account = "123456789012"
	const region = "us-east-1"
	imageURI := account + ".dkr.ecr." + region + ".amazonaws.com/" + repoName + ":latest"

	cache := resource.ResourceCache{
		"ecs-task": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				ecrECSTaskResource("api-task", imageURI),
				ecrECSTaskResource("unrelated-task", "unrelated.dkr.ecr.us-east-1.amazonaws.com/other/repo:latest"),
			},
		},
	}
	source := resource.Resource{
		ID:   repoName,
		Name: repoName,
		Fields: map[string]string{
			"uri": account + ".dkr.ecr." + region + ".amazonaws.com/" + repoName,
		},
		RawStruct: ecrtypes.Repository{
			RepositoryName: aws.String(repoName),
		},
	}

	checker := ecrCheckerByTarget(t, "ecs-task")
	result := checker(context.Background(), nil, source, cache)

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "api-task:1" {
		t.Errorf("ResourceIDs = %v, want [api-task:1]", result.ResourceIDs())
	}
	if result.Err() != nil {
		t.Errorf("unexpected error: %v", result.Err())
	}
	if len(result.FetchFilter()) != 0 {
		t.Errorf("FetchFilter = %v, want empty (reverse-scan must not set FetchFilter)", result.FetchFilter())
	}
}

func TestRelated_ECR_ECSTask_Match_Truncated(t *testing.T) {
	const repoName = "acme/worker"
	const account = "123456789012"
	const region = "us-east-1"
	imageURI := account + ".dkr.ecr." + region + ".amazonaws.com/" + repoName + ":v2"

	cache := resource.ResourceCache{
		"ecs-task": resource.ResourceCacheEntry{
			Resources:   []resource.Resource{ecrECSTaskResource("worker-task", imageURI)},
			IsTruncated: true,
		},
	}
	source := resource.Resource{
		ID:   repoName,
		Name: repoName,
		RawStruct: ecrtypes.Repository{
			RepositoryName: aws.String(repoName),
		},
	}

	checker := ecrCheckerByTarget(t, "ecs-task")
	result := checker(context.Background(), nil, source, cache)

	if result.Count() < 1 {
		t.Errorf("Count = %d, want >= 1", result.Count())
	}
	if !result.Truncated() {
		t.Error("Truncated = false, want true (IsTruncated=true)")
	}
}

func TestRelated_ECR_ECSTask_Empty(t *testing.T) {
	const repoName = "acme/no-match"
	const account = "123456789012"
	const region = "us-east-1"

	cache := resource.ResourceCache{
		"ecs-task": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				ecrECSTaskResource("other-task", account+".dkr.ecr."+region+".amazonaws.com/other/repo:latest"),
			},
		},
	}
	source := resource.Resource{
		ID:   repoName,
		Name: repoName,
		RawStruct: ecrtypes.Repository{
			RepositoryName: aws.String(repoName),
		},
	}

	checker := ecrCheckerByTarget(t, "ecs-task")
	result := checker(context.Background(), nil, source, cache)

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (no task references this repo)", result.Count())
	}
	if result.Err() != nil {
		t.Errorf("unexpected error: %v", result.Err())
	}
}

func TestRelated_ECR_KMS_Match(t *testing.T) {
	const keyARN = "arn:aws:kms:us-east-1:123456789012:key/mrk-abc1234567890def"
	const keyID = "mrk-abc1234567890def"

	source := resource.Resource{
		ID:   "acme/api-service",
		Name: "acme/api-service",
		RawStruct: ecrtypes.Repository{
			RepositoryName: aws.String("acme/api-service"),
			EncryptionConfiguration: &ecrtypes.EncryptionConfiguration{
				EncryptionType: ecrtypes.EncryptionTypeKms,
				KmsKey:         aws.String(keyARN),
			},
		},
	}

	checker := ecrCheckerByTarget(t, "kms")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.TargetType() != "kms" {
		t.Errorf("TargetType = %q, want %q", result.TargetType(), "kms")
	}
	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != keyID {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs(), keyID)
	}
	if result.Err() != nil {
		t.Errorf("unexpected error: %v", result.Err())
	}
}

func TestRelated_ECR_KMS_NoEncryptionConfig(t *testing.T) {
	source := resource.Resource{
		ID:   "acme/api-service",
		Name: "acme/api-service",
		RawStruct: ecrtypes.Repository{
			RepositoryName:          aws.String("acme/api-service"),
			EncryptionConfiguration: nil,
		},
	}

	checker := ecrCheckerByTarget(t, "kms")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (no EncryptionConfiguration)", result.Count())
	}
}

func TestRelated_ECR_KMS_WrongRawStruct(t *testing.T) {
	source := resource.Resource{
		ID:        "acme/api-service",
		RawStruct: "not-a-repository",
	}

	checker := ecrCheckerByTarget(t, "kms")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (wrong RawStruct type → assertStruct fails)", result.Count())
	}
}

func TestRelated_ECR_ECSTask_WrongRawStruct(t *testing.T) {
	const repoName = "acme/api-service"
	const account = "123456789012"
	const region = "us-east-1"
	imageURI := account + ".dkr.ecr." + region + ".amazonaws.com/" + repoName + ":latest"

	cache := resource.ResourceCache{
		"ecs-task": resource.ResourceCacheEntry{
			Resources: []resource.Resource{ecrECSTaskResource("api-task", imageURI)},
		},
	}
	// checkECRECSTask matches on res.ID, not RawStruct.
	source := resource.Resource{
		ID:        repoName,
		Name:      repoName,
		RawStruct: "wrong-type",
	}

	checker := ecrCheckerByTarget(t, "ecs-task")
	result := checker(context.Background(), nil, source, cache)

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1 (checker uses res.ID, not RawStruct)", result.Count())
	}
}
