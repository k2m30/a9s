// vpc_codes.go — canonical FindingCode constants for the vpc resource type.
// The fetcher writes Findings using these codes; the VPC Color func reads
// wave1 Findings (Source == "wave1") to color rows.
package aws

import "github.com/k2m30/a9s/v3/internal/domain"

const (
	// CodeVPCStatePending — VPC is in the "pending" lifecycle state.
	// Severity: SevWarn (transitional).
	CodeVPCStatePending domain.FindingCode = "vpc.state.pending"
)
