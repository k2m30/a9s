// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// wipfix_qa_codex_aws_test.go pins row 51: the Lambda enricher asks
// GetFunction only to settle a question the shared error code cannot answer,
// and reads a failed answer as "the function is there". A denied or throttled
// GetFunction leaves the row with no finding and no uninspected marker, which
// on screen is a function reported healthy on the strength of a call that
// never succeeded.
package unit_test

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
)

// lambdaVerifierFailsFake answers the policy and URL reads with the shared
// "not found" code and then fails the verification itself.
type lambdaVerifierFailsFake struct {
	awsclient.LambdaAPI
	verifyErr error
}

func (f *lambdaVerifierFailsFake) GetPolicy(
	_ context.Context, _ *lambda.GetPolicyInput, _ ...func(*lambda.Options),
) (*lambda.GetPolicyOutput, error) {
	return nil, lambdaNotFound()
}

func (f *lambdaVerifierFailsFake) ListFunctionUrlConfigs(
	_ context.Context, _ *lambda.ListFunctionUrlConfigsInput, _ ...func(*lambda.Options),
) (*lambda.ListFunctionUrlConfigsOutput, error) {
	return nil, lambdaNotFound()
}

func (f *lambdaVerifierFailsFake) GetFunction(
	_ context.Context, _ *lambda.GetFunctionInput, _ ...func(*lambda.Options),
) (*lambda.GetFunctionOutput, error) {
	return nil, f.verifyErr
}

// TestEnrichLambdaPosture_FailedVerifierMarksTheRowUninspected pins row 51's
// third branch. Only a successful GetFunction says the function is present;
// its own failure says nothing at all, and a row nothing was learned about is
// uninspected, not clean.
func TestEnrichLambdaPosture_FailedVerifierMarksTheRowUninspected(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
	}{
		{"access denied", &smithy.GenericAPIError{Code: "AccessDeniedException", Message: "not authorized to perform lambda:GetFunction"}},
		{"throttled", &smithy.GenericAPIError{Code: "TooManyRequestsException", Message: "Rate exceeded"}},
		{"transient", errors.New("read: connection reset by peer")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fake := &lambdaVerifierFailsFake{verifyErr: tc.err}
			res, err := awsclient.EnrichLambdaPosture(context.Background(),
				&awsclient.ServiceClients{Lambda: fake}, wipfixLambdaRow(), nil)
			_ = err

			if !res.TruncatedIDs["example-fn"] {
				t.Errorf("TruncatedIDs[example-fn] = false after GetFunction failed with %v — "+
					"the verifier never answered, so nothing about this function's posture was "+
					"established and the row must not read as clean", tc.err)
			}
			if got := codesOf(res.Findings["example-fn"]); len(got) != 0 {
				t.Errorf("findings = %v, want none — a failed verification invents nothing either", got)
			}
		})
	}
}

// TestEnrichLambdaPosture_SuccessfulVerifierBranchesUnchanged is the negative
// half, both branches the verifier can actually settle: a function that is
// really gone stays a race, and one that answers stays clean.
func TestEnrichLambdaPosture_SuccessfulVerifierBranchesUnchanged(t *testing.T) {
	gone, err := awsclient.EnrichLambdaPosture(context.Background(),
		&awsclient.ServiceClients{Lambda: &lambdaDeletedFunctionFake{}}, wipfixLambdaRow(), nil)
	if err != nil {
		t.Fatalf("deleted function: %v", err)
	}
	if !gone.TruncatedIDs["example-fn"] {
		t.Error("a function GetFunction confirms is absent is no longer recorded as a race")
	}

	live, err := awsclient.EnrichLambdaPosture(context.Background(),
		&awsclient.ServiceClients{Lambda: &lambdaLivePolicylessFake{}}, wipfixLambdaRow(), nil)
	if err != nil {
		t.Fatalf("live function: %v", err)
	}
	if live.TruncatedIDs["example-fn"] {
		t.Error("a live policy-less function is marked uninspected — that is the healthy answer")
	}
}
