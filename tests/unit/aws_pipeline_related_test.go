package unit_test

import (
	"context"
	"testing"

	cptypes "github.com/aws/aws-sdk-go-v2/service/codepipeline/types"
	_ "github.com/k2m30/a9s/v3/core/aws"
	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

func TestNavigableFields_Pipeline_None(t *testing.T) {
	fields := resource.GetNavigableFields("pipeline")
	if len(fields) != 0 {
		t.Errorf("expected no navigable fields for pipeline, got %d: %v", len(fields), fields)
	}
}

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

func TestRelated_Pipeline_EbRule_Match(t *testing.T) {
	src := resource.Resource{
		ID:   "my-pipeline",
		Name: "my-pipeline",
		Fields: map[string]string{
			"arn": "arn:aws:codepipeline:us-east-1:123456789012:my-pipeline",
		},
	}
	clients := &awsclient.ServiceClients{
		EventBridge: &fakeEventBridgeAPI{
			RuleNames: []string{"rule-deploy", "rule-notify", "rule-rollback"},
		},
	}
	checker := pipelineCheckerByTarget(t, "eb-rule")
	result := checker(context.Background(), clients, src, resource.ResourceCache{})

	if result.Count() != 3 {
		t.Errorf("Count = %d, want 3", result.Count())
	}
	if len(result.ResourceIDs()) != 3 {
		t.Errorf("ResourceIDs = %v, want 3 entries", result.ResourceIDs())
	}
}

func TestRelated_Pipeline_EbRule_Empty(t *testing.T) {
	src := resource.Resource{
		ID:     "my-pipeline",
		Name:   "my-pipeline",
		Fields: map[string]string{},
	}
	checker := pipelineCheckerByTarget(t, "eb-rule")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (empty ARN field)", result.Count())
	}
}

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

	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (nil clients)", result.Count())
	}
}

func TestRelated_Pipeline_CB_Match(t *testing.T) {
	const pipelineName = "build-pipeline"
	src := resource.Resource{ID: pipelineName, Name: pipelineName, Fields: map[string]string{}}
	clients := &awsclient.ServiceClients{
		CodePipeline: newFakeCodePipelineWithDeclarations(map[string]*cptypes.PipelineDeclaration{
			pipelineName: pipelineDeclarationWithCodeBuildAction(pipelineName, "my-build-project"),
		}),
	}
	checker := pipelineCheckerByTarget(t, "cb")
	result := checker(context.Background(), clients, src, resource.ResourceCache{})

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) == 0 || result.ResourceIDs()[0] != "my-build-project" {
		t.Errorf("ResourceIDs = %v, want [my-build-project]", result.ResourceIDs())
	}
}

func TestRelated_Pipeline_CB_NoMatch(t *testing.T) {
	const pipelineName = "deploy-only-pipeline"
	src := resource.Resource{ID: pipelineName, Name: pipelineName, Fields: map[string]string{}}
	clients := &awsclient.ServiceClients{
		CodePipeline: newFakeCodePipelineWithDeclarations(map[string]*cptypes.PipelineDeclaration{
			pipelineName: pipelineDeclarationEmpty(pipelineName),
		}),
	}
	checker := pipelineCheckerByTarget(t, "cb")
	result := checker(context.Background(), clients, src, resource.ResourceCache{})

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (no CodeBuild actions)", result.Count())
	}
}

func TestRelated_Pipeline_CB_NilClients(t *testing.T) {
	src := resource.Resource{ID: "build-pipeline", Fields: map[string]string{}}
	checker := pipelineCheckerByTarget(t, "cb")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})

	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (nil clients)", result.Count())
	}
}

