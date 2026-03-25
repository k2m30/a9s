package demo

import (
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	cbtypes "github.com/aws/aws-sdk-go-v2/service/codebuild/types"
	cwlogstypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	cptypes "github.com/aws/aws-sdk-go-v2/service/codepipeline/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	demoData["pipeline"] = pipelineFixtures
	demoData["cb"] = codebuildFixtures

	RegisterChildDemo("cb_builds", func(parentCtx map[string]string) []resource.Resource {
		return cbBuildFixtures(parentCtx["project_name"])
	})
	RegisterChildDemo("cb_build_logs", func(parentCtx map[string]string) []resource.Resource {
		return cbBuildLogFixtures()
	})
	RegisterChildDemo("pipeline_stages", func(parentCtx map[string]string) []resource.Resource {
		return pipelineStageFixtures()
	})
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

// cbBuildFixtures returns demo CodeBuild Build fixtures for a given project.
func cbBuildFixtures(projectName string) []resource.Resource {
	logGroup := fmt.Sprintf("/aws/codebuild/%s", projectName)

	startTime1 := mustParseTime("2026-03-22T03:15:00+00:00")
	startTime2 := mustParseTime("2026-03-22T02:00:00+00:00")
	endTime2 := mustParseTime("2026-03-22T02:04:12+00:00")
	startTime3 := mustParseTime("2026-03-21T15:45:00+00:00")
	endTime3 := mustParseTime("2026-03-21T15:46:03+00:00")
	startTime4 := mustParseTime("2026-03-21T12:00:00+00:00")
	endTime4 := mustParseTime("2026-03-21T12:00:45+00:00")

	return []resource.Resource{
		{
			ID:     fmt.Sprintf("%s:build-142", projectName),
			Name:   "#142",
			Status: "IN_PROGRESS",
			Fields: map[string]string{
				"build_number":            "142",
				"build_status":            "IN_PROGRESS",
				"start_time":              "2026-03-22 03:15:00",
				"end_time":                "",
				"duration":                "~2m 0s",
				"source_version_short":    "a1b2c3d4",
				"initiator":              "codepipeline/acme-api-deploy",
				"build_id":               fmt.Sprintf("%s:build-142", projectName),
				"build_arn":              fmt.Sprintf("arn:aws:codebuild:us-east-1:123456789012:build/%s:build-142", projectName),
				"current_phase":          "BUILD",
				"source_version":         "a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0",
				"resolved_source_version": "a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0",
				"log_group_name":         logGroup,
				"log_stream_name":        fmt.Sprintf("build-142/%s", projectName),
			},
			RawStruct: cbtypes.Build{
				Id:           aws.String(fmt.Sprintf("%s:build-142", projectName)),
				Arn:          aws.String(fmt.Sprintf("arn:aws:codebuild:us-east-1:123456789012:build/%s:build-142", projectName)),
				BuildNumber:  aws.Int64(142),
				BuildStatus:  cbtypes.StatusTypeInProgress,
				StartTime:    &startTime1,
				CurrentPhase: aws.String("BUILD"),
				SourceVersion:         aws.String("a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0"),
				ResolvedSourceVersion: aws.String("a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0"),
				Initiator:    aws.String("codepipeline/acme-api-deploy"),
				ProjectName:  aws.String(projectName),
				Logs: &cbtypes.LogsLocation{
					GroupName:  aws.String(logGroup),
					StreamName: aws.String(fmt.Sprintf("build-142/%s", projectName)),
				},
			},
		},
		{
			ID:     fmt.Sprintf("%s:build-141", projectName),
			Name:   "#141",
			Status: "SUCCEEDED",
			Fields: map[string]string{
				"build_number":            "141",
				"build_status":            "SUCCEEDED",
				"start_time":              "2026-03-22 02:00:00",
				"end_time":                "2026-03-22 02:04:12",
				"duration":                "4m 12s",
				"source_version_short":    "e5f6a7b8",
				"initiator":              "",
				"build_id":               fmt.Sprintf("%s:build-141", projectName),
				"build_arn":              fmt.Sprintf("arn:aws:codebuild:us-east-1:123456789012:build/%s:build-141", projectName),
				"current_phase":          "COMPLETED",
				"source_version":         "e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0a1b2c3d4",
				"resolved_source_version": "e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0a1b2c3d4",
				"log_group_name":         logGroup,
				"log_stream_name":        fmt.Sprintf("build-141/%s", projectName),
			},
			RawStruct: cbtypes.Build{
				Id:           aws.String(fmt.Sprintf("%s:build-141", projectName)),
				Arn:          aws.String(fmt.Sprintf("arn:aws:codebuild:us-east-1:123456789012:build/%s:build-141", projectName)),
				BuildNumber:  aws.Int64(141),
				BuildStatus:  cbtypes.StatusTypeSucceeded,
				StartTime:    &startTime2,
				EndTime:      &endTime2,
				CurrentPhase: aws.String("COMPLETED"),
				SourceVersion:         aws.String("e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0a1b2c3d4"),
				ResolvedSourceVersion: aws.String("e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0a1b2c3d4"),
				ProjectName:  aws.String(projectName),
				Logs: &cbtypes.LogsLocation{
					GroupName:  aws.String(logGroup),
					StreamName: aws.String(fmt.Sprintf("build-141/%s", projectName)),
				},
			},
		},
		{
			ID:     fmt.Sprintf("%s:build-140", projectName),
			Name:   "#140",
			Status: "FAILED",
			Fields: map[string]string{
				"build_number":            "140",
				"build_status":            "FAILED",
				"start_time":              "2026-03-21 15:45:00",
				"end_time":                "2026-03-21 15:46:03",
				"duration":                "1m 3s",
				"source_version_short":    "34a5b6c7",
				"initiator":              "user/admin",
				"build_id":               fmt.Sprintf("%s:build-140", projectName),
				"build_arn":              fmt.Sprintf("arn:aws:codebuild:us-east-1:123456789012:build/%s:build-140", projectName),
				"current_phase":          "COMPLETED",
				"source_version":         "34a5b6c7d8e9f0a1b2c3d4e5f6a7b8c9d0e1f2a3",
				"resolved_source_version": "34a5b6c7d8e9f0a1b2c3d4e5f6a7b8c9d0e1f2a3",
				"log_group_name":         logGroup,
				"log_stream_name":        fmt.Sprintf("build-140/%s", projectName),
			},
			RawStruct: cbtypes.Build{
				Id:           aws.String(fmt.Sprintf("%s:build-140", projectName)),
				Arn:          aws.String(fmt.Sprintf("arn:aws:codebuild:us-east-1:123456789012:build/%s:build-140", projectName)),
				BuildNumber:  aws.Int64(140),
				BuildStatus:  cbtypes.StatusTypeFailed,
				StartTime:    &startTime3,
				EndTime:      &endTime3,
				CurrentPhase: aws.String("COMPLETED"),
				SourceVersion:         aws.String("34a5b6c7d8e9f0a1b2c3d4e5f6a7b8c9d0e1f2a3"),
				ResolvedSourceVersion: aws.String("34a5b6c7d8e9f0a1b2c3d4e5f6a7b8c9d0e1f2a3"),
				Initiator:    aws.String("user/admin"),
				ProjectName:  aws.String(projectName),
				Logs: &cbtypes.LogsLocation{
					GroupName:  aws.String(logGroup),
					StreamName: aws.String(fmt.Sprintf("build-140/%s", projectName)),
				},
			},
		},
		{
			ID:     fmt.Sprintf("%s:build-139", projectName),
			Name:   "#139",
			Status: "STOPPED",
			Fields: map[string]string{
				"build_number":            "139",
				"build_status":            "STOPPED",
				"start_time":              "2026-03-21 12:00:00",
				"end_time":                "2026-03-21 12:00:45",
				"duration":                "45s",
				"source_version_short":    "2b3c4d5e",
				"initiator":              "",
				"build_id":               fmt.Sprintf("%s:build-139", projectName),
				"build_arn":              fmt.Sprintf("arn:aws:codebuild:us-east-1:123456789012:build/%s:build-139", projectName),
				"current_phase":          "COMPLETED",
				"source_version":         "2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c",
				"resolved_source_version": "2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c",
				"log_group_name":         logGroup,
				"log_stream_name":        fmt.Sprintf("build-139/%s", projectName),
			},
			RawStruct: cbtypes.Build{
				Id:           aws.String(fmt.Sprintf("%s:build-139", projectName)),
				Arn:          aws.String(fmt.Sprintf("arn:aws:codebuild:us-east-1:123456789012:build/%s:build-139", projectName)),
				BuildNumber:  aws.Int64(139),
				BuildStatus:  cbtypes.StatusTypeStopped,
				StartTime:    &startTime4,
				EndTime:      &endTime4,
				CurrentPhase: aws.String("COMPLETED"),
				SourceVersion:         aws.String("2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c"),
				ResolvedSourceVersion: aws.String("2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c"),
				ProjectName:  aws.String(projectName),
				Logs: &cbtypes.LogsLocation{
					GroupName:  aws.String(logGroup),
					StreamName: aws.String(fmt.Sprintf("build-139/%s", projectName)),
				},
			},
		},
	}
}

// cbBuildLogFixtures returns demo CodeBuild build log event fixtures.
func cbBuildLogFixtures() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "evt-1711076100000-0",
			Name:   "[Container] Entering phase DOWNLOAD_SOURCE",
			Status: "IN_PROGRESS",
			Fields: map[string]string{
				"timestamp":      "2026-03-22 03:15:00",
				"message":        "[Container] Entering phase DOWNLOAD_SOURCE",
				"ingestion_time": "2026-03-22 03:15:01",
				"event_id":       "evt-1711076100000-0",
			},
			RawStruct: cwlogstypes.OutputLogEvent{
				Timestamp:     aws.Int64(1774253700000),
				Message:       aws.String("[Container] Entering phase DOWNLOAD_SOURCE"),
				IngestionTime: aws.Int64(1774253701000),
			},
		},
		{
			ID:     "evt-1711076105000-1",
			Name:   "[Container] Phase complete: DOWNLOAD_SOURCE",
			Status: "SUCCEEDED",
			Fields: map[string]string{
				"timestamp":      "2026-03-22 03:15:05",
				"message":        "[Container] Phase complete: DOWNLOAD_SOURCE",
				"ingestion_time": "2026-03-22 03:15:06",
				"event_id":       "evt-1711076105000-1",
			},
			RawStruct: cwlogstypes.OutputLogEvent{
				Timestamp:     aws.Int64(1774253705000),
				Message:       aws.String("[Container] Phase complete: DOWNLOAD_SOURCE"),
				IngestionTime: aws.Int64(1774253706000),
			},
		},
		{
			ID:     "evt-1711076110000-2",
			Name:   "[Container] Entering phase INSTALL",
			Status: "IN_PROGRESS",
			Fields: map[string]string{
				"timestamp":      "2026-03-22 03:15:10",
				"message":        "[Container] Entering phase INSTALL",
				"ingestion_time": "2026-03-22 03:15:11",
				"event_id":       "evt-1711076110000-2",
			},
			RawStruct: cwlogstypes.OutputLogEvent{
				Timestamp:     aws.Int64(1774253710000),
				Message:       aws.String("[Container] Entering phase INSTALL"),
				IngestionTime: aws.Int64(1774253711000),
			},
		},
		{
			ID:     "evt-1711076115000-3",
			Name:   "[Container] Running command npm ci",
			Status: "IN_PROGRESS",
			Fields: map[string]string{
				"timestamp":      "2026-03-22 03:15:15",
				"message":        "[Container] Running command npm ci",
				"ingestion_time": "2026-03-22 03:15:16",
				"event_id":       "evt-1711076115000-3",
			},
			RawStruct: cwlogstypes.OutputLogEvent{
				Timestamp:     aws.Int64(1774253715000),
				Message:       aws.String("[Container] Running command npm ci"),
				IngestionTime: aws.Int64(1774253716000),
			},
		},
		{
			ID:     "evt-1711076120000-4",
			Name:   "FAIL src/payment.test.ts",
			Status: "ERROR",
			Fields: map[string]string{
				"timestamp":      "2026-03-22 03:15:20",
				"message":        "FAIL src/payment.test.ts",
				"ingestion_time": "2026-03-22 03:15:21",
				"event_id":       "evt-1711076120000-4",
			},
			RawStruct: cwlogstypes.OutputLogEvent{
				Timestamp:     aws.Int64(1774253720000),
				Message:       aws.String("FAIL src/payment.test.ts"),
				IngestionTime: aws.Int64(1774253721000),
			},
		},
		{
			ID:     "evt-1711076125000-5",
			Name:   "[Container] Command did not exit successfully",
			Status: "ERROR",
			Fields: map[string]string{
				"timestamp":      "2026-03-22 03:15:25",
				"message":        "[Container] Command did not exit successfully",
				"ingestion_time": "2026-03-22 03:15:26",
				"event_id":       "evt-1711076125000-5",
			},
			RawStruct: cwlogstypes.OutputLogEvent{
				Timestamp:     aws.Int64(1774253725000),
				Message:       aws.String("[Container] Command did not exit successfully"),
				IngestionTime: aws.Int64(1774253726000),
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

// pipelineStageFixtures returns demo pipeline stage-action fixtures.
// 4 stages, 6 total stage-action rows (Staging has 2 actions, Production has 2).
func pipelineStageFixtures() []resource.Resource {
	t1 := mustParseTime("2026-03-22T03:10:00+00:00")
	t2 := mustParseTime("2026-03-22T03:14:00+00:00")
	t3 := mustParseTime("2026-03-22T03:18:00+00:00")
	t4 := mustParseTime("2026-03-22T03:22:00+00:00")
	t5 := mustParseTime("2026-03-22T03:23:00+00:00")

	return []resource.Resource{
		{
			ID: "Source/SourceAction", Name: "SourceAction", Status: "running",
			Fields: map[string]string{
				"stage_name": "Source", "stage_status": "Succeeded",
				"action_name": "SourceAction", "action_status": "Succeeded",
				"last_change_time": "2026-03-22 03:10:00",
				"external_url":     "https://github.com/acme/api/commit/a1b2c3d",
				"action_token": "", "action_error_details": "",
				"revision_id": "a1b2c3d4e5f6", "revision_summary": "a1b2c3d4e5f6",
			},
			RawStruct: awsclient.PipelineStageRow{
				StageName: "Source", StageStatus: "Succeeded",
				ActionName: "SourceAction", ActionStatus: "Succeeded",
				LastStatusChange: &t1,
				ExternalURL:     "https://github.com/acme/api/commit/a1b2c3d",
				RevisionId: "a1b2c3d4e5f6", RevisionSummary: "a1b2c3d4e5f6",
			},
		},
		{
			ID: "Build/BuildAction", Name: "BuildAction", Status: "running",
			Fields: map[string]string{
				"stage_name": "Build", "stage_status": "Succeeded",
				"action_name": "BuildAction", "action_status": "Succeeded",
				"last_change_time": "2026-03-22 03:14:00",
				"external_url":     "https://us-east-1.console.aws.amazon.com/codesuite/codebuild",
				"action_token": "", "action_error_details": "",
				"revision_id": "", "revision_summary": "",
			},
			RawStruct: awsclient.PipelineStageRow{
				StageName: "Build", StageStatus: "Succeeded",
				ActionName: "BuildAction", ActionStatus: "Succeeded",
				LastStatusChange: &t2,
				ExternalURL:     "https://us-east-1.console.aws.amazon.com/codesuite/codebuild",
			},
		},
		{
			ID: "Staging/DeployToStaging", Name: "DeployToStaging", Status: "running",
			Fields: map[string]string{
				"stage_name": "Staging", "stage_status": "Succeeded",
				"action_name": "DeployToStaging", "action_status": "Succeeded",
				"last_change_time": "2026-03-22 03:18:00", "external_url": "",
				"action_token": "", "action_error_details": "",
				"revision_id": "", "revision_summary": "",
			},
			RawStruct: awsclient.PipelineStageRow{
				StageName: "Staging", StageStatus: "Succeeded",
				ActionName: "DeployToStaging", ActionStatus: "Succeeded",
				LastStatusChange: &t3,
			},
		},
		{
			ID: "Staging/IntegrationTests", Name: "IntegrationTests", Status: "running",
			Fields: map[string]string{
				"stage_name": "", "stage_status": "",
				"action_name": "IntegrationTests", "action_status": "Succeeded",
				"last_change_time": "2026-03-22 03:22:00", "external_url": "",
				"action_token": "", "action_error_details": "",
				"revision_id": "", "revision_summary": "",
			},
			RawStruct: awsclient.PipelineStageRow{
				StageName: "Staging", StageStatus: "Succeeded",
				ActionName: "IntegrationTests", ActionStatus: "Succeeded",
				LastStatusChange: &t4,
			},
		},
		{
			ID: "Production/ApprovalGate", Name: "ApprovalGate", Status: "pending",
			Fields: map[string]string{
				"stage_name": "Production", "stage_status": "InProgress",
				"action_name": "ApprovalGate", "action_status": "InProgress",
				"last_change_time": "2026-03-22 03:23:00", "external_url": "",
				"action_token": "approval-token-abc123", "action_error_details": "",
				"revision_id": "", "revision_summary": "",
			},
			RawStruct: awsclient.PipelineStageRow{
				StageName: "Production", StageStatus: "InProgress",
				ActionName: "ApprovalGate", ActionStatus: "InProgress",
				LastStatusChange: &t5,
				Token: "approval-token-abc123",
			},
		},
		{
			ID: "Production/DeployToProduction", Name: "DeployToProduction", Status: "terminated",
			Fields: map[string]string{
				"stage_name": "", "stage_status": "",
				"action_name": "DeployToProduction", "action_status": "",
				"last_change_time": "", "external_url": "",
				"action_token": "", "action_error_details": "",
				"revision_id": "", "revision_summary": "",
			},
			RawStruct: awsclient.PipelineStageRow{
				StageName: "Production", StageStatus: "InProgress",
				ActionName: "DeployToProduction",
			},
		},
	}
}
