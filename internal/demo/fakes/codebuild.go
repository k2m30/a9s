package fakes

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/codebuild"
	cbtypes "github.com/aws/aws-sdk-go-v2/service/codebuild/types"

	"github.com/k2m30/a9s/v3/internal/demo/fixtures"
)

// CodeBuildFake implements aws.CodeBuildAPI against fixture data loaded at construction time.
type CodeBuildFake struct {
	fix *fixtures.CodeBuildFixtures
}

// NewCodeBuild constructs a CodeBuildFake backed by fixture data from the fixtures package.
func NewCodeBuild() *CodeBuildFake {
	return &CodeBuildFake{fix: fixtures.NewCodeBuildFixtures()}
}

func (f *CodeBuildFake) ListProjects(_ context.Context, _ *codebuild.ListProjectsInput, _ ...func(*codebuild.Options)) (*codebuild.ListProjectsOutput, error) {
	names := make([]string, 0, len(f.fix.Projects))
	for _, p := range f.fix.Projects {
		if p.Name != nil {
			names = append(names, *p.Name)
		}
	}
	return &codebuild.ListProjectsOutput{Projects: names}, nil
}

func (f *CodeBuildFake) BatchGetProjects(_ context.Context, _ *codebuild.BatchGetProjectsInput, _ ...func(*codebuild.Options)) (*codebuild.BatchGetProjectsOutput, error) {
	return &codebuild.BatchGetProjectsOutput{Projects: f.fix.Projects}, nil
}

func (f *CodeBuildFake) ListBuildsForProject(_ context.Context, input *codebuild.ListBuildsForProjectInput, _ ...func(*codebuild.Options)) (*codebuild.ListBuildsForProjectOutput, error) {
	var projectName string
	if input != nil && input.ProjectName != nil {
		projectName = *input.ProjectName
	}
	builds := f.fix.Builds[projectName]
	ids := make([]string, 0, len(builds))
	for _, b := range builds {
		if b.Id != nil {
			ids = append(ids, *b.Id)
		}
	}
	return &codebuild.ListBuildsForProjectOutput{Ids: ids}, nil
}

func (f *CodeBuildFake) BatchGetBuilds(_ context.Context, input *codebuild.BatchGetBuildsInput, _ ...func(*codebuild.Options)) (*codebuild.BatchGetBuildsOutput, error) {
	idSet := make(map[string]bool, len(input.Ids))
	for _, id := range input.Ids {
		idSet[id] = true
	}
	var result []cbtypes.Build
	for _, builds := range f.fix.Builds {
		for _, b := range builds {
			if b.Id != nil && idSet[*b.Id] {
				result = append(result, b)
			}
		}
	}
	return &codebuild.BatchGetBuildsOutput{Builds: result}, nil
}
