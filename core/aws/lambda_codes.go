// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// lambda_codes.go — canonical FindingCode constants for the lambda resource type.
// The fetcher writes Findings using these codes; the
// lambda Color func reads wave1 Findings (Source == "wave1") to color rows.
package aws

import "github.com/k2m30/a9s/v3/core/domain"

const (
	// CodeLambdaStatePending — function is in the "Pending" lifecycle state.
	// Severity: SevWarn (transitional).
	CodeLambdaStatePending domain.FindingCode = "lambda.state.pending"

	// CodeLambdaStateFailed — function is in the "Failed" lifecycle state.
	// Severity: SevBroken.
	CodeLambdaStateFailed domain.FindingCode = "lambda.state.failed"

	// CodeLambdaNoDLQ — function has no dead-letter queue configured, so
	// failed async invocations are silently dropped. Severity: SevWarn.
	CodeLambdaNoDLQ domain.FindingCode = "lambda.dlq.missing"

	// CodeLambdaDeprecatedRuntime — function uses an AWS end-of-lifed
	// runtime identifier. Severity: SevBroken.
	CodeLambdaDeprecatedRuntime domain.FindingCode = "lambda.runtime.deprecated"

	// CodeLambdaLastUpdateFailed — the function's last code/config update
	// failed to apply. Severity: SevBroken.
	CodeLambdaLastUpdateFailed domain.FindingCode = "lambda.last-update.failed"

	// CodeLambdaInactive — function has been evicted from memory after
	// extended idle time. Severity: SevDim (lifecycle, not an issue).
	CodeLambdaInactive domain.FindingCode = "lambda.state.inactive"

	// CodeLambdaEnvSecret — a plaintext credential sits in the function's
	// environment variables. Severity: SevBroken.
	//nolint:gosec // G101 false positive: a finding code, not a credential
	CodeLambdaEnvSecret domain.FindingCode = "lambda.env-secret"
)
