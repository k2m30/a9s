// fakes_codebuild_test.go is the single source for CodeBuild SDK-interface
// fakes shared across tests/unit.
//
// Convention (docs/go-codebase-checklist.md, DRY section): one configurable
// fake per SDK interface, in a service-named fakes_<service>_test.go file --
// never re-implement the same interface under a new name in another file,
// and never add another wave/batch-named fake file (fakes_us1_batchN_test.go,
// fakes_related_checkers_branch_coverage_test.go, fakes_related_checkers_misc_test.go are historical
// accretion naming, not a pattern to extend).
package unit

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/codebuild"
	codebuildtypes "github.com/aws/aws-sdk-go-v2/service/codebuild/types"
)

// fakeCodeBuildListProjects implements awsclient.CodeBuildListProjectsAPI.
//
// Configure at most one response mode:
//   - Err: always return this error
//   - Output: always return this single output (no pagination)
//   - Pages: sequential paged outputs, one per call, in order
//   - PageFunc: full control, receives the 1-based call number
//
// Calls, Inputs and LastInput are always recorded.
type fakeCodeBuildListProjects struct {
	Err      error
	Output   *codebuild.ListProjectsOutput
	Pages    []*codebuild.ListProjectsOutput
	PageFunc func(call int) (*codebuild.ListProjectsOutput, error)

	Calls     int
	Inputs    []*codebuild.ListProjectsInput
	LastInput *codebuild.ListProjectsInput
}

func (f *fakeCodeBuildListProjects) ListProjects(
	_ context.Context,
	params *codebuild.ListProjectsInput,
	_ ...func(*codebuild.Options),
) (*codebuild.ListProjectsOutput, error) {
	f.Inputs = append(f.Inputs, params)
	f.LastInput = params

	if f.PageFunc != nil {
		f.Calls++
		return f.PageFunc(f.Calls)
	}
	if f.Err != nil {
		return nil, f.Err
	}
	if len(f.Pages) > 0 {
		idx := f.Calls
		if idx >= len(f.Pages) {
			return &codebuild.ListProjectsOutput{}, nil
		}
		f.Calls++
		return f.Pages[idx], nil
	}
	if f.Output != nil {
		return f.Output, nil
	}
	return &codebuild.ListProjectsOutput{}, nil
}

// fakeCodeBuildBatchGetProjects implements awsclient.CodeBuildBatchGetProjectsAPI.
//
// Configure at most one response mode:
//   - Err: always return this error
//   - Output: return this output unfiltered
//   - Output + FilterByName: return Output.Projects filtered down to only
//     those whose Name is present in the request's Names — mirrors the real
//     API, which only returns the projects actually requested
//   - PageFunc: full control, receives the 1-based call number
type fakeCodeBuildBatchGetProjects struct {
	Err          error
	Output       *codebuild.BatchGetProjectsOutput
	FilterByName bool
	PageFunc     func(call int) (*codebuild.BatchGetProjectsOutput, error)

	Calls int
}

func (f *fakeCodeBuildBatchGetProjects) BatchGetProjects(
	_ context.Context,
	params *codebuild.BatchGetProjectsInput,
	_ ...func(*codebuild.Options),
) (*codebuild.BatchGetProjectsOutput, error) {
	if f.PageFunc != nil {
		f.Calls++
		return f.PageFunc(f.Calls)
	}
	if f.Err != nil {
		return nil, f.Err
	}
	if f.FilterByName {
		nameSet := make(map[string]bool)
		for _, n := range params.Names {
			nameSet[n] = true
		}
		var filtered []codebuildtypes.Project
		if f.Output != nil {
			for _, p := range f.Output.Projects {
				if p.Name != nil && nameSet[*p.Name] {
					filtered = append(filtered, p)
				}
			}
		}
		return &codebuild.BatchGetProjectsOutput{Projects: filtered}, nil
	}
	if f.Output != nil {
		return f.Output, nil
	}
	return &codebuild.BatchGetProjectsOutput{}, nil
}
