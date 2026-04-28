// igw_codes.go — canonical FindingCode constants for the igw resource type.
// The fetcher writes Findings using these codes; the IGW Color func reads
// wave1 Findings (Source == "wave1") to color rows.
package aws

import "github.com/k2m30/a9s/v3/internal/domain"

const (
	// CodeIGWStateAttaching — internet gateway is attaching to a VPC.
	// Severity: SevWarn (transitional).
	CodeIGWStateAttaching domain.FindingCode = "igw.state.attaching"

	// CodeIGWStateDetaching — internet gateway is detaching from a VPC.
	// Severity: SevWarn (transitional).
	CodeIGWStateDetaching domain.FindingCode = "igw.state.detaching"
)
