// qa_enricher_finding_builder_discipline_test.go — build-contract gate:
// setWave2Finding is the append-only builder for
// IssueEnricherResult.Findings (core/aws/issue_enrichment.go, its
// "Append-style:" doc paragraph), and no enricher reaches past the builder
// to read r.Findings back directly. Reading result.Findings[r.ID] back to
// decide whether to call setWave2Finding a second time defeats the append
// contract: if one check has already set a finding, a second independently
// evaluated condition is skipped — the guard collapses two conditions into
// "at most one finding, whichever fires first."
//
// SCAN SHAPE: this gate parses every core/aws/*_issue_enrichment.go file
// with go/ast (the glob deliberately does not match core/aws/
// issue_enrichment.go itself — "issue_enrichment.go" is shorter than the
// "_issue_enrichment.go" suffix the pattern requires, so setWave2Finding's
// own legitimate r.Findings[resourceID] index-write is never in scan scope)
// and flags any *ast.IndexExpr or *ast.RangeStmt whose operand is a
// "<expr>.Findings" selector — the two syntactic shapes that give an enricher
// visibility into a specific resource's existing entries (a bare
// `len(result.Findings)` or `maps.Copy(dst, result.Findings)` whole-map read
// cannot gate a per-resource emit decision the way an index or range read
// can, so those are deliberately NOT flagged).
//
// knownFindingMapInspectionDebt lists the benign shape: `for _, fs := range
// result.Findings { for _, f := range fs { if f.Severity == domain.SevBroken
// { issueCount++; break } } }` immediately before `result.IssueCount =
// issueCount` and the function's `return` — the range is the LAST statement,
// strictly after every setWave2Finding call has already run; it computes a
// derived scalar and never influences what gets appended.
package unit_test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// efbdFindingsFieldName is the exact struct field name this gate watches.
// IssueEnricherResult.Findings (core/aws/issue_enrichment.go:197) is the
// only field this gate scans for — AttentionDetails has no enricher-side
// direct-access violations today (verified: `grep -rn "\.AttentionDetails\b"
// core/aws/*_issue_enrichment.go` returns zero matches), so it is out of
// this gate's scope.
const efbdFindingsFieldName = "Findings"

// knownFindingMapInspectionDebt pins the exact inventory of pre-existing
// direct .Findings index/range sites this gate's scanner finds today that are
// NOT the targeted anti-pattern (see file header CENSUS). Keyed
// "<file>:<enclosing-func-or-package-level>#<occurrence>", mirroring
// qa_controller_construction_discipline_test.go's knownConstructionDebt
// ratchet semantics:
//
//   - A found site NOT in this allowlist is a NEW VIOLATION — always fails.
//   - An allowlisted site the live scan no longer finds fails with a
//     "prune from allowlist" message — the burn-down signal.
//   - An allowlisted site the live scan still finds is skipped (logged).
//
// msk_issue_enrichment.go:EnrichMSKCluster#1 is intentionally ABSENT — see
// file header "KNOWN DEBT vs. TARGET" for why.
//
// PRUNED (IssueEnricherResult.IssueCount field deletion): the four sites
// below were each the enclosing function's post-append `result.IssueCount =
// issueCount` aggregation — removed by production alongside the field, so
// none of the four `for _, fs := range result.Findings { ... }` blocks exist
// in core/aws/*_issue_enrichment.go anymore. Pruning rather than leaving them
// allowlisted-but-unfound, which would trip this gate's own BURN-DOWN case.
var knownFindingMapInspectionDebt = map[string]bool{}

// efbdSite is one direct .Findings index/range access site the scanner found.
type efbdSite struct {
	file     string
	line     int
	funcName string
	kind     string // "index" or "range"
	snippet  string
}

