package unit

// qa_sfn_uses_arn_from_fields_test.go — EnrichStepFunctionsStatus must call
// ListExecutions with the state-machine ARN from r.Fields["arn"], NOT the
// bare name in r.ID.
//
// The sfn.go fetcher sets `ID: name` (bare state-machine name) and stores the
// full ARN in Fields["arn"]; ListExecutions rejects the bare name with
// InvalidArn ("Invalid ARN prefix: <name>").
//
// Contract:
//   - The enricher must call ListExecutions with r.Fields["arn"], not r.ID.
//   - A strict fake that rejects non-ARN inputs proves it.

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sfn"
	smithy "github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
)

// strictSFNFake mirrors AWS: rejects ListExecutions when StateMachineArn is not
// a valid ARN (does not start with "arn:aws:"). mu guards listCalledWith,
// which is written concurrently: EnrichStepFunctionsStatus fans out
// ListExecutions calls per resource via core/aws.ForEachParallel
// (EnrichmentParallelism goroutines).
type strictSFNFake struct {
	awsclient.SFNAPI
	mu                 sync.Mutex
	listCalledWith     string
	describeCalledWith string
}

func (f *strictSFNFake) ListExecutions(
	_ context.Context,
	input *sfn.ListExecutionsInput,
	_ ...func(*sfn.Options),
) (*sfn.ListExecutionsOutput, error) {
	got := aws.ToString(input.StateMachineArn)
	f.mu.Lock()
	f.listCalledWith = got
	f.mu.Unlock()
	if !strings.HasPrefix(got, "arn:aws:") {
		return nil, &smithy.GenericAPIError{
			Code:    "InvalidArn",
			Message: "Invalid Arn: 'Invalid ARN prefix: " + got + "'",
		}
	}
	return &sfn.ListExecutionsOutput{}, nil
}

// DescribeStateMachine rejects a bare name exactly as ListExecutions does.
// This fake exists to catch the enricher handing AWS r.ID instead of the ARN
// in Fields["arn"]; the enricher now makes a second ARN-taking call, and a
// permissive stub would leave that one uncovered.
func (f *strictSFNFake) DescribeStateMachine(
	_ context.Context,
	input *sfn.DescribeStateMachineInput,
	_ ...func(*sfn.Options),
) (*sfn.DescribeStateMachineOutput, error) {
	got := aws.ToString(input.StateMachineArn)
	f.mu.Lock()
	f.describeCalledWith = got
	f.mu.Unlock()
	if !strings.HasPrefix(got, "arn:aws:") {
		return nil, &smithy.GenericAPIError{
			Code:    "InvalidArn",
			Message: "Invalid Arn: 'Invalid ARN prefix: " + got + "'",
		}
	}
	return &sfn.DescribeStateMachineOutput{StateMachineArn: input.StateMachineArn}, nil
}

// describeCalledWithSafe returns describeCalledWith, safe for concurrent use.
func (f *strictSFNFake) describeCalledWithSafe() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.describeCalledWith
}

// listCalledWithSafe returns listCalledWith, safe for concurrent use with
// ListExecutions.
func (f *strictSFNFake) listCalledWithSafe() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.listCalledWith
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
	if got := fake.listCalledWithSafe(); got != smARN {
		t.Errorf("ListExecutions was called with %q, want %q (the ARN from Fields[\"arn\"])",
			got, smARN)
	}
	if got := fake.describeCalledWithSafe(); got != smARN {
		t.Errorf("DescribeStateMachine was called with %q, want %q (the ARN from Fields[\"arn\"])",
			got, smARN)
	}
}
