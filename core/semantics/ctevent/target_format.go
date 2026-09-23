// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package ctevent

import "github.com/aws/aws-sdk-go-v2/aws/arn"

// FormatCTTarget collapses an ARN to its resource portion. When the ARN's account segment differs
// from localAccount, the account ID is retained inline as "<acct>:<resource>".
// Non-ARN input is returned unchanged.
func FormatCTTarget(rawARN, localAccount string) string {
	if rawARN == "" {
		return ""
	}
	a, err := arn.Parse(rawARN)
	if err != nil {
		return rawARN
	}
	account, resource := a.AccountID, a.Resource
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
