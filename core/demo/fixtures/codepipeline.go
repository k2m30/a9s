// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package fixtures

import (
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	cptypes "github.com/aws/aws-sdk-go-v2/service/codepipeline/types"
)

// CodePipelineFixtures holds typed fixture data for CodePipeline.
type CodePipelineFixtures struct {
	Pipelines []cptypes.PipelineSummary
	// Declarations maps pipeline name -> full stage/action declaration,
	// served by GetPipeline. Required for every pipeline:* related-panel
	// pivot (checkPipelineCB/CFN/Codeartifact/ECR/ECSSvc/KMS/Lambda/S3/SNS
	// all call pipelineGetDeclaration and scan Stages[].Actions[]), and for
	// the reverse cb:pipeline pivot (checkCbPipeline calls the same API).
	Declarations map[string]*cptypes.PipelineDeclaration
	// States maps pipeline name -> GetPipelineState stage states. Backs
	// EnrichCodePipelineStatus's Wave-2 failed-stage issue check.
	States map[string][]cptypes.StageState
}

const pipelineArtifactStoreKMSKeyID = "arn:aws:kms:us-east-1:123456789012:key/a1b2c3d4-5678-90ab-cdef-111111111111"

func mustParseCPTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}

// NewCodePipelineFixtures constructs CodePipelineFixtures from the canonical demo data.
var sharedCodePipelineFixtures = sync.OnceValue(func() *CodePipelineFixtures {
	return &CodePipelineFixtures{
		Pipelines: []cptypes.PipelineSummary{
			{
				Name:          aws.String("acme-api-deploy"),
				PipelineType:  cptypes.PipelineTypeV2,
				ExecutionMode: cptypes.ExecutionModeQueued,
				Version:       aws.Int32(3),
				Created:       aws.Time(mustParseCPTime("2025-04-10T09:00:00+00:00")),
				Updated:       aws.Time(mustParseCPTime("2026-03-20T11:30:00+00:00")),
			},
			{
				Name:         aws.String("acme-frontend-deploy"),
				PipelineType: cptypes.PipelineTypeV2,
				Version:      aws.Int32(5),
				Created:      aws.Time(mustParseCPTime("2025-05-15T14:00:00+00:00")),
				Updated:      aws.Time(mustParseCPTime("2026-03-19T16:45:00+00:00")),
			},
			{
				Name:         aws.String("acme-infra-pipeline"),
				PipelineType: cptypes.PipelineTypeV1,
				Version:      aws.Int32(12),
				Created:      aws.Time(mustParseCPTime("2024-08-20T08:30:00+00:00")),
				Updated:      aws.Time(mustParseCPTime("2026-03-10T10:00:00+00:00")),
			},
		},
		// acme-api-deploy is the graph-root pipeline declaration used to
		// witness every pipeline:* related-panel pivot and the reverse
		// cb:pipeline pivot. RoleArn + Stages/actions reference real sibling
		// fixtures: acme-ci-deploy-role (iam.go), acme-api-build
		// (codebuild.go), acme-npm/acme-artifacts (codeartifact.go),
		// acme/api-service (ecr.go), api-gateway/acme-services (ecs.go),
		// acme-eks-cluster (cfn.go), api-gateway-authorizer (lambda.go),
		// a9s-demo-healthy (s3.go), ops-alerts (cloudwatch.go
		// relatedAlarmSNSARN topic name).
		Declarations: map[string]*cptypes.PipelineDeclaration{
			"acme-api-deploy": {
				Name:    aws.String("acme-api-deploy"),
				RoleArn: aws.String("arn:aws:iam::123456789012:role/acme-ci-deploy-role"),
				Version: aws.Int32(3),
				ArtifactStore: &cptypes.ArtifactStore{
					Type:     cptypes.ArtifactStoreTypeS3,
					Location: aws.String(HealthyBucketName),
					EncryptionKey: &cptypes.EncryptionKey{
						Id:   aws.String(pipelineArtifactStoreKMSKeyID),
						Type: cptypes.EncryptionKeyTypeKms,
					},
				},
				Stages: []cptypes.StageDeclaration{
					{
						Name: aws.String("Source"),
						Actions: []cptypes.ActionDeclaration{
							{
								Name: aws.String("CodeArtifactSource"),
								ActionTypeId: &cptypes.ActionTypeId{
									Category: cptypes.ActionCategorySource,
									Owner:    cptypes.ActionOwnerAws,
									Provider: aws.String("CodeArtifact"),
									Version:  aws.String("1"),
								},
								Configuration: map[string]string{
									"RepositoryName": "acme-npm",
									"DomainName":     "acme-artifacts",
									"PackageName":    "acme-api-service",
								},
							},
						},
					},
					{
						Name: aws.String("Build"),
						Actions: []cptypes.ActionDeclaration{
							{
								Name: aws.String("BuildAndPushImage"),
								ActionTypeId: &cptypes.ActionTypeId{
									Category: cptypes.ActionCategoryBuild,
									Owner:    cptypes.ActionOwnerAws,
									Provider: aws.String("CodeBuild"),
									Version:  aws.String("1"),
								},
								Configuration: map[string]string{
									"ProjectName": "acme-api-build",
								},
							},
							{
								Name: aws.String("PublishImage"),
								ActionTypeId: &cptypes.ActionTypeId{
									Category: cptypes.ActionCategoryBuild,
									Owner:    cptypes.ActionOwnerAws,
									Provider: aws.String("ECR"),
									Version:  aws.String("1"),
								},
								Configuration: map[string]string{
									"RepositoryName": "acme/api-service",
								},
							},
						},
					},
					{
						Name: aws.String("DeployInfra"),
						Actions: []cptypes.ActionDeclaration{
							{
								Name: aws.String("UpdateEKSStack"),
								ActionTypeId: &cptypes.ActionTypeId{
									Category: cptypes.ActionCategoryDeploy,
									Owner:    cptypes.ActionOwnerAws,
									Provider: aws.String("CloudFormation"),
									Version:  aws.String("1"),
								},
								Configuration: map[string]string{
									"StackName":  "acme-eks-cluster",
									"ActionMode": "CREATE_UPDATE",
								},
							},
						},
					},
					{
						Name: aws.String("Approval"),
						Actions: []cptypes.ActionDeclaration{
							{
								Name: aws.String("ManualApproval"),
								ActionTypeId: &cptypes.ActionTypeId{
									Category: cptypes.ActionCategoryApproval,
									Owner:    cptypes.ActionOwnerAws,
									Provider: aws.String("Manual"),
									Version:  aws.String("1"),
								},
								Configuration: map[string]string{
									"NotificationArn": relatedAlarmSNSARN,
								},
							},
						},
					},
					{
						Name: aws.String("Deploy"),
						Actions: []cptypes.ActionDeclaration{
							{
								Name: aws.String("DeployToECS"),
								ActionTypeId: &cptypes.ActionTypeId{
									Category: cptypes.ActionCategoryDeploy,
									Owner:    cptypes.ActionOwnerAws,
									Provider: aws.String("ECS"),
									Version:  aws.String("1"),
								},
								Configuration: map[string]string{
									"ClusterName": "acme-services",
									"ServiceName": "api-gateway",
								},
							},
							{
								Name: aws.String("InvokeMigrationLambda"),
								ActionTypeId: &cptypes.ActionTypeId{
									Category: cptypes.ActionCategoryInvoke,
									Owner:    cptypes.ActionOwnerAws,
									Provider: aws.String("Lambda"),
									Version:  aws.String("1"),
								},
								Configuration: map[string]string{
									"FunctionName": "api-gateway-authorizer",
								},
							},
							{
								Name: aws.String("UploadReleaseNotes"),
								ActionTypeId: &cptypes.ActionTypeId{
									Category: cptypes.ActionCategoryDeploy,
									Owner:    cptypes.ActionOwnerAws,
									Provider: aws.String("S3"),
									Version:  aws.String("1"),
								},
								Configuration: map[string]string{
									"BucketName": HealthyBucketName,
									"Extract":    "false",
								},
							},
						},
					},
				},
			},
		},
		// States — acme-frontend-deploy has a failed Deploy stage, required
		// for EnrichCodePipelineStatus's Wave-2 issue check.
		States: map[string][]cptypes.StageState{
			"acme-frontend-deploy": {
				{
					StageName: aws.String("Deploy"),
					LatestExecution: &cptypes.StageExecution{
						Status: cptypes.StageExecutionStatusFailed,
					},
					ActionStates: []cptypes.ActionState{
						{
							ActionName: aws.String("DeployToS3"),
							LatestExecution: &cptypes.ActionExecution{
								Status: cptypes.ActionExecutionStatusFailed,
								ErrorDetails: &cptypes.ErrorDetails{
									Message: aws.String("Access Denied: insufficient permissions to write to target bucket"),
								},
							},
						},
					},
				},
			},
		},
	}
})

func NewCodePipelineFixtures() *CodePipelineFixtures {
	return sharedCodePipelineFixtures()
}
