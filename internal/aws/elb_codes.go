// elb_codes.go — canonical FindingCode constants for the elb resource type.
// The fetcher writes Findings using these codes; the ELB Color func reads
// wave1 Findings (Source == "wave1") to color rows.
package aws

import "github.com/k2m30/a9s/v3/internal/domain"

const (
	// CodeELBStateProvisioning — load balancer is in the "provisioning" lifecycle state.
	// Severity: SevWarn (transitional).
	CodeELBStateProvisioning domain.FindingCode = "elb.state.provisioning"

	// CodeELBStateActiveImpaired — load balancer is active but impaired.
	// Severity: SevWarn (degraded).
	CodeELBStateActiveImpaired domain.FindingCode = "elb.state.active-impaired"

	// CodeELBStateFailed — load balancer has failed.
	// Severity: SevBroken.
	CodeELBStateFailed domain.FindingCode = "elb.state.failed"
)
