// eni_codes.go — canonical FindingCode constants for the eni resource type.
// Phase 03 PR-03b. The fetcher writes Findings using these codes; the
// ENI Color func reads Findings[0].Severity to color rows.
package aws

import "github.com/k2m30/a9s/v3/internal/domain"

const (
	// CodeENIStateAttaching — ENI is in the "attaching" transitional state.
	// Severity: SevWarn.
	CodeENIStateAttaching domain.FindingCode = "eni.state.attaching"

	// CodeENIStateDetaching — ENI is in the "detaching" transitional state.
	// Severity: SevWarn.
	CodeENIStateDetaching domain.FindingCode = "eni.state.detaching"

	// CodeENIStateAvailable — ENI is allocated but not attached (potential cost waste).
	// Severity: SevWarn.
	CodeENIStateAvailable domain.FindingCode = "eni.state.available"
)
