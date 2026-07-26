// aws_codebuild_related_test.go contains unit tests for CodeBuild related-resource checkers.
package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cbtypes "github.com/aws/aws-sdk-go-v2/service/codebuild/types"
	cptypes "github.com/aws/aws-sdk-go-v2/service/codepipeline/types"

	_ "github.com/k2m30/a9s/v3/core/aws"
	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// cbSourceResource builds a CodeBuild Project resource used as the parent.
func cbSourceResource(projectName string) resource.Resource {
	return resource.Resource{
		ID:   projectName,
		Name: projectName,
		Fields: map[string]string{
			"project_name": projectName,
		},
		RawStruct: cbtypes.Project{
			Name: aws.String(projectName),
		},
	}
}

// cbPipelineResource builds a pipeline resource for the cache.
func cbPipelineResource(name string) resource.Resource {
	return resource.Resource{
		ID:   name,
		Name: name,
	}
}

// --- cb→pipeline: reverse-scan via cache["pipeline"] + GetPipeline per pipeline ---

// TestRelated_Cb_Pipeline_Match verifies that a pipeline whose GetPipeline
// response contains a CodeBuild action with ProjectName matching the parent
// project is returned with Count=1.
func TestRelated_Cb_Pipeline_Match(t *testing.T) {
	const projectName = "my-build-project"
	const pipelineName = "my-ci-pipeline"

	fakeCp := newFakeCodePipelineWithDeclarations(map[string]*cptypes.PipelineDeclaration{
		pipelineName: pipelineDeclarationWithCodeBuildAction(pipelineName, projectName),
	})
	clients := &awsclient.ServiceClients{CodePipeline: fakeCp}

	cache := resource.ResourceCache{
		"pipeline": resource.ResourceCacheEntry{
			Resources: []resource.Resource{cbPipelineResource(pipelineName)},
		},
	}

	checker := cbCheckerByTarget(t, "pipeline")
	result := checker(context.Background(), clients, cbSourceResource(projectName), cache)

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != pipelineName {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs(), pipelineName)
	}
	if result.Err() != nil {
		t.Errorf("unexpected error: %v", result.Err())
	}
}

// TestRelated_Cb_Pipeline_Match_Truncated verifies that when the pipeline cache
// is truncated, a match still sets Count=1 and Truncated=true.
func TestRelated_Cb_Pipeline_Match_Truncated(t *testing.T) {
	const projectName = "my-build-project"
	const pipelineName = "my-ci-pipeline"

	fakeCp := newFakeCodePipelineWithDeclarations(map[string]*cptypes.PipelineDeclaration{
		pipelineName: pipelineDeclarationWithCodeBuildAction(pipelineName, projectName),
	})
	clients := &awsclient.ServiceClients{CodePipeline: fakeCp}

	cache := resource.ResourceCache{
		"pipeline": resource.ResourceCacheEntry{
			Resources:   []resource.Resource{cbPipelineResource(pipelineName)},
			IsTruncated: true,
		},
	}

	checker := cbCheckerByTarget(t, "pipeline")
	result := checker(context.Background(), clients, cbSourceResource(projectName), cache)

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if !result.Truncated() {
		t.Error("Truncated = false, want true (cache is truncated)")
	}
}

// TestRelated_Cb_Pipeline_Empty verifies that a pipeline cache with no matching
// CodeBuild action returns Count=0.
func TestRelated_Cb_Pipeline_Empty(t *testing.T) {
	const projectName = "my-build-project"
	const pipelineName = "unrelated-pipeline"

	fakeCp := newFakeCodePipelineWithDeclarations(map[string]*cptypes.PipelineDeclaration{
		pipelineName: pipelineDeclarationEmpty(pipelineName),
	})
	clients := &awsclient.ServiceClients{CodePipeline: fakeCp}

	cache := resource.ResourceCache{
		"pipeline": resource.ResourceCacheEntry{
			Resources: []resource.Resource{cbPipelineResource(pipelineName)},
		},
	}

	checker := cbCheckerByTarget(t, "pipeline")
	result := checker(context.Background(), clients, cbSourceResource(projectName), cache)

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (no pipeline references this project)", result.Count())
	}
}

// TestRelated_Cb_Pipeline_WrongRawStruct verifies that a wrong parent RawStruct
// type returns Count=-1 (assertStruct guard).
func TestRelated_Cb_Pipeline_WrongRawStruct(t *testing.T) {
	source := resource.Resource{
		ID:        "my-project",
		RawStruct: "not-a-codebuild-project",
	}
	checker := cbCheckerByTarget(t, "pipeline")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (wrong RawStruct type)", result.Count())
	}
}

