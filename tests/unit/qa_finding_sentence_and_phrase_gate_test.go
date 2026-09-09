// qa_finding_sentence_and_phrase_gate_test.go — three standing rules over the
// FindingDef catalog, and the demo pins that hold their premises to the
// rendered surfaces.
//
// RULE 1 (every issue-tier finding carries an operator sentence). A finding at
// an issue tier reaches the detail Attention block (S3) and the enrichment
// line (S5) per docs/attention-signals.md § "Wave → surface mapping". Those
// two surfaces exist to tell an operator what the condition means and what to
// do about it. A code that renders its phrase alone leaves the operator with a
// two-word status and no next step, and the generated docs cell reads "—", so
// there is nowhere else to look either. The sentence is declared once, on the
// FindingDef, because catalogen generates the docs from that same field —
// a second place to declare it is a second place to let it drift.
//
// RULE 2 (one phrase, one severity, one code). The catalog and the generated
// signal tables are read by a human deciding what a colour means. Two rows
// carrying the same phrase at two severities say that the same words are
// sometimes yellow and sometimes red, which makes the colour unreadable; two
// rows carrying the same phrase under two codes say the two conditions are
// indistinguishable to whoever reads the table. Either way the phrase has
// stopped identifying the condition, which is the only job it has.
//
// RULE 3 (a Detail exists exactly where a surface shows it). The Attention
// block skips a finding that is not an issue severity, so a Dim finding's
// sentence is written, generated into the docs, and shown to nobody. The
// contract in docs/attention-signals.md § "Wave → surface mapping" puts Dim on
// S2 and S4 only — colour and status cell — and says in as many words that a
// Dim finding reaches neither S3 nor S5. Under that contract a Dim sentence
// has no reader, so declaring one is declaring a fact the product does not
// hold: the sentence belongs deleted, not surfaced.
//
// None of the three carries an allowlist. A rule that a surface either obeys
// or does not has nothing to burn down.
package unit_test

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// sentenceGateEntry is one FindingDef with the type that declares it, so a
// failure names the file a reader has to open.
type sentenceGateEntry struct {
	shortName string
	def       catalog.FindingDef
}

// allCatalogFindingDefs flattens every registered top-level and child type's
// findings table. Child types declare findings the same way parents do and
// render them through the same detail surface, so a rule that held only for
// parents would leave every child list unchecked.
func allCatalogFindingDefs(t *testing.T) []sentenceGateEntry {
	t.Helper()
	var out []sentenceGateEntry
	for _, td := range append(catalog.All(), catalog.AllChildren()...) {
		for _, f := range td.Findings {
			out = append(out, sentenceGateEntry{shortName: td.ShortName, def: f})
		}
	}
	if len(out) < 300 {
		t.Fatalf("only %d declared finding defs; the gate is not seeing the catalog "+
			"(TestMain must have called aws.Install())", len(out))
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].shortName != out[j].shortName {
			return out[i].shortName < out[j].shortName
		}
		return out[i].def.Code < out[j].def.Code
	})
	return out
}

// wholePlaceholderPhrase matches a phrase that is nothing but one substitution
// token — "<status, in words>", "<health check reason>". Such a phrase is not
// a phrase an operator ever reads: the emitter replaces the whole thing with
// the resource's own word. Rule 2 still applies to it, because the generated
// signal table prints the token verbatim and a reader of that table sees the
// same words twice; the constant is named here so a failure message can say
// which kind of duplicate it found.
var wholePlaceholderPhrase = regexp.MustCompile(`^<[^<>]*>$`)

// minOperatorSentenceLen is the shortest string that can carry both halves of
// an operator sentence — what the condition means, and what to do about it.
// A one-word Detail ("Broken.", or the phrase repeated) satisfies "non-empty"
// while telling the operator nothing, and 268 cells to fill is exactly the
// situation where that shortcut gets taken.
const minOperatorSentenceLen = 40

