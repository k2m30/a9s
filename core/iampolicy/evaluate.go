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

func evaluate(doc Document, ownAccount string, keys []scopingKey) Exposure {
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

// scopingKey is a condition key that can narrow a wildcard principal,
// paired with the test its value must pass to actually do so. Most keys
// name the caller and need only a concrete value; a key that scopes the
// request path rather than the caller narrows nobody at its permissive
// value, and aws:SourceIp is a range only the IpAddress operator compares.
type scopingKey struct {
	key     string
	narrows func(op, val string) bool
}

func concreteValue(_, val string) bool { return !isWildcardValue(val) }

func concreteKeys(names ...string) []scopingKey {
	out := make([]scopingKey, len(names))
	for i, n := range names {
		out[i] = scopingKey{n, concreteValue}
	}
	return out
}

// restrictiveKeys scope a wildcard principal to an account, org, ARN, VPC,
// caller or address range, or to a signed caller through the function-URL
// auth type. Compared with strings.EqualFold.
var restrictiveKeys = slices.Concat(
	concreteKeys(
		"aws:SourceAccount", "aws:SourceOwner", "aws:SourceArn", "aws:SourceVpc", "aws:SourceVpce",
		"aws:PrincipalAccount", "aws:PrincipalArn", "aws:PrincipalOrgID", "aws:PrincipalOrgPaths",
		"aws:ResourceAccount", "aws:ResourceOrgID", "aws:userid", "aws:username", "s3:ResourceAccount",
		"kms:CallerAccount", "aws:SourceOrgID", "elasticfilesystem:AccessPointArn",
	),
	[]scopingKey{
		{"aws:SourceIp", func(op, val string) bool {
			return strings.EqualFold(bareOperator(op), "IpAddress") && !isAnyIP(val)
		}},
		// AWS_IAM is the only value that forces a signed caller; NONE is
		// what a public function URL is configured with.
		{"lambda:FunctionUrlAuthType", func(_, val string) bool { return strings.EqualFold(val, "AWS_IAM") }},
	},
)

// sourceScopeKeys, a subset of restrictiveKeys, name the account or resource
// on whose behalf a service acts. They answer the confused-deputy question,
// which an address range does not: an AWS service calls a role from
// AWS-owned addresses whoever asked it to.
var sourceScopeKeys = slices.DeleteFunc(slices.Clone(restrictiveKeys), func(k scopingKey) bool {
	return !slices.Contains([]string{"aws:SourceAccount", "aws:SourceArn", "aws:SourceOrgID"}, k.key)
})

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
// the comparison is. IAM defines no other prefix, so an operator carrying
// one is an operator nothing is known about and keeps its full name.
func bareOperator(op string) string {
	for _, prefix := range []string{"ForAllValues:", "ForAnyValue:"} {
		if rest, found := strings.CutPrefix(op, prefix); found {
			return rest
		}
	}
	return op
}

// trustRestrictiveKeys is restrictiveKeys plus the trust-policy-only
// sts:ExternalId. Used by EvaluateTrust.
var trustRestrictiveKeys = slices.Concat(concreteKeys("sts:ExternalId"), restrictiveKeys)

// isRestrictive mirrors Prowler's is_condition_block_restrictive with
// is_cross_account_allowed=True: a positive operator carrying one of the
// caller's scoping keys, every value of which narrows who may call. The operator decides first, so
// a key named under Null, a Not operator, IfExists or an unknown operator
// narrows nothing whatever its value.
func isRestrictive(cond map[string]map[string][]string, keys []scopingKey) bool {
	for op, block := range cond {
		if !isPositiveOperator(op) {
			continue
		}
		for key, vals := range block {
			i := slices.IndexFunc(keys, func(k scopingKey) bool { return strings.EqualFold(k.key, key) })
			if i < 0 || len(vals) == 0 {
				continue
			}
			if !slices.ContainsFunc(vals, func(v string) bool { return !keys[i].narrows(op, v) }) {
				return true
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
// an ARN whose every field after the partition is empty or a wildcard, so it
// names no resource in particular. An ARN whose account is "*" is one too,
// whatever it names inside that account: that is the same "names everyone"
// shape isAnyoneARN reads on a principal, and both answer from one parse.
func isWildcardValue(v string) bool {
	v = strings.TrimSpace(v)
	if v == "" || v == "*" {
		return true
	}
	fields := strings.Split(v, ":")
	if len(fields) < 2 || fields[0] != "arn" {
		return false
	}
	if f, ok := arnFields(v); ok && f[4] == "*" {
		return true
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
