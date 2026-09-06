// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

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

// inlineCBBuildspec is the healthy buildspec shape: the commands live in the
// project definition, so changing them needs CodeBuild permissions rather than
// a pull request. Every project but CBBuildspecFromSource uses it.
const inlineCBBuildspec = `version: 0.2
phases:
  build:
    commands:
      - make ci
`

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
				Type:      cbtypes.SourceTypeGithub,
				Location:  aws.String("https://github.com/acme/api-service.git"),
				Buildspec: aws.String(inlineCBBuildspec),
			},
			// Artifacts.Location — required for the cb:s3 related-panel pivot
			// witness (checkCbS3). a9s-demo-healthy is a real s3.go bucket.
			Artifacts: &cbtypes.ProjectArtifacts{
				Type:     cbtypes.ArtifactsTypeS3,
				Location: aws.String(HealthyBucketName),
			},
			Cache: &cbtypes.ProjectCache{
				Type: cbtypes.CacheTypeLocal,
			},
			Environment: &cbtypes.ProjectEnvironment{
				Type: cbtypes.EnvironmentTypeLinuxContainer,
				// ECR-hosted build image — required for the cb:ecr
				// related-panel pivot witness (checkCbECR). acme/api-service
				// is a real ecr.go repository fixture.
				Image:       aws.String("123456789012.dkr.ecr.us-east-1.amazonaws.com/acme/api-service:builder"),
				ComputeType: cbtypes.ComputeTypeBuildGeneral1Small,
				// EnvironmentVariables — required for the cb:secrets and
				// cb:ssm related-panel pivot witnesses (checkCbSecrets /
				// checkCbSSM). prod/api/gateway-key is a real secrets.go
				// fixture; /acme/prod/app/config is a real ssm.go fixture.
				EnvironmentVariables: []cbtypes.EnvironmentVariable{
					{Name: aws.String("API_GATEWAY_KEY"), Type: cbtypes.EnvironmentVariableTypeSecretsManager, Value: aws.String("prod/api/gateway-key")},
					{Name: aws.String("APP_CONFIG"), Type: cbtypes.EnvironmentVariableTypeParameterStore, Value: aws.String("/acme/prod/app/config")},
				},
			},
			// VpcConfig — required for the cb:sg, cb:subnet and cb:vpc
			// related-panel pivot witnesses (checkCbSG / checkCbSubnet /
			// checkCbVPC). All three IDs are real ec2.go fixtures.
			VpcConfig: &cbtypes.VpcConfig{
				VpcId:            aws.String(fixtProdVPCID),
				Subnets:          []string{fixtProdPrivateSubnetA},
				SecurityGroupIds: []string{fixtProdAPIInternalSGID},
			},
			// EncryptionKey — required for the cb:kms related-panel pivot
			// witness (checkCbKMS). Shared prod KMS key used across fixtures.
			EncryptionKey: aws.String("arn:aws:kms:us-east-1:123456789012:key/a1b2c3d4-5678-90ab-cdef-111111111111"),
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
			// CBBuildspecFromSource witness: the build commands come from a
			// file in the repository, so a pull request can rewrite what runs
			// inside the build role.
			Source: &cbtypes.ProjectSource{
				Type:      cbtypes.SourceTypeCodecommit,
				Location:  aws.String("https://git-codecommit.us-east-1.amazonaws.com/v1/repos/acme-frontend"),
				Buildspec: aws.String("ci/buildspec.yml"),
			},
			LastModified: aws.Time(mustParseCBTime("2026-03-17T15:20:00+00:00")),
			Created:      aws.Time(mustParseCBTime("2025-07-15T11:00:00+00:00")),
		},
		{
			Name:        aws.String("acme-docker-images"),
			Arn:         aws.String("arn:aws:codebuild:us-east-1:123456789012:project/acme-docker-images"),
			Description: aws.String("Base Docker image builder"),
			// The one demo resource whose IAM Role pivot drills to a role
			// carrying a finding (RoleInlinePrivEsc, iam.go): without it the
			// drilled role detail is only reachable from the role list, so
			// nothing compares the two surfaces. Every other project keeps
			// prodCBRoleARN, so the pivot count is one either way.
			ServiceRole: aws.String("arn:aws:iam::123456789012:role/" + RoleInlinePrivEsc),
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
				Type:      cbtypes.SourceTypeGithub,
				Location:  aws.String("https://github.com/acme/integration-tests.git"),
				Buildspec: aws.String(inlineCBBuildspec),
			},
			// CBEnvSecret witness: the test database password is pasted into a
			// plaintext environment variable, so every build log prints it.
			Environment: &cbtypes.ProjectEnvironment{
				Type:        cbtypes.EnvironmentTypeLinuxContainer,
				Image:       aws.String("aws/codebuild/standard:7.0"),
				ComputeType: cbtypes.ComputeTypeBuildGeneral1Small,
				EnvironmentVariables: []cbtypes.EnvironmentVariable{
					{Name: aws.String("AWS_REGION"), Type: cbtypes.EnvironmentVariableTypePlaintext, Value: aws.String("us-east-1")},
					{Name: aws.String("TEST_DB_PASSWORD"), Type: cbtypes.EnvironmentVariableTypePlaintext, Value: aws.String("Tr0ub4dor&3xample")},
				},
			},
			LastModified: aws.Time(mustParseCBTime("2026-04-17T22:10:00+00:00")),
			Created:      aws.Time(mustParseCBTime("2025-08-05T10:00:00+00:00")),
		},
		// CBPublicBuilds witness: build logs and artifacts are readable by
		// anyone on the internet without an AWS account.
		{
			Name:              aws.String(CBPublicBuilds),
			Arn:               aws.String("arn:aws:codebuild:us-east-1:123456789012:project/" + CBPublicBuilds),
			Description:       aws.String("Publishes the public documentation site"),
			ServiceRole:       aws.String(prodCBRoleARN),
			ProjectVisibility: cbtypes.ProjectVisibilityTypePublicRead,
			Source: &cbtypes.ProjectSource{
				Type:      cbtypes.SourceTypeGithub,
				Location:  aws.String("https://github.com/acme/docs-site.git"),
				Buildspec: aws.String(inlineCBBuildspec),
			},
			LastModified: aws.Time(mustParseCBTime("2026-04-02T09:15:00+00:00")),
			Created:      aws.Time(mustParseCBTime("2025-09-12T13:00:00+00:00")),
		},
		// CBSourceURLCredential witness: a personal access token is embedded in
		// the clone address, stored in the project and echoed into build logs.
		{
			Name:        aws.String(CBSourceURLCredential),
			Arn:         aws.String("arn:aws:codebuild:us-east-1:123456789012:project/" + CBSourceURLCredential),
			Description: aws.String("Mirrors the legacy vendor repository nightly"),
			ServiceRole: aws.String(prodCBRoleARN),
			Source: &cbtypes.ProjectSource{
				Type:      cbtypes.SourceTypeBitbucket,
				Location:  aws.String("https://acmebot:Tr0ub4dor3xample@bitbucket.org/acme/legacy-mirror.git"),
				Buildspec: aws.String(inlineCBBuildspec),
			},
			LastModified: aws.Time(mustParseCBTime("2026-02-11T04:00:00+00:00")),
			Created:      aws.Time(mustParseCBTime("2024-11-30T08:45:00+00:00")),
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

// Witness projects for the cb posture findings. Each names the ONE demo
// project that carries its finding; every other project is set to the
// healthy value for that condition.
const (
	// CBPublicBuilds — build results are readable without an AWS account.
	CBPublicBuilds = "acme-docs-publish"
	// CBBuildspecFromSource — the buildspec is a file in the source repo.
	CBBuildspecFromSource = "acme-frontend-build"
	// CBSourceURLCredential — the source location embeds a credential.
	CBSourceURLCredential = "acme-legacy-mirror"
	// CBEnvSecret — a plaintext environment variable holds a credential.
	CBEnvSecret = "acme-integration-tests"
)

func init() {
	Register(Pin{ShortName: "cb", Rows: 6, Issues: 4, CoverageGaps: []string{"dim"}})
}
