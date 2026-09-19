// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package iampolicy

import (
	"slices"
	"strconv"
	"strings"
)

// Exposure is the verdict of Evaluate on a resource policy.
type Exposure struct {
	// Public: at least one Allow statement grants to a wildcard principal
	// (or uses NotPrincipal), with no restrictive condition, an action no
	// Deny statement takes away from everyone or fences to a restrictive
	// scope.
	Public bool
	// Conditioned: wildcard-principal grants exist but every one of them is
	// scoped, by a restrictive condition on the Allow or by a Deny that
	// reaches every caller outside a restrictive scope. False whenever
	// Public.
	Conditioned bool
	// CrossAccount: sorted, de-duplicated account IDs named as principals
	// that differ from ownAccount and keep an action no unconditional Deny
	// takes from them. ownAccount "" reports every account.
	CrossAccount []string
	// PublicActions: sorted, de-duplicated actions that made Public true.
	// nil when !Public.
	PublicActions []string
}

// Principal is one principal a policy grants. Kind is "AWS", "Service",
// "Federated" or "*"; Account is the account an AWS or Federated principal
// belongs to, "" when its value names none.
type Principal struct {
	Kind, Value, Account string
}

// AllowedPrincipals returns the principals the Allow statements grant, less
// every one an unconditional Deny takes all of its granted actions from. A
// Deny under a condition the principal can satisfy takes nothing away.
// Foreign principals are returned with their account, for the caller to
// keep or drop.
func AllowedPrincipals(doc Document, _ string) []Principal {
	var out []Principal
	for _, st := range doc.Statement {
		if st.Effect != "Allow" {
			continue
		}
		for _, p := range st.principals() {
			if !slices.Contains(out, p) && doc.grants(st, p) {
				out = append(out, p)
			}
		}
	}
	return out
}

// AccountFromARN returns the account an ARN names, or "" when s is not an
// ARN or its account field is not an account ID (an S3 bucket ARN has none;
// an AWS-managed policy's reads "aws").
func AccountFromARN(s string) string {
	if f, ok := arnFields(s); ok && isAccountID(f[4]) {
		return f[4]
	}
	return ""
}

// Evaluate classifies the Allow statements of a resource policy, less what
// its explicit Deny statements take away: IAM applies a matching Deny over
// any Allow.
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