// A wired client whose GetPipeline returns no Pipeline made the call and got
// a malformed response, so the pivot is RelatedError
// (docs/related-resources.md); RelatedUnknown is reserved for a missing
// CodePipeline client.
func TestRelated_Pipeline_CB_FailedLookup_ReturnsError(t *testing.T) {
	const pipelineName = "missing-pipeline"
	src := resource.Resource{ID: pipelineName, Fields: map[string]string{}}
	clients := &awsclient.ServiceClients{
		CodePipeline: newFakeCodePipelineWithDeclarations(nil), // GetPipeline misses -> malformed empty response
	}
	checker := pipelineCheckerByTarget(t, "cb")
	result := checker(context.Background(), clients, src, resource.ResourceCache{})

	if result.State() != domain.RelatedError {
		t.Errorf("State = %v, want RelatedError (client wired, call attempted, malformed empty response)", result.State())
	}
}

func TestRelated_Pipeline_Role_Match(t *testing.T) {
	const pipelineName = "role-pipeline"
	src := resource.Resource{ID: pipelineName, Name: pipelineName, Fields: map[string]string{}}
	clients := &awsclient.ServiceClients{
		CodePipeline: newFakeCodePipelineWithDeclarations(map[string]*cptypes.PipelineDeclaration{
			pipelineName: pipelineDeclarationWithRoleArn(pipelineName, "arn:aws:iam::123456789012:role/CodePipelineServiceRole"),
		}),
	}
	checker := pipelineCheckerByTarget(t, "role")
	result := checker(context.Background(), clients, src, resource.ResourceCache{})

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) == 0 || result.ResourceIDs()[0] != "CodePipelineServiceRole" {
		t.Errorf("ResourceIDs = %v, want [CodePipelineServiceRole]", result.ResourceIDs())
	}
}

func TestRelated_Pipeline_Role_NoRole(t *testing.T) {
	const pipelineName = "no-role-pipeline"
	src := resource.Resource{ID: pipelineName, Name: pipelineName, Fields: map[string]string{}}
	clients := &awsclient.ServiceClients{
		CodePipeline: newFakeCodePipelineWithDeclarations(map[string]*cptypes.PipelineDeclaration{
			pipelineName: pipelineDeclarationEmpty(pipelineName),
		}),
	}
	checker := pipelineCheckerByTarget(t, "role")
	result := checker(context.Background(), clients, src, resource.ResourceCache{})

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (no RoleArn set)", result.Count())
	}
}

func TestRelated_Pipeline_Role_FailedLookup_ReturnsError(t *testing.T) {
	const pipelineName = "missing-pipeline"
	src := resource.Resource{ID: pipelineName, Fields: map[string]string{}}
	clients := &awsclient.ServiceClients{
		CodePipeline: newFakeCodePipelineWithDeclarations(nil), // GetPipeline misses -> malformed empty response
	}
	checker := pipelineCheckerByTarget(t, "role")
	result := checker(context.Background(), clients, src, resource.ResourceCache{})

	if result.State() != domain.RelatedError {
		t.Errorf("State = %v, want RelatedError (client wired, call attempted, malformed empty response)", result.State())
	}
}

func TestRelated_Pipeline_CFN_Match(t *testing.T) {
	const pipelineName = "cfn-pipeline"
	src := resource.Resource{ID: pipelineName, Name: pipelineName, Fields: map[string]string{}}
	clients := &awsclient.ServiceClients{
		CodePipeline: newFakeCodePipelineWithDeclarations(map[string]*cptypes.PipelineDeclaration{
			pipelineName: pipelineDeclarationWithCFNAction(pipelineName, "prod-infra-stack"),
		}),
	}
	checker := pipelineCheckerByTarget(t, "cfn")
	result := checker(context.Background(), clients, src, resource.ResourceCache{})

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) == 0 || result.ResourceIDs()[0] != "prod-infra-stack" {
		t.Errorf("ResourceIDs = %v, want [prod-infra-stack]", result.ResourceIDs())
	}
}

