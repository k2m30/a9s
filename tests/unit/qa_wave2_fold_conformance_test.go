package unit

// qa_wave2_fold_conformance_test.go — a test bench that reads an enricher's
// result map is not looking at the row the app renders.
//
// runtime.ApplyWave2ToRow is what turns an IssueEnricherResult into a row:
// it strips the previous wave-2 findings, keeps the wave-1 supporting rows,
// and keys AttentionDetails by (resource, code). A helper that instead reads
// res.Findings[id] and res.AttentionDetails[id] straight onto its own bench
// sees findings the fold would have replaced and a keying the renderer never
// uses, so it can pass while the surface is wrong — and, worse, fail while the
// surface is right.
//
// This gate finds those helpers structurally, so a new bench cannot
// reintroduce the shape.

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

// wave2FoldExempt lists functions that call an enricher and index its result
// without folding, because the result map itself is their subject.
//
// Key shape: "<file>:<func>".
var wave2FoldExempt = map[string]string{}

// TestWave2FoldConformance_BenchHelpersFoldBeforeReading walks every test file
// under tests/ and flags a function that drives a registered wave-2 enricher,
// indexes its Findings or AttentionDetails map, and never calls
// runtime.ApplyWave2ToRow.
func TestWave2FoldConformance_BenchHelpersFoldBeforeReading(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed")
	}
	testsRoot := filepath.Dir(filepath.Dir(thisFile))

	fset := token.NewFileSet()
	var violations []string
	seenExempt := map[string]bool{}

	err := filepath.WalkDir(testsRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		src, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			t.Fatalf("parse %s: %v", path, perr)
		}
		base := filepath.Base(path)
		for _, decl := range src.Decls {
			fn, isFunc := decl.(*ast.FuncDecl)
			if !isFunc || fn.Name == nil || fn.Body == nil {
				continue
			}
			if !drivesWave2Enricher(fn.Body) {
				continue
			}
			if !indexesEnricherResultMap(fn.Body) {
				continue
			}
			if callsApplyWave2ToRow(fn.Body) {
				continue
			}
			key := base + ":" + fn.Name.Name
			if _, exempt := wave2FoldExempt[key]; exempt {
				seenExempt[key] = true
				continue
			}
			violations = append(violations, fmt.Sprintf(
				"%s:%d: %s drives a wave-2 enricher and reads its Findings/AttentionDetails map "+
					"without runtime.ApplyWave2ToRow, so its bench is not the row the app renders",
				base, fset.Position(fn.Pos()).Line, fn.Name.Name))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", testsRoot, err)
	}

	for key, why := range wave2FoldExempt {
		if !seenExempt[key] {
			t.Errorf("wave2FoldExempt lists %q (%s), which this gate no longer finds — delete the entry", key, why)
		}
	}

	if len(violations) > 0 {
		sort.Strings(violations)
		t.Errorf("%d test helper(s) read an enricher result instead of a folded row:\n  %s\n\n"+
			"Fold with runtime.ApplyWave2ToRow(&row, td, res.Findings, res.AttentionDetails) and assert on the row, "+
			"or add the function to wave2FoldExempt with the reason its subject really is the result map.",
			len(violations), strings.Join(violations, "\n  "))
	}
}

// drivesWave2Enricher reports whether the body looks up a registered wave-2
// enricher and calls it, which is the generic bench shape. A test that calls a
// specific Enrich* function directly is asserting that enricher's contract and
// is not building a bench.
func drivesWave2Enricher(body ast.Node) bool {
	return bodyContainsAny(body, "Wave2EnricherFor") && bodyCallsSelector(body, "Fn")
}

// indexesEnricherResultMap reports whether the body indexes a .Findings or
// .AttentionDetails map expression, the shape of reading a result per resource.
func indexesEnricherResultMap(body ast.Node) bool {
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		if found {
			return false
		}
		idx, ok := n.(*ast.IndexExpr)
		if !ok {
			return true
		}
		sel, ok := idx.X.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if sel.Sel.Name == "Findings" || sel.Sel.Name == "AttentionDetails" {
			found = true
			return false
		}
		return true
	})
	return found
}

func callsApplyWave2ToRow(body ast.Node) bool {
	return bodyCallsSelector(body, "ApplyWave2ToRow")
}

// bodyCallsSelector reports whether the body calls a method or function whose
// selector is name.
func bodyCallsSelector(body ast.Node, name string) bool {
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		if found {
			return false
		}
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == name {
			found = true
			return false
		}
		return true
	})
	return found
}
