// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import "github.com/k2m30/a9s/v3/core/domain"

// One code per cause computeCTStatus can report, so an event's Status cell
// says why it is flagged without the wording being built per event under a
// shared code. The two tier codes stay as the cause each tier reduces to when
// no more specific one applies: ct-danger without an error code is a
// destructive call, and ct-attention that is none of write, cross-account or
// sensitive-read is root activity.
const (
	CodeCTEventDanger    domain.FindingCode = "ct_event.severity.danger"
	CodeCTEventAttention domain.FindingCode = "ct_event.severity.attention"

	CodeCTEventFailedCall    domain.FindingCode = "ct_event.danger.failed"
	CodeCTEventWrite         domain.FindingCode = "ct_event.attention.write"
	CodeCTEventCrossAccount  domain.FindingCode = "ct_event.attention.cross-account"
	CodeCTEventSensitiveRead domain.FindingCode = "ct_event.attention.sensitive-read"

	// CodeCTEventInfo — routine, non-sensitive management/read event with no
	// error, no cross-account signal, and no root/sensitive-read match.
	// Severity: SevDim (informational, de-emphasized).
	CodeCTEventInfo domain.FindingCode = "ct_event.severity.info"
)
