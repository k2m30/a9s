package unit

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/glue"
	gluetypes "github.com/aws/aws-sdk-go-v2/service/glue/types"

	awsclient "github.com/k2m30/a9s/internal/aws"
)

// ---------------------------------------------------------------------------
// T-GLUE-001 - Test Glue Jobs response parsing
// ---------------------------------------------------------------------------

func TestFetchGlueJobs_ParsesMultipleJobs(t *testing.T) {
	now := time.Now()
	mock := &mockGlueClient{
		output: &glue.GetJobsOutput{
			Jobs: []gluetypes.Job{
				{
					Name:            aws.String("etl-job-1"),
					Role:            aws.String("arn:aws:iam::123456789012:role/GlueRole"),
					GlueVersion:     aws.String("4.0"),
					WorkerType:      gluetypes.WorkerTypeG2x,
					NumberOfWorkers: aws.Int32(10),
					MaxRetries:      3,
					CreatedOn:       &now,
					LastModifiedOn:  &now,
					Command: &gluetypes.JobCommand{
						Name: aws.String("glueetl"),
					},
				},
				{
					Name:            aws.String("etl-job-2"),
					Role:            aws.String("arn:aws:iam::123456789012:role/GlueRole2"),
					GlueVersion:     aws.String("3.0"),
					WorkerType:      gluetypes.WorkerTypeStandard,
					NumberOfWorkers: aws.Int32(5),
				},
			},
		},
	}

	resources, err := awsclient.FetchGlueJobs(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}

	r := resources[0]
	if r.Name != "etl-job-1" {
		t.Errorf("expected Name 'etl-job-1', got %q", r.Name)
	}
	if r.ID != "etl-job-1" {
		t.Errorf("expected ID 'etl-job-1', got %q", r.ID)
	}
	if r.Fields["job_name"] != "etl-job-1" {
		t.Errorf("expected Fields[job_name] 'etl-job-1', got %q", r.Fields["job_name"])
	}
	if r.Fields["glue_version"] != "4.0" {
		t.Errorf("expected Fields[glue_version] '4.0', got %q", r.Fields["glue_version"])
	}
	if r.Fields["worker_type"] != "G.2X" {
		t.Errorf("expected Fields[worker_type] 'G.2X', got %q", r.Fields["worker_type"])
	}
	if r.Fields["num_workers"] != "10" {
		t.Errorf("expected Fields[num_workers] '10', got %q", r.Fields["num_workers"])
	}

	if r.RawStruct == nil {
		t.Error("expected RawStruct to be set")
	}
}

func TestFetchGlueJobs_EmptyResponse(t *testing.T) {
	mock := &mockGlueClient{
		output: &glue.GetJobsOutput{
			Jobs: []gluetypes.Job{},
		},
	}

	resources, err := awsclient.FetchGlueJobs(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 0 {
		t.Fatalf("expected 0 resources, got %d", len(resources))
	}
}

func TestFetchGlueJobs_APIError(t *testing.T) {
	mock := &mockGlueClient{
		err: &mockAPIError{code: "AccessDeniedException", message: "access denied"},
	}

	_, err := awsclient.FetchGlueJobs(context.Background(), mock)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestFetchGlueJobs_NilFields(t *testing.T) {
	mock := &mockGlueClient{
		output: &glue.GetJobsOutput{
			Jobs: []gluetypes.Job{
				{},
			},
		},
	}

	resources, err := awsclient.FetchGlueJobs(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	r := resources[0]
	if r.Name != "" {
		t.Errorf("expected empty Name, got %q", r.Name)
	}
}
