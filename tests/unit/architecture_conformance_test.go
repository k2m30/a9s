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

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/resource"
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
// contract: every catalog entry whose Wave2 field is non-nil must resolve
// through awsclient.Wave2EnricherFor. The catalog is now the single source of
// truth — this conformance variant iterates catalog.All() directly instead of
// parsing docs/attention-signals.md (the markdown-parsing scaffolding was
// dropped once the catalog became that source of truth).
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
	_ = resource.ValidateRelatedResultAgainstCacheForTest
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

// ---------------------------------------------------------------------------
// Row-store unification (task #17) — no parallel per-type row store outside
// RowStore; no store-row mutation outside the Amend/AmendRows seam.
// ---------------------------------------------------------------------------

// forbiddenRowStoreShapeFieldPatterns matches STRUCT FIELD declarations shaped
// like the legacy per-type row maps the row-store unification plan retired
// (session.ProbeResources/ResourceCache/LazyResourceCache, all
// map[string][]resource.Resource, and the runtime.RuntimeState-adjacent
// map[string]*domain.ListViewCacheEntry shape). A field of either shape
// outside the store itself would be a new parallel per-type row cache
// reintroducing the exact class of dual-write drift task #17 eliminated.
//
// Deliberately anchored on "<fieldName> map[...]" (an identifier immediately
// followed by the map type) rather than a bare "map[string][]resource.Resource"
// substring match, so local variables (`var lazyAdds map[string][]resource.Resource`),
// function parameters, and return types are NOT flagged — only a field
// declared inside a struct body matches this shape (a field decl is the only
// place an identifier is directly followed by a map type with no `:=`, `var`,
// or parameter-list comma/paren context). This is a lexical heuristic, not a
// parser: it deliberately trades perfect precision for a cheap, dependency-free
// scan, matching this file's existing hardcodedAllowlistPatterns convention.
var forbiddenRowStoreShapeFieldPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?m)^\s*[A-Za-z_][A-Za-z0-9_]*\s+map\[string\]\[\]resource\.Resource\s*$`),
	regexp.MustCompile(`(?m)^\s*[A-Za-z_][A-Za-z0-9_]*\s+map\[string\]\*domain\.ListViewCacheEntry\s*$`),
}

// allowedRowStoreShapeFiles lists production files permitted to declare a
// field of one of the forbiddenRowStoreShapeFieldPatterns shapes:
//
//   - core/session/rowstore.go: RowStore itself — the sole per-type row
//     store task #17 unifies onto; TypeRows/RowStore's own fields are the
//     allowed destination, not a violation of the rule they enforce.
//   - core/runtime/state.go: RuntimeState.ResourceCache is a documented
//     derived SNAPSHOT field ("mirrors RowStore's retained... entries for the
//     active session", state.go's own doc comment) — RuntimeState has no
//     production constructor call site at HEAD (verified: no
//     `RuntimeState{` literal anywhere outside this declaration), so it
//     cannot itself become a second source of truth to drift against
//     RowStore. If a future change adds a production `RuntimeState{...}`
//     construction path, this allowance should be revisited alongside it.
//   - The remaining five entries below are all one-shot MESSAGE/EVENT/INTENT/
//     TASK payload structs, not per-session state: each is constructed fresh
//     per call/dispatch, carries its Resources/Adds/LazyAddedResources batch
//     through exactly one handoff (a function call or a single Bubble Tea
//     message delivery), and is then discarded — never held by reference
//     across multiple calls the way session.ProbeResources/ResourceCache/
//     LazyResourceCache were. This is the message-passing analogue of the
//     "fetchers building NEW rows before Observe" row-construction carve-out:
//     a transport payload is not a competing store to drift against RowStore.
//     Verified by direct inspection of each field's owning type at HEAD:
//   - runtime/handlers_resources.go: RelatedCheckResultEvent.LazyAddedResources
//     (a related-check RESULT event, one per checker completion)
//   - runtime/intent.go: PatchLazyResourceCache.Adds (a UIIntent value,
//     applied once by Controller.ApplyIntents then discarded)
//   - runtime/messages/event.go: RelatedCheckResult.LazyAddedResources (the
//     Bubble Tea message mirror of the same event)
//   - runtime/probes.go: DemoPrefetchResult.Resources (a synchronous demo
//     prefetch's one-shot combined result)
//   - runtime/tasks.go: SaveCachePayload.Resources (a TaskKindSaveCache
//     payload, read once by the executor's save-cache case)
var allowedRowStoreShapeFiles = map[string]struct{}{
	"session/rowstore.go":           {},
	"runtime/state.go":              {},
	"runtime/handlers_resources.go": {},
	"runtime/intent.go":             {},
	"runtime/messages/event.go":     {},
	"runtime/probes.go":             {},
	"runtime/tasks.go":              {},
}

// TestConformance_NoParallelPerTypeRowStore_OutsideRowStore scans every
// non-test production file under internal/ for a struct field shaped like
// the legacy per-type row maps task #17 (row-store unification) retired.
// RowStore (core/session/rowstore.go) is the sole per-type row store as
// of Stage 3+; a new field of either forbidden shape elsewhere would
// reintroduce the dual-write-drift defect class (D13-D18) that motivated the
// unification.
func TestConformance_NoParallelPerTypeRowStore_OutsideRowStore(t *testing.T) {
	for _, root := range confProductionScanRoots {
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
			rel, rerr := filepath.Rel(root, path)
			if rerr != nil {
				return rerr
			}
			rel = filepath.ToSlash(rel)
			if _, allowed := allowedRowStoreShapeFiles[rel]; allowed {
				return nil
			}
			data, rerr := os.ReadFile(path)
			if rerr != nil {
				return rerr
			}
			for _, pat := range forbiddenRowStoreShapeFieldPatterns {
				if loc := pat.FindIndex(data); loc != nil {
					snippet := strings.TrimSpace(string(data[loc[0]:loc[1]]))
					t.Errorf(
						"%s: struct field %q matches a retired per-type row-store shape — "+
							"core/session.RowStore is the sole per-type row store (task #17); "+
							"a new field of this shape reintroduces the dual-write-drift defect class "+
							"(D13-D18) the unification eliminated",
						rel, snippet,
					)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s failed: %v", root, err)
		}
	}
}

// confProductionScanRoots are the two production source roots after the
// core/ extraction: the relicensable core and the GPL-only TUI adapter.
var confProductionScanRoots = []string{"../../core", "../../internal"}

var (
	goLineCommentPattern  = regexp.MustCompile(`//[^\n]*`)
	goBlockCommentPattern = regexp.MustCompile(`(?s)/\*.*?\*/`)
)

