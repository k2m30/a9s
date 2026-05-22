// backup_match.go — AWS Backup plan coverage logic.
package aws

import (
	"strings"

	"github.com/k2m30/a9s/v3/internal/semantics/selector"
)

// BackupPlanCoversARN reports whether a backup plan with the given
// comma-joined Resources and NotResources ARN lists covers targetARN.
// A plan covers targetARN iff ANY Resources entry matches AND NO NotResources
// entry matches. Whitespace around CSV entries is trimmed; empty entries
// are ignored.
func BackupPlanCoversARN(resourcesCSV, notResourcesCSV, targetARN string) bool {
	if targetARN == "" {
		return false
	}
	// Exclusion wins: if any NotResources pattern matches, the plan does not cover.
	for p := range strings.SplitSeq(notResourcesCSV, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if selector.MatchARN(p, targetARN) {
			return false
		}
	}
	for p := range strings.SplitSeq(resourcesCSV, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if selector.MatchARN(p, targetARN) {
			return true
		}
	}
	return false
}
