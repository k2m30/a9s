package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/codebuild"
)

// CodeBuildListProjectsAPI defines the interface for the CodeBuild ListProjects operation.
type CodeBuildListProjectsAPI interface {
	ListProjects(ctx context.Context, params *codebuild.ListProjectsInput, optFns ...func(*codebuild.Options)) (*codebuild.ListProjectsOutput, error)
}

// CodeBuildBatchGetProjectsAPI defines the interface for the CodeBuild BatchGetProjects operation.
type CodeBuildBatchGetProjectsAPI interface {
	BatchGetProjects(ctx context.Context, params *codebuild.BatchGetProjectsInput, optFns ...func(*codebuild.Options)) (*codebuild.BatchGetProjectsOutput, error)
}

// CodeBuildListBuildsForProjectAPI defines the interface for the CodeBuild ListBuildsForProject operation.
type CodeBuildListBuildsForProjectAPI interface {
	ListBuildsForProject(ctx context.Context, params *codebuild.ListBuildsForProjectInput, optFns ...func(*codebuild.Options)) (*codebuild.ListBuildsForProjectOutput, error)
}

// CodeBuildBatchGetBuildsAPI defines the interface for the CodeBuild BatchGetBuilds operation.
type CodeBuildBatchGetBuildsAPI interface {
	BatchGetBuilds(ctx context.Context, params *codebuild.BatchGetBuildsInput, optFns ...func(*codebuild.Options)) (*codebuild.BatchGetBuildsOutput, error)
}

// CodeBuildAPI is the aggregate interface covering all CodeBuild operations used by a9s fetchers.
// *codebuild.Client structurally satisfies this interface.
type CodeBuildAPI interface {
	CodeBuildListProjectsAPI
	CodeBuildBatchGetProjectsAPI
	CodeBuildListBuildsForProjectAPI
	CodeBuildBatchGetBuildsAPI
}