// stripGoComments removes // line and /* */ block comments so a source
// scanner matches real code, not a function name referenced in prose. A "//"
// inside a string literal is not handled — no such case exists for the scans
// that use this.
func stripGoComments(src []byte) []byte {
	src = goBlockCommentPattern.ReplaceAll(src, []byte(" "))
	return goLineCommentPattern.ReplaceAll(src, []byte(""))
}

// rowStoreMutationSeamFiles lists the known production call sites of
// ApplyWave2ToRow/applyWave2ToRow — the two enrich-fold mutators task #17's
// RowStore.Amend/Core.AmendRows exist to make copy-on-write-safe (see
// RowStore.Amend's doc comment: "the two enrich-fold implementations in
// runtime/helpers.go and tui/app_enrich_fold.go both mutate resource.Resource
// fields in place on a shared backing array; Amend is their eventual
// dual-write / replacement target"). Verified against HEAD by direct
// inspection: every one of these files either (a) mutates only a
// ListState-owned row slice (never RowStore's own backing array), or (b)
// wraps the mutation inside an Amend/AmendRows copy-on-write callback,
// operating on that callback's freshly-copied slice — never a bare
// RowStore.Snapshot/SnapshotAll result. A file added to this set in the
// future without satisfying (a) or (b) is exactly the mutate-in-place
// regression this pin exists to catch.
var rowStoreMutationSeamFiles = map[string]struct{}{
	"app/list_columns.go":              {}, // read-only Findings[0] access, not a mutation call site
	"app/list_body.go":                 {}, // applyRowFindings: ListState.Rows direct + AmendRows-wrapped store leg
	"tui/app_enrich_fold.go":           {}, // applyEnrichment: AmendRows-wrapped store leg only
	"tui/runtime_adapter_navigate.go":  {}, // comment reference only, no direct mutation call
	"runtime/handlers_availability.go": {}, // AmendRows-wrapped FieldUpdates fold
	"runtime/handlers_resources.go":    {}, // stripWave2FindingsRows: AmendRows-wrapped clear leg only
	"runtime/helpers.go":               {}, // applyEnrichment: AmendRows-wrapped store leg only
}

