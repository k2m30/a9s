// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package iampolicy

import (
	"slices"
	"strings"
)

// AllowsAction reports whether the document's Allow statements grant
// action and no unconditional Deny statement removes it. Resources and the
// Allow's conditions are ignored, as Prowler does for privilege escalation;
// a conditioned Deny is one the caller may satisfy, so it removes nothing.
func (d Document) AllowsAction(action string) bool {
	allowed := false
	for _, st := range d.Statement {
		if !st.covers(action) {
			continue
		}
		switch {
		case st.Effect == "Deny" && len(st.Condition) == 0:
			return false
		case st.Effect == "Allow":
			allowed = true
		}
	}
	return allowed
}

// covers reports whether st's Action or NotAction reaches action, which may
// itself be a pattern ("iam:*"). An Allow NotAction is read as covering
// concrete actions only, since a pattern spans actions the list may exclude;
// a Deny NotAction covers every action no listed entry overlaps.
func (st Statement) covers(action string) bool {
	switch {
	case len(st.NotAction) == 0:
		return matchesAny(st.Action, action)
	case st.Effect == "Deny":
		return !slices.ContainsFunc(st.NotAction, func(p string) bool { return overlaps(p, action) })
	default:
		return !strings.ContainsAny(action, "*?") && !matchesAny(st.NotAction, action)
	}
}

// overlaps reports whether two IAM action patterns match at least one action
// in common. A table over the two patterns rather than recursion, for the
// same reason Match avoids backtracking.
func overlaps(a, b string) bool {
	a, b = strings.ToLower(strings.TrimSpace(a)), strings.ToLower(strings.TrimSpace(b))
	// dp[i][j]: a[i:] and b[j:] match a common string.
	dp := make([][]bool, len(a)+1)
	for i := range dp {
		dp[i] = make([]bool, len(b)+1)
	}
	dp[len(a)][len(b)] = true
	for i := len(a); i >= 0; i-- {
		for j := len(b); j >= 0; j-- {
			switch {
			case i == len(a) && j == len(b):
			case i < len(a) && a[i] == '*':
				dp[i][j] = dp[i+1][j] || (j < len(b) && dp[i][j+1])
			case j < len(b) && b[j] == '*':
				dp[i][j] = dp[i][j+1] || (i < len(a) && dp[i+1][j])
			case i < len(a) && j < len(b):
				dp[i][j] = (a[i] == b[j] || a[i] == '?' || b[j] == '?') && dp[i+1][j+1]
			}
		}
	}
	return dp[0][0]
}

func matchesAny(patterns []string, action string) bool {
	return slices.ContainsFunc(patterns, func(p string) bool { return Match(p, action) })
}

// Match implements IAM wildcard matching: '*' spans any run, '?' exactly one
// character, everything else is literal, case-insensitive. A two-pointer
// scan rather than a translated regexp: '*'-heavy patterns come from the
// account being inspected and must not be able to backtrack.
func Match(pattern, value string) bool {
	p := strings.ToLower(strings.TrimSpace(pattern))
	v := strings.ToLower(value)
	pi, vi := 0, 0
	star, mark := -1, 0
	for vi < len(v) {
		switch {
		case pi < len(p) && (p[pi] == '?' || p[pi] == v[vi]):
			pi++
			vi++
		case pi < len(p) && p[pi] == '*':
			star, mark = pi, vi
			pi++
		case star >= 0:
			pi = star + 1
			mark++
			vi = mark
		default:
			return false
		}
	}
	for pi < len(p) && p[pi] == '*' {
		pi++
	}
	return pi == len(p)
}

// PrivilegeEscalation returns the sorted names of every known
// privilege-escalation combination whose every required action the document
// allows.
func (d Document) PrivilegeEscalation() []string {
	var out []string
	for name, required := range privEscCombos {
		if !slices.ContainsFunc(required, func(a string) bool { return !d.AllowsAction(a) }) {
			out = append(out, name)
		}
	}
	slices.Sort(out)
	return out
}

// IsAdmin reports an unconditioned Allow of every action on every resource
// that no unconditional Deny narrows: a Deny that takes any action away
// leaves less than every action.
func (d Document) IsAdmin() bool {
	admin := false
	for _, st := range d.Statement {
		switch {
		case st.Effect == "Deny" && len(st.Condition) == 0 && st.deniesAny():
			return false
		case st.Effect == "Allow" && len(st.Condition) == 0 &&
			(slices.Contains(st.Action, "*") || slices.Contains(st.Action, "*:*")) &&
			slices.Contains(st.Resource, "*"):
			admin = true
		}
	}
	return admin
}

// deniesAny reports whether a Deny statement names at least one action: an
// Action list always does, a NotAction list does unless it excepts "*".
func (st Statement) deniesAny() bool {
	if len(st.NotAction) > 0 {
		return !slices.ContainsFunc(st.NotAction, func(p string) bool { return Match(p, "*") })
	}
	return len(st.Action) > 0
}

// confusedDeputyServices are the service principals that act on behalf of a
// caller in another account and therefore need aws:SourceAccount /
// aws:SourceArn scoping on a trust policy.
var confusedDeputyServices = []string{
	"cloudformation", "cloudtrail", "cloudwatch", "codebuild", "codepipeline", "config", "ec2", "events",
	"lambda", "logs", "s3", "sns", "sqs", "ssm", "states", "backup", "glue", "apigateway", "eks", "ecs",
	"elasticmapreduce", "firehose", "kinesis", "opensearchservice", "es", "transfer", "ses", "iot", "rds",
}

// HasServicePrincipalWithoutSourceScope returns the sorted confused-deputy
// service prefixes trusted by Allow statements that carry no source scoping
// condition.
func (d Document) HasServicePrincipalWithoutSourceScope() []string {
	found := map[string]struct{}{}
	for _, st := range d.Statement {
		if st.Effect != "Allow" || isRestrictive(st.Condition, sourceScopeKeys) {
			continue
		}
		for _, svc := range st.Principal.Service {
			prefix, _, _ := strings.Cut(strings.ToLower(svc), ".")
			if slices.Contains(confusedDeputyServices, prefix) && d.grants(st, Principal{Kind: "Service", Value: svc}) {
				found[prefix] = struct{}{}
			}
		}
	}
	return sortedKeys(found)
}
