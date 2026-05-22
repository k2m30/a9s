package unit

// architecture_conformance_test.go — Executable checks for the architectural
// contracts called out in docs/architecture.md. Each test pins an invariant
// that would otherwise drift into tribal knowledge: if a new contributor
// accidentally reintroduces a hardcoded allowlist, skips registration, or
// breaks a gen guard, these tests fail.
//
// Scope: invariants that span packages or are enforced by convention rather
// than type system. Tests that live with their feature (e.g. the Wave 2
// dispatch-order tests in enrich_queue_test.go) are not duplicated here.

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/catalog"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ---------------------------------------------------------------------------
// Registry completeness
// ---------------------------------------------------------------------------

// TestConformance_EveryResourceTypeHasPaginatedFetcher pins that every
// top-level resource short name has a PaginatedFetcher registered. A
// registered type with no fetcher would render as an empty page with no
// error — a silent contract break.
func TestConformance_EveryResourceTypeHasPaginatedFetcher(t *testing.T) {
	for _, td := range resource.AllResourceTypes() {
		if resource.GetPaginatedFetcher(td.ShortName) == nil {
			t.Errorf("resource type %q has no PaginatedFetcher registered", td.ShortName)
		}
	}
}

// TestConformance_EveryCatalogWave2ResolvesThroughAccessor pins the Wave 2
// contract post-AS-731: every catalog entry whose Wave2 field is non-nil must
// resolve through awsclient.Wave2EnricherFor. The catalog is now the single
// source of truth — this conformance variant iterates catalog.All() directly
// instead of parsing docs/attention-signals.md (the markdown-parsing
// scaffolding was dropped per AS-731 scope item 6).
func TestConformance_EveryCatalogWave2ResolvesThroughAccessor(t *testing.T) {
	entries := catalog.All()
	if len(entries) == 0 {
		t.Fatal("catalog.All() returned 0 entries — catalog wiring missing")
	}
	for _, td := range entries {
		if td.Wave2 == nil {
			continue
		}
		if _, ok := awsclient.Wave2EnricherFor(td.ShortName); !ok {
			t.Errorf("catalog entry %q has non-nil Wave2 but awsclient.Wave2EnricherFor returns ok=false (accessor wiring broken)", td.ShortName)
		}
	}
}

// ---------------------------------------------------------------------------
// Canonical-ID contract surface
// ---------------------------------------------------------------------------

// TestConformance_RelatedValidatorsExposed pins that the helpers #279 added
// remain public entry points. Regression guard: if someone accidentally
// un-exports or deletes them, related-navigation loses its contract check.
func TestConformance_RelatedValidatorsExposed(t *testing.T) {
	// Shape-only validator.
	_ = resource.ValidateRelatedResult
	// Cross-check against cache validator.
	_ = resource.ValidateRelatedResultAgainstCache
}

// ---------------------------------------------------------------------------
// Stale-result / invalidation guards
// ---------------------------------------------------------------------------

// TestConformance_Wave2Registry_IsNonEmpty pins that the Wave 2 catalog
// surface is non-empty. An empty AllWave2 would silently disable every Wave 2
// background check, since BuildEnrichQueue iterates over it.
func TestConformance_Wave2Registry_IsNonEmpty(t *testing.T) {
	if len(awsclient.AllWave2()) == 0 {
		t.Fatal("awsclient.AllWave2() is empty — Wave 2 dispatch would silently skip every type; catalog wiring missing")
	}
}

// ---------------------------------------------------------------------------
// No-hardcoded-allowlist guard
// ---------------------------------------------------------------------------

// hardcodedAllowlistPatterns lists regex patterns that would indicate a new
// hardcoded supported-type allowlist in dispatch code. The patterns match the
// slice-literal shapes we actively avoid: []string{"dbi", ...}, []string{"ec2", ...},
// etc. This is conservative — the allowlist check only scans runtime dispatch
// code (internal/tui), not tests or fixtures where such literals are fine.
var hardcodedAllowlistPatterns = []*regexp.Regexp{
	// A slice literal containing a Wave 2 short name in TUI runtime code.
	// This would indicate someone reintroducing a manual dispatch list.
	regexp.MustCompile(`\[\]string\s*\{\s*"(dbi|ebs|cb|tg|pipeline|sfn|glue|rds|ec2|ecs-svc)"[\s,]`),
}

// allowedTUIFiles lists internal/tui files where a string-literal slice of
// resource short names is legitimate (test harnesses, non-dispatch helpers).
// Currently empty — the dispatch code uses awsclient.AllWave2 iteration.
var allowedTUIFiles = map[string]struct{}{}

// TestConformance_NoHardcodedTypeAllowlist_InTUIDispatch scans internal/tui
// source files for slice literals of known Wave 2 resource short names. The
// Wave 2 dispatch contract is "iterate awsclient.AllWave2(), sort by
// priority" — a hardcoded allowlist in the TUI package would regress the
// declarative scheduling contract from #277.
//
// This is a cheap lexical guard, not a full parse. False positives are
// handled via allowedTUIFiles.
func TestConformance_NoHardcodedTypeAllowlist_InTUIDispatch(t *testing.T) {
	root := "../../internal/tui"
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		if _, allowed := allowedTUIFiles[rel]; allowed {
			return nil
		}
		data, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		for _, pat := range hardcodedAllowlistPatterns {
			if loc := pat.FindIndex(data); loc != nil {
				snippet := string(data[loc[0]:min(loc[1]+40, len(data))])
				t.Errorf(
					"%s: hardcoded Wave 2 short-name allowlist detected — dispatch must iterate "+
						"awsclient.AllWave2 instead. Snippet: %q",
					rel, snippet,
				)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk internal/tui failed: %v", err)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
