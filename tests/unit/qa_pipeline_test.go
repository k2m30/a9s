package unit

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/codepipeline"
	cptypes "github.com/aws/aws-sdk-go-v2/service/codepipeline/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
)

// ---------------------------------------------------------------------------
// CodePipeline fetcher tests
// ---------------------------------------------------------------------------

func TestFetchCodePipelines_ParsesMultiple(t *testing.T) {
	now := time.Now()
	mock := &mockCodePipelineClient{
		output: &codepipeline.ListPipelinesOutput{
			Pipelines: []cptypes.PipelineSummary{
				{
					Name:          aws.String("deploy-prod"),
					PipelineType:  cptypes.PipelineTypeV2,
					Created:       &now,
					Updated:       &now,
					Version:       aws.Int32(3),
					ExecutionMode: cptypes.ExecutionModeSuperseded,
				},
				{
					Name:          aws.String("build-staging"),
					PipelineType:  cptypes.PipelineTypeV1,
					Created:       &now,
					Updated:       &now,
					Version:       aws.Int32(1),
					ExecutionMode: cptypes.ExecutionModeQueued,
				},
			},
		},
	}

	resources, err := awsclient.FetchCodePipelines(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}

	// Verify first pipeline
	r0 := resources[0]
	if r0.ID != "deploy-prod" {
		t.Errorf("resource[0].ID: expected %q, got %q", "deploy-prod", r0.ID)
	}
	if r0.Name != "deploy-prod" {
		t.Errorf("resource[0].Name: expected %q, got %q", "deploy-prod", r0.Name)
	}

	// Verify required fields
	requiredFields := []string{"name", "pipeline_type", "created", "updated", "version"}
	for i, r := range resources {
		for _, key := range requiredFields {
			if _, ok := r.Fields[key]; !ok {
				t.Errorf("resource[%d].Fields missing key %q", i, key)
			}
		}
	}

	if r0.Fields["pipeline_type"] != "V2" {
		t.Errorf("resource[0].Fields[\"pipeline_type\"]: expected %q, got %q", "V2", r0.Fields["pipeline_type"])
	}
	if r0.Fields["version"] != "3" {
		t.Errorf("resource[0].Fields[\"version\"]: expected %q, got %q", "3", r0.Fields["version"])
	}

	// Verify second pipeline
	r1 := resources[1]
	if r1.Fields["pipeline_type"] != "V1" {
		t.Errorf("resource[1].Fields[\"pipeline_type\"]: expected %q, got %q", "V1", r1.Fields["pipeline_type"])
	}
}

func TestFetchCodePipelines_RawStructPopulated(t *testing.T) {
	now := time.Now()
	mock := &mockCodePipelineClient{
		output: &codepipeline.ListPipelinesOutput{
			Pipelines: []cptypes.PipelineSummary{
				{
					Name:         aws.String("raw-pipeline"),
					PipelineType: cptypes.PipelineTypeV2,
					Created:      &now,
					Version:      aws.Int32(1),
				},
			},
		},
	}

	resources, err := awsclient.FetchCodePipelines(context.Background(), mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	r := resources[0]
	if r.RawStruct == nil {
		t.Fatal("RawStruct must not be nil")
	}
	pl, ok := r.RawStruct.(cptypes.PipelineSummary)
	if !ok {
		t.Fatalf("RawStruct should be cptypes.PipelineSummary, got %T", r.RawStruct)
	}
	if pl.Name == nil || *pl.Name != "raw-pipeline" {
		t.Errorf("RawStruct.Name: expected %q", "raw-pipeline")
	}
}

func TestFetchCodePipelines_ErrorResponse(t *testing.T) {
	mock := &mockCodePipelineClient{
		err: fmt.Errorf("AWS API error: access denied"),
	}

	resources, err := awsclient.FetchCodePipelines(context.Background(), mock)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if resources != nil {
		t.Errorf("expected nil resources on error, got %d", len(resources))
	}
}

func TestFetchCodePipelines_EmptyResponse(t *testing.T) {
	mock := &mockCodePipelineClient{
		output: &codepipeline.ListPipelinesOutput{
			Pipelines: []cptypes.PipelineSummary{},
		},
	}

	resources, err := awsclient.FetchCodePipelines(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(resources))
	}
}
