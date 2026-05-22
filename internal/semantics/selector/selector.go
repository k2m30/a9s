// Package selector holds shared resource-selection predicates used by
// related-checkers and coverage logic.
package selector

import (
	"regexp"
	"strings"
)

// MatchARN reports whether pattern matches arn. pattern may contain '*'
// which matches any sequence of characters (matching AWS Backup
// BackupSelection.Resources wildcard semantics per
// https://docs.aws.amazon.com/aws-backup/latest/devguide/API_BackupSelection.html).
// Other regex metacharacters in pattern are treated as literal.
// Empty pattern or empty arn returns false.
func MatchARN(pattern, arn string) bool {
	if pattern == "" || arn == "" {
		return false
	}
	if !strings.Contains(pattern, "*") {
		return pattern == arn
	}
	re := regexp.QuoteMeta(pattern)
	re = strings.ReplaceAll(re, `\*`, `.*`)
	compiled, err := regexp.Compile("^" + re + "$")
	if err != nil {
		return false
	}
	return compiled.MatchString(arn)
}
