package aws

import (
	"strings"

	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
)

// IsCustomerManagedIAMPolicyARN reports whether an IAM policy ARN belongs to
// the current account rather than the global AWS-managed policy namespace.
func IsCustomerManagedIAMPolicyARN(policyARN string) bool {
	return policyARN != "" && !strings.Contains(policyARN, ":aws:policy/")
}

// attachedPolicyNames returns every PolicyName in the slice, AWS-managed and
// customer-managed alike. The related-panel lazy-add path (SetFetchByIDsForTest
// for "policy") resolves AWS-managed names on demand so the drill lands on a
// real entry even though the paginated policy fetcher filters Scope=Local.
// Previously this helper pre-filtered by ARN to match the fetcher's Scope=Local
// filter, which kept Count and drill consistent but hid AWS-managed attachments
// from the operator entirely.
func attachedPolicyNames(policies []iamtypes.AttachedPolicy) []string {
	ids := make([]string, 0, len(policies))
	for _, p := range policies {
		if p.PolicyName == nil {
			continue
		}
		ids = append(ids, *p.PolicyName)
	}
	return ids
}
