// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import "github.com/k2m30/a9s/v3/core/domain"

// SFN execution-status findings emitted by FetchSFNExecutions. FAILED,
// TIMED_OUT, and ABORTED are all terminal failure states and classify as
// broken; RUNNING, SUCCEEDED, and PENDING_REDRIVE emit no finding (healthy).
const (
	CodeSFNExecutionFailed   domain.FindingCode = "sfn-execution.broken.failed"
	CodeSFNExecutionTimedOut domain.FindingCode = "sfn-execution.broken.timed_out"
	CodeSFNExecutionAborted  domain.FindingCode = "sfn-execution.broken.aborted"
)
