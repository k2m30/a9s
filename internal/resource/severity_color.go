package resource

import "github.com/k2m30/a9s/v3/internal/domain"

// IsIssueSeverity reports whether the given severity contributes to attention
// filtering and issue badges. Allows internal/tui/views to call the
// domain.Severity.IsIssue() predicate without importing internal/domain directly.
func IsIssueSeverity(sev domain.Severity) bool {
	return sev.IsIssue()
}

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

// ColorFromWave1 returns the Color implied by the first wave1 Finding on r and
// ok=true. ok=false signals no wave1 Finding — the caller should fall through
// to its structural classifier.
func ColorFromWave1(r Resource) (Color, bool) {
	for i := range r.Findings {
		if r.Findings[i].Source == "wave1" {
			return ColorFromSeverity(r.Findings[i].Severity), true
		}
	}
	return ColorHealthy, false
}