// TestConformance_Wave2RowMutators_HaveNoUnvettedCallSites pins the CLOSED
// set of production files calling ApplyWave2ToRow/applyWave2ToRow. Each
// entry in rowStoreMutationSeamFiles has been manually verified (see that
// var's doc comment) to route any RowStore-backed mutation through
// Amend/AmendRows rather than mutating a bare Snapshot/SnapshotAll result in
// place. A NEW call site appearing outside this set has not been vetted
// against that discipline — this test fails loudly instead of silently
// trusting an unreviewed mutation call site, forcing the same manual
// verification this file's existing entries already received.
func TestConformance_Wave2RowMutators_HaveNoUnvettedCallSites(t *testing.T) {
	callPattern := regexp.MustCompile(`\bApplyWave2ToRow\s*\(|\bapplyWave2ToRow\s*\(`)
	defPattern := regexp.MustCompile(`func\s+ApplyWave2ToRow\s*\(|func\s+applyWave2ToRow\s*\(`)

	for _, root := range confProductionScanRoots {
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
			rel, rerr := filepath.Rel(root, path)
			if rerr != nil {
				return rerr
			}
			rel = filepath.ToSlash(rel)

			raw, rerr := os.ReadFile(path)
			if rerr != nil {
				return rerr
			}
			// Strip comments before matching: a doc comment mentioning
			// ApplyWave2ToRow(...) is not a call site, and the scanner must not
			// flag it (the regex's `\s*\(` otherwise matches "ApplyWave2ToRow
			// (core/runtime/helpers.go)" inside prose).
			data := stripGoComments(raw)
			if !callPattern.Match(data) {
				return nil
			}
			if defPattern.Match(data) {
				// The function's own definition file always contains its call
				// pattern trivially (the func signature itself); that is not a
				// call site.
				if defOnly := callPattern.FindAllIndex(data, -1); len(defOnly) == 1 {
					return nil
				}
			}
			if _, ok := rowStoreMutationSeamFiles[rel]; !ok {
				t.Errorf(
					"%s: calls ApplyWave2ToRow/applyWave2ToRow but is not in rowStoreMutationSeamFiles — "+
						"a new call site must be manually verified to route any RowStore-backed mutation "+
						"through Amend/AmendRows (never mutate a bare Snapshot/SnapshotAll result in place, "+
						"per RowStore.Amend's copy-on-write doc comment) and then added to that allowlist",
					rel,
				)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s failed: %v", root, err)
		}
	}
}

// ---------------------------------------------------------------------------
// A row marked uninspected says why
// ---------------------------------------------------------------------------

