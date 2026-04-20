package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/codepipeline"
)

// CodePipelineListPipelinesAPI defines the interface for the CodePipeline ListPipelines operation.
type CodePipelineListPipelinesAPI interface {
	ListPipelines(ctx context.Context, params *codepipeline.ListPipelinesInput, optFns ...func(*codepipeline.Options)) (*codepipeline.ListPipelinesOutput, error)
}

// CodePipelineGetPipelineStateAPI defines the interface for the CodePipeline GetPipelineState operation.
type CodePipelineGetPipelineStateAPI interface {
	GetPipelineState(ctx context.Context, params *codepipeline.GetPipelineStateInput, optFns ...func(*codepipeline.Options)) (*codepipeline.GetPipelineStateOutput, error)
}

// CodePipelineGetPipelineAPI defines the interface for the CodePipeline GetPipeline operation.
// Used by related-panel checkers that need the full stage/action structure for
// pipeline→* cross-references (cb, role, s3, sns, cfn, ecr, ecs-svc, lambda, kms, logs).
type CodePipelineGetPipelineAPI interface {
	GetPipeline(ctx context.Context, params *codepipeline.GetPipelineInput, optFns ...func(*codepipeline.Options)) (*codepipeline.GetPipelineOutput, error)
}

// CodePipelineAPI is the aggregate interface covering all CodePipeline operations used by a9s fetchers.
// *codepipeline.Client structurally satisfies this interface.
type CodePipelineAPI interface {
	CodePipelineListPipelinesAPI
	CodePipelineGetPipelineStateAPI
	CodePipelineGetPipelineAPI
}