func TestEveryIssueTierFindingCarriesAnOperatorSentence(t *testing.T) {
	var missing, tooShort, restatesPhrase []string

	for _, e := range allCatalogFindingDefs(t) {
		if !e.def.Severity.IsIssue() {
			continue
		}
		where := fmt.Sprintf("%s %s (%v) phrase=%q", e.shortName, e.def.Code, e.def.Severity, e.def.Phrase)
		detail := strings.TrimSpace(e.def.Detail)
		switch {
		case detail == "":
			missing = append(missing, where)
		case len(detail) < minOperatorSentenceLen:
			tooShort = append(tooShort, fmt.Sprintf("%s: Detail=%q (%d chars)", where, detail, len(detail)))
		case strings.EqualFold(detail, e.def.Phrase),
			strings.EqualFold(strings.TrimSuffix(detail, "."), e.def.Phrase):
			restatesPhrase = append(restatesPhrase, fmt.Sprintf("%s: Detail=%q", where, detail))
		}
	}

	if len(missing) > 0 {
		t.Errorf("%d issue-tier finding codes declare no Detail sentence, so their "+
			"detail Attention entry renders the phrase alone and the generated docs "+
			"cell reads \"—\":\n  %s", len(missing), strings.Join(missing, "\n  "))
	}
	if len(tooShort) > 0 {
		t.Errorf("%d issue-tier Detail sentences are shorter than %d characters — a "+
			"sentence says what the condition means for the operator AND what to do "+
			"about it:\n  %s", len(tooShort), minOperatorSentenceLen, strings.Join(tooShort, "\n  "))
	}
	if len(restatesPhrase) > 0 {
		t.Errorf("%d Detail sentences restate their own phrase, so the Attention entry "+
			"says the same words twice:\n  %s", len(restatesPhrase), strings.Join(restatesPhrase, "\n  "))
	}
}

func TestOneFindingPhraseCarriesOneSeverityAndOneCode(t *testing.T) {
	entries := allCatalogFindingDefs(t)

	// Clause A — a phrase pattern maps to one severity, catalog-wide. The
	// generated signal tables are one document; a reader meeting the same words
	// under two colours in it cannot learn what the colour means.
	bySeverity := map[string]map[domain.Severity][]string{}
	// Clause B — within one resource type, a phrase belongs to one code. Two
	// codes sharing a phrase render identically in that type's list, so the
	// operator cannot tell which condition fired and the docs table repeats a
	// row. Scoped per type because the same lifecycle word legitimately names
	// different conditions on different services.
	byTypeAndPhrase := map[string][]sentenceGateEntry{}

	for _, e := range entries {
		if bySeverity[e.def.Phrase] == nil {
			bySeverity[e.def.Phrase] = map[domain.Severity][]string{}
		}
		owner := fmt.Sprintf("%s/%s", e.shortName, e.def.Code)
		bySeverity[e.def.Phrase][e.def.Severity] = append(bySeverity[e.def.Phrase][e.def.Severity], owner)
		key := e.shortName + "\x00" + e.def.Phrase
		byTypeAndPhrase[key] = append(byTypeAndPhrase[key], e)
	}

	var multiSeverity []string
	for phrase, bySev := range bySeverity {
		if len(bySev) < 2 {
			continue
		}
		var parts []string
		for sev, owners := range bySev {
			sort.Strings(owners)
			parts = append(parts, fmt.Sprintf("%v=%s", sev, strings.Join(owners, ",")))
		}
		sort.Strings(parts)
		multiSeverity = append(multiSeverity, fmt.Sprintf("%q -> %s", phrase, strings.Join(parts, " | ")))
	}
	sort.Strings(multiSeverity)

	var multiCode []string
	for key, group := range byTypeAndPhrase {
		if len(group) < 2 {
			continue
		}
		codes := make([]string, 0, len(group))
		for _, e := range group {
			codes = append(codes, fmt.Sprintf("%s(%v)", e.def.Code, e.def.Severity))
		}
		sort.Strings(codes)
		shortName, phrase, _ := strings.Cut(key, "\x00")
		kind := "literal"
		if wholePlaceholderPhrase.MatchString(phrase) {
			kind = "placeholder"
		}
		multiCode = append(multiCode, fmt.Sprintf("%s %q [%s] -> %s", shortName, phrase, kind, strings.Join(codes, ", ")))
	}
	sort.Strings(multiCode)

	if len(multiSeverity) > 0 {
		t.Errorf("%d finding phrases are declared at more than one severity, so the "+
			"same words carry more than one colour:\n  %s",
			len(multiSeverity), strings.Join(multiSeverity, "\n  "))
	}
	if len(multiCode) > 0 {
		t.Errorf("%d (type, phrase) pairs are shared by more than one finding code, so "+
			"two different conditions render as the same words on the same list:\n  %s",
			len(multiCode), strings.Join(multiCode, "\n  "))
	}
}

