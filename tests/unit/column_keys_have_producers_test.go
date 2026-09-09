package unit

// column_keys_have_producers_test.go — the one gate over one property: every
// column names where its cell comes from, and a named Fields key has something
// that writes it.
//
// It was two gates. column_key_mismatch_test.go walked the catalog's columns
// and this file walked the built-in views' columns, checking the same keys
// against the same registry, reading the same allowlist, and reporting a
// failure in two different sentences. The two lists are one list now
// (TestDefaultConfigColumnsAreTheCatalogs), so the second walk could only ever
// repeat the first: the columns a type declares ARE the columns its view
// renders, for every parent type and every child type (w197 row 12).
//
// TestEnricherFieldKeys_RegisterCallsAreInInitBlock is a stringy smoke test that
// globs core/aws/*_issue_enrichment.go to verify the coder actually wired up
// RegisterEnricherFieldKeys calls rather than declaring the helper and leaving it
// empty.

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

// TestColumnKeysHaveProducers verifies that every List-column Key defined in
// the built-in default views (core/config/defaults_*.go, merged via
// config.DefaultConfig()) is present in GetAllFieldKeys — the union of the
// fetcher (SetFieldKeysForTest) and enricher (RegisterEnricherFieldKeys)
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

			lifecycleKey := "state"
			if td := resource.FindResourceType(shortName); td != nil && td.LifecycleKey != "" {
				lifecycleKey = td.LifecycleKey
			}

			for _, col := range view.List {
				// The type's declared status column is exempt, as a rule about
				// the shape rather than fifteen entries about fifteen types
				// (w197 row 11). Its cell is the finding phrase first, and the
				// save lane persists what the screen showed under this very
				// key, so a fetcher that never writes it is not a gap: the
				// cell is not empty for want of it. Fifteen types are in that
				// position today and the sixteenth needs no line here.
				if config.IsStatusColumn(col.Key, lifecycleKey) {
					continue
				}

				// A column reads its cell from a Fields key or from a
				// RawStruct path, and the registry answers for the first. A
				// column that declares neither is not out of scope: its value
				// is looked up by turning the title into a key, which no
				// registry knows about and nothing here could check — the one
				// shape this gate used to skip by construction (w197 row 8).
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

				if !producerSet[col.Key] {
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

// TestEnricherFieldKeys_RegisterCallsAreInInitBlock is a stringy smoke test that
// globs core/aws/catalog_*.go and counts IssueEnricherFieldKeys: literals
// on per-resource catalog struct literals. Post-AS-795n the Wave 2 field-key
// registrations live in the catalog (the bridge in install.go replays them
// into the legacy resource.SetIssueEnricherFieldKeysForTest map). Requiring at
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

// TestProducerAllowlistCarriesNoLifecycleKeyEntry pins the other half of row
// 11. Fifteen entries in one allowlist sharing one justification is one rule
// written fifteen times: the next type with a findings-filled status column
// adds a sixteenth, and the day one of the fifteen gains a real producer
// nobody removes its line.
//
// A status column whose key is the type's lifecycle key needs no per-type
// exemption at all — the rule is the shape, and the gate can say it once.
func TestProducerAllowlistCarriesNoLifecycleKeyEntry(t *testing.T) {
	for _, td := range resource.AllResourceTypes() {
		lifecycleKey := td.LifecycleKey
		if lifecycleKey == "" {
			lifecycleKey = "state"
		}
		for key := range columnKeyProducerAllowlist[td.ShortName] {
			if key == lifecycleKey || key == "status" {
				t.Errorf("the producers allowlist exempts %s/%q by name, which is the type's lifecycle "+
					"key — a status column filled from findings is one rule about a shape, not fifteen "+
					"entries about fifteen types", td.ShortName, key)
			}
		}
	}
}
