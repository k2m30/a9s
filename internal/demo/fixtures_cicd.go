package demo

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	cbtypes "github.com/aws/aws-sdk-go-v2/service/codebuild/types"
	cptypes "github.com/aws/aws-sdk-go-v2/service/codepipeline/types"
	ecrtypes "github.com/aws/aws-sdk-go-v2/service/ecr/types"

	codeartifacttypes "github.com/aws/aws-sdk-go-v2/service/codeartifact/types"

	"github.com/k2m30/a9s/internal/resource"
)

func init() {
	demoData["cfn"] = cfnFixtures
	demoData["pipeline"] = pipelineFixtures
	demoData["cb"] = codebuildFixtures
	demoData["ecr"] = ecrFixtures
	demoData["codeartifact"] = codeartifactFixtures
}

// cfnFixtures returns demo CloudFormation Stack fixtures.
func cfnFixtures() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "acme-vpc-stack",
			Name:   "acme-vpc-stack",
			Status: "CREATE_COMPLETE",
			Fields: map[string]string{
				"stack_name":    "acme-vpc-stack",
				"status":        "CREATE_COMPLETE",
				"creation_time": "2024-10-15T09:00:00+00:00",
				"last_updated":  "2025-06-20T14:30:00+00:00",
				"description":   "Core VPC networking stack for Acme Corp production",
			},
			RawStruct: cfntypes.Stack{
				StackName:       aws.String("acme-vpc-stack"),
				StackStatus:     cfntypes.StackStatusCreateComplete,
				CreationTime:    aws.Time(mustParseTime("2024-10-15T09:00:00+00:00")),
				LastUpdatedTime: aws.Time(mustParseTime("2025-06-20T14:30:00+00:00")),
				Description:     aws.String("Core VPC networking stack for Acme Corp production"),
				StackId:         aws.String("arn:aws:cloudformation:us-east-1:123456789012:stack/acme-vpc-stack/11111111-1111-1111-1111-111111111111"),
			},
		},
		{
			ID:     "acme-eks-cluster",
			Name:   "acme-eks-cluster",
			Status: "UPDATE_COMPLETE",
			Fields: map[string]string{
				"stack_name":    "acme-eks-cluster",
				"status":        "UPDATE_COMPLETE",
				"creation_time": "2025-01-10T11:00:00+00:00",
				"last_updated":  "2026-03-15T08:45:00+00:00",
				"description":   "EKS cluster and managed node groups",
			},
			RawStruct: cfntypes.Stack{
				StackName:       aws.String("acme-eks-cluster"),
				StackStatus:     cfntypes.StackStatusUpdateComplete,
				CreationTime:    aws.Time(mustParseTime("2025-01-10T11:00:00+00:00")),
				LastUpdatedTime: aws.Time(mustParseTime("2026-03-15T08:45:00+00:00")),
				Description:     aws.String("EKS cluster and managed node groups"),
				StackId:         aws.String("arn:aws:cloudformation:us-east-1:123456789012:stack/acme-eks-cluster/22222222-2222-2222-2222-222222222222"),
			},
		},
		{
			ID:     "acme-rds-aurora",
			Name:   "acme-rds-aurora",
			Status: "CREATE_COMPLETE",
			Fields: map[string]string{
				"stack_name":    "acme-rds-aurora",
				"status":        "CREATE_COMPLETE",
				"creation_time": "2025-03-05T16:20:00+00:00",
				"last_updated":  "",
				"description":   "Aurora PostgreSQL cluster for API backend",
			},
			RawStruct: cfntypes.Stack{
				StackName:    aws.String("acme-rds-aurora"),
				StackStatus:  cfntypes.StackStatusCreateComplete,
				CreationTime: aws.Time(mustParseTime("2025-03-05T16:20:00+00:00")),
				Description:  aws.String("Aurora PostgreSQL cluster for API backend"),
				StackId:      aws.String("arn:aws:cloudformation:us-east-1:123456789012:stack/acme-rds-aurora/33333333-3333-3333-3333-333333333333"),
			},
		},
		{
			ID:     "acme-monitoring",
			Name:   "acme-monitoring",
			Status: "UPDATE_ROLLBACK_COMPLETE",
			Fields: map[string]string{
				"stack_name":    "acme-monitoring",
				"status":        "UPDATE_ROLLBACK_COMPLETE",
				"creation_time": "2025-02-01T10:00:00+00:00",
				"last_updated":  "2026-03-18T22:15:00+00:00",
				"description":   "CloudWatch alarms and dashboards",
			},
			RawStruct: cfntypes.Stack{
				StackName:       aws.String("acme-monitoring"),
				StackStatus:     cfntypes.StackStatusUpdateRollbackComplete,
				CreationTime:    aws.Time(mustParseTime("2025-02-01T10:00:00+00:00")),
				LastUpdatedTime: aws.Time(mustParseTime("2026-03-18T22:15:00+00:00")),
				Description:     aws.String("CloudWatch alarms and dashboards"),
				StackId:         aws.String("arn:aws:cloudformation:us-east-1:123456789012:stack/acme-monitoring/44444444-4444-4444-4444-444444444444"),
			},
		},
	}
}

