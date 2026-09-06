package unit

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
)

// TestAppReadsTheWallClockOnlyThroughItsSeam fails when a file in core/app
// calls time.Now() directly.
//
// core/app is the lane entrance where the instant a date-dependent screen is
// seeded with is produced, and clock.go's Now() is the one place that reads
// it, so a scenario can pin the date and get the same render on any day. A
// second, direct read is invisible to the pin: the screen it stamps comes
// from the machine's clock while everything around it comes from the
// scenario's. The package already states the rule for itself in
// costs_state.go's header — "now is injected, never time.Now() internally" —
// and this is that rule with teeth.
//
// Below core/app the convention is different and deliberate: the costs code
// takes an injected now everywhere, so it needs no seam of its own.
func TestAppReadsTheWallClockOnlyThroughItsSeam(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("..", "..", "core", "app", "*.go"))
	if err != nil {
		t.Fatalf("glob core/app: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("no files found under core/app — check the repo layout")
	}

	fset := token.NewFileSet()
	for _, path := range files {
		base := filepath.Base(path)
		if base == "clock.go" || strings.HasSuffix(base, "_test.go") {
			continue
		}
		src, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			t.Fatalf("parse %s: %v", path, perr)
		}
		ast.Inspect(src, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "Now" {
				return true
			}
			pkg, ok := sel.X.(*ast.Ident)
			if !ok || pkg.Name != "time" {
				return true
			}
			t.Errorf("%s:%d: calls time.Now() directly — use app.Now(), the seam in clock.go, so a pinned scenario sees one clock",
				base, fset.Position(call.Pos()).Line)
			return true
		})
	}
}
