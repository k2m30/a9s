// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package fakes

import (
	"context"
	"slices"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/codepipeline"
	cptypes "github.com/aws/aws-sdk-go-v2/service/codepipeline/types"

	"github.com/k2m30/a9s/v3/core/demo/fixtures"
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

// GetPipelineState returns the fixture-registered stage states for the
// requested pipeline (see CodePipelineFixtures.States). Required for
// EnrichCodePipelineStatus's Wave-2 failed-stage issue check.
func (f *CodePipelineFake) GetPipelineState(_ context.Context, input *codepipeline.GetPipelineStateInput, _ ...func(*codepipeline.Options)) (*codepipeline.GetPipelineStateOutput, error) {
	var name string
	if input != nil && input.Name != nil {
		name = *input.Name
	}
	return &codepipeline.GetPipelineStateOutput{
		PipelineName: &name,
		StageStates:  f.fix.States[name],
	}, nil
}

// GetPipeline returns the fixture-registered stage/action declaration for
// the requested pipeline, falling back to a bare declaration (no stages)
// when no fixture entry exists.
func (f *CodePipelineFake) GetPipeline(_ context.Context, input *codepipeline.GetPipelineInput, _ ...func(*codepipeline.Options)) (*codepipeline.GetPipelineOutput, error) {
	var name string
	if input != nil && input.Name != nil {
		name = *input.Name
	}
	if !f.hasPipeline(name) {
		return nil, &cptypes.PipelineNotFoundException{
			Message: notFoundMessage("Pipeline", name),
		}
	}
	if decl, ok := f.fix.Declarations[name]; ok {
		return &codepipeline.GetPipelineOutput{Pipeline: decl}, nil
	}
	return &codepipeline.GetPipelineOutput{
		Pipeline: &cptypes.PipelineDeclaration{
			Name: &name,
		},
	}, nil
}

// hasPipeline reports whether the fixtures register this pipeline.
func (f *CodePipelineFake) hasPipeline(name string) bool {
	return slices.ContainsFunc(f.fix.Pipelines, func(p cptypes.PipelineSummary) bool {
		return aws.ToString(p.Name) == name
	})
}