// pipelineFixtures returns demo CodePipeline fixtures.
func pipelineFixtures() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "acme-api-deploy",
			Name:   "acme-api-deploy",
			Status: "",
			Fields: map[string]string{
				"name":          "acme-api-deploy",
				"pipeline_type": "V2",
				"version":       "3",
				"created":       "2025-04-10 09:00:00",
				"updated":       "2026-03-20 11:30:00",
			},
			RawStruct: cptypes.PipelineSummary{
				Name:         aws.String("acme-api-deploy"),
				PipelineType: cptypes.PipelineTypeV2,
				Version:      aws.Int32(3),
				Created:      aws.Time(mustParseTime("2025-04-10T09:00:00+00:00")),
				Updated:      aws.Time(mustParseTime("2026-03-20T11:30:00+00:00")),
			},
		},
		{
			ID:     "acme-frontend-deploy",
			Name:   "acme-frontend-deploy",
			Status: "",
			Fields: map[string]string{
				"name":          "acme-frontend-deploy",
				"pipeline_type": "V2",
				"version":       "5",
				"created":       "2025-05-15 14:00:00",
				"updated":       "2026-03-19 16:45:00",
			},
			RawStruct: cptypes.PipelineSummary{
				Name:         aws.String("acme-frontend-deploy"),
				PipelineType: cptypes.PipelineTypeV2,
				Version:      aws.Int32(5),
				Created:      aws.Time(mustParseTime("2025-05-15T14:00:00+00:00")),
				Updated:      aws.Time(mustParseTime("2026-03-19T16:45:00+00:00")),
			},
		},
		{
			ID:     "acme-infra-pipeline",
			Name:   "acme-infra-pipeline",
			Status: "",
			Fields: map[string]string{
				"name":          "acme-infra-pipeline",
				"pipeline_type": "V1",
				"version":       "12",
				"created":       "2024-08-20 08:30:00",
				"updated":       "2026-03-10 10:00:00",
			},
			RawStruct: cptypes.PipelineSummary{
				Name:         aws.String("acme-infra-pipeline"),
				PipelineType: cptypes.PipelineTypeV1,
				Version:      aws.Int32(12),
				Created:      aws.Time(mustParseTime("2024-08-20T08:30:00+00:00")),
				Updated:      aws.Time(mustParseTime("2026-03-10T10:00:00+00:00")),
			},
		},
	}
}

// codebuildFixtures returns demo CodeBuild Project fixtures.
func codebuildFixtures() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "acme-api-build",
			Name:   "acme-api-build",
			Status: "",
			Fields: map[string]string{
				"name":          "acme-api-build",
				"source_type":   "GITHUB",
				"description":   "Build project for API microservice",
				"last_modified": "2026-03-18T10:30:00+00:00",
			},
			RawStruct: cbtypes.Project{
				Name:        aws.String("acme-api-build"),
				Arn:         aws.String("arn:aws:codebuild:us-east-1:123456789012:project/acme-api-build"),
				Description: aws.String("Build project for API microservice"),
				Source: &cbtypes.ProjectSource{
					Type: cbtypes.SourceTypeGithub,
				},
				LastModified: aws.Time(mustParseTime("2026-03-18T10:30:00+00:00")),
				Created:      aws.Time(mustParseTime("2025-06-01T09:00:00+00:00")),
			},
		},
		{
			ID:     "acme-frontend-build",
			Name:   "acme-frontend-build",
			Status: "",
			Fields: map[string]string{
				"name":          "acme-frontend-build",
				"source_type":   "CODECOMMIT",
				"description":   "Build project for React frontend",
				"last_modified": "2026-03-17T15:20:00+00:00",
			},
			RawStruct: cbtypes.Project{
				Name:        aws.String("acme-frontend-build"),
				Arn:         aws.String("arn:aws:codebuild:us-east-1:123456789012:project/acme-frontend-build"),
				Description: aws.String("Build project for React frontend"),
				Source: &cbtypes.ProjectSource{
					Type: cbtypes.SourceTypeCodecommit,
				},
				LastModified: aws.Time(mustParseTime("2026-03-17T15:20:00+00:00")),
				Created:      aws.Time(mustParseTime("2025-07-15T11:00:00+00:00")),
			},
		},
		{
			ID:     "acme-docker-images",
			Name:   "acme-docker-images",
			Status: "",
			Fields: map[string]string{
				"name":          "acme-docker-images",
				"source_type":   "S3",
				"description":   "Base Docker image builder",
				"last_modified": "2026-03-10T08:00:00+00:00",
			},
			RawStruct: cbtypes.Project{
				Name:        aws.String("acme-docker-images"),
				Arn:         aws.String("arn:aws:codebuild:us-east-1:123456789012:project/acme-docker-images"),
				Description: aws.String("Base Docker image builder"),
				Source: &cbtypes.ProjectSource{
					Type: cbtypes.SourceTypeS3,
				},
				LastModified: aws.Time(mustParseTime("2026-03-10T08:00:00+00:00")),
				Created:      aws.Time(mustParseTime("2025-04-20T14:30:00+00:00")),
			},
		},
	}
}

