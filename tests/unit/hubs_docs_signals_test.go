package unit

// hubs_docs_signals_test.go pins that docs/attention-signals.md carries the
// per-type signal rows in exactly one generated block. A hand-written signal
// table is a second copy of the catalog that every batch has to edit by hand,
// which is both a merge hub and a place the page can silently disagree with
// the code.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
)

const (
	signalsBegin = "<!-- BEGIN GENERATED: signals -->"
	signalsEnd   = "<!-- END GENERATED: signals -->"
)

// attentionSignalsDoc returns the page split into the generated signals block
// and everything outside it.
func attentionSignalsDoc(t *testing.T) (generated, handWritten string) {
	t.Helper()
	path := filepath.Join(projectRoot(t), "docs", "attention-signals.md")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content := string(raw)
	begin := strings.Index(content, signalsBegin)
	end := strings.Index(content, signalsEnd)
	if begin == -1 || end == -1 || end < begin {
		t.Fatalf("docs/attention-signals.md has no generated %q block (begin=%d end=%d)", "signals", begin, end)
	}
	return content[begin+len(signalsBegin) : end], content[:begin] + content[end+len(signalsEnd):]
}

// TestAttentionSignals_GeneratedBlockCarriesEveryFindingDef pins that the one
// generated block is the whole signal inventory: every registered FindingDef
// of every type has a row there, under the column set the page promises.
func TestAttentionSignals_GeneratedBlockCarriesEveryFindingDef(t *testing.T) {
	generated, _ := attentionSignalsDoc(t)

	const header = "| shortName | Name | Wave | Code | Phrase | Severity | Detail |"
	if !strings.Contains(generated, header) {
		t.Errorf("generated signals block has no table header %q", header)
	}

	rows := strings.Split(generated, "\n")
	for _, td := range resource.AllResourceTypes() {
		for _, fd := range td.Findings {
			cell := fmt.Sprintf("| `%s` |", td.ShortName)
			found := false
			for _, row := range rows {
				if strings.HasPrefix(strings.TrimSpace(row), cell) &&
					strings.Contains(row, string(fd.Code)) &&
					strings.Contains(row, fd.Phrase) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("generated signals block has no row for %s finding %q (phrase %q)",
					td.ShortName, fd.Code, fd.Phrase)
			}
		}
	}
}

// TestAttentionSignals_NoSecondSignalTable pins that no signal row survives
// outside the generated block: neither the older findings-table block, which
// generates the same FindingDefs a second time, nor a hand-written per-type
// table. Both are places a batch would have to edit by hand, and both can
// disagree with the catalog without any gate noticing.
func TestAttentionSignals_NoSecondSignalTable(t *testing.T) {
	_, handWritten := attentionSignalsDoc(t)

	if strings.Contains(handWritten, "GENERATED: findings-table") {
		t.Error("docs/attention-signals.md still carries the findings-table block — " +
			"the signals block generates the same FindingDefs, so the page states them twice")
	}

	shortNames := make([]string, 0)
	for _, td := range resource.AllResourceTypes() {
		shortNames = append(shortNames, td.ShortName)
	}
	for _, line := range strings.Split(handWritten, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "|") {
			continue
		}
		for _, sn := range shortNames {
			if strings.HasPrefix(trimmed, "| `"+sn+"` |") {
				t.Errorf("hand-written signal row for %q outside the generated block:\n%s", sn, trimmed)
			}
		}
	}
}

// TestAttentionSignals_NotYetImplementedStaysHandWritten pins the other half:
// the Wave 3 ideas the old tables carried are not FindingDefs and cannot be
// generated, so deleting the hand-written tables must not delete them. They
// live in a hand-written section the generator does not touch.
func TestAttentionSignals_NotYetImplementedStaysHandWritten(t *testing.T) {
	generated, handWritten := attentionSignalsDoc(t)

	if !strings.Contains(strings.ToLower(handWritten), "not yet implemented") {
		t.Error("docs/attention-signals.md has no hand-written \"Not yet implemented\" section — " +
			"the Wave 3 signals the deleted tables listed have nowhere to live")
	}
	if strings.Contains(strings.ToLower(generated), "not yet implemented") {
		t.Error("the \"Not yet implemented\" list is inside the generated block, where catalogen overwrites it")
	}
}

// TestAttentionSignals_CoveredByCheckCatalogen pins that the page the
// generator writes is one the freshness gate reads back: without it a catalog
// change could land with the page stale and every gate green.
func TestAttentionSignals_CoveredByCheckCatalogen(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(projectRoot(t), "Makefile"))
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if !strings.HasPrefix(line, "CATALOGEN_DOCS") {
			continue
		}
		if !strings.Contains(line, "docs/attention-signals.md") {
			t.Errorf("check-catalogen does not cover docs/attention-signals.md:\n%s", line)
		}
		return
	}
	t.Error("no CATALOGEN_DOCS assignment found in the Makefile")
}
