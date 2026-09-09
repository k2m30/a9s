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
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
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
// A row marked uninspected goes through the recorder
// ---------------------------------------------------------------------------

// uninspectedSetField is the field only the recorder may write.
const uninspectedSetField = "TruncatedIDs"

// uninspectedRecorderViolations reports every element write to the uninspected
// set in file, wherever the map that is written came from. The rule is the
// call, not the spelling: an enricher marks a row by CALLING the recorder
// (MarkSkipped, MarkUnusable, the cap recorder), and a row that reaches the
// session by any other route arrives without the check that did not answer.
//
// It follows the map one hop: a local the set is assigned to, and a parameter
// of a same-file helper the set is passed to. Reading the set, carrying it
// forward in a composite literal, copying it into a merged map and allocating
// it are all untouched — none of them marks a row.
//
// reached counts the selectors seen, so the walk can say when it is reading
// the wrong tree.
func uninspectedRecorderViolations(fset *token.FileSet, fileName string, file *ast.File) (violations []string, reached int) {
	funcs := map[string]*ast.FuncDecl{}
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Body != nil {
			funcs[fn.Name.Name] = fn
		}
	}

	// tainted names hold the set itself: a local it was assigned to, or a
	// parameter a helper received it in. Collected to a fixpoint, so a helper
	// that passes the set on to a second helper is followed as far as it goes
	// rather than one hop.
	tainted := map[string]bool{}
	holdsTheSet := func(e ast.Expr) bool {
		if isUninspectedSet(e) {
			return true
		}
		id, ok := e.(*ast.Ident)
		return ok && tainted[id.Name]
	}
	for grew := true; grew; {
		grew = false
		note := func(name string) {
			if name != "" && !tainted[name] {
				tainted[name] = true
				grew = true
			}
		}
		ast.Inspect(file, func(n ast.Node) bool {
			switch v := n.(type) {
			case *ast.AssignStmt:
				for i, rhs := range v.Rhs {
					if !holdsTheSet(rhs) || i >= len(v.Lhs) {
						continue
					}
					if id, ok := v.Lhs[i].(*ast.Ident); ok {
						note(id.Name)
					}
				}
			case *ast.CallExpr:
				name, ok := v.Fun.(*ast.Ident)
				if !ok {
					return true
				}
				fn, known := funcs[name.Name]
				if !known {
					return true
				}
				for i, arg := range v.Args {
					if holdsTheSet(arg) {
						note(paramNameAt(fn, i))
					}
				}
			}
			return true
		})
	}
	ast.Inspect(file, func(n ast.Node) bool {
		if isUninspectedSet(n) {
			reached++
		}
		return true
	})

	// maps.Copy writes every key of its source into its destination, which
	// marks rows without an element assignment anywhere.
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || len(call.Args) != 2 || !holdsTheSet(call.Args[0]) {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Copy" {
			return true
		}
		if pkg, ok := sel.X.(*ast.Ident); !ok || pkg.Name != "maps" {
			return true
		}
		violations = append(violations, fmt.Sprintf("%s:%d copies rows into %s wholesale, so every one of them is marked without passing the recorder",
			fileName, fset.Position(call.Pos()).Line, uninspectedSetField))
		return true
	})

	ast.Inspect(file, func(n ast.Node) bool {
		assign, ok := n.(*ast.AssignStmt)
		if !ok {
			return true
		}
		for _, lhs := range assign.Lhs {
			idx, isIdx := lhs.(*ast.IndexExpr)
			if !isIdx {
				continue
			}
			switch target := idx.X.(type) {
			case *ast.SelectorExpr:
				if isUninspectedSet(target) {
					violations = append(violations, fmt.Sprintf("%s:%d writes %s[…] itself instead of calling the recorder",
						fileName, fset.Position(idx.Pos()).Line, uninspectedSetField))
				}
			case *ast.Ident:
				if tainted[target.Name] {
					violations = append(violations, fmt.Sprintf("%s:%d writes an element of %s through %q, so the mark never passes the recorder",
						fileName, fset.Position(idx.Pos()).Line, uninspectedSetField, target.Name))
				}
			}
		}
		return true
	})
	sort.Strings(violations)
	return violations, reached
}

// isUninspectedSet reports whether e selects the uninspected set off anything.
func isUninspectedSet(e ast.Node) bool {
	sel, ok := e.(*ast.SelectorExpr)
	return ok && sel.Sel != nil && sel.Sel.Name == uninspectedSetField
}