// TestRelated_Cb_Pipeline_PartialFailure_RendersInflatedTruncated verifies
// that when some (not all) GetPipeline calls in the reverse-scan loop fail,
// the survivors found so far are not rendered as an exact count. "2 matches,
// 2 failures" must render "(2+)", never a confident "(2)" — the failed
// lookups mean more matches may exist among the pipelines that could not be
// checked.
func TestRelated_Cb_Pipeline_PartialFailure_RendersInflatedTruncated(t *testing.T) {
	const projectName = "my-build-project"

	fakeCp := newFakeCodePipelineWithDeclarations(map[string]*cptypes.PipelineDeclaration{
		"matching-pipeline-1": pipelineDeclarationWithCodeBuildAction("matching-pipeline-1", projectName),
		"matching-pipeline-2": pipelineDeclarationWithCodeBuildAction("matching-pipeline-2", projectName),
		// "failing-pipeline-1"/"-2" deliberately absent: GetPipeline for a
		// name missing from declarationsByName returns an empty output (no
		// Pipeline field), which pipelineGetDeclaration now reports as an
		// error instead of a silent nil.
	})
	clients := &awsclient.ServiceClients{CodePipeline: fakeCp}

	cache := resource.ResourceCache{
		"pipeline": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				cbPipelineResource("matching-pipeline-1"),
				cbPipelineResource("matching-pipeline-2"),
				cbPipelineResource("failing-pipeline-1"),
				cbPipelineResource("failing-pipeline-2"),
			},
		},
	}

	checker := cbCheckerByTarget(t, "pipeline")
	result := checker(context.Background(), clients, cbSourceResource(projectName), cache)

	if result.Count() != 2 {
		t.Fatalf("Count = %d, want 2 (the two survivors)", result.Count())
	}
	if !result.Truncated() {
		t.Fatal("Truncated = false, want true (2 of 4 lookups failed)")
	}
	display := resource.FormatRelatedCount(result.State(), result.Count(), result.Truncated())
	if display != "(2+)" {
		t.Errorf("rendered display = %q, want %q — a partial-failure count must never look exact", display, "(2+)")
	}
}

// TestRelated_Cb_Pipeline_AllFail_UnknownRelated verifies that when every
// GetPipeline call in the reverse-scan loop fails, the result is
// UnknownRelated — not a confident, silent "(0)".
func TestRelated_Cb_Pipeline_AllFail_UnknownRelated(t *testing.T) {
	const projectName = "my-build-project"

	clients := &awsclient.ServiceClients{
		CodePipeline: newFakeCodePipelineWithDeclarations(nil), // every lookup misses -> fails
	}

	cache := resource.ResourceCache{
		"pipeline": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				cbPipelineResource("failing-pipeline-1"),
				cbPipelineResource("failing-pipeline-2"),
			},
		},
	}

	checker := cbCheckerByTarget(t, "pipeline")
	result := checker(context.Background(), clients, cbSourceResource(projectName), cache)

	if result.State() != domain.RelatedUnknown {
		t.Fatalf("State = %v, want RelatedUnknown (every lookup failed)", result.State())
	}
	display := resource.FormatRelatedCount(result.State(), result.Count(), result.Truncated())
	if display != "" {
		t.Errorf("rendered display = %q, want %q — an all-failed scan must never render a confident zero", display, "")
	}
}

// TestRelated_Cb_Pipeline_AllSucceed_ExactCountSurvives is the positive
// control: when every GetPipeline call in the loop succeeds, the exact count
// must survive untouched — guarding against an over-broad fix that marks
// every scan unknown/truncated regardless of whether any lookup failed.
func TestRelated_Cb_Pipeline_AllSucceed_ExactCountSurvives(t *testing.T) {
	const projectName = "my-build-project"

	fakeCp := newFakeCodePipelineWithDeclarations(map[string]*cptypes.PipelineDeclaration{
		"matching-pipeline-1": pipelineDeclarationWithCodeBuildAction("matching-pipeline-1", projectName),
		"matching-pipeline-2": pipelineDeclarationWithCodeBuildAction("matching-pipeline-2", projectName),
		"unrelated-pipeline":  pipelineDeclarationEmpty("unrelated-pipeline"),
	})
	clients := &awsclient.ServiceClients{CodePipeline: fakeCp}

	cache := resource.ResourceCache{
		"pipeline": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				cbPipelineResource("matching-pipeline-1"),
				cbPipelineResource("matching-pipeline-2"),
				cbPipelineResource("unrelated-pipeline"),
			},
		},
	}

	checker := cbCheckerByTarget(t, "pipeline")
	result := checker(context.Background(), clients, cbSourceResource(projectName), cache)

	if result.Count() != 2 {
		t.Fatalf("Count = %d, want 2 (all lookups succeeded, no failures to inflate)", result.Count())
	}
	if result.Truncated() {
		t.Fatal("Truncated = true, want false (nothing failed)")
	}
	display := resource.FormatRelatedCount(result.State(), result.Count(), result.Truncated())
	if display != "(2)" {
		t.Errorf("rendered display = %q, want %q", display, "(2)")
	}
}
