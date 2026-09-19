package unit

// Every column names where its cell comes from, and a named Fields key has
// something that writes it.
//
// The columns a type declares are the columns its view renders, for every
// parent and child type (TestDefaultConfigColumnsAreTheCatalogs), so one walk
// over the catalog's columns covers the built-in views too.

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	_ "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/config"
	"github.com/k2m30/a9s/v3/core/resource"
)

// columnKeyProducerAllowlist documents column keys that INTENTIONALLY have no
// registered producer (computed on-the-fly, display-only cosmetics, etc.).
// Add entries with a justification comment.
//
// Format: map[shortName]map[colKey]justification
//
// For each allowlist entry, the test still verifies the column exists on that
// type — a stale allowlist entry (column was removed) is itself a test failure.
var columnKeyProducerAllowlist = map[string]map[string]string{
	// "<shortName>": {"<colKey>": "justification"},
}

// The built-in defaults are the columns shipped to end users; a default view
// Key referencing a Resource.Fields entry nobody writes renders blank forever.
//
// Intentional gaps go in columnKeyProducerAllowlist with a justification; do
// not weaken the assertion.
func TestColumnKeysHaveProducers(t *testing.T) {
	cfg := config.DefaultConfig()
	if len(cfg.Views) == 0 {
		t.Fatal("config.DefaultConfig().Views is empty — defaults_*.go init may not have run")
	}
	if len(resource.AllResourceTypes()) == 0 {
		t.Fatal("AllResourceTypes() returned zero entries — AWS init() may not have run")
	}

	for shortName, view := range cfg.Views {
		shortName, view := shortName, view
		t.Run(shortName, func(t *testing.T) {
			allKeys := resource.GetAllFieldKeysForTest(shortName)
			producerSet := make(map[string]bool, len(allKeys))
			for _, k := range allKeys {
				producerSet[k] = true
			}
			if len(view.List) > 0 && allKeys == nil && resource.FindResourceType(shortName) != nil {
				t.Errorf("no field keys registered for type %q — add SetFieldKeysForTest in the "+
					"fetcher's init, or the columns below have nothing that could fill them", shortName)
			}

			columnSet := make(map[string]bool, len(view.List))
			for _, col := range view.List {
				if col.Key != "" {
					columnSet[col.Key] = true
				}
			}

			if allowed, ok := columnKeyProducerAllowlist[shortName]; ok {
				for colKey := range allowed {
					if !columnSet[colKey] {
						t.Errorf(
							"stale allowlist entry: column %q on type %q no longer exists — remove it from columnKeyProducerAllowlist",
							colKey, shortName,
						)
					}
				}
			}

			for _, col := range view.List {
				// A column reads its cell from a Fields key or from a
				// RawStruct path, and the registry answers for the first. A
				// column that declares neither is not out of scope: its value
				// is looked up by turning the title into a key, which no
				// registry knows about and nothing here could check.
				if col.Key == "" {
					if col.Path == "" {
						t.Errorf(
							"column %q on type %q declares neither a key nor a path — its cell is "+
								"filled by guessing a field key from the title, so no producer can be "+
								"checked for it. Name the key the fetcher writes, or the path the "+
								"value is read from",
							col.Title, shortName,
						)
					}
					continue
				}

				if allowed, ok := columnKeyProducerAllowlist[shortName]; ok {
					if _, exempted := allowed[col.Key]; exempted {
						continue
					}
				}

				// config.ColumnFilled is the one rule, asked here and by the
				// load's own report, so a column this gate accepts is never one
				// the operator is told is broken. It carries the status-column
				// exemption and counts a Path as the producer it is.
				if td := resource.FindResourceType(shortName); td != nil && !config.ColumnFilled(*td, col) {
					t.Errorf(
						"column %q on type %q has no producer (fetcher or enricher) — "+
							"add SetFieldKeysForTest or RegisterEnricherFieldKeys for this key, "+
							"or add it to columnKeyProducerAllowlist with a justification",
						col.Key, shortName,
					)
				}
			}
		})
	}
}

// At least 10 IssueEnricherFieldKeys: literals across core/aws/catalog_*.go
// show the catalog wires Wave 2 field keys rather than declaring the field and
// leaving it empty everywhere.
func TestEnricherFieldKeys_RegisterCallsAreInInitBlock(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed — cannot determine source file path")
	}
	repoRoot := filepath.Join(filepath.Dir(filename), "..", "..")

	matches, err := filepath.Glob(filepath.Join(repoRoot, "core", "aws", "catalog_*.go"))
	if err != nil {
		t.Fatalf("filepath.Glob failed: %v", err)
	}
	if len(matches) == 0 {
		t.Fatal("filepath.Glob returned zero matches for core/aws/catalog_*.go — check repo layout")
	}

	const needle = "IssueEnricherFieldKeys:"
	total := 0
	for _, path := range matches {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("cannot read %s: %v", path, readErr)
		}
		total += strings.Count(string(data), needle)
	}

	const minExpected = 10
	if total < minExpected {
		t.Errorf(
			"found only %d occurrence(s) of %q across core/aws/catalog_*.go, expected at least %d — "+
				"each Wave 2 issue enricher that writes Resource.Fields keys must declare them on the "+
				"owning catalog.ResourceTypeDef literal's IssueEnricherFieldKeys field",
			total, needle, minExpected,
		)
	}
}
