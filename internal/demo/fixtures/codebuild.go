package fixtures

import (
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	cbtypes "github.com/aws/aws-sdk-go-v2/service/codebuild/types"
)

// CodeBuildFixtures holds typed fixture data for CodeBuild.
type CodeBuildFixtures struct {
	Projects []cbtypes.Project
	// Builds maps project name to its builds (for ListBuildsForProject + BatchGetBuilds).
	Builds map[string][]cbtypes.Build
}

const prodCBRoleARN = "arn:aws:iam::123456789012:role/prod-ci-deploy-role"

func mustParseCBTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}

// NewCodeBuildFixtures constructs CodeBuildFixtures from the canonical demo data.
var sharedCodeBuildFixtures = sync.OnceValue(func() *CodeBuildFixtures {
	projects := []cbtypes.Project{
		{
			Name:                 aws.String("acme-api-build"),
			Arn:                  aws.String("arn:aws:codebuild:us-east-1:123456789012:project/acme-api-build"),
			Description:          aws.String("Build project for API microservice"),
			ServiceRole:          aws.String(prodCBRoleARN),
			ConcurrentBuildLimit: aws.Int32(10),
			Source: &cbtypes.ProjectSource{
				Type: cbtypes.SourceTypeGithub,
			},
			Cache: &cbtypes.ProjectCache{
				Type: cbtypes.CacheTypeLocal,
			},
			Environment: &cbtypes.ProjectEnvironment{
				Type:        cbtypes.EnvironmentTypeLinuxContainer,
				Image:       aws.String("aws/codebuild/standard:7.0"),
				ComputeType: cbtypes.ComputeTypeBuildGeneral1Small,
			},
			LogsConfig: &cbtypes.LogsConfig{
				CloudWatchLogs: &cbtypes.CloudWatchLogsConfig{
					Status:    cbtypes.LogsConfigStatusTypeEnabled,
					GroupName: aws.String("/aws/codebuild/acme-api-build"),
				},
			},
			Tags:         []cbtypes.Tag{{Key: aws.String("Environment"), Value: aws.String("production")}},
			LastModified: aws.Time(mustParseCBTime("2026-03-18T10:30:00+00:00")),
			Created:      aws.Time(mustParseCBTime("2025-06-01T09:00:00+00:00")),
		},
		{
			Name:        aws.String("acme-frontend-build"),
			Arn:         aws.String("arn:aws:codebuild:us-east-1:123456789012:project/acme-frontend-build"),
			Description: aws.String("Build project for React frontend"),
			ServiceRole: aws.String(prodCBRoleARN),
			Source: &cbtypes.ProjectSource{
				Type: cbtypes.SourceTypeCodecommit,
			},
			LastModified: aws.Time(mustParseCBTime("2026-03-17T15:20:00+00:00")),
			Created:      aws.Time(mustParseCBTime("2025-07-15T11:00:00+00:00")),
		},
		{
			Name:        aws.String("acme-docker-images"),
			Arn:         aws.String("arn:aws:codebuild:us-east-1:123456789012:project/acme-docker-images"),
			Description: aws.String("Base Docker image builder"),
			ServiceRole: aws.String(prodCBRoleARN),
			Source: &cbtypes.ProjectSource{
				Type: cbtypes.SourceTypeS3,
			},
			LastModified: aws.Time(mustParseCBTime("2026-03-10T08:00:00+00:00")),
			Created:      aws.Time(mustParseCBTime("2025-04-20T14:30:00+00:00")),
		},
		// Issue: latest build status=FAILED → Broken (build pipeline broken)
		{
			Name:        aws.String("acme-integration-tests"),
			Arn:         aws.String("arn:aws:codebuild:us-east-1:123456789012:project/acme-integration-tests"),
			Description: aws.String("Integration test suite runner"),
			ServiceRole: aws.String(prodCBRoleARN),
			Source: &cbtypes.ProjectSource{
				Type: cbtypes.SourceTypeGithub,
			},
			LastModified: aws.Time(mustParseCBTime("2026-04-17T22:10:00+00:00")),
			Created:      aws.Time(mustParseCBTime("2025-08-05T10:00:00+00:00")),
		},
	}

	buildsByProject := map[string][]cbtypes.Build{
		// Issue: latest build status=FAILED → Broken
		"acme-integration-tests": {
			{
				Id:                    aws.String("acme-integration-tests:build-38"),
				Arn:                   aws.String("arn:aws:codebuild:us-east-1:123456789012:build/acme-integration-tests:build-38"),
				BuildNumber:           aws.Int64(38),
				BuildStatus:           cbtypes.StatusTypeFailed,
				StartTime:             aws.Time(mustParseCBTime("2026-04-17T22:05:00+00:00")),
				EndTime:               aws.Time(mustParseCBTime("2026-04-17T22:09:47+00:00")),
				CurrentPhase:          aws.String("COMPLETED"),
				SourceVersion:         aws.String("b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2"),
				ResolvedSourceVersion: aws.String("b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2"),
				Initiator:             aws.String("codepipeline/acme-api-deploy"),
				ProjectName:           aws.String("acme-integration-tests"),
				Logs: &cbtypes.LogsLocation{
					GroupName:  aws.String("/aws/codebuild/acme-integration-tests"),
					StreamName: aws.String("build-38/acme-integration-tests"),
				},
			},
		},
		"acme-api-build": {
			{
				Id:                    aws.String("acme-api-build:build-142"),
				Arn:                   aws.String("arn:aws:codebuild:us-east-1:123456789012:build/acme-api-build:build-142"),
				BuildNumber:           aws.Int64(142),
				BuildStatus:           cbtypes.StatusTypeInProgress,
				StartTime:             aws.Time(mustParseCBTime("2026-03-22T03:15:00+00:00")),
				CurrentPhase:          aws.String("BUILD"),
				SourceVersion:         aws.String("a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0"),
				ResolvedSourceVersion: aws.String("a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0"),
				Initiator:             aws.String("codepipeline/acme-api-deploy"),
				ProjectName:           aws.String("acme-api-build"),
				Logs: &cbtypes.LogsLocation{
					GroupName:  aws.String("/aws/codebuild/acme-api-build"),
					StreamName: aws.String("build-142/acme-api-build"),
				},
			},
			{
				Id:                    aws.String("acme-api-build:build-141"),
				Arn:                   aws.String("arn:aws:codebuild:us-east-1:123456789012:build/acme-api-build:build-141"),
				BuildNumber:           aws.Int64(141),
				BuildStatus:           cbtypes.StatusTypeSucceeded,
				StartTime:             aws.Time(mustParseCBTime("2026-03-22T02:00:00+00:00")),
				EndTime:               aws.Time(mustParseCBTime("2026-03-22T02:04:12+00:00")),
				CurrentPhase:          aws.String("COMPLETED"),
				SourceVersion:         aws.String("e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0a1b2c3d4"),
				ResolvedSourceVersion: aws.String("e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0a1b2c3d4"),
				ProjectName:           aws.String("acme-api-build"),
				Logs: &cbtypes.LogsLocation{
					GroupName:  aws.String("/aws/codebuild/acme-api-build"),
					StreamName: aws.String("build-141/acme-api-build"),
				},
			},
		},
	}

	return &CodeBuildFixtures{
		Projects: projects,
		Builds:   buildsByProject,
	}
})

func NewCodeBuildFixtures() *CodeBuildFixtures {
	return sharedCodeBuildFixtures()
}