func TestAFindingDetailExistsOnlyAtATierASurfaceShows(t *testing.T) {
	var unreachable []string
	for _, e := range allCatalogFindingDefs(t) {
		if strings.TrimSpace(e.def.Detail) == "" {
			continue
		}
		if e.def.Severity.IsIssue() {
			continue
		}
		unreachable = append(unreachable, fmt.Sprintf(
			"%s %s (%v) phrase=%q Detail=%q",
			e.shortName, e.def.Code, e.def.Severity, e.def.Phrase, e.def.Detail))
	}
	if len(unreachable) > 0 {
		t.Errorf("%d finding codes declare a Detail sentence at a tier no surface "+
			"renders — the detail Attention block skips a finding that is not an "+
			"issue severity, and docs/attention-signals.md puts Dim on S2 and S4 "+
			"only. Delete the sentence, or the contract has to change first:\n  %s",
			len(unreachable), strings.Join(unreachable, "\n  "))
	}
}

// demoCFDisabledID and demoACMInactiveID are the two rows the batch names: a
// distribution switched off by an administrator, and an imported certificate
// nothing is serving. Both are terminal-but-fine states, which is what Dim
// means, and both declare a sentence today.
const (
	demoCFDisabledID   = "E3C4D5E6F7G8H9"
	demoACMInactiveID  = "arn:aws:acm:us-east-1:123456789012:certificate/c9d0e1f2-3456-78ab-cdef-999999999999"
	demoCFDisabledCode = domain.FindingCode("cf.disabled")
	demoACMInactiveCod = domain.FindingCode("acm.status.inactive")
)

// TestTheDimSentenceReachesNoRenderedSurface holds Rule 3's premise to the
// running app rather than to a reading of detail_fields.go: it drives the real
// list body and the real detail body for the two named demo rows and shows
// that the status cell is the only place their state appears. If a later
// change starts rendering Dim findings in the Attention block, this pin fails
// and Rule 3 is the thing to revisit — deleting the sentences is only correct
// while the surface set stays as the contract describes it.
func TestTheDimSentenceReachesNoRenderedSurface(t *testing.T) {
	byType, _ := buildVisibilityTypeCache(t)

	cases := []struct {
		shortName  string
		resourceID string
		code       domain.FindingCode
		wantCell   string
	}{
		{"cf", demoCFDisabledID, demoCFDisabledCode, "disabled (admin-off)"},
		{"acm", demoACMInactiveID, demoACMInactiveCod, "inactive"},
	}

	for _, tc := range cases {
		t.Run(tc.shortName, func(t *testing.T) {
			rows := byType[tc.shortName]
			if len(rows) == 0 {
				t.Fatalf("%s: demo bench produced no rows", tc.shortName)
			}
			var row resource.Resource
			var found bool
			for _, r := range rows {
				if r.ID == tc.resourceID {
					row, found = r, true
					break
				}
			}
			if !found {
				t.Fatalf("%s: demo bench has no row %q; the fixture this pin names is gone",
					tc.shortName, tc.resourceID)
			}

			var finding domain.Finding
			for _, f := range row.Findings {
				if f.Code == tc.code {
					finding = f
					break
				}
			}
			if finding.Code != tc.code {
				t.Fatalf("%s %s: row carries no %s finding", tc.shortName, tc.resourceID, tc.code)
			}
			if finding.Severity != domain.SevDim {
				t.Fatalf("%s %s: %s is %v, not Dim — this pin is about the Dim tier",
					tc.shortName, tc.resourceID, tc.code, finding.Severity)
			}

			td := catalog.FindAny(tc.shortName)
			if td == nil {
				t.Fatalf("%s: not a registered type", tc.shortName)
			}
			cell, ok := listStatusCellFor(t, *td, rows, tc.resourceID)
			if !ok {
				t.Fatalf("%s %s: no status cell rendered", tc.shortName, tc.resourceID)
			}
			if cell != tc.wantCell {
				t.Errorf("%s %s status cell = %q, want %q — S4 is the surface a Dim "+
					"finding does reach, and it carries the phrase",
					tc.shortName, tc.resourceID, cell, tc.wantCell)
			}

			for _, v := range detailAttentionValuesFor(t, row, tc.shortName) {
				if strings.Contains(v, tc.wantCell) || (finding.Detail != "" && strings.Contains(v, finding.Detail)) {
					t.Errorf("%s %s: the detail Attention block renders %q for a Dim "+
						"finding; docs/attention-signals.md puts Dim on S2 and S4 only",
						tc.shortName, tc.resourceID, v)
				}
			}

			if finding.Detail != "" {
				t.Errorf("%s %s: %s emits Detail=%q, and neither the Attention block "+
					"nor the status cell renders it — the sentence has no reader",
					tc.shortName, tc.resourceID, tc.code, finding.Detail)
			}
		})
	}
}

