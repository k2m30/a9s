package unit

// Assertion helpers shared by the CI/CD, data and container posture tests.
//
// The per-type tests pin Code, Phrase, Severity, Source and a non-empty
// Detail through pw1RequireFinding. These helpers pin a supporting row by the
// fact it carries rather than by its label: several rows exist to name a
// CIDR range, a Kubernetes support status or a buildspec path, and the label
// wording is free. Pinning the value alone fails when the fact goes missing
// and stays silent when the label is reworded.

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// w6bRequireRowValue fails unless some supporting row carries value. The label
// is deliberately not asserted: see the file comment.
func w6bRequireRowValue(t *testing.T, rows []domain.DetailRow, value string) domain.DetailRow {
	t.Helper()
	for _, r := range rows {
		if r.Value == value {
			return r
		}
	}
	t.Fatalf("no supporting row carries the value %q (rows: %+v)", value, rows)
	return domain.DetailRow{}
}

// w6bRequireRowValueContains fails unless some supporting row's value mentions
// substr. Used where the row exists to name the setting an operator edits and
// the surrounding phrasing is free.
func w6bRequireRowValueContains(t *testing.T, rows []domain.DetailRow, substr string) {
	t.Helper()
	for _, r := range rows {
		if strings.Contains(r.Label+" "+r.Value, substr) {
			return
		}
	}
	t.Fatalf("no supporting row mentions %q (rows: %+v)", substr, rows)
}

// w6bRequireNoRows pins a finding whose phrase is the whole fact.
//
// The phrase and the row render one line apart, so "Termination protection:
// off" printed under "termination protection off" is one fact stated twice.
// For those findings the correct number of supporting rows is zero, and that
// is worth pinning per finding rather than only in the demo-wide sweep,
// because the sweep cannot see a row that paraphrases instead of repeating.
func w6bRequireNoRows(t *testing.T, rows []domain.DetailRow) {
	t.Helper()
	if len(rows) > 0 {
		t.Errorf("finding carries %d supporting row(s) %+v; its phrase already states the whole fact", len(rows), rows)
	}
}

// w6bWave1Rows returns the rows a wave-1 fetcher attached to a resource.
func w6bWave1Rows(r resource.Resource, code domain.FindingCode) []domain.DetailRow {
	return r.AttentionDetails[code].Rows
}

// w6bRequireNoRawEnum fails when an SDK enum spelling reaches a surface a9s
// draws. AWS writes its states in SCREAMING_SNAKE (EXTENDED_SUPPORT,
// PUBLIC_READ, MUTABLE); those reach a phrase, a detail sentence or a row only
// by handing string(<SDK enum>) straight through, and an operator does not
// speak them.
func w6bRequireNoRawEnum(t *testing.T, f domain.Finding, rows []domain.DetailRow) {
	t.Helper()
	surfaces := []struct{ where, text string }{{"Phrase", f.Phrase}, {"Detail", f.Detail}}
	for _, r := range rows {
		surfaces = append(surfaces,
			struct{ where, text string }{"row label", r.Label},
			struct{ where, text string }{"row " + r.Label, r.Value})
	}
	for _, s := range surfaces {
		where, text := s.where, s.text
		for _, word := range strings.Fields(text) {
			word = strings.Trim(word, ".,;:()\"'")
			if !strings.ContainsRune(word, '_') {
				continue
			}
			if word == strings.ToUpper(word) && word != strings.ToLower(word) {
				t.Errorf("%s carries the SDK enum spelling %q: %q", where, word, text)
			}
		}
	}
}

// w6bRequireNoSecretLeak fails when a credential value reaches any rendered
// surface of a finding: rows carry Where and Kind, never the value.
func w6bRequireNoSecretLeak(t *testing.T, f domain.Finding, rows []domain.DetailRow, secret string) {
	t.Helper()
	if strings.Contains(f.Phrase+" "+f.Detail, secret) {
		t.Errorf("credential value leaked into the finding text: %q / %q", f.Phrase, f.Detail)
	}
	for _, r := range rows {
		if strings.Contains(r.Label+" "+r.Value, secret) {
			t.Errorf("credential value leaked into a supporting row: %+v", r)
		}
	}
}
