// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// views7_verbatim_reason_test.go — the ceiling the verbatim list never had.
//
// verbatimDetailPaths is the one place a raw AWS constant is allowed to reach
// a detail screen, and the only thing that makes it different from an
// allowlist is that every entry is a value a person types back: an access key,
// a role id, an API operation name. Nothing enforced that. A new constant
// could be silenced by writing a sentence beside it, and the list had no
// recorded size, so it could also grow without anyone reading the diff.
//
// Two pins close it: the list's size is written down, and every reason has to
// name the AWS field whose value is the identifier — in the field's own words,
// or by spelling the field. A reason that only says what the value is used for
// does not say which field must not be reworded.
package unit_test

import (
	"regexp"
	"sort"
	"strings"
	"testing"
)

// views7VerbatimPathCount is how many entries verbatimDetailPaths holds. A
// change that adds an entry raises it here, in the same diff, so a new
// exemption is a line a reviewer sees rather than one more key in a long map.
const views7VerbatimPathCount = 36

// views7FieldToken matches a token written the way AWS writes a field name:
// CamelCase with at least two words ("PolicyName"), or snake_case
// ("stop_code"). It is how a reason names a field that the path itself does
// not name — an Attention row carries a policy's name, not a field called
// "attention".
var views7FieldToken = regexp.MustCompile(`^([A-Z][a-z0-9]+){2,}$|^[a-z0-9]+(_[a-z0-9]+)+$`)

// views7Words splits prose into lowercase word tokens.
func views7Words(s string) []string {
	return strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9')
	})
}

// views7ReasonNamesTheField reports whether reason names the field at path:
// either it spells every word of the field's name in prose ("the CloudWatch
// metric's own name" for MetricName), or it writes a field name of its own
// ("PolicyName"). A token carrying a slash is a source location, not a field.
func views7ReasonNamesTheField(path, reason string) bool {
	for _, tok := range strings.Fields(reason) {
		tok = strings.Trim(tok, "(),.;:\"'")
		if strings.ContainsAny(tok, "/") || strings.HasSuffix(tok, ".go") {
			continue
		}
		if views7FieldToken.MatchString(tok) {
			return true
		}
	}

	said := map[string]bool{}
	for _, w := range views7Words(reason) {
		said[w] = true
	}
	field := path
	if _, after, ok := strings.Cut(path, "/"); ok {
		field = after
	}
	if i := strings.LastIndex(field, "."); i >= 0 {
		field = field[i+1:]
	}
	words := views7Words(views7SplitCamel(field))
	if len(words) == 0 {
		return false
	}
	for _, w := range words {
		if !said[w] {
			return false
		}
	}
	return true
}

// views7SplitCamel puts a space before each interior capital so a CamelCase
// field name splits into the words a sentence would use.
func views7SplitCamel(s string) string {
	var b strings.Builder
	runes := []rune(s)
	for i, r := range runes {
		if i > 0 && r >= 'A' && r <= 'Z' && !(runes[i-1] >= 'A' && runes[i-1] <= 'Z') {
			b.WriteByte(' ')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// TestVerbatimReasonNamesTheField holds every entry to the rule.
func TestVerbatimReasonNamesTheField(t *testing.T) {
	var offenders []string
	for path, reason := range verbatimDetailPaths {
		if !views7ReasonNamesTheField(path, reason) {
			offenders = append(offenders, path+": "+reason)
		}
	}
	sort.Strings(offenders)
	for _, o := range offenders {
		t.Errorf("a verbatim entry's reason does not name the field whose value must not be reworded: %s — "+
			"say which AWS field the value comes from, in its own words or by its name, so the next reader "+
			"can tell an identifier from a constant nobody got to", o)
	}
}

// TestVerbatimListHasARecordedSize is the ceiling. Without it the list is the
// one place in this suite where a raw constant can be admitted without anyone
// deciding to admit it.
func TestVerbatimListHasARecordedSize(t *testing.T) {
	if len(verbatimDetailPaths) != views7VerbatimPathCount {
		t.Errorf("verbatimDetailPaths holds %d entries and the recorded size is %d — a value that must "+
			"reach the screen exactly as AWS wrote it is a decision, so raise or lower the recorded size "+
			"in the same change and say in it which field it is",
			len(verbatimDetailPaths), views7VerbatimPathCount)
	}
}

// TestVerbatimReasonRuleTellsThemApart is the rule's own negative case: a
// gate that accepted any prose would pass the list silently.
func TestVerbatimReasonRuleTellsThemApart(t *testing.T) {
	cases := []struct {
		name   string
		path   string
		reason string
		want   bool
	}{
		{"field spelled in prose", "alarm/metric_name", "identifier: the CloudWatch metric's own name", true},
		{"camel field spelled in prose", "alarm/MetricName", "identifier: the CloudWatch metric's own name", true},
		{"field named outright", "role/Attention", "identifier: the attached policy's PolicyName", true},
		{"snake field named outright", "ecs-task/Attention", "identifier: the task's stop_code, which is what an operator searches for", true},
		{"only what the value is for", "ct-events/ENVELOPE.EventName", "identifier: the AWS API operation name", false},
		{"prose about the block, not the field", "ct-events/RAW EVENT.errorCode", "verbatim: the raw event view reproduces the event as AWS sent it", false},
		{"a source location is not a field", "ecs-task/Attention", "identifier by design (core/aws/ecs_task_issue_enrichment.go)", false},
		{"empty reason", "policy/PolicyName", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := views7ReasonNamesTheField(c.path, c.reason); got != c.want {
				t.Errorf("views7ReasonNamesTheField(%q, %q) = %v, want %v", c.path, c.reason, got, c.want)
			}
		})
	}
}
