package demo

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	cbtypes "github.com/aws/aws-sdk-go-v2/service/codebuild/types"
	cptypes "github.com/aws/aws-sdk-go-v2/service/codepipeline/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	demoData["pipeline"] = pipelineFixtures
	demoData["cb"] = codebuildFixtures
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
