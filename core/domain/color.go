// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package domain

// Color classifies a resource's health for display, filtering, and badges.
type Color uint8

const (
	ColorHealthy Color = iota // green  — nominal
	ColorWarning              // yellow — transitioning / degrading
	ColorBroken               // red    — stopped / failed / impaired
	ColorDim                  // grey   — terminated / inactive
)

// IsIssue reports whether this color contributes to attention filtering and issue badges.
func (c Color) IsIssue() bool { return c == ColorWarning || c == ColorBroken }

// Color is the display colour a finding of this severity paints its row. It
// lives here rather than beside either of its callers because the catalog's
// default classifier and the per-type classifiers in core/aws both answer this
// question, and two tables would let a severity paint two colours.
func (s Severity) Color() Color {
	switch s {
	case SevBroken:
		return ColorBroken
	case SevWarn:
		return ColorWarning
	case SevDim:
		return ColorDim
	}
	return ColorHealthy
}
