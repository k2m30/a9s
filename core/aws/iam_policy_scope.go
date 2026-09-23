// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
)

// IsCustomerManagedIAMPolicyARN reports whether an IAM policy ARN belongs to
// the current account rather than the global AWS-managed policy namespace.
func IsCustomerManagedIAMPolicyARN(policyARN string) bool {
	a, err := arn.Parse(policyARN)
	return policyARN != "" && (err != nil || a.AccountID != "aws")
}

// attachedPolicyIDs returns the ARN of every attachment in the slice, which
// is what a policy row is keyed by: an attachment names both the policy's
// name and its ARN, and only the ARN says whether it is the account's own
// policy or the AWS-managed one of that name. The related-panel lazy-add path
// (SetFetchByIDsForTest for "policy") resolves an AWS-managed policy on
// demand, since the paginated policy fetcher filters Scope=Local.
func attachedPolicyIDs(policies []iamtypes.AttachedPolicy) []string {
	ids := make([]string, 0, len(policies))
	for _, p := range policies {
		if p.PolicyArn == nil || *p.PolicyArn == "" {
			continue
		}
		ids = append(ids, *p.PolicyArn)
	}
	return ids
}
