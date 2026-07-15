package aws

import "github.com/k2m30/a9s/v3/core/domain"

const (
	CodeCTEventDanger    domain.FindingCode = "ct_event.severity.danger"
	CodeCTEventAttention domain.FindingCode = "ct_event.severity.attention"

	// CodeCTEventInfo — routine, non-sensitive management/read event with no
	// error, no cross-account signal, and no root/sensitive-read match.
	// Severity: SevDim (informational, de-emphasized).
	CodeCTEventInfo domain.FindingCode = "ct_event.severity.info"
)