// CodePipeline has no CodeArtifact action provider, so a pipeline declaration
// never names a CodeArtifact repository and pipeline offers no pivot to it.
func TestRelated_Pipeline_CodeArtifact_Match(t *testing.T) {
	for _, def := range resource.GetRelated("pipeline") {
		if def.TargetType == "codeartifact" {
			t.Errorf("pipeline registers a codeartifact pivot %q; no pipeline action names a CodeArtifact repository", def.DisplayName)
		}
	}
}

func TestRelated_Pipeline_ECR_Match(t *testing.T) {
	const pipelineName = "ecr-pipeline"
	const repoName = "my-app-image"
	src := resource.Resource{ID: pipelineName, Name: pipelineName, Fields: map[string]string{}}
	clients := &awsclient.ServiceClients{
		CodePipeline: newFakeCodePipelineWithDeclarations(map[string]*cptypes.PipelineDeclaration{
			pipelineName: pipelineDeclarationWithECRSourceAction(pipelineName, repoName),
		}),
	}
	checker := pipelineCheckerByTarget(t, "ecr")
	result := checker(context.Background(), clients, src, resource.ResourceCache{})

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) == 0 || result.ResourceIDs()[0] != repoName {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs(), repoName)
	}
}

func TestRelated_Pipeline_ECSSvc_Match(t *testing.T) {
	const pipelineName = "ecs-deploy-pipeline"
	const serviceName = "my-production-service"
	src := resource.Resource{ID: pipelineName, Name: pipelineName, Fields: map[string]string{}}
	clients := &awsclient.ServiceClients{
		CodePipeline: newFakeCodePipelineWithDeclarations(map[string]*cptypes.PipelineDeclaration{
			pipelineName: pipelineDeclarationWithECSSvcAction(pipelineName, serviceName),
		}),
	}
	checker := pipelineCheckerByTarget(t, "ecs-svc")
	result := checker(context.Background(), clients, src, resource.ResourceCache{})

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	// The action names the cluster it deploys into, and a service name is
	// unique inside a cluster.
	const wantID = "acme-prod/" + serviceName
	if len(result.ResourceIDs()) == 0 || result.ResourceIDs()[0] != wantID {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs(), wantID)
	}
}

func TestRelated_Pipeline_KMS_Match(t *testing.T) {
	const pipelineName = "kms-pipeline"
	const kmsARN = "arn:aws:kms:us-east-1:123456789012:key/deadbeef-aaaa-bbbb-cccc-000000000001"
	src := resource.Resource{ID: pipelineName, Name: pipelineName, Fields: map[string]string{}}
	clients := &awsclient.ServiceClients{
		CodePipeline: newFakeCodePipelineWithDeclarations(map[string]*cptypes.PipelineDeclaration{
			pipelineName: pipelineDeclarationWithArtifactStore(pipelineName, "my-artifacts-bucket", kmsARN),
		}),
	}
	checker := pipelineCheckerByTarget(t, "kms")
	result := checker(context.Background(), clients, src, resource.ResourceCache{})

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) == 0 || result.ResourceIDs()[0] != "deadbeef-aaaa-bbbb-cccc-000000000001" {
		t.Errorf("ResourceIDs = %v, want [deadbeef-aaaa-bbbb-cccc-000000000001]", result.ResourceIDs())
	}
}

func TestRelated_Pipeline_KMS_NoKey(t *testing.T) {
	const pipelineName = "no-kms-pipeline"
	src := resource.Resource{ID: pipelineName, Name: pipelineName, Fields: map[string]string{}}
	clients := &awsclient.ServiceClients{
		CodePipeline: newFakeCodePipelineWithDeclarations(map[string]*cptypes.PipelineDeclaration{
			pipelineName: pipelineDeclarationWithArtifactStore(pipelineName, "plain-bucket", ""),
		}),
	}
	checker := pipelineCheckerByTarget(t, "kms")
	result := checker(context.Background(), clients, src, resource.ResourceCache{})

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (no KMS key configured)", result.Count())
	}
}

