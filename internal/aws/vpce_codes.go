// vpce_codes.go — canonical FindingCode constants for the vpce resource type.
// The fetcher writes Findings using these codes; the VPCE Color func reads
// wave1 Findings (Source == "wave1") to color rows.
package aws

import "github.com/k2m30/a9s/v3/internal/domain"

const (
	// CodeVPCEStatePendingAcceptance — VPC endpoint is awaiting acceptance.
	// Severity: SevWarn (transitional).
	CodeVPCEStatePendingAcceptance domain.FindingCode = "vpce.state.pending-acceptance"

	// CodeVPCEStatePending — VPC endpoint is in the "Pending" lifecycle state.
	// Severity: SevWarn (transitional).
	CodeVPCEStatePending domain.FindingCode = "vpce.state.pending"

	// CodeVPCEStateDeleting — VPC endpoint is being deleted.
	// Severity: SevWarn (transitional).
	CodeVPCEStateDeleting domain.FindingCode = "vpce.state.deleting"

	// CodeVPCEStateFailed — VPC endpoint has failed.
	// Severity: SevBroken.
	CodeVPCEStateFailed domain.FindingCode = "vpce.state.failed"

	// CodeVPCEStateRejected — VPC endpoint was rejected.
	// Severity: SevBroken.
	CodeVPCEStateRejected domain.FindingCode = "vpce.state.rejected"

	// CodeVPCEStateExpired — VPC endpoint has expired.
	// Severity: SevBroken.
	CodeVPCEStateExpired domain.FindingCode = "vpce.state.expired"

	// CodeVPCEStatePartial — VPC endpoint is in a partial state.
	// Severity: SevBroken.
	CodeVPCEStatePartial domain.FindingCode = "vpce.state.partial"
)
