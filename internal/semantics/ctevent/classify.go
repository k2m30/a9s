package ctevent

import "strings"

// ClassifyCTVerb classifies a CloudTrail event into one of:
// "R" (read), "W" (write), "D" (destructive), "S" (service event),
// "I" (insight), "N" (network activity), "?" (unknown).
//
// Implements §2.1 of docs/design/ct-event-list-v2.md. Order matters; first match wins.
//  1. eventCategory == "Insight" → "I"
//  2. eventCategory == "NetworkActivity" → "N"
//  3. eventType == "AwsServiceEvent" → "S"
//  4. BatchGet* prefix → "R" (must beat Batch write prefix)
//  5. KMS use-key exact names → "R" (Decrypt, Encrypt, Sign, Verify, ReEncrypt, GenerateDataKey*)
//  5b. SES Verify* exact override → "W" (VerifyEmailIdentity / VerifyDomainIdentity / VerifyEmailAddress /
//     VerifyDomainDkim trigger AWS to send a verification email and create a verification record —
//     state-mutating writes despite the "Verify" prefix; exact-match beats the read-prefix table)
//  6. Destructive prefix table → "D"
//  7. Read prefix table → "R"
//  8. Write prefix table → "W" ("Assume" prefix catches non-STS Assume* events; all AssumeRole* are exact-match R above)
//  9. "?" (no match)
func ClassifyCTVerb(eventName, eventCategory, eventType string) string {
	// Category / type overrides (highest precedence).
	switch eventCategory {
	case "Insight":
		return "I"
	case "NetworkActivity":
		return "N"
	}
	if eventType == "AwsServiceEvent" {
		return "S"
	}

	// Special-case reads BEFORE prefix matching.
	// BatchGet* must beat the Batch write prefix.
	if strings.HasPrefix(eventName, "BatchGet") {
		return "R"
	}
	// BatchDelete* must beat the Batch write prefix.
	if strings.HasPrefix(eventName, "BatchDelete") {
		return "D"
	}
	// KMS use-key ops and STS role-vending — exact matches (§2.1 row 2 additional).
	// All AssumeRole* operations are STS session vending (identity exchange, not state
	// mutation). They are exact-matched here so the "Assume" W-prefix below only catches
	// non-STS Assume* events from other services (if any).
	switch eventName {
	case "Decrypt", "Encrypt", "Sign", "Verify",
		"ReEncrypt", "GenerateDataKey", "GenerateDataKeyWithoutPlaintext",
		"AssumeRole", "AssumeRoleWithSAML", "AssumeRoleWithWebIdentity":
		return "R"
	}

	// SES verification operations are mutating writes, not reads, despite the
	// "Verify" prefix. Exact-match override beats the read-prefix table.
	switch eventName {
	case "VerifyEmailIdentity", "VerifyDomainIdentity",
		"VerifyEmailAddress", "VerifyDomainDkim":
		return "W"
	}

	// Destructive prefix table (§2.1 row 1).
	for _, p := range []string{
		"Delete", "Terminate", "Destroy", "Remove", "Revoke", "Disable",
		"Stop", "Detach", "Cancel", "Reject", "Abort", "Purge",
		"Deregister", "Disassociate",
	} {
		if strings.HasPrefix(eventName, p) {
			return "D"
		}
	}

	// Read prefix table (§2.1 row 2).
	for _, p := range []string{
		"Get", "Describe", "List", "Lookup", "Search", "Query",
		"Scan", "Head", "Test", "Check", "Validate", "Verify",
	} {
		if strings.HasPrefix(eventName, p) {
			return "R"
		}
	}

	// Write prefix table (§2.1 row 4).
	for _, p := range []string{
		"Create", "Put", "Update", "Modify", "Set", "Add",
		"Attach", "Associate", "Register", "Enable", "Start", "Run",
		"Restore", "Restart", "Reboot", "Tag", "Untag", "Activate",
		"Reset", "Replace", "Apply", "Import", "Export", "Copy",
		"Move", "Upload", "Submit", "Send", "Publish", "Invoke",
		"Execute", "Transition", "Issue", "Renew", "Rotate",
		"Batch", "Assume",
	} {
		if strings.HasPrefix(eventName, p) {
			return "W"
		}
	}

	return "?"
}
