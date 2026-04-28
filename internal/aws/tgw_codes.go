// tgw_codes.go — canonical FindingCode constants for the tgw resource type.
// The fetcher writes Findings using these codes; the TGW Color func reads
// wave1 Findings (Source == "wave1") to color rows.
package aws

import "github.com/k2m30/a9s/v3/internal/domain"

const (
	// CodeTGWStatePending — transit gateway is in the "pending" lifecycle state.
	// Severity: SevWarn (transitional).
	CodeTGWStatePending domain.FindingCode = "tgw.state.pending"

	// CodeTGWStateModifying — transit gateway is being modified.
	// Severity: SevWarn (transitional).
	CodeTGWStateModifying domain.FindingCode = "tgw.state.modifying"

	// CodeTGWStateDeleting — transit gateway is being deleted.
	// Severity: SevWarn (transitional).
	CodeTGWStateDeleting domain.FindingCode = "tgw.state.deleting"

	// CodeTGWStateFailed — transit gateway has failed.
	// Severity: SevBroken.
	CodeTGWStateFailed domain.FindingCode = "tgw.state.failed"
)
