package fakes

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/codepipeline"
	cptypes "github.com/aws/aws-sdk-go-v2/service/codepipeline/types"

	"github.com/k2m30/a9s/v3/internal/demo/fixtures"
)

// CodePipelineFake implements aws.CodePipelineAPI against fixture data loaded at construction time.
type CodePipelineFake struct {
	fix *fixtures.CodePipelineFixtures
}

// NewCodePipeline constructs a CodePipelineFake backed by fixture data from the fixtures package.
func NewCodePipeline() *CodePipelineFake {
	return &CodePipelineFake{fix: fixtures.NewCodePipelineFixtures()}
}

func (f *CodePipelineFake) ListPipelines(_ context.Context, _ *codepipeline.ListPipelinesInput, _ ...func(*codepipeline.Options)) (*codepipeline.ListPipelinesOutput, error) {
	return &codepipeline.ListPipelinesOutput{Pipelines: f.fix.Pipelines}, nil
}

func (f *CodePipelineFake) GetPipelineState(_ context.Context, input *codepipeline.GetPipelineStateInput, _ ...func(*codepipeline.Options)) (*codepipeline.GetPipelineStateOutput, error) {
	var name string
	if input != nil && input.Name != nil {
		name = *input.Name
	}
	return &codepipeline.GetPipelineStateOutput{
		PipelineName: &name,
		StageStates:  []cptypes.StageState{},
	}, nil
}

// GetPipeline returns an empty pipeline declaration — demo mode does not
// model pipeline stage details.
func (f *CodePipelineFake) GetPipeline(_ context.Context, input *codepipeline.GetPipelineInput, _ ...func(*codepipeline.Options)) (*codepipeline.GetPipelineOutput, error) {
	var name string
	if input != nil && input.Name != nil {
		name = *input.Name
	}
	return &codepipeline.GetPipelineOutput{
		Pipeline: &cptypes.PipelineDeclaration{
			Name: &name,
		},
	}, nil
}
