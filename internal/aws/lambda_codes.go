// lambda_codes.go — canonical FindingCode constants for the lambda resource type.
// Phase 03 PR-03b. The fetcher writes Findings using these codes; the
// lambda Color func reads wave1 Findings (Source == "wave1") to color rows.
package aws

import "github.com/k2m30/a9s/v3/internal/domain"

const (
	// CodeLambdaStatePending — function is in the "Pending" lifecycle state.
	// Severity: SevWarn (transitional).
	CodeLambdaStatePending domain.FindingCode = "lambda.state.pending"

	// CodeLambdaStateFailed — function is in the "Failed" lifecycle state.
	// Severity: SevBroken.
	CodeLambdaStateFailed domain.FindingCode = "lambda.state.failed"

)
