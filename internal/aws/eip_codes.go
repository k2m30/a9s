// eip_codes.go — canonical FindingCode constants for the eip resource type.
// The fetcher writes Findings using these codes; the
// EIP Color func reads wave1 Findings (Source == "wave1") to color rows.
package aws

import "github.com/k2m30/a9s/v3/internal/domain"

const (
	// CodeEIPUnassociated — Elastic IP is allocated but not associated with any
	// instance, ENI, or NAT gateway (cost waste). Severity: SevWarn.
	CodeEIPUnassociated domain.FindingCode = "eip.unassociated"
)
