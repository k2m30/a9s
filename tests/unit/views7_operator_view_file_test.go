// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// views7_operator_view_file_test.go — the file an operator edited.
//
// A view file is the one thing in a9s a person is invited to change, and the
// two ways they change a status column are renaming it and writing a key by
// hand. Both used to work by accident: the cell was a status cell if its title
// said Status, or if its key said "status" whatever the type calls its
// lifecycle key. Neither is true now, so a file that says either of those
// things silently stops having a status column — no phrase, no colour, and a
// blank cell where the key matches nothing.
//
// The migration owns the first case: a key this build moved is a key the build
// moves for the operator. Nothing owns the second, so the load says it.
package unit_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	_ "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/config"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// views7WriteViewFile writes one view file into a views directory.
func views7WriteViewFile(t *testing.T, dir, shortName, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, shortName+".yaml"), []byte(body), 0o600); err != nil {
		t.Fatalf("writing %s.yaml: %v", shortName, err)
	}
}

// views7LifecycleKey is the key a type's status cell reads.
func views7LifecycleKey(td *resource.ResourceTypeDef) string {
	if td.LifecycleKey != "" {
		return td.LifecycleKey
	}
	return "state"
}

// views7ResolvedColumn returns the resolved column with the given title.
func views7ResolvedColumn(t *testing.T, cfg *config.ViewsConfig, shortName, title string) config.ListColumn {
	t.Helper()
	td := resource.FindResourceType(shortName)
	if td == nil {
		t.Fatalf("no resource type %q", shortName)
	}
	for _, col := range resource.ResolveListColumnCascade(cfg, shortName, td) {
		if col.Title == title {
			return col
		}
	}
	t.Fatalf("%s has no resolved column titled %q", shortName, title)
	return config.ListColumn{}
}

// TestOperatorsHandTypedStatusKeyMigratesToTheLifecycleKey drives the file an
// operator wrote against the build that read "status" literally: they renamed
// the column and typed the key the old build honoured. The key moved when the
// column list moved, and a key this build moved is one the build moves for
// them — not one they are asked to fix.
func TestOperatorsHandTypedStatusKeyMigratesToTheLifecycleKey(t *testing.T) {
	dir := t.TempDir()
	views7WriteViewFile(t, dir, "tgw", `generated: 6
list:
  Name:
    key: name
    width: 28
  Health:
    key: status
    width: 12
  Owner:
    key: owner_id
    width: 14

detail:
  - TransitGatewayId
`)

	if err := config.EnsureViewsDir(dir); err != nil {
		t.Fatalf("EnsureViewsDir: %v", err)
	}
	cfg, err := config.LoadFromDirs([]string{dir})
	if err != nil {
		t.Fatalf("LoadFromDirs: %v", err)
	}
	if cfg == nil {
		t.Fatal("LoadFromDirs returned no config for a directory holding one file")
	}

	td := resource.FindResourceType("tgw")
	lifecycleKey := views7LifecycleKey(td)

	renamed := views7ResolvedColumn(t, cfg, "tgw", "Health")
	if renamed.Key != lifecycleKey {
		t.Errorf("the operator's renamed status column resolves with key %q, want %q — they typed the "+
			"key the build before this one honoured, and this build moved it",
			renamed.Key, lifecycleKey)
	}

	// The column is only migrated if it still reads as the status column: the
	// phrase, the colour and the widening all follow from that.
	row := resource.Resource{
		ID:     "tgw-0123456789abcdef0",
		Name:   "acme-prod-tgw",
		Fields: map[string]string{"name": "acme-prod-tgw", "state": "available", "owner_id": "123456789012"},
		Findings: []domain.Finding{{
			Code: "tgw-attachment-failed", Phrase: "attachment failed",
			Severity: domain.SevBroken, Source: "wave1",
		}},
	}
	col := app.ColumnDef{Key: renamed.Key, Title: renamed.Title, Width: renamed.Width, Path: renamed.Path}
	if got := app.ExtractCellValue(col, td, row); got != "attachment failed" {
		t.Errorf("the renamed column shows %q, want %q — a column the migration moved is the type's "+
			"status column, and a status column shows what the row is flagged for", got, "attachment failed")
	}

	// The negative half: only the key this build moved is moved. A column the
	// operator keyed themselves, on a fact that is not the status, is theirs.
	owner := views7ResolvedColumn(t, cfg, "tgw", "Owner")
	if owner.Key != "owner_id" {
		t.Errorf("the operator's Owner column resolves with key %q, want %q — the migration moves the "+
			"key it moved, not every key it does not recognise", owner.Key, "owner_id")
	}
}

// TestUnknownColumnKeyIsReportedAtLoad pins the other half. A key that names
// nothing on the type renders an empty cell forever, and the file is the one
// place a9s asks a person to edit — so the load says which file and which key,
// once, and keeps the rest of their file.
//
// The channel is config.Load's error, which the TUI already turns into a
// flash ("Config error: ... (using defaults)", internal/tui/app.go:214) and
// which is the only report a view file has today. If dev routes it somewhere
// else that reaches the operator, the assertion moves with it.
func TestUnknownColumnKeyIsReportedAtLoad(t *testing.T) {
	const unknownKey = "att_stats"
	dir := t.TempDir()
	views7WriteViewFile(t, dir, "vpce", `generated: 6
list:
  Endpoint ID:
    key: vpce_id
    width: 26
  Attachments:
    key: `+unknownKey+`
    width: 12

detail:
  - VpcEndpointId
`)

	cfg, err := config.LoadFromDirs([]string{dir})

	if cfg == nil || len(cfg.Views["vpce"].List) != 2 {
		t.Fatalf("the load returned %v for a file with two columns — a key nobody can fill is one "+
			"column to report, not a reason to drop the operator's whole file", cfg)
	}
	if err == nil {
		t.Fatalf("loading a view file whose column names %q, which no fetcher or enricher on vpce "+
			"writes, reported nothing — the cell is blank on every row and the file is the one place "+
			"an operator is invited to edit", unknownKey)
	}
	report := err.Error()
	if !strings.Contains(report, "vpce.yaml") || !strings.Contains(report, unknownKey) {
		t.Errorf("the load reported %q — it names the file and the key, so the operator can find the "+
			"line", report)
	}
	if n := strings.Count(report, unknownKey); n != 1 {
		t.Errorf("the load names %q %d times in one report; once is what a person reads", unknownKey, n)
	}

	// The negative half: a file whose keys all name something reports nothing.
	clean := t.TempDir()
	views7WriteViewFile(t, clean, "vpce", `generated: 6
list:
  Endpoint ID:
    key: vpce_id
    width: 26

detail:
  - VpcEndpointId
`)
	if _, cleanErr := config.LoadFromDirs([]string{clean}); cleanErr != nil {
		t.Errorf("a view file whose every key names a field on the type reported %v", cleanErr)
	}
}
