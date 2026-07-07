package aws

import "github.com/k2m30/a9s/v3/internal/domain"

// Lambda invocation-status finding emitted by FetchLambdaInvocations. The
// REPORT line's "Status: timeout" marker is the only structured failure
// signal available (see timeoutRegex) — a TIMEOUT invocation classifies as
// broken; OK emits no finding.
const (
	CodeLambdaInvocationTimeout domain.FindingCode = "lambda-invocation.broken.timeout"
)