// paramNameAt returns the name of fn's i-th parameter, or "" when it has none
// (an unnamed or variadic-past-the-end position).
func paramNameAt(fn *ast.FuncDecl, i int) string {
	if fn.Type.Params == nil {
		return ""
	}
	pos := 0
	for _, field := range fn.Type.Params.List {
		for _, name := range field.Names {
			if pos == i {
				return name.Name
			}
			pos++
		}
		if len(field.Names) == 0 {
			pos++
		}
	}
	return ""
}

// TestConformance_UninspectedRowsRecordTheirReason walks core/aws.
// issue_enrichment.go holds the recorder itself, which is what the rule routes
// every mark through.
func TestConformance_UninspectedRowsRecordTheirReason(t *testing.T) {
	entries, err := os.ReadDir("../../core/aws")
	if err != nil {
		t.Fatalf("read core/aws: %v", err)
	}
	fset := token.NewFileSet()
	var violations []string
	recorderUses := 0
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, perr := parser.ParseFile(fset, filepath.Join("../../core/aws", name), nil, 0)
		if perr != nil {
			t.Fatalf("parse %s: %v", name, perr)
		}
		found, reached := uninspectedRecorderViolations(fset, "core/aws/"+name, file)
		if name == "issue_enrichment.go" {
			recorderUses += reached
			continue
		}
		violations = append(violations, found...)
	}
	if recorderUses == 0 {
		t.Fatalf("core/aws/issue_enrichment.go reaches %s nowhere — the gate is not reading the recorder it thinks it is", uninspectedSetField)
	}
	if len(violations) > 0 {
		sort.Strings(violations)
		t.Errorf("%d site(s) reach the uninspected set without calling the recorder, so the row arrives at "+
			"the session without the check that did not answer:\n  %s", len(violations), strings.Join(violations, "\n  "))
	}
}

// TestConformance_UninspectedGateFollowsTheCall is the gate's own probe. Its
// predecessor matched the source text `result.TruncatedIDs[x] = true`, so a
// helper that took the map and wrote an element inside itself never appeared
// in the pattern and passed — the same hole the discharge gate had.
func TestConformance_UninspectedGateFollowsTheCall(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want int
	}{
		{
			name: "marked through the recorder",
			src: `package aws
func enrich(result *IssueEnricherResult, failures *[]Failure) {
	MarkSkipped(result, "i-1", failures, errBoom)
	if _, marked := result.TruncatedIDs["i-1"]; marked {
		_ = len(result.TruncatedIDs)
	}
	for id := range result.TruncatedIDs {
		_ = id
	}
}`,
		},
		{
			name: "marked by writing the element",
			src: `package aws
func enrich(result *IssueEnricherResult) {
	result.TruncatedIDs["i-1"] = ""
}`,
			want: 1,
		},
		{
			name: "marked by a helper of the enricher's own spelling",
			src: `package aws
func noteGone(ids map[string]string, id string) { ids[id] = "" }

func enrich(result *IssueEnricherResult) {
	noteGone(result.TruncatedIDs, "i-1")
}`,
			want: 1,
		},
		{
			name: "the map lifted into a local first",
			src: `package aws
func enrich(result *IssueEnricherResult) {
	set := result.TruncatedIDs
	set["i-1"] = ""
}`,
			want: 1,
		},
		{
			// The two helpers name their parameter differently on purpose:
			// the taint is keyed by name, so a shared name would carry the
			// mark across without the hop ever being followed.
			name: "a second hop, through a helper that hands it on",
			src: `package aws
func noteGone(seen map[string]string, id string) { seen[id] = "" }

func pass(ids map[string]string, id string) { noteGone(ids, id) }

func enrich(result *IssueEnricherResult) {
	pass(result.TruncatedIDs, "i-1")
}`,
			want: 1,
		},
		{
			name: "every row copied in wholesale",
			src: `package aws
func enrich(result *IssueEnricherResult, gone map[string]string) {
	maps.Copy(result.TruncatedIDs, gone)
}`,
			want: 1,
		},
		{
			name: "the set copied OUT into a merged map",
			src: `package aws
func merge(a, b IssueEnricherResult) map[string]string {
	out := make(map[string]string, len(a.TruncatedIDs)+len(b.TruncatedIDs))
	maps.Copy(out, a.TruncatedIDs)
	maps.Copy(out, b.TruncatedIDs)
	return out
}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "probe.go", tc.src, 0)
			if err != nil {
				t.Fatalf("parse probe: %v", err)
			}
			got, _ := uninspectedRecorderViolations(fset, "probe.go", file)
			if len(got) != tc.want {
				t.Errorf("the gate reported %d violation(s) %v, want %d — it matches the shape of a write "+
					"rather than following every route to the set", len(got), got, tc.want)
			}
		})
	}
}
