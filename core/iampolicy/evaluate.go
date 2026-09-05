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
	var ex Exposure
	var conditioned bool
	accounts := map[string]struct{}{}
	actions := map[string]struct{}{}
	for _, st := range doc.Statement {
		if st.Effect != "Allow" {
			continue
		}
		if st.Principal.Wildcard || st.NotPrincipal {
			if isRestrictive(st.Condition) {
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
}

// isRestrictive mirrors Prowler's is_condition_block_restrictive with
// is_cross_account_allowed=True: any positive operator carrying a scoping
// key whose every value is concrete. Negated and IfExists operators cannot
// narrow the principal, so they never count.
func isRestrictive(cond map[string]map[string][]string) bool {
	for op, block := range cond {
		if strings.Contains(op, "Not") || strings.HasSuffix(op, "IfExists") {
			continue
		}
		for key, vals := range block {
			if len(vals) == 0 {
				continue
			}
			switch {
			case strings.EqualFold(key, "aws:SourceIp"):
				if !slices.ContainsFunc(vals, isAnyIP) {
					return true
				}
			case slices.ContainsFunc(restrictiveKeys, func(k string) bool { return strings.EqualFold(k, key) }):
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
// anything that, once the ARN scaffolding is removed, has no concrete part.
func isWildcardValue(v string) bool {
	v = strings.NewReplacer("*", "", ":", "", "arn", "", "aws", "").Replace(strings.TrimSpace(v))
	return v == ""
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
