// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import "testing"

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
		{"one slot", "expires in <N> days", []string{"7"}, "expires in 7 days"},
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
