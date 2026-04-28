// eb_codes.go — canonical FindingCode constants for the eb resource type.
// Phase 03 PR-03b. The fetcher writes Findings using these codes; the
// EB Color func reads Findings[0].Severity to color rows.
package aws

import "github.com/k2m30/a9s/v3/internal/domain"

const (
	// CodeEBHealthYellow — Elastic Beanstalk environment health is "Yellow".
	// Severity: SevWarn (degraded).
	CodeEBHealthYellow domain.FindingCode = "eb.health.yellow"

	// CodeEBHealthGrey — Elastic Beanstalk environment health is "Grey"
	// (EB has not collected health data yet). Severity: SevWarn (transitional).
	CodeEBHealthGrey domain.FindingCode = "eb.health.grey"

	// CodeEBHealthRed — Elastic Beanstalk environment health is "Red".
	// Severity: SevBroken.
	CodeEBHealthRed domain.FindingCode = "eb.health.red"
)
