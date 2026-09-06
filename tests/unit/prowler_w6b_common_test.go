package unit

// prowler_w6b_common_test.go — assertion helpers shared by the batch-w6b
// (CI/CD, DATA, CONTAINERS) behavioural tests.
//
// The per-row tests pin Code, Phrase, Severity, Source and a non-empty Detail
// through pw1RequireFinding, which every Prowler batch already shares. What
// w6b adds is a way to pin a supporting row by the fact it carries rather
// than by the label someone chose for it: several rows in this batch exist to
// name a CIDR range, a Kubernetes support status or a buildspec path, and the
// rendered-surface rulings deliberately leave the label wording free. Pinning
// the value alone fails when the fact goes missing and stays silent when the
// label is reworded, which is the split those rulings ask for.

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
// the surrounding phrasing is the implementer's to choose.
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
// A row that renormalises to its own phrase is the U11 defect: the phrase and
// the row render one line apart, so "Termination protection: off" printed
// under "termination protection off" is one fact stated twice. For those
// findings the correct number of supporting rows is zero, and that is a
// property worth pinning per row rather than only in the batch-wide sweep,
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

// w6bRequireNoSecretLeak fails when a credential value reaches any rendered
// surface of a finding. Rule 7 of the batch contract: rows carry Where and
// Kind, never the value.
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
