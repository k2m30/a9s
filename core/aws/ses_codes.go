// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import "github.com/k2m30/a9s/v3/core/domain"

const (
	CodeSESVerificationFailed     domain.FindingCode = "ses.verification.failed"
	CodeSESVerificationTempFail   domain.FindingCode = "ses.verification.temp_failure"
	CodeSESVerificationNotStarted domain.FindingCode = "ses.verification.not_started"
	CodeSESVerificationPending    domain.FindingCode = "ses.verification.pending"
	CodeSESSendingDisabled        domain.FindingCode = "ses.sending.disabled"
)