// efbdEnclosingFuncIntervals returns, for every top-level FuncDecl in file
// with a body, its name and the [start,end) token.Pos span of that body.
// Nested ast.FuncLit closures (e.g. MSK's ForEachParallel callback) share
// their enclosing FuncDecl's span — Go has no nested named funcs — so a
// position lookup against these intervals alone correctly attributes a call
// buried inside a closure to the containing top-level function. Mirrors
// ccdEnclosingFuncIntervals in qa_controller_construction_discipline_test.go.
func efbdEnclosingFuncIntervals(file *ast.File) []struct {
	name       string
	start, end token.Pos
} {
	var intervals []struct {
		name       string
		start, end token.Pos
	}
	for _, decl := range file.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok || fd.Body == nil {
			continue
		}
		intervals = append(intervals, struct {
			name       string
			start, end token.Pos
		}{fd.Name.Name, fd.Body.Pos(), fd.Body.End()})
	}
	return intervals
}

// efbdIsFindingsSelector reports whether expr is a "<x>.Findings" selector —
// the shape both *ast.IndexExpr.X (result.Findings[k]) and
// *ast.RangeStmt.X (range result.Findings) carry when the access this gate
// bans is present. Matched purely on the selector name (not a resolved
// static type), the same lexical-matching convention
// architecture_conformance_test.go's own scanners use ("a lexical heuristic,
// not a parser").
func efbdIsFindingsSelector(expr ast.Expr) bool {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	return sel.Sel.Name == efbdFindingsFieldName
}

// efbdScanFile parses path and returns every direct IndexExpr/RangeStmt
// access to a "*.Findings" selector, with the enclosing top-level function
// name and the trimmed source line attached for actionable failure messages.
func efbdScanFile(fset *token.FileSet, path string) ([]efbdSite, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(raw), "\n")
	src, err := parser.ParseFile(fset, path, raw, 0)
	if err != nil {
		return nil, err
	}

	intervals := efbdEnclosingFuncIntervals(src)
	enclosingFunc := func(pos token.Pos) string {
		for _, iv := range intervals {
			if iv.start <= pos && pos < iv.end {
				return iv.name
			}
		}
		return ""
	}
	snippetAt := func(pos token.Pos) string {
		p := fset.Position(pos)
		if p.Line-1 >= 0 && p.Line-1 < len(lines) {
			return strings.TrimSpace(lines[p.Line-1])
		}
		return ""
	}

	var sites []efbdSite
	ast.Inspect(src, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.IndexExpr:
			if efbdIsFindingsSelector(node.X) {
				pos := fset.Position(node.Pos())
				sites = append(sites, efbdSite{
					file:     filepath.Base(path),
					line:     pos.Line,
					funcName: enclosingFunc(node.Pos()),
					kind:     "index",
					snippet:  snippetAt(node.Pos()),
				})
			}
		case *ast.RangeStmt:
			if efbdIsFindingsSelector(node.X) {
				pos := fset.Position(node.Pos())
				sites = append(sites, efbdSite{
					file:     filepath.Base(path),
					line:     pos.Line,
					funcName: enclosingFunc(node.Pos()),
					kind:     "range",
					snippet:  snippetAt(node.Pos()),
				})
			}
		}
		return true
	})
	return sites, nil
}

// efbdSiteKey builds this gate's allowlist key: "<file>:<func-or-package-
// level>#<occurrence>", where occurrence disambiguates multiple direct
// .Findings accesses inside the same enclosing function. Mirrors ccdSiteKey.
func efbdSiteKey(site efbdSite, occurrence int) string {
	label := site.funcName
	if label == "" {
		label = "<package-level>"
	}
	return fmt.Sprintf("%s:%s#%d", site.file, label, occurrence)
}

