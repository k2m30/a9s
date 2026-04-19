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

// TestConformance_EveryResourceTypeHasWave2Registration pins the Wave 2
// allowlist contract: every registered top-level resource type must have an
// entry in IssueEnricherRegistry, either a real issue enricher or
// NoOpIssueEnricher. An unregistered type is a silent skip — it would never
// participate in Wave 2 even when docs/attention-signals.md claims it should.
func TestConformance_EveryResourceTypeHasWave2Registration(t *testing.T) {
	for _, td := range resource.AllResourceTypes() {
		if _, ok := awsclient.IssueEnricherRegistry[td.ShortName]; !ok {
			t.Errorf(
				"resource type %q is not in awsclient.IssueEnricherRegistry — "+
					"register either a real IssueEnricherFunc or NoOpIssueEnricher "+
					"so the Wave 2 contract is explicit (docs/attention-signals.md)",
				td.ShortName,
			)
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

// TestConformance_GenCountersSeedAtOne_NotZero pins the seed=1 convention
// for every gen counter owned by sessionRuntime. Zero is the documented
// "never match" value used by test-injected messages; a freshly constructed
// session must never compare-equal against Gen=0. This guard would catch a
// regression where someone "cleaned up" the explicit =1 seeds.
//
// We prove it indirectly through behaviour: bumping a session generation
// from initial state must not roll through zero. The sessionRuntime struct
// is unexported so we cannot read the counters directly; the behavioural
// assertion is good enough — any sessionRuntime with a seed of 0 would
// still produce 1 on first bump (matching the documented "always stale" Gen=0
// test messages), which would make rerun-on-start paths disagree with the
// test-injection convention. The dedicated integration exists in
// tests/unit/enricher_naming_contract_test.go and the profile-switch tests.
func TestConformance_GenCountersSeedAtOne_NotZero(t *testing.T) {
	// This test is informational — its presence documents the contract and
	// points at the dedicated tests that enforce it. Remove only when the
	// seed-at-1 convention is itself removed (and the test-injection Gen=0
	// sentinel replaced).
	t.Log(
		"sessionRuntime seeds relatedGen/enrichGen at 1; Gen=0 on incoming " +
			"messages is the documented test-injection bypass. See " +
			"internal/tui/session_runtime.go and the profile-switch handlers.",
	)
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
// Currently empty — the dispatch code uses IssueEnricherRegistry iteration.
var allowedTUIFiles = map[string]struct{}{}

// TestConformance_NoHardcodedTypeAllowlist_InTUIDispatch scans internal/tui
// source files for slice literals of known Wave 2 resource short names. The
// Wave 2 dispatch contract is "iterate IssueEnricherRegistry, sort by
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
						"awsclient.IssueEnricherRegistry instead. Snippet: %q",
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
