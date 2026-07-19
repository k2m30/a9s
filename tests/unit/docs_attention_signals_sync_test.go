package unit

// docs_attention_signals_sync_test.go pins two structural relationships
// between docs/attention-signals.md's machine-generated findings table
// (written by cmd/catalogen from the installed catalog's FindingDef
// declarations, between the "<!-- BEGIN GENERATED: findings-table -->" and
// "<!-- END GENERATED: findings-table -->" markers) and the hand-written
// per-type prose table above it (columns: shortName | Name | Wave 1 |
// Wave 2 | Wave 3 | Source):
//
//  1. Every resource type with at least one wave2 row in the generated
//     table must have a non-empty, non-"None" Wave 2 cell in its prose row
//     — the prose must acknowledge that wave-2 signals exist for that type.
//  2. Every resource type named in the generated table (any wave) must
//     have a row in the prose table — a new catalog type with declared
//     findings must gain a prose entry, not just a generated-table entry.
//
// This test does not attempt any semantic comparison of prose wording
// against finding phrases or severities — that comparison is prone to
// false positives on legitimate paraphrasing. It checks only the two
// structural invariants above, plus that the generated block still exists
// and parses to at least one row (guards against the block being deleted).
//
// It does not check that the generated block itself is fresh relative to
// the catalog: freshness is guarded by `make check-catalogen` (part of
// `make ready-to-push`), which re-runs cmd/catalogen against a clean tree
// and fails on any resulting diff.

import (
	"os"
	"strings"
	"testing"
)

const (
	genBeginMarker = "<!-- BEGIN GENERATED: findings-table -->"
	genEndMarker   = "<!-- END GENERATED: findings-table -->"
)

// splitMarkdownTableRow splits a single markdown table row line into its
// cells. It trims the line, strips one leading and one trailing "|", then
// splits on " | " (space-pipe-space) column boundaries rather than on bare
// "|" — so a cell containing an escaped or inline-code pipe that is not
// surrounded by spaces is not mistaken for a column boundary.
func splitMarkdownTableRow(line string) []string {
	trimmed := strings.TrimSpace(line)
	trimmed = strings.TrimPrefix(trimmed, "|")
	trimmed = strings.TrimSuffix(trimmed, "|")
	cells := strings.Split(trimmed, " | ")
	for i := range cells {
		cells[i] = strings.TrimSpace(cells[i])
	}
	return cells
}

func isMarkdownSeparatorRow(cells []string) bool {
	for _, cell := range cells {
		if cell == "" {
			return false
		}
		for _, r := range cell {
			if r != '-' && r != ':' {
				return false
			}
		}
	}
	return true
}

type generatedFindingRow struct {
	Type   string
	Source string
}

// parseGeneratedFindingsTable extracts the machine-generated findings table
// between the BEGIN/END markers and returns one row per data line (Type and
// Source columns only — Code/Phrase/Severity are not needed by either
// invariant this file checks).
func parseGeneratedFindingsTable(t *testing.T, docPath, content string) []generatedFindingRow {
	t.Helper()

	beginIdx := strings.Index(content, genBeginMarker)
	if beginIdx == -1 {
		t.Fatalf("%s: missing marker %q", docPath, genBeginMarker)
	}
	endIdx := strings.Index(content, genEndMarker)
	if endIdx == -1 {
		t.Fatalf("%s: missing marker %q", docPath, genEndMarker)
	}
	if endIdx < beginIdx {
		t.Fatalf("%s: %q appears before %q", docPath, genEndMarker, genBeginMarker)
	}

	block := content[beginIdx+len(genBeginMarker) : endIdx]

	var rows []generatedFindingRow
	for _, line := range strings.Split(block, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "|") {
			continue
		}
		cells := splitMarkdownTableRow(line)
		if len(cells) != 5 {
			continue
		}
		if isMarkdownSeparatorRow(cells) || cells[0] == "Type" {
			continue
		}
		rows = append(rows, generatedFindingRow{Type: cells[0], Source: cells[4]})
	}

	if len(rows) == 0 {
		t.Fatalf("%s: generated findings table between %q and %q parsed to zero rows",
			docPath, genBeginMarker, genEndMarker)
	}
	return rows
}

