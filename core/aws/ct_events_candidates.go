// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import "strings"

// This file holds the candidate readings of the values a CloudTrail event
// names. A candidate is never a related ID by itself: ctEventsMatchTarget
// keeps only a candidate equal to a loaded target row's ID, name or ARN.

// ctIDAlternatives returns the candidate ids for one value an event named,
// derived from what the event wrote and never from the ARN's fixed grammar.
// An ARN contributes the id inside its resource part — everything after the
// type word, as one string — and then itself, so no segment of the grammar and
// no type word can answer for a resource. Anything else contributes itself. A
// fragment of a name is not a form of it.
func ctIDAlternatives(v string) []string {
	if parts := strings.SplitN(v, ":", 6); strings.HasPrefix(v, "arn:") && len(parts) == 6 {
		// The whole ARN is offered last, behind the resource-part forms: a
		// list that carries its rows' own ARNs can answer it exactly, which
		// is the only way to confirm an id whose ARN carries a suffix the
		// name does not. Last because the callers that read a group
		// positionally (cfnStackNameFromResourceName, ctLambdaAlternatives)
		// read the resource part at [0] and its stripped form at [1].
		return append(ctStripTypeWord(parts[5], "/:"), v)
	}
	// Not an ARN: the value IS the id. An event may still write the resource
	// part alone ("instance/i-abc"), so the type-word strip is offered behind
	// it — on "/" only, because a trailing ":qualifier" on a bare name is
	// Lambda's business and its head is the function.
	return ctStripTypeWord(v, "/")
}

// ctStripTypeWord returns res and the form inside it, most specific first:
// res as written, then without its leading "<type><sep>"
// ("instance/i-abc" → "i-abc", "secret:prod/api/key" → "prod/api/key").
// Nothing shorter: a fragment of a name belongs to a different resource.
func ctStripTypeWord(res, seps string) []string {
	out := []string{res}
	i := strings.IndexAny(res, seps)
	if i < 0 || i >= len(res)-1 {
		return out
	}
	return append(out, res[i+1:])
}

// ctLambdaAlternatives is ctIDAlternatives plus the Lambda-only rule: a
// function may be named with a trailing ":<alias>" qualifier, so the head
// before it is a candidate after the as-written form.
func ctLambdaAlternatives(group []string) []string {
	// The type word is what precedes the first separator of the widest
	// candidate; a head that equals it is grammar, not a function name.
	typeWord := ""
	if len(group) > 1 {
		if i := strings.IndexAny(group[0], "/:"); i > 0 && group[0][i+1:] == group[1] {
			typeWord = group[0][:i]
		}
	}
	out := group
	for _, c := range group {
		if i := strings.LastIndex(c, ":"); i > 0 && c[:i] != typeWord {
			out = append(out, c[:i])
		}
	}
	return out
}

// cfnStackNameFromResourceName extracts the stack NAME from a CloudTrail
// CloudFormation resource name. Stack ARNs are ".../stack/<name>/<uuid>"; the
// generic last-segment trim (extractCTResourceIDs) would keep the uuid, but cfn
// resources are keyed by stack name. A bare name (no "stack/" segment) passes
// through unchanged.
func cfnStackNameFromResourceName(group []string) []string {
	s := group[0]
	if len(group) > 1 {
		s = group[1] // the id after the "stack/" type word: "<name>/<uuid>"
	}
	name, _, _ := strings.Cut(s, "/")
	return []string{name}
}
