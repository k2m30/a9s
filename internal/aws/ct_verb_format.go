package aws

import "github.com/k2m30/a9s/v3/internal/semantics/ctevent"

// ClassifyCTVerb classifies a CloudTrail event into one of:
// "R" (read), "W" (write), "D" (destructive), "S" (service event),
// "I" (insight), "N" (network activity), "?" (unknown).
//
// Delegates to ctevent.ClassifyCTVerb. Exported for backward compatibility
// with tests and callers that reference internal/aws directly.
func ClassifyCTVerb(eventName, eventCategory, eventType string) string {
	return ctevent.ClassifyCTVerb(eventName, eventCategory, eventType)
}

// FormatCTTarget collapses an ARN to its resource portion.
//
// Delegates to ctevent.FormatCTTarget. Exported for backward compatibility
// with tests and callers that reference internal/aws directly.
func FormatCTTarget(rawARN, localAccount string) string {
	return ctevent.FormatCTTarget(rawARN, localAccount)
}
