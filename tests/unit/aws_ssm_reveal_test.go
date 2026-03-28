package unit

// aws_ssm_reveal_test.go tests RevealSSMParameter() that will be added to
// internal/aws/ssm.go as part of issue #104.
// These tests will FAIL until the coder adds RevealSSMParameter and the
// SSMGetParameterAPI interface to internal/aws/interfaces.go.

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
)

// ---------------------------------------------------------------------------
// TestRevealSSMParameter_ReturnsValue
// ---------------------------------------------------------------------------

// TestRevealSSMParameter_ReturnsValue verifies that RevealSSMParameter returns
// the Value field from the SSM GetParameter response.
func TestRevealSSMParameter_ReturnsValue(t *testing.T) {
	mock := &mockSSMGetParameterClient{
		output: &ssm.GetParameterOutput{
			Parameter: &ssmtypes.Parameter{
				Value: aws.String("my-secret-value"),
			},
		},
	}

	got, err := awsclient.RevealSSMParameter(context.Background(), mock, "/my/param")
	if err != nil {
		t.Fatalf("RevealSSMParameter returned unexpected error: %v", err)
	}
	if got != "my-secret-value" {
		t.Errorf("RevealSSMParameter returned %q, want %q", got, "my-secret-value")
	}
}

// ---------------------------------------------------------------------------
// TestRevealSSMParameter_WithDecryption
// ---------------------------------------------------------------------------

// TestRevealSSMParameter_WithDecryption verifies that the mock receives
// WithDecryption=true in the GetParameter request, ensuring SecureString
// parameter values are decrypted automatically.
func TestRevealSSMParameter_WithDecryption(t *testing.T) {
	mock := &mockSSMGetParameterClient{
		output: &ssm.GetParameterOutput{
			Parameter: &ssmtypes.Parameter{
				Value: aws.String("decrypted-value"),
			},
		},
	}

	_, err := awsclient.RevealSSMParameter(context.Background(), mock, "/secure/param")
	if err != nil {
		t.Fatalf("RevealSSMParameter returned unexpected error: %v", err)
	}

	if mock.capturedInput == nil {
		t.Fatal("mock did not capture any input — GetParameter was not called")
	}
	if mock.capturedInput.WithDecryption == nil || !*mock.capturedInput.WithDecryption {
		t.Error("RevealSSMParameter must set WithDecryption=true in the GetParameter request")
	}
}

// ---------------------------------------------------------------------------
// TestRevealSSMParameter_ErrorResponse
// ---------------------------------------------------------------------------

// TestRevealSSMParameter_ErrorResponse verifies that an API error from
// GetParameter is wrapped and returned as a non-nil error.
func TestRevealSSMParameter_ErrorResponse(t *testing.T) {
	apiErr := errors.New("ParameterNotFound: /missing/param")
	mock := &mockSSMGetParameterClient{
		err: apiErr,
	}

	_, err := awsclient.RevealSSMParameter(context.Background(), mock, "/missing/param")
	if err == nil {
		t.Fatal("RevealSSMParameter should return an error when the API fails, got nil")
	}
	// Error should wrap the original error.
	if !errors.Is(err, apiErr) {
		t.Errorf("expected error to wrap %v, got: %v", apiErr, err)
	}
}

// ---------------------------------------------------------------------------
// TestRevealSSMParameter_NilValue
// ---------------------------------------------------------------------------

// TestRevealSSMParameter_NilValue verifies that when the Parameter.Value is
// nil (e.g., StringList type), RevealSSMParameter returns an empty string
// without error.
func TestRevealSSMParameter_NilValue(t *testing.T) {
	mock := &mockSSMGetParameterClient{
		output: &ssm.GetParameterOutput{
			Parameter: &ssmtypes.Parameter{
				Name:  aws.String("/my/param"),
				Value: nil, // explicitly nil
			},
		},
	}

	got, err := awsclient.RevealSSMParameter(context.Background(), mock, "/my/param")
	if err != nil {
		t.Fatalf("RevealSSMParameter returned unexpected error: %v", err)
	}
	if got != "" {
		t.Errorf("RevealSSMParameter should return empty string for nil Value, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// TestRevealSSMParameter_NilParameter
// ---------------------------------------------------------------------------

// TestRevealSSMParameter_NilParameter verifies that when the output.Parameter
// field itself is nil, RevealSSMParameter returns an empty string without
// panicking or returning an error.
func TestRevealSSMParameter_NilParameter(t *testing.T) {
	mock := &mockSSMGetParameterClient{
		output: &ssm.GetParameterOutput{
			Parameter: nil, // no parameter in the response
		},
	}

	got, err := awsclient.RevealSSMParameter(context.Background(), mock, "/my/param")
	if err != nil {
		t.Fatalf("RevealSSMParameter returned unexpected error: %v", err)
	}
	if got != "" {
		t.Errorf("RevealSSMParameter should return empty string for nil Parameter, got %q", got)
	}
}
