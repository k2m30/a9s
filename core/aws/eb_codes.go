// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// eb_codes.go — canonical FindingCode constants for the eb (Elastic
// Beanstalk) resource type. The fetcher writes Findings using these codes;
// the eb Color func reads them (any Finding, wave1 or wave2) to color rows.
package aws

import "github.com/k2m30/a9s/v3/core/domain"

const (
	// CodeEBHealthRed — environment health is Red (critical).
	CodeEBHealthRed domain.FindingCode = "eb.health.red"

	// CodeEBHealthYellow — environment health is Yellow (degraded).
	CodeEBHealthYellow domain.FindingCode = "eb.health.yellow"

	// CodeEBHealthGrey — environment health is Grey (unknown/no data, e.g.
	// mid-transition).
	CodeEBHealthGrey domain.FindingCode = "eb.health.grey"

	CodeEBTerminated domain.FindingCode = "eb.status.terminated"

	CodeEBLaunching domain.FindingCode = "eb.status.launching"

	CodeEBTerminating domain.FindingCode = "eb.status.terminating"

	CodeEBTransitional       domain.FindingCode = "eb.status.transitional"
	CodeEBUnrecognisedStatus domain.FindingCode = "eb.status.unrecognised"
)

// ebEnvironmentFindings returns the wave1 Finding slice for an Elastic
// Beanstalk environment: its Status from the state table, and its Health, a
// second axis, beside it.
func ebEnvironmentFindings(status, health string) []domain.Finding {
	var posture []domain.Finding
	switch health {
	case "Red":
		posture = append(posture, wave1Finding(CodeEBHealthRed))
	case "Yellow":
		posture = append(posture, wave1Finding(CodeEBHealthYellow))
	case "Grey":
		posture = append(posture, wave1Finding(CodeEBHealthGrey))
	}
	return withState("eb", status, posture)
}