// parseProseSignalsTable extracts every hand-written per-type row from the
// "shortName | Name | Wave 1 | Wave 2 | Wave 3 | Source" tables above the
// generated block, keyed by shortName, with the value set to that row's
// Wave 2 cell. Rows are identified by their first cell being a
// backtick-wrapped shortName (e.g. "`ec2`"), which is how every per-type
// row is written and how these tables are distinguished from the unrelated
// "Visualization Surfaces" table earlier in the doc.
func parseProseSignalsTable(t *testing.T, docPath, content string) map[string]string {
	t.Helper()

	proseWave2ByType := make(map[string]string)
	for lineNo, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "| `") {
			continue
		}
		cells := splitMarkdownTableRow(trimmed)
		if len(cells) != 6 {
			continue
		}
		shortName := strings.Trim(cells[0], "`")
		if shortName == "" {
			continue
		}
		if _, dup := proseWave2ByType[shortName]; dup {
			t.Fatalf("%s:%d: duplicate prose row for shortName %q", docPath, lineNo+1, shortName)
		}
		proseWave2ByType[shortName] = cells[3]
	}

	if len(proseWave2ByType) == 0 {
		t.Fatalf("%s: prose signals table parsed to zero rows", docPath)
	}
	return proseWave2ByType
}

func isEmptyOrNone(cell string) bool {
	cell = strings.TrimSpace(cell)
	return cell == "" || strings.EqualFold(cell, "None")
}

func readAttentionSignalsDoc(t *testing.T) (docPath, content string) {
	t.Helper()
	docPath = "../../docs/attention-signals.md"
	data, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatalf("cannot read %s: %v", docPath, err)
	}
	return docPath, string(data)
}

// TestDocsAttentionSignalsSync_Wave2TypesHaveProseWave2Cell verifies that
// every resource type with at least one wave2 row in the generated
// findings table has a non-empty, non-"None" Wave 2 cell in its prose row.
func TestDocsAttentionSignalsSync_Wave2TypesHaveProseWave2Cell(t *testing.T) {
	docPath, content := readAttentionSignalsDoc(t)
	generatedRows := parseGeneratedFindingsTable(t, docPath, content)
	proseWave2ByType := parseProseSignalsTable(t, docPath, content)

	seen := make(map[string]bool)
	for _, row := range generatedRows {
		if row.Source != "wave2" || seen[row.Type] {
			continue
		}
		seen[row.Type] = true

		wave2Cell, ok := proseWave2ByType[row.Type]
		if !ok {
			t.Errorf("%q has wave2 findings in the generated table but no row at all in the prose table — "+
				"add a prose row for %q to docs/attention-signals.md", row.Type, row.Type)
			continue
		}
		if isEmptyOrNone(wave2Cell) {
			t.Errorf("%q has wave2 findings in the generated table but its prose row's Wave 2 cell is %q — "+
				"describe %q's wave-2 signal in docs/attention-signals.md's prose table", row.Type, wave2Cell, row.Type)
		}
	}
}

// TestDocsAttentionSignalsSync_GeneratedTypesHaveProseRows verifies that
// every resource type named in the generated findings table (any wave) has
// a row in the prose table above it — a new catalog type with declared
// findings must gain a prose entry.
func TestDocsAttentionSignalsSync_GeneratedTypesHaveProseRows(t *testing.T) {
	docPath, content := readAttentionSignalsDoc(t)
	generatedRows := parseGeneratedFindingsTable(t, docPath, content)
	proseWave2ByType := parseProseSignalsTable(t, docPath, content)

	seen := make(map[string]bool)
	for _, row := range generatedRows {
		if seen[row.Type] {
			continue
		}
		seen[row.Type] = true

		if _, ok := proseWave2ByType[row.Type]; !ok {
			t.Errorf("%q appears in the generated findings table but has no row in the prose table — "+
				"add a prose row for %q to docs/attention-signals.md", row.Type, row.Type)
		}
	}
}
