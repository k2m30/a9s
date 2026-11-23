// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package iampolicy

import (
	"net/netip"
	"slices"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws/arn"
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
	// PublicConditions: sorted, de-duplicated conditions of the grants that
	// made Public true, one item per statement, its entries joined by " and "
	// as IAM ANDs them. They scope no caller, and say what the grant is
	// still open to. nil when !Public.
	PublicConditions []string
	// PublicDenyConditions: the same for the conditional Deny statements
	// that overlap a public grant without closing it. They say which callers
	// the Deny still lets through. nil when !Public.
	PublicDenyConditions []string
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
	if a, err := arn.Parse(s); err == nil && isAccountID(a.AccountID) {
		return a.AccountID
	}
	return ""
}

// Evaluate classifies the Allow statements of a resource or trust policy,
// less what its explicit Deny statements take away: IAM applies a matching
// Deny over any Allow.
func Evaluate(doc Document, ownAccount string) Exposure {
	return evaluate(doc, ownAccount, restrictiveKeys)
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
	conditions := map[string]struct{}{}
	denies := map[string]struct{}{}
	for _, st := range doc.Statement {
		if st.Effect != "Allow" {
			continue
		}
		if st.Principal.Wildcard || st.NotPrincipal != nil {
			var open []string
			var overlapping []Statement
			fenced := false
			for _, a := range st.grantedActions() {
				reach, seen := doc.denied(a, st.grantedResources(), anyone, keys)
				switch reach {
				case denyNone:
					open = append(open, a)
					overlapping = append(overlapping, seen...)
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
				if len(st.Condition) > 0 {
					conditions[conditionText(st.Condition)] = struct{}{}
				}
				for _, d := range overlapping {
					if len(d.Condition) > 0 {
						denies[conditionText(d.Condition)] = struct{}{}
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
		ex.PublicConditions = sortedKeys(conditions)
		ex.PublicDenyConditions = sortedKeys(denies)
	}
	return ex
}

// conditionText is one statement's condition as written: IAM ANDs its
// entries, so they are joined by " and ".
func conditionText(cond map[string]map[string][]string) string {
	var out []string
	for op, block := range cond {
		for key, vals := range block {
			out = append(out, op+" "+key+" "+strings.Join(vals, ", "))
		}
	}
	slices.Sort(out)
	return strings.Join(out, " and ")
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

// grantedResources is what an Allow statement grants on. A NotResource grant,
// or one with no Resource element (a trust policy), is read as every
// resource: only a Deny of every resource then covers it.
func (st Statement) grantedResources() []string {
	if len(st.Resource) == 0 || len(st.NotResource) > 0 {
		return []string{"*"}
	}
	return st.Resource
}

// grants reports whether st still grants p at least one action once the
// document's unconditional Deny statements are applied.
func (d Document) grants(st Statement, p Principal) bool {
	return slices.ContainsFunc(st.grantedActions(), func(a string) bool {
		reach, _ := d.denied(a, st.grantedResources(), p, restrictiveKeys)
		return reach != denyAll
	})
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

// denied is how far the document's Deny statements reach p for action on
// resources, a fence being read against keys, and every Deny statement that
// names p and touches that request, for the finding to list. For the
// wildcard principal the reach is the least over the caller classes, each
// class taking the furthest reach of the Denies that apply to it: a grant is
// closed only when every class is.
func (d Document) denied(action string, resources []string, p Principal, keys []scopingKey) (denyReach, []Statement) {
	classes := callerClasses
	if p != anyone {
		classes = []callerClass{callerClasses[slices.IndexFunc(callerClasses, func(c callerClass) bool { return c.isService == (p.Kind == "Service") })]}
	}
	reach := make([]denyReach, len(classes))
	var seen []Statement
	for _, st := range d.Statement {
		if st.Effect != "Deny" || !st.names(p) || !st.touches(action, resources) {
			continue
		}
		seen = append(seen, st)
		if !st.covers(action) || !st.coversResources(resources) {
			continue
		}
		for i, c := range classes {
			r := st.reachFor(c, keys)
			if st.NotPrincipal != nil && p == anyone {
				// Everyone but the named principals is denied: the wildcard grant
				// is fenced to them.
				r = min(r, denyFence)
			}
			reach[i] = max(reach[i], r)
		}
	}
	return slices.Min(reach), seen
}

// touches is the overlap that puts a Deny beside a public grant in the
// finding: matching only part of the request, it does not close the grant
// but still shapes who passes. A NotAction Deny is always shown.
func (st Statement) touches(action string, resources []string) bool {
	actionOK := len(st.NotAction) > 0 || slices.ContainsFunc(st.Action, func(p string) bool { return overlaps(p, action) })
	resourceOK := len(st.Resource) == 0 || slices.ContainsFunc(st.Resource, func(p string) bool {
		return slices.ContainsFunc(resources, func(r string) bool { return overlapsIn(exactCase, p, r) })
	})
	return actionOK && resourceOK
}

// coversResources reports whether a Deny matches every resource in
// resources, each of which may be a pattern. A Deny with no Resource element
// (a trust policy) matches every resource; a NotResource Deny matches those
// no listed entry overlaps. A Resource pattern contains a granted one when
// the granted pattern, read literally, matches it and it has no '?' that a
// granted '*' could stand for.
func (st Statement) coversResources(resources []string) bool {
	switch {
	case len(st.NotResource) > 0:
		return !slices.ContainsFunc(resources, func(r string) bool {
			return slices.ContainsFunc(st.NotResource, func(p string) bool { return overlapsIn(exactCase, p, r) })
		})
	case len(st.Resource) == 0:
		return true
	}
	return !slices.ContainsFunc(resources, func(r string) bool {
		return !slices.ContainsFunc(st.Resource, func(p string) bool {
			return p == r || !strings.Contains(p, "?") && matchIn(exactCase, p, r)
		})
	})
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

// callerClass is one part of the callers a Deny can single out by
// classKey: AWS service principals, or everyone else. IAM ANDs a Deny's
// condition entries, so an entry on classKey confines the Deny to one class,
// and the AWS data perimeter pairs a Deny for each (IAM User Guide, AWS
// global condition context keys, aws:SourceOrgID: the
// RCPEnforceConfusedDeputyProtection example applies its control "only on
// requests by service principals ... that have the aws:SourceAccount key
// present").
type callerClass struct {
	isService bool
	// presence lists the keys whose presence alone (Null key false) fences
	// this class: a service call that carries no source acts for no
	// customer's resource.
	presence []scopingKey
}

const classKey = "aws:PrincipalIsAWSService"

var callerClasses = []callerClass{{isService: false}, {isService: true, presence: sourceScopeKeys}}

// selects reports, for an entry on classKey, whether the Deny applies to the
// class; ok is false when the entry does not split the classes. A plain Bool
// "false" leaves out an anonymous request, which carries no classKey, so only
// its IfExists form reaches every caller that is not a service.
func (c callerClass) selects(op string, vals []string) (applies, ok bool) {
	bare := strings.TrimSuffix(op, "IfExists")
	if !strings.EqualFold(bare, "Bool") || len(vals) != 1 {
		return false, false
	}
	if strings.EqualFold(vals[0], strconv.FormatBool(c.isService)) {
		return c.isService || bare != op, true
	}
	return false, true
}

// reachFor classifies a Deny statement for one caller class by its
// condition, whose entries IAM ANDs: none reaches every caller; entries that
// each confine the Deny to the class or fire outside a restrictive scope
// fence the grant to that scope; an entry that leaves the class out, or one
// a caller can avoid (sending over TLS, presenting MFA), means the Deny
// reaches nobody in the class for certain.
func (st Statement) reachFor(c callerClass, keys []scopingKey) denyReach {
	if len(st.Condition) == 0 {
		return denyAll
	}
	reach := denyAll
	for op, block := range st.Condition {
		for key, vals := range block {
			if strings.EqualFold(key, classKey) {
				if applies, ok := c.selects(op, vals); ok {
					if !applies {
						return denyNone
					}
					continue
				}
			}
			reach = denyFence
			if !fences(op, key, vals, keys) && !c.presenceFences(op, key, vals) {
				return denyNone
			}
		}
	}
	return reach
}

func (c callerClass) presenceFences(op, key string, vals []string) bool {
	return strings.EqualFold(op, "Null") && len(vals) == 1 && strings.EqualFold(vals[0], "false") &&
		slices.ContainsFunc(c.presence, func(k scopingKey) bool { return strings.EqualFold(k.Key, key) })
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

// RestrictingKey is a condition key that scopes a Principal "*" grant, with
// the AWS documentation page that makes it do so.
type RestrictingKey struct{ Key, Citation string }

// RestrictingKeys returns the key tables Evaluate and EvaluateEndpoint read.
func RestrictingKeys() (resource, endpoint []RestrictingKey) {
	view := func(keys []scopingKey) []RestrictingKey {
		out := make([]RestrictingKey, len(keys))
		for i, k := range keys {
			out[i] = k.RestrictingKey
		}
		return out
	}
	return view(restrictiveKeys), view(endpointRestrictiveKeys)
}

// scopingKey is a condition key that can narrow a wildcard principal,
// paired with the test its value must pass to actually do so.
type scopingKey struct {
	RestrictingKey
	narrows func(op, val string) bool
}

const (
	citeS3Public   = "https://docs.aws.amazon.com/AmazonS3/latest/userguide/access-control-block-public-access.html#public-bucket-policies"
	citeEFSPublic  = "https://docs.aws.amazon.com/efs/latest/ug/access-control-block-public-access.html#what-is-a-public-policy"
	citeGlobalKeys = "https://docs.aws.amazon.com/IAM/latest/UserGuide/reference_policies_condition-keys.html"
)

// fixedValue is AWS's test for a non-public grant: a value with no '*' or
// '?' and no policy variable, each of which IAM expands at request time.
func fixedValue(_, val string) bool {
	return val != "" && !strings.ContainsAny(val, "*?") && !strings.Contains(val, "${")
}

// fixedOrgPath is fixedValue for an organization path, which may also end in
// one '*' below a fixed organization ID ("o-…/*", ".../ou-*"): that names the
// organization, an OU subtree, or an OU's children.
func fixedOrgPath(op, val string) bool {
	rest, trailing := strings.CutSuffix(val, "*")
	org, _, below := strings.Cut(rest, "/")
	return fixedValue(op, val) || trailing && below && len(org) > len("o-") && strings.HasPrefix(org, "o-") && fixedValue(op, rest)
}

// fixedUserID is fixedValue for aws:userid, which may also be one role ID
// followed by ":*": every session of that role.
func fixedUserID(op, val string) bool {
	role, sessions := strings.CutSuffix(val, ":*")
	return fixedValue(op, val) || sessions && strings.HasPrefix(role, "AROA") && !strings.Contains(role, ":") && fixedValue(op, role)
}

// boolTrue holds only when a Bool operator asserts the key true: plain Bool
// is false for a request that does not carry the key.
func boolTrue(op, val string) bool {
	return strings.EqualFold(bareOperator(op), "Bool") && strings.EqualFold(val, "true")
}

// narrowSourceRange compares aws:SourceIp as a range under IpAddress only,
// and a range broader than /8 (IPv4) or /32 (IPv6) counts as public.
func narrowSourceRange(op, val string) bool {
	broad, ok := rangeBroaderThan(val, 8, 32)
	return strings.EqualFold(bareOperator(op), "IpAddress") && ok && !broad
}

// restrictiveKeys are matched to a condition key case-insensitively, as IAM
// matches key names.
var restrictiveKeys = []scopingKey{
	{RestrictingKey{"aws:SourceAccount", citeS3Public}, fixedValue},
	{RestrictingKey{"aws:SourceOwner", citeS3Public}, fixedValue},
	{RestrictingKey{"aws:SourceArn", citeS3Public}, fixedValue},
	{RestrictingKey{"aws:SourceVpc", citeS3Public}, fixedValue},
	{RestrictingKey{"aws:SourceVpce", citeS3Public}, fixedValue},
	{RestrictingKey{"aws:SourceOrgID", citeGlobalKeys + "#condition-keys-sourceorgid"}, fixedValue},
	{RestrictingKey{"aws:SourceOrgPaths", citeGlobalKeys + "#condition-keys-sourceorgpaths"}, fixedOrgPath},
	{RestrictingKey{"aws:SourceIp", citeS3Public}, narrowSourceRange},
	{RestrictingKey{"aws:PrincipalAccount", citeS3Public}, fixedValue},
	{RestrictingKey{"aws:PrincipalArn", citeS3Public}, fixedValue},
	{RestrictingKey{"aws:PrincipalOrgID", citeS3Public}, fixedValue},
	{RestrictingKey{"aws:PrincipalOrgPaths", citeGlobalKeys + "#condition-keys-principalorgpaths"}, fixedOrgPath},
	{RestrictingKey{"aws:userid", citeS3Public}, fixedUserID},
	{RestrictingKey{"kms:CallerAccount", "https://docs.aws.amazon.com/kms/latest/developerguide/conditions-kms.html#conditions-kms-caller-account"}, fixedValue},
	{RestrictingKey{"elasticfilesystem:AccessedViaMountTarget", citeEFSPublic}, boolTrue},
}

// namesAccount is the confused-deputy test: the value names the account,
// organization or resource a service acts for. An ARN that carries an
// account names it by a 12-digit account, whatever its resource; any other
// value (an ID, an S3 bucket ARN) must be fixed.
func namesAccount(op, val string) bool {
	a, err := arn.Parse(val)
	if err != nil || a.AccountID == "" {
		return fixedValue(op, val)
	}
	return isAccountID(a.AccountID)
}

// sourceScopeKeys name the account or resource on whose behalf a service
// acts. They answer the confused-deputy question, which an address range
// does not: an AWS service calls a role from AWS-owned addresses whoever
// asked it to.
var sourceScopeKeys = []scopingKey{
	{RestrictingKey{"aws:SourceAccount", "https://docs.aws.amazon.com/IAM/latest/UserGuide/confused-deputy.html"}, namesAccount},
	{RestrictingKey{"aws:SourceArn", "https://docs.aws.amazon.com/IAM/latest/UserGuide/confused-deputy.html"}, namesAccount},
	{RestrictingKey{"aws:SourceOrgID", "https://docs.aws.amazon.com/IAM/latest/UserGuide/confused-deputy.html"}, namesAccount},
	{RestrictingKey{"aws:SourceOrgPaths", citeGlobalKeys + "#condition-keys-sourceorgpaths"}, fixedOrgPath},
}

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

// endpointRestrictiveKeys is restrictiveKeys plus the keys that name the
// resource being called. On a resource's own policy they hold for every
// caller of that resource and scope nobody; on a VPC endpoint policy they
// narrow which resources the endpoint reaches. Used by EvaluateEndpoint.
var endpointRestrictiveKeys = slices.Concat([]scopingKey{
	{RestrictingKey{"aws:ResourceAccount", citeGlobalKeys + "#condition-keys-resourceaccount"}, fixedValue},
	{RestrictingKey{"aws:ResourceOrgID", citeGlobalKeys + "#condition-keys-resourceorgid"}, fixedValue},
	{RestrictingKey{"aws:ResourceOrgPaths", citeGlobalKeys + "#condition-keys-resourceorgpaths"}, fixedOrgPath},
	{RestrictingKey{"s3:ResourceAccount", "https://docs.aws.amazon.com/AmazonS3/latest/userguide/amazon-s3-policy-keys.html#example-object-resource-account"}, fixedValue},
}, restrictiveKeys)

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
			i := slices.IndexFunc(keys, func(k scopingKey) bool { return strings.EqualFold(k.Key, key) })
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

// OpenCIDR reports whether c reaches every address of its family: a prefix
// of length zero, whatever base address is written in front of it, since a
// range matches on its first prefix-length bits only.
func OpenCIDR(c string) bool {
	open, _ := rangeBroaderThan(c, 1, 1)
	return open
}

// rangeBroaderThan reports whether c is a range with fewer than v4 prefix
// bits (IPv4) or v6 prefix bits (IPv6); ok is false when c is neither a
// range nor an address. A bare address is the range of that one address.
func rangeBroaderThan(c string, v4, v6 int) (broad, ok bool) {
	c = strings.TrimSpace(c)
	p, err := netip.ParsePrefix(c)
	if err != nil {
		a, aerr := netip.ParseAddr(c)
		if aerr != nil {
			return false, false
		}
		p = netip.PrefixFrom(a, a.BitLen())
	}
	if p.Addr().Is6() {
		return p.Bits() < v6, true
	}
	return p.Bits() < v4, true
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
