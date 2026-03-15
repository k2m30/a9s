package aws

import (
	"errors"

	"github.com/aws/smithy-go"
)

// ClassifyAWSError inspects an error for a smithy.APIError and returns
// the error code, message, and whether the operation is retryable.
func ClassifyAWSError(err error) (code string, message string, retryable bool) {
	if err == nil {
		return "", "", false
	}

	var apiErr smithy.APIError
	if !errors.As(err, &apiErr) {
		return "Unknown", err.Error(), false
	}

	code = apiErr.ErrorCode()
	message = apiErr.ErrorMessage()

	switch code {
	case "ExpiredToken", "ExpiredTokenException", "RequestExpired":
		retryable = false
	case "AccessDenied", "AccessDeniedException":
		retryable = false
	case "Throttling", "ThrottlingException", "TooManyRequestsException", "RequestLimitExceeded":
		retryable = true
	default:
		retryable = false
	}

	return code, message, retryable
}
