package ctevent

import "strings"

// FormatCTTarget collapses an ARN to its resource portion per §5 of
// docs/design/ct-event-list-v2.md. When the ARN's account segment differs
// from localAccount, the account ID is retained inline as "<acct>:<resource>".
// Non-ARN input is returned unchanged.
func FormatCTTarget(rawARN, localAccount string) string {
	if rawARN == "" {
		return ""
	}
	if !strings.HasPrefix(rawARN, "arn:") {
		return rawARN
	}
	// arn:aws:<service>:<region>:<account>:<resource>
	// Split on ":" with limit 6 so the resource can itself contain colons.
	parts := strings.SplitN(rawARN, ":", 6)
	if len(parts) < 6 {
		return rawARN
	}
	account := parts[4]
	resource := parts[5]
	// When account is empty (e.g. S3 bucket ARNs like arn:aws:s3:::bucket),
	// return the resource portion — there is no account segment to compare.
	if account == "" {
		return resource
	}
	// When localAccount is unknown (recipientAccountId missing from event),
	// we can't tell if this is cross-account — strip the account so same-account
	// events don't render with a spurious prefix.
	if localAccount == "" {
		return resource
	}
	if account != localAccount {
		return account + ":" + resource
	}
	return resource
}
