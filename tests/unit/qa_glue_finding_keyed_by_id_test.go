package unit

// qa_glue_finding_keyed_by_id_test.go — Regression: EnrichGlueJobStatus keys findings by r.ID.
//
// Bug: EnrichGlueJobStatus was writing findings[r.Name] — r.ID was ignored.
// Fix: findings are now keyed by r.ID (falling back to r.Name only when ID is empty).
//
// This test fails if the fix is reverted: findings would be keyed by r.Name
// and the r.ID key would be absent.

import (
	"context"
	"testing"

	gluetypes "github.com/aws/aws-sdk-go-v2/service/glue/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// TestEnrichGlueJobStatus_FindingKeyedByID_NotByName verifies findings are keyed by
// r.ID when r.ID != r.Name. Regresses if the enricher reverts to findings[r.Name].
func TestEnrichGlueJobStatus_FindingKeyedByID_NotByName(t *testing.T) {
	const jobName = "daily-etl-job"
	const jobARN = "arn:aws:glue:us-east-1:123456789012:job/daily-etl-job"

	fake := &glueJobFake{
		jobRuns: map[string]gluetypes.JobRunState{
			jobName: gluetypes.JobRunStateFailed, // FAILED → finding emitted
		},
	}
	clients := &awsclient.ServiceClients{Glue: fake}
	// r.ID is an ARN-like string; r.Name is the human-readable job name.
	resources := []resource.Resource{{ID: jobARN, Name: jobName}}

	result, err := awsclient.EnrichGlueJobStatus(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Finding must be keyed by r.ID (the ARN), not r.Name (the job name).
	if _, ok := result.Findings[jobARN]; !ok {
		t.Errorf("finding must be keyed by r.ID=%q — was the fix to EnrichGlueJobStatus reverted?", jobARN)
	}
	if _, ok := result.Findings[jobName]; ok {
		t.Errorf("finding must NOT be keyed by r.Name=%q — enricher should use r.ID as the key", jobName)
	}
}
