// nat_codes.go — canonical FindingCode constants for the nat resource type.
// The fetcher writes Findings using these codes; the NAT Color func reads
// wave1 Findings (Source == "wave1") to color rows.
package aws

import "github.com/k2m30/a9s/v3/internal/domain"

const (
	// CodeNATStatePending — NAT gateway is in the "pending" lifecycle state.
	// Severity: SevWarn (transitional).
	CodeNATStatePending domain.FindingCode = "nat.state.pending"

	// CodeNATStateDeleting — NAT gateway is being deleted.
	// Severity: SevWarn (transitional).
	CodeNATStateDeleting domain.FindingCode = "nat.state.deleting"

	// CodeNATStateFailed — NAT gateway has failed.
	// Severity: SevBroken.
	CodeNATStateFailed domain.FindingCode = "nat.state.failed"
)
