// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package iampolicy

import (
	"slices"
	"strings"
)

// Exposure is the verdict of Evaluate on a resource policy.
type Exposure struct {
	// Public: at least one Allow statement grants to a wildcard principal
	// (or uses NotPrincipal) with no restrictive condition.
	Public bool
	// Conditioned: wildcard-principal Allow statements exist but every one
	// of them is scoped by a restrictive condition. False whenever Public.
	Conditioned bool
	// CrossAccount: sorted, de-duplicated account IDs named as principals
	// that differ from ownAccount. ownAccount "" reports every account.
	CrossAccount []string
	// PublicActions: sorted, de-duplicated actions of the statements that
	// made Public true. nil when !Public.
	PublicActions []string
}

// Evaluate classifies the Allow statements of a resource policy.
func Evaluate(doc Document, ownAccount string) Exposure {
	return evaluate(doc, ownAccount, restrictiveKeys)
}

// EvaluateTrust classifies a role's trust policy. Identical to Evaluate
// except that sts:ExternalId counts as a restrictive condition: on an
// AssumeRole trust policy a wildcard principal paired with a shared external
// ID is the documented cross-account pattern, while on a resource policy the
// same key scopes nothing.
func EvaluateTrust(doc Document, ownAccount string) Exposure {
	return evaluate(doc, ownAccount, trustRestrictiveKeys)
}

func evaluate(doc Document, ownAccount string, keys []string) Exposure {
	var ex Exposure
	var conditioned bool
	accounts := map[string]struct{}{}
	actions := map[string]struct{}{}
	for _, st := range doc.Statement {
		if st.Effect != "Allow" {
			continue
		}
		if st.Principal.Wildcard || st.NotPrincipal {
			if isRestrictive(st.Condition, keys) {
				conditioned = true
			} else {
				ex.Public = true
				for _, a := range st.Action {
					actions[a] = struct{}{}
				}
			}
		}
		for _, p := range st.Principal.AWS {
			if id := accountID(p); id != "" && id != ownAccount {
				accounts[id] = struct{}{}
			}
		}
	}
	ex.Conditioned = conditioned && !ex.Public
	ex.CrossAccount = sortedKeys(accounts)
	if ex.Public {
		ex.PublicActions = sortedKeys(actions)
	}
	return ex
}

// accountID extracts the 12-digit account from a bare ID or the account
// field of an ARN, in any partition.
func accountID(principal string) string {
	if isAccountID(principal) {
		return principal
	}
	parts := strings.Split(principal, ":")
	if len(parts) >= 6 && parts[0] == "arn" && isAccountID(parts[4]) {
		return parts[4]
	}
	return ""
}

func isAccountID(s string) bool {
	if len(s) != 12 {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// restrictiveKeys scope a wildcard principal to an account, org, ARN, VPC
// or caller. Compared with strings.EqualFold.
var restrictiveKeys = []string{
	"aws:SourceAccount", "aws:SourceOwner", "aws:SourceArn", "aws:SourceVpc", "aws:SourceVpce",
	"aws:PrincipalAccount", "aws:PrincipalArn", "aws:PrincipalOrgID", "aws:PrincipalOrgPaths",
	"aws:ResourceAccount", "aws:ResourceOrgID", "aws:userid", "aws:username", "s3:ResourceAccount",
	"kms:CallerAccount", "kms:ViaService", "lambda:FunctionUrlAuthType", "sns:Endpoint",
	"elasticfilesystem:AccessPointArn", "aws:SourceOrgID",
}

// positiveOperators assert that a key equals or matches a listed value, and
// so are the only operators that can narrow who may call. Null asserts the
// key is absent, a Not operator allows everything but the listed value, an
// IfExists variant lets the caller omit the key, and an operator IAM does
// not define constrains nothing that can be reasoned about; the match is
// exact, so all of them fall outside.
var positiveOperators = []string{
	"StringEquals", "StringEqualsIgnoreCase", "StringLike", "ArnEquals", "ArnLike", "IpAddress",
}

func isPositiveOperator(op string) bool {
	return slices.ContainsFunc(positiveOperators, func(p string) bool { return strings.EqualFold(p, bareOperator(op)) })
}

// bareOperator drops the ForAllValues: / ForAnyValue: set prefix, which
// selects how many values of a multi-valued key must match rather than what
// the comparison is.
func bareOperator(op string) string {
	if _, rest, found := strings.Cut(op, ":"); found {
		return rest
	}
	return op
}

// trustRestrictiveKeys is restrictiveKeys plus the trust-policy-only
// sts:ExternalId. Used by EvaluateTrust.
var trustRestrictiveKeys = slices.Concat([]string{"sts:ExternalId"}, restrictiveKeys)

// isRestrictive mirrors Prowler's is_condition_block_restrictive with
// is_cross_account_allowed=True: a positive operator carrying one of the
// scoping keys, whose every value is concrete. The operator decides first, so a key named
// under Null, a Not operator, IfExists or an unknown operator narrows
// nothing. aws:SourceIp is reached only through IpAddress, the operator
// that compares an address against a range.
func isRestrictive(cond map[string]map[string][]string, keys []string) bool {
	for op, block := range cond {
		if !isPositiveOperator(op) {
			continue
		}
		for key, vals := range block {
			if len(vals) == 0 {
				continue
			}
			switch {
			case strings.EqualFold(key, "aws:SourceIp"):
				if strings.EqualFold(bareOperator(op), "IpAddress") && !slices.ContainsFunc(vals, isAnyIP) {
					return true
				}
			case slices.ContainsFunc(keys, func(k string) bool { return strings.EqualFold(k, key) }):
				if !slices.ContainsFunc(vals, isWildcardValue) {
					return true
				}
			}
		}
	}
	return false
}

func isAnyIP(v string) bool {
	switch strings.TrimSpace(v) {
	case "0.0.0.0/0", "::/0", "*", "":
		return true
	}
	return false
}

// isWildcardValue is true for "*", "arn:aws:*", "arn:*:*:*" and the like:
// an ARN whose every field after the partition is empty or a wildcard, so
// it names no resource in particular.
func isWildcardValue(v string) bool {
	v = strings.TrimSpace(v)
	if v == "" || v == "*" {
		return true
	}
	fields := strings.Split(v, ":")
	if len(fields) < 3 || fields[0] != "arn" {
		return false
	}
	return !slices.ContainsFunc(fields[2:], func(f string) bool { return f != "" && f != "*" })
}

func sortedKeys(set map[string]struct{}) []string {
	if len(set) == 0 {
		return nil
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}
