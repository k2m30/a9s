package unit

// column_keys_have_producers_test.go — systemic contract: every column key in
// every ResourceTypeDef must have a registered producer (fetcher or enricher).
//
// TestColumnKeysHaveProducers walks all registered ResourceTypeDef entries and
// checks that each column's Key is present in GetAllFieldKeys (the union of the
// fetcher field-key registry and the enricher field-key registry).
//
// TestEnricherFieldKeys_RegisterCallsAreInInitBlock is a stringy smoke test that
// globs internal/aws/*_issue_enrichment.go to verify the coder actually wired up
// RegisterEnricherFieldKeys calls rather than declaring the helper and leaving it
// empty.

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/config"
	"github.com/k2m30/a9s/v3/internal/resource"
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

// TestColumnKeysHaveProducers verifies that every List-column Key defined in
// the built-in default views (internal/config/defaults_*.go, merged via
// config.DefaultConfig()) is present in GetAllFieldKeys — the union of the
// fetcher (RegisterFieldKeys) and enricher (RegisterEnricherFieldKeys)
// registries for that resource type.
//
// The built-in defaults are the ACTUAL columns shipped to end users; the
// baked-in ResourceTypeDef.Columns is an older layer that most types no
// longer populate. The gap this guards against: a default view Key references
// a Resource.Fields entry nobody writes → column renders blank forever.
//
// Failure message: column %q on type %q has no producer (fetcher or enricher)
//
// If this test finds real gaps and you believe they are intentional, add them
// to columnKeyProducerAllowlist with a justification comment — do NOT weaken
// the assertion.
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
			allKeys := resource.GetAllFieldKeys(shortName)
			producerSet := make(map[string]bool, len(allKeys))
			for _, k := range allKeys {
				producerSet[k] = true
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
				// Columns that drive rendering from a Path (reflection into
				// RawStruct) instead of a Fields Key are out of scope — nothing
				// for the registry to produce.
				if col.Key == "" {
					continue
				}

				if allowed, ok := columnKeyProducerAllowlist[shortName]; ok {
					if _, exempted := allowed[col.Key]; exempted {
						continue
					}
				}

				if !producerSet[col.Key] {
					t.Errorf(
						"column %q on type %q has no producer (fetcher or enricher) — "+
							"add RegisterFieldKeys or RegisterEnricherFieldKeys for this key, "+
							"or add it to columnKeyProducerAllowlist with a justification",
						col.Key, shortName,
					)
				}
			}
		})
	}
}

// TestEnricherFieldKeys_RegisterCallsAreInInitBlock is a stringy smoke test that
// globs internal/aws/catalog_*.go and counts IssueEnricherFieldKeys: literals
// on per-resource catalog struct literals. Post-AS-795n the Wave 2 field-key
// registrations live in the catalog (the bridge in install.go replays them
// into the legacy resource.RegisterIssueEnricherFieldKeys map). Requiring at
// least 10 occurrences proves the catalog actually wires Wave 2 field keys
// rather than declaring the catalog field and leaving it empty everywhere.
func TestEnricherFieldKeys_RegisterCallsAreInInitBlock(t *testing.T) {
	// Locate the repo root via the test file's own path.
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed — cannot determine source file path")
	}
	// tests/unit/ -> two levels up -> repo root
	repoRoot := filepath.Join(filepath.Dir(filename), "..", "..")

	matches, err := filepath.Glob(filepath.Join(repoRoot, "internal", "aws", "catalog_*.go"))
	if err != nil {
		t.Fatalf("filepath.Glob failed: %v", err)
	}
	if len(matches) == 0 {
		t.Fatal("filepath.Glob returned zero matches for internal/aws/catalog_*.go — check repo layout")
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
			"found only %d occurrence(s) of %q across internal/aws/catalog_*.go, expected at least %d — "+
				"each Wave 2 issue enricher that writes Resource.Fields keys must declare them on the "+
				"owning catalog.ResourceTypeDef literal's IssueEnricherFieldKeys field",
			total, needle, minExpected,
		)
	}
}