// ecrFixtures returns demo ECR Repository fixtures.
func ecrFixtures() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "acme/api-service",
			Name:   "acme/api-service",
			Status: "",
			Fields: map[string]string{
				"repository_name": "acme/api-service",
				"uri":             "123456789012.dkr.ecr.us-east-1.amazonaws.com/acme/api-service",
				"tag_mutability":  "IMMUTABLE",
				"scan_on_push":    "true",
				"created_at":      "2025-03-01 10:00:00",
			},
			RawStruct: ecrtypes.Repository{
				RepositoryName: aws.String("acme/api-service"),
				RepositoryUri:  aws.String("123456789012.dkr.ecr.us-east-1.amazonaws.com/acme/api-service"),
				RepositoryArn:  aws.String("arn:aws:ecr:us-east-1:123456789012:repository/acme/api-service"),
				RegistryId:     aws.String("123456789012"),
				ImageTagMutability: ecrtypes.ImageTagMutabilityImmutable,
				ImageScanningConfiguration: &ecrtypes.ImageScanningConfiguration{
					ScanOnPush: true,
				},
				CreatedAt: aws.Time(mustParseTime("2025-03-01T10:00:00+00:00")),
			},
		},
		{
			ID:     "acme/frontend",
			Name:   "acme/frontend",
			Status: "",
			Fields: map[string]string{
				"repository_name": "acme/frontend",
				"uri":             "123456789012.dkr.ecr.us-east-1.amazonaws.com/acme/frontend",
				"tag_mutability":  "MUTABLE",
				"scan_on_push":    "true",
				"created_at":      "2025-03-01 10:05:00",
			},
			RawStruct: ecrtypes.Repository{
				RepositoryName: aws.String("acme/frontend"),
				RepositoryUri:  aws.String("123456789012.dkr.ecr.us-east-1.amazonaws.com/acme/frontend"),
				RepositoryArn:  aws.String("arn:aws:ecr:us-east-1:123456789012:repository/acme/frontend"),
				RegistryId:     aws.String("123456789012"),
				ImageTagMutability: ecrtypes.ImageTagMutabilityMutable,
				ImageScanningConfiguration: &ecrtypes.ImageScanningConfiguration{
					ScanOnPush: true,
				},
				CreatedAt: aws.Time(mustParseTime("2025-03-01T10:05:00+00:00")),
			},
		},
		{
			ID:     "acme/base-images",
			Name:   "acme/base-images",
			Status: "",
			Fields: map[string]string{
				"repository_name": "acme/base-images",
				"uri":             "123456789012.dkr.ecr.us-east-1.amazonaws.com/acme/base-images",
				"tag_mutability":  "IMMUTABLE",
				"scan_on_push":    "false",
				"created_at":      "2025-01-15 08:30:00",
			},
			RawStruct: ecrtypes.Repository{
				RepositoryName: aws.String("acme/base-images"),
				RepositoryUri:  aws.String("123456789012.dkr.ecr.us-east-1.amazonaws.com/acme/base-images"),
				RepositoryArn:  aws.String("arn:aws:ecr:us-east-1:123456789012:repository/acme/base-images"),
				RegistryId:     aws.String("123456789012"),
				ImageTagMutability: ecrtypes.ImageTagMutabilityImmutable,
				ImageScanningConfiguration: &ecrtypes.ImageScanningConfiguration{
					ScanOnPush: false,
				},
				CreatedAt: aws.Time(mustParseTime("2025-01-15T08:30:00+00:00")),
			},
		},
		{
			ID:     "acme/batch-processor",
			Name:   "acme/batch-processor",
			Status: "",
			Fields: map[string]string{
				"repository_name": "acme/batch-processor",
				"uri":             "123456789012.dkr.ecr.us-east-1.amazonaws.com/acme/batch-processor",
				"tag_mutability":  "MUTABLE",
				"scan_on_push":    "true",
				"created_at":      "2025-06-20 12:00:00",
			},
			RawStruct: ecrtypes.Repository{
				RepositoryName: aws.String("acme/batch-processor"),
				RepositoryUri:  aws.String("123456789012.dkr.ecr.us-east-1.amazonaws.com/acme/batch-processor"),
				RepositoryArn:  aws.String("arn:aws:ecr:us-east-1:123456789012:repository/acme/batch-processor"),
				RegistryId:     aws.String("123456789012"),
				ImageTagMutability: ecrtypes.ImageTagMutabilityMutable,
				ImageScanningConfiguration: &ecrtypes.ImageScanningConfiguration{
					ScanOnPush: true,
				},
				CreatedAt: aws.Time(mustParseTime("2025-06-20T12:00:00+00:00")),
			},
		},
	}
}

