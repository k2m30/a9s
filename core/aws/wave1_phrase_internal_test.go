// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/catalog"
)

// TestFillPhraseSubstitutesEachSlotOnce pins the property the whole Wave-1
// phrase seam rests on: the declared wording is the shape, and each value
// lands in exactly one slot. The angle-bracket case is not hypothetical —
// every value here comes from AWS (a status keyword, an error code, a shard
// id), and a value carrying a "<" would otherwise be re-scanned as the next
// slot's opening bracket and eat the text after it.
func TestFillPhraseSubstitutesEachSlotOnce(t *testing.T) {
	cases := []struct {
		name     string
		declared string
		values   []string
		want     string
	}{
		{"one slot", "expires in <N day(s)>", []string{"7"}, "expires in 7 days"},
		{"two slots", "<N> of <M> instances in service", []string{"2", "5"}, "2 of 5 instances in service"},
		{"whole phrase is the slot", "<status>", []string{"copying"}, "copying"},
		{"no slot, value dropped", "not logging", []string{"ignored"}, "not logging"},
		{"no values", "<N> of <M>", nil, "<N> of <M>"},
		{"fewer values than slots", "<N> of <M>", []string{"2"}, "2 of <M>"},
		{"value carrying an angle bracket", "shard <NodeGroupId>: <status>",
			[]string{"shard-<1>", "modifying"}, "shard shard-<1>: modifying"},
		{"empty declaration", "", []string{"x"}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := fillPhrase(tc.declared, tc.values...); got != tc.want {
				t.Errorf("fillPhrase(%q, %v) = %q, want %q", tc.declared, tc.values, got, tc.want)
			}
		})
	}
}

// TestFindingSlotRefusesAnEmptyValue pins spec row 2 (task aws3): a slot
// filled with an empty value renders a sentence with a hole in it
// ("failed: "). The filler refuses it rather than each emitter guarding.
func TestFindingSlotRefusesAnEmptyValue(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("fillPhrase with an empty value did not panic — an empty slot renders a sentence with a hole in it")
		}
	}()
	fillPhrase("failed: <error>", "")
}

// TestSlotPluralsComeFromTheDeclaration pins spec row 3 (task aws3): number
// agreement is the filler's, read off the declared phrase, so an emit site
// passes the number or the list and never a noun it inflected itself.
func TestSlotPluralsComeFromTheDeclaration(t *testing.T) {
	for _, tc := range []struct{ phrase, value, want string }{
		// A counted noun: the value is the count.
		{"expires in <N day(s)>", "1", "expires in 1 day"},
		{"expires in <N day(s)>", "2", "expires in 2 days"},
		{"expires in <N day(s)>", "0", "expires in 0 days"},
		{"<N job(s)> failed", "1", "1 job failed"},
		{"<N job(s)> failed", "3", "3 jobs failed"},
		// A listed noun: the value is the list, and its length agrees.
		{"<port(s) LIST> in the clear", "80", "port 80 in the clear"},
		{"<port(s) LIST> in the clear", "80, 8080", "ports 80, 8080 in the clear"},
		{"weak TLS policy on <port(s) LIST>", "443", "weak TLS policy on port 443"},
		{"weak TLS policy on <port(s) LIST>", "443, 8443", "weak TLS policy on ports 443, 8443"},
		// A slot with neither token is the whole value, unchanged.
		{"failed: <error>", "AccessDenied", "failed: AccessDenied"},
		{"<N> of <M> unhealthy", "2", "2 of <M> unhealthy"},
	} {
		if got := fillPhrase(tc.phrase, tc.value); got != tc.want {
			t.Errorf("fillPhrase(%q, %q) = %q, want %q", tc.phrase, tc.value, got, tc.want)
		}
	}
}

// TestSlotWithAnAgreeingNounNeverEatsItsValue pins what a probe found: a slot
// that marks a noun for agreement but names no token for the value has no
// place to put it, and the value used to vanish into the inflected noun
// ("<port(s)>" with "80" rendered "ports"). The value is what the reader is
// being shown; it survives whatever the declaration got wrong.
func TestSlotWithAnAgreeingNounNeverEatsItsValue(t *testing.T) {
	if got := fillPhrase("<port(s)>", "80"); !strings.Contains(got, "80") {
		t.Errorf("fillPhrase(\"<port(s)>\", \"80\") = %q — the value was dropped", got)
	}
}

// TestEveryAgreeingSlotNamesItsToken is the declaration-side half: a slot that
// marks a noun must say where the value lands, or the reader gets an inflected
// noun and no number.
func TestEveryAgreeingSlotNamesItsToken(t *testing.T) {
	for _, td := range catalog.All() {
		for _, def := range td.Findings {
			rest := def.Phrase
			for {
				open := strings.Index(rest, "<")
				if open < 0 {
					break
				}
				closeAt := strings.Index(rest[open:], ">")
				if closeAt < 0 {
					break
				}
				slot := rest[open+1 : open+closeAt]
				rest = rest[open+closeAt+1:]
				if !strings.Contains(slot, pluralMarker) {
					continue
				}
				if !strings.Contains(slot, slotCountToken) && !strings.Contains(slot, slotListToken) {
					t.Errorf("%s declares slot <%s>: it agrees a noun but names neither %s nor %s, so the value has nowhere to land",
						def.Code, slot, slotCountToken, slotListToken)
				}
			}
		}
	}
}

// TestDeclaredPhrasesCarryNoBareCountedNoun pins the other half of row 3: a
// declaration that puts a bare plural noun next to a count slot cannot agree,
// so the emit site would have to. Scanning the registered wordings is what
// keeps the rule from being re-broken by the next declaration.
func TestDeclaredPhrasesCarryNoBareCountedNoun(t *testing.T) {
	for _, td := range catalog.All() {
		for _, def := range td.Findings {
			for _, noun := range []string{"<N> days", "<N> jobs", "<N> ports", "<N> hours", "<N> targets"} {
				if strings.Contains(def.Phrase, noun) {
					t.Errorf("%s declares %q — a counted noun beside a bare <N> cannot agree; declare it inside the slot as <N %s(s)>",
						def.Code, def.Phrase, strings.TrimSuffix(strings.TrimPrefix(noun, "<N> "), "s"))
				}
			}
		}
	}
}
