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

func customerManagedAttachedPolicyNames(policies []iamtypes.AttachedPolicy) []string {
	ids := make([]string, 0, len(policies))
	for _, p := range policies {
		if p.PolicyName == nil {
			continue
		}
		if p.PolicyArn != nil && !IsCustomerManagedIAMPolicyARN(*p.PolicyArn) {
			continue
		}
		ids = append(ids, *p.PolicyName)
	}
	return ids
}
