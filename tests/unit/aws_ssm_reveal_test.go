package unit

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
)

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

func TestRevealSSMParameter_ErrorResponse(t *testing.T) {
	apiErr := errors.New("ParameterNotFound: /missing/param")
	mock := &mockSSMGetParameterClient{
		err: apiErr,
	}

	_, err := awsclient.RevealSSMParameter(context.Background(), mock, "/missing/param")
	if err == nil {
		t.Fatal("RevealSSMParameter should return an error when the API fails, got nil")
	}
	if !errors.Is(err, apiErr) {
		t.Errorf("expected error to wrap %v, got: %v", apiErr, err)
	}
}

func TestRevealSSMParameter_NilValue(t *testing.T) {
	mock := &mockSSMGetParameterClient{
		output: &ssm.GetParameterOutput{
			Parameter: &ssmtypes.Parameter{
				Name:  aws.String("/my/param"),
				Value: nil,
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

func TestRevealSSMParameter_NilParameter(t *testing.T) {
	mock := &mockSSMGetParameterClient{
		output: &ssm.GetParameterOutput{
			Parameter: nil,
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
