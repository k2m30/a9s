// The snapshot-order gate. Go
// evaluates return operands left-to-right, so `return c.snapshot(), <call>`
// runs c.snapshot() before <call>; if <call> mutates controller state (as
// forceRefreshCostsLocked does, setting cs.Loading), the returned ViewState is
// the stale pre-mutation snapshot.
//
// The gate scans every non-test *.go file under core/app for
// `return c.snapshot(), X` (or `return c.snapshot(), X, Y`) whose X is a
// method call on a receiver (`<recv>.method(...)`) rather than an
// already-computed value (an identifier, nil, or a literal). A free-function
// call such as costsTaskSlice(task) wraps an already-computed value with no
// receiver to mutate through, so it is not flagged.
package unit_test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// knownSnapshotOrderDebt allowlists sites tracked outside this gate; a site
// not listed here fails the gate.
var knownSnapshotOrderDebt = map[string]bool{}

// sodSite is one `return <snapshotCall>, <mutatingCall>[, ...]` statement
// found by sodScanFile.
type sodSite struct {
	file       string
	funcName   string
	line       int
	mutatingID string // the flagged second operand's own source text, e.g. "c.forceRefreshCostsLocked()"
}

// sodIsSnapshotCall reports whether e is a call to a zero-argument method
// named "snapshot" (case-insensitive on the leading letter would over-match
// unrelated methods, so this matches the exact name c.snapshot() uses).
func sodIsSnapshotCall(e ast.Expr) bool {
	call, ok := e.(*ast.CallExpr)
	if !ok || len(call.Args) != 0 {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	return sel.Sel.Name == "snapshot" || sel.Sel.Name == "Snapshot"
}

// sodIsInlineMethodCall reports whether e is a CallExpr on a receiver
// (`<ident>.<method>(...)`) — the shape a return statement can evaluate
// AFTER an earlier operand already ran, silently reordering a mutation.
// Excludes free-function calls (Fun is a bare Ident, e.g. costsTaskSlice)
// and non-call expressions (idents like `tasks`, `nil`, composite/slice
// literals) — those are either already-computed values (safe regardless of
// return-operand order) or have no receiver to mutate through.
func sodIsInlineMethodCall(e ast.Expr) bool {
	call, ok := e.(*ast.CallExpr)
	if !ok {
		return false
	}
	_, ok = call.Fun.(*ast.SelectorExpr)
	return ok
}

// sodExprString renders the `<ident>.<method>(...)` shape sodIsInlineMethodCall
// matches.
func sodExprString(e ast.Expr) string {
	call, ok := e.(*ast.CallExpr)
	if !ok {
		return "<expr>"
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return "<expr>"
	}
	recv, ok := sel.X.(*ast.Ident)
	if !ok {
		return sel.Sel.Name + "(...)"
	}
	return recv.Name + "." + sel.Sel.Name + "(...)"
}

// sodScanFile parses path and returns every `return c.snapshot(), <call>`
// (or 3-value form) statement whose second operand is an inline receiver
// method call.
func sodScanFile(fset *token.FileSet, path string) ([]sodSite, error) {
	src, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return nil, err
	}

	var sites []sodSite
	var funcStack []string
	ast.Inspect(src, func(n ast.Node) bool {
		switch fn := n.(type) {
		case *ast.FuncDecl:
			funcStack = append(funcStack, fn.Name.Name)
		}
		ret, ok := n.(*ast.ReturnStmt)
		if !ok || len(ret.Results) < 2 {
			return true
		}
		if !sodIsSnapshotCall(ret.Results[0]) {
			return true
		}
		if !sodIsInlineMethodCall(ret.Results[1]) {
			return true
		}
		pos := fset.Position(ret.Pos())
		funcName := ""
		if len(funcStack) > 0 {
			funcName = funcStack[len(funcStack)-1]
		}
		sites = append(sites, sodSite{
			file:       filepath.Base(path),
			funcName:   funcName,
			line:       pos.Line,
			mutatingID: sodExprString(ret.Results[1]),
		})
		return true
	})
	return sites, nil
}

// sodSiteKey builds this gate's allowlist key, matching the construction
// discipline gate's own "<file>:<func>#<line>" convention closely enough to
// be immediately recognizable, disambiguated by line since a func can have
// more than one qualifying return.
func sodSiteKey(site sodSite) string {
	return fmt.Sprintf("%s:%s#%d", site.file, site.funcName, site.line)
}

// TestSnapshotOrderDiscipline_NoInlineMutatingCallAfterSnapshot: no
// `return c.snapshot(), <call>` under core/app evaluates a receiver method call
// as its second operand. The allowlist is empty: every site found returns a
// stale ViewState.
func TestSnapshotOrderDiscipline_NoInlineMutatingCallAfterSnapshot(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed — cannot locate test file")
	}
	// tests/unit/qa_snapshot_order_discipline_test.go -> ../../core/app
	appDir := filepath.Join(filepath.Dir(thisFile), "..", "..", "core", "app")

	pattern := filepath.Join(appDir, "*.go")
	files, err := filepath.Glob(pattern)
	if err != nil {
		t.Fatalf("filepath.Glob(%q): %v", pattern, err)
	}
	if len(files) == 0 {
		t.Fatalf("no .go files found under %s — check path", appDir)
	}

	fset := token.NewFileSet()
	var allSites []sodSite
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		sites, scanErr := sodScanFile(fset, path)
		if scanErr != nil {
			t.Errorf("parse error in %s: %v", path, scanErr)
			continue
		}
		allSites = append(allSites, sites...)
	}

	for _, site := range allSites {
		site := site
		key := sodSiteKey(site)
		t.Run(key, func(t *testing.T) {
			if knownSnapshotOrderDebt[key] {
				t.Skipf("known debt (allowlisted): %s", key)
				return
			}
			t.Errorf("%s:%d (func %s) — `return c.snapshot(), %s` evaluates the snapshot BEFORE %s's own mutation (Go's left-to-right return-operand order) — the caller receives the PRE-mutation ViewState; compute the mutating call's result into a variable first, then return c.snapshot() after it, or reorder so the mutation runs before the snapshot is taken", site.file, site.line, site.funcName, site.mutatingID, site.mutatingID)
		})
	}
}
