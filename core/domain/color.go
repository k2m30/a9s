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
