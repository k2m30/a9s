// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// eb_rule_codes.go — canonical FindingCode constants for the eb-rule resource
// type. The fetcher writes Findings using these codes; the eb-rule Color func
// reads them (any Finding, wave1 or wave2) to color rows.
package aws

import "github.com/k2m30/a9s/v3/core/domain"

const (
	// CodeEBRuleDisabled — rule is in the "DISABLED" lifecycle state.
	// Severity: SevDim (deliberately paused, not an error).
	CodeEBRuleDisabled domain.FindingCode = "eb-rule.state.disabled"
)