// TestEnricherFindingBuilderDisciplineGate is the standing ratchet: every
// direct index/range access to a "*.Findings" selector under
// core/aws/*_issue_enrichment.go must either be pinned in
// knownFindingMapInspectionDebt (pre-existing, vetted-benign) or the gate
// fails. See the file-level doc comment for full census and ratchet-shape
// rationale.
func TestEnricherFindingBuilderDisciplineGate(t *testing.T) {
	root, err := filepath.Abs("../../core/aws")
	if err != nil {
		t.Fatalf("filepath.Abs: %v", err)
	}
	pattern := filepath.Join(root, "*_issue_enrichment.go")
	files, err := filepath.Glob(pattern)
	if err != nil {
		t.Fatalf("filepath.Glob(%q): %v", pattern, err)
	}
	if len(files) == 0 {
		t.Fatalf("no *_issue_enrichment.go files found under %s — check path/glob", root)
	}

	fset := token.NewFileSet()
	found := map[string]efbdSite{}
	fileCounts := map[string]int{}
	perFuncOccurrence := map[string]int{}

	for _, path := range files {
		sites, scanErr := efbdScanFile(fset, path)
		if scanErr != nil {
			t.Errorf("parse error in %s: %v", path, scanErr)
			continue
		}
		for _, site := range sites {
			occKey := site.file + ":" + site.funcName
			perFuncOccurrence[occKey]++
			key := efbdSiteKey(site, perFuncOccurrence[occKey])
			found[key] = site
			fileCounts[site.file]++
		}
	}

	allKeys := map[string]bool{}
	for k := range found {
		allKeys[k] = true
	}
	for k := range knownFindingMapInspectionDebt {
		allKeys[k] = true
	}
	sortedKeys := make([]string, 0, len(allKeys))
	for k := range allKeys {
		sortedKeys = append(sortedKeys, k)
	}
	sort.Strings(sortedKeys)

	var newlyRegressed, readyForBurnDown, stillGapped []string

	for _, key := range sortedKeys {
		key := key
		site, isFound := found[key]
		allowlisted := knownFindingMapInspectionDebt[key]

		t.Run(key, func(t *testing.T) {
			switch {
			case isFound && allowlisted:
				stillGapped = append(stillGapped, key)
				t.Skipf(
					"KNOWN DEBT (allowlisted): %s:%d func=%q kind=%s — %q — direct .Findings %s access "+
						"outside setWave2Finding — pre-existing post-append IssueCount aggregation (runs "+
						"after every setWave2Finding call in the function, cannot gate an emit), see "+
						"knownFindingMapInspectionDebt",
					site.file, site.line, site.funcName, site.kind, site.snippet, site.kind,
				)
			case isFound && !allowlisted:
				newlyRegressed = append(newlyRegressed, key)
				t.Errorf(
					"NEW VIOLATION (not allowlisted): %s:%d func=%q kind=%s — %q — direct .Findings %s "+
						"access defeats setWave2Finding's append-only builder contract (setWave2Finding, "+
						"core/aws/issue_enrichment.go:129, is the legitimate access path — every "+
						"independently-evaluated condition must call it, never read r.Findings back to "+
						"decide whether to call it again). Route through setWave2Finding unconditionally "+
						"for each independently-evaluated condition instead; or, if this is vetted "+
						"pre-existing debt (not a gate-a-second-emit pattern), add %q to "+
						"knownFindingMapInspectionDebt in this file",
					site.file, site.line, site.funcName, site.kind, site.snippet, site.kind, key,
				)
			case !isFound && allowlisted:
				readyForBurnDown = append(readyForBurnDown, key)
				t.Errorf(
					"BURN-DOWN: %q no longer appears in the live scan but is still pinned in "+
						"knownFindingMapInspectionDebt — remove it from the allowlist in this PR", key,
				)
			}
		})
	}

	var fileBreakdown []string
	fileNames := make([]string, 0, len(fileCounts))
	for f := range fileCounts {
		fileNames = append(fileNames, f)
	}
	sort.Strings(fileNames)
	for _, f := range fileNames {
		fileBreakdown = append(fileBreakdown, fmt.Sprintf("%s=%d", f, fileCounts[f]))
	}

	t.Logf("CENSUS SIZE: %d call site(s) found across %d file(s) (of %d *_issue_enrichment.go files scanned)", len(found), len(fileCounts), len(files))
	t.Logf("PER-FILE BREAKDOWN: %s", strings.Join(fileBreakdown, ", "))
	if len(newlyRegressed) > 0 {
		t.Logf("NEW VIOLATION INVENTORY (%d): %v", len(newlyRegressed), newlyRegressed)
	}
	if len(readyForBurnDown) > 0 {
		t.Logf("READY-FOR-BURN-DOWN INVENTORY (%d): %v", len(readyForBurnDown), readyForBurnDown)
	}
	if len(stillGapped) > 0 {
		t.Logf("STILL-GAPPED (allowlisted, skipped) INVENTORY (%d): %v", len(stillGapped), stillGapped)
	}
}
