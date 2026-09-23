// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import "github.com/k2m30/a9s/v3/core/domain"

// Lambda invocation-status findings emitted by FetchLambdaInvocations, from
// the report's status: timeout, or error and failure.
const (
	CodeLambdaInvocationTimeout domain.FindingCode = "lambda-invocation.broken.timeout"
	CodeLambdaInvocationError   domain.FindingCode = "lambda-invocation.broken.error"
)
