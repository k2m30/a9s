// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// eb_codes.go — canonical FindingCode constants for the eb (Elastic
// Beanstalk) resource type. The fetcher writes Findings using these codes;
// the eb Color func reads them (any Finding, wave1 or wave2) to color rows.
package aws

import "github.com/k2m30/a9s/v3/core/domain"

const (
	// CodeEBHealthRed — environment health is Red (critical).
	// Severity: SevBroken.
	CodeEBHealthRed domain.FindingCode = "eb.health.red"

	// CodeEBHealthYellow — environment health is Yellow (degraded).
	// Severity: SevWarn.
	CodeEBHealthYellow domain.FindingCode = "eb.health.yellow"

	// CodeEBHealthGrey — environment health is Grey (unknown/no data, e.g.
	// mid-transition).
	// Severity: SevWarn.
	CodeEBHealthGrey domain.FindingCode = "eb.health.grey"

	// CodeEBTerminated — environment Status is Terminated. Takes precedence
	// over a Grey/Yellow health reading (a torn-down environment's stale
	// health data is not itself the story) but not over Red (a critical
	// health reading on a not-yet-fully-terminated environment still wins).
	// Severity: SevDim.
	CodeEBTerminated domain.FindingCode = "eb.status.terminated"

	// CodeEBLaunching — Status is Launching/Updating with no health signal.
	// Severity: SevWarn.
	CodeEBLaunching domain.FindingCode = "eb.status.launching"

	// CodeEBTerminating — Status is Terminating with no health signal.
	// Severity: SevDim.
	CodeEBTerminating domain.FindingCode = "eb.status.terminating"
)

// ebEnvironmentFindings returns the wave1 Finding slice for an Elastic
// Beanstalk environment, mirroring colorEB's own precedence exactly: a
// Terminated status downgrades any non-Red health reading to dim; Red health
// always wins; otherwise health (Yellow/Grey) wins over status; with no
// health signal at all, status (Launching/Updating/Terminating) is the only
// signal.
func ebEnvironmentFindings(status, health string) []domain.Finding {
	var healthFinding *domain.Finding
	switch health {
	case "Red":
		healthFinding = &domain.Finding{Code: CodeEBHealthRed, Phrase: "health: red", Severity: domain.SevBroken, Source: "wave1"}
	case "Yellow":
		healthFinding = &domain.Finding{Code: CodeEBHealthYellow, Phrase: "health: yellow", Severity: domain.SevWarn, Source: "wave1"}
	case "Grey":
		healthFinding = &domain.Finding{Code: CodeEBHealthGrey, Phrase: "health: grey", Severity: domain.SevWarn, Source: "wave1"}
	}

	if status == "Terminated" && (healthFinding == nil || healthFinding.Severity != domain.SevBroken) {
		return []domain.Finding{{Code: CodeEBTerminated, Phrase: "terminated", Severity: domain.SevDim, Source: "wave1"}}
	}
	if healthFinding != nil {
		return []domain.Finding{*healthFinding}
	}

	switch status {
	case "Launching", "Updating":
		return []domain.Finding{{Code: CodeEBLaunching, Phrase: "launching", Severity: domain.SevWarn, Source: "wave1"}}
	case "Terminating":
		return []domain.Finding{{Code: CodeEBTerminating, Phrase: "terminating", Severity: domain.SevDim, Source: "wave1"}}
	}
	return nil
}
