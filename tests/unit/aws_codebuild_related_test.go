// aws_codebuild_related_test.go contains unit tests for CodeBuild related-resource checkers.
package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cbtypes "github.com/aws/aws-sdk-go-v2/service/codebuild/types"
	cptypes "github.com/aws/aws-sdk-go-v2/service/codepipeline/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
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

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != pipelineName {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, pipelineName)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

// TestRelated_Cb_Pipeline_Match_Truncated verifies that when the pipeline cache
// is truncated, a match still sets Count=1 and Approximate=true.
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

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if !result.Approximate {
		t.Error("Approximate = false, want true (cache is truncated)")
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

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no pipeline references this project)", result.Count)
	}
}

// TestRelated_Cb_Pipeline_MissingCache verifies that when the pipeline key is
// absent from the cache, Count=0 (not -1) is returned.
func TestRelated_Cb_Pipeline_MissingCache(t *testing.T) {
	checker := cbCheckerByTarget(t, "pipeline")
	result := checker(context.Background(), nil, cbSourceResource("my-project"), resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (cache miss should return 0, not -1)", result.Count)
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

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (wrong RawStruct type)", result.Count)
	}
}
