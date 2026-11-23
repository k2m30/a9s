// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// The file an operator edited. A view file is the one thing in a9s a person is
// invited to change, and a column key that names nothing on the type renders
// a blank cell on every row, so the load says it.
package unit_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/config"
	"github.com/k2m30/a9s/v3/core/resource"
)

// views7WriteViewFile writes one view file into a views directory.
func views7WriteViewFile(t *testing.T, dir, shortName, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, shortName+".yaml"), []byte(body), 0o600); err != nil {
		t.Fatalf("writing %s.yaml: %v", shortName, err)
	}
}

// views7TypeOf resolves a view name to its type, parent or child — the two
// registries a view file can name.
func views7TypeOf(t *testing.T, shortName string) *resource.ResourceTypeDef {
	t.Helper()
	if td := resource.FindResourceType(shortName); td != nil {
		return td
	}
	if child := resource.GetChildType(shortName); child != nil {
		return child
	}
	t.Fatalf("no resource type %q", shortName)
	return nil
}

// views7ResolvedColumn returns the resolved column with the given title.
func views7ResolvedColumn(t *testing.T, cfg *config.ViewsConfig, shortName, title string) config.ListColumn {
	t.Helper()
	td := views7TypeOf(t, shortName)
	for _, col := range resource.ResolveListColumnCascade(cfg, shortName, td) {
		if col.Title == title {
			return col
		}
	}
	t.Fatalf("%s has no resolved column titled %q", shortName, title)
	return config.ListColumn{}
}

// TestUnknownColumnKeyIsReportedAtLoad pins the other half. A key that names
// nothing on the type renders an empty cell forever, and the file is the one
// place a9s asks a person to edit — so the load says which file and which key,
// once, and keeps the rest of their file.
//
// The channel is config.Load's error, which the TUI turns into a flash
// ("Config error: ... (using defaults)").
func TestUnknownColumnKeyIsReportedAtLoad(t *testing.T) {
	const unknownKey = "att_stats"
	dir := t.TempDir()
	views7WriteViewFile(t, dir, "vpce", `list:
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
	views7WriteViewFile(t, clean, "vpce", `list:
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
