// backup_match.go — AWS Backup selection ARN matching with wildcard + NotResources support.
package aws

import (
	"regexp"
	"strings"
)

// ARNMatches reports whether pattern matches arn. pattern may contain '*'
// which matches any sequence of characters (matching AWS Backup
// BackupSelection.Resources wildcard semantics per
// https://docs.aws.amazon.com/aws-backup/latest/devguide/API_BackupSelection.html).
// Other regex metacharacters in pattern are treated as literal.
// Empty pattern or empty arn returns false.
func ARNMatches(pattern, arn string) bool {
	if pattern == "" || arn == "" {
		return false
	}
	if !strings.Contains(pattern, "*") {
		return pattern == arn
	}
	// QuoteMeta escapes regex metacharacters; then restore wildcards as .*
	re := regexp.QuoteMeta(pattern)
	re = strings.ReplaceAll(re, `\*`, `.*`)
	compiled, err := regexp.Compile("^" + re + "$")
	if err != nil {
		return false
	}
	return compiled.MatchString(arn)
}

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
		if ARNMatches(p, targetARN) {
			return false
		}
	}
	for p := range strings.SplitSeq(resourcesCSV, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if ARNMatches(p, targetARN) {
			return true
		}
	}
	return false
}