func TestRelated_Pipeline_Lambda_Match(t *testing.T) {
	const pipelineName = "lambda-pipeline"
	const funcName = "my-gate-function"
	src := resource.Resource{ID: pipelineName, Name: pipelineName, Fields: map[string]string{}}
	clients := &awsclient.ServiceClients{
		CodePipeline: newFakeCodePipelineWithDeclarations(map[string]*cptypes.PipelineDeclaration{
			pipelineName: pipelineDeclarationWithLambdaAction(pipelineName, funcName),
		}),
	}
	checker := pipelineCheckerByTarget(t, "lambda")
	result := checker(context.Background(), clients, src, resource.ResourceCache{})

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) == 0 || result.ResourceIDs()[0] != funcName {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs(), funcName)
	}
}

func TestRelated_Pipeline_S3_ArtifactStore(t *testing.T) {
	const pipelineName = "s3-pipeline"
	const bucketName = "my-pipeline-artifacts"
	src := resource.Resource{ID: pipelineName, Name: pipelineName, Fields: map[string]string{}}
	clients := &awsclient.ServiceClients{
		CodePipeline: newFakeCodePipelineWithDeclarations(map[string]*cptypes.PipelineDeclaration{
			pipelineName: pipelineDeclarationWithArtifactStore(pipelineName, bucketName, ""),
		}),
	}
	checker := pipelineCheckerByTarget(t, "s3")
	result := checker(context.Background(), clients, src, resource.ResourceCache{})

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) == 0 || result.ResourceIDs()[0] != bucketName {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs(), bucketName)
	}
}

func TestRelated_Pipeline_S3_DeployAction(t *testing.T) {
	const pipelineName = "s3-deploy-pipeline"
	const deployBucket = "my-website-bucket"
	src := resource.Resource{ID: pipelineName, Name: pipelineName, Fields: map[string]string{}}
	clients := &awsclient.ServiceClients{
		CodePipeline: newFakeCodePipelineWithDeclarations(map[string]*cptypes.PipelineDeclaration{
			pipelineName: pipelineDeclarationWithS3DeployAction(pipelineName, deployBucket),
		}),
	}
	checker := pipelineCheckerByTarget(t, "s3")
	result := checker(context.Background(), clients, src, resource.ResourceCache{})

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) == 0 || result.ResourceIDs()[0] != deployBucket {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs(), deployBucket)
	}
}

func TestRelated_Pipeline_SNS_Match(t *testing.T) {
	const pipelineName = "sns-pipeline"
	const topicARN = "arn:aws:sns:us-east-1:123456789012:pipeline-approvals"
	src := resource.Resource{ID: pipelineName, Name: pipelineName, Fields: map[string]string{}}
	clients := &awsclient.ServiceClients{
		CodePipeline: newFakeCodePipelineWithDeclarations(map[string]*cptypes.PipelineDeclaration{
			pipelineName: pipelineDeclarationWithSNSApprovalAction(pipelineName, topicARN),
		}),
	}
	checker := pipelineCheckerByTarget(t, "sns")
	result := checker(context.Background(), clients, src, resource.ResourceCache{})

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) == 0 || result.ResourceIDs()[0] != topicARN {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs(), topicARN)
	}
}

func TestRelated_Pipeline_SNS_NoApproval(t *testing.T) {
	const pipelineName = "no-approval-pipeline"
	src := resource.Resource{ID: pipelineName, Name: pipelineName, Fields: map[string]string{}}
	clients := &awsclient.ServiceClients{
		CodePipeline: newFakeCodePipelineWithDeclarations(map[string]*cptypes.PipelineDeclaration{
			pipelineName: pipelineDeclarationEmpty(pipelineName),
		}),
	}
	checker := pipelineCheckerByTarget(t, "sns")
	result := checker(context.Background(), clients, src, resource.ResourceCache{})

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (no approval actions)", result.Count())
	}
}