// EvaluateEndpoint classifies a VPC endpoint policy. Identical to Evaluate
// except that the resource-side keys count as restrictive: an endpoint
// policy governs requests to other resources, and aws:ResourceAccount there
// narrows which resources the endpoint reaches.
func EvaluateEndpoint(doc Document) Exposure {
	return evaluate(doc, "", endpointRestrictiveKeys)
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
		if st.Principal.Wildcard || st.NotPrincipal != nil {
			var open []string
			fenced := false
			for _, a := range st.grantedActions() {
				switch doc.denied(a, anyone, keys) {
				case denyNone:
					open = append(open, a)
				case denyFence:
					fenced = true
				}
			}
			switch {
			case len(open) > 0 && !isRestrictive(st.Condition, keys):
				ex.Public = true
				if len(st.Action) > 0 {
					for _, a := range open {
						actions[a] = struct{}{}
					}
				}
			case len(open) > 0 || fenced:
				conditioned = true
			}
		}
		for _, p := range st.principals() {
			if p.Kind == "AWS" && p.Account != "" && p.Account != ownAccount && doc.grants(st, p) {
				accounts[p.Account] = struct{}{}
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

var anyone = Principal{Kind: "*", Value: "*"}

// principals lists who an Allow statement grants. A NotPrincipal grant
// reaches everyone the block does not name, so it reads as the wildcard.
func (st Statement) principals() []Principal {
	var out []Principal
	if st.Principal.Wildcard || st.NotPrincipal != nil {
		out = append(out, anyone)
	}
	for _, v := range st.Principal.AWS {
		out = append(out, Principal{Kind: "AWS", Value: v, Account: accountID(v)})
	}
	for _, v := range st.Principal.Service {
		out = append(out, Principal{Kind: "Service", Value: v})
	}
	for _, v := range st.Principal.Federated {
		out = append(out, Principal{Kind: "Federated", Value: v, Account: AccountFromARN(v)})
	}
	return out
}

// grantedActions is what an Allow statement grants. A NotAction grant is
// every action but the listed ones, which only a Deny of "*" covers.
func (st Statement) grantedActions() []string {
	if len(st.Action) == 0 {
		return []string{"*"}
	}
	return st.Action
}

// grants reports whether st still grants p at least one action once the
// document's unconditional Deny statements are applied.
func (d Document) grants(st Statement, p Principal) bool {
	return slices.ContainsFunc(st.grantedActions(), func(a string) bool { return d.denied(a, p, restrictiveKeys) != denyAll })
}

// denyReach is how far a Deny statement reaches the callers it names.
type denyReach int

const (
	denyNone denyReach = iota
	// denyFence reaches every caller outside a restrictive scope (an
	// account, org, endpoint or address range).
	denyFence
	denyAll
)

// denied is the furthest reach of the document's Deny statements that name p
// and cover action, a fence being read against keys.
func (d Document) denied(action string, p Principal, keys []scopingKey) denyReach {
	reach := denyNone
	for _, st := range d.Statement {
		if st.Effect != "Deny" || !st.covers(action) || !st.names(p) {
			continue
		}
		r := st.denyReach(keys)
		if st.NotPrincipal != nil && p == anyone {
			// Everyone but the named principals is denied: the wildcard grant
			// is fenced to them.
			r = min(r, denyFence)
		}
		reach = max(reach, r)
	}
	return reach
}

// names reports whether a Deny statement reaches p: its Principal block
// names p, or its NotPrincipal block does not.
func (st Statement) names(p Principal) bool {
	if st.NotPrincipal != nil {
		return !st.NotPrincipal.names(p)
	}
	return st.Principal.names(p)
}

// names reports whether the block names p. An account's root ARN or bare ID
// names every principal of that account.
func (b PrincipalBlock) names(p Principal) bool {
	if b.Wildcard {
		return true
	}
	switch p.Kind {
	case "AWS":
		return slices.ContainsFunc(b.AWS, func(v string) bool {
			return v == p.Value || (p.Account != "" && (v == p.Account || AccountFromARN(v) == p.Account && strings.HasSuffix(v, ":root")))
		})
	case "Service":
		return slices.ContainsFunc(b.Service, func(v string) bool { return strings.EqualFold(v, p.Value) })
	case "Federated":
		return slices.Contains(b.Federated, p.Value)
	}
	return false
}

// denyReach classifies a Deny statement by its condition, whose entries IAM
// ANDs: none reaches every caller; entries that each fire outside a
// restrictive scope fence the grant to that scope; any other entry is one a
// caller can avoid (sending over TLS, presenting MFA), so the Deny reaches
// nobody for certain.
func (st Statement) denyReach(keys []scopingKey) denyReach {
	if len(st.Condition) == 0 {
		return denyAll
	}
	for op, block := range st.Condition {
		for key, vals := range block {
			if !fences(op, key, vals, keys) {
				return denyNone
			}
		}
	}
	return denyFence
}

// negatedOperators pairs each negated operator with the positive one it
// negates. A negated operator is true when its key is absent from the
// request, so its IfExists variant fires on the same requests.
var negatedOperators = [][2]string{
	{"StringNotEquals", "StringEquals"}, {"StringNotEqualsIgnoreCase", "StringEqualsIgnoreCase"},
	{"StringNotLike", "StringLike"}, {"ArnNotEquals", "ArnEquals"}, {"ArnNotLike", "ArnLike"},
	{"NotIpAddress", "IpAddress"},
}

// fences reports whether a Deny condition entry fires for every caller the
// positive form of the entry would not admit as scoped. A Bool entry fences
// only with IfExists: plain Bool is false when the key is absent, and an
// anonymous request carries no aws:PrincipalIsAWSService. A ForAnyValue
// entry without IfExists is false when the key is absent, so it does not
// fire for such a request either.
func fences(op, key string, vals []string, keys []scopingKey) bool {
	ifExists := strings.HasSuffix(op, "IfExists")
	if hasSetPrefix(op, "ForAnyValue:") && !ifExists {
		return false
	}
	bare := strings.TrimSuffix(bareOperator(op), "IfExists")
	for _, pair := range negatedOperators {
		if strings.EqualFold(bare, pair[0]) {
			return isRestrictive(map[string]map[string][]string{pair[1]: {key: vals}}, keys)
		}
	}
	if !strings.EqualFold(bareOperator(op), "BoolIfExists") {
		return false
	}
	flipped := make([]string, len(vals))
	for i, v := range vals {
		flipped[i] = strconv.FormatBool(!strings.EqualFold(v, "true"))
	}
	return isRestrictive(map[string]map[string][]string{"Bool": {key: flipped}}, keys)
}

// accountID extracts the 12-digit account from a bare ID or the account
// field of an ARN, in any partition.
func accountID(principal string) string {
	if isAccountID(principal) {
		return principal
	}
	return AccountFromARN(principal)
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
// caller or address range, to AWS service principals, or to a signed caller
// through the function-URL auth type. Compared with strings.EqualFold.
var restrictiveKeys = slices.Concat(
	concreteKeys(
		"aws:SourceAccount", "aws:SourceOwner", "aws:SourceArn", "aws:SourceVpc", "aws:SourceVpce",
		"aws:PrincipalAccount", "aws:PrincipalArn", "aws:PrincipalOrgID", "aws:PrincipalOrgPaths",
		"aws:userid", "aws:username", "kms:CallerAccount", "aws:SourceOrgID", "elasticfilesystem:AccessPointArn",
	),
	[]scopingKey{
		{"aws:SourceIp", func(op, val string) bool {
			return strings.EqualFold(bareOperator(op), "IpAddress") && !isAnyIP(val)
		}},
		// True only for a service principal calling directly; anonymous
		// requests carry no value, so plain Bool excludes them.
		{"aws:PrincipalIsAWSService", func(op, val string) bool {
			return strings.EqualFold(bareOperator(op), "Bool") && strings.EqualFold(val, "true")
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

// positiveOperators assert that a key equals or matches a listed value (Bool
// compares a boolean key with "true" or "false"), and
// so are the only operators that can narrow who may call. Null asserts the
// key is absent, a Not operator allows everything but the listed value, an
// IfExists variant lets the caller omit the key, and an operator IAM does
// not define constrains nothing that can be reasoned about; the match is
// exact, so all of them fall outside.
var positiveOperators = []string{
	"StringEquals", "StringEqualsIgnoreCase", "StringLike", "ArnEquals", "ArnLike", "IpAddress", "Bool",
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
		if hasSetPrefix(op, prefix) {
			return op[len(prefix):]
		}
	}
	return op
}

// trustRestrictiveKeys is restrictiveKeys plus the trust-policy-only
// sts:ExternalId. Used by EvaluateTrust.
var trustRestrictiveKeys = slices.Concat(concreteKeys("sts:ExternalId"), restrictiveKeys)

// endpointRestrictiveKeys is restrictiveKeys plus the keys that name the
// resource being called. On a resource's own policy they hold for every
// caller of that resource and scope nobody; on a VPC endpoint policy they
// narrow which resources the endpoint reaches. Used by EvaluateEndpoint.
var endpointRestrictiveKeys = slices.Concat(concreteKeys(
	"aws:ResourceAccount", "aws:ResourceOrgID", "aws:ResourceOrgPaths", "s3:ResourceAccount",
), restrictiveKeys)

// hasSetPrefix reports whether op carries the ForAllValues: or ForAnyValue:
// set prefix named by prefix.
func hasSetPrefix(op, prefix string) bool {
	return len(op) >= len(prefix) && strings.EqualFold(op[:len(prefix)], prefix)
}

// isRestrictive mirrors Prowler's is_condition_block_restrictive with
// is_cross_account_allowed=True: a positive operator carrying one of the
// caller's scoping keys, every value of which narrows who may call. The
// operator decides first, so a key named under Null, a Not operator,
// IfExists or an unknown operator narrows nothing whatever its value, and
// neither does a ForAllValues operator, which is true when the key is absent
// from the request.
func isRestrictive(cond map[string]map[string][]string, keys []scopingKey) bool {
	for op, block := range cond {
		if !isPositiveOperator(op) || hasSetPrefix(op, "ForAllValues:") {
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
