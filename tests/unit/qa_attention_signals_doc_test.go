package unit

// qa_attention_signals_doc_test.go — enforces docs/attention-signals.md as the
// golden contract for Wave 1 / Wave 2 attention signals against the live registry.
//
// Every row in the doc's markdown tables is parsed and three assertions are made:
//
//  1. Resource type exists — resource.FindResourceType(shortName) must return non-nil.
//  2. Wave 1 non-empty → Color func non-nil.
//  3. Wave 2 non-empty → awsclient.IssueEnricherRegistry[shortName] must be non-nil.
//
// Plus one table-level guard:
//
//  4. At least 50 rows were parsed (regression against a silent parse failure).
//
// When this test fails it means either:
//   - A resource type listed in the doc was not registered, or
//   - A Wave 1 signal is documented but the type has no Color func, or
//   - A Wave 2 signal is documented but no enricher is registered.
//
// Fix the code (or the doc), never soften the assertions.

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// attentionSignalsDocPath locates docs/attention-signals.md by walking upward
// from the test file's directory until go.mod is found, then resolving
// docs/attention-signals.md from that root. This avoids runtime.Caller.
func attentionSignalsDocPath(t *testing.T) string {
	t.Helper()
	// Start from the directory containing this test file.
	// __file__ is not available at runtime, so we resolve relative to the
	// working directory set by `go test` when running from the module root,
	// or walk upward from the current working directory.
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd failed: %v", err)
	}
	// Walk upward until we find go.mod (repo root).
	root := ""
	for {
		if _, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil {
			root = dir
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root without finding go.mod.
			break
		}
		dir = parent
	}
	if root == "" {
		t.Fatalf("could not locate go.mod walking upward from working directory — cannot find repo root")
	}
	p := filepath.Join(root, "docs", "attention-signals.md")
	if _, statErr := os.Stat(p); statErr != nil {
		t.Fatalf("golden doc not found at %s: %v", p, statErr)
	}
	return p
}

// attentionSignalRow holds parsed values from a single data row in the doc.
type attentionSignalRow struct {
	ShortName string // without backticks
	Wave1     string // trimmed cell text
	Wave2     string // trimmed cell text
}

// isNoneCell returns true when the cell value represents "no signal" per the doc spec:
// empty string, "None", or starting with "None —" / "None -" / "None—".
func isNoneCell(cell string) bool {
	cell = strings.TrimSpace(cell)
	if cell == "" || cell == "None" {
		return true
	}
	// Covers "None — ..." with em-dash (U+2014), en-dash, or ASCII double-dash.
	return strings.HasPrefix(cell, "None \u2014") || // em-dash with space
		strings.HasPrefix(cell, "None \u2013") || // en-dash with space
		strings.HasPrefix(cell, "None -") || // ASCII dash with space
		strings.HasPrefix(cell, "None\u2014") || // em-dash without space
		strings.HasPrefix(cell, "None\u2013") || // en-dash without space
		strings.HasPrefix(cell, "None-") // ASCII dash without space
}

// parseAttentionSignalsDoc reads the markdown file and returns all data rows.
// It skips header rows and separator rows.
func parseAttentionSignalsDoc(path string) ([]attentionSignalRow, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var rows []attentionSignalRow
	for rawLine := range bytes.SplitSeq(data, []byte("\n")) {
		line := bytes.TrimSpace(rawLine)
		// Data rows start with "| `" (pipe then space then backtick).
		if !bytes.HasPrefix(line, []byte("| `")) {
			continue
		}
		// Split on "|" — expect at least 7 separators for a 6-column table.
		// Example: | `ec2` | EC2 Instances | wave1 | wave2 | wave3 | source |
		parts := strings.Split(string(line), "|")
		// parts[0] is empty (before leading |), parts[len-1] may be empty (after trailing |).
		// Minimum meaningful parts: ["", "`ec2`", "name", "w1", "w2", "w3", "source", ""]
		if len(parts) < 7 {
			continue
		}
		shortNameCell := strings.TrimSpace(parts[1])
		// Must be wrapped in backticks: `shortName`
		if !strings.HasPrefix(shortNameCell, "`") || !strings.HasSuffix(shortNameCell, "`") {
			continue
		}
		shortName := shortNameCell[1 : len(shortNameCell)-1]
		if shortName == "" {
			continue
		}
		// Skip the header row whose shortName column is "shortName".
		if shortName == "shortName" {
			continue
		}

		wave1 := strings.TrimSpace(parts[3]) // column index 2 after leading empty
		wave2 := strings.TrimSpace(parts[4]) // column index 3

		rows = append(rows, attentionSignalRow{
			ShortName: shortName,
			Wave1:     wave1,
			Wave2:     wave2,
		})
	}
	return rows, nil
}

// TestAttentionSignalsDoc enforces the attention-signals.md golden contract
// against the live resource registry and enricher registry.
//
// TODO(no-middle-state): this is intentionally a registration/wiring guard, not
// a completeness proof. Passing here does not mean the documented signal is
// populated, surfaced in the UI, or tested under partial-data semantics.
func TestAttentionSignalsDoc(t *testing.T) {
	docPath := attentionSignalsDocPath(t)
	rows, err := parseAttentionSignalsDoc(docPath)
	if err != nil {
		t.Fatalf("failed to parse %s: %v", docPath, err)
	}

	// Guard #4: at least 50 rows parsed (catches silent parse regression).
	const minRows = 50
	if len(rows) < minRows {
		t.Fatalf("only %d rows parsed from %s — expected at least %d; parse may be broken", len(rows), docPath, minRows)
	}

	for _, row := range rows {
		t.Run(row.ShortName, func(t *testing.T) {
			// Assertion 1: resource type must be registered.
			rt := resource.FindResourceType(row.ShortName)
			if rt == nil {
				t.Errorf("docs list %q but not registered via ResourceTypeDef", row.ShortName)
				// Cannot proceed with further checks.
				return
			}

			// Assertion 2: Wave 1 non-empty → Color func non-nil.
			// (AlwaysHealthy field has been removed; every type must classify via Color.)
			if !isNoneCell(row.Wave1) {
				if rt.Color == nil {
					t.Errorf("docs Wave 1 signal for %q but ResourceTypeDef.Color is nil", row.ShortName)
				}
			}

			// Assertion 3: Wave 2 non-empty → registered enricher (catalog
			// Wave2 or legacy IssueEnricherRegistry, per aws.GetIssueEnricher).
			// Post-AS-726 PR-04i, messaging Wave 2 enrichers live on the
			// catalog row; un-migrated categories still use the legacy map.
			if !isNoneCell(row.Wave2) {
				if _, ok := awsclient.GetIssueEnricher(row.ShortName); !ok {
					t.Errorf("docs Wave 2 signal for %q but no Wave 2 enricher registered (neither catalog Wave2 nor IssueEnricherRegistry)", row.ShortName)
				}
			}
		})
	}
}