// TestConformance_UninspectedRowsRecordTheirReason pins the "skipped" spec row
// 4: a Wave 2 enricher that marks a row uninspected records the failed call
// that made it so, through MarkSkipped, so the reason reaches the error log
// instead of being dropped on the floor. Writing TruncatedIDs directly is the
// silent-skip shape — the row renders "?" and nobody can say what refused.
//
// The census below is the debt that existed when the rule landed, per file and
// per count. It is a ratchet: a new direct write fails the gate, and a file
// converted to MarkSkipped fails it too until its number comes down. Both
// directions are deliberate — the list only shrinks, and it shrinks visibly.
func TestConformance_UninspectedRowsRecordTheirReason(t *testing.T) {
	directWrite := regexp.MustCompile(`\w+\.TruncatedIDs\[[^\]]*\]\s*=\s*true`)

	seen := map[string]int{}
	entries, err := os.ReadDir("../../core/aws")
	if err != nil {
		t.Fatalf("read core/aws: %v", err)
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		// issue_enrichment.go holds MarkSkipped, markAllUninspected and
		// capAtEnrichmentCap themselves — the helpers the rule routes through.
		if name == "issue_enrichment.go" {
			continue
		}
		raw, rerr := os.ReadFile(filepath.Join("../../core/aws", name))
		if rerr != nil {
			t.Fatalf("read %s: %v", name, rerr)
		}
		if n := len(directWrite.FindAll(stripGoComments(raw), -1)); n > 0 {
			seen[name] = n
		}
	}

	for name, n := range seen {
		want, ok := uninspectedWithoutAReasonCensus[name]
		switch {
		case !ok:
			t.Errorf("core/aws/%s writes TruncatedIDs directly (%d times): a row nobody looked at must "+
				"record what refused, via MarkSkipped(&result, id, &failures, err)", name, n)
		case n > want:
			t.Errorf("core/aws/%s writes TruncatedIDs directly %d times, census says %d: "+
				"the new one must record its reason through MarkSkipped", name, n, want)
		case n < want:
			t.Errorf("core/aws/%s writes TruncatedIDs directly %d times, census says %d: "+
				"lower the number in uninspectedWithoutAReasonCensus", name, n, want)
		}
	}
	for name := range uninspectedWithoutAReasonCensus {
		if _, ok := seen[name]; !ok {
			t.Errorf("core/aws/%s is in uninspectedWithoutAReasonCensus but writes TruncatedIDs "+
				"nowhere any more: drop its entry", name)
		}
	}
}

// uninspectedWithoutAReasonCensus is the per-file count of direct TruncatedIDs
// writes that predate the rule. Every entry is a row that can render "?" with
// no line in the error log to explain it; each is owed a conversion to
// MarkSkipped (or, where there is genuinely no error — a cap, an absent field
// — an explicit decision recorded in the enricher's own doc comment).
var uninspectedWithoutAReasonCensus = map[string]int{
	"apigw_issue_enrichment.go":        3,
	"asg_issue_enrichment.go":          1,
	"athena_issue_enrichment.go":       1,
	"cb_issue_enrichment.go":           1,
	"cf_issue_enrichment.go":           1,
	"codeartifact_issue_enrichment.go": 2,
	"eb_issue_enrichment.go":           2,
	"eb_rule_issue_enrichment.go":      1,
	"ec2_issue_enrichment.go":          1,
	"ecr_issue_enrichment.go":          2,
	"ecs_issue_enrichment.go":          1,
	"ecs_task_issue_enrichment.go":     1,
	"efs_issue_enrichment.go":          2,
	"glue_issue_enrichment.go":         1,
	"iam_group_issue_enrichment.go":    1,
	"iam_policy_issue_enrichment.go":   1,
	"iam_role_issue_enrichment.go":     2,
	"iam_user_issue_enrichment.go":     5,
	"kinesis_issue_enrichment.go":      2,
	"kms_issue_enrichment.go":          2,
	"lambda_issue_enrichment.go":       1,
	"r53_issue_enrichment.go":          3,
	"redshift_issue_enrichment.go":     1,
	"s3_issue_enrichment.go":           2,
	"secrets_issue_enrichment.go":      2,
	"ses_issue_enrichment.go":          1,
	"sfn_issue_enrichment.go":          1,
	"snapshot_cross_ref.go":            1,
	"sns_issue_enrichment.go":          2,
	"tgw_issue_enrichment.go":          1,
	"trail_issue_enrichment.go":        2,
	"vpc_issue_enrichment.go":          1,
}
