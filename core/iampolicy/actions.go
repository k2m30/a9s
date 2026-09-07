// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package iampolicy

import (
	"slices"
	"strings"
)

// AllowsAction reports whether the document's Allow statements grant
// action and no Deny statement removes it. Resources and Conditions are
// ignored, as Prowler does for privilege escalation.
func (d Document) AllowsAction(action string) bool {
	allowed := false
	for _, st := range d.Statement {
		if !st.covers(action) {
			continue
		}
		switch st.Effect {
		case "Deny":
			return false
		case "Allow":
			allowed = true
		}
	}
	return allowed
}

func (st Statement) covers(action string) bool {
	if len(st.NotAction) > 0 {
		return !matchesAny(st.NotAction, action)
	}
	return matchesAny(st.Action, action)
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

// IsAdmin reports an unconditioned Allow of every action on every resource.
func (d Document) IsAdmin() bool {
	for _, st := range d.Statement {
		if st.Effect == "Allow" && len(st.Condition) == 0 &&
			(slices.Contains(st.Action, "*") || slices.Contains(st.Action, "*:*")) &&
			slices.Contains(st.Resource, "*") {
			return true
		}
	}
	return false
}

// confusedDeputyServices are the service principals that act on behalf of a
// caller in another account and therefore need aws:SourceAccount /
// aws:SourceArn scoping on a trust policy.
var confusedDeputyServices = []string{
	"cloudformation", "cloudtrail", "cloudwatch", "codebuild", "codepipeline", "config", "ec2", "events",
	"lambda", "logs", "s3", "sns", "sqs", "ssm", "states", "backup", "glue", "apigateway", "eks", "ecs",
	"elasticmapreduce", "firehose", "kinesis", "opensearchservice", "es", "transfer", "ses", "iot", "rds",
}

var sourceScopeKeys = []string{"aws:SourceAccount", "aws:SourceArn", "aws:SourceOrgID"}

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
			if slices.Contains(confusedDeputyServices, prefix) {
				found[prefix] = struct{}{}
			}
		}
	}
	return sortedKeys(found)
}