// codeartifactFixtures returns demo CodeArtifact Repository fixtures.
func codeartifactFixtures() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "acme-npm",
			Name:   "acme-npm",
			Status: "",
			Fields: map[string]string{
				"repo_name":     "acme-npm",
				"domain_name":   "acme-artifacts",
				"domain_owner":  "123456789012",
				"arn":           "arn:aws:codeartifact:us-east-1:123456789012:repository/acme-artifacts/acme-npm",
				"description":   "Private npm registry for Acme frontend packages",
				"admin_account": "123456789012",
				"created_time":  "2025-04-01 09:00:00",
			},
			RawStruct: codeartifacttypes.RepositorySummary{
				Name:                 aws.String("acme-npm"),
				DomainName:           aws.String("acme-artifacts"),
				DomainOwner:          aws.String("123456789012"),
				Arn:                  aws.String("arn:aws:codeartifact:us-east-1:123456789012:repository/acme-artifacts/acme-npm"),
				Description:          aws.String("Private npm registry for Acme frontend packages"),
				AdministratorAccount: aws.String("123456789012"),
				CreatedTime:          aws.Time(mustParseTime("2025-04-01T09:00:00+00:00")),
			},
		},
		{
			ID:     "acme-pypi",
			Name:   "acme-pypi",
			Status: "",
			Fields: map[string]string{
				"repo_name":     "acme-pypi",
				"domain_name":   "acme-artifacts",
				"domain_owner":  "123456789012",
				"arn":           "arn:aws:codeartifact:us-east-1:123456789012:repository/acme-artifacts/acme-pypi",
				"description":   "Private PyPI repository for data pipeline packages",
				"admin_account": "123456789012",
				"created_time":  "2025-04-01 09:15:00",
			},
			RawStruct: codeartifacttypes.RepositorySummary{
				Name:                 aws.String("acme-pypi"),
				DomainName:           aws.String("acme-artifacts"),
				DomainOwner:          aws.String("123456789012"),
				Arn:                  aws.String("arn:aws:codeartifact:us-east-1:123456789012:repository/acme-artifacts/acme-pypi"),
				Description:          aws.String("Private PyPI repository for data pipeline packages"),
				AdministratorAccount: aws.String("123456789012"),
				CreatedTime:          aws.Time(mustParseTime("2025-04-01T09:15:00+00:00")),
			},
		},
		{
			ID:     "acme-maven",
			Name:   "acme-maven",
			Status: "",
			Fields: map[string]string{
				"repo_name":     "acme-maven",
				"domain_name":   "acme-artifacts",
				"domain_owner":  "123456789012",
				"arn":           "arn:aws:codeartifact:us-east-1:123456789012:repository/acme-artifacts/acme-maven",
				"description":   "Maven repository for Java microservices",
				"admin_account": "123456789012",
				"created_time":  "2025-04-01 09:30:00",
			},
			RawStruct: codeartifacttypes.RepositorySummary{
				Name:                 aws.String("acme-maven"),
				DomainName:           aws.String("acme-artifacts"),
				DomainOwner:          aws.String("123456789012"),
				Arn:                  aws.String("arn:aws:codeartifact:us-east-1:123456789012:repository/acme-artifacts/acme-maven"),
				Description:          aws.String("Maven repository for Java microservices"),
				AdministratorAccount: aws.String("123456789012"),
				CreatedTime:          aws.Time(mustParseTime("2025-04-01T09:30:00+00:00")),
			},
		},
	}
}
