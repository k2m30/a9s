package unit_test

import (
	"context"
	"testing"

	cptypes "github.com/aws/aws-sdk-go-v2/service/codepipeline/types"
	_ "github.com/k2m30/a9s/v3/internal/aws"
	awsclient "github.com/k2m30/a9s/v3/internal/aws"
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

// ---------------------------------------------------------------------------
// checkPipelineCB — CodeBuild project name extraction
// ---------------------------------------------------------------------------

// TestRelated_Pipeline_CB_Match verifies that a CodeBuild action's ProjectName
// is extracted and returned in ResourceIDs.
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

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) == 0 || result.ResourceIDs[0] != "my-build-project" {
		t.Errorf("ResourceIDs = %v, want [my-build-project]", result.ResourceIDs)
	}
}

// TestRelated_Pipeline_CB_NoMatch verifies Count=0 when no CodeBuild actions exist.
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

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no CodeBuild actions)", result.Count)
	}
}

// TestRelated_Pipeline_CB_NilClients verifies Count=-1 when clients are nil.
func TestRelated_Pipeline_CB_NilClients(t *testing.T) {
	src := resource.Resource{ID: "build-pipeline", Fields: map[string]string{}}
	checker := pipelineCheckerByTarget(t, "cb")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (nil clients)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkPipelineRole — IAM role name extraction from RoleArn
// ---------------------------------------------------------------------------

// TestRelated_Pipeline_Role_Match verifies the pipeline-level RoleArn last segment
// is returned as the role name.
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

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) == 0 || result.ResourceIDs[0] != "CodePipelineServiceRole" {
		t.Errorf("ResourceIDs = %v, want [CodePipelineServiceRole]", result.ResourceIDs)
	}
}

// TestRelated_Pipeline_Role_NoRole verifies Count=0 when RoleArn is empty.
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

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no RoleArn set)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkPipelineCFN — CloudFormation stack name extraction
// ---------------------------------------------------------------------------

// TestRelated_Pipeline_CFN_Match verifies that a CloudFormation action's StackName
// is extracted and returned.
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

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) == 0 || result.ResourceIDs[0] != "prod-infra-stack" {
		t.Errorf("ResourceIDs = %v, want [prod-infra-stack]", result.ResourceIDs)
	}
}

// ---------------------------------------------------------------------------
// checkPipelineCodeartifact — CodeArtifact repository name extraction
// ---------------------------------------------------------------------------

// TestRelated_Pipeline_CodeArtifact_Match verifies CodeArtifact repository name
// extraction from a Source action.
func TestRelated_Pipeline_CodeArtifact_Match(t *testing.T) {
	const pipelineName = "ca-pipeline"
	src := resource.Resource{ID: pipelineName, Name: pipelineName, Fields: map[string]string{}}
	clients := &awsclient.ServiceClients{
		CodePipeline: newFakeCodePipelineWithDeclarations(map[string]*cptypes.PipelineDeclaration{
			pipelineName: pipelineDeclarationWithCodeArtifactAction(pipelineName, "my-artifact-repo"),
		}),
	}
	checker := pipelineCheckerByTarget(t, "codeartifact")
	result := checker(context.Background(), clients, src, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) == 0 || result.ResourceIDs[0] != "my-artifact-repo" {
		t.Errorf("ResourceIDs = %v, want [my-artifact-repo]", result.ResourceIDs)
	}
}

// ---------------------------------------------------------------------------
// checkPipelineECR — ECR repository name extraction
// ---------------------------------------------------------------------------

// TestRelated_Pipeline_ECR_Match verifies ECR repository name extraction from
// an ECR Source action.
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

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) == 0 || result.ResourceIDs[0] != repoName {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, repoName)
	}
}

// ---------------------------------------------------------------------------
// checkPipelineECSSvc — ECS service name extraction
// ---------------------------------------------------------------------------

// TestRelated_Pipeline_ECSSvc_Match verifies ECS service name extraction from
// an ECS deploy action.
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

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) == 0 || result.ResourceIDs[0] != serviceName {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, serviceName)
	}
}

// ---------------------------------------------------------------------------
// checkPipelineKMS — KMS key extraction from ArtifactStore.EncryptionKey
// ---------------------------------------------------------------------------

// TestRelated_Pipeline_KMS_Match verifies KMS key ID extraction from
// ArtifactStore.EncryptionKey.Id (last ARN segment).
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

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) == 0 || result.ResourceIDs[0] != "deadbeef-aaaa-bbbb-cccc-000000000001" {
		t.Errorf("ResourceIDs = %v, want [deadbeef-aaaa-bbbb-cccc-000000000001]", result.ResourceIDs)
	}
}

// TestRelated_Pipeline_KMS_NoKey verifies Count=0 when no EncryptionKey is set.
func TestRelated_Pipeline_KMS_NoKey(t *testing.T) {
	const pipelineName = "no-kms-pipeline"
	src := resource.Resource{ID: pipelineName, Name: pipelineName, Fields: map[string]string{}}
	clients := &awsclient.ServiceClients{
		CodePipeline: newFakeCodePipelineWithDeclarations(map[string]*cptypes.PipelineDeclaration{
			// ArtifactStore with bucket but no KMS key
			pipelineName: pipelineDeclarationWithArtifactStore(pipelineName, "plain-bucket", ""),
		}),
	}
	checker := pipelineCheckerByTarget(t, "kms")
	result := checker(context.Background(), clients, src, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no KMS key configured)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkPipelineLambda — Lambda function name extraction
// ---------------------------------------------------------------------------

// TestRelated_Pipeline_Lambda_Match verifies Lambda function name extraction
// from an Invoke action.
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

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) == 0 || result.ResourceIDs[0] != funcName {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, funcName)
	}
}

// ---------------------------------------------------------------------------
// checkPipelineS3 — S3 bucket name extraction
// ---------------------------------------------------------------------------

// TestRelated_Pipeline_S3_ArtifactStore verifies that the ArtifactStore bucket
// location is returned as the S3 resource ID.
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

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) == 0 || result.ResourceIDs[0] != bucketName {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, bucketName)
	}
}

// TestRelated_Pipeline_S3_DeployAction verifies that an S3 deploy action's
// BucketName is included in results alongside (or instead of) ArtifactStore.
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

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) == 0 || result.ResourceIDs[0] != deployBucket {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, deployBucket)
	}
}

// ---------------------------------------------------------------------------
// checkPipelineSNS — SNS topic ARN extraction from Approval actions
// ---------------------------------------------------------------------------

// TestRelated_Pipeline_SNS_Match verifies that a Manual approval action's
// NotificationArn is returned as the SNS resource ID.
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

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) == 0 || result.ResourceIDs[0] != topicARN {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, topicARN)
	}
}

// TestRelated_Pipeline_SNS_NoApproval verifies Count=0 when no approval actions exist.
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

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no approval actions)", result.Count)
	}
}

