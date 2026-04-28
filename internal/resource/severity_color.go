package resource

import "github.com/k2m30/a9s/v3/internal/domain"

// IsIssueSeverity reports whether the given severity contributes to attention
// filtering and issue badges. Allows internal/tui/views to call the
// domain.Severity.IsIssue() predicate without importing internal/domain directly.
func IsIssueSeverity(sev domain.Severity) bool {
	return sev.IsIssue()
}
