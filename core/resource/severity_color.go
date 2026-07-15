// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package resource

import "github.com/k2m30/a9s/v3/core/domain"

// ColorFromSeverity maps a domain.Severity to the corresponding display Color.
// Used by Color funcs that read Findings[0].Severity.
func ColorFromSeverity(sev domain.Severity) Color {
	switch sev {
	case domain.SevBroken:
		return ColorBroken
	case domain.SevWarn:
		return ColorWarning
	case domain.SevDim:
		return ColorDim
	default:
		return ColorHealthy
	}
}
