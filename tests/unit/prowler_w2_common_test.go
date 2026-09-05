package unit

// prowler_w2_common_test.go — shared assertion helpers for the w2
// (DATABASES & STORAGE) Prowler gap-closure batch.
//
// Every helper here pins a contract that holds for all 29 rows of the batch:
// a finding is identified by its exact Code/Phrase/Severity/Source quadruple,
// carries a non-empty Detail sentence, and is declared as a catalog.FindingDef
// on its type's literal. Assertions are on literal strings, never on the
// production constants, so a silent rename of a code or a phrase is caught
// rather than followed.

import (
	"testing"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// w2Find returns the finding with the given code, or false.
func w2Find(fs []domain.Finding, code string) (domain.Finding, bool) {
	for _, f := range fs {
		if string(f.Code) == code {
			return f, true
		}
	}
	return domain.Finding{}, false
}

// w2AssertFinding pins the full identity of one finding: code, phrase,
// severity and source must all match, and Detail must carry the S5 operator
// sentence (rule 1 of the batch contract — an empty Detail leaves the detail
// view with nothing but the phrase it already shows in the list).
func w2AssertFinding(t *testing.T, fs []domain.Finding, code, phrase string, sev domain.Severity, source string) domain.Finding {
	t.Helper()
	f, ok := w2Find(fs, code)
	if !ok {
		t.Fatalf("no finding with code %q; got %s", code, w2Codes(fs))
	}
	if f.Phrase != phrase {
		t.Errorf("%s: Phrase = %q, want %q", code, f.Phrase, phrase)
	}
	if f.Severity != sev {
		t.Errorf("%s: Severity = %v, want %v", code, f.Severity, sev)
	}
	if f.Source != source {
		t.Errorf("%s: Source = %q, want %q", code, f.Source, source)
	}
	if f.Detail == "" {
		t.Errorf("%s: Detail is empty; every finding must carry an S5 operator sentence", code)
	}
	return f
}

// w2AssertNoCode is the negative half of every row: the healthy counterpart
// must emit nothing for that code.
func w2AssertNoCode(t *testing.T, fs []domain.Finding, code string) {
	t.Helper()
	if f, ok := w2Find(fs, code); ok {
		t.Errorf("unexpected finding %q (phrase %q) on a healthy resource", code, f.Phrase)
	}
}

func w2Codes(fs []domain.Finding) []string {
	out := make([]string, 0, len(fs))
	for _, f := range fs {
		out = append(out, string(f.Code))
	}
	return out
}

// w2AssertFindingDef pins the catalog declaration for an emitted code.
// A code with no FindingDef is invisible to the color/badge surfaces that
// read the catalog rather than the finding itself.
func w2AssertFindingDef(t *testing.T, shortName, code, phrase string, sev domain.Severity, source string) {
	t.Helper()
	def := catalog.Find(shortName)
	if def == nil {
		t.Fatalf("catalog.Find(%q) returned nil", shortName)
	}
	for _, fd := range def.Findings {
		if string(fd.Code) != code {
			continue
		}
		if fd.Phrase != phrase {
			t.Errorf("%s FindingDef %s: Phrase = %q, want %q", shortName, code, fd.Phrase, phrase)
		}
		if fd.Severity != sev {
			t.Errorf("%s FindingDef %s: Severity = %v, want %v", shortName, code, fd.Severity, sev)
		}
		if fd.Source != source {
			t.Errorf("%s FindingDef %s: Source = %q, want %q", shortName, code, fd.Source, source)
		}
		return
	}
	t.Errorf("%s: no catalog.FindingDef declared for emitted code %q", shortName, code)
}

// w2Enricher returns the Wave-2 enricher wired on the type's catalog literal.
// Driving the registered function (rather than the production symbol) proves
// the enricher is actually reachable in the app, not merely defined.
func w2Enricher(t *testing.T, shortName string) awsclient.IssueEnricherFunc {
	t.Helper()
	e, ok := awsclient.Wave2EnricherFor(shortName)
	if !ok || e.Fn == nil {
		t.Fatalf("no Wave2 enricher registered for %q", shortName)
	}
	return e.Fn
}

// w2Rows returns the AttentionDetail rows attached to one (resource, code)
// pair. Rows are keyed per finding code so two independent conditions on one
// resource keep their own supporting rows.
func w2Rows(t *testing.T, res awsclient.IssueEnricherResult, id, code string) []domain.DetailRow {
	t.Helper()
	byCode, ok := res.AttentionDetails[id]
	if !ok {
		t.Fatalf("no AttentionDetails for resource %q", id)
	}
	ad, ok := byCode[domain.FindingCode(code)]
	if !ok {
		t.Fatalf("no AttentionDetail rows for %q under code %q", id, code)
	}
	return ad.Rows
}

// w2AssertRow pins one Label: Value pair of an AttentionDetail.
func w2AssertRow(t *testing.T, rows []domain.DetailRow, label, value string) {
	t.Helper()
	for _, r := range rows {
		if r.Label == label {
			if r.Value != value {
				t.Errorf("row %q: Value = %q, want %q", label, r.Value, value)
			}
			return
		}
	}
	t.Errorf("no row labelled %q; got %v", label, rows)
}

// w2AssertEnricherInvariants pins the shape every Wave-2 result must have so
// the merge step downstream never has to nil-check.
func w2AssertEnricherInvariants(t *testing.T, res awsclient.IssueEnricherResult, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("enricher returned error: %v", err)
	}
	w2AssertEnricherShape(t, res)
}

// w2AssertEnricherShape is the map half of the invariants, for the cases that
// legitimately return a composite error: MarkSkipped records a genuine
// per-item failure and Finish folds it into one error, while the successfully
// inspected rows still carry their findings.
func w2AssertEnricherShape(t *testing.T, res awsclient.IssueEnricherResult) {
	t.Helper()
	if res.Findings == nil {
		t.Error("Findings map is nil")
	}
	if res.TruncatedIDs == nil {
		t.Error("TruncatedIDs map is nil")
	}
}

// w2Res builds the minimal resource shape a Wave-2 enricher consumes: an ID
// plus the RawStruct the fetcher retained.
func w2Res(id string, raw any) resource.Resource {
	return resource.Resource{
		ID:        id,
		Name:      id,
		Fields:    map[string]string{},
		RawStruct: raw,
	}
}
