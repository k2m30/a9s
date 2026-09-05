package unit

// prowler_w4_common_test.go — assertion helpers shared by the batch-w4
// (SECURITY & SECRETS) behavioural tests.
//
// Every helper here pins a contract that holds for every finding in the
// batch: a finding is identified by its exact code string, and its Phrase,
// Severity and Source are part of the contract rather than incidental text.
// Findings are looked up by code, never by slice position, because two
// independently-evaluated conditions on one resource may be appended in
// either order.

import (
	"sort"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// w4FindingByCode returns the single finding carrying code. It fails the
// test when the same code is emitted twice: one condition must produce one
// finding, never a duplicate pair that would double-count the issue badge.
func w4FindingByCode(t *testing.T, fs []domain.Finding, code domain.FindingCode) (domain.Finding, bool) {
	t.Helper()
	var found domain.Finding
	n := 0
	for _, f := range fs {
		if f.Code == code {
			found = f
			n++
		}
	}
	if n > 1 {
		t.Fatalf("code %q emitted %d times, want at most 1", code, n)
	}
	return found, n == 1
}

// w4AssertFinding pins the four contract fields of one finding plus the
// non-empty S5 Detail sentence every batch-w4 finding must stamp.
func w4AssertFinding(
	t *testing.T,
	fs []domain.Finding,
	code domain.FindingCode,
	wantPhrase string,
	wantSeverity domain.Severity,
	wantSource string,
) domain.Finding {
	t.Helper()
	f, ok := w4FindingByCode(t, fs, code)
	if !ok {
		t.Fatalf("no finding with code %q; got %s", code, w4CodesOf(fs))
	}
	if f.Phrase != wantPhrase {
		t.Errorf("Phrase = %q, want %q", f.Phrase, wantPhrase)
	}
	if f.Severity != wantSeverity {
		t.Errorf("Severity = %v, want %v", f.Severity, wantSeverity)
	}
	if f.Source != wantSource {
		t.Errorf("Source = %q, want %q", f.Source, wantSource)
	}
	if strings.TrimSpace(f.Detail) == "" {
		t.Errorf("Detail is empty; every batch-w4 finding stamps an operator sentence")
	}
	return f
}

// w4AssertNoCode asserts the healthy counterpart emits nothing for code.
func w4AssertNoCode(t *testing.T, fs []domain.Finding, code domain.FindingCode) {
	t.Helper()
	if _, ok := w4FindingByCode(t, fs, code); ok {
		t.Fatalf("unexpected finding %q on a healthy resource; got %s", code, w4CodesOf(fs))
	}
}

// w4CodesOf renders the emitted codes for failure messages.
func w4CodesOf(fs []domain.Finding) string {
	if len(fs) == 0 {
		return "no findings"
	}
	out := make([]string, 0, len(fs))
	for _, f := range fs {
		out = append(out, string(f.Code))
	}
	sort.Strings(out)
	return strings.Join(out, ", ")
}

// w4AssertRows pins the AttentionDetail rows of one finding: exactly the
// wanted label/value pairs, in order. Rows are the detail-view evidence for
// the phrase, so an empty or reordered set changes what the operator reads.
func w4AssertRows(
	t *testing.T,
	details map[domain.FindingCode]domain.AttentionDetail,
	code domain.FindingCode,
	want []domain.DetailRow,
) {
	t.Helper()
	ad, ok := details[code]
	if !ok {
		t.Fatalf("no AttentionDetail for code %q", code)
	}
	if len(ad.Rows) != len(want) {
		t.Fatalf("AttentionDetail[%q].Rows = %+v, want %+v", code, ad.Rows, want)
	}
	for i, w := range want {
		if ad.Rows[i].Label != w.Label || ad.Rows[i].Value != w.Value {
			t.Errorf("row %d = %q: %q, want %q: %q", i, ad.Rows[i].Label, ad.Rows[i].Value, w.Label, w.Value)
		}
	}
}

// w4FindingDef returns the catalog FindingDef declared for code on the type
// registered as shortName. A code emitted without a FindingDef has no
// registry row, so the menu badge and the docs table never learn about it.
func w4FindingDef(t *testing.T, shortName string, code domain.FindingCode) catalog.FindingDef {
	t.Helper()
	td := resource.FindResourceType(shortName)
	if td == nil {
		t.Fatalf("resource type %q not registered", shortName)
	}
	for _, d := range td.Findings {
		if d.Code == code {
			return d
		}
	}
	t.Fatalf("no catalog.FindingDef for %q on type %q", code, shortName)
	return catalog.FindingDef{}
}
