package unit

// "SlowDown" is an S3 (and DynamoDB) throttle error code, classed like
// Throttling / ThrottlingException / TooManyRequestsException /
// RequestLimitExceeded.

import (
	"testing"

	"github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
)

func TestClassifyAWSError_SlowDown(t *testing.T) {
	err := &MockAPIError{Code: "SlowDown", Message: "Please reduce your request rate.", Fault: smithy.FaultClient}
	code, message := awsclient.ClassifyAWSError(err)
	if code != "SlowDown" {
		t.Errorf("expected code %q, got %q", "SlowDown", code)
	}
	if message != "Please reduce your request rate." {
		t.Errorf("expected message %q, got %q", "Please reduce your request rate.", message)
	}
}