// splitPhraseRows are the four rows the batch splits: two Redshift states that
// both read "modifying", and two ACM expiry tiers that both read "expires in
// <N day(s)>". After the split each phrase has to say which condition it is —
// what is being modified, and how soon the certificate goes.
var splitPhraseRows = []struct {
	shortName string
	code      domain.FindingCode
	wantSev   domain.Severity
	// forbidden is the phrase the code carries today, which after the split
	// must belong to at most one of the pair — the ambiguity is the whole
	// defect, so keeping it on both is not a split.
	forbidden string
}{
	{"redshift", "redshift.warn.modifying", domain.SevWarn, "modifying"},
	{"redshift", "redshift.warn.availability_modifying", domain.SevWarn, "modifying"},
	{"acm", "acm.expires-critical", domain.SevBroken, "expires in <N day(s)>"},
	{"acm", "acm.expires-soon", domain.SevWarn, "expires in <N day(s)>"},
}

func TestTheSplitPhrasesNameTheirDistinction(t *testing.T) {
	seen := map[string][]string{}
	for _, r := range splitPhraseRows {
		phrase := catalog.Phrase(r.code)
		if phrase == "" {
			t.Fatalf("%s: catalog declares no phrase for %s", r.shortName, r.code)
		}
		if got := catalog.Severity(r.code); got != r.wantSev {
			t.Errorf("%s severity = %v, want %v — the split changes the words, not the "+
				"colour", r.code, got, r.wantSev)
		}
		seen[phrase] = append(seen[phrase], string(r.code))
	}

	for phrase, codes := range seen {
		if len(codes) > 1 {
			sort.Strings(codes)
			t.Errorf("phrase %q is still shared by %s — a reader of the list or the "+
				"signal table cannot tell the two conditions apart", phrase, strings.Join(codes, " and "))
		}
	}

	// "modifying" alone does not say what is modifying, and the two Redshift
	// codes exist precisely because two different things can be. Neither may
	// keep the bare word.
	for _, r := range splitPhraseRows[:2] {
		if catalog.Phrase(r.code) == r.forbidden {
			t.Errorf("%s still reads %q, which does not say what is being modified",
				r.code, r.forbidden)
		}
	}

	// The two Redshift rows are on the demo bench, so the split is checkable on
	// the surface an operator reads rather than only in the catalog.
	td := catalog.FindAny("redshift")
	if td == nil {
		t.Fatal("redshift is not a registered type")
	}
	byType, _ := buildVisibilityTypeCache(t)
	rows := byType["redshift"]
	cellA, okA := listStatusCellFor(t, *td, rows, "redshift-modifying")
	cellB, okB := listStatusCellFor(t, *td, rows, "redshift-avail-modifying")
	if !okA || !okB {
		t.Fatalf("redshift demo bench is missing one of the two modifying rows "+
			"(redshift-modifying ok=%v, redshift-avail-modifying ok=%v)", okA, okB)
	}
	if cellA == cellB {
		t.Errorf("both redshift modifying rows render the status cell %q, so the list "+
			"shows one condition twice", cellA)
	}
}
