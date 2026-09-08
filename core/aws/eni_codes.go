// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// eni_codes.go — canonical FindingCode constants for the eni resource type.
// The fetcher writes Findings using these codes; the
// ENI Color func reads wave1 Findings (Source == "wave1") to color rows.
package aws

import "github.com/k2m30/a9s/v3/core/domain"

const (
	// CodeENIStateAttaching — ENI is in the "attaching" transitional state.
	CodeENIStateAttaching domain.FindingCode = "eni.state.attaching"

	// CodeENIStateDetaching — ENI is in the "detaching" transitional state.
	CodeENIStateDetaching domain.FindingCode = "eni.state.detaching"

	// CodeENIStateAvailable — ENI is allocated but not attached (potential cost waste).
	CodeENIStateAvailable domain.FindingCode = "eni.state.available"
)
