package unit

// qa_sfn_uses_arn_from_fields_test.go — Regression: EnrichStepFunctionsStatus must
// call ListExecutions with the state-machine ARN from r.Fields["arn"], NOT the
// bare name in r.ID.
//
// Reported 2026-04-25 from a live profile:
//   [HH:MM:SS] enrich sfn: sfn-enrich: ListExecutions failed for 3 of 3 IDs:
//     example-state-machine: ... InvalidArn: Invalid Arn:
//     'Invalid ARN prefix: example-state-machine'
//
// Root cause (mirrors the tg bug fixed earlier the same day):
//   sfn.go fetcher sets `ID: name` (bare state-machine name) and stores the
//   full ARN in Fields["arn"].
//   sfn_issue_enrichment.go currently does `StateMachineArn: aws.String(r.ID)`
//   — that passes the bare name where AWS requires an ARN.
//
// Contract (post-fix):
//   - The enricher must call ListExecutions with r.Fields["arn"], not r.ID.
//   - A strict fake that rejects non-ARN inputs proves it.

import (
	"context"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sfn"
	smithy "github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// strictSFNFake mirrors AWS: rejects ListExecutions when StateMachineArn is not
// a valid ARN (does not start with "arn:aws:").
type strictSFNFake struct {
	awsclient.SFNAPI
	listCalledWith string
}

func (f *strictSFNFake) ListExecutions(
	_ context.Context,
	input *sfn.ListExecutionsInput,
	_ ...func(*sfn.Options),
) (*sfn.ListExecutionsOutput, error) {
	got := aws.ToString(input.StateMachineArn)
	f.listCalledWith = got
	if !strings.HasPrefix(got, "arn:aws:") {
		return nil, &smithy.GenericAPIError{
			Code:    "InvalidArn",
			Message: "Invalid Arn: 'Invalid ARN prefix: " + got + "'",
		}
	}
	return &sfn.ListExecutionsOutput{}, nil
}

// TestEnrichStepFunctions_UsesARNFromFields verifies the enricher passes
// r.Fields["arn"] (the full state-machine ARN) to ListExecutions, not r.ID
// (the bare name set by the sfn fetcher).
func TestEnrichStepFunctions_UsesARNFromFields(t *testing.T) {
	const smName = "example-state-machine"
	const smARN = "arn:aws:states:us-east-1:123456789012:stateMachine:example-state-machine"

	fake := &strictSFNFake{}
	clients := &awsclient.ServiceClients{SFN: fake}
	// Mirrors what sfn.go fetcher emits: ID/Name = bare name, ARN in Fields["arn"].
	resources := []resource.Resource{{
		ID:     smName,
		Name:   smName,
		Fields: map[string]string{"arn": smARN},
	}}

	_, err := awsclient.EnrichStepFunctionsStatus(context.Background(), clients, resources, nil)
	if err != nil && strings.Contains(err.Error(), "InvalidArn") {
		t.Fatalf("enricher passed the bare name to AWS instead of the ARN; got: %v", err)
	}
	if fake.listCalledWith != smARN {
		t.Errorf("ListExecutions was called with %q, want %q (the ARN from Fields[\"arn\"])",
			fake.listCalledWith, smARN)
	}
}
