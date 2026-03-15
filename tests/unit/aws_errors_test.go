package unit

import (
	"fmt"
	"testing"

	awsclient "github.com/k2m30/a9s/internal/aws"

	"github.com/aws/smithy-go"
)

func TestClassifyAWSError_NilError(t *testing.T) {
	code, message, retryable := awsclient.ClassifyAWSError(nil)
	if code != "" {
		t.Errorf("expected empty code for nil error, got %q", code)
	}
	if message != "" {
		t.Errorf("expected empty message for nil error, got %q", message)
	}
	if retryable {
		t.Error("expected retryable=false for nil error")
	}
}

func TestClassifyAWSError_ExpiredToken(t *testing.T) {
	for _, errCode := range []string{"ExpiredToken", "ExpiredTokenException", "RequestExpired"} {
		t.Run(errCode, func(t *testing.T) {
			err := &mockAPIError{code: errCode, message: "token expired", fault: smithy.FaultClient}
			code, message, retryable := awsclient.ClassifyAWSError(err)
			if code != errCode {
				t.Errorf("expected code %q, got %q", errCode, code)
			}
			if message != "token expired" {
				t.Errorf("expected message %q, got %q", "token expired", message)
			}
			if retryable {
				t.Errorf("expected retryable=false for %s", errCode)
			}
		})
	}
}

func TestClassifyAWSError_AccessDenied(t *testing.T) {
	for _, errCode := range []string{"AccessDenied", "AccessDeniedException"} {
		t.Run(errCode, func(t *testing.T) {
			err := &mockAPIError{code: errCode, message: "access denied", fault: smithy.FaultClient}
			code, message, retryable := awsclient.ClassifyAWSError(err)
			if code != errCode {
				t.Errorf("expected code %q, got %q", errCode, code)
			}
			if message != "access denied" {
				t.Errorf("expected message %q, got %q", "access denied", message)
			}
			if retryable {
				t.Errorf("expected retryable=false for %s", errCode)
			}
		})
	}
}

func TestClassifyAWSError_Throttling(t *testing.T) {
	for _, errCode := range []string{"Throttling", "ThrottlingException", "TooManyRequestsException", "RequestLimitExceeded"} {
		t.Run(errCode, func(t *testing.T) {
			err := &mockAPIError{code: errCode, message: "rate exceeded", fault: smithy.FaultClient}
			code, message, retryable := awsclient.ClassifyAWSError(err)
			if code != errCode {
				t.Errorf("expected code %q, got %q", errCode, code)
			}
			if message != "rate exceeded" {
				t.Errorf("expected message %q, got %q", "rate exceeded", message)
			}
			if !retryable {
				t.Errorf("expected retryable=true for %s", errCode)
			}
		})
	}
}

func TestClassifyAWSError_UnknownCode(t *testing.T) {
	err := &mockAPIError{code: "SomeOtherError", message: "something went wrong", fault: smithy.FaultServer}
	code, message, retryable := awsclient.ClassifyAWSError(err)
	if code != "SomeOtherError" {
		t.Errorf("expected code %q, got %q", "SomeOtherError", code)
	}
	if message != "something went wrong" {
		t.Errorf("expected message %q, got %q", "something went wrong", message)
	}
	if retryable {
		t.Error("expected retryable=false for unknown error code")
	}
}

func TestClassifyAWSError_NonAPIError(t *testing.T) {
	err := fmt.Errorf("plain error")
	code, message, retryable := awsclient.ClassifyAWSError(err)
	if code != "Unknown" {
		t.Errorf("expected code %q, got %q", "Unknown", code)
	}
	if message != "plain error" {
		t.Errorf("expected message %q, got %q", "plain error", message)
	}
	if retryable {
		t.Error("expected retryable=false for non-API error")
	}
}
