package unit_test

// w27_doc_quote_gate_test.go — task w27 row 1's other half: once the sentence
// lives on FindingDef, the doc-quote gate's oracle becomes catalog.All()
// Detail values instead of an AST walk of core/aws (a9s-dev round 0 log,
// "how it retires"). Same package as
// qa_networking_row_values_and_doc_quotes_test.go so this reuses its
// unexported helpers (netDocPath, netTypes, detailConstants, netDocQuotes,
// docQuoteMatches, docQuoteBurnDown) directly rather than duplicating them.
//
// Two things pinned, neither touching the shared docQuoteBurnDown map:
//
//  1. The target oracle (every FindingDef.Detail across the catalog) is
//     empty today — round 0 populated no definition — so with that oracle
//     ANY §4 quote fails to match, which is exactly the direction the
//     retirement wants: "a section 4 quote with no definition behind it
//     fails". This is the same fact TestDetailContract_FullCatalogDemoBench
//     pins from the emitter side; this test pins it from the doc side.
//  2. The current (AST-walk) oracle's per-type unmatched count, recomputed
//     fresh here, never exceeds the count docQuoteBurnDown carries — the
//     "may only shrink" direction the existing gate already enforces
//     (TestNetworkingDocQuotes_EqualADetailConstant's got > want branch),
//     confirmed independently rather than trusted from a single test.

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/catalog"
)

// targetOracleDetails is every non-empty FindingDef.Detail across the whole
// catalog — the oracle the doc-quote gate swaps to once docQuoteBurnDown is
// empty (a9s-dev round 0 log, "how it retires").
func targetOracleDetails(t *testing.T) map[string]bool {
	t.Helper()
	out := map[string]bool{}
	for _, td := range catalog.All() {
		for _, def := range td.Findings {
			if def.Detail != "" {
				out[def.Detail] = true
			}
		}
	}
	return out
}

// TestDocQuoteGate_TargetOracleEmptyTodayFailsEveryQuote pins that the
// end-state oracle (FindingDef.Detail) is empty right now, so every §4 quote
// on every page fails to match it — the direction row 1 and row 2 exist to
// fix, one type at a time.
func TestDocQuoteGate_TargetOracleEmptyTodayFailsEveryQuote(t *testing.T) {
	target := targetOracleDetails(t)
	if len(target) != 0 {
		t.Fatalf("targetOracleDetails() = %d declared sentence(s), want 0 — a FindingDef already declares a "+
			"Detail, so this test's premise (nothing declares one yet) is stale and must be narrowed to "+
			"the types that still have none", len(target))
	}

	// sg is a type whose emitter already carries a real Detail sentence
	// (sg.go:195, attached at :243), so its §4 page necessarily quotes
	// something today. With an empty target oracle, that quote cannot match.
	quotes := netDocQuotes(t, "sg")
	if len(quotes) == 0 {
		t.Fatal("docs/resources/sg.md quotes nothing; nothing to check the empty-oracle direction against")
	}
	for _, q := range quotes {
		if target[q] {
			t.Errorf("docs/resources/sg.md quote %q matched the (supposedly empty) target oracle", q)
		}
	}
}

// TestDocQuoteGate_UnmatchedNeverExceedsBurnDown independently recomputes
// every carried type's unmatched count against the real (AST-walk) oracle
// and confirms it never exceeds docQuoteBurnDown's entry — the "burn-down
// map may only shrink" direction, read-only.
func TestDocQuoteGate_UnmatchedNeverExceedsBurnDown(t *testing.T) {
	constants := detailConstants(t)
	for short, want := range docQuoteBurnDown {
		got := 0
		for _, quote := range netDocQuotes(t, short) {
			matched := false
			for constant := range constants {
				if docQuoteMatches(quote, constant) {
					matched = true
					break
				}
			}
			if !matched {
				got++
			}
		}
		if got > want {
			t.Errorf("docs/resources/%s.md now quotes %d unrenderable sentence(s), up from the %d docQuoteBurnDown "+
				"carries — the burn-down only shrinks", short, got, want)
		}
	}
}
