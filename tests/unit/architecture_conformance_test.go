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

// TestConformance_EveryResourceTypeHasWave2Registration pins the Wave 2
// allowlist contract: every registered top-level resource type must have a
// catalog row whose Wave 2 status is EXPLICIT.
//
// "Explicit" means one of:
//   - catalog.Find(sn).Wave2 != nil — a real Wave 2 enricher is registered on the catalog row, OR
//   - awsclient.IssueEnricherRegistry[sn] exists — legacy registration (real or NoOpIssueEnricher), OR
//   - catalog.Find(sn) != nil AND its row is intentionally marked "no Wave 2" (Wave2 nil, no legacy entry).
//
// AS-726 PR-04i deletes the NoOp-only files for sns-sub and kinesis and routes
// their "no Wave 2" signal through catalog Wave2 == nil. Pre-AS-726 the only
// explicit signal was the IssueEnricherRegistry map. Post-AS-726 catalog
// presence is the new explicit signal — silent skips happen only when neither
// path knows about the shortName.
func TestConformance_EveryResourceTypeHasWave2Registration(t *testing.T) {
	for _, td := range resource.AllResourceTypes() {
		_, legacyOK := awsclient.IssueEnricherRegistry[td.ShortName]
		catalogRow := catalog.Find(td.ShortName)
		if legacyOK {
			continue
		}
		if catalogRow != nil {
			// Catalog-authoritative — Wave2 may be nil (explicit "no Wave 2")
			// or non-nil (Wave 2 registered via catalog.RegisterWave2). Either
			// way the type is known to the Wave 2 layer.
			continue
		}
		t.Errorf(
			"resource type %q has no explicit Wave 2 signal — "+
				"either register via catalog.RegisterWave2 (catalog-authoritative; nil is the "+
				"explicit \"no Wave 2\" signal) or add an IssueEnricherRegistry entry "+
				"(legacy path). Currently neither path knows about this type.",
			td.ShortName,
		)
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

// TestConformance_Wave2Registry_IsNonEmpty pins that the Wave 2 registry
// has been wired at package init. An empty registry would silently disable
// every Wave 2 background check, since buildEnrichQueue iterates over it.
func TestConformance_Wave2Registry_IsNonEmpty(t *testing.T) {
	if len(awsclient.IssueEnricherRegistry) == 0 {
		t.Fatal("awsclient.IssueEnricherRegistry is empty — Wave 2 dispatch would silently skip every type; init() wiring missing")
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

// ---------------------------------------------------------------------------
// AS-726 PR-04i — messaging catalog cutover (Phase 04)
// ---------------------------------------------------------------------------

// TestMessagingCatalogIsAuthoritative pins the AS-726 PR-04i acceptance: every
// messaging top-level type has its Wave 1 fetcher, FieldKeys, and Related defs
// living on the catalog row — not in the legacy resource/IssueEnricher registries.
//
// Snapshot counts come from the spec §1 file inventory (verified against the
// legacy RegisterRelated() blocks in internal/aws/*_related.go BEFORE the cutover).
// If a related def is added or removed, this test must change too — that's the
// contract: docs/related-resources.md is the source of truth, and the catalog
// row must mirror it.
//
// IssueEnricherFieldKeys are checked for the 5 messaging types that legacy
// registered them (sqs, sns, eb-rule, sfn, ses). msk, sns-sub, and kinesis
// have no Wave 2 field keys (msk's enricher writes no extra keys; sns-sub
// and kinesis are the NoOp deletions covered by TestNoOpWave2ReturnsAbsent).
//
// Navigable is checked for the 4 types whose legacy init() called
// RegisterDefaultNavFields: sns-sub, eb-rule, msk, sfn.
func TestMessagingCatalogIsAuthoritative(t *testing.T) {
	cases := []struct {
		shortName              string
		expectedRelated        int
		expectedNavigable      int
		expectedIssueFieldKeys []string // exact, in order
		expectsWave2           bool
	}{
		{"sqs", 7, 0, []string{"dlq"}, true},
		{"sns", 4, 0, []string{"subs_count"}, true},
		{"sns-sub", 3, 1, nil, false}, // Wave 2 file deleted — see TestNoOpWave2ReturnsAbsent
		{"eb-rule", 7, 1, []string{"target_count"}, true},
		{"kinesis", 5, 0, nil, false}, // Wave 2 file deleted — see TestNoOpWave2ReturnsAbsent
		{"msk", 10, 1, nil, true},
		{"sfn", 6, 1, []string{"last_run"}, true},
		{"ses", 5, 0, []string{"status"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.shortName, func(t *testing.T) {
			ct := catalog.Find(tc.shortName)
			if ct == nil {
				t.Fatalf("catalog.Find(%q) returned nil — messaging row missing from catalog", tc.shortName)
			}
			if ct.Fetcher == nil {
				t.Errorf("%s: Fetcher missing on catalog row — legacy registration still authoritative", tc.shortName)
			}
			if len(ct.FieldKeys) == 0 {
				t.Errorf("%s: FieldKeys empty on catalog row — legacy registration still authoritative", tc.shortName)
			}
			if got := len(ct.Related); got != tc.expectedRelated {
				t.Errorf("%s: catalog Related has %d entries, want %d (per spec §1 inventory)",
					tc.shortName, got, tc.expectedRelated)
			}
			if got := len(ct.Navigable); got != tc.expectedNavigable {
				t.Errorf("%s: catalog Navigable has %d entries, want %d",
					tc.shortName, got, tc.expectedNavigable)
			}
			if tc.expectsWave2 {
				if ct.Wave2 == nil {
					t.Errorf("%s: catalog Wave2 nil, want non-nil (legacy registered a Wave 2 enricher)", tc.shortName)
				}
			} else {
				if ct.Wave2 != nil {
					t.Errorf("%s: catalog Wave2 non-nil, want nil (NoOp file deleted per spec §4)", tc.shortName)
				}
			}
			if got := ct.IssueEnricherFieldKeys; !stringSliceEqual(got, tc.expectedIssueFieldKeys) {
				t.Errorf("%s: catalog IssueEnricherFieldKeys = %v, want %v",
					tc.shortName, got, tc.expectedIssueFieldKeys)
			}
		})
	}
}

func stringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
